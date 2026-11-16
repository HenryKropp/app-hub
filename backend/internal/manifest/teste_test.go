package manifest

import (
	"strings"
	"testing"
	"time"
)

func validTeste() *Teste {
	return &Teste{
		Acessa: []string{"BTG (login com BTG_SENHA)"},
		Pula:   "mover para a pasta oficial",
		Espera: []Espera{{Arquivos: "*.pdf", NoMinimo: 1}, {LogContem: "Concluído"}},
	}
}

func TestTesteProblems(t *testing.T) {
	m := valid()
	m.Teste = validTeste()
	if errs := m.Validate(); len(errs) != 0 {
		t.Fatalf("complete teste should pass: %v", errs)
	}
	m = valid()
	if errs := m.Validate(); len(errs) != 0 {
		t.Fatalf("script without teste stays valid: %v", errs)
	}
	nada := validTeste()
	nada.Pula = "nada"
	m.Teste = nada
	if errs := m.Validate(); len(errs) != 0 {
		t.Fatalf(`pula "nada" is accepted: %v`, errs)
	}

	cases := map[string]func(t *Teste){
		"sem acessa":     func(t *Teste) { t.Acessa = nil },
		"pula vazio":     func(t *Teste) { t.Pula = " " },
		"espera sem arq": func(t *Teste) { t.Espera = []Espera{{LogContem: "ok"}} },
		"tela_contem":    func(t *Teste) { t.Espera = append(t.Espera, Espera{TelaContem: "x"}) },
		"acao":           func(t *Teste) { t.Acao = "POST /rodar" },
		"tempo 16":       func(t *Teste) { t.TempoMaximoMin = 16 },
		"tempo -1":       func(t *Teste) { t.TempoMaximoMin = -1 },
		"glob inválido":  func(t *Teste) { t.Espera[0].Arquivos = "[a" },
		"no_minimo -1":   func(t *Teste) { t.Espera[0].NoMinimo = -1 },
	}
	for name, mutate := range cases {
		tt := validTeste()
		mutate(tt)
		m := valid()
		m.Teste = tt
		if errs := m.Validate(); len(errs) == 0 {
			t.Errorf("%s: expected a problem", name)
		}
	}
	tt := validTeste()
	tt.Acao = "POST /rodar"
	m = valid()
	m.Teste = tt
	if errs := m.Validate(); !strings.Contains(strings.Join(errs, " "), "tela_contem e acao são só para apps com tela.") {
		t.Errorf("acao message: %v", errs)
	}
}

func TestTesteTimeout(t *testing.T) {
	if got := (Teste{}).Timeout(); got != 15*time.Minute {
		t.Errorf("default = %v", got)
	}
	if got := (Teste{TempoMaximoMin: 3}).Timeout(); got != 3*time.Minute {
		t.Errorf("3 = %v", got)
	}
}
