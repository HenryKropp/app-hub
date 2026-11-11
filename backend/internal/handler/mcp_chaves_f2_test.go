package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// MCP F2: a .env shared by several programs does not fill the vault with
// names the app's code never reads.
func TestEnviarChavesIgnoraNomesQueOCodigoNaoLe(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	id := webWithSettings(t, c)
	_, _, folder, _ := appRow(t, id)
	code := "import os\nsenha = os.environ[\"SENHA_EMAIL\"]\ntoken = os.getenv(\"API_TOKEN\", \"\")\nmodo = os.getenv(\"MODO\")\n"
	if err := os.WriteFile(filepath.Join(folder, "app.py"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, out := callTool(t, toolSession(t, c), "enviar_chaves", map[string]any{"app_id": id})
	env := "SENHA_EMAIL=s3gredo\nAPI_TOKEN=abc\nMODO=rapido\nOUTRO_PROGRAMA_KEY=zzz\n"
	status, body := putEnvio(t, securedServer(t, h), out["endereco"].(string), []byte(env))
	if status != http.StatusOK {
		t.Fatalf("put %d %v", status, body)
	}
	raw, _ := json.Marshal(body)
	var r chavesResult
	_ = json.Unmarshal(raw, &r)
	if !slices.Equal(r.Guardadas, []string{"API_TOKEN", "SENHA_EMAIL"}) || !slices.Equal(r.Configuracoes, []string{"MODO"}) ||
		len(r.Ignoradas) != 1 || r.Ignoradas[0].Nome != "OUTRO_PROGRAMA_KEY" || r.Ignoradas[0].Motivo != msgKeyNotRead {
		t.Fatalf("result %+v", r)
	}
}

// apagar_chave asks first, then deletes; only managers can.
func TestApagarChave(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	id := webWithSettings(t, c)
	if _, err := testPool.Exec(context.Background(), `INSERT INTO app_config (app_id, name, is_secret, value_cipher) VALUES ($1, 'OUTRO_TOKEN', true, $2)`,
		id, mustSeal(t, h, "valor-xyz")); err != nil {
		t.Fatal(err)
	}
	s := toolSession(t, c)
	res, text, out := callTool(t, s, "apagar_chave", map[string]any{"app_id": id, "nome": "OUTRO_TOKEN"})
	if res.IsError || out["apagada"] != false || !strings.Contains(text, "confirme com a pessoa") {
		t.Fatalf("no confirm: %s %v", text, out)
	}
	if n := configRows(t, id, "OUTRO_TOKEN"); n != 1 {
		t.Fatalf("deleted before confirming: %d", n)
	}
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	if res, text, _ := callTool(t, toolSession(t, ana), "apagar_chave", map[string]any{"app_id": id, "nome": "OUTRO_TOKEN", "confirmado": true}); !res.IsError {
		t.Fatalf("ana deleted: %s", text)
	}
	res, text, out = callTool(t, s, "apagar_chave", map[string]any{"app_id": id, "nome": "OUTRO_TOKEN", "confirmado": true})
	if res.IsError || out["apagada"] != true || strings.Contains(text, "valor-xyz") {
		t.Fatalf("delete: %s %v", text, out)
	}
	if n := configRows(t, id, "OUTRO_TOKEN"); n != 0 {
		t.Fatalf("still there: %d", n)
	}
	if res, _, _ := callTool(t, s, "apagar_chave", map[string]any{"app_id": id, "nome": "OUTRO_TOKEN", "confirmado": true}); !res.IsError {
		t.Fatal("deleted twice")
	}
	// The Ziv's own rows are not keys.
	if res, _, _ := callTool(t, s, "apagar_chave", map[string]any{"app_id": id, "nome": "_db_password", "confirmado": true}); !res.IsError {
		t.Fatal("deleted a Ziv row")
	}
}

func configRows(t *testing.T, id int64, name string) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM app_config WHERE app_id=$1 AND name=$2`, id, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMergeEnv(t *testing.T) {
	cur := []manifest.EnvVar{
		{Name: "BR_RAIZ", Label: "BR_RAIZ", Required: true},      // now read with a default
		{Name: "VAR_DEMO", Label: "VAR_DEMO", Required: true},    // no longer read, no value: goes
		{Name: "GUARDADA", Label: "Guardada", Secret: true},      // not read, but has a value: stays
		{Name: "COM_PADRAO", Label: "Com padrão", Default: "x"},  // not read, has a default: stays
		{Name: "DECLARADA", Label: "DECLARADA", Required: false}, // in keys: keys win
	}
	reads := map[string]bool{"BR_RAIZ": false, "NOVA_KEY": true, "PORT": true}
	keys := []manifest.Key{{Name: "DECLARADA", Label: "Declarada", Required: true}}
	got := mergeEnv(cur, reads, keys, map[string]bool{"GUARDADA": true})
	var names []string
	for _, e := range got {
		names = append(names, e.Name)
	}
	if !slices.Equal(names, []string{"BR_RAIZ", "GUARDADA", "COM_PADRAO", "DECLARADA", "NOVA_KEY"}) {
		t.Fatalf("names %v", names)
	}
	if got[0].Required || !got[3].Required || got[3].Label != "Declarada" || !got[4].Required {
		t.Fatalf("env %+v", got)
	}
}

// A new version of a screen app that declares keys keeps them (MCP F2: they
// used to be dropped with mudou false).
func TestRegistrarTelaVersaoComChaves(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	s := toolSession(t, c)
	id := registrarTela(t, h, s, "")
	t.Cleanup(func() { cleanupAppFolder(t, id) })
	envio := sendEnvio(t, h, s, map[string]string{"app.py": "import os, streamlit as st\nst.title(os.environ['OPENAI_API_KEY'])\n", "requirements.txt": "streamlit\n"})
	keys := []map[string]any{{"name": "OPENAI_API_KEY", "label": "Chave da OpenAI", "required": true}}
	res, text, out := registrar(t, s, map[string]any{"envio": envio, "app_id": id, "manifesto": map[string]any{"keys": keys}})
	if res.IsError || out["mudou"] != true {
		t.Fatalf("v2 %q %v", text, out)
	}
	if _, _, _, m := appRow(t, id); len(m.Keys) != 1 || m.Keys[0].Name != "OPENAI_API_KEY" || !m.Keys[0].Required {
		t.Fatalf("keys %+v", m.Keys)
	}
}
