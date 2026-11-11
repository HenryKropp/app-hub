package handler

import (
	"fmt"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// MCP D: the Ziv way of speaking. Tools that finish something hand Claude a
// ready sentence for the person (mensagem_para_pessoa), in the Ziv's words,
// so every answer sounds the same and has no technical terms.

// jargaoProibido are words the person never reads (tests enforce it).
var jargaoProibido = []string{"manifesto", "manifest", "entrypoint", "mcp", "token", "deploy", "container", "endpoint", "json", "api", "kind"}

// msgPessoaApp: an app just registered or a new version sent.
func msgPessoaApp(m manifest.Manifest, version int32, isNew, changed, web bool, link string) string {
	tipo := "app"
	if web {
		tipo = "app com tela"
	}
	var b strings.Builder
	switch {
	case !changed:
		fmt.Fprintf(&b, "Nada mudou: o código é igual ao da versão %d do %s %q.", version, tipo, m.Name)
	case isNew:
		fmt.Fprintf(&b, "Pronto: o %s %q está no Ziv.", tipo, m.Name)
	default:
		fmt.Fprintf(&b, "Pronto: a versão %d do %s %q está no Ziv.", version, tipo, m.Name)
	}
	if changed && !web {
		if pede := labels(inputLabels(m.Inputs)); pede != "" {
			b.WriteString(" Ele pede " + pede + ".")
		}
		if entrega := labels(outputLabels(m.Outputs)); entrega != "" {
			b.WriteString(" Entrega " + entrega + ".")
		}
	}
	if changed && web {
		b.WriteString(" Falta publicar para ele ficar no ar; já estou cuidando disso.")
	}
	b.WriteString(" Abrir: " + link)
	return b.String()
}

func inputLabels(in []manifest.Input) []string {
	var out []string
	for _, i := range in {
		if l := strings.TrimSpace(i.Label); l != "" {
			out = append(out, lowerFirst(l))
		}
	}
	return out
}

func outputLabels(in []manifest.Output) []string {
	var out []string
	for _, o := range in {
		if l := strings.TrimSpace(o.Label); l != "" {
			out = append(out, lowerFirst(l))
		}
	}
	return out
}

// labels joins "a", "a e b", "a, b e c".
func labels(l []string) string {
	switch len(l) {
	case 0:
		return ""
	case 1:
		return l[0]
	}
	return strings.Join(l[:len(l)-1], ", ") + " e " + l[len(l)-1]
}

// lowerFirst lowers the first letter unless the word looks like a name or
// acronym (two capitals in a row).
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) > 1 && r[1] >= 'A' && r[1] <= 'Z' {
		return s
	}
	return strings.ToLower(string(r[:1])) + string(r[1:])
}

// msgPessoaTela: how a publish of a screen app ended.
func msgPessoaTela(live bool, endereco string, versaoNoAr *int32) string {
	if live {
		return "Pronto: o app com tela está no ar em " + endereco + "."
	}
	if versaoNoAr != nil {
		return fmt.Sprintf("A versão nova não entrou no ar; a versão %d continua funcionando normalmente. Estou vendo o motivo para corrigir.", *versaoNoAr)
	}
	return "O app com tela ainda não entrou no ar. Estou vendo o motivo para corrigir."
}

// msgPessoaExecucao: a run that finished.
func msgPessoaExecucao(app, status, erro, pasta string, files int, link string) string {
	switch status {
	case "done":
		if files == 0 {
			return fmt.Sprintf("Pronto: a execução do %s terminou. Abrir: %s", app, link)
		}
		return fmt.Sprintf("Pronto: a execução do %s terminou e deixou %d arquivo(s) em %s. Abrir: %s", app, files, pasta, link)
	case "no_files":
		return fmt.Sprintf("A execução do %s terminou, mas não gerou arquivos novos. Abrir: %s", app, link)
	case "error", "interrupted", "stopped":
		msg := fmt.Sprintf("A execução do %s não terminou bem", app)
		if e := strings.TrimSpace(erro); e != "" {
			msg += ": " + strings.TrimSuffix(e, ".")
		}
		return msg + ". Abrir: " + link
	}
	return ""
}

// msgPessoaArtifact: an artifact published.
func msgPessoaArtifact(name string, version int32, link string) string {
	if version > 1 {
		return fmt.Sprintf("Pronto: a versão %d do artifact %q está no Ziv. Abrir: %s", version, name, link)
	}
	return fmt.Sprintf("Pronto: o artifact %q está no Ziv. Abrir: %s", name, link)
}

// temJargao lists the forbidden words found in a text for the person.
func temJargao(text string) []string {
	var found []string
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r == '_' || r > 127)
	}) {
		for _, bad := range jargaoProibido {
			if w == bad {
				found = append(found, w)
			}
		}
	}
	return found
}
