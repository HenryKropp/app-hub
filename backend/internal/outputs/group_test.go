package outputs

import (
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

var filesPrep = []manifest.Output{
	{Label: "Documentos da CVM", Pattern: "*/*/*.pdf"},
	{Label: "Data Pack", Pattern: "*/MD_Output/*_DataPack_Consolidado.md"},
	{Label: "FRE cortado", Pattern: "*_cortado_secoes.pdf"},
}

func TestMatch(t *testing.T) {
	cases := []struct {
		rel   string
		index int
		label string
	}{
		{"PETROBRAS/FRE/FRE_2025.pdf", 0, "Documentos da CVM"},
		{"PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", 1, "Data Pack"},
		{"PETROBRAS/FRE/FRE_2025_cortado_secoes.pdf", 2, "FRE cortado"},
		{"FRE_cortado_secoes.pdf", 2, "FRE cortado"},
		{"log_download.txt", -1, OtherLabel},
		{"PETROBRAS/x.pdf", -1, OtherLabel},
	}
	for _, c := range cases {
		i, l := Match(filesPrep, c.rel)
		if i != c.index || l != c.label {
			t.Errorf("Match(%q) = %d %q, want %d %q", c.rel, i, l, c.index, c.label)
		}
	}
}

func TestMatchNoOutputs(t *testing.T) {
	if i, l := Match(nil, "a.txt"); i != -1 || l != OtherLabel {
		t.Fatalf("got %d %q", i, l)
	}
}

// Real layout of the Eq. Research Files Prep, seen in the first run.
var filesPrepReal = []manifest.Output{
	{Label: "Documentos da CVM", Pattern: "*/*/**/*.pdf"},
	{Label: "Data Pack", Pattern: "*/MD_Output/*_DataPack_Consolidado.md"},
	{Label: "FRE cortado", Pattern: "*_secoes.pdf"},
}

func TestMatchDoubleStar(t *testing.T) {
	cases := []struct {
		rel   string
		index int
	}{
		{"PETROBRAS/5. Formulário Referência/2025/FRE_2025-12-31_v1_completo.pdf", 0},
		{"PETROBRAS/3. ITR/2025/1T/ITR_2025-03-31_v1.pdf", 0},
		{"PETROBRAS/FRE/x.pdf", 0}, // ** matches zero folders
		{"PETROBRAS/5. Formulário Referência/2025/FRE_2025-12-31_v1_secoes.pdf", 2},
		{"PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", 1},
		{"PETROBRAS/relatorio BTG.pdf", -1},
		{"log.txt", -1},
	}
	for _, c := range cases {
		if i, _ := Match(filesPrepReal, c.rel); i != c.index {
			t.Errorf("Match(%q) = %d, want %d", c.rel, i, c.index)
		}
	}
}
