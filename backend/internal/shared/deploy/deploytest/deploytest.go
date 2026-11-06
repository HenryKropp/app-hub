// Package deploytest prepares a shared app on the shared test database,
// with the fake engine and fake databases, for deploy and supervisor tests.
package deploytest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/sharedtest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/tenant"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

type Env struct {
	S   *deploy.Service
	Q   *sqlc.Queries
	Eng *engine.Fake
	DBs *appdb.Fake
	App sqlc.App
	M   manifest.Manifest

	key     string
	dataDir string
	pool    *pgxpool.Pool
}

// Pool opens the test database called name (each test package uses its own:
// "apphub_test_deploy", "apphub_test_supervisor"). nil, nil without DATABASE_URL.
func Pool(ctx context.Context, name string) (*pgxpool.Pool, error) {
	return sharedtest.Pool(ctx, name)
}

// Testdata finds backend/testdata/shared/<name> from this file, wherever the test runs.
func Testdata(name string) string { return testdata(name) }

// testdata finds backend/testdata/shared from this file, wherever the test runs.
func testdata(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "../../../../testdata/shared", name)
}

// Setup resets the test database and registers a Streamlit app (with a
// database) whose folder is a copy of testdata/shared/streamlit.
func Setup(t *testing.T, pool *pgxpool.Pool) *Env {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, sharedtest.Reset); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	CopyDir(t, testdata("streamlit"), dir)
	m := manifest.Manifest{ManifestVersion: 1, ID: "notas", Name: "Notas", Kind: manifest.KindWeb, Entrypoint: "app.py",
		Shared: &manifest.Shared{Enabled: true, Database: "postgres"}}
	raw, _ := json.Marshal(m)
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO apps (slug, name, kind, area, area_id, folder_path, owner_id, manifest, created_by)
		VALUES ('notas', 'Notas', 'web', 'Research', 1, $1, 1, $2, 1) RETURNING id`, dir, raw).Scan(&id); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(pool)
	app, err := q.GetApp(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	e := &Env{Q: q, Eng: engine.NewFake(), DBs: appdb.NewFake(), App: app, M: m, key: vault.NewKey(), dataDir: t.TempDir(), pool: pool}
	t.Cleanup(e.Eng.Close)
	e.S = e.NewService()
	if _, err := e.S.Configure(ctx, app, m); err != nil {
		t.Fatal(err)
	}
	return e
}

// NewService is a fresh service on the same database, engine, databases and
// vault key, with nothing in memory: what the Hub looks like after a restart.
func (e *Env) NewService() *deploy.Service {
	v, _ := vault.New(e.key)
	return deploy.New(e.Q, e.Eng, e.DBs, v, deploy.Options{DataDir: e.dataDir, Domain: "localhost:8080", Scheme: "http", HealthTimeout: time.Second})
}

// NewServiceOpts is NewService with extra options; the data folder, domain,
// scheme and health wait are filled in when left empty.
func (e *Env) NewServiceOpts(opt deploy.Options) *deploy.Service {
	v, _ := vault.New(e.key)
	if opt.DataDir == "" {
		opt.DataDir = e.dataDir
	}
	if opt.Domain == "" {
		opt.Domain, opt.Scheme = "localhost:8080", "http"
	}
	if opt.HealthTimeout == 0 {
		opt.HealthTimeout = time.Second
	}
	return deploy.New(e.Q, e.Eng, e.DBs, v, opt)
}

// NewTenantService is a service for a company: its engine view is scoped to
// the company's key, its databases carry the company prefix, and its data
// folder is the company's.
func (e *Env) NewTenantService(t tenant.Tenant, folderRoot string, maxApps, maxMemMB int) *deploy.Service {
	v, _ := vault.New(e.key)
	dbs := appdb.NewFake()
	dbs.Prefix = t.DBPrefix()
	return deploy.New(e.Q, engine.Scoped(e.Eng, t.Key), dbs, v, deploy.Options{DataDir: t.DataDir(e.dataDir),
		Domain: t.AppsDomain("localhost:8080"), Scheme: "http", HealthTimeout: time.Second,
		Tenant: t, FolderRoot: folderRoot, MaxApps: maxApps, MaxMemoryMB: maxMemMB})
}

// NewTenantServiceSlots is a company service (no folder or app limits) that
// shares the server-wide running limit slots.
func (e *Env) NewTenantServiceSlots(t tenant.Tenant, slots *deploy.Slots) *deploy.Service {
	v, _ := vault.New(e.key)
	dbs := appdb.NewFake()
	dbs.Prefix = t.DBPrefix()
	return deploy.New(e.Q, engine.Scoped(e.Eng, t.Key), dbs, v, deploy.Options{DataDir: t.DataDir(e.dataDir),
		Domain: t.AppsDomain("localhost:8080"), Scheme: "http", HealthTimeout: time.Second, Tenant: t, Slots: slots})
}

// AddApp registers another Streamlit app (a copy of the testdata) with this slug.
func (e *Env) AddApp(t *testing.T, slug string) sqlc.App {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	CopyDir(t, testdata("streamlit"), dir)
	raw, _ := json.Marshal(e.M)
	var id int64
	if err := e.pool.QueryRow(ctx, `INSERT INTO apps (slug, name, kind, area, area_id, folder_path, owner_id, manifest, created_by)
		VALUES ($1, $1, 'web', 'Research', 1, $2, 1, $3, 1) RETURNING id`, slug, dir, raw).Scan(&id); err != nil {
		t.Fatal(err)
	}
	app, err := e.Q.GetApp(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

// NewServiceWithoutVault is the Hub started without HUB_SECRET_KEY.
func (e *Env) NewServiceWithoutVault() *deploy.Service {
	return deploy.New(e.Q, e.Eng, e.DBs, nil, deploy.Options{DataDir: e.dataDir, Domain: "localhost:8080", Scheme: "http", HealthTimeout: time.Second})
}

func CopyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// WaitDeployment polls until the deployment leaves "building" and the
// service has let go of the app (so the next operation is not busy).
func (e *Env) WaitDeployment(t *testing.T, id int64) sqlc.SharedDeployment {
	t.Helper()
	for i := 0; i < 200; i++ {
		d, err := e.Q.GetDeployment(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if d.Status != "building" && !e.S.Busy(d.AppID) {
			return d
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("deployment never finished")
	return sqlc.SharedDeployment{}
}

func (e *Env) Shared(t *testing.T) sqlc.SharedApp {
	t.Helper()
	sa, err := e.Q.GetSharedApp(context.Background(), e.App.ID)
	if err != nil {
		t.Fatal(err)
	}
	return sa
}

func (e *Env) Publish(t *testing.T) sqlc.SharedDeployment {
	t.Helper()
	d, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	return e.WaitDeployment(t, d.ID)
}

func (e *Env) StartTest(t *testing.T) sqlc.SharedDeployment {
	t.Helper()
	d, err := e.S.Test(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	return e.WaitDeployment(t, d.ID)
}

func Itoa(n int64) string { return strconv.FormatInt(n, 10) }

// DataDir is the service's SHARED_DATA_DIR.
func (e *Env) DataDir() string { return e.dataDir }

// Vault opens the same key the service uses (to seal settings in tests).
func (e *Env) Vault() *vault.Vault {
	v, _ := vault.New(e.key)
	return v
}
