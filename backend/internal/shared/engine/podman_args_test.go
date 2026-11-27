package engine

import (
	"slices"
	"testing"
)

// Every app box drops Linux capabilities, cannot gain privileges and has a
// process limit (Fase 0 de segurança).
func TestStartArgsLockDown(t *testing.T) {
	args := startArgs(StartSpec{Name: "ziv-x-1", Image: "img", Network: "ziv-apps-1", Port: 8080, MemoryMB: 128, CPUs: 0.5}, "/tmp/env")
	for _, pair := range [][2]string{{"--cap-drop", "ALL"}, {"--security-opt", "no-new-privileges"}, {"--pids-limit", "256"}} {
		i := slices.Index(args, pair[0])
		if i < 0 || i+1 >= len(args) || args[i+1] != pair[1] {
			t.Fatalf("missing %s %s in %v", pair[0], pair[1], args)
		}
	}
	if args[len(args)-1] != "img" {
		t.Fatalf("image must be last: %v", args)
	}
}
