package sharedrun

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/handler"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/shared/supervisor"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/HenryKropp/app-hub/backend/internal/workspace"
)

const testDB = "apphub_test_sharedrun"
const setupCode = "test-setup-code"

// freshDB empties the sharedrun test database (its own, never the dev one)
// and returns its pool and config.
func freshDB(t *testing.T) (*pgxpool.Pool, config.Config) {
	t.Helper()
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		t.Skip("DATABASE_URL not configured")
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dev)
	if err != nil {
		t.Fatal(err)
	}
	var exists bool
	err = c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, testDB).Scan(&exists)
	if err == nil && !exists {
		_, err = c.Exec(ctx, "CREATE DATABASE "+testDB)
	}
	c.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dev)
	u.Path = "/" + testDB
	cfg := config.Config{DatabaseURL: u.String(), BaseURL: "http://localhost", SetupCode: setupCode, CompanyLimit: 20, CompanySignupsPerDay: 10}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	var actual string
	if err = pool.QueryRow(ctx, `SELECT current_database()`).Scan(&actual); err != nil || actual != testDB {
		t.Fatal("refusing to reset a database that is not the sharedrun test database")
	}
	rows, err := pool.Query(ctx, `SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'tenant_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var schemas []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		schemas = append(schemas, s)
	}
	rows.Close()
	for _, s := range schemas {
		if _, err = pool.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{s}.Sanitize()+" CASCADE"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, cfg
}

type client struct {
	t    *testing.T
	c    *http.Client
	base string
}

func (c *client) want(method, path string, body any, status int) []byte {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, c.base+path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status {
		c.t.Fatalf("%s %s: got %d want %d; %s", method, path, resp.StatusCode, status, data)
	}
	return data
}

// insertWebApp registers app "notas" (id 1 in a fresh company) whose folder
// is a copy of the streamlit testdata at dir.
func insertWebApp(t *testing.T, pool *pgxpool.Pool, dir string) int64 {
	t.Helper()
	ctx := context.Background()
	deploytest.CopyDir(t, deploytest.Testdata("streamlit"), dir)
	m := manifest.Manifest{ManifestVersion: 1, ID: "notas", Name: "Notas", Kind: manifest.KindWeb, Entrypoint: "app.py", Area: "Research",
		Shared: &manifest.Shared{Enabled: true, Database: "postgres", MemoryMB: 256}}
	raw, _ := json.Marshal(m)
	var areaID, personID, id int64
	if err := pool.QueryRow(ctx, `INSERT INTO areas (name) VALUES ('Research') ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`).Scan(&areaID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM people ORDER BY id LIMIT 1`).Scan(&personID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO apps (slug, name, kind, area, area_id, folder_path, owner_id, manifest, created_by)
		VALUES ('notas', 'Notas', 'web', 'Research', $1, $2, $3, $4, $3) RETURNING id`, areaID, dir, personID, raw).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTwoCompaniesSameAppID(t *testing.T) {
	ctx := context.Background()
	pool, cfg := freshDB(t)
	fake := engine.NewFake()
	t.Cleanup(fake.Close)
	v, _ := vault.New(vault.NewKey())
	root := t.TempDir()
	router := gateway.NewRouter("localhost:8080")
	d := Deps{Engine: fake, NewDBs: func(p string) appdb.DBs { f := appdb.NewFake(); f.Prefix = p; return f },
		Router: router, Group: supervisor.NewGroup(fake), Vault: v, DataRoot: root, Domain: "localhost:8080", Scheme: "http",
		MaxApps: 2, MaxMemoryMB: 256, HealthTimeout: time.Second}
	companies := map[string]workspace.Company{}
	services := map[string]*deploy.Service{}
	portal, err := workspace.New(ctx, pool, cfg, workspace.WithShared(func(c workspace.Company, h *handler.Handler) {
		companies[c.Tenant.Slug] = c
		services[c.Tenant.Slug] = d.attach(c, h)
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(portal.Close)
	d.Enabled = portal.SharedEnabled // what main does once the portal exists
	loopCtx, stopLoop := context.WithCancel(ctx)
	looped := make(chan struct{})
	go func() { d.Group.Loop(loopCtx); close(looped) }()
	t.Cleanup(func() { stopLoop(); <-looped })
	srv := httptest.NewServer(portal)
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	b := &client{t: t, c: &http.Client{Jar: jar, Timeout: 15 * time.Second}, base: srv.URL}
	admin := map[string]any{"name": "Admin", "email": "admin@example.test", "password": "test-password-not-a-real-secret", "setup_code": setupCode}
	b.want("POST", "/api/auth/setup", admin, 201)
	b.want("POST", "/api/workspaces", map[string]any{"name": "Teste", "slug": "teste", "password": admin["password"]}, 201)
	b.want("POST", "/w/teste/api/auth/login", map[string]any{"email": admin["email"], "password": admin["password"]}, 200)
	teste := companies["teste"]
	key := teste.Tenant.Key
	if key == "" {
		t.Fatal("teste has no key")
	}

	idP := insertWebApp(t, pool, t.TempDir())
	idT := insertWebApp(t, teste.Pool, filepath.Join(root, "workspaces", key, "code", "1"))
	if idP != 1 || idT != 1 {
		t.Fatalf("app ids %d %d", idP, idT)
	}
	for slug, c := range map[string]workspace.Company{"principal": companies["principal"], "teste": teste} {
		app, err := sqlcApp(ctx, c.Pool, 1)
		if err != nil {
			t.Fatal(err)
		}
		var m manifest.Manifest
		_ = json.Unmarshal(app.Manifest, &m)
		if _, err := services[slug].Configure(ctx, app, m); err != nil {
			t.Fatal(slug, err)
		}
	}
	b.want("PUT", "/api/admin/companies/teste/shared", map[string]bool{"on": true}, 200) // a new company starts off
	// Publishing needs the app approved, like running it.
	approve := map[string]any{"classification": "internal", "purpose": "Notas do time", "approved": true}
	b.want("PUT", "/api/security/apps/1", approve, 200)
	b.want("PUT", "/w/teste/api/security/apps/1", approve, 200)
	b.want("POST", "/api/apps/1/shared/publish", nil, 202)
	b.want("POST", "/w/teste/api/apps/1/shared/publish", nil, 202)
	d1 := waitLive(t, pool)
	d2 := waitLive(t, teste.Pool)

	running := map[string]bool{}
	for _, n := range fake.Running() {
		running[n] = true
	}
	if len(running) != 2 || !running["ziv-notas-"+strconv.FormatInt(d1, 10)] || !running["ziv-"+key+"-notas-"+strconv.FormatInt(d2, 10)] {
		t.Fatal(fake.Running())
	}
	for _, c := range fake.Containers { // the company's boxes carry its key; principal's carry none
		want := ""
		if c.Spec.Name == "ziv-"+key+"-notas-"+strconv.FormatInt(d2, 10) {
			want = key
		}
		if got := c.Spec.Labels[engine.WorkspaceLabel]; got != want {
			t.Fatalf("%s: workspace label %q, want %q", c.Spec.Name, got, want)
		}
	}
	if !fake.HasNetwork("ziv-apps-1") || !fake.HasNetwork("ziv-apps-"+key+"-1") {
		t.Fatal("networks")
	}
	if n := dbName(t, pool); n != "app_1" {
		t.Fatal(n)
	}
	if n := dbName(t, teste.Pool); n != "app_"+key+"_1" {
		t.Fatal(n)
	}

	// Each address reaches its own company's gateway.
	root2 := router.Root(portal)
	for host, want := range map[string]string{"notas.localhost:8080": "http://localhost/api/shared/authorize", "notas.teste.localhost:8080": "http://localhost/w/teste/api/shared/authorize"} {
		r := httptest.NewRequest("GET", "http://"+host+"/", nil)
		r.Host = host
		r.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()
		root2.ServeHTTP(w, r)
		if loc := w.Header().Get("Location"); w.Code != http.StatusFound || len(loc) < len(want) || loc[:len(want)] != want {
			t.Fatalf("%s: %d %s", host, w.Code, loc)
		}
	}

	// Backups land in each company's own folder.
	b.want("POST", "/api/apps/1/shared/backups", nil, 201)
	b.want("POST", "/w/teste/api/apps/1/shared/backups", nil, 201)
	for _, dir := range []string{filepath.Join(root, "backups", "1"), filepath.Join(root, "workspaces", key, "backups", "1")} {
		if entries, err := os.ReadDir(dir); err != nil || len(entries) == 0 {
			t.Fatalf("no backup in %s: %v", dir, err)
		}
	}

	// Stopping in teste leaves the principal company's box running.
	b.want("POST", "/w/teste/api/apps/1/shared/stop", nil, 200)
	if got := fake.Running(); len(got) != 1 || got[0] != "ziv-notas-"+strconv.FormatInt(d1, 10) {
		t.Fatal(got)
	}
}

func dbName(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var n string
	if err := pool.QueryRow(context.Background(), `SELECT db_name FROM shared_apps WHERE app_id = 1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// waitLive waits for app 1's publish and returns the live deployment id.
func waitLive(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	for i := 0; i < 200; i++ {
		var id int64
		var status, msg string
		err := pool.QueryRow(context.Background(), `SELECT id, status, error_message FROM shared_deployments WHERE app_id = 1 ORDER BY id DESC LIMIT 1`).Scan(&id, &status, &msg)
		if err == nil && status == "live" {
			var running string
			_ = pool.QueryRow(context.Background(), `SELECT status FROM shared_apps WHERE app_id = 1`).Scan(&running)
			if running == "running" {
				return id
			}
		}
		if err == nil && status == "failed" {
			t.Fatalf("publish failed: %s", msg)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("publish never finished")
	return 0
}

func sqlcApp(ctx context.Context, pool *pgxpool.Pool, id int64) (sqlc.App, error) {
	return sqlc.New(pool).GetApp(ctx, id)
}
