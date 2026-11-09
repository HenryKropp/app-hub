package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Arquivar app (ADR 031). An archived app leaves the catalog, the Home, the
// favorites and Claude's lists, cannot run, has its schedules paused and its
// server version stopped. Whoever could archive it can bring it back for 30
// days; after that the cleanup deletes it for good. The app's folder is never
// touched.
const (
	archiveKeep        = 30 * 24 * time.Hour
	archiveCleanupEach = time.Hour

	msgArchived        = "Este app foi arquivado. Quem cuida dele pode trazê-lo de volta em Apps › Arquivados."
	msgArchivedRun     = "Este app foi arquivado e não roda mais. Quem cuida dele pode trazê-lo de volta em Apps › Arquivados."
	msgArchivedFolder  = "O app desta pasta está arquivado. Traga-o de volta em Apps › Arquivados em vez de registrar de novo."
	schedPausedArchive = "O app foi arquivado"
)

// canArchive: archiving and bringing back belong to who controls the app's
// access (owner, backup, area leader, or whoever manages the areas).
func canArchive(c access.Can) bool { return c.View && c.Access }

// archiveTarget loads the app in the URL, archived or not, for someone who may
// archive it. Anyone else gets the same 404 a hidden app gives.
func (h *Handler) archiveTarget(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return sqlc.App{}, me, false
	}
	a, ok := h.appFromPath(w, r)
	if !ok {
		return sqlc.App{}, me, false
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return sqlc.App{}, me, false
	}
	can := acc.For(appRef(a))
	if !can.View {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return sqlc.App{}, me, false
	}
	if !canArchive(can) {
		writeError(w, http.StatusForbidden, "Só o responsável pelo app ou um administrador pode arquivar ou trazer de volta.")
		return sqlc.App{}, me, false
	}
	return a, me, true
}

// archiveApp is "Arquivar": schedules paused, server version stopped (its
// data kept), then the app leaves every list.
func (h *Handler) archiveApp(w http.ResponseWriter, r *http.Request) {
	a, me, ok := h.archiveTarget(w, r)
	if !ok {
		return
	}
	if a.ArchivedAt.Valid {
		writeError(w, http.StatusConflict, "Este app já está arquivado.")
		return
	}
	unlock := h.lockApp(a.ID)
	defer unlock()
	if err := h.stopSharedForArchive(r.Context(), a.ID, me.ID); err != nil {
		sharedError(w, err)
		return
	}
	if _, err := h.q.PauseAppSchedules(r.Context(), sqlc.PauseAppSchedulesParams{AppID: a.ID, PausedReason: schedPausedArchive}); err != nil {
		internalError(w, "pause schedules", err)
		return
	}
	archived, err := h.q.ArchiveApp(r.Context(), sqlc.ArchiveAppParams{ID: a.ID, ArchivedBy: pgtype.Int8{Int64: me.ID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "Este app já está arquivado.")
		return
	}
	if err != nil {
		internalError(w, "archive app", err)
		return
	}
	h.archiveEvent(r.Context(), a.ID, me.ID, "Arquivou o app")
	writeJSON(w, http.StatusOK, archivedJSON(archived.ID, archived.Name, archived.ArchivedAt, me.Name, time.Now()))
}

// stopSharedForArchive stops the app's server version, if it has one running
// or sleeping, the same way "Parar" does: container off, image, database and
// copies kept, so "Ligar" brings it back. Open tests end.
func (h *Handler) stopSharedForArchive(ctx context.Context, appID, by int64) error {
	if h.shared.Svc == nil {
		return nil
	}
	sa, err := h.q.GetSharedApp(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if sa.UnsharedAt.Valid {
		return nil
	}
	if tests, err := h.q.ListOpenTests(ctx, appID); err == nil {
		for _, d := range tests {
			if err := h.shared.Svc.EndTest(ctx, d.ID); err != nil {
				slog.Warn("archive: end test", "app", appID, "deployment", d.ID, "err", err)
			}
		}
	}
	if !sa.CurrentDeploymentID.Valid || sa.Status == "stopped" {
		return nil
	}
	return h.shared.Svc.Stop(ctx, appID, by)
}

// restoreArchivedApp is "Trazer de volta". Schedules and the server version
// stay off: the person turns them on again if needed.
func (h *Handler) restoreArchivedApp(w http.ResponseWriter, r *http.Request) {
	a, me, ok := h.archiveTarget(w, r)
	if !ok {
		return
	}
	if !a.ArchivedAt.Valid {
		writeError(w, http.StatusConflict, "Este app não está arquivado.")
		return
	}
	unlock := h.lockApp(a.ID)
	defer unlock()
	back, err := h.q.UnarchiveApp(r.Context(), a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "Este app não está arquivado.")
		return
	}
	if err != nil {
		internalError(w, "restore archived app", err)
		return
	}
	h.archiveEvent(r.Context(), a.ID, me.ID, "Trouxe o app de volta do arquivo")
	writeJSON(w, http.StatusOK, map[string]any{"id": back.ID, "name": back.Name})
}

func (h *Handler) archiveEvent(ctx context.Context, appID, by int64, label string) {
	raw, _ := json.Marshal([]manifest.Change{{Field: "archive", Label: label}})
	if err := h.q.InsertAppChange(ctx, sqlc.InsertAppChangeParams{AppID: appID, PersonID: pgtype.Int8{Int64: by, Valid: true}, Changes: raw}); err != nil {
		slog.Warn("archive event", "app", appID, "err", err)
	}
}

type archivedAppJSON struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Summary        string    `json:"summary"`
	Kind           string    `json:"kind"`
	Area           string    `json:"area"`
	OwnerName      string    `json:"owner_name"`
	ArchivedAt     time.Time `json:"archived_at"`
	ArchivedByName string    `json:"archived_by_name"`
	PurgeAt        time.Time `json:"purge_at"`
	DaysLeft       int       `json:"days_left"`
}

