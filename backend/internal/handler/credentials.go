package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/jackc/pgx/v5"
)

// Metadata is intentionally not a database-row serialization. Even ciphertext
// and destination are excluded from list/export responses.
type credentialMeta struct {
	ID          string    `json:"id"`
	AppID       int64     `json:"app_id"`
	Environment string    `json:"environment"`
	RoleID      int64     `json:"role_id"`
	Revision    int       `json:"revision"`
	Revoked     bool      `json:"revoked"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *Handler) registerCredentials(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/security/credentials", h.listCredentials)
	mux.HandleFunc("POST /api/security/credentials", h.createCredential)
	mux.HandleFunc("PUT /api/security/credentials/{cid}", h.replaceCredential)
	mux.HandleFunc("DELETE /api/security/credentials/{cid}", h.revokeCredential)
	mux.HandleFunc("POST /api/security/credentials/rewrap", h.rewrapCredentials)
	mux.HandleFunc("GET /api/security/credentials/audit", h.credentialAudit)
	mux.HandleFunc("POST /api/apps/{id}/credentials/{cid}/check", h.checkCredential)
}

// All credential failures are static: a pg error can include a failing row.
// Never pass raw errors, request structs or upstream responses to slog.
func credentialFailure(w http.ResponseWriter) {
	writeError(w, 503, "Não foi possível concluir a operação segura do cofre.")
}

func (h *Handler) credentialReady(w http.ResponseWriter) bool {
	if h.credentialKeys == nil {
		writeError(w, 503, "Cofre bloqueado. Configure CREDENTIAL_VAULT_KEYRING nos Secrets do workspace.")
		return false
	}
	return true
}

func (h *Handler) credentialTx(ctx context.Context) (pgx.Tx, error) {
	db, ok := h.q.SecurityDB().(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, vault.ErrEnvelope
	}
	return db.Begin(ctx)
}

func (h *Handler) credentialIdentity(c credentialMeta, endpoint string) []byte {
	scope := "principal"
	if h.workspace != nil {
		scope = h.workspace.Slug
	}
	b, _ := json.Marshal([]any{"credential-v1", scope, c.ID, c.AppID, c.Environment, c.RoleID, endpoint, c.Revision})
	return b
}

func readCredential(w http.ResponseWriter, r *http.Request) (credentialMeta, string, []byte, bool) {
	var b struct {
		AppID       int64  `json:"app_id"`
		Environment string `json:"environment"`
		RoleID      int64  `json:"role_id"`
		Endpoint    string `json:"endpoint"`
		Value       string `json:"value"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 24<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&b) != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, "Dados de credencial inválidos.")
		return credentialMeta{}, "", nil, false
	}
	value := []byte(b.Value)
	b.Value = ""
	if b.AppID < 1 || b.RoleID < 1 || (b.Environment != "development" && b.Environment != "staging" && b.Environment != "production") ||
		!vault.ValidateEndpoint(b.Endpoint) || len(value) == 0 || len(value) > 8192 || strings.ContainsAny(string(value), "\r\n") {
		clear(value)
		writeError(w, 400, "Informe app, cargo, ambiente e destino HTTPS válidos e um token Bearer de até 8192 bytes, sem quebras de linha.")
		return credentialMeta{}, "", nil, false
	}
	return credentialMeta{AppID: b.AppID, Environment: b.Environment, RoleID: b.RoleID}, b.Endpoint, value, true
}

func (h *Handler) listCredentials(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	rows, err := h.q.SecurityDB().Query(r.Context(), `SELECT id,app_id,environment,role_id,revision,revoked,updated_at FROM credential_vault ORDER BY created_at DESC`)
	if err != nil {
		credentialFailure(w)
		return
	}
	defer rows.Close()
	items := []credentialMeta{}
	for rows.Next() {
		var c credentialMeta
		if rows.Scan(&c.ID, &c.AppID, &c.Environment, &c.RoleID, &c.Revision, &c.Revoked, &c.UpdatedAt) != nil {
			credentialFailure(w)
			return
		}
		items = append(items, c)
	}
	if rows.Err() != nil {
		credentialFailure(w)
		return
	}
	writeJSON(w, 200, map[string]any{"available": h.credentialKeys != nil, "credentials": items})
}

func (h *Handler) createCredential(w http.ResponseWriter, r *http.Request) {
	h.saveCredential(w, r, false)
}
func (h *Handler) replaceCredential(w http.ResponseWriter, r *http.Request) {
	h.saveCredential(w, r, true)
}

