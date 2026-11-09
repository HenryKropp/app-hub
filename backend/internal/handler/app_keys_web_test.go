package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"


	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// webWithSettings registers a screen app whose Configurações declare a
// secret (SENHA_EMAIL, required for the 24h version) and a plain MODO.
func webWithSettings(t *testing.T, c *client) int64 {
	t.Helper()
	id := importWeb(t, c, "Notícias", &manifest.Shared{Env: []manifest.EnvVar{
		{Name: "SENHA_EMAIL", Label: "Senha do e-mail", Secret: true, Required: true},
		{Name: "MODO", Label: "Modo"},
	}})
	t.Cleanup(func() { cleanupAppFolder(t, id) })
	return id
}

// A screen app has a vault: the shared secret settings, plus keys added on
// the site, minus the Ziv's own "_" rows. The Agente hands them over when
// the app opens on a computer.
func TestWebAppVault(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h) // screen apps on, with the server key
	id := webWithSettings(t, c)
	ctx := context.Background()
	base := "/api/apps/" + itoa(id) + "/keys"

	// The app database's password is the Ziv's own: never in the vault.
	sealed, _ := h.shared.Vault.Seal("senha-do-banco")
	if err := h.q.UpsertAppConfig(ctx, sqlc.UpsertAppConfigParams{AppID: id, Name: "_db_password", IsSecret: true, ValueCipher: sealed}); err != nil {
		t.Fatal(err)
	}
	var got keysOut
	if code := c.do("GET", base, nil, &got); code != http.StatusOK || len(got.Keys) != 1 || got.Keys[0].Name != "SENHA_EMAIL" || got.Keys[0].Required || !got.CanManage {
		t.Fatalf("list %d %+v", code, got)
	}
	var e map[string]string
	if code := c.do("PUT", base+"/MODO", map[string]string{"value": "rapido"}, &e); code != http.StatusBadRequest || !strings.Contains(e["error"], "Outros ajustes") {
		t.Fatalf("plain setting %d %v", code, e)
	}
	for _, name := range []string{"SENHA_EMAIL", "API_TOKEN"} {
		if code := c.do("PUT", base+"/"+name, map[string]string{"value": keyValue}, nil); code != http.StatusOK {
			t.Fatalf("put %s %d", name, code)
		}
	}
	c.do("GET", base, nil, &got)
	if len(got.Keys) != 2 || got.Keys[1].Name != "API_TOKEN" || !got.Keys[0].HasValue {
		t.Fatalf("after put %+v", got)
	}

	token := pairComputer(t, c, "PC")
	body := map[string]any{"output_folder": `C:\Notícias`}
	// An Agente that cannot hand keys to a screen app is asked to update.
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.3.4"}, nil)
	var refused map[string]string
	if code := c.do("POST", appPath(id, "/runs"), body, &refused); code != http.StatusConflict || !strings.Contains(refused["error"], "Agente") {
		t.Fatalf("old agent %d %v", code, refused)
	}
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	if code := c.do("POST", appPath(id, "/runs"), body, &refused); code != http.StatusCreated {
		t.Fatalf("run %d %v", code, refused)
	}
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	var p agentproto.RunPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || !p.Keys || p.Script != nil || strings.Contains(string(job.Payload), keyValue) {
		t.Fatalf("payload %s %v", job.Payload, err)
	}
	var keys map[string]string
	if code := c.agent("GET", "/api/agent/jobs/"+itoa(job.ID)+"/keys", token, nil, &keys); code != http.StatusOK ||
		len(keys) != 2 || keys["SENHA_EMAIL"] != keyValue || keys["API_TOKEN"] != keyValue {
		t.Fatalf("keys %d %v", code, keys)
	}
}

// A screen app opens on the computer from the code the Ziv keeps, started
// with the same recipe as the 24h version, on the computer only.
func TestWebAppOpensOnComputer(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	id := webWithSettings(t, c)
	token := pairComputer(t, c, "PC")
	body := map[string]any{"output_folder": `C:\Dados Notícias`}
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.3.4"}, nil)
	var out map[string]any
	if code := c.do("POST", appPath(id, "/runs"), body, &out); code != http.StatusConflict {
		t.Fatalf("old agent %d %v", code, out)
	}
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	if code := c.do("POST", appPath(id, "/runs"), body, &out); code != http.StatusCreated {
		t.Fatalf("run %d %v", code, out)
	}
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	var p agentproto.RunPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || p.Web == nil || p.Keys {
		t.Fatalf("payload %s %v", job.Payload, err)
	}
	w := p.Web
	if w.AppID != id || w.Runtime != "python" || w.DepsFile != "requirements.txt" || w.Sources["app.py"].ContentBase64 == "" ||
		strings.Join(w.Args, " ") != "-m streamlit run app.py --server.port {PORT} --server.address 127.0.0.1 --server.headless true" {
		t.Fatalf("web %+v", w)
	}
}

