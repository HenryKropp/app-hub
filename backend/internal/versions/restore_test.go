package versions

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
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

func readRel(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A file that is binary now (too big) has no copy in the Hub: it goes to the
// trash before the target text is written over it.
func TestRestoreTrashesFileThatIsBinaryNow(t *testing.T) {
	root := t.TempDir()
	write(t, root, "dados.json", "{\"v\":1}\n")
	f := LocalFolder{Root: root}
	v1, contents, err := Scan(f, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", MaxCodeSize+10)
	write(t, root, "dados.json", big)
	cur, _, _ := Scan(f, nil, v1)
	if cur[0].Kind != KindBinary {
		t.Fatalf("setup: %+v", cur[0])
	}
	plan, err := PlanRestoreIn(f, cur, v1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rewrite) != 1 || len(plan.ToTrash) != 1 || plan.ToTrash[0] != "dados.json" {
		t.Fatalf("plan %+v", plan)
	}
	trash := TrashFolder(time.Now())
	done, failed := ApplyRestore(f, plan, v1, func(sha string) ([]byte, error) { return contents[sha], nil }, trash)
	if len(failed) != 0 || len(done) != 1 {
		t.Fatalf("done %v failed %v", done, failed)
	}
	if readRel(t, root, "dados.json") != "{\"v\":1}\n" || readRel(t, root, trash+"/dados.json") != big {
		t.Fatal("big content must be in the trash and the target text in place")
	}
}

// A file excluded from the scan (now an output) but still on disk is moved
// to the trash before it comes back; a symlink too.
func TestRestoreTrashesFileOnDiskButNotInSnapshot(t *testing.T) {
	root := t.TempDir()
	write(t, root, "config/report.json", "{\"a\":1}\n")
	write(t, root, "link.py", "print(1)\n")
	write(t, root, "elsewhere.txt", "alvo\n")
	f := LocalFolder{Root: root}
	v1, contents, err := Scan(f, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "config/report.json", "{\"gerado\":true}\n")
	os.Remove(filepath.Join(root, "link.py"))
	if err := os.Symlink("elsewhere.txt", filepath.Join(root, "link.py")); err != nil {
		t.Skip(err)
	}
	outs := []manifest.Output{{Label: "Relatórios", Pattern: "*.json"}}
	cur, _, err := Scan(f, outs, v1)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRestoreIn(f, cur, v1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Recreate) != 2 {
		t.Fatalf("plan %+v", plan)
	}
	inTrash := map[string]bool{}
	for _, p := range plan.ToTrash {
		inTrash[p] = true
	}
	if !inTrash["config/report.json"] || !inTrash["link.py"] {
		t.Fatalf("preview must list them in to_trash: %+v", plan)
	}
	trash := TrashFolder(time.Now())
	done, failed := ApplyRestore(f, plan, v1, func(sha string) ([]byte, error) { return contents[sha], nil }, trash)
	if len(failed) != 0 {
		t.Fatalf("done %v failed %v", done, failed)
	}
	if readRel(t, root, "config/report.json") != "{\"a\":1}\n" || readRel(t, root, trash+"/config/report.json") != "{\"gerado\":true}\n" {
		t.Fatal("excluded file must be trashed, then restored")
	}
	if fi, err := os.Lstat(filepath.Join(root, "link.py")); err != nil || fi.Mode()&os.ModeSymlink != 0 || readRel(t, root, "link.py") != "print(1)\n" {
		t.Fatal("link.py must be the restored regular file")
	}
	if l, err := os.Readlink(filepath.Join(root, filepath.FromSlash(trash), "link.py")); err != nil || l != "elsewhere.txt" {
		t.Fatalf("the symlink must be in the trash: %q %v", l, err)
	}
	if readRel(t, root, "elsewhere.txt") != "alvo\n" {
		t.Fatal("link target untouched")
	}
}

// A code file that changed is already saved in the safety version: no trash.
func TestRestoreCodeRewriteDoesNotTrash(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.py", "v1\n")
	f := LocalFolder{Root: root}
	v1, contents, _ := Scan(f, nil, nil)
	write(t, root, "a.py", "v2\n")
	cur, _, _ := Scan(f, nil, v1)
	plan, err := PlanRestoreIn(f, cur, v1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rewrite) != 1 || len(plan.ToTrash) != 0 {
		t.Fatalf("plan %+v", plan)
	}
	trash := TrashFolder(time.Now())
	if _, failed := ApplyRestore(f, plan, v1, func(sha string) ([]byte, error) { return contents[sha], nil }, trash); len(failed) != 0 {
		t.Fatal(failed)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(trash))); !os.IsNotExist(err) {
		t.Fatal("no trash folder for a plain code rewrite")
	}
}

// If moving the unsaved file to the trash fails, it is not overwritten.
func TestRestoreMoveFailureKeepsUnsavedFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ro/dados.json", "{}\n")
	f := LocalFolder{Root: root}
	v1, contents, _ := Scan(f, nil, nil)
	big := strings.Repeat("y", MaxCodeSize+1)
	write(t, root, "ro/dados.json", big)
	cur, _, _ := Scan(f, nil, v1)
	plan, err := PlanRestoreIn(f, cur, v1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "ro"), 0o555); err != nil {
		t.Skip(err)
	}
	defer os.Chmod(filepath.Join(root, "ro"), 0o755)
	done, failed := ApplyRestore(f, plan, v1, func(sha string) ([]byte, error) { return contents[sha], nil }, TrashFolder(time.Now()))
	if len(done) != 0 || len(failed) != 1 || failed[0].Path != "ro/dados.json" {
		t.Fatalf("done %v failed %+v", done, failed)
	}
	if readRel(t, root, "ro/dados.json") != big {
		t.Fatal("file must stay as it was")
	}
}
