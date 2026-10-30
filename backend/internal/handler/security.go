package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// WorkspaceOptions are supplied by the server, never by a request header.
// Each workspace owns a distinct Handler, pool/search_path and in-memory state.
type WorkspaceOptions struct {
	Slug, Name, BasePath string
	Create               func(context.Context, string, string, sqlc.Person) (string, error)
}

func (h *Handler) SetWorkspace(o WorkspaceOptions) { h.workspace = &o }

func (h *Handler) cookiePath() string {
	if h.workspace != nil && h.workspace.BasePath != "" {
		return h.workspace.BasePath + "/"
	}
	return "/"
}

type securitySettings struct {
	RetentionDays   int  `json:"retention_days"`
	RequireApproval bool `json:"require_app_approval"`
	BlockSecrets    bool `json:"block_embedded_secrets"`
	LocalExecution  bool `json:"local_execution_enabled"`
}

type securityApp struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Classification string   `json:"classification"`
	Purpose        string   `json:"purpose"`
	Domains        []string `json:"external_domains"`
	Approved       bool     `json:"approved"`
	Suspended      bool     `json:"suspended"`
}

func (h *Handler) securityConfig(ctx context.Context) (securitySettings, error) {
	var s securitySettings
	err := h.q.SecurityDB().QueryRow(ctx, `SELECT retention_days,require_app_approval,block_embedded_secrets,local_execution_enabled FROM security_settings WHERE id=TRUE`).
		Scan(&s.RetentionDays, &s.RequireApproval, &s.BlockSecrets, &s.LocalExecution)
	return s, err
}

func (h *Handler) workspaceInfo(w http.ResponseWriter, r *http.Request) {
	slug, name, base := "principal", "Minha empresa", ""
	if h.workspace != nil {
		slug, name, base = h.workspace.Slug, h.workspace.Name, h.workspace.BasePath
	}
	manage := false
	if c, err := r.Cookie(cookieName); err == nil {
		if p, err := h.q.GetSessionPerson(r.Context(), c.Value); err == nil {
			if acc, err := h.loadAccess(r.Context(), p); err == nil {
				manage = acc.Has(access.ManageRoles)
			}
		}
	}
	// Company names are public within their explicit workspace URL, not a directory.
	if p, err := h.q.OnboardingProgress(r.Context(), 0); err == nil && p.CompanyName != "" {
		name = p.CompanyName
	}
	writeJSON(w, 200, map[string]any{"slug": slug, "name": name, "base_path": base, "can_manage": manage, "local_execution_is_sandboxed": false, "filesystem_registration_enabled": h.workspace == nil})
}

func (h *Handler) createWorkspace(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	if h.workspace == nil || h.workspace.Create == nil {
		writeError(w, 503, "A criação de empresas não está configurada.")
		return
	}
	var b struct {
		Name     string `json:"name"`
		Slug     string `json:"slug"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	b.Name, b.Slug = strings.TrimSpace(b.Name), strings.ToLower(strings.TrimSpace(b.Slug))
	if b.Name == "" || utf8.RuneCountInString(b.Name) > 120 || !regexp.MustCompile(`^[a-z][a-z0-9-]{2,39}$`).MatchString(b.Slug) {
		writeError(w, 400, "Informe um nome e um endereço de 3 a 40 letras minúsculas, números ou hífens.")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(me.PasswordHash), []byte(b.Password)) != nil {
		writeError(w, 401, "Confirme sua senha atual para criar a empresa.")
		return
	}
	url, err := h.workspace.Create(r.Context(), b.Name, b.Slug, me)
	if err != nil {
		if errors.Is(err, ErrWorkspaceExists) {
			writeError(w, 409, "Esse endereço de empresa já está em uso.")
			return
		}
		internalError(w, "create workspace", err)
		return
	}
	writeJSON(w, 201, map[string]string{"slug": b.Slug, "name": b.Name, "url": url})
}

var ErrWorkspaceExists = errors.New("workspace already exists")

func (h *Handler) securityOverview(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	s, err := h.securityConfig(r.Context())
	if err != nil {
		internalError(w, "security settings", err)
		return
	}
	rows, err := h.q.SecurityDB().Query(r.Context(), `SELECT a.id,a.name,COALESCE(s.classification,'internal'),COALESCE(s.purpose,''),COALESCE(s.external_domains,'{}'),COALESCE(s.approved AND s.reviewed_manifest_hash=encode(sha256(convert_to(a.manifest::text,'UTF8')),'hex') AND s.reviewed_version=COALESCE((SELECT max(number) FROM app_versions WHERE app_id=a.id),0),FALSE),COALESCE(s.suspended,FALSE) FROM apps a LEFT JOIN app_security s ON s.app_id=a.id ORDER BY a.name`)
	if err != nil {
		internalError(w, "security apps", err)
		return
	}
	defer rows.Close()
	apps := []securityApp{}
	for rows.Next() {
		var a securityApp
		if err := rows.Scan(&a.ID, &a.Name, &a.Classification, &a.Purpose, &a.Domains, &a.Approved, &a.Suspended); err != nil {
			internalError(w, "security app", err)
			return
		}
		apps = append(apps, a)
	}
	if err := rows.Err(); err != nil {
		internalError(w, "security apps", err)
		return
	}
	writeJSON(w, 200, map[string]any{"can_manage": true, "settings": s, "apps": apps, "limits": map[string]bool{"sandboxed": false, "vault_available": h.credentialKeys != nil, "external_marketplace_enabled": false}})
}

func (h *Handler) updateSecuritySettings(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	var s securitySettings
	if !readJSON(w, r, &s) {
		return
	}
	if s.RetentionDays < 1 || s.RetentionDays > 365 {
		writeError(w, 400, "A retenção deve ser de 1 a 365 dias.")
		return
	}
	_, err := h.q.SecurityDB().Exec(r.Context(), `WITH settings AS (UPDATE security_settings SET retention_days=$1,require_app_approval=$2,block_embedded_secrets=$3,local_execution_enabled=$4 WHERE id=TRUE RETURNING id), cancelled AS (UPDATE agent_jobs SET status='failed',payload=payload-'script',done_at=now() WHERE NOT $4 AND kind='run' AND status='pending' RETURNING run_id) UPDATE runs SET status='interrupted',finished_at=now(),error_message='Execuções locais desativadas pela empresa.' WHERE id IN(SELECT run_id FROM cancelled)`, s.RetentionDays, s.RequireApproval, s.BlockSecrets, s.LocalExecution)
	if err != nil {
		internalError(w, "update security settings", err)
		return
	}
	writeJSON(w, 200, s)
}

func validDomain(s string) bool {
	if len(s) > 253 || strings.ContainsAny(s, "/:@*\\ \t\n\r") {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`).MatchString(part) {
			return false
		}
	}
	return true
}

