package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/shared/supervisor"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

// Shared holds the parts that run shared apps. Svc nil = not running.
type Shared struct {
	Svc     *deploy.Service
	Sup     *supervisor.Supervisor // nil nos testes
	Vault   *vault.Vault           // nil = sem HUB_SECRET_KEY
	DBs     appdb.DBs
	Eng     engine.Engine
	Codes   *gateway.Codes
	DataDir string
}

// SetShared plugs in the shared-app parts. Call before serving.
func (h *Handler) SetShared(s Shared) { h.shared = s }

// sharedReady answers 503 when the shared-app parts are not running.
func (h *Handler) sharedReady(w http.ResponseWriter) bool {
	if h.shared.Svc == nil || h.shared.Codes == nil {
		writeError(w, http.StatusServiceUnavailable, "O servidor de apps não está disponível. Tente de novo em alguns minutos ou fale com o administrador.")
		return false
	}
	return true
}

// sharedAuthorize hands a signed-in person over to an app address: it sends
// the browser to the app with a single-use code (or to the app's "no access"
// page), or to the Hub's sign-in first.
func (h *Handler) sharedAuthorize(w http.ResponseWriter, r *http.Request) {
	if !h.sharedReady(w) {
		return
	}
	q := r.URL.Query()
	slug, test := q.Get("app"), q.Get("teste") == "1"
	volta := gateway.SafeVolta(q.Get("volta"))
	me, found, err := h.sessionPerson(r)
	if err != nil {
		internalError(w, "authorize session", err)
		return
	}
	if !found {
		http.Redirect(w, r, "/entrar?volta="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	sa, err := h.q.GetSharedAppBySlug(r.Context(), slug)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "App não encontrado. Confira o endereço ou abra o app pela lista do Hub.")
		return
	}
	if err != nil {
		internalError(w, "authorize shared app", err)
		return
	}
	app, err := h.q.GetApp(r.Context(), sa.AppID)
	if err != nil {
		internalError(w, "authorize app", err)
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "authorize access", err)
		return
	}
	can := acc.For(appRef(app))
	base := h.shared.Svc.AppURL(sa.Slug)
	allowed := can.View
	if test {
		base, allowed = h.shared.Svc.TestURL(sa.Slug), can.Restore
	}
	if !allowed {
		http.Redirect(w, r, base+"/__apphub/sem-acesso", http.StatusFound)
		return
	}
	code := h.shared.Codes.Issue(me.ID, app.ID, test)
	http.Redirect(w, r, base+"/__apphub/entrar?"+url.Values{"code": {code}, "volta": {volta}}.Encode(), http.StatusFound)
}

// sessionPerson is currentPerson without writing a 401.
func (h *Handler) sessionPerson(r *http.Request) (sqlc.Person, bool, error) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return sqlc.Person{}, false, nil
	}
	p, err := h.q.GetSessionPerson(r.Context(), c.Value)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Person{}, false, nil
	}
	return p, err == nil, err
}

type gatewayBackend struct{ h *Handler }

// GatewayBackend is what the gateway asks the Hub (apps, people, access).
func (h *Handler) GatewayBackend() gateway.Backend { return gatewayBackend{h} }

func (b gatewayBackend) Lookup(ctx context.Context, slug string, test bool) (gateway.Target, bool, error) {
	sa, err := b.h.q.GetSharedAppBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return gateway.Target{}, false, nil
	}
	if err != nil {
		return gateway.Target{}, false, err
	}
	app, err := b.h.q.GetApp(ctx, sa.AppID)
	if err != nil {
		return gateway.Target{}, false, err
	}
	t := gateway.Target{AppID: app.ID, Name: app.Name, Status: sa.Status}
	if owner, err := b.h.q.GetPerson(ctx, app.OwnerID); err == nil {
		t.Owner = owner.Name
	}
	if sa.StoppedBy.Valid {
		if p, err := b.h.q.GetPerson(ctx, sa.StoppedBy.Int64); err == nil {
			t.StoppedBy = p.Name
		}
	}
	if b.h.shared.Svc != nil {
		if port, ok := b.h.shared.Svc.Target(app.ID, test); ok {
			t.Port = port
		}
	}
	return t, true, nil
}

func (b gatewayBackend) Allowed(ctx context.Context, personID, appID int64, test bool) (gateway.Identity, bool, error) {
	p, err := b.h.q.GetPerson(ctx, personID)
	if errors.Is(err, pgx.ErrNoRows) {
		return gateway.Identity{}, false, nil
	}
	if err != nil {
		return gateway.Identity{}, false, err
	}
	app, err := b.h.q.GetApp(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return gateway.Identity{}, false, nil
	}
	if err != nil {
		return gateway.Identity{}, false, err
	}
	acc, err := b.h.loadAccess(ctx, p)
	if err != nil {
		return gateway.Identity{}, false, err
	}
	can := acc.For(appRef(app))
	id := gateway.Identity{ID: p.ID, Email: p.Email, Name: p.Name}
	if role, err := b.h.q.GetRole(ctx, p.RoleID); err == nil {
		id.Role = role.Name
	}
	if test {
		return id, can.Restore, nil
	}
	return id, can.View, nil
}

func (b gatewayBackend) Wake(appID int64) {
	if b.h.shared.Svc != nil {
		b.h.shared.Svc.Wake(appID)
	}
}

func (b gatewayBackend) Touch(appID int64) {
	if b.h.shared.Sup != nil {
		b.h.shared.Sup.Touch(appID)
	}
}
