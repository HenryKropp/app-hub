package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/bundle"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// checkSentFiles validates code sent to the Ziv (site import, Claude upload)
// for a manifest whose Entrypoint and Runtime.DependenciesFile are final, and
// decodes it. On refusal it has answered.
func (h *Handler) checkSentFiles(w http.ResponseWriter, r *http.Request, m manifest.Manifest,
	files map[string]manifest.InputFile, confirm bool) (map[string][]byte, bool) {
	if len(files) == 0 || len(files) > 100 {
		writeError(w, http.StatusBadRequest, "Envie de 1 a 100 arquivos do script.")
		return nil, false
	}
	if _, ok := files[m.Entrypoint]; !ok {
		writeError(w, http.StatusBadRequest, "Inclua o arquivo que inicia o script.")
		return nil, false
	}
	if m.Runtime.DependenciesFile != "" {
		if _, ok := files[m.Runtime.DependenciesFile]; !ok {
			writeError(w, http.StatusBadRequest, "Inclua o arquivo de dependências escolhido.")
			return nil, false
		}
	}
	total := 0
	seen := map[string]bool{}
	decoded := map[string][]byte{}
	for name, f := range files {
		key := strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
		base := strings.ToLower(filepath.Base(name))
		if bundle.IsSecretFile(base) {
			writeError(w, http.StatusBadRequest, "Não envie arquivos de credenciais (.env, .pem, .key). Retire as senhas do pacote antes de importar.")
			return nil, false
		}
		if !manifest.SafeRelative(name) || manifest.IsManifestName(base) || name == manifest.FichaName || strings.HasPrefix(base, ".apphub-") || seen[key] || strings.Contains(name, `\`) {
			writeError(w, http.StatusBadRequest, "Use nomes de arquivos relativos, sem repetições ou caminhos externos.")
			return nil, false
		}
		seen[key] = true
		data, err := base64.StdEncoding.DecodeString(f.ContentBase64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Um arquivo enviado está inválido.")
			return nil, false
		}
		total += len(data)
		if total > manifest.MaxInputBytes {
			writeError(w, http.StatusBadRequest, "Os arquivos podem somar no máximo 10 MB.")
			return nil, false
		}
		decoded[name] = data
	}
	// Scan every file before any filesystem or database persistence. Only a count
	// leaves this handler: filenames, variable names and source excerpts can also
	// contain sensitive values. Confirmation never changes the executable sources.
	secretCount := 0
	for name, data := range decoded {
		secretCount += len(scanner.FindSecrets(name, string(data)))
	}
	if secretCount > 0 && h.workspace != nil {
		settings, err := h.securityConfig(r.Context())
		if err != nil {
			internalError(w, "script security policy", err)
			return nil, false
		}
		if settings.BlockSecrets {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"code": "script_secrets_blocked", "error": "Possíveis senhas ou chaves no código. Retire as credenciais antes de importar; a política da empresa não permite ignorar este bloqueio.", "secret_count": secretCount})
			return nil, false
		}
	}
	if secretCount > 0 && !confirm {
		writeJSON(w, http.StatusConflict, map[string]any{
			"code":         "script_secrets_confirmation_required",
			"error":        "Possíveis senhas ou chaves no código. Revise os arquivos ou confirme o envio sem alterações.",
			"secret_count": secretCount,
		})
		return nil, false
	}
	return decoded, true
}

// recordSentVersion scans the app's stored-code folder and records it:
// "original" when the app has no version yet, else "upload"; channel "mcp"
// when ctx is Claude's. changed is false (v is the latest) when nothing
// changed. The caller holds lockApp.
func (h *Handler) recordSentVersion(ctx context.Context, a sqlc.App, by int64, note string) (sqlc.AppVersion, bool, error) {
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return sqlc.AppVersion{}, false, err
	}
	folder, err := h.serverFolder(a)
	if err != nil {
		return sqlc.AppVersion{}, false, err
	}
	latest, err := h.q.LatestAppVersion(ctx, a.ID)
	hasLatest := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AppVersion{}, false, err
	}
	var stored versions.Snapshot
	if hasLatest {
		if err := json.Unmarshal(latest.Files, &stored); err != nil {
			return latest, false, err
		}
	}
	snap, contents, err := versions.Scan(folder, m.Outputs, stored)
	if err != nil {
		return latest, false, err
	}
	if hasLatest && versions.Same(stored, snap) {
		h.storeScan(a.ID, latest.Number, snap)
		return latest, false, nil
	}
	origin := "upload"
	if !hasLatest {
		origin = "original"
	}
	v, err := h.insertVersion(ctx, a, stored, snap, contents, origin, pgtype.Int4{}, pgtype.Int8{Int64: by, Valid: true}, false, originOf(ctx), note)
	if err != nil {
		return v, false, err
	}
	h.storeScan(a.ID, v.Number, snap)
	return v, true, nil
}
