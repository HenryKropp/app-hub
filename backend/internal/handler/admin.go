package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/jackc/pgx/v5"
)

// approvedSQL is true when the app's current manifest and version are the ones
// an administrator approved; a new version needs a new approval.
const approvedSQL = `COALESCE(s.approved AND s.reviewed_manifest_hash=encode(sha256(convert_to(a.manifest::text,'UTF8')),'hex') AND s.reviewed_version=COALESCE((SELECT max(number) FROM app_versions WHERE app_id=a.id),0),FALSE)`

type adminApp struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Area      string     `json:"area"`
	Owner     string     `json:"owner"`
	Approved  bool       `json:"approved"`
	Suspended bool       `json:"suspended"`
	Runs      int        `json:"runs_30d"`
	Errors    int        `json:"errors_30d"`
	LastRunAt *time.Time `json:"last_run_at"`
}

type adminPerson struct {
	Name string `json:"name"`
	Runs int    `json:"runs_30d"`
}

type adminComputer struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Person       string     `json:"person"`
	AgentVersion string     `json:"agent_version"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	Connected    bool       `json:"connected"`
	Outdated     bool       `json:"outdated"`
}

type adminEvent struct {
	Kind  string    `json:"kind"` // app_registered, app_updated, app_approved, app_suspended, app_unapproved, person_joined
	Name  string    `json:"name"`
	Actor string    `json:"actor"`
	At    time.Time `json:"at"`
	AppID *int64    `json:"app_id"`
}

// adminOverview is the Admin dashboard: apps by area with usage and status,
// the team's computers and recent activity.
func (h *Handler) adminOverview(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	ctx := r.Context()
	settings, err := h.securityConfig(ctx)
	if err != nil {
		internalError(w, "admin settings", err)
		return
	}
	apps, err := h.adminApps(ctx)
	if err != nil {
		internalError(w, "admin apps", err)
		return
	}
	people, err := collect(ctx, h, `SELECT p.name,count(*) FROM runs r JOIN people p ON p.id=r.person_id WHERE r.created_at > now()-interval '30 days' GROUP BY p.id ORDER BY 2 DESC, p.name LIMIT 5`,
		func(row pgx.Rows) (adminPerson, error) {
			var p adminPerson
			return p, row.Scan(&p.Name, &p.Runs)
		})
	if err != nil {
		internalError(w, "admin people", err)
		return
	}
	computers, err := collect(ctx, h, `SELECT c.id,c.name,p.name,c.agent_version,c.last_seen_at,COALESCE(c.last_seen_at > now()-interval '60 seconds',FALSE) FROM computers c JOIN people p ON p.id=c.person_id WHERE c.revoked_at IS NULL ORDER BY c.last_seen_at DESC NULLS LAST`,
		func(row pgx.Rows) (adminComputer, error) {
			var c adminComputer
			err := row.Scan(&c.ID, &c.Name, &c.Person, &c.AgentVersion, &c.LastSeenAt, &c.Connected)
			c.Outdated = c.AgentVersion != "" && c.AgentVersion != agentproto.Version
			return c, err
		})
	if err != nil {
		internalError(w, "admin computers", err)
		return
	}
	// Version 1 is the registration itself; reviews by the 015 migration have no reviewer and are not news.
	activity, err := collect(ctx, h, `SELECT * FROM (
  SELECT 'app_registered' AS kind, a.name, p.name AS actor, a.created_at AS at, a.id FROM apps a JOIN people p ON p.id=a.created_by
  UNION ALL SELECT 'app_updated', a.name, COALESCE(p.name,''), v.created_at, a.id FROM app_versions v JOIN apps a ON a.id=v.app_id LEFT JOIN people p ON p.id=v.author_id WHERE v.number > 1
  UNION ALL SELECT CASE WHEN s.suspended THEN 'app_suspended' WHEN s.approved THEN 'app_approved' ELSE 'app_unapproved' END, a.name, p.name, s.reviewed_at, a.id FROM app_security s JOIN apps a ON a.id=s.app_id JOIN people p ON p.id=s.reviewed_by WHERE s.reviewed_at IS NOT NULL
  UNION ALL SELECT 'person_joined', p.name, '', p.created_at, NULL FROM people p
) e ORDER BY at DESC LIMIT 15`,
		func(row pgx.Rows) (adminEvent, error) {
			var e adminEvent
			return e, row.Scan(&e.Kind, &e.Name, &e.Actor, &e.At, &e.AppID)
		})
	if err != nil {
		internalError(w, "admin activity", err)
		return
	}
	writeJSON(w, 200, map[string]any{"settings": settings, "apps": apps, "top_people": people, "computers": computers, "activity": activity})
}

func (h *Handler) adminApps(ctx context.Context) ([]adminApp, error) {
	return collect(ctx, h, `SELECT a.id,a.name,a.area,o.name,`+approvedSQL+`,COALESCE(s.suspended,FALSE),
  count(r.id) FILTER (WHERE r.created_at > now()-interval '30 days'),
  count(r.id) FILTER (WHERE r.created_at > now()-interval '30 days' AND r.status='error'),
  max(r.created_at)
