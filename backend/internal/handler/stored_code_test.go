package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func importBody(name string) map[string]any {
	m := manifest.Manifest{Name: name, Summary: "Teste", Kind: "script", Entrypoint: "main.py", Area: "Research"}
	return map[string]any{"manifest": m, "files": map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte("print('ok')"))}}}
}

func firstVersion(t *testing.T, id int64) (origin, channel string, author int64) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(), `SELECT origin, channel, author_id FROM app_versions WHERE app_id=$1 AND number=1`, id).Scan(&origin, &channel, &author); err != nil {
		t.Fatal(err)
	}
	return
}

func TestImportViaClaudeMarksChannel(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	code, raw, err := h.callAPI(withMCPPerson(context.Background(), admin.ID), "POST", "/api/scripts/import", importBody("Pelo Claude"))
	if err != nil || code != http.StatusCreated {
		t.Fatalf("%d %s %v", code, raw, err)
	}
	var out struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(raw, &out)
	if o, ch, a := firstVersion(t, out.ID); o != "original" || ch != "mcp" || a != admin.ID {
		t.Fatalf("%s %s %d", o, ch, a)
	}
	var site struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", "/api/scripts/import", importBody("Pelo site"), &site); code != http.StatusCreated {
		t.Fatal(code)
	}
	if _, ch, _ := firstVersion(t, site.ID); ch != "manual" {
		t.Fatal(ch)
	}
}

func TestImportAnswersBlockedReason(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	var before bool
	_ = testPool.QueryRow(context.Background(), `SELECT require_app_approval FROM security_settings WHERE id = TRUE`).Scan(&before)
	exec(t, `UPDATE security_settings SET require_app_approval = true WHERE id = TRUE`)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE security_settings SET require_app_approval = $1 WHERE id = TRUE`, before)
	})
	var out struct {
		ID            int64  `json:"id"`
		Version       int32  `json:"version"`
		BlockedReason string `json:"blocked_reason"`
	}
	var e map[string]any
	if code := c.do("POST", "/api/scripts/import", importBody("Precisa aprovar"), &e); code != http.StatusCreated {
		t.Fatal(code, e)
	}
	b, _ := json.Marshal(e)
	json.Unmarshal(b, &out)
	if out.Version != 1 || out.BlockedReason != "Este app está esperando a aprovação de um administrador." {
		t.Fatalf("%+v", out)
	}
}

func b64files(files map[string]string) map[string]manifest.InputFile {
	out := map[string]manifest.InputFile{}
	for name, body := range files {
		out[name] = manifest.InputFile{Name: filepath.Base(name), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body))}
	}
	return out
}

// importFiles imports a script with these files (entrypoint main.py, one number input "amount").
func importFiles(t *testing.T, c *client, files map[string]string) int64 {
	t.Helper()
	m := manifest.Manifest{Name: "Soma", Summary: "Soma", Kind: "script", Entrypoint: "main.py", Area: "Research",
		Inputs: []manifest.Input{{ID: "amount", Label: "Valor", Type: "number", Source: "argument", Required: true}}}
	var out struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", "/api/scripts/import", map[string]any{"manifest": m, "files": b64files(files)}, &out); code != http.StatusCreated {
		t.Fatalf("import %d", code)
	}
	t.Cleanup(func() {
		var p string
		_ = testPool.QueryRow(context.Background(), "SELECT folder_path FROM apps WHERE id=$1", out.ID).Scan(&p)
		if p != "" {
			_ = os.RemoveAll(p)
		}
	})
	return out.ID
}

type sourcesOut struct {
	ID            int64     `json:"id"`
	Version       int32     `json:"version"`
	Changed       bool      `json:"changed"`
	BlockedReason string    `json:"blocked_reason"`
	UpdatedAt     time.Time `json:"updated_at"`
	Code          string    `json:"code"`
	Error         string    `json:"error"`
}

func send(t *testing.T, c *client, id int64, files map[string]string, extra map[string]any) (int, sourcesOut) {
	t.Helper()
	body := map[string]any{"files": b64files(files)}
	for k, v := range extra {
		body[k] = v
	}
	var out sourcesOut
	code := c.do("POST", appPath(id, "/sources"), body, &out)
	return code, out
}

func storedNames(t *testing.T, id int64) map[string]string {
	t.Helper()
	var raw []byte
	if err := testPool.QueryRow(context.Background(), "SELECT files FROM script_sources WHERE app_id=$1", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var files map[string]manifest.InputFile
	json.Unmarshal(raw, &files)
	out := map[string]string{}
	for k, f := range files {
		b, _ := base64.StdEncoding.DecodeString(f.ContentBase64)
		out[k] = string(b)
	}
	return out
}

func appFolder(t *testing.T, id int64) string {
	t.Helper()
	var p string
	if err := testPool.QueryRow(context.Background(), "SELECT folder_path FROM apps WHERE id=$1", id).Scan(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func detail(t *testing.T, c *client, id int64) appDetail {
	t.Helper()
	var d appDetail
	if code := c.do("GET", appPath(id, ""), nil, &d); code != http.StatusOK {
		t.Fatal(code)
	}
	return d
}

func TestSourcesNewVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	code, out := send(t, c, id, map[string]string{"main.py": "print('v2')"}, map[string]any{"note": "Corrige a soma"})
	if code != http.StatusOK || out.Version != 2 || !out.Changed {
		t.Fatalf("%d %+v", code, out)
	}
	var origin, desc string
	testPool.QueryRow(context.Background(), "SELECT origin, description FROM app_versions WHERE app_id=$1 AND number=2", id).Scan(&origin, &desc)
	if origin != "upload" || desc != "Corrige a soma" {
		t.Fatal(origin, desc)
	}
	if got := storedNames(t, id)["main.py"]; got != "print('v2')" {
		t.Fatal(got)
	}
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	var e map[string]any
	if code := c.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\Resultados`, "inputs": map[string]string{"amount": "2"}}, &e); code != http.StatusCreated {
		t.Fatalf("run %d %+v", code, e)
	}
}

