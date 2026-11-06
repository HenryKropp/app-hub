package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// zipOf builds a zip from name → content.
func zipOf(t *testing.T, files map[string]string) []byte {
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
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// sendEnvio prepares an envio through s and uploads files to it.
func sendEnvio(t *testing.T, h *Handler, s *mcp.ClientSession, files map[string]string) int64 {
	t.Helper()
	res, text, out := callTool(t, s, "preparar_envio", map[string]any{})
	if res.IsError {
		t.Fatalf("preparar_envio: %s", text)
	}
	if code, body := putEnvio(t, securedServer(t, h), out["endereco"].(string), zipOf(t, files)); code != http.StatusOK {
		t.Fatalf("upload = %d %+v", code, body)
	}
	return int64(out["envio"].(float64))
}

func scriptManifesto() map[string]any {
	return map[string]any{"name": "Conta CNPJs", "summary": "Recebe um valor e entrega um relatório.", "kind": "script",
		"entrypoint": "main.py", "area": "Research",
		"inputs":  []any{map[string]any{"id": "amount", "label": "Valor", "type": "number", "source": "argument", "required": true}},
		"outputs": []any{map[string]any{"label": "Relatório", "pattern": "relatorio.txt"}}}
}

func registrar(t *testing.T, s *mcp.ClientSession, args map[string]any) (*mcp.CallToolResult, string, map[string]any) {
	t.Helper()
	return callTool(t, s, "registrar_app", args)
}

// registrarNovo registers Conta CNPJs and returns its id.
func registrarNovo(t *testing.T, h *Handler, s *mcp.ClientSession) int64 {
	t.Helper()
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v1')\n"})
	res, text, out := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()})
	if res.IsError {
		t.Fatalf("registrar: %s", text)
	}
	id := int64(out["app_id"].(float64))
	t.Cleanup(func() { cleanupAppFolder(t, id) })
	return id
}

func TestRegistrarScriptNovo(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v1')\n"})
	res, text, out := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()})
	if res.IsError {
		t.Fatalf("registrar: %s", text)
	}
	id := int64(out["app_id"].(float64))
	t.Cleanup(func() { cleanupAppFolder(t, id) })
	if !strings.Contains(text, `Registrei o app "Conta CNPJs" (versão 1)`) || !strings.Contains(text, fmt.Sprintf("http://hub.test/apps/%d", id)) {
		t.Fatalf("text %q", text)
	}
	var kind, channel string
	if err := testPool.QueryRow(context.Background(), `SELECT a.kind, v.channel FROM apps a JOIN app_versions v ON v.app_id = a.id AND v.number = 1 WHERE a.id = $1`, id).Scan(&kind, &channel); err != nil {
		t.Fatal(err)
	}
	if kind != "script" || channel != "mcp" {
		t.Fatalf("kind %s channel %s", kind, channel)
	}
	var used bool
	if err := testPool.QueryRow(context.Background(), `SELECT used_at IS NOT NULL FROM mcp_uploads WHERE id = $1`, envio).Scan(&used); err != nil || !used {
		t.Fatalf("used %v %v", used, err)
	}
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()}); !res.IsError || !strings.Contains(text, "já foi usado") {
		t.Fatalf("reuse: %q", text)
	}
}

func cleanupAppFolder(t *testing.T, id int64) {
	var p string
	_ = testPool.QueryRow(context.Background(), "SELECT folder_path FROM apps WHERE id=$1", id).Scan(&p)
	if strings.Contains(p, "apphub-imports") {
		_ = os.RemoveAll(p)
	}
}

func TestRegistrarVersaoNova(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	id := registrarNovo(t, h, s)
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v2')\n"})
	res, text, out := registrar(t, s, map[string]any{"envio": envio, "app_id": id, "nota": "Corrige o total"})
	if res.IsError || out["versao"] != float64(2) || out["mudou"] != true || !strings.Contains(text, `Mandei a versão 2 do "Conta CNPJs"`) {
		t.Fatalf("update %v %q %+v", res.IsError, text, out)
	}
	var n int
	testPool.QueryRow(context.Background(), `SELECT count(*) FROM apps WHERE name = 'Conta CNPJs'`).Scan(&n)
	if n != 1 {
		t.Fatalf("apps = %d", n)
	}
	envio = sendEnvio(t, h, s, map[string]string{"main.py": "print('v2')\n"})
	if res, text, out := registrar(t, s, map[string]any{"envio": envio, "app_id": id}); res.IsError || out["mudou"] != false || !strings.Contains(text, "nada mudou") {
		t.Fatalf("same %v %q", res.IsError, text)
	}
}

