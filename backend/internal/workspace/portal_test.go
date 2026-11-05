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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/handler"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
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
	cfg := config.Config{DatabaseURL: u.String(), BaseURL: "http://localhost", SetupCode: testSetupCode}
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

const testSetupCode = "test-setup-code"

func (b *testBrowser) do(method, path string, body any) (int, []byte) {
	b.t.Helper()
	if path == "/api/auth/setup" {
		fields := map[string]any{}
		raw, _ := json.Marshal(body)
		_ = json.Unmarshal(raw, &fields)
		if _, ok := fields["setup_code"]; !ok {
			fields["setup_code"] = testSetupCode
		}
		body = fields
	}
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
	// New companies start with approval off; this test exercises it on.
	b.want("PUT", prefix+"/api/security/settings", map[string]any{"retention_days": 30, "require_app_approval": true, "block_embedded_secrets": true, "local_execution_enabled": true}, 200)
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

// The shared-app runtime is plugged into the main company only, never into
// a company created by signup.
func TestPortalPrincipalHookSkipsOtherCompanies(t *testing.T) {
	p, pool, srv := testPortal(t)
	b := browser(t, srv.URL)
	credentials := map[string]any{"name": "Test admin", "email": "admin@example.test", "password": "test-password-not-a-real-secret"}
	b.want("POST", "/api/auth/setup", credentials, 201)
	b.want("POST", "/api/workspaces", map[string]any{"name": "Other company", "slug": "other-company", "password": credentials["password"]}, 201)
	calls := 0
	p2, err := New(context.Background(), pool, p.cfg, WithPrincipal(func(*handler.Handler) { calls++ }))
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if len(p2.entries) < 2 || calls != 1 {
		t.Fatalf("principal hook ran %d times for %d companies", calls, len(p2.entries))
	}
}

// A company created through signup gets every table and column the main
// company has. Migration 006 once checked for runs.dismissed_at in any schema,
// so new companies never got it and their active-runs strip failed.
func TestNewCompanySchemaMatchesMain(t *testing.T) {
	p, pool, _ := testPortal(t)
	ctx := context.Background()
	if _, err := p.create(ctx, "Horizonte", "horizonte", sqlc.Person{Name: "Ana", Email: "ana@example.com", PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := pool.QueryRow(ctx, `SELECT schema_name FROM public.workspace_registry WHERE slug='horizonte'`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	var missing []string
	rows, err := pool.Query(ctx, `SELECT m.table_name||'.'||m.column_name FROM information_schema.columns m
		WHERE m.table_schema='public' AND m.table_name IN (SELECT table_name FROM information_schema.tables WHERE table_schema=$1)
		AND NOT EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema=$1 AND c.table_name=m.table_name AND c.column_name=m.column_name)
		ORDER BY 1`, schema)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			t.Fatal(err)
		}
		missing = append(missing, col)
	}
	rows.Close()
	if len(missing) > 0 {
		t.Fatalf("new company is missing columns the main company has: %v", missing)
	}
}

// Companies created before the fix are repaired the next time the portal starts.
func TestPortalStartRepairsMissingDismissedAt(t *testing.T) {
	p, pool, _ := testPortal(t)
	ctx := context.Background()
	if _, err := p.create(ctx, "Horizonte", "horizonte", sqlc.Person{Name: "Ana", Email: "ana@example.com", PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := pool.QueryRow(ctx, `SELECT schema_name FROM public.workspace_registry WHERE slug='horizonte'`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	s := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, `ALTER TABLE `+s+`.runs DROP COLUMN IF EXISTS dismissed_at; DELETE FROM `+s+`.schema_migrations WHERE name LIKE '%014_%'`); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(ctx, pool, p.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	var ok bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='runs' AND column_name='dismissed_at')`, schema).Scan(&ok); err != nil || !ok {
		t.Fatalf("dismissed_at not repaired on start (err=%v)", err)
	}
}

// Wrong passwords lock only the account being guessed; people behind the same
// proxy (one network address) keep signing in, and successful logins never count.
func TestLoginLimitIsPerAccount(t *testing.T) {
	_, _, srv := testPortal(t)
	b := browser(t, srv.URL)
	credentials := map[string]any{"name": "Test admin", "email": "admin@example.test", "password": "test-password-not-a-real-secret"}
	b.want("POST", "/api/auth/setup", credentials, 201)
	for i := 0; i < 25; i++ {
		b.want("POST", "/api/auth/login", credentials, 200)
	}
	for i := 0; i < 10; i++ {
		b.want("POST", "/api/auth/login", map[string]string{"email": "guessed@example.test", "password": "wrong"}, 401)
	}
	b.want("POST", "/api/auth/login", map[string]string{"email": "Guessed@example.test", "password": "wrong"}, 429)
	b.want("POST", "/api/auth/login", credentials, 200)
}

// Apps a company already used before approvals existed stay runnable; an app
// a reviewer suspended keeps its decision.
func TestExistingAppsApprovedOnce(t *testing.T) {
	_, pool, srv := testPortal(t)
	ctx := context.Background()
	b := browser(t, srv.URL)
	b.want("POST", "/api/auth/setup", map[string]any{"name": "Test admin", "email": "admin@example.test", "password": "test-password-not-a-real-secret"}, 201)
	used := importApp(t, b, "", "Already used app")
	suspended := importApp(t, b, "", "Suspended app")
	for _, sql := range []string{`DELETE FROM app_security`, `INSERT INTO app_security(app_id,suspended,reviewed_at) VALUES(` + strconv.FormatInt(suspended, 10) + `,TRUE,now())`, `DELETE FROM schema_migrations WHERE name LIKE '%015_%'`} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	// The main company migrates in main.go, before the portal starts.
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int64]bool{used: true, suspended: false} {
		var approved bool
		if err := pool.QueryRow(ctx, `SELECT s.approved AND s.reviewed_manifest_hash=encode(sha256(convert_to(a.manifest::text,'UTF8')),'hex') FROM app_security s JOIN apps a ON a.id=s.app_id WHERE a.id=$1`, id).Scan(&approved); err != nil || approved != want {
			t.Fatalf("app %d approved=%v want %v (err=%v)", id, approved, want, err)
		}
	}
}

// A company created by another server process answers here without a restart.
func TestPortalLoadsCompanyCreatedElsewhere(t *testing.T) {
	p, pool, _ := testPortal(t)
	other, err := New(context.Background(), pool, p.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.create(context.Background(), "Horizonte", "horizonte", sqlc.Person{Name: "Ana", Email: "ana@example.com", PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(p)
	defer srv.Close()
	browser(t, srv.URL).want("GET", "/w/horizonte/api/workspace", nil, 200)
	browser(t, srv.URL).want("GET", "/w/nobody-here/api/workspace", nil, 404)
}

// Anonymous calls outside the entry routes do not write audit rows.
func TestAnonymousCallsDoNotFloodAudit(t *testing.T) {
	_, pool, srv := testPortal(t)
	b := browser(t, srv.URL)
	var before, after int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM security_audit`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		b.do("POST", "/api/apps/1/run", map[string]string{})
		b.do("POST", "/api/no-such-route", map[string]string{})
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM security_audit`).Scan(&after); err != nil || after != before {
		t.Fatalf("anonymous calls wrote %d audit rows (err=%v)", after-before, err)
	}
}

// Artifacts need no approval, and a new company starts with app approval off.
func TestNewCompanyStartsWithoutApprovalRequired(t *testing.T) {
	p, pool, _ := testPortal(t)
	ctx := context.Background()
	if _, err := p.create(ctx, "Aurora", "aurora", sqlc.Person{Name: "Ana", Email: "ana@example.com", PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := pool.QueryRow(ctx, `SELECT schema_name FROM public.workspace_registry WHERE slug='aurora'`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	var required bool
	if err := pool.QueryRow(ctx, `SELECT require_app_approval FROM `+pgx.Identifier{schema}.Sanitize()+`.security_settings`).Scan(&required); err != nil {
		t.Fatal(err)
	}
	if required {
		t.Fatal("new company has require_app_approval = true, want false")
	}
}

// One e-mail and password at the main entry finds every company where they
// sign in, without opening a session; the main company is not listed.
func TestPortalFindLogins(t *testing.T) {
	p, _, srv := testPortal(t)
	ctx := context.Background()
	b := browser(t, srv.URL)
	b.want("POST", "/api/auth/setup", map[string]any{"name": "Chefe", "email": "chefe@x.com", "password": "senha-chefe-123"}, 201)
	hash := func(pw string) string {
		h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		return string(h)
	}
	for _, c := range []struct{ name, slug, pw string }{{"Cedro", "cedro", "senha-a-123"}, {"Bravo", "bravo", "senha-b-123"}, {"Alfa", "alfa", "senha-a-123"}} {
		if _, err := p.create(ctx, c.name, c.slug, sqlc.Person{Name: "Ana", Email: "ana@x.com", PasswordHash: hash(c.pw)}); err != nil {
			t.Fatal(err)
		}
	}
	found := func(email, pw string) string {
		t.Helper()
		var out struct {
			Companies []handler.LoginMatch `json:"companies"`
		}
		raw := b.want("POST", "/api/entry/login", map[string]string{"email": email, "password": pw}, 200)
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, c := range out.Companies {
			got = append(got, c.Name+"="+c.Slug+"@"+c.URL)
		}
		return strings.Join(got, ",")
	}
	if got := found(" Ana@X.com ", "senha-a-123"); got != "Alfa=alfa@/w/alfa/,Cedro=cedro@/w/cedro/" {
		t.Fatalf("senha-a-123 found %q", got)
	}
	if got := found("ana@x.com", "senha-b-123"); got != "Bravo=bravo@/w/bravo/" {
		t.Fatalf("senha-b-123 found %q", got)
	}
	raw := b.want("POST", "/api/entry/login", map[string]string{"email": "ana@x.com", "password": "errada-123"}, 401)
	if !strings.Contains(string(raw), "E-mail ou senha não conferem.") {
		t.Fatalf("wrong password message: %s", raw)
	}
	b.want("POST", "/api/entry/login", map[string]string{"email": "ninguem@x.com", "password": "senha-a-123"}, 401)
	b.want("POST", "/api/entry/login", map[string]string{"email": "chefe@x.com", "password": "senha-chefe-123"}, 401)
	b.want("POST", "/w/alfa/api/entry/login", map[string]string{"email": "ana@x.com", "password": "senha-a-123"}, 403)
	// The search opens no session anywhere.
	b.want("GET", "/w/alfa/api/apps", nil, 401)
}
