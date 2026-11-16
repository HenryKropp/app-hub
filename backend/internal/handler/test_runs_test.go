package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// testeFicha is a draft ficha with a test plan that expects one PDF.
func testeFicha() map[string]any {
	return map[string]any{"teste": map[string]any{
		"acessa": []string{"nada externo"}, "pula": "nada",
		"espera":   []map[string]any{{"arquivos": "*.pdf"}},
		"entradas": map[string]string{"amount": "1"},
	}}
}

func computerID(t *testing.T, name string) int64 {
	t.Helper()
	var id int64
	if err := testPool.QueryRow(context.Background(), "SELECT id FROM computers WHERE name=$1 ORDER BY id DESC LIMIT 1", name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// runningTestJob puts draft d in testando/rodando with a test job for the
// computer and lets the agent take it. It returns the job id.
func runningTestJob(t *testing.T, c *client, h *Handler, appID, draftID int64, token, pcName string) int64 {
	t.Helper()
	ctx := context.Background()
	if n, err := h.q.StartDraftTest(ctx, draftID); err != nil || n != 1 {
		t.Fatalf("start %d %v", n, err)
	}
	if n, err := h.q.SetDraftStage(ctx, sqlc.SetDraftStageParams{ID: draftID, FromStage: "esperando_confirmacao", ToStage: "rodando"}); err != nil || n != 1 {
		t.Fatalf("stage %d %v", n, err)
	}
	payload, _ := json.Marshal(agentproto.TestPayload{DraftID: draftID, AppID: appID})
	j, err := h.q.CreateTestJob(ctx, sqlc.CreateTestJobParams{ComputerID: computerID(t, pcName), DraftID: pgtype.Int8{Int64: draftID, Valid: true}, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.q.SetDraftTestJob(ctx, sqlc.SetDraftTestJobParams{ID: draftID, JobID: pgtype.Int8{Int64: j.ID, Valid: true}}); err != nil {
		t.Fatal(err)
	}
	var job agentproto.Job
	if code := c.agent("GET", "/api/agent/next", token, nil, &job); code != http.StatusOK || job.ID != j.ID || job.Kind != agentproto.KindTest {
		t.Fatalf("next %d %+v", code, job)
	}
	return j.ID
}

func draftRow(t *testing.T, h *Handler, id int64) sqlc.AppDraft {
	t.Helper()
	d, err := h.q.GetDraftByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func agentResultFor(c *client, token string, job int64, res agentproto.Result) int {
	return c.agent("POST", "/api/agent/jobs/"+itoa(job)+"/result", token, res, nil)
}

// draftWithPC imports a script, pairs a PC and sends a draft with a test plan.
func draftWithPC(t *testing.T, c *client, h *Handler) (appID, draftID int64, token string) {
	t.Helper()
	appID = importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	token = pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	code, out := sendDraft(t, c, appID, map[string]string{"main.py": "print('v2')"}, map[string]any{"ficha": testeFicha()})
	if code != http.StatusCreated {
		t.Fatalf("draft %d %+v", code, out)
	}
	return appID, out.DraftID, token
}

func TestTestResultPassesAndApproves(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	job := runningTestJob(t, c, h, appID, draftID, token, "PC")
	res := agentproto.Result{Event: agentproto.EventFinished, ExitCode: 0, Log: "ok", Files: []agentproto.FileEntry{{Path: "a.pdf", Size: 3}}}
	if code := agentResultFor(c, token, job, res); code != http.StatusNoContent {
		t.Fatalf("result %d", code)
	}
	d := draftRow(t, h, draftID)
	// Approved drafts go straight on to publishing (Task 5).
	if d.Status != "no_ar" {
		t.Fatalf("status %q %q", d.Status, d.StatusReason)
	}
	var checks []struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(d.TestChecks, &checks); err != nil || len(checks) != 2 || !checks[0].OK || !checks[1].OK {
		t.Fatalf("checks %s %v", d.TestChecks, err)
	}
}

func TestTestResultFailsKeepsLive(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	before := versionCount(t, appID)
	job := runningTestJob(t, c, h, appID, draftID, token, "PC")
	agentResultFor(c, token, job, agentproto.Result{Event: agentproto.EventFinished, ExitCode: 1, Files: []agentproto.FileEntry{{Path: "a.pdf"}}})
	d := draftRow(t, h, draftID)
	if d.Status != "reprovada" || !strings.HasPrefix(d.StatusReason, "O teste não passou:") {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
	if storedNames(t, appID)["main.py"] != "print('v1')" || versionCount(t, appID) != before {
		t.Fatal("live app changed")
	}
}

func TestTestJobFailedEvent(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	job := runningTestJob(t, c, h, appID, draftID, token, "PC")
	agentResultFor(c, token, job, agentproto.Result{Event: agentproto.EventFailed, Error: "Instale Python"})
	if d := draftRow(t, h, draftID); d.Status != "reprovada" || d.StatusReason != "O teste não rodou: Instale Python" {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
}

func TestTestLogMaskedOnServer(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	appID := importKeyScript(t, c)
	c.do("PUT", "/api/apps/"+itoa(appID)+"/keys/BTG_SENHA", map[string]string{"value": keyValue}, nil)
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	_, out := sendDraft(t, c, appID, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": testeFicha()})
	job := runningTestJob(t, c, h, appID, out.DraftID, token, "PC")
	agentResultFor(c, token, job, agentproto.Result{Event: agentproto.EventFinished, Log: "senha=" + keyValue + "\n", Files: []agentproto.FileEntry{{Path: "a.pdf"}}})
	d := draftRow(t, h, out.DraftID)
	if strings.Contains(d.TestLog, keyValue) || !strings.Contains(d.TestLog, "••••") {
		t.Fatalf("log %q", d.TestLog)
	}
}

func evidenceZipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

func postEvidence(t *testing.T, c *client, token string, job int64, body []byte) int {
	t.Helper()
	req, _ := http.NewRequest("POST", c.srv.URL+"/api/agent/jobs/"+itoa(job)+"/evidence", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestEvidenceStoredAndScoped(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	job := runningTestJob(t, c, h, appID, draftID, token, "PC")
	other := pairComputer(t, c, "OUTRO")
	z := evidenceZipOf(t, map[string]string{"a.pdf": "pdf", "../x": "fora"})
	if code := postEvidence(t, c, other, job, z); code != http.StatusNotFound {
		t.Fatalf("other computer %d", code)
	}
	if code := postEvidence(t, c, token, job, z); code != http.StatusNoContent {
		t.Fatalf("evidence %d", code)
	}
	paths, err := h.q.ListEvidencePaths(context.Background(), draftID)
	if err != nil || len(paths) != 1 || paths[0] != "a.pdf" {
		t.Fatalf("paths %v %v", paths, err)
	}
	runJob, err := h.q.CreateJob(context.Background(), sqlc.CreateJobParams{ComputerID: computerID(t, "PC"), Kind: "run", Payload: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if code := postEvidence(t, c, token, runJob.ID, z); code != http.StatusBadRequest {
		t.Fatalf("run job %d", code)
	}
}

func TestAgentKeysForTestJob(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	appID := importKeyScript(t, c)
	c.do("PUT", "/api/apps/"+itoa(appID)+"/keys/BTG_SENHA", map[string]string{"value": keyValue}, nil)
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	_, out := sendDraft(t, c, appID, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": testeFicha()})
	job := runningTestJob(t, c, h, appID, out.DraftID, token, "PC")
	path := "/api/agent/jobs/" + itoa(job) + "/keys"
	var keys map[string]string
	if code := c.agent("GET", path, token, nil, &keys); code != http.StatusOK || keys["BTG_SENHA"] != keyValue {
		t.Fatalf("keys %d %v", code, keys)
	}
	if code := c.agent("GET", path, token, nil, nil); code != http.StatusNotFound {
		t.Fatalf("second %d", code)
	}
}

// Review fix 5: a test receives only the keys its plan declares, the ones the
// confirmation showed; a key added on the site later does not reach it.
func TestAgentKeysForTestJobOnlyDeclared(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	appID := importKeyScript(t, c)
	c.do("PUT", "/api/apps/"+itoa(appID)+"/keys/BTG_SENHA", map[string]string{"value": keyValue}, nil)
	if code := c.do("PUT", "/api/apps/"+itoa(appID)+"/keys/OUTRA_SENHA", map[string]string{"value": "outra-senha-123"}, nil); code != http.StatusOK {
		t.Fatalf("extra key %d", code)
	}
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	_, out := sendDraft(t, c, appID, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": testeFicha()})
	job := runningTestJob(t, c, h, appID, out.DraftID, token, "PC")
	var keys map[string]string
	if code := c.agent("GET", "/api/agent/jobs/"+itoa(job)+"/keys", token, nil, &keys); code != http.StatusOK || keys["BTG_SENHA"] != keyValue || keys["OUTRA_SENHA"] != "" {
		t.Fatalf("keys %d %v", code, keys)
	}
}
