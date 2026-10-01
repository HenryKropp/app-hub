package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
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
