package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotDiff(t *testing.T) {
	root := filepath.Join(t.TempDir(), "8. Programação")
	write(t, root, "PETROBRAS/FRE/antigo.pdf", "a")
	write(t, root, "muda.txt", "1")
	write(t, root, "Ferramenta CVM.exe", "exe")
	before, err := TakeSnapshot(root, "Ferramenta CVM.exe")
	if err != nil {
		t.Fatal(err)
	}

	write(t, root, "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", "novo")
	write(t, root, "muda.txt", "22")
	later := time.Now().Add(time.Minute)
	_ = os.Chtimes(filepath.Join(root, "muda.txt"), later, later)
	write(t, root, "Ferramenta CVM.exe", "exe mudou") // skipped
	after, err := TakeSnapshot(root, "Ferramenta CVM.exe")
	if err != nil {
		t.Fatal(err)
	}

	got := Diff(before, after)
	if len(got) != 2 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	if got[0].Path != "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md" || got[0].Change != "new" || got[0].Size != 4 {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Path != "muda.txt" || got[1].Change != "changed" {
		t.Errorf("second = %+v", got[1])
	}
}

func TestSnapshotMissingFolder(t *testing.T) {
	if _, err := TakeSnapshot(filepath.Join(t.TempDir(), "nao-existe")); err == nil {
		t.Fatal("want error")
	}
}