func archivedJSON(id int64, name string, at pgtype.Timestamptz, by string, now time.Time) archivedAppJSON {
	purge := at.Time.Add(archiveKeep)
	return archivedAppJSON{ID: id, Name: name, ArchivedAt: at.Time, ArchivedByName: by, PurgeAt: purge, DaysLeft: daysLeft(purge, now)}
}

// daysLeft rounds up: "1 dia" until the very last hour, never below 0.
func daysLeft(purge, now time.Time) int {
	d := purge.Sub(now)
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Hours() / 24))
}

// listArchivedApps is the "Arquivados" list: only the archived apps the
// person could bring back.
func (h *Handler) listArchivedApps(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	rows, err := h.q.ListArchivedApps(r.Context())
	if err != nil {
		internalError(w, "list archived apps", err)
		return
	}
	now := time.Now()
	out := []archivedAppJSON{}
	for _, a := range rows {
		ref := access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted}
		if !canArchive(acc.For(ref)) {
			continue
		}
		j := archivedJSON(a.ID, a.Name, a.ArchivedAt, a.ArchivedByName, now)
		j.Summary, j.Kind, j.Area, j.OwnerName = a.Summary, a.Kind, a.Area, a.OwnerName
		out = append(out, j)
	}
	writeJSON(w, http.StatusOK, out)
}

// isArchived tells whether the app is archived; a missing app is not.
func (h *Handler) isArchived(ctx context.Context, appID int64) (bool, error) {
	archived, err := h.q.AppArchived(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return archived, err
}

// StartArchiveCleanup deletes, once an hour, the apps archived more than 30
// days ago, until ctx ends.
func (h *Handler) StartArchiveCleanup(ctx context.Context) {
	go func() {
		t := time.NewTicker(archiveCleanupEach)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				h.purgeArchived(ctx, now)
			}
		}
	}()
}

// purgeArchived deletes for good the apps archived before now - 30 days:
// first what the server kept for its screen version (as "Deixar de
// compartilhar" and its cleanup do), then the stars, then the app and all
// that hangs on it. The app's folder is never touched. It returns how many
// apps went away.
func (h *Handler) purgeArchived(ctx context.Context, now time.Time) int {
	cutoff := pgtype.Timestamptz{Time: now.Add(-archiveKeep), Valid: true}
	ids, err := h.q.ListArchivedBefore(ctx, cutoff)
	if err != nil {
		slog.Error("archive cleanup: list", "err", err)
		return 0
	}
	n := 0
	for _, id := range ids {
		if err := h.purgeOne(ctx, id, cutoff); err != nil {
			slog.Error("archive cleanup", "app", id, "err", err)
			continue
		}
		n++
	}
	return n
}

func (h *Handler) purgeOne(ctx context.Context, id int64, cutoff pgtype.Timestamptz) error {
	unlock := h.lockApp(id)
	defer unlock()
	a, err := h.q.GetApp(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Brought back (or archived again) since the list was read: not this time.
	if !a.ArchivedAt.Valid || !a.ArchivedAt.Time.Before(cutoff.Time) {
		return nil
	}
	if err := h.purgeShared(ctx, a); err != nil {
		return err
	}
	if err := h.q.DeleteItemFavorites(ctx, sqlc.DeleteItemFavoritesParams{Kind: "app", ItemID: id}); err != nil {
		return err
	}
	// Same audit trail as a request: no person, the cleanup as the route.
	eventID, err := h.auditStart(ctx, nil, "archive.purge.started")
	if err != nil {
		return err
	}
	gone, err := h.q.PurgeArchivedApp(ctx, sqlc.PurgeArchivedAppParams{ID: id, ArchivedAt: cutoff})
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
	}
	h.auditFinish(ctx, eventID, "archive.purge.completed", "archive cleanup (30 dias)", status)
	if err != nil {
		return err
	}
	if gone > 0 {
		slog.Info("archive cleanup: app deleted for good", "app", id, "name", a.Name)
	}
	return nil
}

// purgeShared removes the containers, images, network, database, copies and
// data of the app's screen version: Unshare then Purge, the same path
// "Deixar de compartilhar" takes 30 days later.
func (h *Handler) purgeShared(ctx context.Context, a sqlc.App) error {
	sa, err := h.q.GetSharedApp(ctx, a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if h.shared.Svc == nil {
		return errors.New("shared apps are off: cannot clean the screen version yet")
	}
	if !sa.UnsharedAt.Valid {
		by := a.OwnerID
		if a.ArchivedBy.Valid {
			by = a.ArchivedBy.Int64
		}
		if err := h.shared.Svc.Unshare(ctx, a.ID, by); err != nil {
			return err
		}
	}
	return h.shared.Svc.Purge(ctx, a.ID)
}
