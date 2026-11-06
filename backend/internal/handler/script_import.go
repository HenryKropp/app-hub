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
	decoded, ok := h.checkSentFiles(w, r, m, b.Files, b.ConfirmSecrets)
	if !ok {
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
	}, m.Kind, files)
	if err != nil {
		internalError(w, "import script", err)
		return
	}
	success = true
	var number int32
	if a, err := h.q.GetApp(r.Context(), id); err == nil {
		unlock := h.lockApp(id)
		v, _, err := h.recordSentVersion(r.Context(), a, me.ID, "")
		unlock()
		if err != nil {
			slog.Error("import script version", "app", id, "err", err)
		}
		number = v.Number
	}
	blocked, err := h.executionBlock(r.Context(), id)
	if err != nil {
		slog.Error("import script approval", "app", id, "err", err)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "version": number, "blocked_reason": blocked})
}

func (h *Handler) importedSources(ctx context.Context, id int64) (map[string]manifest.InputFile, error) {
	raw, err := h.q.ScriptSources(ctx, id)
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
