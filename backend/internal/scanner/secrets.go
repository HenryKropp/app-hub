package scanner

import (
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

// SecretFinding points at a probable secret. The value itself is never kept.
type SecretFinding struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
	Hint string `json:"hint"`
}

var secretTokenPatterns = []struct {
	re   *regexp.Regexp
	hint string
}{
	{regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{20,}`), "Chave da Anthropic (Claude)"},
	{regexp.MustCompile(`sk-(?:proj-)?[A-Za-z0-9_\-]{20,}`), "Chave da OpenAI"},
	{regexp.MustCompile(`AIza[0-9A-Za-z_\-]{35}`), "Chave do Google"},
	{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "Chave da AWS"},
	{regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`), "Token do GitHub"},
	{regexp.MustCompile(`xox[abprs]-[A-Za-z0-9\-]{10,}`), "Token do Slack"},
	{regexp.MustCompile(`hf_[A-Za-z0-9]{30,}`), "Token do Hugging Face"},
}

// secretName matches a variable or key name that holds a secret.
const secretName = `[\w\-]*?(?:api[_\-]?key|apikey|secret|token|password|passwd|senha|pwd)[\w\-]*`

var secretAssignment = regexp.MustCompile(`(?i)(?:^|[^\w\-])(` + secretName + `)["']?\s*[:=]\s*[fbrFBR]?["']([^"'\s]{8,})["']`)

// unquotedSecret is a config-style line (NAME=value, name: value, export
// NAME=value, a YAML list item) whose value has no quotes. Group 2 is the value.
var unquotedSecret = regexp.MustCompile(`(?i)^(\s*(?:-\s+)?(?:export\s+)?["']?` + secretName + `["']?\s*[:=]\s*)([^\s"'#][^\s#]*)`)

// envLine is any NAME=value line of a .env file. Group 2 is the value.
var envLine = regexp.MustCompile(`^(\s*(?:export\s+)?[A-Za-z_][\w.\-]*\s*=\s*)(\S.*?)\s*$`)

var placeholderWords = []string{"replace", "your", "seu_", "sua_", "xxxx", "example", "exemplo", "changeme", "<", "${", "{{", "os.environ", "getenv", "none", "null", "placeholder"}

func looksLikePlaceholder(v string) bool {
	lv := strings.ToLower(v)
	for _, w := range placeholderWords {
		if strings.Contains(lv, w) {
			return true
		}
	}
	// A value made of one repeated character ("********") is a mask, not a secret.
	return strings.Trim(lv, lv[:1]) == ""
}

// FindSecrets reuses folder scanning detection without retaining secret values.
// Callers must not expose source text alongside these findings.
func FindSecrets(rel, text string) []SecretFinding {
	return findSecrets(rel, text)
}

// findSecrets scans text and returns probable secrets with the line where they appear.
func findSecrets(rel, text string) []SecretFinding {
	var out []SecretFinding
	for i, line := range strings.Split(text, "\n") {
		lineNo := i + 1
		found := false
		if m := secretAssignment.FindStringSubmatch(line); m != nil && !looksLikePlaceholder(m[2]) {
			out = append(out, SecretFinding{Name: envName(m[1]), File: rel, Line: lineNo, Hint: "Senha ou chave escrita no código"})
			found = true
		}
		if found {
			continue
		}
		for _, p := range secretTokenPatterns {
			if p.re.MatchString(line) {
				out = append(out, SecretFinding{Name: envName(p.hint), File: rel, Line: lineNo, Hint: p.hint})
				break
			}
		}
	}
	return out
}

var nonEnv = regexp.MustCompile(`[^A-Z0-9]+`)

func envName(s string) string {
	return strings.Trim(nonEnv.ReplaceAllString(strings.ToUpper(foldAccents(s)), "_"), "_")
}

const secretMask = "••••"

// isEnvFile reports whether a path is a .env file (.env, prod.env, .env.local…).
func isEnvFile(rel string) bool {
	base := path.Base(strings.ReplaceAll(rel, "\\", "/"))
	return base == ".env" || strings.HasSuffix(base, ".env") || strings.HasPrefix(base, ".env.")
}

// MaskSecretsIn is MaskSecrets for a file at rel: in .env files every value
// is hidden, whatever its name.
func MaskSecretsIn(rel, text string) string {
	if isEnvFile(rel) {
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if m := envLine.FindStringSubmatchIndex(line); m != nil && utf8.RuneCountInString(line[m[4]:m[5]]) >= 4 {
				lines[i] = line[:m[4]] + secretMask + line[m[5]:]
			}
		}
		text = strings.Join(lines, "\n")
	}
	return MaskSecrets(text)
}

// MaskSecrets hides the values findSecrets would report, line by line, so code
// can be shown on screen, plus unquoted values of secret-named keys in
// config-style lines. Placeholders (os.getenv, ${X}…) stay as they are.
func MaskSecrets(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if m := unquotedSecret.FindStringSubmatchIndex(line); m != nil {
			v := line[m[4]:m[5]]
			if utf8.RuneCountInString(v) >= 4 && !strings.Contains(v, "(") && !looksLikePlaceholder(v) {
				line = line[:m[4]] + secretMask + line[m[5]:]
			}
		}
		ms := secretAssignment.FindAllStringSubmatchIndex(line, -1)
		for k := len(ms) - 1; k >= 0; k-- {
			s, e := ms[k][4], ms[k][5]
			if !looksLikePlaceholder(line[s:e]) {
				line = line[:s] + secretMask + line[e:]
			}
		}
		for _, p := range secretTokenPatterns {
			line = p.re.ReplaceAllString(line, secretMask)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
