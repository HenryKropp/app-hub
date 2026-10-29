package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) importScript(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.RegisterApps)
	if !ok {
		return
	}
	var b struct {
		Manifest       manifest.Manifest             `json:"manifest"`
		Files          map[string]manifest.InputFile `json:"files"`
		ConfirmSecrets bool                          `json:"confirm_secrets"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 15<<20))
	dec.DisallowUnknownFields()
	if dec.Decode(&b) != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "Não consegui ler os arquivos. Envie até 10 MB.")
		return
	}
	m := b.Manifest
	m.ManifestVersion, m.Kind = manifest.Version, manifest.KindScript
	m.Name, m.Summary = strings.TrimSpace(m.Name), strings.TrimSpace(m.Summary)
	m.ID = manifest.Slugify(m.Name)
	m.Owner = &manifest.PersonRef{Name: me.Name, Email: me.Email}
	m.Backup = nil
	m.Run.Command = "" // The Hub chooses the interpreter; no shell commands.
	if len(m.Name) > 120 || len(m.Summary) > 2000 || m.ID == "" {
		writeError(w, http.StatusBadRequest, "Informe um nome de até 120 caracteres e uma descrição de até 2.000.")
		return
	}
	if issues := m.Validate(); len(issues) > 0 {
		writeError(w, http.StatusBadRequest, strings.Join(issues, " "))
		return
	}
	if len(b.Files) == 0 || len(b.Files) > 100 {
		writeError(w, http.StatusBadRequest, "Envie de 1 a 100 arquivos do script.")
		return
	}
	if _, ok := b.Files[m.Entrypoint]; !ok {
		writeError(w, http.StatusBadRequest, "Inclua o arquivo que inicia o script.")
		return
	}
	if m.Runtime.DependenciesFile != "" {
		if _, ok := b.Files[m.Runtime.DependenciesFile]; !ok {
			writeError(w, http.StatusBadRequest, "Inclua o arquivo de dependências escolhido.")
			return
		}
	}
	total := 0
	seen := map[string]bool{}
	decoded := map[string][]byte{}
	for name, f := range b.Files {
		key := strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
		base := strings.ToLower(filepath.Base(name))
		secretFile := base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") || base == "id_rsa"
		if secretFile {
			writeError(w, http.StatusBadRequest, "Não envie arquivos de credenciais (.env, .pem, .key). Retire as senhas do pacote antes de importar.")
			return
		}
		if !manifest.SafeRelative(name) || manifest.IsManifestName(base) || name == manifest.FichaName || strings.HasPrefix(base, ".apphub-") || seen[key] || strings.Contains(name, `\`) {
			writeError(w, http.StatusBadRequest, "Use nomes de arquivos relativos, sem repetições ou caminhos externos.")
			return
		}
		seen[key] = true
		data, err := base64.StdEncoding.DecodeString(f.ContentBase64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Um arquivo enviado está inválido.")
			return
		}
		total += len(data)
		if total > manifest.MaxInputBytes {
			writeError(w, http.StatusBadRequest, "Os arquivos podem somar no máximo 10 MB.")
			return
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
			return
		}
		if settings.BlockSecrets {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"code": "script_secrets_blocked", "error": "Possíveis senhas ou chaves no código. Retire as credenciais antes de importar; a política da empresa não permite ignorar este bloqueio.", "secret_count": secretCount})
			return
		}
	}
	if secretCount > 0 && !b.ConfirmSecrets {
		writeJSON(w, http.StatusConflict, map[string]any{
			"code":         "script_secrets_confirmation_required",
			"error":        "Possíveis senhas ou chaves no código. Revise os arquivos ou confirme o envio sem alterações.",
			"secret_count": secretCount,
		})
		return
	}
	area, err := h.q.GetAreaByName(r.Context(), m.Area)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Escolha uma área existente para o app.")
		return
	}
	token, err := randomToken()
	if err != nil {
		internalError(w, "script folder", err)
		return
	}
	folder := filepath.Join(os.TempDir(), "apphub-imports", token)
	// Files are persisted in PostgreSQL; this copy is for existing versioning.
	if err := os.MkdirAll(folder, 0o700); err != nil {
		internalError(w, "script folder", err)
		return
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(folder)
		}
	}()
	for name, data := range decoded {
		target := filepath.Join(folder, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			internalError(w, "script files", err)
			return
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			internalError(w, "script files", err)
			return
		}
	}
	m.Language = manifest.ScriptRuntime(m.Entrypoint)
	m.ID += "-" + token[:8]
	if err := manifest.WriteToFolder(folder, m); err != nil {
		internalError(w, "script manifest", err)
		return
	}
	raw, _ := json.Marshal(m)
	files, _ := json.Marshal(b.Files)
	id, err := h.q.ImportScript(r.Context(), sqlc.UpsertAppParams{
		Slug: m.ID, Name: m.Name, Summary: m.Summary, Language: m.Language, Area: m.Area, AreaID: area.ID,
		FolderPath: folder, OwnerID: me.ID, Manifest: raw,
	}, files)
	if err != nil {
		internalError(w, "import script", err)
		return
	}
	success = true
	if a, err := h.q.GetApp(r.Context(), id); err == nil {
		if _, _, err := h.checkVersion(r.Context(), a); err != nil {
			slog.Error("import script version", "app", id, "err", err)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *Handler) importedSources(r *http.Request, id int64) (map[string]manifest.InputFile, error) {
	raw, err := h.q.ScriptSources(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files map[string]manifest.InputFile
	err = json.Unmarshal(raw, &files)
	return files, err
}

// Rebuild the disposable versioning copy after a host restart. The database,
// not this folder, holds browser-imported sources.
func (h *Handler) hydrateImportedScript(ctx context.Context, a sqlc.App, m manifest.Manifest) error {
	if _, err := os.Stat(a.FolderPath); err == nil {
		return nil
	}
	raw, err := h.q.ScriptSources(ctx, a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var files map[string]manifest.InputFile
	if err := json.Unmarshal(raw, &files); err != nil {
		return err
	}
	for name, f := range files {
		if !manifest.SafeRelative(name) {
			return errors.New("invalid stored script path")
		}
		data, err := base64.StdEncoding.DecodeString(f.ContentBase64)
		if err != nil {
			return err
		}
		target := filepath.Join(a.FolderPath, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return err
		}
	}
	return manifest.WriteToFolder(a.FolderPath, m)
}