func TestSourcesRemovedFileDisappears(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)", "old.py": "x=1"})
	if code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, nil); code != 200 {
		t.Fatal(code, out)
	}
	if _, ok := storedNames(t, id)["old.py"]; ok {
		t.Fatal("old.py still stored")
	}
	if _, err := os.Stat(filepath.Join(appFolder(t, id), "old.py")); err == nil {
		t.Fatal("old.py still in the folder")
	}
}

func TestSourcesUnchangedNoVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	code, out := send(t, c, id, map[string]string{"main.py": "print(1)"}, nil)
	if code != 200 || out.Changed || out.Version != 1 || versionCount(t, id) != 1 {
		t.Fatalf("%d %+v %d", code, out, versionCount(t, id))
	}
}

func TestSourcesFolderAppRefused(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := insertApp(t, filesPrepManifest())
	code, out := send(t, c, id, map[string]string{"main.py": "print(1)"}, nil)
	if code != http.StatusConflict || out.Code != "no_stored_code" || !strings.Contains(out.Error, "registrar_app sem app_id") {
		t.Fatalf("%d %+v", code, out)
	}
}

func TestSourcesNeedsEdit(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	c.do("POST", "/api/people", map[string]string{"name": "Pessoa", "email": "user@example.test", "password": "test-password", "role": "usuario"}, nil)
	other := c.fresh()
	other.do("POST", "/api/auth/login", map[string]string{"email": "user@example.test", "password": "test-password"}, nil)
	if code, out := send(t, other, id, map[string]string{"main.py": "print(2)"}, nil); code != http.StatusForbidden {
		t.Fatalf("%d %+v", code, out)
	}
}

func TestSourcesRefusedDuringRun(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	if code := c.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\R`, "inputs": map[string]string{"amount": "2"}}, nil); code != http.StatusCreated {
		t.Fatal(code)
	}
	code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, nil)
	if code != http.StatusConflict || !strings.Contains(out.Error, "Espere a execução terminar") {
		t.Fatalf("%d %+v", code, out)
	}
	if storedNames(t, id)["main.py"] != "print(1)" {
		t.Fatal("sources changed")
	}
}

func TestSourcesSecretsNeedConfirm(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	secret := map[string]string{"main.py": "KEY = 'AKIAIOSFODNN7EXAMPLQ'\nprint(KEY)"}
	if code, out := send(t, c, id, secret, nil); code != http.StatusConflict || out.Code != "script_secrets_confirmation_required" {
		t.Fatalf("%d %+v", code, out)
	}
	if code, out := send(t, c, id, secret, map[string]any{"confirm_secrets": true}); code != 200 || out.Version != 2 {
		t.Fatalf("%d %+v", code, out)
	}
}

func TestSourcesWaitsApproval(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	var before bool
	_ = testPool.QueryRow(context.Background(), `SELECT require_app_approval FROM security_settings WHERE id = TRUE`).Scan(&before)
	exec(t, `UPDATE security_settings SET require_app_approval = true WHERE id = TRUE`)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE security_settings SET require_app_approval = $1 WHERE id = TRUE`, before)
	})
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	if code := c.do("PUT", "/api/security/apps/"+itoa(id), map[string]any{"classification": "internal", "purpose": "Teste", "approved": true, "external_domains": []string{}}, nil); code != 200 {
		t.Fatal("approve", code)
	}
	if b, _ := h.executionBlock(context.Background(), id); b != "" {
		t.Fatal("not approved:", b)
	}
	if code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, nil); code != 200 || out.BlockedReason != "Este app está esperando a aprovação de um administrador." {
		t.Fatalf("%d %+v", code, out)
	}
}

