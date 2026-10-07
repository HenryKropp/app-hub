package supervisor

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	var err error
	pool, err = deploytest.Pool(context.Background(), "apphub_test_supervisor")
	if err != nil {
		panic(err)
	}
	if pool == nil {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }

func live(t *testing.T) (*deploytest.Env, *Supervisor, *clock) {
	e := deploytest.Setup(t, pool)
	e.Publish(t)
	c := &clock{t: time.Now()}
	e.S.SetNow(c.now)
	s := New(e.Q, e.Eng, e.S)
	s.SetNow(c.now)
	return e, s, c
}

func currentContainer(t *testing.T, e *deploytest.Env) string {
	d, err := e.Q.GetDeployment(context.Background(), e.Shared(t).CurrentDeploymentID.Int64)
	if err != nil {
		t.Fatal(err)
	}
	return d.ContainerID
}

// holdBusy starts a publish whose background job never runs, so the app stays
// Busy (and a "building" deployment stays recorded) for the whole test.
func holdBusy(t *testing.T, e *deploytest.Env) (sqlc.SharedDeployment, func()) {
	t.Helper()
	var job func()
	e.S.SetSpawn(func(f func()) { job = f })
	d, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !e.S.Busy(e.App.ID) {
		t.Fatal("app should be busy")
	}
	return d, func() { job() }
}

// later is a supervisor whose clock is an hour ahead, so "building" rows
// created just now are old enough to be called lost.
func later(e *deploytest.Env, svc *deploy.Service) *Supervisor {
	s := New(e.Q, e.Eng, svc)
	s.SetNow(func() time.Time { return time.Now().Add(time.Hour) })
	return s
}

func mustStart(t *testing.T, e *deploytest.Env, spec engine.StartSpec) string {
	t.Helper()
	id, err := e.Eng.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustExec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), q, args...); err != nil {
		t.Fatal(err)
	}
}

func TestRestartsThreeTimesThenCrashed(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	for i := 1; i <= 3; i++ {
		e.Eng.Crash(currentContainer(t, e))
		s.Tick(ctx)
		if st := e.Shared(t).Status; st != "running" {
			t.Fatalf("restart %d: status %s", i, st)
		}
		c.add(time.Minute)
	}
	e.Eng.Crash(currentContainer(t, e))
	s.Tick(ctx)
	sa := e.Shared(t)
	if sa.Status != "crashed" || sa.StatusMessage == "" {
		t.Fatalf("%+v", sa)
	}
	var who string
	pool.QueryRow(ctx, `SELECT COALESCE(p.name,'Hub') FROM app_changes c LEFT JOIN people p ON p.id = c.person_id
		WHERE c.app_id = $1 ORDER BY c.id DESC LIMIT 1`, e.App.ID).Scan(&who)
	if who != "Hub" {
		t.Fatalf("last event by %q", who)
	}
	s.Tick(ctx) // caiu: não tenta mais
	if len(e.Eng.Running()) != 0 {
		t.Fatal("kept restarting after crashed")
	}
}

func TestRestartWindowResets(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	for i := 0; i < 3; i++ {
		e.Eng.Crash(currentContainer(t, e))
		s.Tick(ctx)
	}
	c.add(11 * time.Minute)
	e.Eng.Crash(currentContainer(t, e))
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("a new 10-minute window should allow restarting again")
	}
}

func TestUnhealthyButRunningIsRestarted(t *testing.T) {
	ctx := context.Background()
	e, s, _ := live(t)
	old := currentContainer(t, e)
	e.Eng.SetHandler(old, http500{})
	s.Tick(ctx)
	if currentContainer(t, e) == old {
		t.Fatal("an app answering 500 should be restarted")
	}
}

