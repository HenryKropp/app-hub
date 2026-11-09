package handler

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// secretLike is a name or label that reads like a password: such a value is
// always a password in Configurações, whatever the ficha says, and a run never
// keeps it (ADR 032, ADR 033).
var secretLike = regexp.MustCompile(`(?i)senha|password|passwd|pwd|token|segredo|secret|api[ _-]?key|chave (de api|de acesso|secreta)`)

// looksSecret says a setting should be treated as a password by its name or label.
func looksSecret(name, label string) bool {
	n := strings.NewReplacer("_", " ", "-", " ").Replace(name)
	return secretLike.MatchString(name) || secretLike.MatchString(n) || secretLike.MatchString(label)
}

// sealLooseSecrets locks the plain values whose name reads like a password,
// so a password saved as an ordinary setting (by the old form, or a ficha that
// did not mark it) stops showing. Without a server key it leaves them as they are.
func (h *Handler) sealLooseSecrets(ctx context.Context, a sqlc.App) error {
	if h.shared.Vault == nil {
		return nil
	}
	rows, err := h.q.ListAppConfig(ctx, a.ID)
	if err != nil {
		return err
	}
	m := manifestOf(a)
	for _, row := range rows {
		if row.IsSecret || len(row.ValueCipher) > 0 || row.ValuePlain == "" || strings.HasPrefix(row.Name, "_") {
			continue
		}
		label := ""
		if e := declaredEnv(m, row.Name); e != nil {
			label = e.Label
		}
		if looksSecret(row.Name, label) {
			if err := h.sealSetting(ctx, a.ID, row); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Handler) sealSetting(ctx context.Context, appID int64, row sqlc.AppConfig) error {
	sealed, err := h.shared.Vault.Seal(row.ValuePlain)
	if err != nil {
		return err
	}
	return h.q.UpsertAppConfig(ctx, sqlc.UpsertAppConfigParams{AppID: appID, Name: row.Name, IsSecret: true,
		ValueCipher: sealed, UpdatedBy: row.UpdatedBy})
}

// makeSecret turns one plain value into a password ("Tornar senha"): it is
// sealed, leaves Outros ajustes and is shown only on Mostrar from then on.
func (h *Handler) makeSecret(w http.ResponseWriter, r *http.Request) {
	a, me, rows, _, ok := h.settingFor(w, r)
	if !ok {
		return
	}
	if h.shared.Vault == nil {
		writeError(w, http.StatusConflict, "O servidor está sem a chave que protege as senhas. Peça ao administrador do Ziv para configurá-la.")
		return
	}
	name := r.PathValue("name")
	var row *sqlc.AppConfig
	for i := range rows {
		if rows[i].Name == name {
			row = &rows[i]
		}
	}
	switch {
	case row == nil || row.ValuePlain == "" && !row.IsSecret:
		writeError(w, http.StatusNotFound, "Esse ajuste não tem valor para virar senha.")
		return
	case row.IsSecret || len(row.ValueCipher) > 0:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	row.UpdatedBy = pgtype.Int8{Int64: me.ID, Valid: true}
	if err := h.sealSetting(r.Context(), a.ID, *row); err != nil {
		internalError(w, "make secret", err)
		return
	}
	h.keyAudit(r.Context(), me.ID, "app_setting.made_secret", "POST /api/apps/{id}/settings/{name}/secret")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "needs_restart": h.sharedRunning(r.Context(), a.ID)})
}
