package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// Run starts the live version (new container from the kept image), waits for
// health and marks the app running. Used to wake a sleeping app, by
// "Ligar"/"Religar" and by the supervisor to restart a crashed one.
func (s *Service) Run(ctx context.Context, appID int64) error {
	if s.vault == nil {
		return ErrNoVault
	}
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil || !sa.CurrentDeploymentID.Valid || sa.UnsharedAt.Valid {
		return ErrNotShared
	}
	dep, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil {
		return err
	}
	app, err := s.q.GetApp(ctx, appID)
	if err != nil {
		return err
	}
	if dep.ContainerID != "" {
		_ = s.eng.Remove(ctx, dep.ContainerID)
		_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: dep.ID})
	}
	s.setTarget(appID, false, 0)
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "starting"})
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
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "crashed", StatusMessage: f.msg})
		return errors.New(f.msg)
	}
	s.setTarget(appID, false, hostPort)
	_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: dep.ID, ContainerID: cid})
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "running"})
	return nil
}

// Wake runs the app in the background, once at a time per app.
func (s *Service) Wake(appID int64) {
	s.mu.Lock()
	if s.waking[appID] {
		s.mu.Unlock()
		return
	}
	s.waking[appID] = true
	s.mu.Unlock()
	s.spawn(func() {
		defer func() { s.mu.Lock(); delete(s.waking, appID); s.mu.Unlock() }()
		if err := s.Run(context.Background(), appID); err != nil {
			slog.Warn("shared wake", "app", appID, "err", err)
		}
	})
}

// Start is "Ligar"/"Religar": it clears the restart count and wakes the app.
func (s *Service) Start(ctx context.Context, appID int64) error {
	if _, err := s.q.GetSharedApp(ctx, appID); err != nil {
		return ErrNotShared
	}
	if err := s.q.SetSharedRestarts(ctx, sqlc.SetSharedRestartsParams{AppID: appID}); err != nil {
		return err
	}
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "starting"})
	s.Wake(appID)
	return nil
}

// removeLive stops and removes the live container (the image stays).
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
			_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: d.ID})
		}
	}
	return nil
}

// Sleep turns the app off after a while without use; the next visit wakes it.
func (s *Service) Sleep(ctx context.Context, appID int64) error {
	if err := s.removeLive(ctx, appID); err != nil {
		return err
	}
	return s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "sleeping"})
}

// Stop turns the app off until someone turns it on again.
func (s *Service) Stop(ctx context.Context, appID, by int64) error {
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
			_ = s.EndTest(ctx, d.ID)
		}
	}
	s.setTarget(appID, false, 0)
	for _, id := range []pgtype.Int8{sa.CurrentDeploymentID, sa.PreviousDeploymentID} {
		if !id.Valid {
			continue
		}
		if d, err := s.q.GetDeployment(ctx, id.Int64); err == nil {
			if d.ContainerID != "" {
				_ = s.eng.Remove(ctx, d.ContainerID)
			}
			if d.ImageID != "" {
				_ = s.eng.RemoveImage(ctx, d.ImageID)
			}
			_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: d.ID})
			_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: d.ID, Status: "retired"})
		}
	}
	if err := s.q.MarkUnshared(ctx, appID); err != nil {
		return err
	}
	s.event(ctx, appID, &by, "Deixou de compartilhar (banco e cópias guardados por 30 dias)")
	return nil
}

// Purge deletes what Unshare kept, 30 days later (called by the supervisor).
func (s *Service) Purge(ctx context.Context, appID int64) error {
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
	_ = os.RemoveAll(filepath.Join(s.opt.DataDir, "backups", strconv.FormatInt(appID, 10)))
	_ = os.RemoveAll(filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(appID, 10)))
	if err := s.q.DeleteBackupsOfApp(ctx, appID); err != nil {
		return err
	}
	return s.q.DeleteSharedApp(ctx, appID)
}