func TestSleepsWhenIdleUnlessAlwaysOn(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	c.add(29 * time.Minute)
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("slept too early")
	}
	s.Touch(e.App.ID)
	c.add(20 * time.Minute)
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("a recent access should keep it awake")
	}
	c.add(31 * time.Minute)
	s.Tick(ctx)
	if e.Shared(t).Status != "sleeping" || len(e.Eng.Running()) != 0 {
		t.Fatalf("status %s", e.Shared(t).Status)
	}
	// sempre ligado
	mustExec(t, "UPDATE shared_apps SET always_on = true WHERE app_id = $1", e.App.ID)
	if err := e.S.Run(ctx, e.App.ID); err != nil {
		t.Fatal(err)
	}
	c.add(5 * time.Hour)
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("always_on app slept")
	}
}

func TestUsageEveryMinute(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	c.t = c.t.Truncate(time.Minute).Add(5 * time.Second) // inside a minute, whatever the real time
	s.Tick(ctx)
	c.add(30 * time.Second)
	s.Tick(ctx) // mesmo minuto: não grava de novo
	c.add(40 * time.Second)
	s.Tick(ctx)
	var n int
	pool.QueryRow(ctx, "SELECT count(*) FROM shared_usage WHERE app_id = $1", e.App.ID).Scan(&n)
	if n != 2 {
		t.Fatalf("usage rows %d", n)
	}
}

func TestDailyBackupAfterThree(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	// tomorrow, so the fake clock is never behind the app's real timestamps
	n := time.Now().AddDate(0, 0, 1)
	day := time.Date(n.Year(), n.Month(), n.Day(), 2, 59, 0, 0, time.Local)
	c.t = day
	s.Touch(e.App.ID)
	s.Tick(ctx)
	count := func() int {
		var n int
		pool.QueryRow(ctx, "SELECT count(*) FROM shared_backups WHERE app_id = $1 AND kind = 'daily'", e.App.ID).Scan(&n)
		return n
	}
	if count() != 0 {
		t.Fatal("backup before 3h")
	}
	c.t = day.Add(2 * time.Minute)
	s.Tick(ctx)
	s.Tick(ctx)
	if count() != 1 {
		t.Fatalf("daily backups %d", count())
	}
}

func TestExpiredTestEnded(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	d := e.StartTest(t)
	c.add(31 * time.Minute)
	c.add(time.Hour) // a limpeza roda de hora em hora
	s.Tick(ctx)
	got, _ := e.Q.GetDeployment(ctx, d.ID)
	if got.Status != "retired" {
		t.Fatalf("test status %s", got.Status)
	}
}

func TestReconcileOnStart(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	orphan := mustStart(t, e, engine.StartSpec{Name: "apphub-velho-1", Labels: map[string]string{engine.Label: "99"}})
	foreign := mustStart(t, e, engine.StartSpec{Name: "sistema-alis-db"}) // não é do Hub
	// "reinício": serviço novo, sem portas em memória
	svc2 := e.NewService()
	s := New(e.Q, e.Eng, svc2)
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.Eng.Inspect(ctx, orphan); st.Exists {
		t.Fatal("orphan Hub container kept")
	}
	if st, _ := e.Eng.Inspect(ctx, foreign); !st.Running {
		t.Fatal("touched a container that is not the Hub's")
	}
	if _, ok := svc2.Target(e.App.ID, false); !ok {
		t.Fatal("live app not adopted")
	}
	// caixa sumiu durante o reinício: liga de novo
	e.Eng.Remove(ctx, currentContainer(t, e))
	svc3 := e.NewService()
	if err := New(e.Q, e.Eng, svc3).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc3.Target(e.App.ID, false); !ok || e.Shared(t).Status != "running" {
		t.Fatal("missing container not restarted")
	}
}

