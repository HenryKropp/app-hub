package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// Run starts the live version (new container from the kept image), waits for
// health and marks the app running. Used to wake a sleeping app, by
// "Ligar"/"Religar" and by the supervisor to restart a crashed one. It
// returns ErrBusy while another operation (or another Run) holds the app,
// and ErrStopped when someone pressed "Parar" (only Start undoes that).
func (s *Service) Run(ctx context.Context, appID int64) error {
	return s.runLocked(ctx, appID, false)
}

// RunIf is Run for the supervisor's restarts. Under the app's lock it re-reads
// the app and hands it to still, which says whether the restart still makes
// sense (a publish, swap, rollback, sleep or stop may have landed since the
// caller looked). An error from still skips the start and is returned; nil
// starts the app as Run does. Like Run, it returns ErrBusy while another
// operation holds the app.
func (s *Service) RunIf(ctx context.Context, appID int64, still func(sqlc.SharedApp) error) error {
	if s.vault == nil {
		return ErrNoVault
	}
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return err
	}
	if err := still(sa); err != nil {
		return err
	}
	return s.runApp(ctx, appID, false)
}

func (s *Service) runLocked(ctx context.Context, appID int64, explicit bool) error {
	if s.vault == nil {
		return ErrNoVault
	}
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	return s.runApp(ctx, appID, explicit)
}

// run is Run for callers that already hold the app's lock.
func (s *Service) run(ctx context.Context, appID int64) error { return s.runApp(ctx, appID, false) }

// runApp re-reads the app under its lock: a "Parar" (or unshare) that came
// first wins over a restart or a wake, unless this is an explicit Ligar.
func (s *Service) runApp(ctx context.Context, appID int64, explicit bool) error {
	if s.vault == nil {
		return ErrNoVault
	}
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil || !sa.CurrentDeploymentID.Valid || sa.UnsharedAt.Valid {
		return ErrNotShared
	}
	if sa.Status == "stopped" && !explicit {
		return ErrStopped
	}
	dep, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil {
		return err
	}
	app, err := s.q.GetApp(ctx, appID)
	if err != nil {
		return err
	}
	s.setTarget(appID, false, 0)
	if dep.ContainerID != "" {
		logErr(s.eng.Remove(ctx, dep.ContainerID), "remove container", "container", dep.ContainerID)
		logErr(s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: dep.ID}), "clear container", "deployment", dep.ID)
	}
	logErr(s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "starting"}), "set status", "app", appID)
	m := manifestOf(app)
	port := int(dep.Port)
	if port == 0 {
		rec, _ := recipes.Detect(versions.LocalFolder{Root: app.FolderPath}, m)
		port = rec.Port
		if port == 0 {
			port = int(sa.Port)
		}
	}
	j := job{app: app, sa: sa, m: m, dep: dep, rec: recipes.Recipe{Port: port}}
	cid, hostPort, f := s.start(ctx, j, dep.ImageID, sa.DbName)
	if f != nil {
		logErr(s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "crashed", StatusMessage: f.msg}), "set status", "app", appID)
		return errors.New(f.msg)
	}
	if err := s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: dep.ID, ContainerID: cid}); err != nil {
		// Without the container ID the Hub would lose track of it: do not leave it running.
		logErr(s.eng.Remove(ctx, cid), "remove container", "container", cid)
		logErr(s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "crashed",
			StatusMessage: "Não foi possível registrar o app ligado no Hub."}), "set status", "app", appID)
		return fmt.Errorf("record container: %w", err)
	}
	s.setTarget(appID, false, hostPort)
	logErr(s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "running"}), "set status", "app", appID)
	return nil
}

// Wake runs the app in the background, once at a time per app. It does
// nothing while another operation holds the app, nor for a stopped app.
func (s *Service) Wake(appID int64) { s.wake(appID, false) }

func (s *Service) wake(appID int64, explicit bool) {
	s.mu.Lock()
	if s.waking[appID] || s.busy[appID] {
		s.mu.Unlock()
		return
	}
	s.waking[appID] = true
	s.mu.Unlock()
	s.spawn(func() {
		defer func() { s.mu.Lock(); delete(s.waking, appID); s.mu.Unlock() }()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("shared wake panic", "app", appID, "panic", r, "stack", string(debug.Stack()))
			}
		}()
		if err := s.runLocked(context.Background(), appID, explicit); errors.Is(err, ErrStopped) {
			slog.Info("shared wake: app is stopped", "app", appID)
		} else if err != nil {
			slog.Warn("shared wake", "app", appID, "err", err)
		}
	})
}

