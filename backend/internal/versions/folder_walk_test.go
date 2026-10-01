package versions

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type walked struct {
	Rel  string
	Dir  bool
	Size int64
}

func TestWalkReportsTree(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("main.py", "hello")
	mk("pkg/a.py", "ab")
	mk("pkg/deep/b.txt", "abc")
	mk("junk/skip.bin", "zzzz")
	if err := os.Symlink(filepath.Join(root, "main.py"), filepath.Join(root, "link_file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "pkg"), filepath.Join(root, "link_dir")); err != nil {
		t.Fatal(err)
	}

	var got []walked
	var mod time.Time
	err := LocalFolder{Root: root}.Walk(func(rel string, dir bool, size int64, m time.Time) error {
		if dir && rel == "junk" {
			got = append(got, walked{rel, true, 0})
			return fs.SkipDir
		}
		if rel == "main.py" {
			mod = m
		}
		if dir && !m.IsZero() {
			t.Errorf("dir %s has mtime", rel)
		}
		got = append(got, walked{rel, dir, size})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []walked{
		{"junk", true, 0},
		{"main.py", false, 5},
		{"pkg", true, 0},
		{"pkg/a.py", false, 2},
		{"pkg/deep", true, 0},
		{"pkg/deep/b.txt", false, 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	st, err := os.Lstat(filepath.Join(root, "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !mod.Equal(st.ModTime()) {
		t.Errorf("mtime %v != lstat %v", mod, st.ModTime())
	}
}

func TestWalkReturnsFnError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := LocalFolder{Root: root}.Walk(func(string, bool, int64, time.Time) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("got %v", err)
	}
}

func TestWalkUnreadableDirIsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	root := t.TempDir()
	bad := filepath.Join(root, "locked")
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o755) })
	err := LocalFolder{Root: root}.Walk(func(string, bool, int64, time.Time) error { return nil })
	if err == nil {
		t.Fatal("expected error for unreadable directory")
	}
}

func TestWalkMissingRoot(t *testing.T) {
	err := LocalFolder{Root: filepath.Join(t.TempDir(), "nope")}.Walk(func(string, bool, int64, time.Time) error { return nil })
	if !errors.Is(err, ErrFolderMissing) {
		t.Errorf("got %v", err)
	}
}
