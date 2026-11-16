package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

type testPlanJSON struct {
	Plan *struct {
		Acessa []string `json:"acessa"`
		Pula   string   `json:"pula"`
		Keys   []string `json:"keys"`
		Place  string   `json:"place"`
	} `json:"plan"`
	Fingerprint string `json:"fingerprint"`
	Confirmed   *struct {
		By         string `json:"by"`
		ComputerID int64  `json:"computer_id"`
	} `json:"confirmed"`
	Matches   bool `json:"matches"`
	Computers []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"computers"`
	CanConfirm bool `json:"can_confirm"`
}

type draftViewJSON struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	StatusReason string `json:"status_reason"`
	Stage        string `json:"stage"`
	StageLabel   string `json:"stage_label"`
	ConfirmURL   string `json:"confirm_url"`
}

func appPathf(id int64, rest string) string { return "/api/apps/" + itoa(id) + rest }

func errorOf(t *testing.T, c *client, method, path string, body any) (int, map[string]string) {
	t.Helper()
	var out map[string]string
	code := c.do(method, path, body, &out)
	return code, out
}

// sentDraft imports the Soma script and sends a draft with the given ficha.
func sentDraft(t *testing.T, c *client, ficha map[string]any) (int64, int64) {
	t.Helper()
	appID := importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	extra := map[string]any{}
	if ficha != nil {
		extra["ficha"] = ficha
	}
	code, out := sendDraft(t, c, appID, map[string]string{"main.py": "print('v2')"}, extra)
	if code != http.StatusCreated {
		t.Fatalf("draft %d %+v", code, out)
	}
	return appID, out.DraftID
}

func TestStartTestNeedsPlan(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID := sentDraft(t, c, nil)
	code, out := errorOf(t, c, "POST", appPathf(appID, "/drafts/"+itoa(draftID)+"/test"), nil)
	if code != http.StatusConflict || out["code"] != "test_plan_missing" ||
		out["error"] != "Falta a seção teste da receita. Escreva a seção teste; veja receita_app_ziv." {
		t.Fatalf("%d %v", code, out)
	}
}

func TestStartTestRefusesWeb(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID := sentDraft(t, c, testeFicha())
	exec(t, `UPDATE apps SET manifest = jsonb_set(manifest, '{kind}', '"web"') WHERE id = $1`, appID)
	_ = h
	code, out := errorOf(t, c, "POST", appPathf(appID, "/drafts/"+itoa(draftID)+"/test"), nil)
	if code != http.StatusConflict || out["code"] != "test_kind" ||
		out["error"] != "Por enquanto o teste automático é só para scripts. Apps com tela ainda publicam pelo caminho de hoje." {
		t.Fatalf("%d %v", code, out)
	}
}

func TestStartTestGoesToConfirmation(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID := sentDraft(t, c, testeFicha())
	var v draftViewJSON
	if code := c.do("POST", appPathf(appID, "/drafts/"+itoa(draftID)+"/test"), nil, &v); code != http.StatusOK {
		t.Fatalf("start %d", code)
	}
	if v.Status != "testando" || v.Stage != "esperando_confirmacao" || v.StageLabel != "esperando confirmação do plano de teste" ||
		v.ConfirmURL != "/apps/"+itoa(appID)+"/plano-de-teste" {
		t.Fatalf("%+v", v)
	}
	code, out := errorOf(t, c, "POST", appPathf(appID, "/drafts/"+itoa(draftID)+"/test"), nil)
	if code != http.StatusConflict || out["code"] != "draft_closed" {
		t.Fatalf("second start %d %v", code, out)
	}
}

func TestStartTestInvalidInputs(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	f := testeFicha()
	f["teste"].(map[string]any)["entradas"] = map[string]string{}
	appID, draftID := sentDraft(t, c, f)
	code, out := errorOf(t, c, "POST", appPathf(appID, "/drafts/"+itoa(draftID)+"/test"), nil)
	if code != http.StatusBadRequest || out["code"] != "invalid_inputs" {
		t.Fatalf("%d %v", code, out)
	}
}

// startAndPlan starts the test of the draft and reads the plan screen.
func startAndPlan(t *testing.T, c *client, appID, draftID int64) testPlanJSON {
	t.Helper()
	if code := c.do("POST", appPathf(appID, "/drafts/"+itoa(draftID)+"/test"), nil, nil); code != http.StatusOK {
		t.Fatalf("start %d", code)
	}
	var p testPlanJSON
	if code := c.do("GET", appPathf(appID, "/test-plan"), nil, &p); code != http.StatusOK {
		t.Fatalf("plan %d", code)
	}
	return p
}

func TestTestPlanScreen(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	appID, draftID := sentDraft(t, c, testeFicha())
	p := startAndPlan(t, c, appID, draftID)
	if p.Plan == nil || p.Plan.Pula != "nada" || len(p.Plan.Acessa) != 1 || p.Plan.Place != "pc" ||
		p.Fingerprint == "" || p.Confirmed != nil || p.Matches || !p.CanConfirm || len(p.Computers) != 1 {
		t.Fatalf("%+v", p)
	}
}

func TestConfirmPlanSiteOnly(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	pairComputer(t, c, "PC")
	appID, draftID := sentDraft(t, c, testeFicha())
	p := startAndPlan(t, c, appID, draftID)
	if mcpRouteAllowed("POST", "/api/apps/1/test-plan/confirm") || mcpRouteAllowed("PUT", "/api/apps/1/test-plan/computer") {
		t.Fatal("confirm reachable by MCP")
	}
	mux := http.NewServeMux()
	h.Register(mux)
	body, _ := json.Marshal(map[string]any{"fingerprint": p.Fingerprint, "computer_id": p.Computers[0].ID})
	req := httptest.NewRequest("POST", appPathf(appID, "/test-plan/confirm"), bytes.NewReader(body))
	req = req.WithContext(withMCPPerson(req.Context(), admin.ID))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mcp confirm %d %s", rec.Code, rec.Body)
	}
	if _, err := h.q.GetTestPlan(context.Background(), appID); err == nil {
		t.Fatal("plan stored via MCP")
	}
}

func TestConfirmWrongFingerprint(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	appID, draftID := sentDraft(t, c, testeFicha())
	p := startAndPlan(t, c, appID, draftID)
	code, out := errorOf(t, c, "POST", appPathf(appID, "/test-plan/confirm"), map[string]any{"fingerprint": "outra", "computer_id": p.Computers[0].ID})
	if code != http.StatusConflict || out["error"] != "Este plano mudou. Recarregue a página." {
		t.Fatalf("%d %v", code, out)
	}
}

func TestConfirmOtherPersonsComputer(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID := sentDraft(t, c, testeFicha())
	createPersonAs(t, c, "Ana", "ana@exemplo.com", "criador")
	ana := c.fresh()
	loginAs(t, ana, "ana@exemplo.com")
	pairComputer(t, ana, "PC-ANA")
	p := startAndPlan(t, c, appID, draftID)
	code, _ := errorOf(t, c, "POST", appPathf(appID, "/test-plan/confirm"), map[string]any{"fingerprint": p.Fingerprint, "computer_id": computerID(t, "PC-ANA")})
	if code != http.StatusBadRequest {
		t.Fatalf("other's computer %d", code)
	}
	if _, err := h.q.GetTestPlan(context.Background(), appID); err == nil {
		t.Fatal("plan stored")
	}
}

func TestConfirmMovesDraftOn(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	appID, draftID := sentDraft(t, c, testeFicha())
	p := startAndPlan(t, c, appID, draftID)
	var after testPlanJSON
	if code := c.do("POST", appPathf(appID, "/test-plan/confirm"), map[string]any{"fingerprint": p.Fingerprint, "computer_id": p.Computers[0].ID}, &after); code != http.StatusOK {
		t.Fatalf("confirm %d", code)
	}
	if after.Confirmed == nil || !after.Matches || after.Confirmed.ComputerID != p.Computers[0].ID {
		t.Fatalf("%+v", after)
	}
	// The PC never said hello: the draft waits for it.
	if v := getDraftView(t, c, appID, draftID); v.Stage != "esperando_pc" {
		t.Fatalf("%+v", v)
	}
}

func getDraftView(t *testing.T, c *client, appID, draftID int64) draftViewJSON {
	t.Helper()
	var v draftViewJSON
	if code := c.do("GET", appPathf(appID, "/drafts/"+itoa(draftID)), nil, &v); code != http.StatusOK {
		t.Fatalf("draft %d", code)
	}
	return v
}

func TestPlanChangeNeedsNewConfirm(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC")
	appID, draftID := sentDraft(t, c, testeFicha())
	p := startAndPlan(t, c, appID, draftID)
	c.do("POST", appPathf(appID, "/test-plan/confirm"), map[string]any{"fingerprint": p.Fingerprint, "computer_id": p.Computers[0].ID}, nil)

	send := func(ficha map[string]any) int64 {
		code, out := sendDraft(t, c, appID, map[string]string{"main.py": "print('v3')"}, map[string]any{"ficha": ficha})
		if code != http.StatusCreated {
			t.Fatalf("draft %d", code)
		}
		if code := c.do("POST", appPathf(appID, "/drafts/"+itoa(out.DraftID)+"/test"), nil, nil); code != http.StatusOK {
			t.Fatalf("start %d", code)
		}
		return out.DraftID
	}
	other := testeFicha()
	other["teste"].(map[string]any)["acessa"] = []string{"o site do BTG"}
	if v := getDraftView(t, c, appID, send(other)); v.Stage != "esperando_confirmacao" {
		t.Fatalf("acessa changed: %+v", v)
	}
	sameAccess := testeFicha()
	sameAccess["teste"].(map[string]any)["espera"] = []map[string]any{{"arquivos": "*.csv"}}
	if v := getDraftView(t, c, appID, send(sameAccess)); v.Stage != "esperando_pc" {
		t.Fatalf("only espera changed: %+v", v)
	}
}

func TestEvidenceDownloadOwnerOnly(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	job := runningTestJob(t, c, h, appID, draftID, token, "PC")
	if code := postEvidence(t, c, token, job, evidenceZipOf(t, map[string]string{"a.pdf": "conteúdo"})); code != http.StatusNoContent {
		t.Fatalf("evidence %d", code)
	}
	path := appPathf(appID, "/drafts/"+itoa(draftID)+"/files?path=a.pdf")
	if code, body := getText(t, c, path); code != http.StatusOK || body != "conteúdo" {
		t.Fatalf("owner %d %q", code, body)
	}
	createPersonAs(t, c, "Ana", "ana@exemplo.com", "criador")
	ana := c.fresh()
	loginAs(t, ana, "ana@exemplo.com")
	var anaID int64
	testPool.QueryRow(context.Background(), "SELECT id FROM people WHERE email='ana@exemplo.com'").Scan(&anaID)
	grant(t, appID, anaID, "edit")
	if code := ana.do("GET", appPathf(appID, "/drafts/"+itoa(draftID)), nil, nil); code != http.StatusOK {
		t.Fatalf("editor sees the draft %d", code)
	}
	if code, _ := getText(t, ana, path); code != http.StatusForbidden {
		t.Fatalf("editor download %d", code)
	}
}

func TestDraftViewHidesSecret(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	appID := importKeyScript(t, c)
	c.do("PUT", "/api/apps/"+itoa(appID)+"/keys/BTG_SENHA", map[string]string{"value": keyValue}, nil)
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	_, out := sendDraft(t, c, appID, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": testeFicha()})
	job := runningTestJob(t, c, h, appID, out.DraftID, token, "PC")
	agentResultFor(c, token, job, agentproto.Result{Event: agentproto.EventFinished, ExitCode: 1, Log: "senha=" + keyValue, Files: []agentproto.FileEntry{{Path: "a.pdf"}}})
	code, body := getText(t, c, appPathf(appID, "/drafts/"+itoa(out.DraftID)))
	if code != http.StatusOK || strings.Contains(body, keyValue) || !strings.Contains(body, "••••") {
		t.Fatalf("%d %s", code, body)
	}
}

func TestImportInTest(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	body := map[string]any{"manifest": map[string]any{"name": "Novo", "summary": "s", "kind": "script", "entrypoint": "main.py", "area": "Research"},
		"files": b64files(map[string]string{"main.py": "print(1)"}), "in_test": true}
	var out struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", "/api/scripts/import", body, &out); code != http.StatusCreated {
		t.Fatalf("import %d", code)
	}
	t.Cleanup(func() { cleanupAppFolder(t, out.ID) })
	a, err := h.q.GetApp(context.Background(), out.ID)
	if err != nil || !a.InTest || !a.Restricted {
		t.Fatalf("%v in_test=%v restricted=%v", err, a.InTest, a.Restricted)
	}
}

var _ = io.EOF
