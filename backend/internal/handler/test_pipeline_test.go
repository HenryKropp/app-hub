package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/testplan"
	"github.com/jackc/pgx/v5/pgtype"
)

// confirmPlan records the draft's plan as confirmed, testing on computer pc.
func confirmPlan(t *testing.T, h *Handler, appID, draftID int64, pc string) {
	t.Helper()
	ctx := context.Background()
	a, err := h.q.GetApp(ctx, appID)
	if err != nil {
		t.Fatal(err)
	}
	m, err := draftManifest(a, draftRow(t, h, draftID))
	if err != nil {
		t.Fatal(err)
	}
	var me int64
	testPool.QueryRow(ctx, "SELECT id FROM people ORDER BY id LIMIT 1").Scan(&me)
	if _, err := h.q.UpsertTestPlan(ctx, sqlc.UpsertTestPlanParams{AppID: appID, Fingerprint: testplan.Fingerprint(m, testplan.PlacePC),
		ComputerID: pgtype.Int8{Int64: computerID(t, pc), Valid: true}, ConfirmedBy: pgtype.Int8{Int64: me, Valid: true}}); err != nil {
		t.Fatal(err)
	}
}

func startTest(t *testing.T, h *Handler, draftID int64) {
	t.Helper()
	if n, err := h.q.StartDraftTest(context.Background(), draftID); err != nil || n != 1 {
		t.Fatalf("start %d %v", n, err)
	}
}

func testJobs(t *testing.T, draftID int64) int {
	t.Helper()
	var n int
	testPool.QueryRow(context.Background(), "SELECT count(*) FROM agent_jobs WHERE kind='test' AND draft_id=$1", draftID).Scan(&n)
	return n
}

func noticeTexts(t *testing.T, appID int64) []string {
	t.Helper()
	rows, err := testPool.Query(context.Background(), "SELECT text FROM notices WHERE app_id=$1 ORDER BY id", appID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		out = append(out, s)
	}
	return out
}

func TestPipelineWaitsConfirmation(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	_, draftID, _ := draftWithPC(t, c, h)
	startTest(t, h, draftID)
	h.testPipelineTick(context.Background(), time.Now())
	if d := draftRow(t, h, draftID); d.Stage != "esperando_confirmacao" || testJobs(t, draftID) != 0 {
		t.Fatalf("stage %q jobs %d", d.Stage, testJobs(t, draftID))
	}
	if got := draftStageLabel(draftRow(t, h, draftID), nil, nil, time.Now()); got != "esperando confirmação do plano de teste" {
		t.Fatalf("label %q", got)
	}
}

func TestPipelineQueuesWhenPCOnline(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	startTest(t, h, draftID)
	confirmPlan(t, h, appID, draftID, "PC")
	h.testPipelineTick(context.Background(), time.Now())
	d := draftRow(t, h, draftID)
	if d.Stage != "rodando" || testJobs(t, draftID) != 1 || !d.TestJobID.Valid {
		t.Fatalf("stage %q jobs %d", d.Stage, testJobs(t, draftID))
	}
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	if job.Kind != agentproto.KindTest || !strings.Contains(string(job.Payload), `"draft_id":`+itoa(draftID)) {
		t.Fatalf("job %+v", job)
	}
	if strings.Contains(draftStageLabel(d, nil, nil, time.Now()), "testando no PC") == false {
		t.Fatalf("label %q", draftStageLabel(d, nil, nil, time.Now()))
	}
}

func TestPipelineWaitsOldAgent(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.3.5"}, nil)
	startTest(t, h, draftID)
	confirmPlan(t, h, appID, draftID, "PC")
	h.testPipelineTick(context.Background(), time.Now())
	d := draftRow(t, h, draftID)
	if d.Stage != "esperando_pc" || testJobs(t, draftID) != 0 {
		t.Fatalf("stage %q", d.Stage)
	}
	if got := h.draftLabel(context.Background(), d); got != "esperando o Agente do PC de teste se atualizar" {
		t.Fatalf("label %q", got)
	}
}

