package handler

import (
	"context"
	"strings"
	"testing"
)

// problemas gathers, in one answer, a failed run, a schedule that skipped, a
// screen app whose publish failed and a required setting still empty.
func TestProblemas(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	sd := withShared(t, h)
	s := toolSession(t, c)

	res, text, out := callTool(t, s, "problemas", map[string]any{})
	if res.IsError || !strings.Contains(text, "Nenhum problema") || len(out["problemas"].([]any)) != 0 {
		t.Fatalf("empty %q %+v", text, out)
	}

	script := registrarNovo(t, h, s)
	pairComputer(t, c, "PC")
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `INSERT INTO runs (app_id, person_id, computer_id, output_folder, status, error_message, finished_at)
		VALUES ($1, 1, 1, 'C:\x', 'error', 'O app fechou com erro (código 2).', now())`, script); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO runs (app_id, person_id, computer_id, output_folder, status, created_at)
		VALUES ($1, 1, 1, 'C:\x', 'error', now() - interval '20 days')`, script); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO schedules (app_id, person_id, computer_id, frequency, output_folder, next_run_at, situation)
		VALUES ($1, 1, 1, '{"kind":"daily","at":"07:00"}', 'C:\saida', now(), 'Pulada: o computador estava desligado')`, script); err != nil {
		t.Fatal(err)
	}

	tela := registrarTela(t, h, s, "")
	t.Cleanup(func() { cleanupAppFolder(t, tela) })
	code := 1
	sd.eng.CrashOnStart = &code
	sd.eng.SetLogs("Traceback (most recent call last):\nIndexError: 4")
	publicar(t, s, map[string]any{"app_id": tela})

	vault := webWithSettings(t, c) // SENHA_EMAIL is required and empty

	res, text, out = callTool(t, s, "problemas", map[string]any{})
	if res.IsError {
		t.Fatalf("problemas %q", text)
	}
	tipos := map[string]int{}
	for _, p := range out["problemas"].([]any) {
		tipos[p.(map[string]any)["tipo"].(string)]++
	}
	if tipos["execucao_com_erro"] != 1 || tipos["agenda_com_problema"] != 1 || tipos["publicacao_falhou"] != 1 || tipos["chave_faltando"] != 1 {
		t.Fatalf("tipos %v %q", tipos, text)
	}
	for _, want := range []string{"código 2", "computador estava desligado", "IndexError: 4", "Senha do e-mail", "/execucoes/"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}

	// One app only.
	_, text, out = callTool(t, s, "problemas", map[string]any{"app_id": vault})
	if n := len(out["problemas"].([]any)); n != 1 || !strings.Contains(text, "Senha do e-mail") {
		t.Fatalf("one app %d %q", n, text)
	}
	// 30 days back sees the old run too.
	_, _, out = callTool(t, s, "problemas", map[string]any{"app_id": script, "dias": 30})
	if n := len(out["problemas"].([]any)); n != 3 {
		t.Fatalf("30 days %d %+v", n, out)
	}

	// Someone who takes care of nothing sees none of it.
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	_, text, out = callTool(t, toolSession(t, ana), "problemas", map[string]any{})
	if n := len(out["problemas"].([]any)); n != 0 || strings.Contains(text, "IndexError") {
		t.Fatalf("ana sees %d %q", n, text)
	}
	if res, _, _ := callTool(t, toolSession(t, ana), "problemas", map[string]any{"app_id": tela}); !res.IsError {
		t.Fatal("ana got an app she cannot see")
	}
}
