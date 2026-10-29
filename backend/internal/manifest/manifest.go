// Package manifest defines the generic, versioned description of an app that
// the Hub stores next to the app's code (kova.yaml, or app-hub.yaml in older apps). Every feature of the
// Hub — catalog, ficha, run form, MCP — is driven by this file.
package manifest

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	Version      = 1
	FileName     = "ziv.yaml"
	FichaName    = "FICHA.md"
	headerMarker = "# Ziv — manifesto do app"
	fichaMarker  = "<!-- ziv:ficha -->"
)

// Names written before Ziv, newest first: by Kova, and by the App Hub before
// it. Apps registered back then keep their kova.yaml or app-hub.yaml: it is
// still read and saved in place.
var (
	LegacyFileNames     = []string{"kova.yaml", "app-hub.yaml"}
	legacyHeaderMarkers = []string{"# Kova — manifesto do app", "# App Hub — manifesto do app"}
	legacyFichaMarkers  = []string{"<!-- kova:ficha -->", "<!-- app-hub:ficha -->"}
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

// "window" means the app's own window asks for it after opening: the Hub shows
// it in the ficha but never asks for it before running.
var InputSources = []string{"fixed_file", "dialog", "prompt", "argument", "window"}

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
	DependsOn       []string    `yaml:"depends_on,omitempty" json:"depends_on"`
	Build           *Build      `yaml:"build,omitempty" json:"build"`
	Notes           []string    `yaml:"notes,omitempty" json:"notes"`
	Shared          *Shared     `yaml:"shared,omitempty" json:"shared"`
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

// Build says where the source of a compiled app lives and how to rebuild it.
type Build struct {
	Source  string `yaml:"source" json:"source"`
	Command string `yaml:"command,omitempty" json:"command"`
}

type Input struct {
	ID       string   `yaml:"id" json:"id"`
	Label    string   `yaml:"label" json:"label"`
	Type     string   `yaml:"type" json:"type"`
	Source   string   `yaml:"source" json:"source"`
	Path     string   `yaml:"path,omitempty" json:"path"`
	Options  []string `yaml:"options,omitempty" json:"options"`
	Multiple bool     `yaml:"multiple,omitempty" json:"multiple"`
	Required bool     `yaml:"required" json:"required"`
	Flag     string   `yaml:"flag,omitempty" json:"flag,omitempty"`
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
	if m.Kind == KindScript {
		errs = append(errs, m.ScriptProblems()...)
	}
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
		if in.Multiple && in.Type != "choice" {
			errs = append(errs, fmt.Sprintf("Só uma lista de opções pode aceitar várias escolhas: %q.", in.Label))
		}
		if in.Type == "choice" && len(in.Options) == 0 {
			errs = append(errs, fmt.Sprintf("Escreva ao menos uma opção para %q.", in.Label))
		}
		if seen[in.ID] {
			errs = append(errs, fmt.Sprintf("Entrada repetida: %q.", in.ID))
		}
		seen[in.ID] = true
	}
	for _, dep := range m.DependsOn {
		if dep == m.ID && dep != "" {
			errs = append(errs, "O app não pode depender dele mesmo.")
		}
	}
	if m.Build != nil && strings.TrimSpace(m.Build.Source) == "" {
		errs = append(errs, "Informe onde está o código usado para gerar o app.")
	}
	errs = append(errs, m.sharedProblems()...)
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

// Shared describes how the Hub hosts a web app for the whole team.
type Shared struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	Port       int      `yaml:"port,omitempty" json:"port"`
	HealthPath string   `yaml:"health_path,omitempty" json:"health_path"`
	Database   string   `yaml:"database,omitempty" json:"database"` // "" ou "postgres"
	AlwaysOn   bool     `yaml:"always_on,omitempty" json:"always_on"`
	IdleMin    int      `yaml:"idle_minutes,omitempty" json:"idle_minutes"`
	MemoryMB   int      `yaml:"memory_mb,omitempty" json:"memory_mb"`
	CPUs       float64  `yaml:"cpus,omitempty" json:"cpus"`
	Python     string   `yaml:"python,omitempty" json:"python"`   // versão para a receita automática; padrão 3.12
	Node       string   `yaml:"node,omitempty" json:"node"`       // padrão 22
	Command    string   `yaml:"command,omitempty" json:"command"` // comando de início confirmado (Python web)
	Env        []EnvVar `yaml:"env,omitempty" json:"env"`
}

// EnvVar declares a setting the app needs. It never holds the value.
type EnvVar struct {
	Name     string `yaml:"name" json:"name"`
	Label    string `yaml:"label" json:"label"`
	Secret   bool   `yaml:"secret,omitempty" json:"secret"`
	Required bool   `yaml:"required,omitempty" json:"required"`
	Default  string `yaml:"default,omitempty" json:"default"`
}

// MsgCommandLines refuses a start command with line breaks.
const MsgCommandLines = "O comando de início precisa estar numa linha só."

// ReservedEnv are filled in by the Hub and cannot be declared by the app.
var ReservedEnv = []string{"PORT", "BASE_URL", "APPHUB_DATA_DIR", "DATABASE_URL"}

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// WithDefaults returns a copy with the Hub's defaults applied.
func (s *Shared) WithDefaults() Shared {
	d := *s
	if d.HealthPath == "" {
		d.HealthPath = "/"
	}
	if d.IdleMin == 0 {
		d.IdleMin = 30
	}
	if d.MemoryMB == 0 {
		d.MemoryMB = 512
	}
	if d.CPUs == 0 {
		d.CPUs = 1
	}
	if d.Python == "" {
		d.Python = "3.12"
	}
	if d.Node == "" {
		d.Node = "22"
	}
	return d
}

func (m Manifest) sharedProblems() []string {
	s := m.Shared
	if s == nil || !s.Enabled {
		return nil
	}
	var out []string
	if m.Kind != KindWeb {
		out = append(out, "Só apps com tela no navegador podem ser compartilhados.")
	}
	if s.Database != "" && s.Database != "postgres" {
		out = append(out, "O banco do app compartilhado precisa ser postgres.")
	}
	if s.Port < 0 || s.Port > 65535 {
		out = append(out, "A porta do app precisa estar entre 1 e 65535.")
	}
	if s.MemoryMB != 0 && (s.MemoryMB < 128 || s.MemoryMB > 16384) {
		out = append(out, "A memória do app precisa estar entre 128 e 16384 MB.")
	}
	if s.CPUs < 0 || s.CPUs > 16 {
		out = append(out, "O processamento do app precisa estar entre 0,1 e 16.")
	}
	if s.IdleMin != 0 && (s.IdleMin < 5 || s.IdleMin > 1440) {
		out = append(out, "O tempo para dormir precisa estar entre 5 e 1440 minutos.")
	}
	if s.HealthPath != "" && !strings.HasPrefix(s.HealthPath, "/") {
		out = append(out, "O endereço de saúde precisa começar com /.")
	}
	if strings.ContainsAny(s.Command, "\r\n") {
		out = append(out, MsgCommandLines)
	}
	seen := map[string]bool{}
	for _, e := range s.Env {
		switch {
		case slices.Contains(ReservedEnv, e.Name):
			out = append(out, e.Name+" é preenchida pelo Ziv; tire da lista de configurações.")
		case !envName.MatchString(e.Name):
			out = append(out, fmt.Sprintf("O nome da configuração %q só pode ter letras maiúsculas, números e _.", e.Name))
		case seen[e.Name]:
			out = append(out, "A configuração "+e.Name+" aparece duas vezes.")
		}
		seen[e.Name] = true
	}
	return out
}
