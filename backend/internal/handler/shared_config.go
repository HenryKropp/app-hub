package handler

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
)

type configItem struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Secret   bool   `json:"secret"`
	Required bool   `json:"required"`
	HasValue bool   `json:"has_value"`
	Value    string `json:"value"` // only for settings that are not secret
	Default  string `json:"default"`
}

// declaredEnv finds a setting the manifest declares. Names starting with "_"
// are the Hub's own (the app database's password) and never reachable here.
func declaredEnv(m manifest.Manifest, name string) *manifest.EnvVar {
	if m.Shared == nil || name == "" || strings.HasPrefix(name, "_") {
		return nil
	}
	for i := range m.Shared.Env {
		if m.Shared.Env[i].Name == name {
			return &m.Shared.Env[i]
		}
	}
	return nil
}

// getSharedConfig lists the settings the manifest declares, with their state.
// Secret values never leave the Hub.
func (h *Handler) getSharedConfig(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	m := manifestOf(a)
	rows, err := h.q.ListAppConfig(r.Context(), a.ID)
	if err != nil {
		internalError(w, "list config", err)
		return
	}
	have := map[string]sqlc.AppConfig{}
	for _, row := range rows {
		have[row.Name] = row
	}
	out := []configItem{}
	if m.Shared != nil {
		for _, e := range m.Shared.Env {
			if strings.HasPrefix(e.Name, "_") {
				continue
			}
			row, ok := have[e.Name]
			it := configItem{Name: e.Name, Label: e.Label, Secret: e.Secret, Required: e.Required, Default: e.Default,
				HasValue: ok && (row.ValuePlain != "" || len(row.ValueCipher) > 0)}
			if ok && !e.Secret && !row.IsSecret {
				it.Value = row.ValuePlain
			}
			out = append(out, it)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// putSharedConfig saves one setting ({name, value}; empty value deletes it).
// A secret is sealed with the Hub's key before it is stored.
func (h *Handler) putSharedConfig(w http.ResponseWriter, r *http.Request) {
	a, me, _, sa, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	var body struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	decl := declaredEnv(manifestOf(a), body.Name)
	if decl == nil {
		writeError(w, http.StatusBadRequest, "Essa configuração não está na lista do app. Acrescente-a na lista de configurações primeiro.")
		return
	}
	if strings.ContainsAny(body.Value, "\r\n\x00") {
		writeError(w, http.StatusBadRequest, "O valor não pode ter quebra de linha. Cole o valor numa linha só.")
		return
	}
	if len(body.Value) > 4096 {
		writeError(w, http.StatusBadRequest, "O valor passa de 4.096 caracteres. Use um valor mais curto.")
		return
	}
	if body.Value == "" {
		if err := h.q.DeleteAppConfig(r.Context(), sqlc.DeleteAppConfigParams{AppID: a.ID, Name: body.Name}); err != nil {
			internalError(w, "delete config", err)
			return
		}
	} else {
		p := sqlc.UpsertAppConfigParams{AppID: a.ID, Name: body.Name, IsSecret: decl.Secret, UpdatedBy: pgtype.Int8{Int64: me.ID, Valid: true}}
		if decl.Secret {
			if h.shared.Vault == nil {
				sharedError(w, deploy.ErrNoVault)
				return
			}
			sealed, err := h.shared.Vault.Seal(body.Value)
			if err != nil {
				internalError(w, "seal config", err)
				return
			}
			p.ValueCipher = sealed
		} else {
			p.ValuePlain = body.Value
		}
		if err := h.q.UpsertAppConfig(r.Context(), p); err != nil {
			internalError(w, "upsert config", err)
			return
		}
	}
	running := sa != nil && !sa.UnsharedAt.Valid && sa.Status == "running"
	writeJSON(w, http.StatusOK, map[string]bool{"needs_restart": running})
}
