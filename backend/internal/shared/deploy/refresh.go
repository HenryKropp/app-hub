package deploy

import (
	"context"
	"errors"

	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// ErrRefreshRunning: another refresh is building somewhere on the server.
// The caller tries again later.
var ErrRefreshRunning = errors.New("deploy: another refresh is running")

// NeedsRefresh reports whether the app's version in the air was built with a
// recipe the Ziv no longer generates (the Ziv learned something, like the
// browser in ADR 040), while its code is still the one published. Only then
// the Ziv may rebuild it alone: the person sees the same app, now built the
// new way. An app that brings its own Dockerfile, a code change waiting to be
// published or a folder the Ziv cannot read never counts.
func (s *Service) NeedsRefresh(ctx context.Context, appID int64) (bool, error) {
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil || !sa.CurrentDeploymentID.Valid || sa.UnsharedAt.Valid {
		return false, err
	}
	dep, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil {
		return false, err
	}
	if v, err := s.q.LatestAppVersion(ctx, appID); err == nil {
		if !dep.VersionID.Valid || dep.VersionID.Int64 != v.ID {
			return false, nil // new code waits for the person to publish it
		}
	} else if dep.VersionID.Valid {
		return false, nil
	}
	app, err := s.q.GetApp(ctx, appID)
	if err != nil {
		return false, err
	}
	if s.CheckFolder(app.FolderPath) != nil {
		return false, nil
	}
	rec, err := recipes.Detect(versions.LocalFolder{Root: app.FolderPath}, manifestOf(app))
	if err != nil || rec.Kind == "" || rec.Kind == "dockerfile" {
		return false, nil
	}
	return rec.Dockerfile != dep.Recipe, nil
}

// Refresh rebuilds the app's version in the air with the current recipe, the
// way a publish does (the old box stays in the air until the new one
// answers), recorded as done by the Ziv. One refresh at a time on the whole
// server: ErrRefreshRunning while another one builds.
func (s *Service) Refresh(ctx context.Context, appID int64) (bool, error) {
	ok, err := s.NeedsRefresh(ctx, appID)
	if err != nil || !ok {
		return false, err
	}
	done, held := s.opt.Slots.TryRefresh()
	if !held {
		return false, ErrRefreshRunning
	}
	app, err := s.q.GetApp(ctx, appID)
	if err != nil {
		done()
		return false, err
	}
	if _, err := s.publish(ctx, app, manifestOf(app), 0, done); err != nil {
		done()
		return false, err
	}
	return true, nil
}
