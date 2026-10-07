// Package supervisor is the single loop that keeps shared apps healthy:
// restart, sleep when idle, measure, daily copies, cleanup.
package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/health"
)

// msgStale is the reason shown for a deployment the Hub lost track of.
const msgStale = "O Hub reiniciou durante a montagem. Publique de novo."

type Supervisor struct {
	Interval   time.Duration
	BackupHour int

	q   *sqlc.Queries
	eng engine.Engine
	svc *deploy.Service
	now func() time.Time

	mu          sync.Mutex
	access      map[int64]time.Time
	lastUsage   time.Time
	lastCleanup time.Time
	backupDay   string
	down        bool
}

func New(q *sqlc.Queries, eng engine.Engine, svc *deploy.Service) *Supervisor {
	return &Supervisor{Interval: 30 * time.Second, BackupHour: 3, q: q, eng: eng, svc: svc, now: time.Now, access: map[int64]time.Time{}}
}

func (s *Supervisor) SetNow(fn func() time.Time) { s.now = fn }

// Touch records that the app was just used (called by the gateway).
func (s *Supervisor) Touch(appID int64) {
	s.mu.Lock()
	s.access[appID] = s.now()
	s.mu.Unlock()
}

func (s *Supervisor) Run(ctx context.Context) {
	if err := s.Reconcile(ctx); err != nil {
		slog.Warn("supervisor reconcile", "err", err)
	}
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

// Reconcile runs when the Hub starts: it clears what a crash left half done,
// removes containers the Hub no longer expects and brings back the apps that
// should be up.
func (s *Supervisor) Reconcile(ctx context.Context) error {
	if err := s.eng.Available(ctx); err != nil {
		return err
	}
	s.clearStaleBuilding(ctx)
	if tests, err := s.q.ListAllOpenTests(ctx); err == nil {
		for _, d := range tests {
			if err := s.svc.EndTest(ctx, d.ID); err != nil {
				slog.Warn("supervisor end test", "deployment", d.ID, "err", err)
			}
		}
	}
	expected := map[string]bool{}
	ids, err := s.q.ListLabelledContainersExpected(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		expected[id] = true
	}
	list, err := s.eng.List(ctx)
	if err != nil {
		return err
	}
	for _, c := range list {
		if !expected[c.ID] {
			slog.Info("supervisor removes orphan", "name", c.Name)
			if err := s.eng.Remove(ctx, c.ID); err != nil {
				slog.Warn("supervisor remove orphan", "name", c.Name, "err", err)
			}
		}
	}
	apps, err := s.q.ListSharedApps(ctx)
	if err != nil {
		return err
	}
	for _, a := range apps {
		if (a.Status != "running" && a.Status != "starting") || s.svc.Busy(a.AppID) {
			continue
		}
		if ok, _ := s.svc.Adopt(ctx, a.AppID); !ok {
			if err := s.svc.Run(ctx, a.AppID); err != nil {
				slog.Warn("supervisor start on boot", "app", a.AppID, "err", err)
			}
		}
	}
	return nil
}

// clearStaleBuilding fails the deployments left "building" by a Hub that died
// (or lost the database) in the middle of a publish or test. An app this Hub
// is working on right now (Busy) is never touched: its "building" is real.
// A first publish goes back to draft, as deploy's own failure path does.
func (s *Supervisor) clearStaleBuilding(ctx context.Context) {
	list, err := s.q.ListBuildingDeployments(ctx)
	if err != nil {
		slog.Warn("supervisor stale deployments", "err", err)
		return
	}
	var running []engine.Container
	if len(list) > 0 {
		running, _ = s.eng.List(ctx)
	}
	for _, d := range list {
		if s.svc.Busy(d.AppID) {
			continue
		}
		if cur, err := s.q.GetDeployment(ctx, d.ID); err != nil || cur.Status != "building" || s.svc.Busy(d.AppID) {
			continue // finished meanwhile
		}
		slog.Warn("supervisor clears stale deployment", "app", d.AppID, "deployment", d.ID)
		if d.ContainerID != "" {
			_ = s.eng.Remove(ctx, d.ContainerID)
		}
		for _, c := range running {
			if c.Labels["app-hub.deployment"] == strconv.FormatInt(d.ID, 10) {
				_ = s.eng.Remove(ctx, c.ID)
			}
		}
		if sa, err := s.q.GetSharedApp(ctx, d.AppID); err == nil && d.Kind == "live" && !sa.CurrentDeploymentID.Valid && !sa.UnsharedAt.Valid {
			_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: d.AppID, Status: "draft", StatusMessage: msgStale})
		}
		if err := s.q.FinishDeployment(ctx, sqlc.FinishDeploymentParams{ID: d.ID, Status: "failed", ImageID: d.ImageID,
			Port: d.Port, BuildLog: d.BuildLog, ErrorMessage: msgStale}); err != nil {
			slog.Error("supervisor stale deployment", "deployment", d.ID, "err", err)
			continue
		}
		if d.Kind == "test" { // test database, data and container go with it
			if err := s.svc.EndTest(ctx, d.ID); err != nil {
				slog.Warn("supervisor end stale test", "deployment", d.ID, "err", err)
			}
		}
	}
}

