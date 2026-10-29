package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPortalRetentionAndTenantAgent(t *testing.T) {
	p, pool, srv := testPortal(t)
	b := browser(t, srv.URL)
	password := "a-test-password-not-a-real-credential"
	b.want("POST", "/api/auth/setup", map[string]string{"name": "Admin", "email": "admin@example.test", "password": password}, 201)
	var stored string
	if err := pool.QueryRow(context.Background(), `SELECT token FROM sessions LIMIT 1`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 64 {
		t.Fatal("raw bearer token was persisted")
	}
	b.want("POST", "/api/workspaces", map[string]string{"name": "Security test", "slug": "secure-company", "password": password}, 201)
	const prefix = "/w/secure-company"
	b.want("POST", prefix+"/api/auth/login", map[string]string{"email": "admin@example.test", "password": password}, 200)
	id := importApp(t, b, prefix, "Retention test app")
	tenant := p.entries["secure-company"]
	ctx := context.Background()
	var computer, old, active, stopped int64
	err := tenant.pool.QueryRow(ctx, `INSERT INTO computers(person_id,name,token_hash) VALUES(1,'security test agent','testhash') RETURNING id`).Scan(&computer)
	if err != nil {
		t.Fatal(err)
	}
	err = tenant.pool.QueryRow(ctx, `INSERT INTO runs(app_id,person_id,computer_id,output_folder,status,finished_at) VALUES($1,1,$2,'C:\Results','stopped',now()-interval '40 days') RETURNING id`, id, computer).Scan(&stopped)
	if err != nil {
		t.Fatal(err)
	}
	err = tenant.pool.QueryRow(ctx, `INSERT INTO runs(app_id,person_id,computer_id,output_folder,status,finished_at) VALUES($1,1,$2,'C:\Results','done',now()-interval '40 days') RETURNING id`, id, computer).Scan(&old)
	if err != nil {
		t.Fatal(err)
	}
	err = tenant.pool.QueryRow(ctx, `INSERT INTO runs(app_id,person_id,computer_id,output_folder,status) VALUES($1,1,$2,'C:\Results','waiting') RETURNING id`, id, computer).Scan(&active)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tenant.pool.Exec(ctx, `INSERT INTO agent_jobs(computer_id,run_id,kind,payload) VALUES($1,$2,'run','{"script":{"inputs":{"secret":"never-log"}}}')`, computer, active)
	if err != nil {
		t.Fatal(err)
	}
	b.want("PUT", prefix+"/api/security/settings", map[string]any{"retention_days": 30, "require_app_approval": true, "block_embedded_secrets": true, "local_execution_enabled": false}, 200)
	var status, payload string
	if err := tenant.pool.QueryRow(ctx, `SELECT status,payload::text FROM agent_jobs WHERE run_id=$1`, active).Scan(&status, &payload); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || strings.Contains(payload, "never-log") {
		t.Fatal("pending script was not cancelled and scrubbed")
	}
	b.want("POST", prefix+"/api/security/purge", map[string]bool{"confirm": false}, 400)
	raw := b.want("POST", prefix+"/api/security/purge", map[string]bool{"confirm": true}, 200)
	if !bytes.Contains(raw, []byte(`"deleted_runs":2`)) {
		t.Fatalf("wrong retention result: %s", raw)
	}
	var n int
	if err := tenant.pool.QueryRow(ctx, `SELECT count(*) FROM runs WHERE id=$1`, old).Scan(&n); err != nil || n != 0 {
		t.Fatal("old run still exists")
	}
	if err := tenant.pool.QueryRow(ctx, `SELECT count(*) FROM runs WHERE id=$1`, stopped).Scan(&n); err != nil || n != 0 {
		t.Fatal("old stopped run still exists")
	}
	if err := tenant.pool.QueryRow(ctx, `SELECT count(*) FROM runs WHERE id=$1`, active).Scan(&n); err != nil || n != 1 {
		t.Fatal("recent run unexpectedly purged")
	}
	// Tenant downloads must carry their own endpoint rather than silently
	// pairing with the original company encoded in the universal EXE.
	req, _ := http.NewRequest("GET", srv.URL+prefix+"/api/agent/download", nil)
	resp, err := b.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatal("tenant agent package missing")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sidecar := ""
	foundExe := false
	for _, f := range z.File {
		if f.Name == "ZivAgente.exe" {
			foundExe = true
		}
		if f.Name == "apphub-url.txt" {
			file, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := io.ReadAll(file)
			file.Close()
			sidecar = string(raw)
		}
	}
	if !foundExe || sidecar != "http://localhost"+prefix+"\n" {
		t.Fatal("agent ZIP is not scoped to company URL")
	}
	req, _ = http.NewRequest("GET", srv.URL+"/api/agent/download", nil)
	resp, err = b.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/vnd.microsoft.portable-executable" {
		t.Fatal("legacy agent download changed")
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "ZivAgente.exe") {
		t.Fatal("root download did not use the Ziv name")
	}
}

func TestPortalStopStaysInsideCompany(t *testing.T) {
	p, pool, srv := testPortal(t)
	b := browser(t, srv.URL)
	credentials := map[string]string{"name": "Admin", "email": "admin@example.test", "password": "test-password-not-a-real-credential"}
	b.want("POST", "/api/auth/setup", credentials, 201)
	rootApp := importApp(t, b, "", "Original app")
	b.want("POST", "/api/workspaces", map[string]string{"name": "Other company", "slug": "stop-company", "password": credentials["password"]}, 201)
	const prefix = "/w/stop-company"
	b.want("POST", prefix+"/api/auth/login", credentials, 200)
	importApp(t, b, prefix, "Other app")
	ctx := context.Background()
	seed := `INSERT INTO computers(person_id,name,token_hash) VALUES(1,'test computer','stop-test-token');
INSERT INTO runs(app_id,person_id,computer_id,output_folder,status) VALUES(1,1,1,'C:\Results','waiting');
INSERT INTO agent_jobs(computer_id,run_id,kind,payload) VALUES(1,1,'run','{"script":{"sources":{"private":"not-to-be-retained"}}}');`
	if rootApp != 1 {
		t.Fatal("expected first app ID")
	}
	if _, err := pool.Exec(ctx, seed); err != nil {
		t.Fatal(err)
	}
	b.want("POST", prefix+"/api/runs/1/stop", nil, 404)
	tenant := p.entries["stop-company"]
	if _, err := tenant.pool.Exec(ctx, seed); err != nil {
		t.Fatal(err)
	}
	b.want("POST", prefix+"/api/runs/1/stop", nil, 204)
	var status, payload string
	if err := pool.QueryRow(ctx, `SELECT status FROM runs WHERE id=1`).Scan(&status); err != nil || status != "waiting" {
		t.Fatal("stopping tenant run changed root run")
	}
	if err := tenant.pool.QueryRow(ctx, `SELECT status FROM runs WHERE id=1`).Scan(&status); err != nil || status != "stopped" {
		t.Fatal("tenant run was not stopped")
	}
	if err := tenant.pool.QueryRow(ctx, `SELECT payload::text FROM agent_jobs WHERE run_id=1`).Scan(&payload); err != nil || strings.Contains(payload, "private") {
		t.Fatal("cancelled job retained script source")
	}
	b.want("POST", prefix+"/api/runs/1/stop", nil, 409)
	b.want("GET", "/api/apps", nil, 200)
	b.want("GET", prefix+"/api/apps", nil, 200)
}
