// Package manifest defines the generic, versioned description of an app that
// the Hub stores next to the app's code (app-hub.yaml). Every feature of the
// Hub — catalog, ficha, run form, MCP — is driven by this file.
package manifest

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	Version      = 1
	FileName     = "app-hub.yaml"
	FichaName    = "FICHA.md"
	headerMarker = "# App Hub — manifesto do app"
	fichaMarker  = "<!-- app-hub:ficha -->"
)

// Kinds of app the Hub understands.
const (
	KindScript     = "script"
	KindWeb        = "web"
	KindDesktop    = "desktop"
	KindScheduled  = "scheduled"
	KindExecutable = "executable"
	KindReference  = "reference"
	KindSystem     = "system"
)

var Kinds = []string{KindScript, KindWeb, KindDesktop, KindScheduled, KindExecutable, KindReference, KindSystem}

// KindLabels are the plain-Portuguese names shown to people.
var KindLabels = map[string]string{
	KindScript:     "Script (roda e entrega um resultado)",
	KindWeb:        "App com tela no navegador",
	KindDesktop:    "App com janela própria",
	KindScheduled:  "Tarefa recorrente",
	KindExecutable: "Executável pronto (.exe)",
	KindReference:  "Documento ou atalho (não roda)",
	KindSystem:     "Sistema complexo (só registro)",
}

// Input types and sources.
var InputTypes = []string{"file", "folder", "text", "number", "date", "choice"}
var InputSources = []string{"fixed_file", "dialog", "prompt", "argument"}

type Manifest struct {
	ManifestVersion int         `yaml:"manifest_version" json:"manifest_version"`
	ID              string      `yaml:"id" json:"id"`
	Name            string      `yaml:"name" json:"name"`
	Summary         string      `yaml:"summary" json:"summary"`
	Kind            string      `yaml:"kind" json:"kind"`
	Language        string      `yaml:"language,omitempty" json:"language"`
	Entrypoint      string      `yaml:"entrypoint,omitempty" json:"entrypoint"`
	Run             Run         `yaml:"run,omitempty" json:"run"`
	Runtime         Runtime     `yaml:"runtime,omitempty" json:"runtime"`
	Inputs          []Input     `yaml:"inputs,omitempty" json:"inputs"`
	Outputs         []Output    `yaml:"outputs,omitempty" json:"outputs"`
	Secrets         []SecretRef `yaml:"secrets,omitempty" json:"secrets"`
	Schedule        string      `yaml:"schedule,omitempty" json:"schedule"`
	Area            string      `yaml:"area,omitempty" json:"area"`
	Owner           *PersonRef  `yaml:"owner,omitempty" json:"owner"`
	Backup          *PersonRef  `yaml:"backup,omitempty" json:"backup"`
	Tags            []string    `yaml:"tags,omitempty" json:"tags"`
	Notes           []string    `yaml:"notes,omitempty" json:"notes"`
}

type Run struct {
	Command string `yaml:"command,omitempty" json:"command"`
	URL     string `yaml:"url,omitempty" json:"url"`
}

type Runtime struct {
	Language         string   `yaml:"language,omitempty" json:"language"`
	DependenciesFile string   `yaml:"dependencies_file,omitempty" json:"dependencies_file"`
	Dependencies     []string `yaml:"dependencies,omitempty" json:"dependencies"`
}

type Input struct {
	ID       string   `yaml:"id" json:"id"`
	Label    string   `yaml:"label" json:"label"`
	Type     string   `yaml:"type" json:"type"`
	Source   string   `yaml:"source" json:"source"`
	Path     string   `yaml:"path,omitempty" json:"path"`
	Options  []string `yaml:"options,omitempty" json:"options"`
	Required bool     `yaml:"required" json:"required"`
}

type Output struct {
	Label   string `yaml:"label" json:"label"`
	Pattern string `yaml:"pattern" json:"pattern"`
}

// SecretRef points at where a secret lives. It never holds the value.
type SecretRef struct {
	Name string `yaml:"name" json:"name"`
	File string `yaml:"file" json:"file"`
	Line int    `yaml:"line" json:"line"`
	Hint string `yaml:"hint,omitempty" json:"hint"`
}

type PersonRef struct {
	Name  string `yaml:"name" json:"name"`
	Email string `yaml:"email" json:"email"`
}

// Validate returns plain-Portuguese problems that block publishing.
func (m Manifest) Validate() []string {
	var errs []string
	if strings.TrimSpace(m.Name) == "" {
		errs = append(errs, "O app precisa de um nome.")
	}
	if !contains(Kinds, m.Kind) {
		errs = append(errs, "Escolha o tipo do app.")
	}
	if m.Owner == nil || m.Owner.Email == "" {
		errs = append(errs, "Todo app precisa de um responsável.")
	}
	if m.Owner != nil && m.Backup != nil && m.Owner.Email != "" && m.Owner.Email == m.Backup.Email {
		errs = append(errs, "O reserva precisa ser uma pessoa diferente do responsável.")
	}
	runnable := m.Kind == KindScript || m.Kind == KindWeb || m.Kind == KindDesktop || m.Kind == KindScheduled || m.Kind == KindExecutable
	if runnable && strings.TrimSpace(m.Entrypoint) == "" {
		errs = append(errs, "Informe o arquivo que inicia o app.")
	}
	seen := map[string]bool{}
	for _, in := range m.Inputs {
		if strings.TrimSpace(in.Label) == "" {
			errs = append(errs, "Toda entrada precisa de um nome.")
		}
		if !contains(InputTypes, in.Type) {
			errs = append(errs, fmt.Sprintf("Tipo de entrada inválido: %q.", in.Type))
		}
		if !contains(InputSources, in.Source) {
			errs = append(errs, fmt.Sprintf("Origem de entrada inválida: %q.", in.Source))
		}
		if seen[in.ID] {
			errs = append(errs, fmt.Sprintf("Entrada repetida: %q.", in.ID))
		}
		seen[in.ID] = true
	}
	return errs
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns "Rentabilidade Fundos (v2)" into "rentabilidade-fundos-v2".
func Slugify(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	folded, _, _ := transform.String(t, strings.ToLower(s))
	slug := strings.Trim(nonSlug.ReplaceAllString(folded, "-"), "-")
	if slug == "" {
		return "app"
	}
	return slug
}