func (s *Supervisor) Tick(ctx context.Context) {
	if err := s.eng.Available(ctx); err != nil {
		if !s.down {
			slog.Warn("supervisor: container engine unavailable", "err", err)
		}
		s.down = true
		return
	}
	s.down = false
	now := s.now()
	s.flushAccess(ctx)
	apps, err := s.q.ListSharedApps(ctx)
	if err != nil {
		slog.Error("supervisor list", "err", err)
		return
	}
	for _, a := range apps {
		if a.Status != "running" || s.svc.Busy(a.AppID) {
			continue // something else is working on it
		}
		if !s.healthy(ctx, a) {
			s.restart(ctx, a, now)
			continue
		}
		last := a.StatusChangedAt.Time
		if a.LastAccessAt.Valid && a.LastAccessAt.Time.After(last) {
			last = a.LastAccessAt.Time
		}
		if !a.AlwaysOn && now.Sub(last) > time.Duration(a.IdleMinutes)*time.Minute {
			if err := s.svc.Sleep(ctx, a.AppID); err != nil {
				slog.Warn("supervisor sleep", "app", a.AppID, "err", err)
			}
		}
	}
	if now.Sub(s.lastUsage) >= time.Minute {
		s.lastUsage = now
		s.measure(ctx, apps, now)
	}
	if day := now.Format("2006-01-02"); now.Hour() >= s.BackupHour && s.backupDay != day {
		s.backupDay = day
		s.dailyBackups(ctx, apps, now)
	}
	if now.Sub(s.lastCleanup) >= time.Hour {
		s.lastCleanup = now
		s.cleanup(ctx, now)
	}
}

func (s *Supervisor) flushAccess(ctx context.Context) {
	s.mu.Lock()
	acc := s.access
	s.access = map[int64]time.Time{}
	s.mu.Unlock()
	for id, t := range acc {
		if err := s.q.TouchSharedAccess(ctx, sqlc.TouchSharedAccessParams{AppID: id, LastAccessAt: pgtype.Timestamptz{Time: t, Valid: true}}); err != nil {
			slog.Warn("supervisor touch access", "app", id, "err", err)
		}
	}
}

func (s *Supervisor) container(ctx context.Context, a sqlc.ListSharedAppsRow) string {
	if !a.CurrentDeploymentID.Valid {
		return ""
	}
	d, err := s.q.GetDeployment(ctx, a.CurrentDeploymentID.Int64)
	if err != nil {
		return ""
	}
	return d.ContainerID
}

func (s *Supervisor) healthy(ctx context.Context, a sqlc.ListSharedAppsRow) bool {
	id := s.container(ctx, a)
	if id == "" {
		return false
	}
	st, err := s.eng.Inspect(ctx, id)
	if err != nil || !st.Running || st.HostPort == 0 {
		return false
	}
	return health.Check(ctx, fmt.Sprintf("http://127.0.0.1:%d%s", st.HostPort, a.HealthPath))
}

