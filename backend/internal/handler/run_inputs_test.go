package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func TestSecretInput(t *testing.T) {
	for _, in := range []manifest.Input{
		{ID: "senha", Label: "Senha"},
		{ID: "x", Label: "Token da API"},
		{ID: "api_key", Label: "Chave"},
		{ID: "x", Label: "Chave de acesso"},
		{ID: "x", Label: "Algo", Type: "password"},
	} {
		if !secretInput(in) {
			t.Errorf("%+v should be secret", in)
		}
	}
	for _, in := range []manifest.Input{{ID: "mes", Label: "Mês"}, {ID: "busca", Label: "Palavra-chave"}} {
		if secretInput(in) {
			t.Errorf("%+v should not be secret", in)
		}
	}
}

func TestRecordInputsKeepsNoContentsNorSecrets(t *testing.T) {
	m := manifest.Manifest{Inputs: []manifest.Input{
		{ID: "mes", Label: "Mês", Type: "text"},
		{ID: "senha", Label: "Senha do portal", Type: "text"},
		{ID: "base", Label: "Base", Type: "file"},
		{ID: "vazio", Label: "Vazio", Type: "text"},
	}}
	raw := recordInputs(m, map[string][]string{"mes": {"outubro"}, "senha": {"abc123"}, "vazio": {" "}},
		map[string]manifest.InputFile{"base": {Name: "base.csv", ContentBase64: "c2VncmVkbw=="}})
	s := string(raw)
	if strings.Contains(s, "abc123") || strings.Contains(s, "c2VncmVkbw") || strings.Contains(s, "vazio") {
		t.Fatalf("kept too much: %s", s)
	}
	if !strings.Contains(s, "outubro") || !strings.Contains(s, "base.csv") || !strings.Contains(s, `"masked":true`) {
		t.Fatalf("kept too little: %s", s)
	}
	if string(recordInputs(manifest.Manifest{}, nil, nil)) != "{}" {
		t.Fatal("a run without a form is an empty object")
	}
}

