package manifest

import "encoding/json"

// Change is one field of the ficha that changed. Long fields (lists, blocks,
// the summary) appear as "X alterado" with before/after on demand.
type Change struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Long   bool   `json:"long,omitempty"`
}

// Created is the first entry of an app's ficha history.
func Created() []Change { return []Change{{Field: "created", Label: "Ficha criada"}} }

// Diff lists what changed from ficha a to ficha b, in display order.
func Diff(a, b Manifest) []Change {
	var out []Change
	add := func(field, label, x, y string, long bool) {
		if x != y {
			out = append(out, Change{Field: field, Label: label, Before: x, After: y, Long: long})
		}
	}
	add("id", "Identificador", a.ID, b.ID, false)
	add("name", "Nome", a.Name, b.Name, false)
	add("summary", "Resumo", a.Summary, b.Summary, true)
	add("kind", "Tipo", KindLabels[a.Kind], KindLabels[b.Kind], false)
	add("language", "Linguagem", a.Language, b.Language, false)
	add("area", "Área", a.Area, b.Area, false)
	add("owner", "Responsável", personName(a.Owner), personName(b.Owner), false)
	add("backup", "Reserva", personName(a.Backup), personName(b.Backup), false)
	add("entrypoint", "Arquivo que abre o app", a.Entrypoint, b.Entrypoint, false)
	add("schedule", "Agendamento", a.Schedule, b.Schedule, false)
	add("run", "Como roda", jsonText(a.Run), jsonText(b.Run), true)
	add("runtime", "Ambiente", jsonText(a.Runtime), jsonText(b.Runtime), true)
	add("inputs", "Entradas", jsonText(a.Inputs), jsonText(b.Inputs), true)
	add("outputs", "Saídas", jsonText(a.Outputs), jsonText(b.Outputs), true)
	add("secrets", "Chaves e senhas", jsonText(a.Secrets), jsonText(b.Secrets), true)
	add("tags", "Etiquetas", jsonText(a.Tags), jsonText(b.Tags), true)
	add("depends_on", "Depende de", jsonText(a.DependsOn), jsonText(b.DependsOn), true)
	add("build", "Como recompilar", jsonText(a.Build), jsonText(b.Build), true)
	add("notes", "Observações", jsonText(a.Notes), jsonText(b.Notes), true)
	add("shared", "Compartilhado", jsonText(a.Shared), jsonText(b.Shared), true)
	return out
}

func personName(p *PersonRef) string {
	if p == nil {
		return ""
	}
	return p.Name
}

// jsonText renders a block for before/after; nil and empty are both "".
func jsonText(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	switch s := string(b); s {
	case "null", "[]", "{}":
		return ""
	default:
		return s
	}
}
