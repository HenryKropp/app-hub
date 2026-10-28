package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const isolatedDB = "apphub_workspace_security_test"

func hashForTest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

func testPortal(t *testing.T) (*Portal, *pgxpool.Pool, *httptest.Server) {
	t.Helper()
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		t.Skip("DATABASE_URL not configured")
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dev)
	if err != nil {
		t.Fatal("connect test bootstrap")
	}
	var exists bool
	err = c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, isolatedDB).Scan(&exists)
	if err == nil && !exists {
		_, err = c.Exec(ctx, "CREATE DATABASE "+isolatedDB)
	}
	c.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dev)
	if err != nil {
		t.Fatal("parse test URL")
	}
	u.Path = "/" + isolatedDB
	cfg := config.Config{DatabaseURL: u.String(), BaseURL: "http://localhost"}
	cfg.AgentBinaryPath = filepath.Join(t.TempDir(), "AppHubAgente.exe")
	if err := os.WriteFile(cfg.AgentBinaryPath, []byte("fixture binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal("test pool")
	}
	var actual string
	if err = pool.QueryRow(ctx, `SELECT current_database()`).Scan(&actual); err != nil || actual != isolatedDB {
		t.Fatal("refusing to reset non-test database")
	}
	// This database belongs exclusively to this integration suite.
	rows, err := pool.Query(ctx, `SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'tenant_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var schemas []string
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
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
	p, err := New(ctx, pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(p)
	t.Cleanup(func() { srv.Close(); p.Close(); pool.Close() })
	return p, pool, srv
}

type testBrowser struct {
	t    *testing.T
	c    *http.Client
	base string
}

func browser(t *testing.T, base string) *testBrowser {
	j, _ := cookiejar.New(nil)
	return &testBrowser{t, &http.Client{Jar: j, Timeout: 15 * time.Second}, base}
}
func (b *testBrowser) do(method, path string, body any) (int, []byte) {
	b.t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			b.t.Fatal(err)
		}
	}
	req, _ := http.NewRequest(method, b.base+path, &payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.c.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		b.t.Fatal(err)
	}
	return resp.StatusCode, data
}
func (b *testBrowser) want(method, path string, body any, status int) []byte {
	b.t.Helper()
	got, data := b.do(method, path, body)
	if got != status {
		b.t.Fatalf("%s %s: got %d want %d; %s", method, path, got, status, data)
	}
	return data
}
func scriptBody(name string, code string) map[string]any {
	return map[string]any{"manifest": manifest.Manifest{Name: name, Kind: "script", Entrypoint: "main.py", Area: "Compliance"},
		"files": map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte(code))}}}
}
func importApp(t *testing.T, b *testBrowser, prefix, name string) int64 {
	t.Helper()
	raw := b.want("POST", prefix+"/api/scripts/import", scriptBody(name, "print('ok')\n"), 201)
	var out struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func TestPortalIsolationSecurityLifecycle(t *testing.T) {
	p, pool, srv := testPortal(t)
	b := browser(t, srv.URL)
	credentials := map[string]any{"name": "Test administrator", "email": "admin@example.test", "password": "test-password-not-a-real-secret"}
	b.want("POST", "/api/auth/setup", credentials, 201)
	rootApp := importApp(t, b, "", "Root private app")
	importApp(t, b, "", "Root second app")
	b.want("POST", "/api/workspaces", map[string]any{"name": "Second company", "slug": "second-company", "password": "incorrect"}, 401)
	b.want("POST", "/api/workspaces", map[string]any{"name": "Second company", "slug": "second-company", "password": credentials["password"]}, 201)
	b.want("POST", "/api/workspaces", map[string]any{"name": "Duplicate", "slug": "second-company", "password": credentials["password"]}, 409)
	const prefix = "/w/second-company"
	// A root-company administrator is NOT an authenticated member of another company.
	b.want("GET", prefix+"/api/apps", nil, 401)
	b.want("GET", prefix+"/api/people", nil, 401)
	b.want("GET", "/w/not-a-company/api/apps", nil, 404)
	raw := b.want("GET", prefix+"/api/auth/status", nil, 200)
	if !bytes.Contains(raw, []byte(`"person":null`)) || !bytes.Contains(raw, []byte(`"needs_setup":false`)) {
		t.Fatal("new company is not provisioned independently")
	}
	b.want("POST", prefix+"/api/auth/login", credentials, 200)
	raw = b.want("GET", prefix+"/api/apps", nil, 200)
	if string(bytes.TrimSpace(raw)) != "[]" {
		t.Fatal("root apps leaked into new company")
	}
	tenantApp := importApp(t, b, prefix, "Tenant app")
	if rootApp != tenantApp {
		t.Fatal("test needs overlapping IDs to exercise namespace isolation")
	}
	b.want("GET", prefix+"/api/apps/2", nil, 404)
	raw = b.want("GET", "/api/apps", nil, 200)
	if bytes.Contains(raw, []byte("Tenant app")) || !bytes.Contains(raw, []byte("Root private app")) {
		t.Fatal("cookie paths or data isolation failed")
	}
	// Agent identities and pairing codes cannot cross company boundaries.
	ctx := context.Background()
	token := "test-agent-token"
	var computerID int64
	err := pool.QueryRow(ctx, `INSERT INTO computers(person_id,name,token_hash) VALUES(1,'test agent',$1) RETURNING id`, hashForTest(token)).Scan(&computerID)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", srv.URL+prefix+"/api/agent/next", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatal("agent token crossed companies")
	}
	b.want("POST", prefix+"/api/scan", map[string]string{"root": "/tmp"}, 403)
	b.want("POST", prefix+"/api/apps", map[string]string{}, 403)
	secretBody := scriptBody("Unsafe code", "PASSWORD = 'this-is-a-test-only-password'\n")
	secretBody["confirm_secrets"] = true
	b.want("POST", prefix+"/api/scripts/import", secretBody, 422)
	// Execution approval is mandatory by default and tied to the reviewed source.
	b.want("POST", prefix+"/api/apps/1/runs", map[string]any{"output_folder": `C:\Results`}, 403)
	profile := map[string]any{"classification": "confidential", "purpose": "Internal test processing", "external_domains": []string{"api.example.test"}, "approved": true, "suspended": false}
	b.want("PUT", prefix+"/api/security/apps/1", profile, 200)
	b.want("POST", prefix+"/api/apps/1/runs", map[string]any{"output_folder": `C:\Results`}, 409) // no agent
	tenant := p.entries["second-company"]
	if _, err = tenant.pool.Exec(ctx, `UPDATE apps SET manifest=jsonb_set(manifest,'{summary}','"changed"') WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	b.want("POST", prefix+"/api/apps/1/runs", map[string]any{"output_folder": `C:\Results`}, 403)
	b.want("PUT", prefix+"/api/security/apps/1", profile, 200)
	profile["suspended"] = true
	b.want("PUT", prefix+"/api/security/apps/1", profile, 200)
	b.want("POST", prefix+"/api/apps/1/runs", map[string]any{"output_folder": `C:\Results`}, 403)
	// Create a regular member; company administrators do not leak their privileges.
	b.want("POST", prefix+"/api/people", map[string]any{"name": "Member", "email": "member@example.test", "password": "member-test-password", "role": "usuario"}, 201)
	member := browser(t, srv.URL)
	member.want("POST", prefix+"/api/auth/login", map[string]any{"email": "member@example.test", "password": "member-test-password"}, 200)
	member.want("GET", prefix+"/api/security", nil, 403)
	member.want("GET", prefix+"/api/security/export", nil, 403)
	member.want("PUT", prefix+"/api/security/apps/1", profile, 403)
	member.want("POST", prefix+"/api/workspaces", map[string]string{}, 403)
	// Unknown JSON/body fields never appear in audit; credential values stay out.
	raw = b.want("GET", prefix+"/api/security/export", nil, 200)
	if bytes.Contains(raw, []byte(credentials["password"].(string))) || bytes.Contains(raw, []byte("PASSWORD =")) {
		t.Fatal("sensitive data in security export")
	}
	raw = b.want("GET", prefix+"/api/security/audit", nil, 200)
	if !bytes.Contains(raw, []byte("PUT /api/security/apps/{id}")) {
		t.Fatal("audit route template missing")
	}
	if bytes.Contains(raw, []byte("Internal test processing")) {
		t.Fatal("audit logged request bodies")
	}
	// There is no public-schema fallback even if a tenant table is absent.
	if _, err = tenant.pool.Exec(ctx, `ALTER TABLE people RENAME TO people_hidden`); err != nil {
		t.Fatal(err)
	}
	b.want("GET", prefix+"/api/people", nil, 500)
	if _, err = tenant.pool.Exec(ctx, `ALTER TABLE people_hidden RENAME TO people`); err != nil {
		t.Fatal(err)
	}
	// Logout clears only the company session; the original session remains.
	b.want("POST", prefix+"/api/auth/logout", nil, 204)
	b.want("GET", prefix+"/api/apps", nil, 401)
	b.want("GET", "/api/apps", nil, 200)
	b.want("POST", prefix+"/api/auth/login", credentials, 200)
	// Browser origin defenses reject cross-site state changes.
	req, _ = http.NewRequest("PUT", srv.URL+prefix+"/api/security/settings", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://untrusted.example")
	resp, err = b.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("cross-site mutation accepted")
	}
	for i := 0; i < 21; i++ {
		status, _ := b.do("POST", "/api/auth/login", map[string]string{"email": "no-user@example.test", "password": "invalid"})
		if i == 20 && status != 429 {
			t.Fatal("login attempts not rate limited")
		}
	}
	// Build a second portal over the same database: tenant data survives restarts.
	p2, err := New(ctx, pool, p.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if p2.entries["second-company"] == nil {
		t.Fatal("company missing after restart")
	}
}

func TestPortalSharedRuntimeFailsClosed(t *testing.T) {
	_, pool, srv := testPortal(t)
	b := browser(t, srv.URL)
	credentials := map[string]any{"name": "Test admin", "email": "admin@example.test", "password": "test-password-not-a-real-secret"}
	b.want("POST", "/api/auth/setup", credentials, 201)
	importApp(t, b, "", "Original company app")
	b.want("POST", "/api/workspaces", map[string]any{"name": "Other company", "slug": "other-company", "password": credentials["password"]}, 201)
	b.want("POST", "/w/other-company/api/auth/login", credentials, 200)
	importApp(t, b, "/w/other-company", "Other company app")
	for _, prefix := range []string{"", "/w/other-company"} {
		for _, route := range []string{"/api/server", "/api/apps/1/shared"} {
			data := b.want("GET", prefix+route, nil, 503)
			if !bytes.Contains(data, []byte(`"code":"shared_runtime_unavailable"`)) {
				t.Fatalf("missing explicit runtime capability error for %s", prefix+route)
			}
		}
		b.want("POST", prefix+"/api/apps/1/shared/publish", map[string]any{}, 503)
		b.want("GET", prefix+"/api/shared/authorize?app=notas", nil, 503)
		req, _ := http.NewRequest("GET", srv.URL+prefix+"/api/shared/authorize?app=notas", nil)
		req.Header.Set("Accept", "text/html")
		resp, err := b.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 503 || !bytes.Contains(data, []byte(`href="`+prefix+`/"`)) {
			t.Fatal("unavailable page lost its company home link")
		}
	}
	var jobs int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM shared_deployments`).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("disabled shared runtime created deployments: count=%d err=%v", jobs, err)
	}
}