func TestRegistrarSecretsRetrySameEnvio(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "API_KEY = \"sk-proj-abcdefghijklmnopqrstuvwxyz0123456789ABCD\"\nprint(API_KEY)\n"})
	res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()})
	if !res.IsError || !strings.Contains(text, "confirmar_segredos") || !strings.Contains(text, "mesmo envio") {
		t.Fatalf("first %v %q", res.IsError, text)
	}
	res, text, out := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto(), "confirmar_segredos": true})
	if res.IsError {
		t.Fatalf("retry: %s", text)
	}
	cleanupAppFolder(t, int64(out["app_id"].(float64)))
}

func TestRegistrarDropsEnv(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print(1)\n", ".env": "SENHA=x\n"})
	res, text, out := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()})
	if res.IsError || !strings.Contains(text, "Deixei de fora: .env") || !strings.Contains(text, "nunca mande os valores") {
		t.Fatalf("%v %q", res.IsError, text)
	}
	id := int64(out["app_id"].(float64))
	t.Cleanup(func() { cleanupAppFolder(t, id) })
	if names := storedNames(t, id); names[".env"] != "" || len(names) != 1 {
		t.Fatalf("stored %v", names)
	}
}

func TestRegistrarRefusesDesktopAndExe(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print(1)\n", "app.exe": "MZ"})
	m := scriptManifesto()
	m["kind"] = "desktop"
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": m}); !res.IsError || !strings.Contains(text, "Apps de janela") {
		t.Fatalf("desktop %q", text)
	}
	m = scriptManifesto()
	m["entrypoint"] = "app.exe"
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": m}); !res.IsError || !strings.Contains(text, "Apps de janela") {
		t.Fatalf("exe %q", text)
	}
	m = scriptManifesto()
	m["kind"] = "scheduled"
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": m}); !res.IsError || !strings.Contains(text, "Registre como script") {
		t.Fatalf("scheduled %q", text)
	}
}

func TestRegistrarOtherPersonsEnvio(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	createPersonAs(t, c, "Ana", "ana@alis.com", "admin")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	envio := sendEnvio(t, h, toolSession(t, c), map[string]string{"main.py": "print(1)\n"})
	if res, text, _ := registrar(t, toolSession(t, ana), map[string]any{"envio": envio, "manifesto": scriptManifesto()}); !res.IsError || !strings.Contains(text, "Não achei esse envio") {
		t.Fatalf("%q", text)
	}
}

func TestRegistrarMissingEntrypointHint(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	envio := sendEnvio(t, h, s, map[string]string{"pasta/main.py": "print(1)\n"})
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()}); !res.IsError || !strings.Contains(text, "de dentro da pasta") {
		t.Fatalf("%q", text)
	}
}

func TestRegistrarFolderAppRefused(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	id := insertApp(t, manifest.Manifest{ManifestVersion: 1, ID: "da-pasta", Name: "Da pasta", Kind: manifest.KindScript, Entrypoint: "main.py", Area: "Research"})
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print(1)\n"})
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "app_id": id}); !res.IsError || !strings.Contains(text, "pasta do computador") {
		t.Fatalf("%q", text)
	}
}

func TestRegistrarAudited(t *testing.T) {
	c, h := newPoolClient(t)
	p := setupAdmin(t, c)
	s := toolSession(t, c)
	before := maxAuditID(t)
	id := registrarNovo(t, h, s)
	if n := claudeAudits(t, before, "POST /api/scripts/import", p.ID); n != 1 {
		t.Fatalf("import audits = %d", n)
	}
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v2')\n"})
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "app_id": id}); res.IsError {
		t.Fatal(text)
	}
	if n := claudeAudits(t, before, "POST /api/apps/{id}/sources", p.ID); n != 1 {
		t.Fatalf("sources audits = %d", n)
	}
}

func TestFormatoDoApp(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	res, text, out := callTool(t, toolSession(t, c), "formato_do_app", map[string]any{})
	if res.IsError || !strings.Contains(text, "pode registrar") || !strings.Contains(text, "Research") {
		t.Fatalf("%v %q", res.IsError, text)
	}
	tipos, _ := out["tipos"].([]any)
	if len(tipos) != 2 || tipos[0] != "script" || tipos[1] != "web" {
		t.Fatalf("tipos %v", out["tipos"])
	}
	if areas, _ := out["areas"].([]any); !slices.Contains(areas, any("Research")) {
		t.Fatalf("areas %v", out["areas"])
	}
}

