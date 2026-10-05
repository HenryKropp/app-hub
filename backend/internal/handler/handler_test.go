package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

var testPool *pgxpool.Pool

// testDatabaseName is where handler tests run, next to the development database.
const testDatabaseName = "apphub_test"

func TestMain(m *testing.M) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		panic("DATABASE_URL must be set to run handler tests (load .env)")
	}
	ctx := context.Background()
	testURL, err := ensureTestDatabase(ctx, url)
	if err != nil {
		panic(err)
	}
	testPool, err = pgxpool.New(ctx, testURL)
	if err != nil {
		panic(err)
	}
	if err := database.Migrate(ctx, testPool); err != nil {
		panic(err)
	}
	code := m.Run()
	testPool.Close()
	os.Exit(code)
}

// client is an HTTP client with a cookie jar talking to a test server whose
// database work happens inside a transaction that is rolled back afterwards.
// resetSQL empties the Hub and puts back the built-in roles and two areas
// (Research = 1, Investimentos = 2), so every test starts the same way.
const resetSQL = `
TRUNCATE run_files, agent_jobs, runs, app_changes, app_versions, code_blobs, pairings, computers,
         app_grants, app_viewers, area_grants, area_leaders, area_members, apps, sessions, people, areas
         RESTART IDENTITY CASCADE;
DELETE FROM roles WHERE key IS NULL;
UPDATE roles SET name = 'Administrador', permissions = ARRAY['register_apps','edit_any_app','restore_any_app','see_tech_any_app','see_all_runs','see_all_apps','manage_people','manage_areas','manage_roles'] WHERE key = 'admin';
UPDATE roles SET name = 'Criador', permissions = ARRAY['register_apps'] WHERE key = 'criador';
UPDATE roles SET name = 'Usuário', permissions = ARRAY[]::text[] WHERE key = 'usuario';
INSERT INTO areas (name) VALUES ('Research'), ('Investimentos');
`

type client struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
}

func newClient(t *testing.T, devMode bool) *client {
	t.Helper()
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Start from an empty Hub inside the transaction.
	if _, err := tx.Exec(ctx, resetSQL); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	New(sqlc.New(tx), config.Config{DevMode: devMode}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
		_ = tx.Rollback(ctx)
	})
	jar, _ := cookiejar.New(nil)
	return &client{t: t, srv: srv, c: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, c.srv.URL+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

// fresh returns a second client (new cookie jar) against the same server.
func (c *client) fresh() *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: c.t, srv: c.srv, c: &http.Client{Jar: jar}}
}

func setupAdmin(t *testing.T, c *client) personJSON {
	t.Helper()
	var p personJSON
	if code := c.do("POST", "/api/auth/setup", map[string]string{"name": "Henry", "email": "Henry@Alis.com", "password": "senha-forte-1"}, &p); code != http.StatusCreated {
		t.Fatalf("setup status %d", code)
	}
	return p
}

// newPoolClient talks to a Handler backed by the shared pool (not a single
// transaction), for tests with concurrent requests (long-poll, downloads).
// Tables are emptied before and after.
func newPoolClient(t *testing.T) (*client, *Handler) {
	t.Helper()
	ctx := context.Background()
	truncate := func() {
		if _, err := testPool.Exec(ctx, resetSQL); err != nil {
			t.Fatal(err)
		}
	}
	truncate()
	h := New(sqlc.New(testPool), config.Config{BaseURL: "http://hub.test"})
	h.waitNext, h.waitFile, h.waitFolder, h.tick = 400*time.Millisecond, 2*time.Second, 2*time.Second, 20*time.Millisecond
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
		truncate()
	})
	jar, _ := cookiejar.New(nil)
	return &client{t: t, srv: srv, c: &http.Client{Jar: jar}}, h
}

// agent sends a request as an agent (Bearer token, no cookies).
func (c *client) agent(method, path, token string, body any, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, c.srv.URL+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}