func (s *Supervisor) restart(ctx context.Context, a sqlc.ListSharedAppsRow, now time.Time) {
	count, start := a.RestartCount, a.RestartWindowStart
	if !start.Valid || now.Sub(start.Time) > 10*time.Minute {
		count, start = 0, pgtype.Timestamptz{Time: now, Valid: true}
	}
	if count >= 3 {
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: a.AppID, Status: "crashed",
			StatusMessage: "O app parou de responder 3 vezes em 10 minutos."})
		s.svc.HubEvent(ctx, a.AppID, "O app caiu e o Hub parou de tentar religar")
		return
	}
	if err := s.q.SetSharedRestarts(ctx, sqlc.SetSharedRestartsParams{AppID: a.AppID, RestartCount: count + 1, RestartWindowStart: start}); err != nil {
		slog.Warn("supervisor restart count", "app", a.AppID, "err", err)
	}
	slog.Info("supervisor restarts app", "app", a.AppID, "try", count+1)
	if err := s.svc.Run(ctx, a.AppID); err != nil {
		slog.Warn("supervisor restart", "app", a.AppID, "err", err)
	}
}

func (s *Supervisor) measure(ctx context.Context, apps []sqlc.ListSharedAppsRow, now time.Time) {
	byContainer := map[string]int64{}
	var ids []string
	for _, a := range apps {
		if a.Status == "running" {
			if id := s.container(ctx, a); id != "" {
				byContainer[id] = a.AppID
				ids = append(ids, id)
			}
		}
	}
	usage, err := s.eng.Stats(ctx, ids)
	if err != nil {
		slog.Warn("supervisor stats", "err", err)
		return
	}
	at := pgtype.Timestamptz{Time: now.Truncate(time.Minute), Valid: true}
	for _, u := range usage {
		appID, ok := byContainer[u.ContainerID]
		if !ok { // o Podman pode devolver o ID curto
			for full, id := range byContainer {
				if len(u.ContainerID) >= 12 && len(full) >= 12 && full[:12] == u.ContainerID[:12] {
					appID, ok = id, true
				}
			}
		}
		if ok {
			if err := s.q.InsertUsage(ctx, sqlc.InsertUsageParams{AppID: appID, At: at, MemBytes: u.MemBytes, CpuPct: float32(u.CPUPct)}); err != nil {
				slog.Warn("supervisor usage", "app", appID, "err", err)
			}
		}
	}
}

func (s *Supervisor) dailyBackups(ctx context.Context, apps []sqlc.ListSharedAppsRow, now time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, a := range apps {
		if a.DbName == "" {
			continue
		}
		if b, err := s.q.LastDailyBackup(ctx, a.AppID); err == nil && !b.CreatedAt.Time.Before(today) {
			continue
		}
		if _, err := s.svc.Backup(ctx, a.AppID, "daily", nil, nil); err != nil {
			slog.Error("supervisor daily backup", "app", a.AppID, "err", err)
		}
	}
}

func (s *Supervisor) cleanup(ctx context.Context, now time.Time) {
	ts := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	s.clearStaleBuilding(ctx) // a building deployment nobody is working on can never finish
	if tests, err := s.q.ListExpiredTests(ctx, ts(now)); err == nil {
		for _, d := range tests {
			if err := s.svc.EndTest(ctx, d.ID); err != nil {
				slog.Warn("supervisor end expired test", "deployment", d.ID, "err", err)
			}
		}
	}
	if old, err := s.q.ListExpiredBackups(ctx, ts(now)); err == nil {
		for _, b := range old {
			if err := os.Remove(b.Path); err != nil && !os.IsNotExist(err) {
				slog.Warn("supervisor remove backup file", "path", b.Path, "err", err)
			}
			if err := s.q.DeleteBackup(ctx, b.ID); err != nil {
				slog.Warn("supervisor delete backup", "backup", b.ID, "err", err)
			}
		}
	}
	if err := s.q.DeleteUsageBefore(ctx, ts(now.Add(-90*24*time.Hour))); err != nil {
		slog.Warn("supervisor prune usage", "err", err)
	}
	if gone, err := s.q.ListUnsharedBefore(ctx, ts(now.Add(-30*24*time.Hour))); err == nil {
		for _, a := range gone {
			if err := s.svc.Purge(ctx, a.AppID); err != nil {
				slog.Error("supervisor purge", "app", a.AppID, "err", err)
			}
		}
	}
}
