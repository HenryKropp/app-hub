package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func TestOnboardingRealProgressAndAuthorization(t *testing.T) {
	c, _ := newPoolClient(t)
	if code := c.do("GET", "/api/onboarding", nil, nil); code != http.StatusUnauthorized {
		t.Fatal(code)
	}
	setupAdmin(t, c)
	var p struct {
		CompanyName string `json:"company_name"`
		HasApps     bool   `json:"has_apps"`
		CanManage   bool   `json:"can_manage"`
		HasTeam     bool   `json:"has_team"`
	}
	if code := c.do("GET", "/api/onboarding", nil, &p); code != 200 || p.HasApps || p.CompanyName != "" || !p.CanManage || p.HasTeam {
		t.Fatalf("%d %+v", code, p)
	}
	if code := c.do("PUT", "/api/onboarding", map[string]string{"company_name": "  Minha Empresa  "}, &p); code != 200 || p.CompanyName != "Minha Empresa" {
		t.Fatalf("%d %+v", code, p)
	}
	if code := c.do("PUT", "/api/onboarding", map[string]string{"company_name": ""}, nil); code != 400 {
		t.Fatal(code)
	}
	c.do("POST", "/api/people", map[string]string{"name": "Pessoa", "email": "user@example.test", "password": "test-password", "role": "usuario"}, nil)
	other := c.fresh()
	other.do("POST", "/api/auth/login", map[string]string{"email": "user@example.test", "password": "test-password"}, nil)
	if code := other.do("PUT", "/api/onboarding", map[string]string{"company_name": "Changed"}, nil); code != 403 {
		t.Fatal(code)
	}
	c.do("GET", "/api/onboarding", nil, &p)
	if !p.HasTeam {
		t.Fatal("team progress not reflected")
	}
}

func importTestScript(t *testing.T, c *client) int64 {
	t.Helper()
	m := manifest.Manifest{Name: "Script de teste", Summary: "Teste", Kind: "script", Entrypoint: "main.py", Area: "Research",
		Inputs: []manifest.Input{{ID: "amount", Label: "Valor", Type: "number", Source: "argument", Required: true}}}
	var out struct {
		ID int64 `json:"id"`
	}
	body := map[string]any{"manifest": m, "files": map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte("print('ok')"))}}}
	if code := c.do("POST", "/api/scripts/import", body, &out); code != http.StatusCreated {
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

func TestScriptImportQueueValidationAndCleanup(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importTestScript(t, c)
	token := pairComputer(t, c, "PC")
	var run runJSON
	if code := c.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\Resultados`, "inputs": map[string]string{"amount": "NaN"}}, nil); code != 400 {
		t.Fatalf("invalid number %d", code)
	}
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	if code := c.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\Resultados`, "inputs": map[string]string{"amount": "2"}}, &run); code != http.StatusCreated {
		t.Fatalf("run %d", code)
	}
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	var p agentproto.RunPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || p.Script == nil || p.Script.Sources["main.py"].ContentBase64 == "" || p.Script.Inputs["amount"][0] != "2" {
		t.Fatalf("payload %+v %v", p, err)
	}
	path := "/api/agent/jobs/" + strconv.FormatInt(job.ID, 10) + "/result"
	c.agent("POST", path, token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	c.agent("POST", path, token, agentproto.Result{Event: agentproto.EventFinished, ExitCode: 7, Files: []agentproto.FileEntry{{Path: "Execucao-1/log.txt", Size: 4, Change: "new"}}}, nil)
	var status string
	if err := testPool.QueryRow(context.Background(), "SELECT status FROM runs WHERE id=$1", run.ID).Scan(&status); err != nil || status != "error" {
		t.Fatalf("status %s %v", status, err)
	}
	var retained bool
	testPool.QueryRow(context.Background(), "SELECT payload ? 'script' FROM agent_jobs WHERE id=$1", job.ID).Scan(&retained)
	if retained {
		t.Fatal("script input bodies were retained")
	}
}

func TestScriptImportRejectsTraversalAndCredentialFiles(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	for _, name := range []string{"../main.py", ".env", "key.pem", "config/.env.local", "cert.key", "id_rsa", "ziv.yaml", "kova.yaml", "app-hub.yaml"} {
		body := map[string]any{"confirm_secrets": true, "manifest": manifest.Manifest{Name: "Bad", Kind: "script", Entrypoint: "main.py", Area: "Research"}, "files": map[string]manifest.InputFile{
			"main.py": {Name: "main.py", ContentBase64: ""}, name: {Name: name, ContentBase64: ""},
		}}
		if code := c.do("POST", "/api/scripts/import", body, nil); code != 400 {
			t.Fatalf("%s %d", name, code)
		}
	}
}