func (h *Handler) updateSecurityApp(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "App inválido.")
		return
	}
	a, err := h.q.GetApp(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "App não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "security app", err)
		return
	}
	var b securityApp
	if !readJSON(w, r, &b) {
		return
	}
	b.Purpose = strings.TrimSpace(b.Purpose)
	if (b.Classification != "internal" && b.Classification != "confidential" && b.Classification != "restricted") || utf8.RuneCountInString(b.Purpose) > 1000 || len(b.Domains) > 30 {
		writeError(w, 400, "Revise a classificação, a finalidade e os domínios.")
		return
	}
	if b.Approved && b.Purpose == "" {
		writeError(w, 400, "Descreva a finalidade antes de aprovar o uso interno.")
		return
	}
	for i, d := range b.Domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if !validDomain(d) {
			writeError(w, 400, "Informe domínios sem protocolo, caminho ou curingas.")
			return
		}
		b.Domains[i] = d
	}
	if b.Domains == nil {
		b.Domains = []string{}
	}
	unlock := h.lockApp(id)
	defer unlock()
	_, err = h.q.SecurityDB().Exec(r.Context(), `INSERT INTO app_security(app_id,classification,purpose,external_domains,approved,suspended,reviewed_by,reviewed_at,reviewed_manifest_hash,reviewed_version) SELECT $1,$2,$3,$4,$5,$6,$7,now(),encode(sha256(convert_to(manifest::text,'UTF8')),'hex'),COALESCE((SELECT max(number) FROM app_versions WHERE app_id=$1),0) FROM apps WHERE id=$1 ON CONFLICT(app_id) DO UPDATE SET classification=$2,purpose=$3,external_domains=$4,approved=$5,suspended=$6,reviewed_by=$7,reviewed_at=now(),reviewed_manifest_hash=EXCLUDED.reviewed_manifest_hash,reviewed_version=EXCLUDED.reviewed_version`, id, b.Classification, b.Purpose, b.Domains, b.Approved, b.Suspended, me.ID)
	if err != nil {
		internalError(w, "save app security", err)
		return
	}
	// Suspension also cancels unclaimed jobs; an already running/offline agent
	// cannot be stopped remotely by this control.
	if b.Suspended {
		if _, err = h.q.SecurityDB().Exec(r.Context(), `WITH cancelled AS (UPDATE agent_jobs SET status='failed',payload=payload-'script',done_at=now() WHERE kind='run' AND status='pending' AND run_id IN (SELECT id FROM runs WHERE app_id=$1) RETURNING run_id) UPDATE runs SET status='interrupted',finished_at=now(),error_message='App suspenso pela empresa.' WHERE id IN (SELECT run_id FROM cancelled)`, id); err != nil {
			internalError(w, "suspend pending jobs", err)
			return
		}
	}
	b.ID, b.Name = id, a.Name
	writeJSON(w, 200, b)
}

