package handler

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
)

func waitShared(t *testing.T, c *client, id int64, want string) map[string]any {
	t.Helper()
	var st struct {
		Shared map[string]any `json:"shared"`
	}
	for i := 0; i < 100; i++ {
		c.do("GET", appPath(id, "/shared"), nil, &st)
		if st.Shared != nil && st.Shared["status"] == want {
			return st.Shared
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("status never became %s: %+v", want, st.Shared)
	return nil
}

func TestSharedCheckAndPublish(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var rep struct {
		Ready bool                           `json:"ready"`
		Items []struct{ Key, Status string } `json:"items"`
	}
	if code := c.do("GET", appPath(id, "/shared/check"), nil, &rep); code != 200 || !rep.Ready {
		t.Fatalf("check %d %+v", code, rep)
	}
	if _, err := testPool.Exec(context.Background(), "INSERT INTO app_versions (app_id, number, origin, files, summary) VALUES ($1, 1, 'original', '[]', '{}')", id); err != nil {
		t.Fatal(err)
	}
	var dep struct {
		ID            int64  `json:"id"`
		VersionNumber *int32 `json:"version_number"`
		CreatedByName string `json:"created_by_name"`
	}
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &dep); code != http.StatusAccepted || dep.ID == 0 {
		t.Fatalf("publish %d", code)
	}
	if dep.VersionNumber == nil || *dep.VersionNumber != 1 || dep.CreatedByName == "" {
		t.Fatalf("publish answer should carry version_number and created_by_name: %+v", dep)
	}
	sh := waitShared(t, c, id, "running")
	if sh["url"] != "http://notas.localhost:8080" {
		t.Fatalf("%+v", sh)
	}
	var list []struct {
		Shared *struct{ Status, URL string } `json:"shared"`
	}
	c.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || list[0].Shared == nil || list[0].Shared.Status != "running" {
		t.Fatalf("catalog %+v", list)
	}
	var det struct {
		Shared *struct{ Status, URL, Slug string } `json:"shared"`
	}
	c.do("GET", appPath(id, ""), nil, &det)
	if det.Shared == nil || det.Shared.Slug != "notas" || det.Shared.URL != "http://notas.localhost:8080" {
		t.Fatalf("detail %+v", det.Shared)
	}
	// Progress of the deployment (admin sees the technical part).
	var prog map[string]any
	if code := c.do("GET", appPath(id, "/shared/deployments/"+itoa(dep.ID)), nil, &prog); code != 200 || prog["status"] != "live" {
		t.Fatalf("deployment %d %+v", code, prog)
	}
	if _, ok := prog["build_log"]; !ok {
		t.Fatalf("tech should see build_log: %+v", prog)
	}
	if prog["version_number"] == nil || prog["created_by_name"] == "" {
		t.Fatalf("deployment should carry version_number and created_by_name: %+v", prog)
	}
	var state struct {
		LiveVersion any   `json:"live_version"`
		Deployments []any `json:"deployments"`
	}
	c.do("GET", appPath(id, "/shared"), nil, &state)
	if len(state.Deployments) == 0 {
		t.Fatalf("tech should see deployments: %+v", state)
	}
	// Messages: last lines, from the live container.
	var logs struct{ Text string }
	if code := c.do("GET", appPath(id, "/shared/logs"), nil, &logs); code != 200 || !strings.Contains(logs.Text, "linha 1") {
		t.Fatalf("logs %d %q", code, logs.Text)
	}
	var usage []map[string]any
	if code := c.do("GET", appPath(id, "/shared/usage"), nil, &usage); code != 200 || usage == nil {
		t.Fatalf("usage %d %v", code, usage)
	}
}

func TestSharedDeploymentOfOtherAppIs404(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	a := sharedApp(t, h, d, "notas")
	b := sharedApp(t, h, d, "caixa")
	var dep struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", appPath(a, "/shared/publish"), nil, &dep); code != http.StatusAccepted {
		t.Fatalf("publish %d", code)
	}
	waitShared(t, c, a, "running")
	if code := c.do("GET", appPath(b, "/shared/deployments/"+itoa(dep.ID)), nil, nil); code != http.StatusNotFound {
		t.Fatalf("other app's deployment: %d", code)
	}
}