// insertBuilding records a deployment left "building" by a Hub that died, with
// a labelled container as the build/start would have left.
func insertBuilding(t *testing.T, e *deploytest.Env, kind string) (sqlc.SharedDeployment, string) {
	t.Helper()
	ctx := context.Background()
	d, err := e.Q.InsertDeployment(ctx, sqlc.InsertDeploymentParams{AppID: e.App.ID, Kind: kind,
		CreatedBy: pgtype.Int8{Int64: 1, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	cid := mustStart(t, e, engine.StartSpec{Name: "apphub-notas-" + strconv.FormatInt(d.ID, 10),
		Labels: map[string]string{engine.Label: strconv.FormatInt(e.App.ID, 10), "app-hub.deployment": strconv.FormatInt(d.ID, 10)}})
	return d, cid
}

func TestReconcileClearsStaleBuildingOfLiveApp(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	d, cid := insertBuilding(t, e, "live")
	svc2 := e.NewService()
	if err := later(e, svc2).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := e.Q.GetDeployment(ctx, d.ID)
	if got.Status != "failed" || got.ErrorMessage == "" {
		t.Fatalf("stale deployment: %s %q", got.Status, got.ErrorMessage)
	}
	if st, _ := e.Eng.Inspect(ctx, cid); st.Exists {
		t.Fatal("container of the stale deployment kept")
	}
	if sa := e.Shared(t); sa.Status != "running" || sa.CurrentDeploymentID.Int64 == d.ID {
		t.Fatalf("the live version must stay: %+v", sa)
	}
	// and the next publish is not blocked by the leftover (its job is not run)
	svc2.SetSpawn(func(func()) {})
	if _, err := svc2.Publish(ctx, e.App, e.M, 1); err != nil {
		t.Fatalf("publish after clearing: %v", err)
	}
}

func TestReconcileClearsStaleBuildingOfFirstPublish(t *testing.T) {
	ctx := context.Background()
	e := deploytest.Setup(t, pool) // draft, never published
	if err := e.Q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: e.App.ID, Status: "starting"}); err != nil {
		t.Fatal(err)
	}
	d, cid := insertBuilding(t, e, "live")
	if err := later(e, e.NewService()).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	sa := e.Shared(t)
	if sa.Status != "draft" || sa.StatusMessage == "" {
		t.Fatalf("first publish should go back to draft with a reason: %s %q", sa.Status, sa.StatusMessage)
	}
	if got, _ := e.Q.GetDeployment(ctx, d.ID); got.Status != "failed" {
		t.Fatalf("deployment %s", got.Status)
	}
	if st, _ := e.Eng.Inspect(ctx, cid); st.Exists {
		t.Fatal("container kept")
	}
}

func TestReconcileClearsStaleBuildingTest(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	d, cid := insertBuilding(t, e, "test")
	if err := later(e, e.NewService()).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.Q.GetDeployment(ctx, d.ID); got.Status != "failed" {
		t.Fatalf("test deployment %s", got.Status)
	}
	if st, _ := e.Eng.Inspect(ctx, cid); st.Exists {
		t.Fatal("test container kept")
	}
	if e.Shared(t).Status != "running" {
		t.Fatal("a stale test must not touch the live app")
	}
}

func TestBusyAppIsLeftAlone(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	d, _ := holdBusy(t, e) // a publish in progress in this Hub: "building" is real
	// it would be crashed (3 restarts used) if the supervisor looked at it
	mustExec(t, "UPDATE shared_apps SET restart_count = 3, restart_window_start = $2 WHERE app_id = $1", e.App.ID, c.t)
	e.Eng.Crash(currentContainer(t, e))
	c.add(2 * time.Hour) // idle too
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	s.Tick(ctx)
	sa := e.Shared(t)
	if sa.Status != "running" || sa.RestartCount != 3 {
		t.Fatalf("busy app was touched: %s count %d", sa.Status, sa.RestartCount)
	}
	if got, _ := e.Q.GetDeployment(ctx, d.ID); got.Status != "building" {
		t.Fatalf("the in-progress deployment was cleared: %s", got.Status)
	}
}

func TestRecentBuildingIsNotStaleInHourlyCleanup(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	d, _ := insertBuilding(t, e, "live")
	New(e.Q, e.Eng, e.NewService()).Tick(ctx) // hourly path, real clock: row is seconds old
	if got, _ := e.Q.GetDeployment(ctx, d.ID); got.Status != "building" {
		t.Fatalf("a recent deployment (maybe another process) was cleared: %s", got.Status)
	}
}

func TestExpiredTestSkippedWhileBusy(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	x := e.StartTest(t)
	holdBusy(t, e) // a publish is starting: it owns the app's test area
	c.add(3 * time.Hour)
	s.Tick(ctx)
	if got, _ := e.Q.GetDeployment(ctx, x.ID); got.Status != "ready" {
		t.Fatalf("expired test of a busy app was ended: %s", got.Status)
	}
}

func TestEndTestTwiceKeepsNewerTest(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	x := e.StartTest(t)
	if err := e.S.EndTest(ctx, x.ID); err != nil {
		t.Fatal(err)
	}
	y := e.StartTest(t)
	if _, ok := e.S.Target(e.App.ID, true); !ok {
		t.Fatal("test Y has no target")
	}
	if err := e.S.EndTest(ctx, x.ID); err != nil { // X is already retired
		t.Fatal(err)
	}
	if _, ok := e.S.Target(e.App.ID, true); !ok {
		t.Fatal("ending the retired test X took away test Y's target")
	}
	if got, _ := e.Q.GetDeployment(ctx, y.ID); got.Status != "ready" {
		t.Fatalf("Y %s", got.Status)
	}
}

func TestAdoptedStartingAppBecomesRunning(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	if err := e.Q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: e.App.ID, Status: "starting"}); err != nil {
		t.Fatal(err)
	}
	svc2 := e.NewService()
	if err := New(e.Q, e.Eng, svc2).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if st := e.Shared(t).Status; st != "running" {
		t.Fatalf("status %s", st)
	}
}