func TestPipelineFailsAfter24h(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.3.5"}, nil)
	startTest(t, h, draftID)
	confirmPlan(t, h, appID, draftID, "PC")
	h.testPipelineTick(context.Background(), time.Now())
	h.testPipelineTick(context.Background(), time.Now().Add(25*time.Hour))
	if d := draftRow(t, h, draftID); d.Status != "reprovada" || d.StatusReason != "O PC de teste ficou 24 h sem ligar ao Ziv." {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
}

func TestPipelineResumesAfterRestart(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, _ := draftWithPC(t, c, h)
	if _, err := testPool.Exec(context.Background(), "UPDATE app_drafts SET status='aprovada', stage='publicando', stage_since=now() WHERE id=$1", draftID); err != nil {
		t.Fatal(err)
	}
	h2 := New(sqlc.New(testPool), config.Config{BaseURL: "http://hub.test", SetupCode: testSetupCode})
	h2.testPipelineTick(context.Background(), time.Now())
	if d := draftRow(t, h, draftID); d.Status != "no_ar" {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
	if storedNames(t, appID)["main.py"] != "print('v2')" {
		t.Fatal("not published")
	}
}

// passTest runs the whole pipeline with the fake agent reporting res.
func passTest(t *testing.T, c *client, h *Handler, appID, draftID int64, token string, res agentproto.Result) {
	t.Helper()
	startTest(t, h, draftID)
	confirmPlan(t, h, appID, draftID, "PC")
	h.testPipelineTick(context.Background(), time.Now())
	var job agentproto.Job
	if code := c.agent("GET", "/api/agent/next", token, nil, &job); code != http.StatusOK || job.Kind != agentproto.KindTest {
		t.Fatalf("next %d %+v", code, job)
	}
	if code := agentResultFor(c, token, job.ID, res); code != http.StatusNoContent {
		t.Fatalf("result %d", code)
	}
}

var pdfResult = agentproto.Result{Event: agentproto.EventFinished, Log: "ok", Files: []agentproto.FileEntry{{Path: "a.pdf", Size: 3}}}

func TestPassedTestPublishesAndNotifies(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	before := versionCount(t, appID)
	passTest(t, c, h, appID, draftID, token, pdfResult)
	d := draftRow(t, h, draftID)
	if d.Status != "no_ar" || !d.VersionNumber.Valid {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
	if versionCount(t, appID) != before+1 {
		t.Fatal("no new version")
	}
	var tested bool
	var channel string
	testPool.QueryRow(context.Background(), "SELECT tested, channel FROM app_versions WHERE app_id=$1 AND number=$2", appID, d.VersionNumber.Int32).Scan(&tested, &channel)
	if !tested || channel != "manual" {
		t.Fatalf("tested %v channel %q", tested, channel)
	}
	want := "Soma versão " + itoa(int64(d.VersionNumber.Int32)) + " no ar — testada: 1 arquivo salvo"
	if got := noticeTexts(t, appID); len(got) != 1 || got[0] != want {
		t.Fatalf("notices %v want %q", got, want)
	}
}

func TestFailedTestChangesNothing(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	before, versions := appManifestRaw(t, appID), versionCount(t, appID)
	fail := pdfResult
	fail.ExitCode = 1
	passTest(t, c, h, appID, draftID, token, fail)
	h.testPipelineTick(context.Background(), time.Now())
	if d := draftRow(t, h, draftID); d.Status != "reprovada" {
		t.Fatalf("%q", d.Status)
	}
	if storedNames(t, appID)["main.py"] != "print('v1')" || appManifestRaw(t, appID) != before || versionCount(t, appID) != versions {
		t.Fatal("live app changed")
	}
	if n := noticeTexts(t, appID); len(n) != 0 {
		t.Fatalf("notices %v", n)
	}
}

func TestPublishWaitsForRun(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	var me int64
	testPool.QueryRow(context.Background(), "SELECT id FROM people ORDER BY id LIMIT 1").Scan(&me)
	run, err := h.q.CreateRun(context.Background(), sqlc.CreateRunParams{AppID: appID, PersonID: me, ComputerID: computerID(t, "PC"), OutputFolder: `C:\R`, Origin: "manual", Inputs: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	passTest(t, c, h, appID, draftID, token, pdfResult)
	if d := draftRow(t, h, draftID); d.Stage != "publicando" {
		t.Fatalf("should wait: %q %q %q", d.Status, d.Stage, d.StatusReason)
	}
	testPool.Exec(context.Background(), "UPDATE runs SET status='done' WHERE id=$1", run.ID)
	h.testPipelineTick(context.Background(), time.Now())
	if d := draftRow(t, h, draftID); d.Status != "no_ar" {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
}

func TestSourcesCancelsOpenDraft(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, _ := draftWithPC(t, c, h)
	startTest(t, h, draftID)
	if code, out := send(t, c, appID, map[string]string{"main.py": "print('v3')"}, nil); code != http.StatusOK {
		t.Fatalf("sources %d %+v", code, out)
	}
	if d := draftRow(t, h, draftID); d.Status != "cancelada" || d.StatusReason != "substituída por outra versão no ar" || d.Stage != "" {
		t.Fatalf("%q %q %q", d.Status, d.StatusReason, d.Stage)
	}
}

func TestRestoreCancelsOpenDraft(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, _ := draftWithPC(t, c, h)
	if code, _ := send(t, c, appID, map[string]string{"main.py": "print('v1b')"}, nil); code != http.StatusOK {
		t.Fatal("sources")
	}
	code, out := sendDraft(t, c, appID, map[string]string{"main.py": "print('v2')"}, map[string]any{"ficha": testeFicha()})
	if code != http.StatusCreated {
		t.Fatal("draft")
	}
	draftID = out.DraftID
	startTest(t, h, draftID)
	if code := c.do("POST", appPath(appID, "/versions/1/restore"), nil, nil); code != http.StatusOK {
		t.Fatalf("restore %d", code)
	}
	if d := draftRow(t, h, draftID); d.Status != "cancelada" || d.StatusReason != "substituída por outra versão no ar" {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
}

func TestUnchangedDraftMarksCurrentTested(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID := importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	_, out := sendDraft(t, c, appID, map[string]string{"main.py": "print('v1')"}, map[string]any{"ficha": testeFicha()})
	versions := versionCount(t, appID)
	passTest(t, c, h, appID, out.DraftID, token, pdfResult)
	if d := draftRow(t, h, out.DraftID); d.Status != "no_ar" {
		t.Fatalf("%q %q", d.Status, d.StatusReason)
	}
	var tested *bool
	testPool.QueryRow(context.Background(), "SELECT tested FROM app_versions WHERE app_id=$1 ORDER BY number DESC LIMIT 1", appID).Scan(&tested)
	if versionCount(t, appID) != versions || tested == nil || !*tested {
		t.Fatalf("versions %d→%d tested %v", versions, versionCount(t, appID), tested)
	}
}

func TestInTestAppRefusesRuns(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, _, _ := draftWithPC(t, c, h)
	if err := h.q.SetInTest(context.Background(), appID); err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	code := c.do("POST", appPath(appID, "/runs"), map[string]any{"output_folder": `C:\R`, "inputs": map[string][]string{"amount": {"1"}}}, &out)
	if code != http.StatusConflict || out["error"] != "Este app ainda está em teste. Ele fica disponível depois que o primeiro teste passar." {
		t.Fatalf("%d %v", code, out)
	}
}
