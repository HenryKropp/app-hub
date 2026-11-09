package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// The app's vault (cofre): the secrets a script reads from its environment.
// The ficha names them (manifest.Keys); the values are sealed in app_config
// with the server key. Only the app's owner or an administrator fills,
// replaces, reveals or deletes them; anyone who sees the app sees the names
// and whether each one is filled. The Agente gets them per run (agent_keys.go).

const (
	msgKeysLocked   = "O cofre está fechado neste servidor (falta a chave do servidor). Fale com o responsável pelo Ziv."
	msgKeysManage   = "Só o dono do app ou um administrador mexe no cofre."
	msgKeyNotListed = "Essa chave não está na ficha do app."
	msgKeyBadValue  = "O valor pode ter até 4.096 caracteres, numa linha só."
	maxKeyValue     = 4096
)

type keyItem struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	Required  bool       `json:"required"`
	HasValue  bool       `json:"has_value"`
	UpdatedBy string     `json:"updated_by"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type keysList struct {
	Keys      []keyItem `json:"keys"`
	CanManage bool      `json:"can_manage"`
	Ready     bool      `json:"ready"`   // every required key is filled
	Missing   []string  `json:"missing"` // required keys still empty
	Locked    bool      `json:"locked"`  // no server key: the vault cannot open
}

func (h *Handler) registerKeys(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/apps/{id}/keys", h.listKeys)
	mux.HandleFunc("PUT /api/apps/{id}/keys/{name}", h.putKey)
	mux.HandleFunc("DELETE /api/apps/{id}/keys/{name}", h.deleteKey)
	mux.HandleFunc("POST /api/apps/{id}/keys/{name}/reveal", h.revealKey)
	mux.HandleFunc("GET /api/agent/jobs/{id}/keys", h.agentKeys)
}

// canManageKeys: the app's owner or someone who manages roles (administrator).
func canManageKeys(a sqlc.App, me sqlc.Person, acc *access.Person) bool {
	return a.OwnerID == me.ID || (acc != nil && acc.Has(access.ManageRoles))
}

func declaredKey(m manifest.Manifest, name string) *manifest.Key {
	for i := range m.Keys {
		if m.Keys[i].Name == name {
			return &m.Keys[i]
		}
	}
	return nil
}

// appKeys lists the ficha's keys with their state, never the values.
func (h *Handler) appKeys(ctx context.Context, a sqlc.App) (keysList, error) {
	m := manifestOf(a)
	out := keysList{Keys: []keyItem{}, Missing: []string{}, Locked: h.shared.Vault == nil}
	if len(m.Keys) == 0 {
		out.Ready = true
		return out, nil
	}
	rows, err := h.q.ListAppConfig(ctx, a.ID)
	if err != nil {
		return out, err
	}
	have := map[string]sqlc.AppConfig{}
	for _, row := range rows {
		have[row.Name] = row
	}
	names := map[int64]string{}
	for _, k := range m.Keys {
		it := keyItem{Name: k.Name, Label: k.Label, Required: k.Required}
		if row, ok := have[k.Name]; ok && len(row.ValueCipher) > 0 {
			it.HasValue = true
			if row.UpdatedAt.Valid {
				t := row.UpdatedAt.Time
				it.UpdatedAt = &t
			}
			if row.UpdatedBy.Valid {
				id := row.UpdatedBy.Int64
				if _, seen := names[id]; !seen {
					if p, err := h.q.GetPerson(ctx, id); err == nil {
						names[id] = p.Name
					}
				}
				it.UpdatedBy = names[id]
			}
		}
		if k.Required && !it.HasValue {
			out.Missing = append(out.Missing, k.Name)
		}
		out.Keys = append(out.Keys, it)
	}
	out.Ready = len(out.Missing) == 0
	return out, nil
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	a, me, acc, _, ok := h.appFor(w, r)
	if !ok {
		return
	}
	out, err := h.appKeys(r.Context(), a)
	if err != nil {
		internalError(w, "list keys", err)
		return
	}
	out.CanManage = canManageKeys(a, me, acc)
	writeJSON(w, http.StatusOK, out)
}

// keyFor loads the app and the declared key in the URL for someone who may
// manage the vault, answering the error otherwise.
func (h *Handler) keyFor(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, string, bool) {
	a, me, acc, _, ok := h.appFor(w, r)
	if !ok {
		return a, me, "", false
	}
	if !canManageKeys(a, me, acc) {
		writeError(w, http.StatusForbidden, msgKeysManage)
		return a, me, "", false
	}
	name := r.PathValue("name")
	if declaredKey(manifestOf(a), name) == nil {
		writeError(w, http.StatusNotFound, msgKeyNotListed)
		return a, me, "", false
	}
	if h.shared.Vault == nil {
		writeError(w, http.StatusServiceUnavailable, msgKeysLocked)
		return a, me, "", false
	}
	return a, me, name, true
}

func (h *Handler) putKey(w http.ResponseWriter, r *http.Request) {
	a, me, name, ok := h.keyFor(w, r)
	if !ok {
		return
	}
	var b struct {
		Value string `json:"value"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if b.Value == "" || len(b.Value) > maxKeyValue || strings.ContainsAny(b.Value, "\r\n\x00") {
		writeError(w, http.StatusBadRequest, msgKeyBadValue)
		return
	}
	sealed, err := h.shared.Vault.Seal(b.Value)
	if err != nil {
		internalError(w, "seal key", err)
		return
	}
	if err := h.q.UpsertAppConfig(r.Context(), sqlc.UpsertAppConfigParams{AppID: a.ID, Name: name, IsSecret: true,
		ValueCipher: sealed, UpdatedBy: pgtype.Int8{Int64: me.ID, Valid: true}}); err != nil {
		internalError(w, "save key", err)
		return
	}
	h.keyAudit(r.Context(), me.ID, "app_key.saved", "PUT /api/apps/{id}/keys/{name}")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	a, me, acc, _, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !canManageKeys(a, me, acc) {
		writeError(w, http.StatusForbidden, msgKeysManage)
		return
	}
	name := r.PathValue("name")
	if declaredKey(manifestOf(a), name) == nil {
		writeError(w, http.StatusNotFound, msgKeyNotListed)
		return
	}
	if err := h.q.DeleteAppConfig(r.Context(), sqlc.DeleteAppConfigParams{AppID: a.ID, Name: name}); err != nil {
		internalError(w, "delete key", err)
		return
	}
	h.keyAudit(r.Context(), me.ID, "app_key.deleted", "DELETE /api/apps/{id}/keys/{name}")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// revealKey shows one value to the owner or an administrator; every reveal