func TestScriptImportSecretsRequireConfirmationBeforePersistence(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	var logs bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })
	source := "password = 'FictitiousValue987'\nprint('ok')\n"
	helper := "const token = 'AnotherFakeValue987';\n"
	files := map[string]manifest.InputFile{
		"main.py":         {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte("print('ok')"))},
		"lib/settings.py": {Name: "settings.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte(source))},
		"helper.js":       {Name: "helper.js", ContentBase64: base64.StdEncoding.EncodeToString([]byte(helper))},
	}
	body := map[string]any{"manifest": manifest.Manifest{Name: "Aviso", Kind: "script", Entrypoint: "main.py", Area: "Research"}, "files": files}
	for _, confirmation := range []bool{false, false} {
		body["confirm_secrets"] = confirmation
		var out map[string]any
		if code := c.do("POST", "/api/scripts/import", body, &out); code != http.StatusConflict {
			t.Fatalf("unconfirmed import status %d", code)
		}
		if out["code"] != "script_secrets_confirmation_required" || out["secret_count"] != float64(2) || len(out) != 3 {
			t.Fatal("unexpected confirmation response")
		}
		raw, _ := json.Marshal(out)
		for _, value := range []string{"FictitiousValue987", "AnotherFakeValue987", "settings.py", "helper.js", "password", "const token"} {
			if strings.Contains(string(raw), value) || strings.Contains(logs.String(), value) {
				t.Fatal("response or logs exposed source data")
			}
		}
		var count int
		if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM apps").Scan(&count); err != nil || count != 0 {
			t.Fatal("unconfirmed import persisted an app")
		}
		if _, err := os.Stat(filepath.Join(temp, "apphub-imports")); !os.IsNotExist(err) {
			t.Fatal("unconfirmed import created a source folder")
		}
	}
	body["confirm_secrets"] = true
	var out struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", "/api/scripts/import", body, &out); code != http.StatusCreated {
		t.Fatalf("confirmed import status %d", code)
	}
	var raw []byte
	var folder string
	if err := testPool.QueryRow(context.Background(), "SELECT s.files, a.folder_path FROM apps a JOIN script_sources s ON s.app_id=a.id WHERE a.id=$1", out.ID).Scan(&raw, &folder); err != nil {
		t.Fatal(err)
	}
	var stored map[string]manifest.InputFile
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	for name, input := range files {
		if stored[name] != input {
			t.Fatal("persisted source was modified")
		}
		disk, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(name)))
		if err != nil || base64.StdEncoding.EncodeToString(disk) != input.ContentBase64 {
			t.Fatal("versioning source was modified")
		}
	}
	if strings.Contains(logs.String(), "FictitiousValue987") || strings.Contains(logs.String(), "AnotherFakeValue987") {
		t.Fatal("confirmed import logged a secret")
	}
}

func TestScriptImportPlaceholdersNeedNoConfirmation(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	t.Setenv("TMPDIR", t.TempDir())
	body := map[string]any{
		"manifest": manifest.Manifest{Name: "Placeholder", Kind: "script", Entrypoint: "main.py", Area: "Research"},
		"files":    map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte("password = 'your_password'\napi_key = os.getenv('API_KEY')"))}},
	}
	if code := c.do("POST", "/api/scripts/import", body, nil); code != http.StatusCreated {
		t.Fatalf("placeholder import status %d", code)
	}
}

func TestScriptImportStillRequiresPermission(t *testing.T) {
	c, _ := newPoolClient(t)
	if code := c.do("POST", "/api/scripts/import", map[string]any{"confirm_secrets": true}, nil); code != http.StatusUnauthorized {
		t.Fatal(code)
	}
	setupAdmin(t, c)
	c.do("POST", "/api/people", map[string]string{"name": "Pessoa", "email": "user@example.test", "password": "test-password", "role": "usuario"}, nil)
	other := c.fresh()
	other.do("POST", "/api/auth/login", map[string]string{"email": "user@example.test", "password": "test-password"}, nil)
	if code := other.do("POST", "/api/scripts/import", map[string]any{"confirm_secrets": true}, nil); code != http.StatusForbidden {
		t.Fatal(code)
	}
}