// Without the code kept in the Ziv there is nothing to open on the computer.
func TestWebAppWithoutStoredCode(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := insertApp(t, manifest.Manifest{Name: "Painel", Summary: "x", Kind: manifest.KindWeb, Entrypoint: "app.py", Area: "Research"})
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	var out map[string]string
	if code := c.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\D`}, &out); code != http.StatusConflict || out["error"] != msgWebNoCode {
		t.Fatalf("run %d %v", code, out)
	}
}

func TestParseDotEnv(t *testing.T) {
	lines, bad := parseDotEnv([]byte("\xef\xbb\xbf# comentário\r\nexport A=\"um \\\"dois\\\"\"\r\nB='três # quatro'\nC=cinco # nota\n\nlinha ruim\nD=\nA=último\n"))
	got := map[string]string{}
	for _, l := range lines {
		got[l.name] = l.value
	}
	if len(lines) != 4 || got["A"] != "último" || got["B"] != "três # quatro" || got["C"] != "cinco" || got["D"] != "" {
		t.Fatalf("lines %+v", lines)
	}
	if len(bad) != 1 || bad[0].Nome != "linha 6" {
		t.Fatalf("bad %+v", bad)
	}
	lines, _ = parseDotEnv([]byte(`X="a \\ b"`))
	if lines[0].value != `a \ b` {
		t.Fatalf("escape %q", lines[0].value)
	}
}

// enviar_chaves: Claude sends the .env straight to a one-time address; the
// vault is filled and the answer names what was saved, never a value.
func TestEnviarChaves(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h) // screen apps on, with the server key
	id := webWithSettings(t, c)
	s := toolSession(t, c)
	srv := securedServer(t, h)

	res, text, out := callTool(t, s, "enviar_chaves", map[string]any{"app_id": id})
	if res.IsError || !strings.Contains(text, "NÃO abra") || !strings.Contains(out["comando_windows"].(string), "curl.exe") {
		t.Fatalf("enviar_chaves %s %v", text, out)
	}
	endereco := out["endereco"].(string)
	before := auditCount(t, "app_key.imported")
	env := "# chaves\nexport SENHA_EMAIL=\"s3 gredo\"\nAPI_TOKEN=abc # nota\nMODO=rapido\nPATH=x\nVAZIA=\nlinha ruim\nAPI_TOKEN=def\n"
	code, body := putEnvio(t, srv, endereco, []byte(env))
	if code != http.StatusOK {
		t.Fatalf("put %d %v", code, body)
	}
	raw, _ := json.Marshal(body)
	for _, v := range []string{"s3 gredo", "def", "rapido"} {
		if strings.Contains(string(raw), v) {
			t.Fatalf("answer shows a value: %s", raw)
		}
	}
	var r chavesResult
	_ = json.Unmarshal(raw, &r)
	if !slices.Equal(r.Guardadas, []string{"API_TOKEN", "SENHA_EMAIL"}) || !slices.Equal(r.Configuracoes, []string{"MODO"}) || len(r.Ignoradas) != 3 {
		t.Fatalf("result %+v", r)
	}
	a, _ := h.q.GetApp(context.Background(), id)
	values, err := h.keyValues(context.Background(), a)
	if err != nil || values["SENHA_EMAIL"] != "s3 gredo" || values["API_TOKEN"] != "def" || values["MODO"] != "rapido" || len(values) != 3 {
		t.Fatalf("vault %v %v", values, err)
	}
	var plain string
	var secret bool
	if err := testPool.QueryRow(context.Background(), `SELECT value_plain, is_secret FROM app_config WHERE app_id=$1 AND name='MODO'`, id).Scan(&plain, &secret); err != nil || plain != "rapido" || secret {
		t.Fatalf("MODO %q %v %v", plain, secret, err)
	}
	if n := auditCount(t, "app_key.imported") - before; n != 1 {
		t.Fatalf("audit %d", n)
	}
	// The address works once.
	if code, _ := putEnvio(t, srv, endereco, []byte(env)); code != http.StatusNotFound {
		t.Fatalf("second use %d", code)
	}
	// A made-up address is refused.
	if code, _ := putEnvio(t, srv, "http://hub.test/api/mcp/chaves/inventado", []byte(env)); code != http.StatusNotFound {
		t.Fatalf("made up %d", code)
	}
	// A refused file does not use up the address.
	_, _, out = callTool(t, s, "enviar_chaves", map[string]any{"app_id": id})
	if code, _ := putEnvio(t, srv, out["endereco"].(string), []byte("  \n")); code != http.StatusBadRequest {
		t.Fatalf("empty %d", code)
	}
	if code, _ := putEnvio(t, srv, out["endereco"].(string), []byte("NOVA=1\n")); code != http.StatusOK {
		t.Fatalf("after refused %d", code)
	}
}

// Only the app's owner or an administrator gets an address.
func TestEnviarChavesOnlyManagers(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h) // screen apps on, with the server key
	id := webWithSettings(t, c)
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	res, text, _ := callTool(t, toolSession(t, ana), "enviar_chaves", map[string]any{"app_id": id})
	if !res.IsError {
		t.Fatalf("ana got an address: %s", text)
	}
	// An address made by an admin who later loses the right is refused on use.
	_, _, out := callTool(t, toolSession(t, c), "enviar_chaves", map[string]any{"app_id": id})
	var other int64
	if err := testPool.QueryRow(context.Background(), `SELECT id FROM people WHERE email='ana@alis.com'`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(context.Background(), `UPDATE mcp_key_uploads SET person_id=$1 WHERE used_at IS NULL AND app_id=$2`, other, id); err != nil {
		t.Fatal(err)
	}
	if code, _ := putEnvio(t, securedServer(t, h), out["endereco"].(string), []byte("X_TOKEN=1\n")); code != http.StatusForbidden {
		t.Fatalf("lost right %d", code)
	}
}