// importFormScript imports a script whose form has a number, a password-like
// text and a file.
func importFormScript(t *testing.T, c *client) int64 {
	t.Helper()
	m := manifest.Manifest{Name: "Script com formulário", Summary: "Teste", Kind: "script", Entrypoint: "main.py", Area: "Research",
		Inputs: []manifest.Input{
			{ID: "amount", Label: "Valor", Type: "number", Source: "argument", Required: true},
			{ID: "token", Label: "Token do portal", Type: "text", Source: "argument"},
			{ID: "planilha", Label: "Planilha", Type: "file", Source: "argument"},
		}}
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

func TestRunShowsInputsConsoleAndLiveFields(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFormScript(t, c)
	token := pairComputer(t, c, "PC do Henry")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC do Henry", Version: agentproto.Version}, nil)
	content := base64.StdEncoding.EncodeToString([]byte("conteudo-da-planilha"))
	var run runJSON
	if code := c.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\Resultados`,
		"inputs": map[string]string{"amount": "2", "token": "segredo-123"},
		"files":  map[string]any{"planilha": map[string]string{"name": "dados.xlsx", "content_base64": content}}}, &run); code != http.StatusCreated {
		t.Fatalf("run %d", code)
	}
	if !run.InputsRecorded || len(run.Inputs) != 3 {
		t.Fatalf("inputs %+v", run.Inputs)
	}
	want := []runInputJSON{
		{ID: "amount", Label: "Valor", Type: "number", Values: []string{"2"}},
		{ID: "token", Label: "Token do portal", Type: "text", Values: []string{}, Masked: true},
		{ID: "planilha", Label: "Planilha", Type: "file", Values: []string{}, File: "dados.xlsx"},
	}
	for i, w := range want {
		g := run.Inputs[i]
		if g.ID != w.ID || g.Label != w.Label || g.Type != w.Type || g.File != w.File || g.Masked != w.Masked || strings.Join(g.Values, ",") != strings.Join(w.Values, ",") {
			t.Fatalf("input %d: got %+v want %+v", i, g, w)
		}
	}
	var stored string
	if err := testPool.QueryRow(context.Background(), "SELECT inputs::text FROM runs WHERE id=$1", run.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "segredo-123") || strings.Contains(stored, content) {
		t.Fatalf("stored a secret or file contents: %s", stored)
	}

	// While it runs, the strip knows when it started and on which computer.
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	path := "/api/agent/jobs/" + strconv.FormatInt(job.ID, 10) + "/result"
	c.agent("POST", path, token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	var active []activeRunJSON
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 1 || active[0].StartedAt == nil || active[0].ComputerName != "PC do Henry" || active[0].Status != "running" {
		t.Fatalf("active %+v", active)
	}

	// After it ends the payload loses the form, but the run keeps it, and
	// points at the console log.
	console := "Execucao-" + strconv.FormatInt(run.ID, 10) + "/.apphub-console.txt"
	c.agent("POST", path, token, agentproto.Result{Event: agentproto.EventFinished, Files: []agentproto.FileEntry{
		{Path: "Execucao-" + strconv.FormatInt(run.ID, 10) + "/saida.csv", Size: 4, Change: "new"},
		{Path: console, Size: 42, Change: "new"},
	}}, nil)
	var done runJSON
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &done)
	if done.Status != "done" || len(done.Inputs) != 3 || done.Inputs[0].Values[0] != "2" {
		t.Fatalf("done %+v", done)
	}
	if done.Console == nil || done.Console.Size != 42 {
		t.Fatalf("console %+v", done.Console)
	}
	var consoleID int64
	_ = testPool.QueryRow(context.Background(), "SELECT id FROM run_files WHERE run_id=$1 AND rel_path=$2", run.ID, console).Scan(&consoleID)
	if done.Console.FileID != consoleID {
		t.Fatalf("console id %d want %d", done.Console.FileID, consoleID)
	}
}

func TestRunWithoutFormSaysNothingFilled(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	var run runJSON
	if code := c.do("POST", appPath(app, "/runs"), map[string]string{}, &run); code != http.StatusCreated {
		t.Fatalf("run %d", code)
	}
	if !run.InputsRecorded || len(run.Inputs) != 0 || run.Console != nil {
		t.Fatalf("%+v", run)
	}
	// An older run, from before the record: nothing to say, and it says so.
	if _, err := testPool.Exec(context.Background(), "UPDATE runs SET inputs = NULL WHERE id=$1", run.ID); err != nil {
		t.Fatal(err)
	}
	var old runJSON
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &old)
	if old.InputsRecorded || old.Inputs == nil || len(old.Inputs) != 0 {
		t.Fatalf("old %+v", old)
	}
	raw, _ := json.Marshal(old)
	if !strings.Contains(string(raw), `"inputs":[]`) {
		t.Fatalf("inputs must be a list: %s", raw)
	}
}

func TestScheduleRunUsesScheduleInputs(t *testing.T) {
	admin, _, _, _ := team(t)
	app := insertApp(t, scriptManifest())
	comp := computerOf(t, 1, "PC")
	var s scheduleJSON
	if code := admin.do("POST", appPath(app, "/schedules"), scheduleForm(comp), &s); code != 201 {
		t.Fatalf("schedule %d", code)
	}
	// A missed run never went to the computer: it says what the schedule would use.
	var runID int64
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO runs (app_id, person_id, computer_id, output_folder, origin, schedule_id, status, started_at, finished_at)
		 VALUES ($1, 1, $2, 'C:\saida', 'schedule', $3, 'missed', now(), now()) RETURNING id`, app, comp, s.ID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	var run runJSON
	if code := admin.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10), nil, &run); code != 200 {
		t.Fatalf("get %d", code)
	}
	if !run.InputsRecorded || len(run.Inputs) != 2 || run.Inputs[0].Label != "Mês" || run.Inputs[0].Values[0] != "outubro" ||
		run.Inputs[1].Type != "file" || run.Inputs[1].File != "base.csv" {
		t.Fatalf("inputs %+v", run.Inputs)
	}
}
