package handler

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// errFolderNotOnServer: outside the main company, the Ziv reads and writes on
// its own disk only the folders it created for that company (imported scripts
// and the screen apps' code). Any other path could be a server folder (ziv.env
// with HUB_SECRET_KEY, another company's data), so it is refused (ADR 025).
var errFolderNotOnServer = errors.New("O código deste app não está guardado no Ziv, então o Ziv não abre a pasta dele no servidor.")

// importToken is the folder name script_import.go creates: a 256-bit random
// token, never shown to another company.
var importToken = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func (h *Handler) principal() bool {
	return h.workspace == nil || h.workspace.Slug == "principal"
}

// serverPath checks, outside principal, that path is a folder the Ziv made
// for this company. Symlinks are resolved before the check.
func (h *Handler) serverPath(path string, appID int64) error {
	if h.principal() {
		return nil
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(real) {
		return errFolderNotOnServer
	}
	if imports, err := filepath.EvalSymlinks(filepath.Join(os.TempDir(), "apphub-imports")); err == nil &&
		filepath.Dir(real) == imports && importToken.MatchString(filepath.Base(real)) {
		return nil
	}
	if h.shared.Svc != nil && appID > 0 {
		if code, err := filepath.EvalSymlinks(h.shared.Svc.CodeDir(appID)); err == nil &&
			(real == code || strings.HasPrefix(real, code+string(filepath.Separator))) {
			return nil
		}
	}
	return errFolderNotOnServer
}

// serverFolder is the app's folder on the server disk, when the company may
// open it.
func (h *Handler) serverFolder(a sqlc.App) (versions.Folder, error) {
	if err := h.serverPath(a.FolderPath, a.ID); err != nil {
		return nil, err
	}
	return versions.LocalFolder{Root: a.FolderPath}, nil
}

func writeNotOnServer(w http.ResponseWriter) {
	writeJSON(w, http.StatusConflict, map[string]string{"error": errFolderNotOnServer.Error(), "code": "folder_not_on_server"})
}