FROM apps a JOIN people o ON o.id=a.owner_id LEFT JOIN app_security s ON s.app_id=a.id LEFT JOIN runs r ON r.app_id=a.id
GROUP BY a.id,o.name,s.app_id ORDER BY a.area,a.name`,
		func(row pgx.Rows) (adminApp, error) {
			var a adminApp
			return a, row.Scan(&a.ID, &a.Name, &a.Area, &a.Owner, &a.Approved, &a.Suspended, &a.Runs, &a.Errors, &a.LastRunAt)
		})
}

func collect[T any](ctx context.Context, h *Handler, sql string, scan func(pgx.Rows) (T, error)) ([]T, error) {
	rows, err := h.q.SecurityDB().Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// appControl is what the app page shows about approval and suspension.
type appControl struct {
	Approved        bool `json:"approved"`
	Suspended       bool `json:"suspended"`
	RequireApproval bool `json:"require_approval"`
	CanManage       bool `json:"can_manage"`
}

func (h *Handler) appControlOf(ctx context.Context, appID int64, canManage bool) (*appControl, error) {
	if h.workspace == nil {
		return nil, nil
	}
	s, err := h.securityConfig(ctx)
	if err != nil {
		return nil, err
	}
	c := appControl{RequireApproval: s.RequireApproval, CanManage: canManage}
	err = h.q.SecurityDB().QueryRow(ctx, `SELECT `+approvedSQL+`,COALESCE(s.suspended,FALSE) FROM apps a LEFT JOIN app_security s ON s.app_id=a.id WHERE a.id=$1`, appID).Scan(&c.Approved, &c.Suspended)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// setAppControl approves or suspends one app from its own page. It keeps the
// app's other security notes as they are.
func (h *Handler) setAppControl(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "App inválido.")
		return
	}
	if _, err := h.q.GetApp(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "App não encontrado.")
		return
	} else if err != nil {
		internalError(w, "app control", err)
		return
	}
	var b struct {
		Approved  bool `json:"approved"`
		Suspended bool `json:"suspended"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	unlock := h.lockApp(id)
	defer unlock()
	_, err = h.q.SecurityDB().Exec(r.Context(), `INSERT INTO app_security(app_id,approved,suspended,reviewed_by,reviewed_at,reviewed_manifest_hash,reviewed_version) SELECT $1,$2,$3,$4,now(),encode(sha256(convert_to(manifest::text,'UTF8')),'hex'),COALESCE((SELECT max(number) FROM app_versions WHERE app_id=$1),0) FROM apps WHERE id=$1 ON CONFLICT(app_id) DO UPDATE SET approved=$2,suspended=$3,reviewed_by=$4,reviewed_at=now(),reviewed_manifest_hash=EXCLUDED.reviewed_manifest_hash,reviewed_version=EXCLUDED.reviewed_version`, id, b.Approved, b.Suspended, me.ID)
	if err != nil {
		internalError(w, "save app control", err)
		return
	}
	if b.Suspended {
		if _, err = h.q.SecurityDB().Exec(r.Context(), `WITH cancelled AS (UPDATE agent_jobs SET status='failed',payload=payload-'script',done_at=now() WHERE kind='run' AND status='pending' AND run_id IN (SELECT id FROM runs WHERE app_id=$1) RETURNING run_id) UPDATE runs SET status='interrupted',finished_at=now(),error_message='App suspenso pela empresa.' WHERE id IN (SELECT run_id FROM cancelled)`, id); err != nil {
			internalError(w, "suspend pending jobs", err)
			return
		}
	}
	c, err := h.appControlOf(r.Context(), id, true)
	if err != nil {
		internalError(w, "app control", err)
		return
	}
	writeJSON(w, 200, c)
}

// adminCompanies lists the companies and whether their screen apps are on.
// It exists only in the principal company.
func (h *Handler) adminCompanies(w http.ResponseWriter, r *http.Request) {
	if h.workspace == nil || h.workspace.Companies == nil {
		writeError(w, http.StatusNotFound, "Página não encontrada.")
		return
	}
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	list, err := h.workspace.Companies(r.Context())
	if err != nil {
		internalError(w, "companies", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// setCompanyShared turns a company's screen apps on or off.
func (h *Handler) setCompanyShared(w http.ResponseWriter, r *http.Request) {
	if h.workspace == nil || h.workspace.Companies == nil || h.workspace.SetCompanyShared == nil {
		writeError(w, http.StatusNotFound, "Página não encontrada.")
		return
	}
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	var b struct {
		On *bool `json:"on"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if b.On == nil {
		writeError(w, http.StatusBadRequest, "Diga se os apps com tela ficam ligados ou desligados.")
		return
	}
	slug := r.PathValue("slug")
	if err := h.workspace.SetCompanyShared(r.Context(), slug, *b.On); errors.Is(err, ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "Empresa não encontrada.")
		return
	} else if err != nil {
		internalError(w, "set company shared", err)
		return
	}
	list, err := h.workspace.Companies(r.Context())
	if err != nil {
		internalError(w, "companies", err)
		return
	}
	for _, c := range list {
		if c.Slug == slug {
			writeJSON(w, http.StatusOK, c)
			return
		}
	}
	writeError(w, http.StatusNotFound, "Empresa não encontrada.")
}
