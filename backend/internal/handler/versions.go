package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

type versionRef struct {
	Number    int32     `json:"number"`
	CreatedAt time.Time `json:"created_at"`
}

// scanCache is the last successful scan of an app and the version number that
// was latest at that time.
type scanCache struct {
	number int32
	snap   versions.Snapshot
}

func folderOf(a sqlc.App) versions.Folder { return versions.LocalFolder{Root: a.FolderPath} }

func (h *Handler) lockApp(id int64) func() {
	h.locksMu.Lock()
	m, ok := h.locks[id]
	if !ok {
		m = &sync.Mutex{}
		h.locks[id] = m
	}
	h.locksMu.Unlock()
	m.Lock()
	return m.Unlock
}

func (h *Handler) cachedScan(id int64) (scanCache, bool) {
	h.scansMu.Lock()
	defer h.scansMu.Unlock()
	c, ok := h.scans[id]
	return c, ok
}

func (h *Handler) storeScan(id int64, number int32, snap versions.Snapshot) {
	h.scansMu.Lock()
	h.scans[id] = scanCache{number: number, snap: snap}
	h.scansMu.Unlock()
}

// checkVersion compares the app folder with its latest version and records a
// new version when the code changed. readable is false when the folder could
// not be read; then nothing is recorded and v is the latest version (or zero).
func (h *Handler) checkVersion(ctx context.Context, a sqlc.App) (sqlc.AppVersion, bool, error) {
	defer h.lockApp(a.ID)()
	v, _, readable, err := h.checkVersionLocked(ctx, a)
	return v, readable, err
}

// checkVersionLocked is checkVersion for callers that already hold the app lock.
func (h *Handler) checkVersionLocked(ctx context.Context, a sqlc.App) (sqlc.AppVersion, versions.Snapshot, bool, error) {
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return sqlc.AppVersion{}, nil, false, err
	}
	latest, err := h.q.LatestAppVersion(ctx, a.ID)
	hasLatest := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AppVersion{}, nil, false, err
	}
	var stored versions.Snapshot
	if hasLatest {
		if err := json.Unmarshal(latest.Files, &stored); err != nil {
			return latest, nil, false, err
		}
	}
	// The cache only speeds up the scan (fresher mtimes); versions are always
	// compared with the stored snapshot.
	trusted := stored
	if c, ok := h.cachedScan(a.ID); ok && hasLatest && c.number == latest.Number {
		trusted = c.snap
	}
	snap, contents, err := versions.Scan(folderOf(a), m.Outputs, trusted)
	if err != nil {
		slog.Warn("version check: folder not readable", "app", a.ID, "err", err)
		return latest, stored, false, nil
	}
	if hasLatest && versions.Same(stored, snap) {
		h.storeScan(a.ID, latest.Number, snap)
		return latest, snap, true, nil
	}
	origin := "folder"
	if !hasLatest {
		origin = "original"
	}
	v, err := h.insertVersion(ctx, a, stored, snap, contents, origin, pgtype.Int4{}, pgtype.Int8{Int64: a.OwnerID, Valid: true}, true)
	if err != nil {
		return v, snap, true, err
	}
	h.storeScan(a.ID, v.Number, snap)
	return v, snap, true, nil
}

func (h *Handler) insertVersion(ctx context.Context, a sqlc.App, prev, snap versions.Snapshot, contents map[string][]byte,
	origin string, restoredFrom pgtype.Int4, author pgtype.Int8, probable bool) (sqlc.AppVersion, error) {
	for sha, data := range contents {
		if err := h.q.PutCodeBlob(ctx, sqlc.PutCodeBlobParams{Sha256: sha, Content: data, Size: int64(len(data))}); err != nil {
			return sqlc.AppVersion{}, err
		}
	}
	changes := versions.Changes(prev, snap)
	files, err := json.Marshal(snap)
	if err != nil {
		return sqlc.AppVersion{}, err
	}
	summary, err := json.Marshal(versions.Summarize(changes))
	if err != nil {
		return sqlc.AppVersion{}, err
	}
	changedAt := pgtype.Timestamptz{}
	if t, ok := versions.LastModified(changes); ok {
		changedAt = pgtype.Timestamptz{Time: t, Valid: true}
	}
	return h.q.InsertAppVersion(ctx, sqlc.InsertAppVersionParams{
		AppID: a.ID, Origin: origin, RestoredFrom: restoredFrom, AuthorID: author, AuthorProbable: probable,
		Files: files, Summary: summary, FilesChangedAt: changedAt,
	})
}

type fileChangeJSON struct {
	Path    string          `json:"path"`
	Status  string          `json:"status"`
	Kind    string          `json:"kind"`
	OldSize *int64          `json:"old_size"`
	NewSize *int64          `json:"new_size"`
	Hunks   []versions.Hunk `json:"hunks"`
}

type versionFileJSON struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Kind     string    `json:"kind"`
	Modified time.Time `json:"modified"`
}

