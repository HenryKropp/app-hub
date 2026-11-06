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
	"strconv"
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
	switch m.Kind {
	case "":
		m.Kind = manifest.KindScript
	case manifest.KindScript, manifest.KindWeb:
	default:
		writeError(w, http.StatusBadRequest, "Escolha script ou app com tela.")
		return
	}
	web := m.Kind == manifest.KindWeb
	if web && !h.sharedOn(r.Context()) { // before any write
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "shared_runtime_unavailable", "error": msgWebOff})
		return
	}
	m.ManifestVersion = manifest.Version
	if web {
		// Only the variable names come from the sender; secret defaults are
		// dropped, and the command and resources are confirmed on the
		// Compartilhar screen (PUT /shared, can.Restore).
		env := []manifest.EnvVar{}
		if m.Shared != nil {
			for _, e := range m.Shared.Env {
				if e.Secret {
					e.Default = ""
				}
				env = append(env, e)
			}
		}
		m.Shared = &manifest.Shared{Enabled: true, Env: env}
	} else {
		m.Shared = nil
	}
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
	if web {
		slug, err := h.freeSlug(r.Context(), m.ID)
		if err != nil {
			internalError(w, "web slug", err)
			return
		}
		m.ID = slug
	} else {
		m.ID += "-" + token[:8]
	}
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
	if web {
		// Screen apps keep their code in the shared-app code folder, which
		// the deploy service accepts in every company.
		if err := h.moveToCodeDir(r.Context(), id, folder, m); err != nil {
			_, _ = h.q.DeleteApp(r.Context(), id)
			internalError(w, "web code folder", err)
			return
		}
	}
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

const msgWebOff = "Os apps com tela ainda não estão ligados nesta empresa. Peça ao responsável pelo Ziv para ligar. Scripts sobem normalmente."

// sharedOn says whether this company can run screen apps now.
func (h *Handler) sharedOn(ctx context.Context) bool {
	if h.workspace != nil && h.workspace.SharedEnabled != nil && !h.workspace.SharedEnabled(ctx) {
		return false
	}
	return h.shared.Svc != nil
}

// freeSlug is base cut to 40 characters (an app address), with -2, -3… when taken.
func (h *Handler) freeSlug(ctx context.Context, base string) (string, error) {
	cut := func(s string, n int) string {
		if len(s) > n {
			s = s[:n]
		}
		return strings.TrimRight(s, "-")
	}
	for i := 1; ; i++ {
		slug := cut(base, 40)
		if i > 1 {
			suffix := "-" + strconv.Itoa(i)
			slug = cut(base, 40-len(suffix)) + suffix
		}
		taken, err := h.q.SlugExists(ctx, sqlc.SlugExistsParams{Slug: slug})
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
	}
}

// moveToCodeDir points a just-imported screen app at its code folder and
// rebuilds the files there from the stored code.
func (h *Handler) moveToCodeDir(ctx context.Context, id int64, temp string, m manifest.Manifest) error {
	dir := h.shared.Svc.CodeDir(id)
	if err := h.q.SetAppFolder(ctx, id, dir); err != nil {
		return err
	}
	_ = os.RemoveAll(temp)
	a, err := h.q.GetApp(ctx, id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}
	return h.hydrateImportedScript(ctx, a, m)
}
