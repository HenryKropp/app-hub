// Command validate-manifest checks a draft kova.yaml (or a legacy app-hub.yaml) with Kova's own
// reader and rules, and prints the ficha people would see. Used by the
// registration protocol tests (docs/PROTOCOLO-REGISTRO.md) before showing a draft.
package main

import (
	"fmt"
	"os"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: validate-manifest <caminho/kova.yaml>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m, err := manifest.Parse(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "formato inválido:", err)
		os.Exit(1)
	}
	if errs := m.Validate(); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "-", e)
		}
		os.Exit(1)
	}
	fmt.Print(manifest.RenderFicha(m))
}