func TestSharedPublishTwiceIs409(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	release := make(chan struct{})
	d.svc.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	var first struct {
		ID int64 `json:"id"`
	}
	c.do("POST", appPath(id, "/shared/publish"), nil, &first)
	defer func() { // the background publish must end before the test's folders go away
		close(release)
		for i := 0; i < 200; i++ {
			dep, err := h.q.GetDeployment(context.Background(), first.ID)
			if err == nil && dep.Status != "building" && !d.svc.Busy(id) {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Error("publish never finished")
	}()
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &e); code != http.StatusConflict || e.Error != "Já existe uma publicação em andamento para este app." {
		t.Fatalf("%d %s", code, e.Error)
	}
	if code := c.do("POST", appPath(id, "/shared/test"), nil, &e); code != http.StatusConflict || e.Error != "Já existe uma publicação em andamento para este app." {
		t.Fatalf("test while publishing: %d %s", code, e.Error)
	}
	if code := c.do("POST", appPath(id, "/shared/stop"), nil, &e); code != http.StatusConflict || !strings.Contains(e.Error, "tente de novo") {
		t.Fatalf("stop while busy: %d %s", code, e.Error)
	}
}

func TestSharedPermissions(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	var st map[string]any
	if code := ana.do("GET", appPath(id, "/shared"), nil, &st); code != 200 {
		t.Fatalf("view %d", code)
	}
	if _, ok := st["deployments"]; ok {
		t.Fatalf("plain user sees deployments: %+v", st)
	}
	if _, ok := st["test_url"]; ok {
		t.Fatalf("plain user sees test box: %+v", st)
	}
	for _, p := range []string{"/shared/check", "/shared/config", "/shared/backups", "/shared/deployments/1", "/shared/backups/1/download"} {
		if code := ana.do("GET", appPath(id, p), nil, nil); code != http.StatusForbidden {
			t.Errorf("GET %s: %d", p, code)
		}
	}
	for _, p := range []string{"/shared/publish", "/shared/test", "/shared/stop", "/shared/start", "/shared/unshare", "/shared/rollback", "/shared/backups", "/shared/backups/1/restore"} {
		if code := ana.do("POST", appPath(id, p), map[string]any{}, nil); code != http.StatusForbidden {
			t.Errorf("POST %s: %d", p, code)
		}
	}
	for _, p := range []string{"/shared", "/shared/config"} {
		if code := ana.do("PUT", appPath(id, p), map[string]any{}, nil); code != http.StatusForbidden {
			t.Errorf("PUT %s: %d", p, code)
		}
	}
	for _, p := range []string{"/shared/logs", "/shared/usage"} {
		if code := ana.do("GET", appPath(id, p), nil, nil); code != http.StatusForbidden {
			t.Errorf("GET %s: %d", p, code)
		}
	}
	// Nothing ran: the app is still a draft with no deployment.
	if calls := d.eng.CallsCopy(); len(calls) != 0 {
		t.Fatalf("engine called: %v", calls)
	}
}

func TestSharedSettingsWriteManifest(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct {
		UpdatedAt string `json:"updated_at"`
	}
	c.do("GET", appPath(id, ""), nil, &app)
	body := map[string]any{"enabled": true, "always_on": true, "memory_mb": 1024, "cpus": 1, "database": "postgres",
		"env":  []map[string]any{{"name": "ADMIN_EMAILS", "label": "E-mails dos administradores", "required": true}},
		"acks": []string{"file_data", "nonsense"}, "updated_at": app.UpdatedAt}
	var pe struct{ Error string }
	if code := c.do("PUT", appPath(id, "/shared"), body, &pe); code != 200 {
		t.Fatalf("put %d %s", code, pe.Error)
	}
	var det struct {
		Manifest struct {
			Shared struct {
				AlwaysOn bool `json:"always_on"`
				MemoryMB int  `json:"memory_mb"`
			} `json:"shared"`
		} `json:"manifest"`
	}
	c.do("GET", appPath(id, ""), nil, &det)
	if !det.Manifest.Shared.AlwaysOn || det.Manifest.Shared.MemoryMB != 1024 {
		t.Fatalf("%+v", det.Manifest.Shared)
	}
	sa, err := h.q.GetSharedApp(context.Background(), id)
	if err != nil || sa.MemoryMb != 1024 || !sa.WantsDb || len(sa.Acks) != 1 || sa.Acks[0] != "file_data" {
		t.Fatalf("shared row %+v %v", sa, err)
	}
	// The same stale updated_at is refused.
	if code := c.do("PUT", appPath(id, "/shared"), body, nil); code != http.StatusConflict {
		t.Fatalf("stale put %d", code)
	}
	// configuração obrigatória vazia barra a publicação com a frase certa
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &e); code != 400 || e.Error != "Falta preencher: E-mails dos administradores." {
		t.Fatalf("%d %q", code, e.Error)
	}
}

