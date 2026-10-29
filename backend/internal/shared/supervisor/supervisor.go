// Package supervisor is the single loop that keeps shared apps healthy:
// restart, sleep when idle, measure, daily copies, cleanup.
package supervisor

import (
	"context"
	"errors"
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

// staleAfter is how old a "building" deployment must be before it can be
// called lost: well beyond the longest build plus the 90 s health wait, so a
// second Hub process (or a late job) is never cut off.
const staleAfter = 30 * time.Minute

// msgGaveUp is the status message once the supervisor stops restarting.
const msgGaveUp = "O app parou de responder 3 vezes em 10 minutos."

// restartWindow is how long restart tries are counted.
const restartWindow = 10 * time.Minute

// msgStale is the reason shown for a deployment the Hub lost track of.
const msgStale = "O Kova reiniciou durante a montagem. Publique de novo."

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
	backupTry   map[int64]time.Time // last failed daily copy per app
	down        bool

	fails         map[int64]failedChecks // failed health checks in a row, per app
	beforeRestart func(appID int64)      // tests: runs between the health check and the restart
}

// failedChecks counts failed health checks in a row for one live container.
type failedChecks struct {
	container string
	n         int
}

// failsToRestart is how many failed health checks in a row restart a running
// app: one slow answer (the check gives up after 5 s) is not enough.
const failsToRestart = 2

// errChanged: the app is no longer what the supervisor checked.
var errChanged = errors.New("app changed since the check")

// errGaveUp: the restart limit was reached; nothing is started.
var errGaveUp = errors.New("gave up restarting")

func New(q *sqlc.Queries, eng engine.Engine, svc *deploy.Service) *Supervisor {
	return &Supervisor{Interval: 30 * time.Second, BackupHour: 3, q: q, eng: eng, svc: svc, now: time.Now, access: map[int64]time.Time{}, backupTry: map[int64]time.Time{}, fails: map[int64]failedChecks{}}
}

func (s *Supervisor) SetNow(fn func() time.Time) { s.now = fn }

// Touch records that the app was just used (called by the gateway).
func (s *Supervisor) Touch(appID int64) {
	s.mu.Lock()
	s.access[appID] = s.now()
	s.mu.Unlock()
}

// Run is Reconcile followed by Loop.
func (s *Supervisor) Run(ctx context.Context) {
	if err := s.Reconcile(ctx); err != nil {
		slog.Warn("supervisor reconcile", "err", err)
	}
	s.Loop(ctx)
}

// Loop ticks every Interval until ctx ends. The server calls Reconcile itself
// before serving, then runs Loop in the background.
func (s *Supervisor) Loop(ctx context.Context) {
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
	s.clearStaleBuilding(ctx, s.now(), 0) // no job of this process exists yet: every non-busy row is lost
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
		label, _ := engine.AppLabel(c.Labels)
		if id, err := strconv.ParseInt(label, 10, 64); err == nil && s.svc.Busy(id) {
			continue // being built or started right now by this Hub
		}
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
		if ok, _ := s.svc.Adopt(ctx, a.AppID); ok {
			if a.Status == "starting" { // the Hub died before saying "running"
				if err := s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: a.AppID, Status: "running"}); err != nil {
					slog.Warn("supervisor set running", "app", a.AppID, "err", err)
				}
			}
		} else if err := s.svc.Run(ctx, a.AppID); err != nil {
			slog.Warn("supervisor start on boot", "app", a.AppID, "err", err)
		}
	}
	return nil
}

// clearStaleBuilding fails the deployments left "building" by a Hub that died
// (or lost the database) in the middle of a publish or test. An app this Hub
// is working on right now (Busy) is never touched: its "building" is real.
// A first publish goes back to draft, as deploy's own failure path does.
func (s *Supervisor) clearStaleBuilding(ctx context.Context, now time.Time, minAge time.Duration) {
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
		if s.svc.Busy(d.AppID) || now.Sub(d.CreatedAt.Time) < minAge {
			continue
		}
		if cur, err := s.q.GetDeployment(ctx, d.ID); err != nil || cur.Status != "building" || s.svc.Busy(d.AppID) {
			continue // finished meanwhile
		}
		slog.Warn("supervisor clears stale deployment", "app", d.AppID, "deployment", d.ID)
		// First claim the row (only a still-"building" one is updated), then clean up.
		if err := s.svc.Finish(ctx, sqlc.FinishDeploymentParams{ID: d.ID, Status: "failed", ImageID: d.ImageID,
			Port: d.Port, BuildLog: d.BuildLog, ErrorMessage: msgStale}); err != nil {
			if !errors.Is(err, deploy.ErrNotBuilding) {
				slog.Error("supervisor stale deployment", "deployment", d.ID, "err", err)
			}
			continue
		}
		if d.ContainerID != "" {
			_ = s.eng.Remove(ctx, d.ContainerID)
		}
		for _, c := range running {
			if engine.DeploymentID(c.Labels) == strconv.FormatInt(d.ID, 10) {
				_ = s.eng.Remove(ctx, c.ID)
			}
		}
		if sa, err := s.q.GetSharedApp(ctx, d.AppID); err == nil && d.Kind == "live" && !sa.CurrentDeploymentID.Valid && !sa.UnsharedAt.Valid {
			_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: d.AppID, Status: "draft", StatusMessage: msgStale})
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
		if a.Status != "running" {
			delete(s.fails, a.AppID) // failed checks count only while it runs
		}
		if s.svc.Busy(a.AppID) {
			continue // something else is working on it
		}
		if a.Status == "crashed" {
			// A restart that failed health leaves "crashed": keep trying inside
			// the window, and say so once when giving up.
			// (its tries never reset here, only once it runs again.)
			if a.RestartWindowStart.Valid && a.StatusMessage != msgGaveUp {
				s.restart(ctx, a, now)
			}
			continue
		}
		if a.Status != "running" {
			continue
		}
		if h, cid := s.healthy(ctx, a); h != up {
			f := s.fails[a.AppID]
			if f.container != cid { // a new container starts a new count
				f = failedChecks{container: cid}
			}
			f.n++
			if h == noAnswer && f.n < failsToRestart { // a dead container needs no second look
				s.fails[a.AppID] = f
				continue
			}
			delete(s.fails, a.AppID)
			if s.beforeRestart != nil {
				s.beforeRestart(a.AppID)
			}
			s.restart(ctx, a, now)
			continue
		}
		delete(s.fails, a.AppID)
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
		if s.dailyBackups(ctx, apps, now) {
			s.backupDay = day
		}
	}
	s.endExpiredTests(ctx, now)
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

