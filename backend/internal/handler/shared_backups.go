package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
)

const backupNotFound = "Cópia não encontrada. Recarregue a lista de cópias."

// backupOut is a copy as the API shows it (never the path on the server).
type backupOut struct {
	ID            int64              `json:"id"`
	Kind          string             `json:"kind"`
	SizeBytes     int64              `json:"size_bytes"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
	ExpiresAt     pgtype.Timestamptz `json:"expires_at"`
	CreatedByName string             `json:"created_by_name"`
	VersionNumber *int32             `json:"version_number"`
}

func (h *Handler) listSharedBackups(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	rows, err := h.q.ListBackups(r.Context(), a.ID)
	if err != nil {
		internalError(w, "list backups", err)
		return
	}
	out := make([]backupOut, 0, len(rows))
	for _, b := range rows {
		o := backupOut{ID: b.ID, Kind: b.Kind, SizeBytes: b.SizeBytes, CreatedAt: b.CreatedAt, ExpiresAt: b.ExpiresAt,
			CreatedByName: b.CreatedByName.String}
		if b.VersionNumber.Valid {
			n := b.VersionNumber.Int32
			o.VersionNumber = &n
		}
		out = append(out, o)
	}
	writeJSON(w, http.StatusOK, out)
}

// createSharedBackup is "Fazer cópia agora".
func (h *Handler) createSharedBackup(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.sharedRow(w, r, canRestore)
	if !ok {
		return
	}
	if h.shared.Svc.Busy(a.ID) {
		sharedError(w, deploy.ErrBusy)
		return
	}
	b, err := h.shared.Svc.Backup(r.Context(), a.ID, "manual", nil, &me.ID)
	if errors.Is(err, deploy.ErrBackupFailed) { // the service logged the technical reason
		writeError(w, http.StatusInternalServerError, deploy.ErrBackupFailed.Error())
		return
	}
	if err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, backupOut{ID: b.ID, Kind: b.Kind, SizeBytes: b.SizeBytes, CreatedAt: b.CreatedAt,
		ExpiresAt: b.ExpiresAt, CreatedByName: me.Name})
}

func backupID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("bid"), 10, 64)
	return id, err == nil && id > 0
}

func (h *Handler) restoreSharedBackup(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.sharedRow(w, r, canRestore)
	if !ok {
		return
	}
	bid, ok := backupID(r)
	if !ok {
		writeError(w, http.StatusNotFound, backupNotFound)
		return
	}
	if _, err := h.q.GetBackup(r.Context(), sqlc.GetBackupParams{ID: bid, AppID: a.ID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, backupNotFound)
			return
		}
		internalError(w, "get backup", err)
		return
	}
	if err := h.shared.Svc.Restore(r.Context(), a.ID, bid, me.ID); err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// downloadSharedBackup serves a finished copy of this app as
// "<slug>-<data>.dump". Only files inside the Hub's copies folder of this app.
func (h *Handler) downloadSharedBackup(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	bid, ok := backupID(r)
	if !ok {
		writeError(w, http.StatusNotFound, backupNotFound)
		return
	}
	b, err := h.q.GetBackup(r.Context(), sqlc.GetBackupParams{ID: bid, AppID: a.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, backupNotFound)
		return
	}
	if err != nil {
		internalError(w, "get backup", err)
		return
	}
	// The file must be a finished copy inside this app's copies folder (the
	// partial file of a copy in progress starts with ".partial-").
	dir := filepath.Join(filepath.Clean(h.shared.DataDir), "backups", strconv.FormatInt(a.ID, 10)) + string(filepath.Separator)
	path := filepath.Clean(b.Path)
	if h.shared.DataDir == "" || !strings.HasPrefix(path, dir) || strings.ContainsRune(path[len(dir):], filepath.Separator) ||
		strings.HasPrefix(filepath.Base(path), ".") || filepath.Ext(path) != ".dump" {
		slog.Warn("shared backup download: path outside copies folder", "app", a.ID, "backup", b.ID)
		writeError(w, http.StatusNotFound, backupNotFound)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		slog.Warn("shared backup download", "app", a.ID, "backup", b.ID, "err", err)
		writeError(w, http.StatusNotFound, "O arquivo da cópia não foi encontrado no servidor. Faça uma cópia nova.")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, backupNotFound)
		return
	}
	name := safeFileSlug(a.Slug) + "-" + b.CreatedAt.Time.Local().Format("2006-01-02-1504") + ".dump"
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, st.ModTime(), f)
}

// safeFileSlug keeps only [a-z0-9-] so the name is safe in Content-Disposition.
func safeFileSlug(s string) string {
	out := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return -1
	}, strings.ToLower(s))
	if out == "" {
		return "app"
	}
	return out
}