func TestSharedSecretNeverReturned(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct {
		UpdatedAt string `json:"updated_at"`
	}
	c.do("GET", appPath(id, ""), nil, &app)
	if code := c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "updated_at": app.UpdatedAt,
		"env": []map[string]any{{"name": "SMTP_PASSWORD", "label": "Senha do e-mail", "secret": true}, {"name": "TITULO", "label": "Título"}}}, nil); code != 200 {
		t.Fatalf("put %d", code)
	}
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "SMTP_PASSWORD", "value": "s3nh@-forte"}, nil); code != 200 {
		t.Fatalf("set %d", code)
	}
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "TITULO", "value": "Notas"}, nil); code != 200 {
		t.Fatalf("set plain %d", code)
	}
	res := c.raw("GET", appPath(id, "/shared/config"), nil)
	buf := new(strings.Builder)
	io.Copy(buf, res.Body)
	if strings.Contains(buf.String(), "s3nh@-forte") || !strings.Contains(buf.String(), `"has_value":true`) || !strings.Contains(buf.String(), `"value":"Notas"`) {
		t.Fatalf("%s", buf.String())
	}
	if strings.Contains(buf.String(), "_db_password") {
		t.Fatalf("internal setting listed: %s", buf.String())
	}
	// The secret is sealed in the database, never kept in plain text.
	rows, _ := h.q.ListAppConfig(context.Background(), id)
	for _, r := range rows {
		if r.Name == "SMTP_PASSWORD" && (r.ValuePlain != "" || strings.Contains(string(r.ValueCipher), "s3nh@-forte")) {
			t.Fatalf("secret stored in the clear")
		}
	}
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "SMTP_PASSWORD", "value": "a\nb"}, nil); code != 400 {
		t.Fatalf("newline accepted: %d", code)
	}
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "_db_password", "value": "x"}, nil); code != 400 {
		t.Fatalf("internal name accepted: %d", code)
	}
	// Empty value deletes.
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "SMTP_PASSWORD", "value": ""}, nil); code != 200 {
		t.Fatalf("delete %d", code)
	}
	var items []struct {
		Name     string `json:"name"`
		HasValue bool   `json:"has_value"`
	}
	c.do("GET", appPath(id, "/shared/config"), nil, &items)
	for _, it := range items {
		if it.Name == "SMTP_PASSWORD" && it.HasValue {
			t.Fatalf("not deleted: %+v", items)
		}
	}
}

func TestSharedEngineDown(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	d.eng.SetDown(true)
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &e); code != 503 || !strings.Contains(e.Error, "Podman") {
		t.Fatalf("%d %q", code, e.Error)
	}
}

func TestSharedNotSharedActions(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	id := insertApp(t, filesPrepManifest())
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/stop"), nil, &e); code != 400 || e.Error != "Este app ainda não é compartilhado." {
		t.Fatalf("%d %q", code, e.Error)
	}
	var st map[string]any
	if code := c.do("GET", appPath(id, "/shared"), nil, &st); code != 200 || st["shared"] != nil {
		t.Fatalf("%d %+v", code, st)
	}
}

func TestSharedBackupsFlow(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct {
		UpdatedAt string `json:"updated_at"`
	}
	c.do("GET", appPath(id, ""), nil, &app)
	if code := c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "database": "postgres", "updated_at": app.UpdatedAt}, nil); code != 200 {
		t.Fatalf("put %d", code)
	}
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	waitShared(t, c, id, "running")
	var b struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", appPath(id, "/shared/backups"), nil, &b); code != 201 {
		t.Fatalf("backup %d", code)
	}
	res := c.raw("GET", appPath(id, "/shared/backups/"+itoa(b.ID)+"/download"), nil)
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Disposition"), "notas-") {
		t.Fatalf("download %d %s", res.StatusCode, res.Header.Get("Content-Disposition"))
	}
	res = c.raw("GET", appPath(id, "/shared/backups"), nil)
	buf := new(strings.Builder)
	io.Copy(buf, res.Body)
	if !strings.Contains(buf.String(), `"kind":"manual"`) || strings.Contains(buf.String(), `"path"`) {
		t.Fatalf("list %s", buf.String())
	}
	if code := c.do("POST", appPath(id, "/shared/backups/"+itoa(b.ID)+"/restore"), nil, nil); code != 200 {
		t.Fatalf("restore %d", code)
	}
	// A copy of another app is not served.
	other := sharedApp(t, h, d, "caixa")
	if res := c.raw("GET", appPath(other, "/shared/backups/"+itoa(b.ID)+"/download"), nil); res.StatusCode != 404 {
		t.Fatalf("other app's copy: %d", res.StatusCode)
	}
}

