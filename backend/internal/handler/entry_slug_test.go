package handler

import "testing"

func TestSlugFromName(t *testing.T) {
	for name, want := range map[string]string{
		"Empresa Piloto": "empresa-piloto",
		"Ação & Cia.":        "acao-cia",
		"  Horizonte  ":      "horizonte",
		"3M do Brasil":       "m-do-brasil",
		"XP":                 "empresa-xp",
		"99":                 "empresa-99",
		"!!!":                "empresa",
		"Uma Empresa Com Um Nome Muito Comprido Mesmo": "uma-empresa-com-um-nome-muito-comp",
	} {
		got := slugFromName(name)
		if got != want || !companySlug.MatchString(got) {
			t.Errorf("slugFromName(%q) = %q, want %q", name, got, want)
		}
	}
}
