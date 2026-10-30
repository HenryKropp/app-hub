package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/jackc/pgx/v5/pgxpool"
)

func credentialBackupEnv(pool *pgxpool.Pool) []string {
	c := pool.Config().ConnConfig
	return append(os.Environ(), "PGHOST="+c.Host, "PGPORT="+strconv.Itoa(int(c.Port)),
		"PGDATABASE="+c.Database, "PGUSER="+c.User, "PGPASSWORD="+c.Password, "PGOPTIONS=")
}

func credentialRingJSON(t *testing.T, active string, keys map[string]string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"active": active, "keys": keys})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPortalCredentialVaultLifecycle(t *testing.T) {
	oldKey, newKey := vault.NewKey(), vault.NewKey()
	t.Setenv("CREDENTIAL_VAULT_KEYRING", credentialRingJSON(t, "old", map[string]string{"old": oldKey}))
	p, pool, srv := testPortal(t)
	ctx := context.Background()
	b := browser(t, srv.URL)
	password := "synthetic-test-account-password"
	b.want("POST", "/api/auth/setup", map[string]string{"name": "Vault Admin", "email": "vault@example.test", "password": password}, 201)
	app := importApp(t, b, "", "Vault test")
	otherApp := importApp(t, b, "", "Other app")
	b.want("POST", "/api/workspaces", map[string]string{"name": "Vault isolation", "slug": "vault-company", "password": password}, 201)
	const prefix = "/w/vault-company"
	b.want("POST", prefix+"/api/auth/login", map[string]string{"email": "vault@example.test", "password": password}, 200)
	tenantApp := importApp(t, b, prefix, "Tenant app")
	// Approval still applies to broker use, independently of the vault grant.
	for path, id := range map[string]int64{"": app, prefix: tenantApp} {
		b.want("PUT", fmt.Sprintf("%s/api/security/apps/%d", path, id), map[string]any{"classification": "internal", "purpose": "Vault tests", "external_domains": []string{}, "approved": true}, 200)
	}
	const secret = "synthetic-vault-value-never-in-api-or-audit"
	input := map[string]any{"app_id": app, "environment": "production", "role_id": 1, "endpoint": "https://unresolvable.invalid/check", "value": secret}
	var logs bytes.Buffer
	before := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(before)
	raw := b.want("POST", "/api/security/credentials", input, 201)
	var credential struct {
		ID       string `json:"id"`
		Revision int    `json:"revision"`
	}
	if json.Unmarshal(raw, &credential) != nil || credential.ID == "" || credential.Revision != 1 {
		t.Fatal("missing credential receipt")
	}
	route := "/api/security/credentials/" + credential.ID
	check := fmt.Sprintf("/api/apps/%d/credentials/%s/check", app, credential.ID)
	// No value-read route; all metadata APIs omit ciphertext and destination.
	b.want("GET", route, nil, 405)
	for _, path := range []string{"/api/security/credentials", "/api/security/credentials/audit", "/api/security/export", "/api/security/audit"} {
		response := b.want("GET", path, nil, 200)
		if bytes.Contains(response, []byte(secret)) || bytes.Contains(response, []byte("wrapped")) || bytes.Contains(response, []byte("unresolvable.invalid")) {
			t.Fatal("credential content in metadata/export")
		}
	}
	var envelope []byte
	if err := pool.QueryRow(ctx, `SELECT envelope FROM credential_vault WHERE id=$1`, credential.ID).Scan(&envelope); err != nil || bytes.Contains(envelope, []byte(secret)) {
		t.Fatal("plaintext in storage")
	}
	b.want("POST", check, map[string]string{"environment": "staging"}, 404)
	b.want("POST", fmt.Sprintf("/api/apps/%d/credentials/%s/check", otherApp, credential.ID), map[string]string{"environment": "production"}, 403) // other app is unapproved
	b.want("POST", prefix+fmt.Sprintf("/api/apps/%d/credentials/%s/check", tenantApp, credential.ID), map[string]string{"environment": "production"}, 404)
	b.want("DELETE", prefix+route, nil, 404)
	tenantList := b.want("GET", prefix+"/api/security/credentials", nil, 200)
	if bytes.Contains(tenantList, []byte(credential.ID)) {
		t.Fatal("tenant saw another company's credential")
	}
	// The broker never queues values or returns provider errors/content.
	response := b.want("POST", check, map[string]string{"environment": "production"}, 503)
	if bytes.Contains(response, []byte(secret)) || bytes.Contains(response, []byte("unresolvable.invalid")) {
		t.Fatal("broker leaked content")
	}
	var attempts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credential_audit WHERE credential_id=$1 AND action='use.started'`, credential.ID).Scan(&attempts); err != nil || attempts < 1 {
		t.Fatal("missing durable use attempt")
	}
	// App suspension and role policy are independent revocation boundaries.
	b.want("PUT", fmt.Sprintf("/api/security/apps/%d", app), map[string]any{"classification": "internal", "purpose": "Vault tests", "external_domains": []string{}, "approved": true, "suspended": true}, 200)
	b.want("POST", check, map[string]string{"environment": "production"}, 403)
	b.want("PUT", fmt.Sprintf("/api/security/apps/%d", app), map[string]any{"classification": "internal", "purpose": "Vault tests", "external_domains": []string{}, "approved": true}, 200)
	input["role_id"] = 3
	b.want("PUT", route, input, 200)
	b.want("POST", check, map[string]string{"environment": "production"}, 404) // even admin cannot bypass role
	input["role_id"] = 1
	input["value"] = "synthetic-replacement-token"
	b.want("PUT", route, input, 200)
	// Master-key rotation is atomic and survives removal of the old key.
	ring := credentialRingJSON(t, "new", map[string]string{"old": oldKey, "new": newKey})
	p.credentialKeys, _ = vault.ParseKeyring(ring)
	p.attach(p.entries["principal"])
	b.want("POST", "/api/security/credentials/rewrap", nil, 200)
	var current []byte
	if err := pool.QueryRow(ctx, `SELECT envelope FROM credential_vault WHERE id=$1`, credential.ID).Scan(&current); err != nil {
		t.Fatal(err)
	}
	var e vault.Envelope
	_ = json.Unmarshal(current, &e)
	if e.KeyID != "new" {
		t.Fatal("master key not rotated")
	}
	// Exercise a real PostgreSQL archive, not just a JSON serialization. Only
	// synthetic fixture data lives in this test database. Never print archive
	// contents, connection parameters, stderr or environment on failure.
	dump := exec.Command("pg_dump", "--format=custom", "--data-only", "--table=public.credential_vault", "--no-owner", "--no-acl")
	dump.Env = credentialBackupEnv(pool)
	archive, err := dump.Output()
	if err != nil {
		t.Fatal("credential test backup failed")
	}
	if bytes.Contains(archive, []byte(secret)) || bytes.Contains(archive, []byte("synthetic-replacement-token")) {
		t.Fatal("credential value in backup archive")
	}
	p.credentialKeys, _ = vault.ParseKeyring(credentialRingJSON(t, "new", map[string]string{"new": newKey}))
	p.attach(p.entries["principal"])
	b.want("POST", "/api/security/credentials/rewrap", nil, 200)
	b.want("DELETE", route, nil, 200)
	b.want("POST", check, map[string]string{"environment": "production"}, 404)
	b.want("PUT", route, input, 404) // a tombstone is never resurrected by replace
	var gone bool
	if err := pool.QueryRow(ctx, `SELECT revoked AND envelope IS NULL FROM credential_vault WHERE id=$1`, credential.ID).Scan(&gone); err != nil || !gone {
		t.Fatal("revoked ciphertext retained")
	}
	// Restore the old active snapshot. The mandatory recovery step
	// quarantines it before the portal is made available, not just current rows.
	if _, err := pool.Exec(ctx, `TRUNCATE credential_vault`); err != nil {
		t.Fatal(err)
	}
	restore := exec.Command("pg_restore", "--data-only", "--no-owner", "--exit-on-error", "--dbname="+pool.Config().ConnConfig.Database)
	restore.Env = credentialBackupEnv(pool)
	restore.Stdin = bytes.NewReader(archive)
	if err := restore.Run(); err != nil {
		t.Fatal("credential test restore failed")
	}
	tenantInput := map[string]any{"app_id": tenantApp, "environment": "staging", "role_id": 1, "endpoint": "https://unresolvable.invalid/check", "value": secret}
	b.want("POST", prefix+"/api/security/credentials", tenantInput, 201)
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "quarantine-restored-credentials.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	b.want("POST", check, map[string]string{"environment": "production"}, 404)
	for _, entry := range p.entries {
		var active int
		if err := entry.pool.QueryRow(ctx, `SELECT count(*) FROM credential_vault WHERE NOT revoked OR envelope IS NOT NULL`).Scan(&active); err != nil || active != 0 {
			t.Fatal("restore quarantine missed a company")
		}
	}
	// Failure logs, source bundles, manifests, audit and jobs never carry values.
	for _, query := range []string{
		`SELECT COALESCE(string_agg(manifest::text,''),'') FROM apps`,
		`SELECT COALESCE(string_agg(files::text,''),'') FROM script_sources`,
		`SELECT COALESCE(string_agg(payload::text,''),'') FROM agent_jobs`,
		`SELECT COALESCE(string_agg(to_jsonb(a)::text,''),'') FROM security_audit a`,
		`SELECT COALESCE(string_agg(to_jsonb(a)::text,''),'') FROM credential_audit a`,
	} {
		var data string
		if err := pool.QueryRow(ctx, query).Scan(&data); err != nil || strings.Contains(data, secret) || strings.Contains(data, "synthetic-replacement-token") {
			t.Fatal("credential appeared outside encrypted storage")
		}
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "synthetic-replacement-token") {
		t.Fatal("credential value in logs")
	}
	// A locked vault still permits metadata and revocation, never creation.
	p.credentialKeys = nil
	p.attach(p.entries["principal"])
	b.want("POST", "/api/security/credentials", input, 503)
	b.want("GET", "/api/security/credentials", nil, 200)
	b.want("DELETE", route, nil, 200)
	anonymous := browser(t, srv.URL)
	anonymous.want("GET", "/api/security/credentials", nil, 401)
	anonymous.want("POST", "/api/security/credentials", input, 401)
}

func TestPortalCredentialAuthorizationAndAtomicRewrap(t *testing.T) {
	oldKey, newKey := vault.NewKey(), vault.NewKey()
	t.Setenv("CREDENTIAL_VAULT_KEYRING", credentialRingJSON(t, "old", map[string]string{"old": oldKey}))
	p, pool, srv := testPortal(t)
	ctx := context.Background()
	admin := browser(t, srv.URL)
	admin.want("POST", "/api/auth/setup", map[string]string{"name": "Vault Admin", "email": "vault@example.test", "password": "synthetic-admin-password"}, 201)
	app := importApp(t, admin, "", "Vault role test")
	admin.want("PUT", fmt.Sprintf("/api/security/apps/%d", app), map[string]any{"classification": "internal", "purpose": "Vault tests", "external_domains": []string{}, "approved": true}, 200)
	admin.want("POST", "/api/people", map[string]any{"name": "Member", "email": "member@example.test", "password": "synthetic-member-password", "role_id": 3}, 201)
	user := browser(t, srv.URL)
	user.want("POST", "/api/auth/login", map[string]string{"email": "member@example.test", "password": "synthetic-member-password"}, 200)
	input := map[string]any{"app_id": app, "environment": "production", "role_id": 3, "endpoint": "https://unresolvable.invalid/check", "value": "synthetic-test-token"}
	var ids []string
	for i := 0; i < 2; i++ {
		var result struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(admin.want("POST", "/api/security/credentials", input, 201), &result) != nil {
			t.Fatal("create failed")
		}
		ids = append(ids, result.ID)
	}
	route := "/api/security/credentials/" + ids[0]
	for _, path := range []string{"/api/security/credentials", "/api/security/credentials/audit"} {
		user.want("GET", path, nil, 403)
	}
	user.want("POST", "/api/security/credentials", input, 403)
	user.want("PUT", route, input, 403)
	user.want("DELETE", route, nil, 403)
	user.want("POST", "/api/security/credentials/rewrap", nil, 403)
	// A company member with the explicit role may use but never read/manage.
	check := fmt.Sprintf("/api/apps/%d/credentials/%s/check", app, ids[0])
	user.want("POST", check, map[string]string{"environment": "production"}, 503)
	var used int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credential_audit WHERE credential_id=$1 AND revision=1 AND action='use.failed'`, ids[0]).Scan(&used); err != nil || used != 1 {
		t.Fatal("authorized use not audited with revision")
	}
	const accidentalTokenInPath = "cafebabedeadbeefcafebabedeadbeef"
	user.want("POST", fmt.Sprintf("/api/apps/%d/credentials/%s/check", app, accidentalTokenInPath), map[string]string{"environment": "production"}, 404)
	for _, path := range []string{"/api/security/credentials/audit", "/api/security/audit", "/api/security/export"} {
		if bytes.Contains(admin.want("GET", path, nil, 200), []byte(accidentalTokenInPath)) {
			t.Fatal("untrusted path value copied to audit/export")
		}
	}
	// Invalid references and request shapes do not create records or leak input.
	input["role_id"] = 99999
	admin.want("POST", "/api/security/credentials", input, 400)
	input["role_id"] = 3
	input["environment"] = "unknown"
	admin.want("POST", "/api/security/credentials", input, 400)
	input["environment"] = "production"
	input["endpoint"] = "https://user:password@api.example.test/"
	admin.want("POST", "/api/security/credentials", input, 400)
	input["endpoint"] = "https://api.example.test/?token=synthetic-test-token"
	admin.want("POST", "/api/security/credentials", input, 400)
	input["endpoint"] = "https://api.example.test/"
	input["value"] = "bad\r\nInjected: header"
	admin.want("POST", "/api/security/credentials", input, 400)
	// Corrupt the LAST row so successful earlier work must roll back.
	var last string
	if err := pool.QueryRow(ctx, `SELECT max(id) FROM credential_vault`).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE credential_vault SET envelope=jsonb_set(envelope,'{version}','999') WHERE id=$1`, last); err != nil {
		t.Fatal(err)
	}
	p.credentialKeys, _ = vault.ParseKeyring(credentialRingJSON(t, "new", map[string]string{"old": oldKey, "new": newKey}))
	p.attach(p.entries["principal"])
	admin.want("POST", "/api/security/credentials/rewrap", nil, 503)
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credential_vault WHERE envelope->>'key_id'='old'`).Scan(&count); err != nil || count != 2 {
		t.Fatal("partial rewrap committed")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credential_audit WHERE action='rotated'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("rolled-back rotation audited as successful")
	}
}