// An app whose code reads DATABASE_URL gets its database even when "Tornar
// compartilhado" sent no database (the checklist promised it).
func TestSharedDatabaseTurnedOnWhenCodeReadsIt(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	a, err := h.q.GetApp(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.FolderPath, "db.py"), []byte("import os\nurl = os.environ.get(\"DATABASE_URL\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var app struct {
		UpdatedAt string `json:"updated_at"`
	}
	c.do("GET", appPath(id, ""), nil, &app)
	if code := c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "database": "", "updated_at": app.UpdatedAt}, nil); code != 200 {
		t.Fatalf("put %d", code)
	}
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &e); code != http.StatusAccepted {
		t.Fatalf("publish %d %s", code, e.Error)
	}
	sh := waitShared(t, c, id, "running")
	if sh["wants_db"] != true {
		t.Fatalf("wants_db should be on (the backups section depends on it): %+v", sh)
	}
	found := false
	for _, ctr := range d.eng.Containers {
		if strings.HasPrefix(ctr.Spec.Env["DATABASE_URL"], "postgres://app_") {
			found = true
		}
	}
	if !found {
		t.Fatal("container has no DATABASE_URL")
	}
	var list []map[string]any
	if code := c.do("GET", appPath(id, "/shared/backups"), nil, &list); code != 200 {
		t.Fatalf("backups %d", code)
	}
}

func TestSharedCommandWithNewlineRefused(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct {
		UpdatedAt string `json:"updated_at"`
	}
	c.do("GET", appPath(id, ""), nil, &app)
	var e struct{ Error string }
	code := c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "command": "gunicorn app:app\nRUN curl x", "updated_at": app.UpdatedAt}, &e)
	if code != 400 || !strings.Contains(e.Error, "O comando de início precisa estar numa linha só.") {
		t.Fatalf("%d %q", code, e.Error)
	}
}

func TestSharedLogsMaskPasswords(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	waitShared(t, c, id, "running")
	d.eng.SetLogs("conectando password=abc123 no banco\nurl postgres://app_1:s3gr3d0@db/app_1\nAPI_TOKEN: tk-998877\nok\n")
	var logs struct{ Text string }
	if code := c.do("GET", appPath(id, "/shared/logs"), nil, &logs); code != 200 || strings.Contains(logs.Text, "abc123") || strings.Contains(logs.Text, "s3gr3d0") ||
		strings.Contains(logs.Text, "tk-998877") || !strings.Contains(logs.Text, "conectando password=") || !strings.Contains(logs.Text, "app_1:****@db") {
		t.Fatalf("%d %q", code, logs.Text)
	}
}

// Only finished copies inside this app's copies folder are ever served.
func TestSharedBackupDownloadPathGuard(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	dir := filepath.Join(h.shared.DataDir, "backups", itoa(id))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "segredo.dump")
	partial := filepath.Join(dir, ".partial-123")
	sneaky := filepath.Join(dir, "..", "..", "segredo.dump")
	for _, p := range []string{outside, partial, sneaky, filepath.Join(dir, "x.txt")} {
		os.WriteFile(p, []byte("dados"), 0o600)
		var bid int64
		if err := testPool.QueryRow(context.Background(), `INSERT INTO shared_backups (app_id, kind, path, size_bytes, expires_at)
			VALUES ($1, 'manual', $2, 5, now() + interval '1 day') RETURNING id`, id, p).Scan(&bid); err != nil {
			t.Fatal(err)
		}
		res := c.raw("GET", appPath(id, "/shared/backups/"+itoa(bid)+"/download"), nil)
		if res.StatusCode != 404 {
			t.Errorf("%s served: %d", p, res.StatusCode)
		}
	}
}

