package handler

import (
	"strings"
	"testing"
)

// MCP F4: publicar_app_com_tela keeps a text to check, and a version whose
// page does not show it never takes the place of the one in the air.
func TestPublicarTelaConferirTexto(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	s := toolSession(t, c)
	id := registrarTela(t, h, s, "")
	t.Cleanup(func() { cleanupAppFolder(t, id) })

	if _, text, out := publicar(t, s, map[string]any{"app_id": id, "conferir_texto": "ok "}); out["situacao"] != "no_ar" {
		t.Fatalf("v1 %q %+v", text, out)
	}
	if _, _, _, m := appRow(t, id); m.Shared == nil || m.Shared.CheckText != "ok " && m.Shared.CheckText != "ok" {
		t.Fatalf("check text not kept: %+v", m.Shared)
	}
	res, text, out := publicar(t, s, map[string]any{"app_id": id, "conferir_texto": "Painel do mês"})
	if !res.IsError || !strings.Contains(text, `não mostra o texto esperado "Painel do mês"`) || !strings.Contains(text, "a versão 1 continua no ar") {
		t.Fatalf("missing text %q %+v", text, out)
	}
	// An empty text stops checking.
	if _, text, out := publicar(t, s, map[string]any{"app_id": id, "conferir_texto": ""}); out["situacao"] != "no_ar" {
		t.Fatalf("cleared %q %+v", text, out)
	}
	if _, _, _, m := appRow(t, id); m.Shared.CheckText != "" {
		t.Fatalf("not cleared: %q", m.Shared.CheckText)
	}
}
