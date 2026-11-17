// Package risk reads, from a version's ficha and code, what that version can
// reach: vault keys, sites, a browser, running 24h on the server, and sending
// things out of the company. The governance rule "só quando muda o risco"
// (ADR 046) asks for approval when a version adds to this set.
package risk

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

type Set struct {
	Keys     []string `json:"keys"`
	Domains  []string `json:"domains"`
	Browser  bool     `json:"browser"`
	AlwaysOn bool     `json:"always_on"`
	SendsOut bool     `json:"sends_out"`
}

const maxFile = 200 << 10

var (
	textExt = map[string]bool{".py": true, ".js": true, ".ts": true, ".mjs": true, ".cjs": true, ".html": true, ".htm": true,
		".yaml": true, ".yml": true, ".json": true, ".toml": true, ".txt": true, ".cfg": true, ".ini": true}
	urlRe     = regexp.MustCompile(`(?i)https?://([a-z0-9.-]+)`)
	ignored   = map[string]bool{"localhost": true, "127.0.0.1": true, "0.0.0.0": true, "example.com": true, "www.example.com": true, "www.w3.org": true}
	browserRe = regexp.MustCompile(`(?mi)^\s*(?:from|import)\s+(?:playwright|selenium|pyppeteer)\b|^\s*playwright\b|^\s*selenium\b|require\(\s*["'](?:playwright|puppeteer)|from\s+["'](?:playwright|puppeteer)`)
	sendsRe   = regexp.MustCompile(`(?mi)^\s*(?:from|import)\s+(?:smtplib|yagmail|sendgrid|resend)\b|win32com\.client[\s\S]{0,400}Outlook|require\(\s*["'](?:nodemailer|resend|@sendgrid/mail)|from\s+["'](?:nodemailer|resend|@sendgrid/mail)`)
)

// Of reads the risk of a ficha plus its files (path → content). With nil
// files (a ficha-only change) only Keys and AlwaysOn are known; use Merge.
func Of(m manifest.Manifest, files map[string][]byte) Set {
	var s Set
	keys := map[string]bool{}
	for _, k := range m.Keys {
		keys[k.Name] = true
	}
	if m.Shared != nil {
		s.AlwaysOn = m.Shared.Enabled
		for _, e := range m.Shared.Env {
			if e.Secret {
				keys[e.Name] = true
			}
		}
	}
	s.Keys = sorted(keys)
	domains := map[string]bool{}
	for name, b := range files {
		if !textExt[strings.ToLower(path.Ext(name))] || len(b) > maxFile {
			continue
		}
		for _, mm := range urlRe.FindAllSubmatch(b, -1) {
			host := strings.Trim(strings.ToLower(string(mm[1])), ".")
			if host != "" && !ignored[host] && !strings.HasPrefix(host, "schemas.") && strings.Contains(host, ".") {
				domains[host] = true
			}
		}
		s.Browser = s.Browser || browserRe.Match(b)
		s.SendsOut = s.SendsOut || sendsRe.Match(b)
	}
	s.Domains = sorted(domains)
	return s
}

// Merge: a ficha-only change takes Keys and AlwaysOn from the new ficha and
// keeps what only the code tells from the previous version.
func Merge(ficha, prev Set) Set {
	return Set{Keys: ficha.Keys, Domains: prev.Domains, Browser: prev.Browser, AlwaysOn: ficha.AlwaysOn, SendsOut: prev.SendsOut}
}

// Added lists, in Portuguese, what s has that prev did not. What was removed
// never counts.
func (s Set) Added(prev Set) []string {
	var out []string
	for _, k := range minus(s.Keys, prev.Keys) {
		out = append(out, "chave do Cofre: "+k)
	}
	for _, d := range minus(s.Domains, prev.Domains) {
		out = append(out, "site: "+d)
	}
	if s.Browser && !prev.Browser {
		out = append(out, "usa navegador")
	}
	if s.AlwaysOn && !prev.AlwaysOn {
		out = append(out, "roda 24h no servidor")
	}
	if s.SendsOut && !prev.SendsOut {
		out = append(out, "manda coisas para fora")
	}
	return out
}

func minus(a, b []string) []string {
	have := map[string]bool{}
	for _, x := range b {
		have[x] = true
	}
	var out []string
	for _, x := range a {
		if !have[x] {
			out = append(out, x)
		}
	}
	return out
}

func sorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}