// Start is "Ligar"/"Religar": it clears the restart count and wakes the app.
func (s *Service) Start(ctx context.Context, appID int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	if _, err := s.q.GetSharedApp(ctx, appID); err != nil {
		unlock()
		return ErrNotShared
	}
	if err := s.q.SetSharedRestarts(ctx, sqlc.SetSharedRestartsParams{AppID: appID}); err != nil {
		unlock()
		return err
	}
	logErr(s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "starting"}), "set status", "app", appID)
	unlock()
	s.wake(appID, true)
	return nil
}

// removeLive stops and removes the live container (the image stays). The
// caller holds the app's lock.
func (s *Service) removeLive(ctx context.Context, appID int64) error {
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	s.setTarget(appID, false, 0)
	if sa.CurrentDeploymentID.Valid {
		if d, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64); err == nil && d.ContainerID != "" {
			if err := s.eng.Remove(ctx, d.ContainerID); err != nil {
				return err
			}
			logErr(s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: d.ID}), "clear container", "deployment", d.ID)
		}
	}
	return nil
}

// Sleep turns the app off after a while without use; the next visit wakes
// it. It does nothing while another operation holds the app.
func (s *Service) Sleep(ctx context.Context, appID int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return nil
	}
	defer unlock()
	if err := s.removeLive(ctx, appID); err != nil {
		return err
	}
	return s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "sleeping"})
}

// Stop turns the app off until someone turns it on again.
func (s *Service) Stop(ctx context.Context, appID, by int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.removeLive(ctx, appID); err != nil {
		return err
	}
	if err := s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "stopped",
		StoppedBy: pgtype.Int8{Int64: by, Valid: true}}); err != nil {
		return err
	}
	s.event(ctx, appID, &by, "Parou o app")
	return nil
}

// Unshare removes containers and images; the database and copies stay for
// 30 days (Purge).
func (s *Service) Unshare(ctx context.Context, appID, by int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	if tests, err := s.q.ListOpenTests(ctx, appID); err == nil {
		for _, d := range tests {
			logErr(s.EndTest(ctx, d.ID), "end test", "deployment", d.ID)
		}
	}
	s.setTarget(appID, false, 0)
	for _, id := range []pgtype.Int8{sa.CurrentDeploymentID, sa.PreviousDeploymentID} {
		if !id.Valid {
			continue
		}
		if d, err := s.q.GetDeployment(ctx, id.Int64); err == nil {
			if d.ContainerID != "" {
				logErr(s.eng.Remove(ctx, d.ContainerID), "remove container", "container", d.ContainerID)
			}
			if d.ImageID != "" {
				logErr(s.eng.RemoveImage(ctx, d.ImageID), "remove image", "image", d.ImageID)
			}
			logErr(s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: d.ID}), "clear container", "deployment", d.ID)
			logErr(s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: d.ID, Status: "retired"}), "retire", "deployment", d.ID)
		}
	}
	logErr(s.eng.RemoveNetwork(ctx, engine.AppNetwork(appID)), "remove network", "app", appID)
	if err := s.q.MarkUnshared(ctx, appID); err != nil {
		return err
	}
	s.event(ctx, appID, &by, "Deixou de compartilhar (banco e cópias guardados por 30 dias)")
	return nil
}

// Purge deletes what Unshare kept, 30 days later (called by the supervisor).
func (s *Service) Purge(ctx context.Context, appID int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return err
	}
	if !sa.UnsharedAt.Valid {
		return fmt.Errorf("purge app %d: still shared", appID)
	}
	if sa.DbName != "" {
		if err := s.dbs.Drop(ctx, sa.DbName); err != nil {
			return fmt.Errorf("drop %s: %w", sa.DbName, err)
		}
	}
	logErr(s.eng.RemoveNetwork(ctx, engine.AppNetwork(appID)), "remove network", "app", appID) // if Unshare could not
	logErr(os.RemoveAll(filepath.Join(s.opt.DataDir, "backups", strconv.FormatInt(appID, 10))), "remove backups", "app", appID)
	logErr(os.RemoveAll(filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(appID, 10))), "remove app data", "app", appID)
	if err := s.q.DeleteBackupsOfApp(ctx, appID); err != nil {
		return err
	}
	return s.q.DeleteSharedApp(ctx, appID)
}

// Adopt looks at the live container after a Hub restart: if it is running,
// the gateway target is restored and true is returned.
func (s *Service) Adopt(ctx context.Context, appID int64) (bool, error) {
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil || !sa.CurrentDeploymentID.Valid {
		return false, err
	}
	d, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil || d.ContainerID == "" {
		return false, err
	}
	st, err := s.eng.Inspect(ctx, d.ContainerID)
	if err != nil || !st.Running || st.HostPort == 0 {
		return false, err
	}
	s.setTarget(appID, false, st.HostPort)
	return true, nil
}

// HubEvent records a line on the app's timeline with no person ("Hub").
func (s *Service) HubEvent(ctx context.Context, appID int64, label string) {
	s.event(ctx, appID, nil, label)
}
