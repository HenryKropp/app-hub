package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// gerar makes the key file (owner-only) and prints the public half; publica
// prints it again; assinar writes <exe>.sig that the Agente accepts.
func TestGenerateSignVerify(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "agente-assinatura.key")
	pub, err := generate(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generate(keyFile); err == nil {
		t.Fatal("generate must not overwrite an existing key")
	}
	if again, err := publicOf(keyFile); err != nil || again != pub {
		t.Fatalf("publica = %q, %v; want %q", again, err, pub)
	}
	exe := filepath.Join(dir, "AppHubAgente.exe")
	os.WriteFile(exe, []byte("MZ-programa"), 0o644)
	if err := sign(keyFile, exe); err != nil {
		t.Fatal(err)
	}
	sig, _ := os.ReadFile(exe + ".sig")
	if !agentproto.VerifyUpdate(pub, []byte("MZ-programa"), strings.TrimSpace(string(sig))) {
		t.Fatal("signature not accepted")
	}
}
