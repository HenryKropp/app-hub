package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

type restoreJSON struct {
	Number int32              `json:"number"`
	Done   []string           `json:"done"`
	Failed []versions.Failure `json:"failed"`
	Error  string             `json:"error,omitempty"`
}

// prepareRestore runs the checks shared by preview and restore, with the app
// lock held by the caller: role, no active run, folder readable, not the
// current version. It records the current state as a version (safety copy).
func (h *Handler) prepareRestore(w http.ResponseWriter, r *http.Request, a sqlc.App, target sqlc.GetAppVersionRow) (sqlc.AppVersion, versions.Snapshot, bool) {
	ctx := r.Context()
	if err := h.q.ExpireStaleRuns(ctx); err != nil {
		internalError(w, "expire runs", err)
		return sqlc.AppVersion{}, nil, false
	}
	active, err := h.q.AppHasActiveRun(ctx, a.ID)
	if err != nil {
		internalError(w, "active run", err)
		return sqlc.AppVersion{}, nil, false
	}
	if active {
		writeError(w, http.StatusConflict, "Espere a execução terminar antes de voltar o código.")
		return sqlc.AppVersion{}, nil, false
	}
	cur, snap, readable, err := h.checkVersionLocked(ctx, a)
	if err != nil {
		internalError(w, "version check", err)
		return cur, nil, false
	}
	if !readable {
		writeError(w, http.StatusConflict, "Não consegui ler a pasta do app. Confira se ela existe e se o OneDrive está sincronizado.")
		return cur, nil, false
	}
	if cur.Number == target.Number {
		writeError(w, http.StatusBadRequest, "Esta já é a versão atual.")
		return cur, nil, false
	}
	return cur, snap, true
}

// planRestore looks at the folder too, so files with no copy in the Hub are
// listed for the trash before anything is written over them.
func planRestore(w http.ResponseWriter, a sqlc.App, snap, targetSnap versions.Snapshot) (versions.RestorePlan, bool) {
	plan, err := versions.PlanRestoreIn(folderOf(a), snap, targetSnap)
	if err != nil {
		slog.Error("plan restore", "app", a.ID, "err", err)
		writeError(w, http.StatusConflict, "Não consegui ler a pasta do app. Confira se ela existe e se o OneDrive está sincronizado.")
		return plan, false
	}
	return plan, true
}

func (h *Handler) restorePreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireRole(w, r, "admin", "criador"); !ok {
		return
	}
	a, target, targetSnap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	defer h.lockApp(a.ID)()
	cur, snap, ok := h.prepareRestore(w, r, a, target)
	if !ok {
		return
	}
	plan, ok := planRestore(w, a, snap, targetSnap)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "current": cur.Number})
}

func (h *Handler) restoreVersion(w http.ResponseWriter, r *http.Request) {
	me, ok := h.requireRole(w, r, "admin", "criador")
	if !ok {
		return
	}
	a, target, targetSnap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	defer h.lockApp(a.ID)()
	_, snap, ok := h.prepareRestore(w, r, a, target)
	if !ok {
		return
	}
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	// From here files change on disk: finish and record even if the client leaves.
	ctx := context.WithoutCancel(r.Context())
	plan, ok := planRestore(w, a, snap, targetSnap)
	if !ok {
		return
	}
	blob := func(sha string) ([]byte, error) { return h.q.GetCodeBlob(ctx, sha) }
	folder := folderOf(a)
	done, failed := versions.ApplyRestore(folder, plan, targetSnap, blob, versions.TrashFolder(time.Now()))
	if done == nil {
		done = []string{}
	}
	if failed == nil {
		failed = []versions.Failure{}
	}

	after, contents, err := versions.Scan(folder, m.Outputs, snap)
	if err != nil {
		slog.Error("scan after restore", "app", a.ID, "err", err)
		writeJSON(w, http.StatusConflict, restoreJSON{Done: done, Failed: failed,
			Error: "Voltei parte do código, mas não consegui conferir a pasta depois. Abra a ficha de novo para registrar como ela ficou."})
		return
	}
	v, err := h.insertVersion(ctx, a, snap, after, contents, "restore",
		pgtype.Int4{Int32: target.Number, Valid: true}, pgtype.Int8{Int64: me.ID, Valid: true}, false)
	if err != nil {
		internalError(w, "insert restore version", err)
		return
	}
	h.storeScan(a.ID, v.Number, after) // new number bypasses the old cache entry; keep it fresh anyway
	writeJSON(w, http.StatusOK, restoreJSON{Number: v.Number, Done: done, Failed: failed})
}