// is recorded in the security history (never the value).
func (h *Handler) revealKey(w http.ResponseWriter, r *http.Request) {
	a, me, name, ok := h.keyFor(w, r)
	if !ok {
		return
	}
	value, err := h.openKey(r.Context(), a.ID, name)
	if errors.Is(err, errKeyEmpty) {
		writeError(w, http.StatusNotFound, "Essa chave ainda não foi preenchida.")
		return
	}
	if err != nil {
		internalError(w, "open key", err)
		return
	}
	h.keyAudit(r.Context(), me.ID, "app_key.revealed", "POST /api/apps/{id}/keys/{name}/reveal")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"value": value})
}

var errKeyEmpty = errors.New("key not filled")

// openKey returns one stored key's value.
func (h *Handler) openKey(ctx context.Context, appID int64, name string) (string, error) {
	rows, err := h.q.ListAppConfig(ctx, appID)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		if row.Name == name && len(row.ValueCipher) > 0 {
			return h.shared.Vault.Open(row.ValueCipher)
		}
	}
	return "", errKeyEmpty
}

// keyValues opens every declared key that is filled, for one run.
func (h *Handler) keyValues(ctx context.Context, a sqlc.App) (map[string]string, error) {
	m := manifestOf(a)
	out := map[string]string{}
	if len(m.Keys) == 0 {
		return out, nil
	}
	if h.shared.Vault == nil {
		return nil, errors.New(msgKeysLocked)
	}
	rows, err := h.q.ListAppConfig(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if declaredKey(m, row.Name) == nil || len(row.ValueCipher) == 0 {
			continue
		}
		v, err := h.shared.Vault.Open(row.ValueCipher)
		if err != nil {
			return nil, err
		}
		out[row.Name] = v
	}
	return out, nil
}

// keyAudit records a vault event with the route template, never the value.
func (h *Handler) keyAudit(ctx context.Context, actor int64, action, route string) {
	var who *int64
	if actor != 0 {
		who = &actor
	}
	id, err := h.auditStart(ctx, who, action)
	if err != nil {
		return
	}
	h.auditFinish(ctx, id, action, route, http.StatusOK)
}

// agentKeys hands a run's keys to the Agente that took the job, once, while
// the job is running. Anything else answers 404, the same for every case.
func (h *Handler) agentKeys(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	jobID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Trabalho não encontrado.")
		return
	}
	ctx := r.Context()
	var runID int64
	err = h.q.SecurityDB().QueryRow(ctx, `UPDATE agent_jobs SET keys_delivered_at = now()
		WHERE id = $1 AND computer_id = $2 AND kind = 'run' AND status = 'taken' AND keys_delivered_at IS NULL AND run_id IS NOT NULL
		RETURNING run_id`, jobID, c.ID).Scan(&runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Trabalho não encontrado.")
		return
	}
	run, err := h.q.GetRun(ctx, runID)
	if err != nil {
		internalError(w, "keys run", err)
		return
	}
	a, err := h.q.GetApp(ctx, run.AppID)
	if err != nil {
		internalError(w, "keys app", err)
		return
	}
	values, err := h.keyValues(ctx, a)
	if err != nil {
		slog.Error("open app keys", "app", a.ID, "err", err)
		writeError(w, http.StatusServiceUnavailable, msgKeysLocked)
		return
	}
	h.keyAudit(ctx, run.PersonID, "app_key.delivered", "GET /api/agent/jobs/{id}/keys")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, values)
}
