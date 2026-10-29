// Package readiness checks whether an app folder can run as a shared app,
// says what the Hub resolves by itself, and writes the text to paste into
// Claude for what needs a code change.
package readiness

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// Item is one line of the checklist shown before sharing an app.
type Item struct {
	Key        string `json:"key"` // "recipe","port","database","file_data","windows_paths","settings","secrets","container_limits"
	Title      string `json:"title"`
	Status     string `json:"status"` // "ok" | "auto" (Hub resolved it) | "missing" (blocks) | "warn" (does not block)
	Detail     string `json:"detail"`
	ClaudeText string `json:"claude_text"` // ready to paste into Claude ("" if none)
	Acked      bool   `json:"acked"`       // warning marked "I am aware"
}

// Report is the whole check.
type Report struct {
	Ready      bool              `json:"ready"` // no "missing" and every "warn" acked
	Recipe     recipes.Recipe    `json:"recipe"`
	WantsDB    bool              `json:"wants_db"`
	SuggestEnv []manifest.EnvVar `json:"suggest_env"` // settings read in code and missing from the manifest
	Items      []Item            `json:"items"`
}

var (
	envReadRe  = regexp.MustCompile(`(?:os\.getenv\(|os\.environ\.get\(|os\.environ\[|process\.env\.)\s*["']?([A-Z][A-Z0-9_]*)`)
	fileDataRe = regexp.MustCompile(`sqlite3\.connect|\.to_csv\(|\.to_excel\(|openpyxl\.Workbook\(|\.save\(\s*["'][^"']+\.xlsx|open\([^)]*,\s*["'](?:w|a)`)
	winPathRe  = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])[A-Za-z]:\\(?:\\|[^nrtbfv0\s"'])|/mnt/[a-z]/|(?i:OneDrive)`)
	browserRe  = regexp.MustCompile(`playwright|puppeteer|chromium|wkhtmltopdf|selenium`)
	secretWord = regexp.MustCompile(`(?i)PASS|SECRET|TOKEN|KEY`)
	codeExt    = map[string]bool{".py": true, ".js": true, ".mjs": true, ".ts": true}
)

type hit struct {
	file string
	line int
}

func Check(src versions.Folder, m manifest.Manifest, secrets []scanner.SecretFinding, acks []string) (Report, error) {
	rec, err := recipes.Detect(src, m)
	if err != nil {
		return Report{}, err
	}
	s := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		s = *m.Shared
	}
	r := Report{Recipe: rec, WantsDB: s.Database == "postgres"}
	var fileData, winPaths, browser []hit
	envSeen := map[string]bool{}
	err = src.Walk(func(rel string, dir bool, _ int64, _ time.Time) error {
		if dir {
			if b := path.Base(rel); b == ".git" || b == "node_modules" || b == ".venv" || b == "venv" || b == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}
		if !codeExt[path.Ext(rel)] {
			return nil
		}
		f, err := src.Open(rel)
		if err != nil {
			return nil
		}
		b, _ := io.ReadAll(io.LimitReader(f, 1<<20))
		f.Close()
		for i, line := range strings.Split(string(b), "\n") {
			for _, mm := range envReadRe.FindAllStringSubmatch(line, -1) {
				envSeen[mm[1]] = true
			}
			if fileDataRe.MatchString(line) {
				fileData = append(fileData, hit{rel, i + 1})
			}
			if winPathRe.MatchString(line) {
				winPaths = append(winPaths, hit{rel, i + 1})
			}
			if browserRe.MatchString(line) {
				browser = append(browser, hit{rel, i + 1})
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	add := func(it Item) {
		if it.Status == "warn" && slices.Contains(acks, it.Key) {
			it.Acked = true
		}
		r.Items = append(r.Items, it)
	}

	// Receita
	switch rec.Kind {
	case "":
		add(Item{Key: "recipe", Title: "Receita para ligar o app", Status: "missing",
			Detail:     "O Ziv não reconheceu o tipo do app (Streamlit, Flask, FastAPI, Django ou Node).",
			ClaudeText: "Escreva um Dockerfile na raiz deste app para rodá-lo num contêiner Linux. O app deve escutar em 0.0.0.0 na porta da variável de ambiente PORT. Não copie senhas para a imagem."})
	case "dockerfile":
		add(Item{Key: "recipe", Title: "Receita para ligar o app", Status: "ok", Detail: "O app tem Dockerfile próprio."})
	default:
		detail := "O Ziv monta a receita sozinho (" + rec.Kind + ")."
		if len(rec.Requirements) > 0 {
			detail += " Sem requirements.txt; bibliotecas encontradas: " + strings.Join(rec.Requirements, ", ") + "."
		}
		st := "auto"
		if rec.NeedsConfirm {
			st, detail = "missing", detail+" Confirme o comando de início: "+rec.Command
		}
		add(Item{Key: "recipe", Title: "Receita para ligar o app", Status: st, Detail: detail})
	}

	// Porta
	switch {
	case rec.Kind != "" && rec.Kind != "dockerfile":
		add(Item{Key: "port", Title: "Porta", Status: "auto", Detail: fmt.Sprintf("O Ziv usa a porta %d.", rec.Port)})
	case rec.Port > 0:
		add(Item{Key: "port", Title: "Porta", Status: "ok", Detail: fmt.Sprintf("Porta %d.", rec.Port)})
	case rec.Kind == "dockerfile":
		add(Item{Key: "port", Title: "Porta", Status: "missing", Detail: "Diga em qual porta o app escuta.",
			ClaudeText: "Acrescente EXPOSE <porta> ao Dockerfile com a porta em que o app escuta."})
	}

	// Banco
	if envSeen["DATABASE_URL"] {
		r.WantsDB = true
	}
	if r.WantsDB {
		add(Item{Key: "database", Title: "Banco de dados", Status: "auto", Detail: "O Ziv cria um banco só deste app e entrega em DATABASE_URL."})
	}

	// Dados em arquivo
	if len(fileData) > 0 {
		add(Item{Key: "file_data", Title: "Dados gravados em arquivo", Status: "warn",
			Detail:     "O app grava dados em arquivo (" + where(fileData) + "). Com várias pessoas ao mesmo tempo, gravações podem se perder.",
			ClaudeText: "Este app vai rodar uma vez só para várias pessoas. Troque a gravação em arquivo (" + where(fileData) + ") por um banco Postgres lido da variável DATABASE_URL, criando as tabelas na primeira execução."})
	}

	// Pastas do Windows
	if len(winPaths) > 0 {
		add(Item{Key: "windows_paths", Title: "Pastas do computador no código", Status: "warn",
			Detail:     "O código usa pastas do Windows ou do OneDrive (" + where(winPaths) + "), que não existem no servidor.",
			ClaudeText: "Este app vai rodar num servidor Linux. Troque os caminhos de pastas do Windows/OneDrive (" + where(winPaths) + ") pela pasta da variável APPHUB_DATA_DIR, ou por um envio de arquivo na própria tela do app."})
	}

	// Configurações
	declared := map[string]bool{}
	for _, e := range s.Env {
		declared[e.Name] = true
	}
	var names []string
	for n := range envSeen {
		if !declared[n] && !slices.Contains(manifest.ReservedEnv, n) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		r.SuggestEnv = append(r.SuggestEnv, manifest.EnvVar{Name: n, Label: n, Secret: secretWord.MatchString(n), Required: true})
	}
	if len(names) > 0 {
		add(Item{Key: "settings", Title: "Configurações", Status: "auto",
			Detail: "O código lê " + strings.Join(names, ", ") + ". O Ziv acrescenta à lista para você preencher."})
	} else if len(envSeen) > 0 || len(s.Env) > 0 {
		add(Item{Key: "settings", Title: "Configurações", Status: "ok", Detail: "Todas as configurações estão na lista."})
	}

	// Senhas no código
	if len(secrets) > 0 {
		var ss []string
		for _, f := range secrets {
			ss = append(ss, fmt.Sprintf("%s (%s, linha %d)", f.Name, f.File, f.Line))
		}
		add(Item{Key: "secrets", Title: "Senha escrita no código", Status: "warn",
			Detail:     "Encontramos o que parece ser senha no código: " + strings.Join(ss, "; ") + ".",
			ClaudeText: "Tire do código as senhas " + strings.Join(ss, "; ") + " e leia cada uma de uma variável de ambiente com o mesmo nome."})
	}

	// O que não cabe na caixa
	if len(browser) > 0 && rec.Kind != "dockerfile" {
		add(Item{Key: "container_limits", Title: "Recurso que não vem na receita", Status: "warn",
			Detail:     "O app chama um navegador automático (" + where(browser) + "), usado por exemplo para gerar PDF. A receita automática não traz navegador; essa parte não vai funcionar no servidor.",
			ClaudeText: "Este app vai rodar num contêiner Linux sem navegador. Escreva um Dockerfile que instale o que " + where(browser) + " precisa (Node, Playwright e o Chromium com as dependências do sistema), ou troque a geração de PDF por uma biblioteca Python que não precise de navegador."})
	}

	r.Ready = true
	for _, it := range r.Items {
		if it.Status == "missing" || it.Status == "warn" && !it.Acked {
			r.Ready = false
		}
	}
	return r, nil
}

// where lists up to three places, "app.py:12, db.py:3 e mais 2".
func where(h []hit) string {
	var out []string
	for i, x := range h {
		if i == 3 {
			out = append(out, fmt.Sprintf("e mais %d", len(h)-3))
			break
		}
		out = append(out, fmt.Sprintf("%s:%d", x.file, x.line))
	}
	return strings.Join(out, ", ")
}
