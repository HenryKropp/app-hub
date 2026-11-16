package handler

import (
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func TestFichaChangesTeste(t *testing.T) {
	cur := manifest.Manifest{Kind: manifest.KindScript}
	next := cur
	next.Teste = &manifest.Teste{Acessa: []string{"BTG"}, Pula: "nada", Espera: []manifest.Espera{{Arquivos: "*.pdf"}}}
	f, changed, _ := fichaChanges(cur, map[string]any{"teste": map[string]any{}}, next)
	if f == nil || f.Teste == nil || len(changed) != 1 || changed[0] != "plano de teste" {
		t.Fatalf("f=%+v changed=%v", f, changed)
	}
	got := applyFicha(cur, f)
	if got.Teste == nil || got.Teste.Acessa[0] != "BTG" {
		t.Fatalf("applyFicha lost teste: %+v", got.Teste)
	}
	if f, _, _ := fichaChanges(next, map[string]any{"teste": map[string]any{}}, next); f != nil {
		t.Error("same teste is not a change")
	}
}
