package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// testeManifesto is scriptManifesto with a test plan that expects one PDF.
func testeManifesto() map[string]any {
	m := scriptManifesto()
	m["teste"] = map[string]any{
		"acessa": []string{"nada externo"}, "pula": "nada",
		"entradas": map[string]string{"amount": "1"},
		"espera":   []map[string]any{{"arquivos": "*.pdf"}},
	}
	return m
}

func enviarVersao(t *testing.T, s *mcp.ClientSession, args map[string]any) (*mcp.CallToolResult, string, map[string]any) {
	t.Helper()
	return callTool(t, s, "enviar_versao", args)
}

// enviarNovo sends Conta CNPJs as a new app under test; it returns app and draft.
func enviarNovo(t *testing.T, h *Handler, s *mcp.ClientSession) (int64, int64, string) {
	t.Helper()
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v1')\n"})
	res, text, out := enviarVersao(t, s, map[string]any{"envio": envio, "manifesto": testeManifesto()})
	if res.IsError {
		t.Fatalf("enviar_versao: %s", text)
	}
	id := int64(out["app_id"].(float64))
	t.Cleanup(func() { cleanupAppFolder(t, id) })
	return id, int64(out["versao"].(float64)), text
}

func TestEnviarVersaoCommandFirst(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	res, text, out := enviarVersao(t, s, map[string]any{"manifesto": testeManifesto()})
	if res.IsError || out["envio"] == nil || out["comando_windows"] == nil {
		t.Fatalf("%s %+v", text, out)
	}
	if !strings.Contains(text, `Quando o curl mostrar "recebido": true, chame enviar_versao de novo com envio `) || strings.Contains(text, "registrar_app") {
		t.Fatalf("text %q", text)
	}
}

func TestEnviarVersaoNewAppInTest(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	appID, draftID, text := enviarNovo(t, h, s)
	a, err := h.q.GetApp(context.Background(), appID)
	if err != nil || !a.InTest || !a.Restricted {
		t.Fatalf("app %v in_test=%v", err, a.InTest)
	}
	d := draftRow(t, h, draftID)
	if d.Status != "testando" || d.Stage != "esperando_confirmacao" || d.Channel != "mcp" {
		t.Fatalf("draft %q %q %q", d.Status, d.Stage, d.Channel)
	}
	link := "http://hub.test/apps/" + itoa(appID) + "/plano-de-teste"
	if !strings.Contains(text, "Mande este link ao responsável pelo app para confirmar o plano de teste: "+link+". Depois acompanhe com ver_versao.") {
		t.Fatalf("text %q", text)
	}
}

func TestEnviarVersaoNeedsTeste(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v1')\n"})
	res, text, _ := enviarVersao(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()})
	if !res.IsError || text != "Falta a seção teste da receita. Escreva a seção teste; veja receita_app_ziv." {
		t.Fatalf("%v %q", res.IsError, text)
	}
	if n := countRows(t, "SELECT count(*) FROM apps"); n != 0 {
		t.Fatalf("apps %d", n)
	}
}

func TestEnviarVersaoRefusesWeb(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"app.py": "print('v1')\n"})
	m := testeManifesto()
	m["kind"], m["entrypoint"] = "web", "app.py"
	res, text, _ := enviarVersao(t, s, map[string]any{"envio": envio, "manifesto": m})
	if !res.IsError || text != "Apps com tela ainda usam registrar_app e publicar_app_com_tela; enviar_versao chega para eles na próxima etapa." {
		t.Fatalf("%v %q", res.IsError, text)
	}
}

func TestEnviarVersaoExistingApp(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	appID := importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v2')\n"})
	res, text, out := enviarVersao(t, s, map[string]any{"envio": envio, "app_id": appID, "manifesto": map[string]any{"teste": testeManifesto()["teste"]}, "nota": "PDF novo"})
	if res.IsError {
		t.Fatalf("%s", text)
	}
	d := draftRow(t, h, int64(out["versao"].(float64)))
	if d.Stage != "esperando_confirmacao" || d.Note != "PDF novo" || storedNames(t, appID)["main.py"] != "print('v1')" {
		t.Fatalf("draft %q %q", d.Stage, d.Note)
	}
}

func TestVerVersaoShowsChecks(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	appID, draftID, _ := enviarNovo(t, h, s)
	confirmPlan(t, h, appID, draftID, "PC")
	h.testPipelineTick(context.Background(), time.Now())
	var job agentproto.Job
	if code := c.agent("GET", "/api/agent/next", token, nil, &job); code != http.StatusOK || job.Kind != agentproto.KindTest {
		t.Fatalf("next %d %+v", code, job)
	}
	agentResultFor(c, token, job.ID, agentproto.Result{Event: agentproto.EventFinished, Log: "Concluído\n", Files: []agentproto.FileEntry{{Path: "relatorio.pdf", Size: 10}}})
	res, text, out := callTool(t, s, "ver_versao", map[string]any{"app_id": appID, "versao": draftID})
	if res.IsError {
		t.Fatalf("%s", text)
	}
	for _, want := range []string{"✓ código de saída 0", "✓ arquivos *.pdf: 1 (mínimo 1)", "Concluído", "relatorio.pdf"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if out["situacao"] != "no ar" {
		t.Fatalf("%+v", out)
	}
}

func TestVerVersaoHidesKey(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	s := toolSession(t, c)
	appID := importKeyScript(t, c)
	c.do("PUT", "/api/apps/"+itoa(appID)+"/keys/BTG_SENHA", map[string]string{"value": keyValue}, nil)
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	_, out := sendDraft(t, c, appID, map[string]string{"main.py": "print(2)"}, map[string]any{"ficha": testeFicha()})
	job := runningTestJob(t, c, h, appID, out.DraftID, token, "PC")
	agentResultFor(c, token, job, agentproto.Result{Event: agentproto.EventFinished, ExitCode: 1, Log: "senha=" + keyValue})
	_, text, _ := callTool(t, s, "ver_versao", map[string]any{"app_id": appID, "versao": out.DraftID})
	if strings.Contains(text, keyValue) || !strings.Contains(text, "✗") {
		t.Fatalf("%q", text)
	}
}

func TestBaixarArquivoTesteText(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	appID, draftID, token := draftWithPC(t, c, h)
	job := runningTestJob(t, c, h, appID, draftID, token, "PC")
	postEvidence(t, c, token, job, evidenceZipOf(t, map[string]string{"saida/relatorio.csv": "a;b\n1;2\n", "foto.png": "\x89PNG\x00\x01"}))
	res, text, _ := callTool(t, s, "baixar_arquivo_teste", map[string]any{"app_id": appID, "versao": draftID, "caminho": "saida/relatorio.csv"})
	if res.IsError || !strings.Contains(text, "a;b\n1;2") {
		t.Fatalf("%v %q", res.IsError, text)
	}
	res, text, _ = callTool(t, s, "baixar_arquivo_teste", map[string]any{"app_id": appID, "versao": draftID, "caminho": "foto.png"})
	if res.IsError || strings.Contains(text, "PNG") || !strings.Contains(text, "http://hub.test/apps/"+itoa(appID)) {
		t.Fatalf("binary %v %q", res.IsError, text)
	}
}

func TestRegistrarRefusesScript(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v1')\n"})
	res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()})
	want := "Scripts agora vão por enviar_versao, que testa antes de pôr no ar. Escreva a seção teste da receita (veja receita_app_ziv) e chame enviar_versao com o mesmo envio."
	if !res.IsError || text != want {
		t.Fatalf("%v %q", res.IsError, text)
	}
	appID := importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "app_id": appID}); !res.IsError || text != want {
		t.Fatalf("existing %v %q", res.IsError, text)
	}
}

func TestMCPCannotConfirmPlan(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	_, _, _ = enviarNovo(t, h, s)
	for _, r := range [][2]string{{"POST", "/api/apps/1/test-plan/confirm"}, {"PUT", "/api/apps/1/test-plan/computer"}, {"GET", "/api/apps/1/test-plan"}, {"POST", "/api/apps/1/drafts/2/publish"}} {
		if mcpRouteAllowed(r[0], r[1]) {
			t.Fatalf("%s %s allowed", r[0], r[1])
		}
	}
	for _, r := range [][2]string{{"POST", "/api/apps/1/drafts"}, {"POST", "/api/apps/1/drafts/2/test"}, {"GET", "/api/apps/1/drafts/2"}, {"GET", "/api/apps/1/drafts/2/files"}} {
		if !mcpRouteAllowed(r[0], r[1]) {
			t.Fatalf("%s %s refused", r[0], r[1])
		}
	}
	tools, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if strings.Contains(tool.Name, "confirm") || strings.Contains(tool.Name, "plano") {
			t.Fatalf("tool %s", tool.Name)
		}
	}
}

// A version that waited for a key starts its test with app_id and versao,
// without a new envio.
func TestEnviarVersaoRestartAfterKeys(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	s := toolSession(t, c)
	m := testeManifesto()
	m["keys"] = []any{map[string]any{"name": "BTG_SENHA", "label": "Senha do BTG", "required": true}}
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "import os\nprint(len(os.environ['BTG_SENHA']))\n"})
	res, text, out := enviarVersao(t, s, map[string]any{"envio": envio, "manifesto": m})
	if res.IsError || out["etapa"] != "" {
		t.Fatalf("first %v %q %+v", res.IsError, text, out)
	}
	id, versao := int64(out["app_id"].(float64)), int64(out["versao"].(float64))
	t.Cleanup(func() { cleanupAppFolder(t, id) })
	if !strings.Contains(text, "chame enviar_versao com app_id "+itoa(id)+" e versao "+itoa(versao)) {
		t.Fatalf("text %q", text)
	}
	if code := c.do("PUT", "/api/apps/"+itoa(id)+"/keys/BTG_SENHA", map[string]string{"value": "segredo-btg-123"}, nil); code != http.StatusOK {
		t.Fatalf("put key %d", code)
	}
	res, text, out = enviarVersao(t, s, map[string]any{"app_id": id, "versao": versao})
	if res.IsError || out["situacao"] == nil || draftRow(t, h, versao).Stage != "esperando_confirmacao" {
		t.Fatalf("restart %v %q %+v", res.IsError, text, out)
	}
	if strings.Contains(text, "segredo-btg-123") {
		t.Fatal("key value in text")
	}
}
