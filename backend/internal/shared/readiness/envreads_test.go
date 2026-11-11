package readiness

import (
	"maps"
	"testing"
)

// MCP F2: the cases from the baixar-relatorios publish (2026-11-11).
func TestEnvReadsIn(t *testing.T) {
	for _, c := range []struct {
		line string
		want map[string]bool
	}{
		{`DEMO = bool(os.environ.get("BR_RAIZ", ""))`, map[string]bool{"BR_RAIZ": false}},
		{`DEMO = bool(os.environ.get(VAR_DEMO, ""))`, map[string]bool{}},
		{`VAR_DEMO = "BR_RAIZ"`, map[string]bool{}},
		{`key = os.environ["API_KEY"]`, map[string]bool{"API_KEY": true}},
		{`key = os.environ[ 'API_KEY' ]`, map[string]bool{"API_KEY": true}},
		{`tz = os.getenv('TZ_NAME', 'America/Sao_Paulo')`, map[string]bool{"TZ_NAME": false}},
		{`u = os.getenv("SMTP_USER")`, map[string]bool{"SMTP_USER": true}},
		{`u = os.environ.get("SMTP_USER")`, map[string]bool{"SMTP_USER": true}},
		{`const k = process.env.OPENAI_KEY`, map[string]bool{"OPENAI_KEY": true}},
		{`const p = process.env.PORT_X || 3000`, map[string]bool{"PORT_X": false}},
		{`const p = process.env["MODE"] ?? "dev"`, map[string]bool{"MODE": false}},
		{`os.environ.update(BR_RAIZ=str(raiz))`, map[string]bool{}},
		{`os.environ["BR_TRABALHO"] = str(x)`, map[string]bool{}},
		{`if os.environ["MODO"] == "x":`, map[string]bool{"MODO": true}},
	} {
		got := map[string]bool{}
		envReadsIn(c.line, got)
		if !maps.Equal(got, c.want) {
			t.Errorf("%s: got %v want %v", c.line, got, c.want)
		}
	}
}

// A name read once with a default and once without is required.
func TestEnvReadsRequiredWins(t *testing.T) {
	got := map[string]bool{}
	envReadsIn(`a = os.getenv("X", "1")`, got)
	envReadsIn(`b = os.environ["X"]`, got)
	envReadsIn(`c = os.getenv("X", "2")`, got)
	if !got["X"] {
		t.Fatal("X should be required")
	}
}