func TestFailedRestartKeepsTryingThenGivesUp(t *testing.T) {
	ctx := context.Background()
	e, s, _ := live(t)
	starts := func() int {
		n := 0
		for _, c := range e.Eng.CallsCopy() {
			if strings.HasPrefix(c, "start ") {
				n++
			}
		}
		return n
	}
	before := starts()
	e.Eng.Crash(currentContainer(t, e))
	e.Eng.Handler = http500{} // every new container answers 500: the restart fails health
	for i := 1; i <= 3; i++ {
		s.Tick(ctx)
		if st := e.Shared(t).Status; st != "crashed" {
			t.Fatalf("try %d: status %s", i, st)
		}
		if got := starts() - before; got != i {
			t.Fatalf("after tick %d: %d restarts", i, got)
		}
	}
	s.Tick(ctx) // limit reached: gives up, once
	s.Tick(ctx)
	if got := starts() - before; got != 3 {
		t.Fatalf("%d restart tries, want 3", got)
	}
	sa := e.Shared(t)
	if sa.Status != "crashed" || sa.StatusMessage != msgGaveUp {
		t.Fatalf("%s %q", sa.Status, sa.StatusMessage)
	}
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM app_changes WHERE app_id = $1 AND person_id IS NULL
		AND changes::text LIKE '%parou de tentar religar%'`, e.App.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("give-up events: %d", n)
	}
}

func TestOrphanSweepSkipsBusyApp(t *testing.T) {
	ctx := context.Background()
	e, s, _ := live(t)
	holdBusy(t, e)
	id := mustStart(t, e, engine.StartSpec{Name: "apphub-notas-novo",
		Labels: map[string]string{engine.Label: strconv.FormatInt(e.App.ID, 10)}})
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.Eng.Inspect(ctx, id); !st.Running {
		t.Fatal("the container being started by a publish was removed as an orphan")
	}
}

func TestDailyBackupWaitsForBusyApp(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	n := time.Now().AddDate(0, 0, 1)
	c.t = time.Date(n.Year(), n.Month(), n.Day(), 3, 5, 0, 0, time.Local)
	s.Touch(e.App.ID)
	_, release := holdBusy(t, e)
	s.Tick(ctx)
	count := func() int {
		var n int
		pool.QueryRow(ctx, "SELECT count(*) FROM shared_backups WHERE app_id = $1 AND kind = 'daily'", e.App.ID).Scan(&n)
		return n
	}
	if count() != 0 {
		t.Fatal("copied a busy app")
	}
	release() // the publish ends; the app is free again
	s.Tick(ctx)
	s.Tick(ctx)
	if count() != 1 {
		t.Fatalf("daily copies after the app was free: %d", count())
	}
}

func TestReconcileClearsRecentBuilding(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	d, _ := insertBuilding(t, e, "live")
	if err := New(e.Q, e.Eng, e.NewService()).Reconcile(ctx); err != nil { // startup: nothing of this process is running
		t.Fatal(err)
	}
	if got, _ := e.Q.GetDeployment(ctx, d.ID); got.Status != "failed" {
		t.Fatalf("recent leftover at startup: %s", got.Status)
	}
}

func TestLateCleanupOfLostTestKeepsNewerTest(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	x, _ := insertBuilding(t, e, "test") // lost by a restart
	// The supervisor claims it (building -> failed); only then can a new test start.
	if err := e.Q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: x.ID, Status: "failed"}); err != nil {
		t.Fatal(err)
	}
	y := e.StartTest(t)
	if got, _ := e.Q.GetDeployment(ctx, y.ID); got.Status != "ready" {
		t.Fatalf("Y %s", got.Status)
	}
	marker := filepath.Join(e.DataDir(), "apps", deploytest.Itoa(e.App.ID), "test-data", "marker")
	if err := os.WriteFile(marker, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	// ...and X's own cleanup arrives late
	if err := e.S.EndTest(ctx, x.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("the lost test's cleanup wiped test Y's data")
	}
	if _, ok := e.S.Target(e.App.ID, true); !ok {
		t.Fatal("the lost test's cleanup took away test Y's route")
	}
	// ending Y itself still cleans everything
	if err := e.S.EndTest(ctx, y.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("test data kept after ending the current test")
	}
}

func TestCrashedRetryCrossesWindowBoundary(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	e.Eng.Crash(currentContainer(t, e))
	e.Eng.Handler = http500{}
	s.Tick(ctx)
	s.Tick(ctx) // two failed tries inside the window
	c.add(11 * time.Minute)
	s.Tick(ctx) // the window passed: still the same crash, so this is the third try
	sa := e.Shared(t)
	if sa.Status != "crashed" || sa.StatusMessage == msgGaveUp || sa.RestartCount != 3 {
		t.Fatalf("%s %q count %d", sa.Status, sa.StatusMessage, sa.RestartCount)
	}
	s.Tick(ctx)
	if sa := e.Shared(t); sa.StatusMessage != msgGaveUp {
		t.Fatalf("no give-up after the third try: %q", sa.StatusMessage)
	}
}

func TestFailingDailyCopyRetriedOncePerHour(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	n := time.Now().AddDate(0, 0, 1)
	c.t = time.Date(n.Year(), n.Month(), n.Day(), 3, 5, 0, 0, time.Local)
	s.Touch(e.App.ID)
	e.DBs.FailDump = errors.New("sem espaço")
	s.Tick(ctx) // fails
	e.DBs.FailDump = nil
	s.Tick(ctx) // same hour: not tried again
	count := func() int {
		var n int
		pool.QueryRow(ctx, "SELECT count(*) FROM shared_backups WHERE app_id = $1 AND kind = 'daily'", e.App.ID).Scan(&n)
		return n
	}
	if count() != 0 {
		t.Fatal("retried within the hour")
	}
	c.add(61 * time.Minute)
	s.Tick(ctx)
	if count() != 1 {
		t.Fatalf("copies after an hour: %d", count())
	}
}

func TestSlowTicksStillGiveUpAfterThreeTries(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	starts := func() int {
		n := 0
		for _, call := range e.Eng.CallsCopy() {
			if strings.HasPrefix(call, "start ") {
				n++
			}
		}
		return n
	}
	before := starts()
	e.Eng.Crash(currentContainer(t, e))
	e.Eng.Handler = http500{}
	for i := 0; i < 8; i++ { // each tick lands > 200 s after the last (slow failed tries)
		s.Tick(ctx)
		c.add(270 * time.Second)
	}
	if got := starts() - before; got != 3 {
		t.Fatalf("%d restart tries, want 3", got)
	}
	if sa := e.Shared(t); sa.Status != "crashed" || sa.StatusMessage != msgGaveUp {
		t.Fatalf("%s %q", sa.Status, sa.StatusMessage)
	}
}

func countStarts(e *deploytest.Env) int {
	n := 0
	for _, call := range e.Eng.CallsCopy() {
		if strings.HasPrefix(call, "start ") {
			n++
		}
	}
	return n
}

// Ticks far apart (slower than the 10-minute window across tries) must never
// buy a crashing app more than three starts.
func TestWidelySpacedTicksGiveUpAfterThreeTries(t *testing.T) {
	for _, gap := range []time.Duration{301 * time.Second, 700 * time.Second} {
		t.Run(gap.String(), func(t *testing.T) {
			ctx := context.Background()
			e, s, c := live(t)
			before := countStarts(e)
			e.Eng.Crash(currentContainer(t, e))
			e.Eng.Handler = http500{}
			for i := 0; i < 10; i++ {
				s.Tick(ctx)
				c.add(gap)
			}
			if got := countStarts(e) - before; got != 3 {
				t.Fatalf("%d restart tries, want 3", got)
			}
			if sa := e.Shared(t); sa.Status != "crashed" || sa.StatusMessage != msgGaveUp {
				t.Fatalf("%s %q", sa.Status, sa.StatusMessage)
			}
			var n int
			pool.QueryRow(ctx, `SELECT count(*) FROM app_changes WHERE app_id = $1 AND person_id IS NULL
				AND changes::text LIKE '%parou de tentar religar%'`, e.App.ID).Scan(&n)
			if n != 1 {
				t.Fatalf("give-up events: %d", n)
			}
		})
	}
}

// An app that recovered and ran healthy past the window gets a fresh set of
// tries when it goes down again.
func TestRecoveredAppGetsFreshTries(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	e.Eng.Crash(currentContainer(t, e))
	e.Eng.Handler = http500{}
	s.Tick(ctx) // try 1 fails
	c.add(time.Minute)
	s.Tick(ctx) // try 2 fails
	e.Eng.Handler = nil
	c.add(time.Minute)
	s.Tick(ctx) // try 3 succeeds
	if sa := e.Shared(t); sa.Status != "running" || sa.RestartCount != 3 {
		t.Fatalf("%s count %d", sa.Status, sa.RestartCount)
	}
	for i := 0; i < 3; i++ { // healthy for 12 minutes
		c.add(4 * time.Minute)
		s.Touch(e.App.ID)
		s.Tick(ctx)
	}
	if sa := e.Shared(t); sa.Status != "running" {
		t.Fatalf("status %s while healthy", sa.Status)
	}
	before := countStarts(e)
	e.Eng.Handler = http500{}
	e.Eng.SetHandler(currentContainer(t, e), http500{})
	for i := 0; i < 6; i++ {
		s.Tick(ctx)
		c.add(time.Minute)
	}
	if got := countStarts(e) - before; got != 3 {
		t.Fatalf("%d fresh tries, want 3", got)
	}
	if sa := e.Shared(t); sa.Status != "crashed" || sa.StatusMessage != msgGaveUp {
		t.Fatalf("%s %q", sa.Status, sa.StatusMessage)
	}
}

type http500 struct{}

func (http500) ServeHTTP(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }
