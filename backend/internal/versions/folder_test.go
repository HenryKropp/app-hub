package versions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFolderRefusesEscapes(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "app")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := LocalFolder{Root: root}
	for _, bad := range []string{"../x", "a/../../x", "/etc/passwd"} {
		if err := f.Write(bad, []byte("y")); err == nil {
			t.Errorf("Write(%q) should fail", bad)
		}
		if err := f.Move("a.py", bad); err == nil {
			t.Errorf("Move to %q should fail", bad)
		}
		if err := f.Move(bad, "b.py"); err == nil {
			t.Errorf("Move from %q should fail", bad)
		}
		if _, err := f.Open(bad); err == nil {
			t.Errorf("Open(%q) should fail", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "x")); err == nil {
		t.Fatal("file escaped the root")
	}
	if err := f.Write("sub/ok.py", []byte("y")); err != nil {
		t.Fatal(err)
	}
}

func TestLocalFolderExists(t *testing.T) {
	root := t.TempDir()
	f := LocalFolder{Root: root}
	if err := os.WriteFile(filepath.Join(root, "a.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "link.py")); err != nil {
		t.Skip(err)
	}
	for rel, want := range map[string]bool{"a.py": true, "link.py": true, "b.py": false, "sub/c.py": false} {
		got, err := f.Exists(rel)
		if err != nil || got != want {
			t.Errorf("Exists(%q) = %v, %v", rel, got, err)
		}
	}
	if _, err := f.Exists("../x"); err == nil {
		t.Error("Exists must refuse paths outside the folder")
	}
}
