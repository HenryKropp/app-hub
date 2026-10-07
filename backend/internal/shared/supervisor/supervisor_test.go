package supervisor

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
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
func holdBusy(t *testing.T, e *deploytest.Env) sqlc.SharedDeployment {
	t.Helper()
	e.S.SetSpawn(func(func()) {})
	d, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !e.S.Busy(e.App.ID) {
		t.Fatal("app should be busy")
	}
	return d
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
	pool.Exec(ctx, "UPDATE shared_apps SET always_on = true WHERE app_id = $1", e.App.ID)
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
	orphan, _ := e.Eng.Start(ctx, engine.StartSpec{Name: "apphub-velho-1", Labels: map[string]string{engine.Label: "99"}})
	foreign, _ := e.Eng.Start(ctx, engine.StartSpec{Name: "sistema-alis-db"}) // não é do Hub
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
	cid, _ := e.Eng.Start(ctx, engine.StartSpec{Name: "apphub-notas-" + strconv.FormatInt(d.ID, 10),
		Labels: map[string]string{engine.Label: strconv.FormatInt(e.App.ID, 10), "app-hub.deployment": strconv.FormatInt(d.ID, 10)}})
	return d, cid
}

func TestReconcileClearsStaleBuildingOfLiveApp(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	d, cid := insertBuilding(t, e, "live")
	svc2 := e.NewService()
	if err := New(e.Q, e.Eng, svc2).Reconcile(ctx); err != nil {
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
	if err := New(e.Q, e.Eng, e.NewService()).Reconcile(ctx); err != nil {
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
	if err := New(e.Q, e.Eng, e.NewService()).Reconcile(ctx); err != nil {
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
	d := holdBusy(t, e) // a publish in progress in this Hub: "building" is real
	// it would be crashed (3 restarts used) if the supervisor looked at it
	pool.Exec(ctx, "UPDATE shared_apps SET restart_count = 3, restart_window_start = $2 WHERE app_id = $1", e.App.ID, c.t)
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

type http500 struct{}

func (http500) ServeHTTP(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }
