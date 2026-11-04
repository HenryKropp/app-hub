package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// toolSession connects to the client's /mcp with a fresh personal code.
func toolSession(t *testing.T, c *client) *mcp.ClientSession {
	t.Helper()
	code := newMCPCode(t, c)
	s, err := mcpSession(t, c.srv.URL+"/mcp", code.Code)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// callTool calls one tool and returns the result, its text and its structured content.
func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, string, map[string]any) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	var out map[string]any
	if res.StructuredContent != nil {
		raw, _ := json.Marshal(res.StructuredContent)
		_ = json.Unmarshal(raw, &out)
	}
	return res, text.String(), out
}

func maxAuditID(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := testPool.QueryRow(context.Background(), `SELECT COALESCE(max(id),0) FROM security_audit`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// claudeAudits counts the audit rows written by MCP tools since id after.
func claudeAudits(t *testing.T, after int64, route string, actor int64) int {
	t.Helper()
	var n int
	err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM security_audit WHERE id > $1 AND action = 'mcp.request.completed' AND route = $2 AND actor_id = $3 AND status BETWEEN 200 AND 299`,
		after, route, actor).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestToolBuscarRespectsAccess(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	m := filesPrepManifest()
	m.ID, m.Name = "analise-credito", "Análise de Crédito"
	app := insertApp(t, m)
	if _, err := testPool.Exec(context.Background(), `UPDATE apps SET restricted = true, summary = 'Fecha o mês' WHERE id = $1`, app); err != nil {
		t.Fatal(err)
	}
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")

	link := fmt.Sprintf("http://hub.test/apps/%d", app)
	owner := toolSession(t, c)
	res, text, out := callTool(t, owner, "buscar", map[string]any{"texto": "ANALISE"})
	if res.IsError || !strings.Contains(text, "Análise de Crédito") || !strings.Contains(text, link) {
		t.Fatalf("owner: %v %q", res.IsError, text)
	}
	items, _ := out["itens"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["link"] != link || items[0].(map[string]any)["tipo"] != "app" {
		t.Fatalf("owner items %v", out)
	}
	if _, text, _ := callTool(t, owner, "buscar", map[string]any{"texto": "analise", "tipo": "artifact"}); strings.Contains(text, "Análise") {
		t.Fatalf("tipo artifact found the app: %q", text)
	}

	other := toolSession(t, ana)
	res, text, out = callTool(t, other, "buscar", map[string]any{"texto": "análise"})
	if res.IsError || strings.Contains(text, "Análise de Crédito") {
		t.Fatalf("no access: %v %q", res.IsError, text)
	}
	if items, _ := out["itens"].([]any); len(items) != 0 {
		t.Fatalf("no access items %v", out)
	}
}

func TestToolRodarSemComputador(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	s := toolSession(t, c)
	res, text, _ := callTool(t, s, "rodar_app", map[string]any{"id": app})
	if !res.IsError || !strings.Contains(text, "computador não está conectado") {
		t.Fatalf("%v %q", res.IsError, text)
	}
}

func TestToolRodarEVerExecucao(t *testing.T) {
	c, _ := newPoolClient(t)
	admin := setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")
	s := toolSession(t, c)
	before := maxAuditID(t)

	res, text, out := callTool(t, s, "rodar_app", map[string]any{"id": app})
	if res.IsError {
		t.Fatalf("rodar: %q", text)
	}
	runID := int64(out["execucao_id"].(float64))
	if out["link"] != fmt.Sprintf("http://hub.test/execucoes/%d", runID) || out["situacao"] == "" {
		t.Fatalf("rodar out %v", out)
	}
	var origin string
	if err := testPool.QueryRow(context.Background(), `SELECT origin FROM runs WHERE id = $1`, runID).Scan(&origin); err != nil || origin != "mcp" {
		t.Fatalf("origin %q %v", origin, err)
	}
	if n := claudeAudits(t, before, "POST /api/apps/{id}/runs", admin.ID); n != 1 {
		t.Fatalf("audit rows = %d", n)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(300 * time.Millisecond)
		var job agentproto.Job
		c.agent("GET", "/api/agent/next", token, nil, &job)
		files := []agentproto.FileEntry{{Path: "PETROBRAS/FRE/FRE_2025.pdf", Size: 10, Modified: time.Now(), Change: "new"}}
		c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventFinished, Files: files}, nil)
	}()
	start := time.Now()
	res, text, out = callTool(t, s, "ver_execucao", map[string]any{"id": runID, "esperar": 2})
	<-done
	if res.IsError || !strings.Contains(text, "Concluída") || !strings.Contains(text, "FRE_2025.pdf") {
		t.Fatalf("ver: %v %q", res.IsError, text)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("waited too long: %s", time.Since(start))
	}
	arquivos, _ := out["arquivos"].([]any)
	if len(arquivos) != 1 || arquivos[0].(map[string]any)["caminho"] != `C:\apps\eq-research-files-prep\PETROBRAS\FRE\FRE_2025.pdf` {
		t.Fatalf("arquivos %v", out)
	}

	res, text, _ = callTool(t, s, "minhas_execucoes", map[string]any{})
	if res.IsError || !strings.Contains(text, "Eq. Research Files Prep") || !strings.Contains(text, fmt.Sprintf("/execucoes/%d", runID)) {
		t.Fatalf("minhas: %q", text)
	}
}

func TestToolPublicarArtifact(t *testing.T) {
	c, _ := newPoolClient(t)
	admin := setupAdmin(t, c)
	s := toolSession(t, c)
	before := maxAuditID(t)
	html := "<!doctype html><html><body><h1>Painel</h1></body></html>"

	res, text, out := callTool(t, s, "publicar_artifact", map[string]any{"nome": "Painel", "html": html, "resumo": "Um painel", "etiquetas": []string{"teste"}, "area": "investimentos"})
	if res.IsError {
		t.Fatalf("criar: %q", text)
	}
	id := int64(out["artifact_id"].(float64))
	if out["versao"].(float64) != 1 || out["link"] != fmt.Sprintf("http://hub.test/artifacts/%d", id) {
		t.Fatalf("criar out %v", out)
	}
	var origin string
	var area int64
	if err := testPool.QueryRow(context.Background(), `SELECT origin, area_id FROM artifacts WHERE id = $1`, id).Scan(&origin, &area); err != nil || origin != "mcp" || area != 2 {
		t.Fatalf("origin %q area %d %v", origin, area, err)
	}

	res, text, out = callTool(t, s, "publicar_artifact", map[string]any{"html": html + "<p>2</p>", "artifact_id": id, "nota": "segunda"})
	if res.IsError || out["versao"].(float64) != 2 || int64(out["artifact_id"].(float64)) != id {
		t.Fatalf("versão: %v %q %v", res.IsError, text, out)
	}
	if n := claudeAudits(t, before, "POST /api/artifacts", admin.ID); n != 1 {
		t.Fatalf("create audit rows = %d", n)
	}
	if n := claudeAudits(t, before, "POST /api/artifacts/{id}/versions", admin.ID); n != 1 {
		t.Fatalf("version audit rows = %d", n)
	}

	res, text, _ = callTool(t, s, "ver_artifact", map[string]any{"id": id})
	if res.IsError || !strings.Contains(text, "Painel") || !strings.Contains(text, "teste") {
		t.Fatalf("ver_artifact: %q", text)
	}
}

func TestToolPublicarSegredos(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	html := `<html><script>const k = "sk-ant-` + strings.Repeat("a1B2", 10) + `";</script></html>`
	if res, text, _ := callTool(t, s, "publicar_artifact", map[string]any{"html": "<html></html>"}); !res.IsError || !strings.Contains(text, "nome") {
		t.Fatalf("create without nome: %v %q", res.IsError, text)
	}
	res, text, _ := callTool(t, s, "publicar_artifact", map[string]any{"nome": "Com chave", "html": html})
	if !res.IsError || !strings.Contains(text, "confirmar_segredos") {
		t.Fatalf("%v %q", res.IsError, text)
	}
	res, text, out := callTool(t, s, "publicar_artifact", map[string]any{"nome": "Com chave", "html": html, "confirmar_segredos": true})
	if res.IsError || out["versao"].(float64) != 1 {
		t.Fatalf("confirmed: %q", text)
	}
}

func TestToolGuiaVisual(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	res, text, _ := callTool(t, s, "guia_visual", map[string]any{})
	if res.IsError || !strings.Contains(text, "Geist") || !strings.Contains(text, "#FF6B4A") {
		t.Fatalf("%q", text)
	}
}

// Only rodar_app and publicar_artifact change something.
func TestToolAnnotations(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	list, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"buscar": true, "ver_app": true, "ver_artifact": true, "rodar_app": false, "ver_execucao": true,
		"minhas_execucoes": true, "publicar_artifact": false, "guia_visual": true}
	if len(list.Tools) != len(want) {
		t.Fatalf("tools = %d", len(list.Tools))
	}
	for _, tool := range list.Tools {
		ro, ok := want[tool.Name]
		if !ok || tool.Annotations == nil || tool.Annotations.ReadOnlyHint != ro {
			t.Fatalf("%s: annotations %+v", tool.Name, tool.Annotations)
		}
		if !ro && (tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint) {
			t.Fatalf("%s: destructiveHint must be false", tool.Name)
		}
	}
}

// ver_app shows the ficha from an explicit list of fields, never secrets.
func TestToolVerApp(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	m := filesPrepManifest()
	m.Secrets = []manifest.SecretRef{{Name: "ZIVREF_NOME_SEGREDO", File: "config_segredo.py", Line: 7, Hint: "dica-do-cofre-xyz"}}
	m.Shared = &manifest.Shared{Env: []manifest.EnvVar{{Name: "ZIVENV_TOKEN_API", Label: "Token da API", Secret: true, Default: "valor-padrao-secreto-123"}}}
	app := insertApp(t, m)
	if _, err := testPool.Exec(context.Background(), `UPDATE apps SET name = 'Nome no Catálogo', summary = 'Resumo do catálogo' WHERE id = $1`, app); err != nil {
		t.Fatal(err)
	}
	s := toolSession(t, c)
	res, text, out := callTool(t, s, "ver_app", map[string]any{"id": app})
	if res.IsError || !strings.Contains(text, "Nome no Catálogo") || !strings.Contains(text, "Resumo do catálogo") ||
		out["nome"] != "Nome no Catálogo" || out["resumo"] != "Resumo do catálogo" ||
		out["link"] != fmt.Sprintf("http://hub.test/apps/%d", app) || out["pode_rodar"] != true {
		t.Fatalf("%v %q %v", res.IsError, text, out)
	}
	if _, ok := out["manifest"]; ok {
		t.Fatalf("raw manifest passed through: %v", out)
	}
	raw, _ := json.Marshal(out)
	for _, secret := range []string{"ZIVREF_NOME_SEGREDO", "config_segredo.py", "dica-do-cofre-xyz", "ZIVENV_TOKEN_API", "Token da API", "valor-padrao-secreto-123"} {
		if strings.Contains(text, secret) || strings.Contains(string(raw), secret) {
			t.Fatalf("ver_app leaked %q: %q %s", secret, text, raw)
		}
	}
	res, text, _ = callTool(t, s, "ver_app", map[string]any{"id": app + 99})
	if !res.IsError || text == "" {
		t.Fatalf("missing app: %v %q", res.IsError, text)
	}
}
