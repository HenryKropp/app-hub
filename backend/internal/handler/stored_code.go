package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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

const noStoredCodeMsg = "Este app foi registrado a partir de uma pasta do computador, e o Ziv não guarda o código dele. Para mandar versões pelo Claude, suba o app de novo como app novo (registrar_app sem app_id)."

// fichaUpdate holds the only ficha fields a code upload may change. Name,
// area, owner and access change only on the site.
type fichaUpdate struct {
	Summary    *string            `json:"summary"`
	Inputs     *[]manifest.Input  `json:"inputs"`
	Outputs    *[]manifest.Output `json:"outputs"`
	Entrypoint *string            `json:"entrypoint"` // scripts only; ignored for web
	Runtime    *manifest.Runtime  `json:"runtime"`    // scripts only; ignored for web
}

// applyFicha returns m with f's set fields; nil fields keep m's value.
func applyFicha(m manifest.Manifest, f *fichaUpdate) manifest.Manifest {
	if f == nil {
		return m
	}
	if f.Summary != nil {
		m.Summary = strings.TrimSpace(*f.Summary)
	}
	if f.Inputs != nil {
		m.Inputs = *f.Inputs
	}
	if f.Outputs != nil {
		m.Outputs = *f.Outputs
	}
	if m.Kind != manifest.KindWeb {
		if f.Entrypoint != nil {
			m.Entrypoint = *f.Entrypoint
		}
		if f.Runtime != nil {
			m.Runtime = *f.Runtime
		}
	}
	return m
}

// sendSources stores a new version of an app whose code the Ziv keeps
// (POST /api/apps/{id}/sources), optionally with ficha changes.
func (h *Handler) sendSources(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode mudar este app.")
		return
	}
	var b struct {
		Files          map[string]manifest.InputFile `json:"files"`
		ConfirmSecrets bool                          `json:"confirm_secrets"`
		Note           string                        `json:"note"`
		Ficha          *fichaUpdate                  `json:"ficha"`
		UpdatedAt      time.Time                     `json:"updated_at"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 15<<20))
	dec.DisallowUnknownFields()
	if dec.Decode(&b) != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "Não consegui ler os arquivos. Envie até 10 MB, só com files, confirm_secrets, note, ficha e updated_at.")
		return
	}
	b.Note = strings.TrimSpace(b.Note)
	if utf8.RuneCountInString(b.Note) > 2000 {
		writeError(w, http.StatusBadRequest, "A nota da versão pode ter até 2.000 caracteres.")
		return
	}
	ctx := r.Context()
	if _, err := h.q.ScriptSources(ctx, a.ID); errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusConflict, map[string]string{"code": "no_stored_code", "error": noStoredCodeMsg})
		return
	} else if err != nil {
		internalError(w, "script sources", err)
		return
	}
	// The lock is held while the code changes and its version is recorded;
	// it is released before the ficha is saved, because writeApp checks the
	// version under the same lock (and finds nothing new by then).
	unlock := h.lockApp(a.ID)
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()
	// The app may have changed while waiting for the lock.
	a, err := h.q.GetApp(ctx, a.ID)
	if err != nil {
		internalError(w, "get app", err)
		return
	}
	if err := h.q.ExpireStaleRuns(ctx); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	if active, err := h.q.AppHasActiveRun(ctx, a.ID); err != nil {
		internalError(w, "active run", err)
		return
	} else if active {
		writeError(w, http.StatusConflict, "Espere a execução terminar antes de mandar uma versão nova.")
		return
	}
	var current manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &current); err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	m := current
	if b.Ficha != nil {
		if !b.UpdatedAt.Equal(a.UpdatedAt.Time) {
			writeError(w, http.StatusConflict, "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova.")
			return
		}
		m = applyFicha(current, b.Ficha)
		if len(m.Summary) > 2000 {
			writeError(w, http.StatusBadRequest, "Informe um nome de até 120 caracteres e uma descrição de até 2.000.")
			return
		}
		if issues := m.Validate(); len(issues) > 0 {
			writeError(w, http.StatusBadRequest, strings.Join(issues, " "))
			return
		}
	}
	if _, ok := h.checkSentFiles(w, r, m, b.Files, b.ConfirmSecrets); !ok {
		return
	}
	files, _ := json.Marshal(b.Files)
	if err := h.q.ReplaceScriptSources(ctx, a.ID, files); err != nil {
		internalError(w, "replace sources", err)
		return
	}
	if err := os.RemoveAll(a.FolderPath); err != nil {
		internalError(w, "clear folder", err)
		return
	}
	if err := h.hydrateImportedScript(ctx, a, current); err != nil {
		internalError(w, "rebuild folder", err)
		return
	}
	v, changed, err := h.recordSentVersion(ctx, a, me.ID, b.Note)
	if err != nil {
		internalError(w, "record version", err)
		return
	}
	unlock()
	locked = false
	if b.Ficha != nil {
		var backup *int64
		if a.BackupID.Valid {
			backup = &a.BackupID.Int64
		}
		saved, ok := h.writeApp(w, r, me, a.FolderPath, appBody{FolderPath: a.FolderPath, OwnerID: a.OwnerID, BackupID: backup, Manifest: m}, &a)
		if !ok {
			return
		}
		a = saved
	}
	blocked, err := h.executionBlock(ctx, a.ID)
	if err != nil {
		internalError(w, "approval", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": a.ID, "version": v.Number, "changed": changed, "blocked_reason": blocked, "updated_at": a.UpdatedAt.Time})
}
