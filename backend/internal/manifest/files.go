package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrForeignFile means a file with the Hub's name exists but was not written by the Hub.
var ErrForeignFile = errors.New("já existe um arquivo com esse nome na pasta que não foi criado pelo Hub")

// Marshal renders the manifest as commented YAML.
func Marshal(m Manifest) ([]byte, error) {
	m.ManifestVersion = Version
	var buf bytes.Buffer
	buf.WriteString(headerMarker + "\n")
	buf.WriteString("# Gerado e mantido pelo App Hub. Descreve como o app roda, o que recebe e o que devolve.\n")
	buf.WriteString("# Pode ser lido e editado fora do Hub. Nunca coloque senhas ou chaves aqui.\n\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Parse reads a manifest file's content.
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	err := yaml.Unmarshal(data, &m)
	return m, err
}

// WriteToFolder saves app-hub.yaml and FICHA.md in dir. It only ever creates or
// replaces those two files, and refuses to replace a file the Hub did not write.
func WriteToFolder(dir string, m Manifest) error {
	manifestBytes, err := Marshal(m)
	if err != nil {
		return err
	}
	files := []struct {
		name, marker string
		content      []byte
	}{
		{FileName, headerMarker, manifestBytes},
		{FichaName, fichaMarker, []byte(RenderFicha(m))},
	}
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if existing, err := os.ReadFile(path); err == nil {
			if !bytes.Contains(existing, []byte(f.marker)) {
				return fmt.Errorf("%s: %w", f.name, ErrForeignFile)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), f.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

var inputTypeLabels = map[string]string{
	"file": "arquivo", "folder": "pasta", "text": "texto", "number": "número", "date": "data", "choice": "lista de opções",
}

var inputSourceLabels = map[string]string{
	"fixed_file": "arquivo com nome fixo na pasta do app",
	"dialog":     "janela de seleção do Windows",
	"prompt":     "pergunta no terminal",
	"argument":   "parâmetro na linha de comando",
}

// RenderFicha produces the human-readable ficha exported next to the app.
func RenderFicha(m Manifest) string {
	var b strings.Builder
	b.WriteString(fichaMarker + "\n")
	fmt.Fprintf(&b, "# %s\n\n", m.Name)
	if m.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", m.Summary)
	} else {
		b.WriteString("_Resumo ainda não escrito._\n\n")
	}
	fmt.Fprintf(&b, "- **Tipo:** %s\n", KindLabels[m.Kind])
	if m.Area != "" {
		fmt.Fprintf(&b, "- **Área:** %s\n", m.Area)
	}
	if m.Owner != nil {
		fmt.Fprintf(&b, "- **Responsável:** %s\n", m.Owner.Name)
	}
	if m.Backup != nil {
		fmt.Fprintf(&b, "- **Reserva:** %s\n", m.Backup.Name)
	} else {
		b.WriteString("- **Reserva:** ninguém definido ainda\n")
	}
	if m.Language != "" {
		fmt.Fprintf(&b, "- **Linguagem:** %s\n", m.Language)
	}
	if m.Entrypoint != "" {
		fmt.Fprintf(&b, "- **Arquivo que inicia o app:** `%s`\n", m.Entrypoint)
	}
	if m.Schedule != "" {
		fmt.Fprintf(&b, "- **Agenda:** %s\n", m.Schedule)
	}

	b.WriteString("\n## O que ele recebe\n\n")
	if len(m.Inputs) == 0 {
		b.WriteString("Nada. É só rodar.\n")
	}
	for _, in := range m.Inputs {
		fmt.Fprintf(&b, "- **%s** (%s, via %s)", in.Label, inputTypeLabels[in.Type], inputSourceLabels[in.Source])
		if in.Path != "" {
			fmt.Fprintf(&b, ": `%s`", in.Path)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n## O que ele devolve\n\n")
	if len(m.Outputs) == 0 {
		b.WriteString("Ainda não identificado.\n")
	}
	for _, out := range m.Outputs {
		fmt.Fprintf(&b, "- **%s**: `%s`\n", out.Label, out.Pattern)
	}

	if len(m.Runtime.Dependencies) > 0 {
		b.WriteString("\n## Do que ele depende\n\n")
		for _, d := range m.Runtime.Dependencies {
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}

	if len(m.Secrets) > 0 {
		b.WriteString("\n## Chaves e senhas\n\n")
		b.WriteString("Este app usa chaves ou senhas. Os valores **não** aparecem aqui.\n\n")
		for _, s := range m.Secrets {
			fmt.Fprintf(&b, "- %s (em `%s`, linha %d)\n", s.Name, s.File, s.Line)
		}
	}

	if len(m.Notes) > 0 {
		b.WriteString("\n## Pontos de atenção\n\n")
		for _, n := range m.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	b.WriteString("\n---\n_Ficha gerada pelo App Hub a partir do manifesto (`app-hub.yaml`)._\n")
	return b.String()
}
