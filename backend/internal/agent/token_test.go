package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The token never sits in agente.json as plain text on Windows (DPAPI ties it
// to this Windows user); it reads back, and an old plain file is upgraded.
func TestTokenStoredProtected(t *testing.T) {
	dir := t.TempDir()
	if err := SaveToken(dir, "ziv_segredo_do_pc"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "agente.json"))
	if runtime.GOOS == "windows" && strings.Contains(string(raw), "ziv_segredo_do_pc") {
		t.Fatalf("token in plain text: %s", raw)
	}
	if got := LoadToken(dir); got != "ziv_segredo_do_pc" {
		t.Fatalf("LoadToken = %q", got)
	}

	old := t.TempDir()
	os.WriteFile(filepath.Join(old, "agente.json"), []byte(`{"token":"ziv_antigo"}`), 0o600)
	if got := LoadToken(old); got != "ziv_antigo" {
		t.Fatalf("old file = %q", got)
	}
	raw, _ = os.ReadFile(filepath.Join(old, "agente.json"))
	if runtime.GOOS == "windows" && strings.Contains(string(raw), "ziv_antigo") {
		t.Fatalf("old plain file not upgraded: %s", raw)
	}
	if got := LoadToken(old); got != "ziv_antigo" {
		t.Fatalf("upgraded file = %q", got)
	}

	if err := SaveToken(dir, ""); err != nil || LoadToken(dir) != "" {
		t.Fatal("forgetting the token must leave none")
	}
}