func (h *Handler) allowExecution(w http.ResponseWriter, r *http.Request, id int64) bool {
	// Legacy unit handlers stay compatible; the portal always sets workspace.
	if h.workspace == nil {
		return true
	}
	reason, err := h.executionBlock(r.Context(), id)
	if err != nil {
		internalError(w, "execution policy", err)
		return false
	}
	if reason != "" {
		writeError(w, 403, reason)
		return false
	}
	return true
}

func (h *Handler) executionBlock(ctx context.Context, id int64) (string, error) {
	if h.workspace == nil {
		return "", nil
	}
	s, err := h.securityConfig(ctx)
	if err != nil {
		return "", err
	}
	if !s.LocalExecution {
		return "A empresa desativou as execuções locais em Segurança.", nil
	}
	var approved, suspended bool
	err = h.q.SecurityDB().QueryRow(ctx, `SELECT s.approved AND s.reviewed_manifest_hash=encode(sha256(convert_to(a.manifest::text,'UTF8')),'hex') AND s.reviewed_version=COALESCE((SELECT max(number) FROM app_versions WHERE app_id=a.id),0),s.suspended FROM app_security s JOIN apps a ON a.id=s.app_id WHERE a.id=$1`, id).Scan(&approved, &suspended)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if suspended {
		return "Este app foi suspenso pela empresa.", nil
	}
	if s.RequireApproval && !approved {
		return "Um administrador precisa aprovar este app para uso interno em Segurança.", nil
	}
	return "", nil
}

type auditEvent struct {
	ID      int64     `json:"id"`
	Actor   *int64    `json:"actor_id"`
	Action  string    `json:"action"`
	Route   string    `json:"route"`
	Status  int       `json:"status"`
	Created time.Time `json:"created_at"`
}

func (h *Handler) auditEvents(ctx context.Context) ([]auditEvent, error) {
	rows, err := h.q.SecurityDB().Query(ctx, `SELECT id,actor_id,action,route,status,created_at FROM security_audit ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []auditEvent{}
	for rows.Next() {
		var e auditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Route, &e.Status, &e.Created); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (h *Handler) securityAudit(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	events, err := h.auditEvents(r.Context())
	if err != nil {
		internalError(w, "audit", err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": events})
}

func (h *Handler) securityExport(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	s, err := h.securityConfig(r.Context())
	if err != nil {
		internalError(w, "export settings", err)
		return
	}
	events, err := h.auditEvents(r.Context())
	if err != nil {
		internalError(w, "export audit", err)
		return
	}
	// This is a deliberately limited metadata export, not a personal-data
	// access request response, full backup or credential export.
	var raw []byte
	err = h.q.SecurityDB().QueryRow(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',a.id,'name',a.name,'classification',COALESCE(s.classification,'internal'),'approved',COALESCE(s.approved,false),'suspended',COALESCE(s.suspended,false),'purpose',COALESCE(s.purpose,''),'external_domains',COALESCE(s.external_domains,'{}'))),'[]') FROM apps a LEFT JOIN app_security s ON s.app_id=a.id`).Scan(&raw)
	if err != nil {
		internalError(w, "export apps", err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="seguranca-empresa.json"`)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"exported_at": time.Now().UTC(), "scope": "security_metadata_only", "settings": s, "apps": json.RawMessage(raw), "audit_last_100": events, "excludes": []string{"credentials", "source_code", "input_files", "output_files", "backups", "personal_data_request"}})
}

func (h *Handler) securityPurge(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	var b struct {
		Confirm bool `json:"confirm"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if !b.Confirm {
		writeError(w, 400, "Confirme a exclusão dos registros antigos.")
		return
	}
	s, err := h.securityConfig(r.Context())
	if err != nil {
		internalError(w, "retention policy", err)
		return
	}
	// One statement is atomic; preserve active executions and audit records.
	var n int64
	err = h.q.SecurityDB().QueryRow(r.Context(), `WITH old AS (SELECT id FROM runs WHERE status IN ('done','no_files','error','interrupted','stopped') AND finished_at < now()-make_interval(days=>$1)), jobs AS (DELETE FROM agent_jobs WHERE run_id IN(SELECT id FROM old)), removed AS (DELETE FROM runs WHERE id IN(SELECT id FROM old) RETURNING id) SELECT count(*) FROM removed`, s.RetentionDays).Scan(&n)
	if err != nil {
		internalError(w, "retention purge", err)
		return
	}
	writeJSON(w, 200, map[string]int64{"deleted_runs": n})
}
