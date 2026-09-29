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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		panic("DATABASE_URL must be set to run handler tests (load .env)")
	}
	ctx := context.Background()
	var err error
	testPool, err = pgxpool.New(ctx, url)
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
	if _, err := tx.Exec(ctx, "TRUNCATE apps, sessions, people RESTART IDENTITY CASCADE"); err != nil {
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
