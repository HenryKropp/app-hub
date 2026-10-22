package manifest

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxInputBytes = 10 << 20

type InputFile struct {
	Name          string `json:"name"`
	ContentBase64 string `json:"content_base64"`
}

var inputID = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
var inputFlag = regexp.MustCompile(`^--?[a-zA-Z][a-zA-Z0-9_-]*$`)

// SafeRelative rejects traversal, drive paths and ambiguous Windows filenames.
func SafeRelative(s string) bool {
	s = strings.ReplaceAll(s, `\`, "/")
	if s == "" || strings.HasPrefix(s, "/") || strings.ContainsAny(s, ":\x00") {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" ||
			(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
			return false
		}
		if strings.ContainsAny(part, `<>|?*"`) {
			return false
		}
	}
	return true
}

func ScriptRuntime(entry string) string {
	if !SafeRelative(entry) {
		return ""
	}
	switch strings.ToLower(path.Ext(strings.ReplaceAll(entry, `\`, "/"))) {
	case ".py":
		return "python"
	case ".js", ".mjs", ".cjs":
		return "node"
	}
	return ""
}

func (m Manifest) ScriptProblems() []string {
	var errs []string
	if ScriptRuntime(m.Entrypoint) == "" {
		errs = append(errs, "Escolha um script Python (.py) ou JavaScript (.js, .mjs, .cjs) dentro da pasta do app.")
	}
	if m.Runtime.DependenciesFile != "" && !SafeRelative(m.Runtime.DependenciesFile) {
		errs = append(errs, "O arquivo de dependências precisa ficar dentro da pasta do app.")
	}
	for _, in := range m.Inputs {
		if !inputID.MatchString(in.ID) {
			errs = append(errs, "Use um identificador de entrada com letras, números, hífen ou sublinhado, começando por uma letra.")
		}
		if in.Flag != "" && !inputFlag.MatchString(in.Flag) {
			errs = append(errs, fmt.Sprintf("O argumento de %q deve ser uma opção como --arquivo.", in.Label))
		}
		if in.Source == "fixed_file" {
			ext := strings.ToLower(path.Ext(in.Path))
			executable := contains([]string{".py", ".js", ".mjs", ".cjs", ".exe", ".dll", ".bat", ".cmd", ".ps1", ".sh"}, ext)
			if in.Type != "file" || !SafeRelative(in.Path) || executable || strings.EqualFold(in.Path, FileName) || strings.EqualFold(in.Path, "package.json") || strings.HasPrefix(strings.ToLower(in.Path), ".apphub-") {
				errs = append(errs, fmt.Sprintf("Defina um caminho relativo de dados, não de código, para o arquivo %q.", in.Label))
			}
		}
	}
	return errs
}

// ValidateRunInputs runs both at the Hub boundary and again on the agent.
func (m Manifest) ValidateRunInputs(values map[string][]string, files map[string]InputFile) error {
	if problems := m.ScriptProblems(); len(problems) > 0 {
		return errors.New(strings.Join(problems, " "))
	}
	known := map[string]bool{}
	total := 0
	targets := map[string]bool{}
	for _, in := range m.Inputs {
		if known[in.ID] {
			return errors.New("Há identificadores de entrada repetidos na ficha.")
		}
		known[in.ID] = true
		v := values[in.ID]
		f, hasFile := files[in.ID]
		if in.Source == "window" {
			if len(v) > 0 || hasFile {
				return fmt.Errorf("%s é preenchido na janela do app.", in.Label)
			}
			continue
		}
		if in.Type == "file" {
			if len(v) > 0 {
				return fmt.Errorf("Envie um arquivo para %s.", in.Label)
			}
			if !hasFile {
				if in.Required {
					return fmt.Errorf("Selecione o arquivo %s.", in.Label)
				}
				continue
			}
			if !SafeRelative(f.Name) || strings.ContainsAny(f.Name, `/\`) || len(f.Name) > 180 {
				return errors.New("O nome do arquivo enviado é inválido.")
			}
			data, err := base64.StdEncoding.DecodeString(f.ContentBase64)
			if err != nil {
				return errors.New("Não consegui ler o arquivo enviado. Selecione-o novamente.")
			}
			total += len(data)
			if total > MaxInputBytes {
				return errors.New("Os arquivos de entrada podem somar no máximo 10 MB.")
			}
			dest := strings.ToLower(strings.ReplaceAll(in.Path, `\`, "/"))
			if in.Source == "fixed_file" {
				if targets[dest] || strings.EqualFold(dest, strings.ReplaceAll(m.Entrypoint, `\`, "/")) {
					return errors.New("O destino de um arquivo de entrada coincide com outro arquivo de entrada ou com o script.")
				}
				targets[dest] = true
			}
			continue
		}
		if hasFile {
			return fmt.Errorf("%s não aceita arquivo.", in.Label)
		}
		if len(v) == 0 || (len(v) == 1 && strings.TrimSpace(v[0]) == "") {
			if in.Required {
				return fmt.Errorf("Preencha %s.", in.Label)
			}
			continue
		}
		if len(v) > 1 && !(in.Type == "choice" && in.Multiple) {
			return fmt.Errorf("%s aceita apenas um valor.", in.Label)
		}
		if len(v) > 100 {
			return errors.New("Há opções demais nesta entrada.")
		}
		seen := map[string]bool{}
		for _, value := range v {
			if len(value) > 4096 || strings.ContainsRune(value, 0) || (in.Source == "prompt" && strings.ContainsAny(value, "\r\n")) {
				return fmt.Errorf("O valor de %s é inválido.", in.Label)
			}
			switch in.Type {
			case "number":
				n, err := strconv.ParseFloat(value, 64)
				if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
					return fmt.Errorf("Informe um número válido para %s.", in.Label)
				}
			case "date":
				if _, err := time.Parse("2006-01-02", value); err != nil {
					return fmt.Errorf("Informe uma data válida para %s.", in.Label)
				}
			case "choice":
				if !contains(in.Options, value) || seen[value] {
					return fmt.Errorf("Escolha opções válidas para %s.", in.Label)
				}
				seen[value] = true
			case "text", "folder":
			default:
				return errors.New("A ficha contém um tipo de entrada não suportado.")
			}
		}
	}
	for id := range values {
		if !known[id] {
			return errors.New("O formulário contém uma entrada que não existe na ficha.")
		}
	}
	for id := range files {
		if !known[id] {
			return errors.New("O formulário contém um arquivo que não existe na ficha.")
		}
	}
	return nil
}
