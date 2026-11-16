package testplan

import (
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func plan() manifest.Manifest {
	return manifest.Manifest{
		Name: "Relatórios", Kind: manifest.KindScript, Entrypoint: "r.py",
		Keys:   []manifest.Key{{Name: "BTG_USUARIO"}, {Name: "BTG_SENHA"}},
		Inputs: []manifest.Input{{ID: "data", Label: "Data", Type: "date", Source: "argument"}, {ID: "modo", Label: "Modo", Type: "text", Source: "argument"}},
		Teste: &manifest.Teste{
			Acessa:   []string{"BTG"},
			Pula:     "mover",
			Entradas: map[string]string{"data": "hoje", "modo": "rápido"},
			Espera:   []manifest.Espera{{Arquivos: "*.pdf", NoMinimo: 2}},
		},
	}
}

func TestFingerprint(t *testing.T) {
	base := Fingerprint(plan(), "pc")
	if len(base) != 64 {
		t.Fatalf("not sha256 hex: %q", base)
	}
	same := []func(m *manifest.Manifest){
		func(m *manifest.Manifest) { m.Teste.Espera = []manifest.Espera{{Arquivos: "*.csv"}} },
		func(m *manifest.Manifest) { m.Teste.Entradas = map[string]string{"data": "2026-01-01"} },
		func(m *manifest.Manifest) { m.Teste.TempoMaximoMin = 5 },
		func(m *manifest.Manifest) { m.Keys = []manifest.Key{{Name: "BTG_SENHA"}, {Name: "BTG_USUARIO", Label: "x"}} },
	}
	for i, f := range same {
		m := plan()
		f(&m)
		if got := Fingerprint(m, "pc"); got != base {
			t.Errorf("same[%d] changed the fingerprint", i)
		}
	}
	diff := []func(m *manifest.Manifest){
		func(m *manifest.Manifest) { m.Teste.Acessa = []string{"Itaú"} },
		func(m *manifest.Manifest) { m.Teste.Pula = "nada" },
		func(m *manifest.Manifest) { m.Keys = []manifest.Key{{Name: "BTG_SENHA"}} },
	}
	for i, f := range diff {
		m := plan()
		f(&m)
		if got := Fingerprint(m, "pc"); got == base {
			t.Errorf("diff[%d] kept the fingerprint", i)
		}
	}
	if Fingerprint(plan(), "servidor") == base {
		t.Error("place must change the fingerprint")
	}
}

func TestEvaluate(t *testing.T) {
	te := *plan().Teste
	files := []agentproto.FileEntry{{Path: "a.pdf"}, {Path: "sub/b.pdf"}, {Path: ".apphub-console.txt"}}
	checks := Evaluate(te, 0, files, "tudo ok")
	if !Passed(checks) {
		t.Fatalf("should pass: %+v", checks)
	}
	if len(checks) != 2 || checks[0].Item != "código de saída 0" || checks[1].Detail != "arquivos *.pdf: 2 (mínimo 2)" {
		t.Fatalf("checks = %+v", checks)
	}

	te.Espera = []manifest.Espera{{Arquivos: "*.txt"}}
	checks = Evaluate(te, 0, []agentproto.FileEntry{{Path: ".apphub-console.txt"}}, "")
	if Passed(checks) {
		t.Errorf("console never counts: %+v", checks)
	}

	te.Espera = []manifest.Espera{{Arquivos: "*.pdf"}, {LogContem: "Concluído"}}
	if Passed(Evaluate(te, 0, files, "rodando...")) {
		t.Error("log_contem missing should fail")
	}
	if !Passed(Evaluate(te, 0, files, "x\nConcluído\n")) {
		t.Error("log_contem present should pass")
	}

	checks = Evaluate(te, 2, files, "Concluído")
	if Passed(checks) || checks[0].OK || checks[0].Item != "código de saída 0" {
		t.Errorf("exit 2 should fail on the first item: %+v", checks)
	}
}

func TestSummary(t *testing.T) {
	te := manifest.Teste{Espera: []manifest.Espera{{LogContem: "x"}, {Arquivos: "*.pdf"}}}
	many := []agentproto.FileEntry{}
	for range 12 {
		many = append(many, agentproto.FileEntry{Path: "x" + string(rune('a'+len(many))) + ".pdf"})
	}
	if got := Summary(Evaluate(te, 0, many, "x")); got != "12 arquivos salvos" {
		t.Errorf("12: %q", got)
	}
	if got := Summary(Evaluate(te, 0, many[:1], "x")); got != "1 arquivo salvo" {
		t.Errorf("1: %q", got)
	}
}

func TestResolveInputs(t *testing.T) {
	now := time.Date(2026, 11, 17, 1, 30, 0, 0, time.UTC) // 16/11 22:30 em São Paulo
	got := ResolveInputs(plan(), now)
	if got["data"][0] != "2026-11-16" || got["modo"][0] != "rápido" {
		t.Errorf("got %v", got)
	}
}