// Without the key, publishers get a plain message; the key's name goes to the log.
func TestSharedNoVaultPlainMessage(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	noVault := deploy.New(h.q, d.eng, d.dbs, nil, deploy.Options{DataDir: h.shared.DataDir, Domain: "localhost:8080", Scheme: "http", HealthTimeout: time.Second})
	sh := h.shared
	sh.Svc, sh.Vault = noVault, nil
	h.SetShared(sh)
	var e struct{ Error string }
	code := c.do("POST", appPath(id, "/shared/publish"), nil, &e)
	if code != 503 || strings.Contains(e.Error, "HUB_SECRET_KEY") || !strings.Contains(e.Error, "administrador") {
		t.Fatalf("%d %q", code, e.Error)
	}
}

// Turning sharing off before the first publish leaves nothing shared behind.
func TestSharedOffBeforeFirstPublish(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct {
		UpdatedAt string `json:"updated_at"`
	}
	c.do("GET", appPath(id, ""), nil, &app)
	if code := c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": false, "updated_at": app.UpdatedAt}, nil); code != 200 {
		t.Fatalf("put %d", code)
	}
	var list []struct {
		Shared *struct{ Status string } `json:"shared"`
	}
	c.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || list[0].Shared != nil {
		t.Fatalf("catalog still shows it shared: %+v", list[0].Shared)
	}
	var st map[string]any
	if c.do("GET", appPath(id, "/shared"), nil, &st); st["shared"] != nil {
		t.Fatalf("%+v", st)
	}
	// Sharing again starts as a draft, not as "parado".
	c.do("GET", appPath(id, ""), nil, &app)
	if code := c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "updated_at": app.UpdatedAt}, nil); code != 200 {
		t.Fatalf("put again %d", code)
	}
	if c.do("GET", appPath(id, "/shared"), nil, &st); st["shared"] == nil || st["shared"].(map[string]any)["status"] != "draft" {
		t.Fatalf("%+v", st)
	}
}

// A slug that cannot be an address is refused before the ficha is saved.
func TestSharedBadSlugSavesNothing(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	m := manifest.Manifest{ManifestVersion: 1, ID: "notas--velhas", Name: "Notas", Kind: manifest.KindWeb, Entrypoint: "app.py", Area: "Research"}
	id := insertApp(t, m)
	dir := t.TempDir()
	deploytest.CopyDir(t, "../../testdata/shared/streamlit", dir)
	testPool.Exec(context.Background(), "UPDATE apps SET folder_path = $1 WHERE id = $2", dir, id)
	var app struct {
		UpdatedAt string `json:"updated_at"`
	}
	c.do("GET", appPath(id, ""), nil, &app)
	var e struct{ Error string }
	if code := c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "updated_at": app.UpdatedAt}, &e); code != 400 || !strings.Contains(e.Error, "nome curto") {
		t.Fatalf("%d %q", code, e.Error)
	}
	var after struct {
		UpdatedAt string `json:"updated_at"`
		Manifest  struct {
			Shared any `json:"shared"`
		} `json:"manifest"`
	}
	c.do("GET", appPath(id, ""), nil, &after)
	if after.UpdatedAt != app.UpdatedAt || after.Manifest.Shared != nil {
		t.Fatalf("ficha was saved: %+v", after)
	}
}

// PUT /shared says when the running app must be turned on again for the change to count.
func TestSharedSettingsSayWhenRestartNeeded(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	type out struct {
		UpdatedAt    string `json:"updated_at"`
		NeedsRestart bool   `json:"needs_restart"`
	}
	var app out
	c.do("GET", appPath(id, ""), nil, &app)
	put := func(body map[string]any) out {
		t.Helper()
		body["enabled"], body["updated_at"] = true, app.UpdatedAt
		var o out
		if code := c.do("PUT", appPath(id, "/shared"), body, &o); code != 200 {
			t.Fatalf("put %d", code)
		}
		app = o
		return o
	}
	if put(map[string]any{"memory_mb": 1024}).NeedsRestart {
		t.Fatal("a draft needs no restart")
	}
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	waitShared(t, c, id, "running")
	c.do("GET", appPath(id, ""), nil, &app)
	if put(map[string]any{"memory_mb": 1024, "always_on": true}).NeedsRestart {
		t.Fatal("always on does not need a restart")
	}
	if !put(map[string]any{"memory_mb": 2048, "always_on": true}).NeedsRestart {
		t.Fatal("new memory on a running app needs a restart")
	}
}
