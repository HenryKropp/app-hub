package manifest

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// Teste is the test plan of a version: what the test touches, the
// irreversible step test mode skips, and what the Ziv checks to say "passed".
type Teste struct {
	Acessa         []string          `yaml:"acessa,omitempty" json:"acessa"`
	Pula           string            `yaml:"pula,omitempty" json:"pula"`
	Entradas       map[string]string `yaml:"entradas,omitempty" json:"entradas,omitempty"`
	Espera         []Espera          `yaml:"espera,omitempty" json:"espera"`
	Acao           string            `yaml:"acao,omitempty" json:"acao,omitempty"`
	TempoMaximoMin int               `yaml:"tempo_maximo_min,omitempty" json:"tempo_maximo_min,omitempty"`
}

// Espera is one check of the test result. Exactly one field is set.
type Espera struct {
	Arquivos   string `yaml:"arquivos,omitempty" json:"arquivos,omitempty"`
	NoMinimo   int    `yaml:"no_minimo,omitempty" json:"no_minimo,omitempty"`
	LogContem  string `yaml:"log_contem,omitempty" json:"log_contem,omitempty"`
	TelaContem string `yaml:"tela_contem,omitempty" json:"tela_contem,omitempty"`
}

// MaxTestMinutes caps tempo_maximo_min: the Agente already stops scripts at 15 min.
const MaxTestMinutes = 15

// MsgScreenOnly refuses screen-app checks in a script's plan.
const MsgScreenOnly = "tela_contem e acao são só para apps com tela."

// Timeout is the test's time limit, 15 min when unset.
func (t Teste) Timeout() time.Duration {
	if t.TempoMaximoMin <= 0 {
		return MaxTestMinutes * time.Minute
	}
	return time.Duration(t.TempoMaximoMin) * time.Minute
}

func (m Manifest) testeProblems() []string {
	t := m.Teste
	if t == nil {
		return nil
	}
	var out []string
	hasAcessa := false
	for _, a := range t.Acessa {
		if strings.TrimSpace(a) != "" {
			hasAcessa = true
		}
	}
	if !hasAcessa {
		out = append(out, "Diga em teste.acessa o que o teste acessa (sites, sistemas, pastas).")
	}
	if strings.TrimSpace(t.Pula) == "" {
		out = append(out, `Diga em teste.pula o passo irreversível que o modo teste pula, ou "nada".`)
	}
	if t.TempoMaximoMin < 0 || t.TempoMaximoMin > MaxTestMinutes {
		out = append(out, fmt.Sprintf("teste.tempo_maximo_min precisa estar entre 1 e %d.", MaxTestMinutes))
	}
	script := m.Kind == KindScript
	screenOnly := t.Acao != ""
	work := false
	for _, e := range t.Espera {
		if e.TelaContem != "" {
			screenOnly = true
			if !script {
				work = true
			}
		}
		if e.NoMinimo < 0 {
			out = append(out, "teste.espera.no_minimo não pode ser negativo.")
		}
		if e.Arquivos != "" {
			work = true
			if _, err := path.Match(e.Arquivos, ""); err != nil {
				out = append(out, fmt.Sprintf("O padrão de arquivos %q é inválido.", e.Arquivos))
			}
		}
	}
	if script && screenOnly {
		out = append(out, MsgScreenOnly)
	}
	if !work {
		out = append(out, "teste.espera precisa conferir arquivos gerados (arquivos), não só o log.")
	}
	return out
}
