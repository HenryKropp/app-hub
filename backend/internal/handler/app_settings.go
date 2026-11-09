package handler

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Configurações do app (ADR 032): one list with every value an app reads from
// its environment, in two kinds. Chaves e senhas are the vault keys
// (app_keys.go: sealed, revealed on a click, audited). Outros ajustes are
// plain values (a folder, an e-mail): the ficha's non-secret shared settings
// plus the ones added on the site. Both live in app_config, and the same
// values reach the computer (the Agente, per run) and the 24h version (the
// container's environment). Only the owner or an administrator changes them;
// anyone who sees the app sees the names and whether each one is filled.

type settingItem struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	Secret    bool       `json:"secret"`
	Required  bool       `json:"required"`
	HasValue  bool       `json:"has_value"`
	Value     string     `json:"value,omitempty"` // only plain values, only for who manages
	Default   string     `json:"default,omitempty"`
	UpdatedBy string     `json:"updated_by"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type settingsList struct {
	Items     []settingItem `json:"items"`
	CanManage bool          `json:"can_manage"`
	Locked    bool          `json:"locked"`  // no server key: secrets cannot be filled or shown
	Missing   []string      `json:"missing"` // labels of required values still empty
}

const msgSettingIsSecret = " é uma senha do app: troque o valor no bloco Chaves e senhas."

// plainDefs lists an app's plain values: the ficha's non-secret shared
// settings first, then the plain rows added on the site (by name). A name the
// ficha declares as a secret never shows here.
func plainDefs(m manifest.Manifest, rows []sqlc.AppConfig) []manifest.EnvVar {
	secret := map[string]bool{}
	for _, k := range declaredKeys(m) {
		secret[k.Name] = true
	}
	for _, row := range rows { // a value made a password on the site stays one
		if row.IsSecret || len(row.ValueCipher) > 0 || looksSecret(row.Name, "") {
			secret[row.Name] = true
		}
	}
	var out []manifest.EnvVar
	seen := map[string]bool{}
	if m.Shared != nil {
		for _, e := range m.Shared.Env {
			if !e.Secret && !strings.HasPrefix(e.Name, "_") && !secret[e.Name] && !seen[e.Name] {
				out = append(out, e)
				seen[e.Name] = true
			}
		}
	}
	for _, row := range rows {
		if !row.IsSecret && len(row.ValueCipher) == 0 && !strings.HasPrefix(row.Name, "_") && !secret[row.Name] && !seen[row.Name] {
			out = append(out, manifest.EnvVar{Name: row.Name})
			seen[row.Name] = true
		}
	}
	return out
}

// plainValues are the filled plain values of an app, by name: what the
// Agente hands over next to the keys. Defaults stay with the server version.
func (h *Handler) plainValues(ctx context.Context, a sqlc.App) (map[string]string, error) {
	out := map[string]string{}
	if !manifest.HasVault(manifestOf(a).Kind) {
		return out, nil
	}
	if err := h.sealLooseSecrets(ctx, a); err != nil {
		return nil, err
	}
	rows, err := h.q.ListAppConfig(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	byName := map[string]sqlc.AppConfig{}
	for _, row := range rows {
		byName[row.Name] = row
	}
	for _, d := range plainDefs(manifestOf(a), rows) {
		if row, ok := byName[d.Name]; ok && !row.IsSecret && row.ValuePlain != "" {
			out[d.Name] = row.ValuePlain
		}
	}
	return out, nil
}

// appSettings lists the keys and the plain values of an app, with their
// state. Values of plain settings come along; the caller blanks them for who
// does not manage the app. Secret values never come.
func (h *Handler) appSettings(ctx context.Context, a sqlc.App) (settingsList, error) {
	out := settingsList{Items: []settingItem{}, Missing: []string{}, Locked: h.shared.Vault == nil}
	m := manifestOf(a)
	if !manifest.HasVault(m.Kind) {
		return out, nil
	}
	if err := h.sealLooseSecrets(ctx, a); err != nil {
		return out, err
	}
	keys, err := h.appKeys(ctx, a)
	if err != nil {
		return out, err
	}
	required := map[string]bool{}
	defaults := map[string]string{}
	if m.Shared != nil {
		for _, e := range m.Shared.Env {
			if e.Required {
				required[e.Name] = true
			}
			defaults[e.Name] = e.Default
		}
	}
	// A default (kept by the server version) counts as filled; a secret's
	// default never leaves the ficha.
	add := func(it settingItem) {
		label := it.Label
		if label == "" {
			label = it.Name
		}
		if it.Required && !it.HasValue && defaults[it.Name] == "" {
			out.Missing = append(out.Missing, label)
		}
		out.Items = append(out.Items, it)
	}
	for _, k := range keys.Keys {
		add(settingItem{Name: k.Name, Label: k.Label, Secret: true, Required: k.Required || required[k.Name], HasValue: k.HasValue,
			UpdatedBy: k.UpdatedBy, UpdatedAt: k.UpdatedAt})
	}
	rows, err := h.q.ListAppConfig(ctx, a.ID)
	if err != nil {
		return out, err
	}
	byName := map[string]sqlc.AppConfig{}
	for _, row := range rows {
		byName[row.Name] = row
	}
	names := map[int64]string{}
	for _, d := range plainDefs(m, rows) {
		it := settingItem{Name: d.Name, Label: d.Label, Required: d.Required, Default: defaults[d.Name]}
		if row, ok := byName[d.Name]; ok && !row.IsSecret && row.ValuePlain != "" {
			it.HasValue = true
			it.Value = row.ValuePlain
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
		add(it)
	}
	return out, nil
}

func (h *Handler) registerSettings(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/apps/{id}/settings", h.listSettings)
	mux.HandleFunc("PUT /api/apps/{id}/settings/{name}", h.putSetting)
	mux.HandleFunc("DELETE /api/apps/{id}/settings/{name}", h.deleteSetting)
	mux.HandleFunc("POST /api/apps/{id}/settings/{name}/secret", h.makeSecret)
}

func (h *Handler) listSettings(w http.ResponseWriter, r *http.Request) {
	a, me, acc, _, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !manifest.HasVault(manifestOf(a).Kind) {
		writeError(w, http.StatusNotFound, msgKeysNoVault)
		return
	}
	out, err := h.appSettings(r.Context(), a)
	if err != nil {
		internalError(w, "list settings", err)
		return
	}
	out.CanManage = canManageKeys(a, me, acc)
	if !out.CanManage {
		for i := range out.Items {
			out.Items[i].Value = ""
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// settingFor loads the app for someone who may change its settings, with its
// rows and plain settings, answering the error otherwise.
func (h *Handler) settingFor(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, []sqlc.AppConfig, []manifest.EnvVar, bool) {
	a, me, acc, _, ok := h.appFor(w, r)
	if !ok {
		return a, me, nil, nil, false
	}
	if !canManageKeys(a, me, acc) {
		writeError(w, http.StatusForbidden, msgKeysManage)
		return a, me, nil, nil, false
	}
	if !manifest.HasVault(manifestOf(a).Kind) {
		writeError(w, http.StatusNotFound, msgKeysNoVault)
		return a, me, nil, nil, false
	}
	rows, err := h.q.ListAppConfig(r.Context(), a.ID)
	if err != nil {
		internalError(w, "list settings", err)
		return a, me, nil, nil, false
	}
	return a, me, rows, plainDefs(manifestOf(a), rows), true
}

// putSetting saves one plain value ({value}). A name not yet listed becomes a
// new plain value while there is room; a secret's name is refused, so a
// password never turns visible by mistake.
func (h *Handler) putSetting(w http.ResponseWriter, r *http.Request) {
	a, me, rows, defs, ok := h.settingFor(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	var b struct {
		Value string `json:"value"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	listed := slices.ContainsFunc(defs, func(d manifest.EnvVar) bool { return d.Name == name })
	if !listed {
		m := manifestOf(a)
		secret := looksSecret(name, "") || slices.ContainsFunc(declaredKeys(m), func(k manifest.Key) bool { return k.Name == name }) ||
			slices.ContainsFunc(rows, func(row sqlc.AppConfig) bool { return row.Name == name && (row.IsSecret || len(row.ValueCipher) > 0) })
		added := 0
		for _, d := range defs {
			if declaredEnv(m, d.Name) == nil {
				added++
			}
		}
		switch {
		case secret:
			writeError(w, http.StatusBadRequest, name+msgSettingIsSecret)
			return
		case manifest.KeyNameProblem(name) != "":
			writeError(w, http.StatusBadRequest, manifest.KeyNameProblem(name))
			return
		case slices.Contains(manifest.ReservedEnv, name):
			writeError(w, http.StatusBadRequest, name+" é dado pelo Ziv. Escolha outro nome.")
			return
		case added >= manifest.MaxKeys:
			writeError(w, http.StatusBadRequest, "O app guarda no máximo "+strconv.Itoa(manifest.MaxKeys)+" ajustes acrescentados aqui.")
			return
		}
	}
	if b.Value == "" || len(b.Value) > maxKeyValue || strings.ContainsAny(b.Value, "\r\n\x00") {
		writeError(w, http.StatusBadRequest, msgKeyBadValue)
		return
	}
	if err := h.q.UpsertAppConfig(r.Context(), sqlc.UpsertAppConfigParams{AppID: a.ID, Name: name, ValuePlain: b.Value,
		UpdatedBy: pgtype.Int8{Int64: me.ID, Valid: true}}); err != nil {
		internalError(w, "save setting", err)
		return
	}
	h.keyAudit(r.Context(), me.ID, "app_setting.saved", "PUT /api/apps/{id}/settings/{name}")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "needs_restart": h.sharedRunning(r.Context(), a.ID)})
}

// deleteSetting empties a plain value of the ficha and removes one added on the site.
func (h *Handler) deleteSetting(w http.ResponseWriter, r *http.Request) {
	a, me, _, defs, ok := h.settingFor(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if !slices.ContainsFunc(defs, func(d manifest.EnvVar) bool { return d.Name == name }) {
		writeError(w, http.StatusNotFound, "Esse ajuste não está nas configurações do app.")
		return
	}
	if err := h.q.DeleteAppConfig(r.Context(), sqlc.DeleteAppConfigParams{AppID: a.ID, Name: name}); err != nil {
		internalError(w, "delete setting", err)
		return
	}
	h.keyAudit(r.Context(), me.ID, "app_setting.deleted", "DELETE /api/apps/{id}/settings/{name}")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "needs_restart": h.sharedRunning(r.Context(), a.ID)})
}
