package supervisor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/sharedtest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/tenant"
)

var (
	companyA = tenant.Tenant{Slug: "a", Key: "wa0000000000"}
	companyB = tenant.Tenant{Slug: "b", Key: "wb0000000000"}
)

type fakeHolder struct {
	mu    sync.Mutex
	calls []string
}

func (h *fakeHolder) Hold()    { h.mu.Lock(); h.calls = append(h.calls, "hold"); h.mu.Unlock() }
func (h *fakeHolder) Release() { h.mu.Lock(); h.calls = append(h.calls, "release"); h.mu.Unlock() }
func (h *fakeHolder) got() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.calls...)
}

func sameCalls(a []string, b ...string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var poolB *pgxpool.Pool

// emptyCompany is a supervisor on a second, empty database (a company with
// no shared apps), seeing the engine through the given company key.
func emptyCompany(t *testing.T, e *deploytest.Env, t2 tenant.Tenant) *Supervisor {
	t.Helper()
	ctx := context.Background()
	if poolB == nil {
		p, err := sharedtest.Pool(ctx, "apphub_test_supervisor_b")
		if err != nil {
			t.Fatal(err)
		}
		poolB = p
	}
	if _, err := poolB.Exec(ctx, sharedtest.Reset); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(poolB)
	eng := engine.Scoped(e.Eng, t2.Key)
	dbs := appdb.NewFake()
	dbs.Prefix = t2.DBPrefix()
	svc := deploy.New(q, eng, dbs, e.Vault(), deploy.Options{DataDir: t2.DataDir(e.DataDir()),
		Domain: t2.AppsDomain("localhost:8080"), Scheme: "http", HealthTimeout: time.Second, Tenant: t2})
	return New(q, eng, svc)
}

// companyLive is company A with its app published, and A's supervisor.
func companyLive(t *testing.T) (*deploytest.Env, *Supervisor) {
	t.Helper()
	e := deploytest.Setup(t, pool)
	e.S = e.NewTenantService(companyA, "", 0, 0)
	if d := e.Publish(t); d.Status != "live" {
		t.Fatalf("%s: %s", d.Status, d.ErrorMessage)
	}
	return e, New(e.Q, engine.Scoped(e.Eng, companyA.Key), e.S)
}

func TestGroupReconcileDoesNotTouchOtherCompany(t *testing.T) {
	ctx := context.Background()
	e, sA := companyLive(t)
	before := e.Eng.Running()
	g := NewGroup(e.Eng)
	g.Add(sA, &fakeHolder{}, nil)
	g.Add(emptyCompany(t, e, companyB), &fakeHolder{}, nil)
	g.Add(emptyCompany(t, e, tenant.Principal), &fakeHolder{}, nil)
	g.reconcileQueued(ctx)
	g.tick(ctx)
	if got := e.Eng.Running(); len(got) != 1 || len(before) != 1 || got[0] != before[0] {
		t.Fatalf("before %v, after %v", before, got)
	}
}

func TestGroupHoldsUntilReconciled(t *testing.T) {
	ctx := context.Background()
	e, sA := companyLive(t)
	g := NewGroup(e.Eng)
	h := &fakeHolder{}
	g.Add(sA, h, nil)
	if !sameCalls(h.got(), "hold") {
		t.Fatal(h.got())
	}
	g.reconcileQueued(ctx)
	if !sameCalls(h.got(), "hold", "release") {
		t.Fatal(h.got())
	}

	e.Eng.SetDown(true)
	down := &fakeHolder{}
	g.Add(New(e.Q, engine.Scoped(e.Eng, companyA.Key), e.S), down, nil)
	g.reconcileQueued(ctx)
	if !sameCalls(down.got(), "hold", "release") {
		t.Fatal("engine down:", down.got())
	}
}

func TestGroupSkipsCompaniesWithoutApps(t *testing.T) {
	ctx := context.Background()
	e, sA := companyLive(t)
	sB := emptyCompany(t, e, companyB)
	var stepsA, stepsB int
	sA.beforeStep = func() { stepsA++ }
	sB.beforeStep = func() { stepsB++ }
	g := NewGroup(e.Eng)
	g.Add(sA, &fakeHolder{}, nil)
	g.Add(sB, &fakeHolder{}, nil)
	g.reconcileQueued(ctx)
	g.tick(ctx)
	if stepsA != 1 || stepsB != 0 {
		t.Fatalf("A %d, B %d", stepsA, stepsB)
	}
}

func TestGroupAddAfterStart(t *testing.T) {
	e, sA := companyLive(t)
	g := NewGroup(e.Eng)
	g.Interval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { g.Loop(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	time.Sleep(20 * time.Millisecond)
	h := &fakeHolder{}
	g.Add(sA, h, nil)
	for i := 0; i < 200; i++ {
		if sameCalls(h.got(), "hold", "release") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(h.got())
}

func TestGroupSleepsOffCompany(t *testing.T) {
	ctx := context.Background()
	e, sA := companyLive(t)
	if d := e.StartTest(t); d.Status != "ready" {
		t.Fatalf("test box: %s %s", d.Status, d.ErrorMessage)
	}
	steps := 0
	sA.beforeStep = func() { steps++ }
	g := NewGroup(e.Eng)
	g.Add(sA, &fakeHolder{}, func(context.Context) bool { return false })
	g.reconcileQueued(ctx)
	g.tick(ctx)
	if st := e.Shared(t).Status; st != "sleeping" {
		t.Fatal(st)
	}
	if steps != 0 {
		t.Fatal("Step ran for a company that is off")
	}
	if got := e.Eng.Running(); len(got) != 0 {
		t.Fatal(got)
	}
}
