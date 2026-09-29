package scanner

import (
	"regexp"
	"strings"
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

var secretAssignment = regexp.MustCompile(`(?i)(?:^|[^\w\-])([\w\-]*?(?:api[_\-]?key|apikey|secret|token|password|passwd|senha|pwd)[\w\-]*)["']?\s*[:=]\s*[fbrFBR]?["']([^"'\s]{8,})["']`)

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

// findSecrets scans text and returns probable secrets with the line where they appear.
func findSecrets(rel, text string) []SecretFinding {
	var out []SecretFinding
	for i, line := range strings.Split(text, "\n") {
		if len(line) > 2000 {
			line = line[:2000]
		}
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