func (h *Handler) versionFromPath(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.GetAppVersionRow, versions.Snapshot, bool) {
	a, ok := h.appFromPath(w, r)
	if !ok {
		return a, sqlc.GetAppVersionRow{}, nil, false
	}
	n, err := strconv.ParseInt(r.PathValue("n"), 10, 32)
	if err != nil {
		writeError(w, http.StatusNotFound, "Versão não encontrada.")
		return a, sqlc.GetAppVersionRow{}, nil, false
	}
	v, err := h.q.GetAppVersion(r.Context(), sqlc.GetAppVersionParams{AppID: a.ID, Number: int32(n)})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Versão não encontrada.")
		return a, v, nil, false
	}
	if err != nil {
		internalError(w, "get version", err)
		return a, v, nil, false
	}
	var snap versions.Snapshot
	if err := json.Unmarshal(v.Files, &snap); err != nil {
		internalError(w, "decode version files", err)
		return a, v, nil, false
	}
	return a, v, snap, true
}

func (h *Handler) versionItemOf(r *http.Request, a sqlc.App, v sqlc.GetAppVersionRow) versionItem {
	var current int32
	if latest, err := h.q.LatestAppVersion(r.Context(), a.ID); err == nil {
		current = latest.Number
	}
	return newVersionItem(v.Number, v.Origin, v.RestoredFrom, v.AuthorName, v.AuthorProbable, v.Description, v.Summary, v.FilesChangedAt, v.CreatedAt, current)
}

func (h *Handler) getVersion(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	a, v, _, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.versionItemOf(r, a, v))
}

// codeText returns the masked text of a code file, or "" for binaries/absent files.
func (h *Handler) codeText(r *http.Request, f *versions.File) (string, error) {
	if f == nil || f.Kind != versions.KindCode {
		return "", nil
	}
	data, err := h.q.GetCodeBlob(r.Context(), f.SHA256)
	if err != nil {
		return "", err
	}
	return scanner.MaskSecrets(string(data)), nil
}

func (h *Handler) versionChanges(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	a, v, snap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	var prev versions.Snapshot
	if v.Number > 1 {
		pv, err := h.q.GetAppVersion(r.Context(), sqlc.GetAppVersionParams{AppID: a.ID, Number: v.Number - 1})
		if err != nil {
			internalError(w, "get previous version", err)
			return
		}
		if err := json.Unmarshal(pv.Files, &prev); err != nil {
			internalError(w, "decode previous files", err)
			return
		}
	}
	files := []fileChangeJSON{}
	for _, c := range versions.Changes(prev, snap) {
		fc := fileChangeJSON{Path: c.Path, Status: c.Status, Hunks: []versions.Hunk{}}
		if c.Old != nil {
			fc.OldSize, fc.Kind = &c.Old.Size, c.Old.Kind
		}
		if c.New != nil {
			fc.NewSize, fc.Kind = &c.New.Size, c.New.Kind
		}
		before, err := h.codeText(r, c.Old)
		if err != nil {
			internalError(w, "get blob", err)
			return
		}
		after, err := h.codeText(r, c.New)
		if err != nil {
			internalError(w, "get blob", err)
			return
		}
		if fc.Kind == versions.KindCode {
			fc.Hunks = versions.DiffLines(before, after, 3)
		}
		files = append(files, fc)
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": h.versionItemOf(r, a, v), "files": files})
}

func (h *Handler) versionFiles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	_, _, snap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	out := make([]versionFileJSON, 0, len(snap))
	for _, f := range snap {
		out = append(out, versionFileJSON{Path: f.Path, Size: f.Size, Kind: f.Kind, Modified: f.Modified})
	}
	writeJSON(w, http.StatusOK, out)
}

// versionContent only serves paths listed in the version, from the stored
// copy; it never reads the disk.
func (h *Handler) versionContent(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	_, _, snap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	rel := r.URL.Query().Get("path")
	for i := range snap {
		if snap[i].Path != rel {
			continue
		}
		if snap[i].Kind != versions.KindCode {
			writeError(w, http.StatusBadRequest, "Este arquivo não é texto. O Hub guarda só o tamanho e a data dele.")
			return
		}
		text, err := h.codeText(r, &snap[i])
		if err != nil {
			internalError(w, "get blob", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"path": rel, "text": text})
		return
	}
	writeError(w, http.StatusNotFound, "Arquivo não encontrado nesta versão.")
}

func (h *Handler) describeVersion(w http.ResponseWriter, r *http.Request) {
	me, ok := h.requireRole(w, r, "admin", "criador")
	if !ok {
		return
	}
	var body struct {
		Description string `json:"description"`
		AuthorID    *int64 `json:"author_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	desc := strings.TrimSpace(body.Description)
	if len([]rune(desc)) > 2000 {
		writeError(w, http.StatusBadRequest, "A descrição pode ter até 2000 caracteres.")
		return
	}
	a, v, _, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	author := pgtype.Int8{}
	if body.AuthorID != nil && *body.AuthorID != 0 {
		if _, err := h.q.GetPerson(r.Context(), *body.AuthorID); err != nil {
			writeError(w, http.StatusBadRequest, "Pessoa não encontrada.")
			return
		}
		author = pgtype.Int8{Int64: *body.AuthorID, Valid: true}
	}
	if err := h.q.DescribeAppVersion(r.Context(), sqlc.DescribeAppVersionParams{
		Description: desc, AuthorID: author, DescribedBy: pgtype.Int8{Int64: me.ID, Valid: true}, AppID: a.ID, Number: v.Number,
	}); err != nil {
		internalError(w, "describe version", err)
		return
	}
	v, err := h.q.GetAppVersion(r.Context(), sqlc.GetAppVersionParams{AppID: a.ID, Number: v.Number})
	if err != nil {
		internalError(w, "get version", err)
		return
	}
	writeJSON(w, http.StatusOK, h.versionItemOf(r, a, v))
}
