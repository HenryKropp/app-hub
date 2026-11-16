// Package testplan decides whether a version's test passed: the fingerprint of
// what a person confirmed, and the check of each item of teste.espera.
package testplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Place values of the fingerprint.
const PlacePC = "pc"

const consoleLog = ".apphub-console.txt"

// Fingerprint hashes what the person confirms: what the test touches, the step
// it skips, the vault keys it uses and where it runs. espera, entradas and
// tempo_maximo_min do not enter: changing them needs no new confirmation.
func Fingerprint(m manifest.Manifest, place string) string {
	var acessa []string
	pula := ""
	if m.Teste != nil {
		acessa = m.Teste.Acessa
		pula = m.Teste.Pula
	}
	keys := make([]string, 0, len(m.Keys))
	for _, k := range m.Keys {
		keys = append(keys, k.Name)
	}
	sort.Strings(keys)
	b, _ := json.Marshal(struct {
		Acessa []string `json:"acessa"`
		Pula   string   `json:"pula"`
		Keys   []string `json:"keys"`
		Place  string   `json:"place"`
	}{acessa, pula, keys, place})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Check is the result of one item of the test.
type Check struct {
	Item   string `json:"item"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
}

const exitItem = "código de saída 0"

// Evaluate checks the exit code and each item of t.Espera.
func Evaluate(t manifest.Teste, exit int, files []agentproto.FileEntry, log string) []Check {
	out := []Check{{Item: exitItem, Detail: fmt.Sprintf("código de saída %d", exit), OK: exit == 0}}
	for _, e := range t.Espera {
		switch {
		case e.Arquivos != "":
			min := max(e.NoMinimo, 1)
			n := countMatches(e.Arquivos, files)
			out = append(out, Check{
				Item:   "arquivos " + e.Arquivos,
				Detail: fmt.Sprintf("arquivos %s: %d (mínimo %d)", e.Arquivos, n, min),
				OK:     n >= min,
			})
		case e.LogContem != "":
			ok := strings.Contains(log, e.LogContem)
			d := fmt.Sprintf("o log contém %q", e.LogContem)
			if !ok {
				d = fmt.Sprintf("o log não contém %q", e.LogContem)
			}
			out = append(out, Check{Item: "log contém " + e.LogContem, Detail: d, OK: ok})
		}
	}
	return out
}

func countMatches(pattern string, files []agentproto.FileEntry) int {
	n := 0
	for _, f := range files {
		p := strings.ReplaceAll(f.Path, "\\", "/")
		if path.Base(p) == consoleLog {
			continue
		}
		if ok, _ := path.Match(pattern, p); ok {
			n++
		} else if ok, _ := path.Match(pattern, path.Base(p)); ok {
			n++
		}
	}
	return n
}

// Passed reports whether every check is OK.
func Passed(checks []Check) bool {
	for _, c := range checks {
		if !c.OK {
			return false
		}
	}
	return len(checks) > 0
}

// Summary is the notice's short result, from the first arquivos item.
func Summary(checks []Check) string {
	for _, c := range checks {
		if !strings.HasPrefix(c.Item, "arquivos ") {
			continue
		}
		var n int
		// Detail is "arquivos <pattern>: N (mínimo M)".
		if i := strings.LastIndex(c.Detail, ": "); i >= 0 {
			fmt.Sscanf(c.Detail[i+2:], "%d", &n)
		}
		if n == 1 {
			return "1 arquivo salvo"
		}
		return fmt.Sprintf("%d arquivos salvos", n)
	}
	return "conferências ok"
}

var saoPaulo = func() *time.Location {
	if l, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		return l
	}
	return time.FixedZone("BRT", -3*3600)
}()

// ResolveInputs turns teste.entradas into run values: "hoje" in a date input
// becomes today in São Paulo (2006-01-02); other values pass as they are.
func ResolveInputs(m manifest.Manifest, now time.Time) map[string][]string {
	out := map[string][]string{}
	if m.Teste == nil {
		return out
	}
	types := map[string]string{}
	for _, in := range m.Inputs {
		types[in.ID] = in.Type
	}
	for id, v := range m.Teste.Entradas {
		if types[id] == "date" && strings.EqualFold(strings.TrimSpace(v), "hoje") {
			v = now.In(saoPaulo).Format("2006-01-02")
		}
		out[id] = []string{v}
	}
	return out
}
