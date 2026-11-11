package handler

import (
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func semJargao(t *testing.T, where, text string) {
	t.Helper()
	if text == "" {
		t.Fatalf("%s: no message for the person", where)
	}
	if bad := temJargao(text); len(bad) > 0 {
		t.Fatalf("%s: technical words %v in %q", where, bad, text)
	}
}

func TestTemJargao(t *testing.T) {
	if bad := temJargao("Mandei o manifesto pela API, veja o JSON"); len(bad) != 3 {
		t.Fatalf("%v", bad)
	}
	if bad := temJargao("Pronto: o app está no Ziv. Abrir: https://zivapp.duckdns.org/apps/3#chaves"); len(bad) != 0 {
		t.Fatalf("%v", bad)
	}
}

func TestMsgPessoa(t *testing.T) {
	m := manifest.Manifest{Name: "Conta CNPJs",
		Inputs:  []manifest.Input{{Label: "Planilha com os CNPJs"}, {Label: "Mês"}},
		Outputs: []manifest.Output{{Label: "Relatório em Excel"}}}
	got := msgPessoaApp(m, 1, true, true, false, "L")
	if got != `Pronto: o app "Conta CNPJs" está no Ziv. Ele pede planilha com os CNPJs e mês. Entrega relatório em Excel. Abrir: L` {
		t.Fatalf("%q", got)
	}
	if got := msgPessoaApp(m, 3, false, false, false, "L"); !strings.HasPrefix(got, "Nada mudou") {
		t.Fatalf("%q", got)
	}
	if got := msgPessoaApp(m, 2, false, true, true, "L"); !strings.Contains(got, `a versão 2 do app com tela "Conta CNPJs"`) || !strings.Contains(got, "Falta publicar") {
		t.Fatalf("%q", got)
	}
	v := int32(4)
	for _, s := range []string{
		msgPessoaApp(m, 2, false, true, true, "L"),
		msgPessoaTela(true, "https://x", nil), msgPessoaTela(false, "", &v), msgPessoaTela(false, "", nil),
		msgPessoaExecucao("Conta", "done", "", `C:\Saida`, 2, "L"), msgPessoaExecucao("Conta", "error", "O app fechou com erro (código 1).", "", 0, "L"),
		msgPessoaArtifact("Painel", 1, "L"), msgPessoaArtifact("Painel", 2, "L"),
	} {
		semJargao(t, "msg", s)
	}
	if got := lowerFirst("CNPJs do fundo"); got != "CNPJs do fundo" {
		t.Fatalf("%q", got)
	}
}

// The tools that finish something give the person a ready sentence, without
// technical words.
func TestMensagemParaPessoaNasFerramentas(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	sd := withShared(t, h)
	s := toolSession(t, c)

	envio := sendEnvio(t, h, s, map[string]string{"main.py": "print('v1')\n"})
	_, _, out := registrar(t, s, map[string]any{"envio": envio, "manifesto": scriptManifesto()})
	script := int64(out["app_id"].(float64))
	t.Cleanup(func() { cleanupAppFolder(t, script) })
	msg, _ := out["mensagem_para_pessoa"].(string)
	semJargao(t, "registrar_app", msg)
	if !strings.Contains(msg, "Ele pede valor") || !strings.Contains(msg, "Entrega relatório") {
		t.Fatalf("registrar %q", msg)
	}

	tela := registrarTela(t, h, s, "")
	t.Cleanup(func() { cleanupAppFolder(t, tela) })
	_, _, out = publicar(t, s, map[string]any{"app_id": tela})
	msg, _ = out["mensagem_para_pessoa"].(string)
	semJargao(t, "publicar no ar", msg)
	if !strings.Contains(msg, "está no ar em http") {
		t.Fatalf("publicar %q", msg)
	}
	code := 1
	sd.eng.CrashOnStart = &code
	envio = sendEnvio(t, h, s, map[string]string{"app.py": "import streamlit as st\nst.title('v2')\n", "requirements.txt": "streamlit\n"})
	registrar(t, s, map[string]any{"envio": envio, "app_id": tela})
	_, _, out = publicar(t, s, map[string]any{"app_id": tela})
	msg, _ = out["mensagem_para_pessoa"].(string)
	semJargao(t, "publicar falhou", msg)
	if !strings.Contains(msg, "a versão 1 continua funcionando") {
		t.Fatalf("publicar falhou %q", msg)
	}

	_, _, out = callTool(t, s, "publicar_artifact", map[string]any{"nome": "Painel do mês", "html": "<html><body><h1>Oi</h1></body></html>"})
	msg, _ = out["mensagem_para_pessoa"].(string)
	semJargao(t, "publicar_artifact", msg)
}
