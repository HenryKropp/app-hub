package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func filesPrepManifest() manifest.Manifest {
	return manifest.Manifest{
		ManifestVersion: 1, ID: "eq-research-files-prep", Name: "Eq. Research Files Prep", Kind: "desktop",
		Entrypoint: "Ferramenta CVM.exe",
		Outputs: []manifest.Output{
			{Label: "Documentos da CVM", Pattern: "*/*/*.pdf"},
			{Label: "Data Pack", Pattern: "*/MD_Output/*_DataPack_Consolidado.md"},
			{Label: "FRE cortado", Pattern: "*_cortado_secoes.pdf"},
		},
	}
}

// insertApp stores an app directly (the registration flow is tested elsewhere).
func insertApp(t *testing.T, m manifest.Manifest) int64 {
	t.Helper()
	raw, _ := json.Marshal(m)
	var id int64
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO apps (slug, name, kind, folder_path, owner_id, manifest, created_by)
		 VALUES ($1, $2, $3, '/mnt/c/apps/files-prep', 1, $4, 1) RETURNING id`,
		m.ID, m.Name, m.Kind, raw).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func appPath(id int64, suffix string) string {
	return "/api/apps/" + strconv.FormatInt(id, 10) + suffix
}

func TestCanRun(t *testing.T) {
	m := filesPrepManifest()
	if !canRun(m) {
		t.Fatal("exe desktop should run")
	}
	m.Entrypoint = "app.py"
	if canRun(m) {
		t.Fatal("py should not run in this phase")
	}
	m.Entrypoint, m.Kind = "x.EXE", "desktop"
	if !canRun(m) {
		t.Fatal("case-insensitive exe")
	}
}

func TestRunWithoutComputer(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	var e map[string]string
	if code := c.do("POST", appPath(app, "/runs"), map[string]string{}, &e); code != http.StatusConflict || e["error"] != "Seu computador não está conectado." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestRunLifecycle(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")

	var run runJSON
	if code := c.do("POST", appPath(app, "/runs"), map[string]string{}, &run); code != http.StatusCreated {
		t.Fatalf("create run %d", code)
	}
	if run.Status != "waiting" || run.OutputFolder != `C:\apps\files-prep` {
		t.Fatalf("run %+v", run)
	}

	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	var p agentproto.RunPayload
	_ = json.Unmarshal(job.Payload, &p)
	if job.Kind != agentproto.KindRun || p.RunID != run.ID || p.Exe != "Ferramenta CVM.exe" || p.AppFolder != `C:\apps\files-prep` {
		t.Fatalf("job %+v payload %+v", job, p)
	}
	jobPath := "/api/agent/jobs/" + strconv.FormatInt(job.ID, 10) + "/result"

	c.agent("POST", jobPath, token, agentproto.Result{Event: agentproto.EventStarted, AppVersion: "2026-08-13 17:54 · 75,6 MB"}, nil)
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "running" || run.StartedAt == nil {
		t.Fatalf("running %+v", run)
	}

	now := time.Now()
	files := []agentproto.FileEntry{
		{Path: "PETROBRAS/FRE/FRE_2025.pdf", Size: 10, Modified: now, Change: "new"},
		{Path: "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", Size: 20, Modified: now, Change: "new"},
		{Path: "log.txt", Size: 1, Modified: now, Change: "changed"},
	}
	c.agent("POST", jobPath, token, agentproto.Result{Event: agentproto.EventFinished, Files: files}, nil)
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "done" || run.FinishedAt == nil || run.AppVersion == "" {
		t.Fatalf("done %+v", run)
	}
	labels := []string{}
	counts := []int{}
	for _, g := range run.Groups {
		labels = append(labels, g.Label)
		counts = append(counts, len(g.Files))
	}
	want := []string{"Documentos da CVM", "Data Pack", "FRE cortado", "Outros arquivos"}
	if len(labels) != 4 || labels[0] != want[0] || labels[1] != want[1] || labels[2] != want[2] || labels[3] != want[3] {
		t.Fatalf("labels %v", labels)
	}
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 0 || counts[3] != 1 {
		t.Fatalf("counts %v", counts)
	}

	var list []runSummaryJSON
	c.do("GET", appPath(app, "/runs"), nil, &list)
	if len(list) != 1 || list[0].FileCount != 3 {
		t.Fatalf("list %+v", list)
	}
}

func TestRunNoFilesAndFailure(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")

	for _, tc := range []struct {
		res    agentproto.Result
		status string
	}{
		{agentproto.Result{Event: agentproto.EventFinished}, "no_files"},
		{agentproto.Result{Event: agentproto.EventFailed, Error: "Já existe um Ferramenta CVM.exe nessa pasta."}, "error"},
	} {
		var run runJSON
		c.do("POST", appPath(app, "/runs"), map[string]string{"output_folder": `D:\Saídas`}, &run)
		var job agentproto.Job
		c.agent("GET", "/api/agent/next", token, nil, &job)
		c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, tc.res, nil)
		c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
		if run.Status != tc.status || run.OutputFolder != `D:\Saídas` || run.ErrorMessage != tc.res.Error {
			t.Fatalf("%s: %+v", tc.status, run)
		}
	}
}

func TestRunRejectsBadFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	if code := c.do("POST", appPath(app, "/runs"), map[string]string{"output_folder": "pasta relativa"}, nil); code != http.StatusBadRequest {
		t.Fatalf("relative folder %d", code)
	}
}

func TestStaleRunIsInterrupted(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	if _, err := testPool.Exec(context.Background(), "UPDATE agent_jobs SET created_at = now() - interval '3 minutes'"); err != nil {
		t.Fatal(err)
	}
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "interrupted" || run.ErrorMessage == "" {
		t.Fatalf("stale %+v", run)
	}
}

func TestHelloInterruptsOpenRun(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "interrupted" {
		t.Fatalf("after hello %+v", run)
	}
}

func TestRunVisibility(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)

	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, nil); code != http.StatusNotFound {
		t.Fatalf("other person sees run: %d", code)
	}
	var list []runSummaryJSON
	ana.do("GET", appPath(app, "/runs"), nil, &list)
	if len(list) != 0 {
		t.Fatalf("other person lists %d runs", len(list))
	}
}

func TestChooseFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	go func() {
		var job agentproto.Job
		c.agent("GET", "/api/agent/next", token, nil, &job)
		c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token,
			agentproto.Result{Event: agentproto.EventDone, Folder: `C:\Users\Henry\Relatórios`}, nil)
	}()
	var out struct {
		Folder   string `json:"folder"`
		Canceled bool   `json:"canceled"`
	}
	if code := c.do("POST", "/api/choose-folder", map[string]string{"initial": `C:\`}, &out); code != http.StatusOK || out.Folder != `C:\Users\Henry\Relatórios` {
		t.Fatalf("%d %+v", code, out)
	}
}

// finishedRun creates a run with the given files and returns run id, token.
func finishedRun(t *testing.T, c *client, files []agentproto.FileEntry) (int64, string) {
	t.Helper()
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventFinished, Files: files}, nil)
	return run.ID, token
}

// fakeAgentSend answers one send_file job with body, or reports missing.
func fakeAgentSend(t *testing.T, c *client, token string, body string, missing bool) <-chan agentproto.SendFilePayload {
	got := make(chan agentproto.SendFilePayload, 1)
	go func() {
		var job agentproto.Job
		for job.ID == 0 {
			c.agent("GET", "/api/agent/next", token, nil, &job)
		}
		var p agentproto.SendFilePayload
		_ = json.Unmarshal(job.Payload, &p)
		got <- p
		id := strconv.FormatInt(job.ID, 10)
		if missing {
			c.agent("POST", "/api/agent/jobs/"+id+"/result", token, agentproto.Result{Event: agentproto.EventMissing}, nil)
			return
		}
		req, _ := http.NewRequest("POST", c.srv.URL+"/api/agent/jobs/"+id+"/upload", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
	return got
}

func TestDownloadFile(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, token := finishedRun(t, c, []agentproto.FileEntry{
		{Path: "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", Size: 5, Modified: time.Now(), Change: "new"},
	})
	sent := fakeAgentSend(t, c, token, "# Data", false)

	resp, err := c.c.Get(c.srv.URL + "/api/runs/" + strconv.FormatInt(runID, 10) + "/files/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "# Data" {
		t.Fatalf("%d %q", resp.StatusCode, b)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "PETROBRAS_DataPack_Consolidado.md") {
		t.Fatalf("disposition %q", cd)
	}
	p := recvPayload(t, sent)
	if p.Zip || len(p.Paths) != 1 || p.Paths[0] != "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md" {
		t.Fatalf("payload %+v", p)
	}
}

func TestDownloadMissingFile(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, token := finishedRun(t, c, []agentproto.FileEntry{{Path: "a.txt", Size: 1, Modified: time.Now(), Change: "new"}})
	fakeAgentSend(t, c, token, "", true)
	var e map[string]string
	if code := c.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/1", nil, &e); code != http.StatusNotFound || e["error"] != "Esse arquivo não está mais na pasta." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestDownloadGroupZipPayload(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	now := time.Now()
	runID, token := finishedRun(t, c, []agentproto.FileEntry{
		{Path: "P/FRE/a.pdf", Size: 1, Modified: now, Change: "new"},
		{Path: "P/DFP/b.pdf", Size: 1, Modified: now, Change: "new"},
		{Path: "log.txt", Size: 1, Modified: now, Change: "new"},
	})
	sent := fakeAgentSend(t, c, token, "PK", false)
	resp, err := c.c.Get(c.srv.URL + "/api/runs/" + strconv.FormatInt(runID, 10) + "/groups/0/zip")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	p := recvPayload(t, sent)
	if !p.Zip || len(p.Paths) != 2 || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("payload %+v type %q", p, resp.Header.Get("Content-Type"))
	}
}

func TestDownloadRefusesUnknownFileAndOtherPerson(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, _ := finishedRun(t, c, []agentproto.FileEntry{{Path: "a.txt", Size: 1, Modified: time.Now(), Change: "new"}})
	if code := c.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/999", nil, nil); code != http.StatusNotFound {
		t.Fatalf("unknown file %d", code)
	}
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/1", nil, nil); code != http.StatusNotFound {
		t.Fatalf("other person %d", code)
	}
	var jobs int
	_ = testPool.QueryRow(context.Background(), "SELECT count(*) FROM agent_jobs WHERE kind = 'send_file'").Scan(&jobs)
	if jobs != 0 {
		t.Fatalf("refused downloads must not reach the agent, got %d jobs", jobs)
	}
}

func TestDownloadComputerOffline(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, _ := finishedRun(t, c, []agentproto.FileEntry{{Path: "a.txt", Size: 1, Modified: time.Now(), Change: "new"}})
	_, _ = testPool.Exec(context.Background(), "UPDATE computers SET last_seen_at = now() - interval '5 minutes'")
	if code := c.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/1", nil, nil); code != http.StatusConflict {
		t.Fatalf("offline %d", code)
	}
}

func recvPayload(t *testing.T, ch <-chan agentproto.SendFilePayload) agentproto.SendFilePayload {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(5 * time.Second):
		t.Fatal("the agent never received a send_file job")
		return agentproto.SendFilePayload{}
	}
}
