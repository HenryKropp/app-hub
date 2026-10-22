package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
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
	for _, name := range []string{"../main.py", ".env", "key.pem"} {
		body := map[string]any{"manifest": manifest.Manifest{Name: "Bad", Kind: "script", Entrypoint: "main.py", Area: "Research"}, "files": map[string]manifest.InputFile{
			"main.py": {Name: "main.py", ContentBase64: ""}, name: {Name: name, ContentBase64: ""},
		}}
		if code := c.do("POST", "/api/scripts/import", body, nil); code != 400 {
			t.Fatalf("%s %d", name, code)
		}
	}
}