func (h *Handler) saveCredential(w http.ResponseWriter, r *http.Request, replace bool) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok || !h.credentialReady(w) {
		return
	}
	c, endpoint, value, ok := readCredential(w, r)
	if !ok {
		return
	}
	defer clear(value)
	tx, err := h.credentialTx(r.Context())
	if err != nil {
		credentialFailure(w)
		return
	}
	defer tx.Rollback(r.Context())
	action, status := "created", http.StatusCreated
	c.Revision = 1
	if replace {
		action, status, c.ID = "rotated", http.StatusOK, r.PathValue("cid")
		var appID int64
		var environment string
		if tx.QueryRow(r.Context(), `SELECT app_id,environment,revision+1 FROM credential_vault WHERE id=$1 AND NOT revoked FOR UPDATE`, c.ID).Scan(&appID, &environment, &c.Revision) != nil {
			writeError(w, 404, "Credencial ativa não encontrada.")
			return
		}
		if appID != c.AppID || environment != c.Environment {
			writeError(w, 400, "App e ambiente são imutáveis. Crie outra credencial.")
			return
		}
	} else {
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			credentialFailure(w)
			return
		}
		c.ID = hex.EncodeToString(id)
	}
	// Existence is checked in this company's pool, never against public.
	var valid bool
	if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM apps WHERE id=$1) AND EXISTS(SELECT 1 FROM roles WHERE id=$2)`, c.AppID, c.RoleID).Scan(&valid) != nil || !valid {
		writeError(w, 400, "App ou cargo não pertence a esta empresa.")
		return
	}
	envelope, err := h.credentialKeys.Encrypt(value, h.credentialIdentity(c, endpoint))
	if err != nil {
		credentialFailure(w)
		return
	}
	raw, _ := json.Marshal(envelope)
	if replace {
		err = tx.QueryRow(r.Context(), `UPDATE credential_vault SET role_id=$2,endpoint=$3,revision=$4,envelope=$5,updated_at=now() WHERE id=$1 RETURNING updated_at`, c.ID, c.RoleID, endpoint, c.Revision, raw).Scan(&c.UpdatedAt)
	} else {
		err = tx.QueryRow(r.Context(), `INSERT INTO credential_vault(id,app_id,environment,role_id,endpoint,envelope) VALUES($1,$2,$3,$4,$5,$6) RETURNING updated_at`, c.ID, c.AppID, c.Environment, c.RoleID, endpoint, raw).Scan(&c.UpdatedAt)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO credential_audit(credential_id,actor_id,revision,action) VALUES($1,$2,$3,$4)`, c.ID, me.ID, c.Revision, action)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		credentialFailure(w)
		return
	}
	writeJSON(w, status, c)
}

func (h *Handler) revokeCredential(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	// Revocation must work even with a missing keyring.
	var n int
	err := h.q.SecurityDB().QueryRow(r.Context(), `WITH revoked AS (
		UPDATE credential_vault SET revoked=TRUE,envelope=NULL,updated_at=now() WHERE id=$1 RETURNING id,revision)
		INSERT INTO credential_audit(credential_id,actor_id,revision,action) SELECT id,$2,revision,'revoked' FROM revoked RETURNING revision`, r.PathValue("cid"), me.ID).Scan(&n)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "Credencial não encontrada.")
		return
	}
	if err != nil {
		credentialFailure(w)
		return
	}
	writeJSON(w, 200, map[string]bool{"revoked": true})
}

func (h *Handler) rewrapCredentials(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok || !h.credentialReady(w) {
		return
	}
	tx, err := h.credentialTx(r.Context())
	if err != nil {
		credentialFailure(w)
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT id,app_id,environment,role_id,revision,endpoint,envelope FROM credential_vault WHERE NOT revoked ORDER BY id FOR UPDATE`)
	if err != nil {
		credentialFailure(w)
		return
	}
	type pending struct {
		c        credentialMeta
		endpoint string
		raw      []byte
	}
	var items []pending
	for rows.Next() {
		var p pending
		if rows.Scan(&p.c.ID, &p.c.AppID, &p.c.Environment, &p.c.RoleID, &p.c.Revision, &p.endpoint, &p.raw) != nil {
			rows.Close()
			credentialFailure(w)
			return
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		credentialFailure(w)
		return
	}
	for _, p := range items {
		var e vault.Envelope
		if json.Unmarshal(p.raw, &e) != nil {
			credentialFailure(w)
			return
		}
		e, err = h.credentialKeys.Rewrap(e, h.credentialIdentity(p.c, p.endpoint))
		if err != nil {
			credentialFailure(w)
			return
		}
		raw, _ := json.Marshal(e)
		if _, err = tx.Exec(r.Context(), `UPDATE credential_vault SET envelope=$2,updated_at=now() WHERE id=$1`, p.c.ID, raw); err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO credential_audit(credential_id,actor_id,revision,action) VALUES($1,$2,$3,'rotated')`, p.c.ID, me.ID, p.c.Revision)
		}
		if err != nil {
			credentialFailure(w)
			return
		}
	}
	if tx.Commit(r.Context()) != nil {
		credentialFailure(w)
		return
	}
	writeJSON(w, 200, map[string]int{"rewrapped": len(items)})
}

func (h *Handler) credentialAudit(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	var raw []byte
	err := h.q.SecurityDB().QueryRow(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(e)),'[]') FROM (SELECT id,credential_id,actor_id,revision,action,created_at FROM credential_audit ORDER BY id DESC LIMIT 100) e`).Scan(&raw)
	if err != nil {
		credentialFailure(w)
		return
	}
	writeJSON(w, 200, json.RawMessage(raw))
}

func credentialIDValid(id string) bool {
	decoded, err := hex.DecodeString(id)
	return err == nil && len(decoded) == 16
}
