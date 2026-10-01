package manifest

import "testing"

func TestDiff(t *testing.T) {
	a := Manifest{Name: "Files Prep", Summary: "Baixa", Area: "Research", Kind: KindDesktop,
		Owner: &PersonRef{Name: "Ana"}, Inputs: []Input{{ID: "x", Label: "X", Type: "text", Source: "window"}}}
	b := a
	b.Area = "Equity"
	b.Owner = &PersonRef{Name: "Henry"}
	b.Summary = "Baixa da CVM"
	b.Inputs = []Input{{ID: "x", Label: "Empresa", Type: "text", Source: "window"}}
	got := Diff(a, b)
	if len(got) != 4 {
		t.Fatalf("%+v", got)
	}
	if got[0].Field != "summary" || !got[0].Long || got[1].Field != "area" || got[1].Before != "Research" || got[1].After != "Equity" ||
		got[2].Label != "Responsável" || got[2].After != "Henry" || got[3].Field != "inputs" || !got[3].Long {
		t.Fatalf("%+v", got)
	}
	// nil and empty lists are the same.
	c := a
	c.Tags = []string{}
	if d := Diff(a, c); len(d) != 0 {
		t.Fatalf("empty vs nil %+v", d)
	}
	if cr := Created(); len(cr) != 1 || cr[0].Label != "Ficha criada" {
		t.Fatalf("%+v", cr)
	}
}
