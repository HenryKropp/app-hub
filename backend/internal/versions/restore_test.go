package versions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanRestore(t *testing.T) {
	target := Snapshot{
		{Path: "a.py", SHA256: "1", Kind: KindCode},
		{Path: "app.exe", SHA256: "old", Kind: KindBinary},
		{Path: "gone.py", SHA256: "2", Kind: KindCode},
		{Path: "same.py", SHA256: "3", Kind: KindCode},
	}
	current := Snapshot{
		{Path: "a.py", SHA256: "1b", Kind: KindCode},
		{Path: "app.exe", SHA256: "new", Kind: KindBinary},
		{Path: "extra.py", SHA256: "4", Kind: KindCode},
		{Path: "same.py", SHA256: "3", Kind: KindCode},
	}
	p := PlanRestore(current, target)
	if len(p.Rewrite) != 1 || p.Rewrite[0] != "a.py" || len(p.Recreate) != 1 || p.Recreate[0] != "gone.py" ||
		len(p.ToTrash) != 1 || p.ToTrash[0] != "extra.py" || len(p.Binaries) != 1 || p.Binaries[0] != "app.exe" {
		t.Fatalf("%+v", p)
	}
	if e := PlanRestore(target, target); len(e.Rewrite)+len(e.Recreate)+len(e.ToTrash)+len(e.Binaries) != 0 || e.Rewrite == nil {
		t.Fatalf("same snapshot plan %+v", e)
	}
}

func TestApplyRestore(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/a.py", "v1\n")
	write(t, root, "src/b.py", "b\n")
	f := LocalFolder{Root: root}
	v1, contents, err := Scan(f, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Change a.py, remove b.py, add c.py.
	write(t, root, "src/a.py", "v2 quebrado\n")
	os.Remove(filepath.Join(root, "src", "b.py"))
	write(t, root, "src/c.py", "novo\n")
	cur, _, _ := Scan(f, nil, v1)

	plan := PlanRestore(cur, v1)
	trash := TrashFolder(time.Date(2026, 10, 1, 15, 30, 0, 0, time.Local))
	if trash != ".app-hub-lixeira/2026-10-01 153000" {
		t.Fatalf("trash %q", trash)
	}
	blob := func(sha string) ([]byte, error) { return contents[sha], nil }
	done, failed := ApplyRestore(f, plan, v1, blob, trash)
	if len(failed) != 0 || len(done) != 3 {
		t.Fatalf("done %v failed %v", done, failed)
	}
	read := func(rel string) string {
		b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return string(b)
	}
	if read("src/a.py") != "v1\n" || read("src/b.py") != "b\n" || read(trash+"/src/c.py") != "novo\n" {
		t.Fatal("files not restored")
	}
	if _, err := os.Stat(filepath.Join(root, "src", "c.py")); !os.IsNotExist(err) {
		t.Fatal("extra file must leave the code folder")
	}
	after, _, _ := Scan(f, nil, nil)
	if !Same(after, v1) {
		t.Fatal("folder must equal the target version (trash is excluded)")
	}
}

func TestApplyRestoreStopsOnFirstFailure(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.py", "1\n")
	write(t, root, "b.py", "2\n")
	f := LocalFolder{Root: root}
	target, _, _ := Scan(f, nil, nil)
	plan := RestorePlan{Rewrite: []string{"a.py", "b.py"}, Recreate: []string{}, ToTrash: []string{}, Binaries: []string{}}
	blob := func(sha string) ([]byte, error) { return nil, errors.New("sem cópia") }
	done, failed := ApplyRestore(f, plan, target, blob, TrashFolder(time.Now()))
	if len(done) != 0 || len(failed) != 2 || failed[0].Path != "a.py" || failed[1].Error != "não foi tentado" {
		t.Fatalf("done %v failed %+v", done, failed)
	}
}

func TestApplyRestoreWriteFailureLeavesOtherFilesAlone(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ro/a.py", "1\n")
	write(t, root, "b.py", "2\n")
	f := LocalFolder{Root: root}
	target, contents, _ := Scan(f, nil, nil)
	write(t, root, "ro/a.py", "mudou\n")
	write(t, root, "b.py", "mudou\n")
	if err := os.Chmod(filepath.Join(root, "ro"), 0o555); err != nil {
		t.Skip(err)
	}
	defer os.Chmod(filepath.Join(root, "ro"), 0o755)
	cur, _, _ := Scan(f, nil, target)
	blob := func(sha string) ([]byte, error) { return contents[sha], nil }
	done, failed := ApplyRestore(f, PlanRestore(cur, target), target, blob, TrashFolder(time.Now()))
	if len(done) != 1 || done[0] != "b.py" || len(failed) != 1 || failed[0].Path != "ro/a.py" {
		t.Fatalf("done %v failed %+v", done, failed)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "ro", "a.py")); string(b) != "mudou\n" {
		t.Fatal("failed file must be untouched")
	}
	if _, err := os.Stat(filepath.Join(root, "ro", "a.py"+tempSuffix)); !os.IsNotExist(err) {
		t.Fatal("no temp file may be left behind")
	}
}