func TestAppDetailStoredCode(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	var raw map[string]any
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	c.do("GET", appPath(id, ""), nil, &raw)
	if raw["stored_code"] != true {
		t.Fatalf("%+v", raw["stored_code"])
	}
	folderApp := insertApp(t, filesPrepManifest())
	raw = nil
	c.do("GET", appPath(folderApp, ""), nil, &raw)
	if raw["stored_code"] != false {
		t.Fatalf("%+v", raw["stored_code"])
	}
}

func TestSourcesWithFicha(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	d := detail(t, c, id)
	inputs := append(d.Manifest.Inputs, manifest.Input{ID: "data", Label: "Data", Type: "text", Source: "argument"})
	code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, map[string]any{
		"ficha": map[string]any{"summary": "Novo resumo", "inputs": inputs}, "updated_at": d.UpdatedAt.Time})
	if code != 200 || out.Version != 2 || out.UpdatedAt.IsZero() {
		t.Fatalf("%d %+v", code, out)
	}
	d2 := detail(t, c, id)
	if d2.Manifest.Summary != "Novo resumo" || len(d2.Manifest.Inputs) != 2 || d2.Manifest.Name != "Soma" {
		t.Fatalf("%+v", d2.Manifest)
	}
	var changes string
	if err := testPool.QueryRow(context.Background(), "SELECT changes::text FROM app_changes WHERE app_id=$1 ORDER BY id DESC LIMIT 1", id).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(changes, "summary") || !strings.Contains(changes, "inputs") {
		t.Fatal(changes)
	}
}

func TestSourcesFichaInvalidTouchesNothing(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	d := detail(t, c, id)
	bad := []manifest.Input{{ID: "x", Label: "X", Type: "xyz", Source: "argument"}}
	code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": map[string]any{"inputs": bad}, "updated_at": d.UpdatedAt.Time})
	if code != 400 || !strings.Contains(out.Error, "Tipo de entrada inválido") {
		t.Fatalf("%d %+v", code, out)
	}
	if storedNames(t, id)["main.py"] != "print(1)" || versionCount(t, id) != 1 {
		t.Fatal("touched")
	}
	if b, _ := os.ReadFile(filepath.Join(appFolder(t, id), "main.py")); string(b) != "print(1)" {
		t.Fatal("folder touched", string(b))
	}
}

func TestSourcesFichaStale(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	d := detail(t, c, id)
	code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": map[string]any{"summary": "x"}, "updated_at": d.UpdatedAt.Time.Add(-time.Hour)})
	if code != http.StatusConflict || !strings.Contains(out.Error, "Alguém mudou esta ficha") {
		t.Fatalf("%d %+v", code, out)
	}
	if storedNames(t, id)["main.py"] != "print(1)" || versionCount(t, id) != 1 {
		t.Fatal("touched")
	}
}

func TestSourcesFichaNewEntrypoint(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	d := detail(t, c, id)
	code, out := send(t, c, id, map[string]string{"main.py": "print(1)"}, map[string]any{"ficha": map[string]any{"entrypoint": "run.py"}, "updated_at": d.UpdatedAt.Time})
	if code != 400 || out.Error != "Inclua o arquivo que inicia o script." {
		t.Fatalf("%d %+v", code, out)
	}
	code, out = send(t, c, id, map[string]string{"main.py": "print(1)", "run.py": "print(2)"}, map[string]any{"ficha": map[string]any{"entrypoint": "run.py"}, "updated_at": d.UpdatedAt.Time})
	if code != 200 {
		t.Fatalf("%d %+v", code, out)
	}
	if e := detail(t, c, id).Manifest.Entrypoint; e != "run.py" {
		t.Fatal(e)
	}
}

func TestSourcesBodyHasNoNameOrArea(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	d := detail(t, c, id)
	if code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": map[string]any{"name": "X"}, "updated_at": d.UpdatedAt.Time}); code != 400 {
		t.Fatalf("%d %+v", code, out)
	}
	if code, out := send(t, c, id, map[string]string{"main.py": "print(2)"}, map[string]any{"area": "Investimentos"}); code != 400 {
		t.Fatalf("%d %+v", code, out)
	}
	if m := detail(t, c, id).Manifest; m.Name != "Soma" || m.Area != "Research" || versionCount(t, id) != 1 {
		t.Fatalf("%+v", m)
	}
}