// checkResult is what one check of the live container found.
type checkResult int

const (
	up       checkResult = iota
	down                 // no container, or it is not running: certain
	noAnswer             // running, but / did not answer well in time: maybe just slow
)

// healthy checks the live container and also says which container it was.
func (s *Supervisor) healthy(ctx context.Context, a sqlc.ListSharedAppsRow) (checkResult, string) {
	id := s.container(ctx, a)
	if id == "" {
		return down, ""
	}
	st, err := s.eng.Inspect(ctx, id)
	if err != nil || !st.Running || st.HostPort == 0 {
		return down, id
	}
	if !health.Check(ctx, fmt.Sprintf("http://127.0.0.1:%d%s", st.HostPort, a.HealthPath)) {
		return noAnswer, id
	}
	return up, id
}

// restart starts the app again, under its lock and only if it is still what
// the tick saw: same live deployment, same status, same moment of the last
// status change. A publish, swap, rollback, sleep or stop that landed after
// the check wins, and nothing is counted.
func (s *Supervisor) restart(ctx context.Context, a sqlc.ListSharedAppsRow, now time.Time) {
	err := s.svc.RunIf(ctx, a.AppID, func(sa sqlc.SharedApp) error {
		if sa.UnsharedAt.Valid || sa.Status != a.Status || sa.CurrentDeploymentID != a.CurrentDeploymentID ||
			!sa.StatusChangedAt.Time.Equal(a.StatusChangedAt.Time) {
			return errChanged
		}
		count, start := sa.RestartCount, sa.RestartWindowStart
		// Only an app found "running" (it came back up and was healthy) may open
		// a fresh window. A "crashed" app is still the same failure: its tries keep
		// counting however far apart the ticks land, so it never gets a 4th start.
		if sa.Status == "running" && (!start.Valid || now.Sub(start.Time) > restartWindow) {
			count, start = 0, pgtype.Timestamptz{Time: now, Valid: true}
		}
		if count >= 3 {
			_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: a.AppID, Status: "crashed",
				StatusMessage: msgGaveUp})
			s.svc.HubEvent(ctx, a.AppID, "O app caiu e o Kova parou de tentar religar")
			return errGaveUp
		}
		if err := s.q.SetSharedRestarts(ctx, sqlc.SetSharedRestartsParams{AppID: a.AppID, RestartCount: count + 1, RestartWindowStart: start}); err != nil {
			slog.Warn("supervisor restart count", "app", a.AppID, "err", err)
		}
		slog.Info("supervisor restarts app", "app", a.AppID, "try", count+1)
		return nil
	})
	switch {
	case err == nil, errors.Is(err, errGaveUp):
	case errors.Is(err, errChanged), errors.Is(err, deploy.ErrBusy), errors.Is(err, deploy.ErrStopped):
		slog.Info("supervisor skips restart: the app changed", "app", a.AppID, "err", err) // not a try
	default:
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

// dailyBackups makes today's copy of every app with a database that has none
// yet. It reports whether every app is covered; busy or failed ones are
// retried on a later tick.
func (s *Supervisor) dailyBackups(ctx context.Context, apps []sqlc.ListSharedAppsRow, now time.Time) bool {
	done := true
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, a := range apps {
		if a.DbName == "" {
			continue
		}
		if b, err := s.q.LastDailyBackup(ctx, a.AppID); err == nil && !b.CreatedAt.Time.Before(today) {
			continue
		}
		if s.svc.Busy(a.AppID) {
			done = false
			continue
		}
		if t, ok := s.backupTry[a.AppID]; ok && now.Sub(t) < time.Hour {
			done = false // failed recently: try again later, not every tick
			continue
		}
		if _, err := s.svc.Backup(ctx, a.AppID, "daily", nil, nil); err != nil {
			slog.Error("supervisor daily backup", "app", a.AppID, "err", err)
			s.backupTry[a.AppID] = now
			done = false
		}
	}
	return done
}

// endExpiredTests turns off the test boxes past their 30 minutes (every tick,
// so they go away on time).
func (s *Supervisor) endExpiredTests(ctx context.Context, now time.Time) {
	tests, err := s.q.ListExpiredTests(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		slog.Warn("supervisor expired tests", "err", err)
		return
	}
	for _, d := range tests {
		if s.svc.Busy(d.AppID) {
			continue // a publish or a newer test is in progress; next tick
		}
		if err := s.svc.EndTest(ctx, d.ID); err != nil {
			slog.Warn("supervisor end expired test", "deployment", d.ID, "err", err)
		}
	}
}

func (s *Supervisor) cleanup(ctx context.Context, now time.Time) {
	ts := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	s.clearStaleBuilding(ctx, now, staleAfter) // a building deployment nobody is working on can never finish
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