func TestVerAppCodigoGuardado(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	id := registrarNovo(t, h, s)
	res, text, out := callTool(t, s, "ver_app", map[string]any{"id": id})
	if res.IsError || out["codigo_guardado"] != true || !strings.Contains(text, "Pode receber versão nova pelo Claude") {
		t.Fatalf("%v %q", res.IsError, text)
	}
	ficha, _ := out["ficha_editavel"].(map[string]any)
	if ficha == nil || ficha["entrypoint"] != "main.py" || ficha["updated_at"] == nil || len(ficha["inputs"].([]any)) != 1 {
		t.Fatalf("ficha %+v", out["ficha_editavel"])
	}
	folder := insertApp(t, manifest.Manifest{ManifestVersion: 1, ID: "da-pasta", Name: "Da pasta", Kind: manifest.KindScript, Entrypoint: "main.py", Area: "Research"})
	if _, text, out := callTool(t, s, "ver_app", map[string]any{"id": folder}); out["codigo_guardado"] != false || !strings.Contains(text, "pasta do computador") {
		t.Fatalf("folder app %q", text)
	}

	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	res, text, out = callTool(t, toolSession(t, ana), "ver_app", map[string]any{"id": id})
	if res.IsError {
		t.Fatalf("viewer: %s", text)
	}
	if _, ok := out["ficha_editavel"]; ok {
		t.Fatalf("viewer got ficha_editavel")
	}
}

func TestRegistrarVersaoMudaFicha(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	id := registrarNovo(t, h, s)
	m := scriptManifesto()
	m["summary"] = "Resumo novo."
	m["inputs"] = append(m["inputs"].([]any), map[string]any{"id": "data", "label": "Data", "type": "date", "source": "argument", "required": false})
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v2')\n"})
	res, text, _ := registrar(t, s, map[string]any{"envio": envio, "app_id": id, "manifesto": m})
	if res.IsError || !strings.Contains(text, "resumo") || !strings.Contains(text, "entradas") {
		t.Fatalf("%v %q", res.IsError, text)
	}
	d := detail(t, c, id)
	if d.Manifest.Summary != "Resumo novo." || len(d.Manifest.Inputs) != 2 {
		t.Fatalf("ficha %q %d", d.Manifest.Summary, len(d.Manifest.Inputs))
	}
}

func TestRegistrarVersaoIgnoraNomeArea(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	id := registrarNovo(t, h, s)
	before := detail(t, c, id)
	m := scriptManifesto()
	m["name"], m["area"] = "Outro", "Compliance"
	m["owner"] = map[string]any{"name": "Ana", "email": "ana@alis.com"}
	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v2')\n"})
	res, text, _ := registrar(t, s, map[string]any{"envio": envio, "app_id": id, "manifesto": m})
	if res.IsError || !strings.Contains(text, "isso só muda pelo site") {
		t.Fatalf("%v %q", res.IsError, text)
	}
	after := detail(t, c, id)
	if after.Manifest.Name != before.Manifest.Name || after.AreaID != before.AreaID || after.OwnerID != before.OwnerID {
		t.Fatalf("changed: %q %d %d", after.Manifest.Name, after.AreaID, after.OwnerID)
	}
}

func TestRegistrarWebApp(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	s := toolSession(t, c)
	m := scriptManifesto()
	m["kind"], m["entrypoint"], m["name"] = "web", "app.py", "Painel"
	files := map[string]string{"app.py": "import streamlit as st\nst.write('oi')\n", "requirements.txt": "streamlit\n"}
	envio := sendEnvio(t, h, s, files)
	if res, text, _ := registrar(t, s, map[string]any{"envio": envio, "manifesto": m}); !res.IsError || !strings.Contains(text, "Scripts sobem normalmente") {
		t.Fatalf("off: %q", text)
	}
	withShared(t, h)
	res, text, out := registrar(t, s, map[string]any{"envio": envio, "manifesto": m})
	if res.IsError {
		t.Fatalf("on: %s", text)
	}
	id := int64(out["app_id"].(float64))
	if slug, kind, folder, _ := appRow(t, id); slug != "painel" || kind != "web" || folder != h.shared.Svc.CodeDir(id) {
		t.Fatalf("%s %s %s", slug, kind, folder)
	}
}
