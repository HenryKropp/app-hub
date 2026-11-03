package agentproto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHubURLTrailer(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.exe")
	if err := os.WriteFile(plain, []byte("MZ program bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadHubURLTrailer(plain); err != nil || got != "" {
		t.Fatalf("plain EXE trailer = %q, %v", got, err)
	}
	tagged := filepath.Join(dir, "tagged.exe")
	want := "https://vibehub.replit.app/w/horizonte"
	if err := os.WriteFile(tagged, append([]byte("MZ program bytes"), HubURLTrailer(want)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadHubURLTrailer(tagged); err != nil || got != want {
		t.Fatalf("tagged EXE trailer = %q, %v", got, err)
	}
	short := filepath.Join(dir, "short.exe")
	if err := os.WriteFile(short, []byte("MZ"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadHubURLTrailer(short); err != nil || got != "" {
		t.Fatalf("short file trailer = %q, %v", got, err)
	}
}
