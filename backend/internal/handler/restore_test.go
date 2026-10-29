package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func TestRestoreFlow(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('quebrado')\n")
	if err := os.WriteFile(filepath.Join(dir, "src", "extra.py"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Not checked yet: the preview must record the current state first (versão 2).

	var prev struct {
		Plan    versions.RestorePlan `json:"plan"`
		Current int32                `json:"current"`
	}
	if code := c.do("GET", appPath(id, "/versions/1/restore-preview"), nil, &prev); code != http.StatusOK {
		t.Fatalf("preview %d", code)
	}
	if prev.Current != 2 || len(prev.Plan.Rewrite) != 1 || len(prev.Plan.ToTrash) != 1 || prev.Plan.ToTrash[0] != "src/extra.py" {
		t.Fatalf("%+v", prev)
	}

	var res restoreJSON
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &res); code != http.StatusOK {
		t.Fatalf("restore %d", code)
	}
	if res.Number != 3 || len(res.Failed) != 0 || len(res.Done) != 2 {
		t.Fatalf("%+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "src", "main.py"))
	if string(b) != "print('v1')\n" {
		t.Fatalf("main.py = %q", b)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, versions.TrashDir, "*", "src", "extra.py"))
	if len(matches) != 1 {
		t.Fatal("extra.py must be in the trash")
	}
	var origin string
	var from int32
	var probable bool
	if err := testPool.QueryRow(context.Background(), `SELECT origin, restored_from, author_probable FROM app_versions WHERE app_id=$1 AND number=3`, id).Scan(&origin, &from, &probable); err != nil {
		t.Fatal(err)
	}
	if origin != "restore" || from != 1 || probable {
		t.Fatalf("%s %d %v", origin, from, probable)
	}
	// Opening the ficha afterwards does not create another version (trash is excluded).
	var d appDetail
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version.Number != 3 {
		t.Fatalf("after restore %d", d.Version.Number)
	}
	if code := c.do("POST", appPath(id, "/versions/3/restore"), nil, nil); code != http.StatusBadRequest {
		t.Fatalf("restore current %d", code)
	}
}

func TestRestoreRequiresRole(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	if code := bia.do("POST", appPath(id, "/versions/1/restore"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("usuario restore %d", code)
	}
	if code := bia.do("GET", appPath(id, "/versions/1/restore-preview"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("usuario preview %d", code)
	}
}

func TestRestoreBlockedWhileRunning(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `INSERT INTO computers (person_id, name, token_hash) SELECT id, 'PC', 'h-restore' FROM people ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO runs (app_id, person_id, computer_id, output_folder, status, started_at)
		SELECT $1, c.person_id, c.id, 'C:\x', 'running', now() FROM computers c WHERE c.token_hash = 'h-restore'`, id); err != nil {
		t.Fatal(err)
	}
	var e map[string]string
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &e); code != http.StatusConflict || e["error"] != "Espere a execução terminar antes de voltar o código." {
		t.Fatalf("%d %v", code, e)
	}
	if code := c.do("GET", appPath(id, "/versions/1/restore-preview"), nil, nil); code != http.StatusConflict {
		t.Fatalf("preview while running %d", code)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "src", "main.py")); string(b) != "print('v2')\n" {
		t.Fatalf("main.py touched: %q", b)
	}
}

func TestRestoreStopsOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)
	src := filepath.Join(dir, "src")
	os.Chmod(src, 0o555) // cannot create the temp file next to main.py
	defer os.Chmod(src, 0o755)
	var res restoreJSON
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &res); code != http.StatusOK {
		t.Fatalf("restore %d", code)
	}
	if len(res.Failed) != 1 || res.Failed[0].Path != "src/main.py" || res.Number != 3 || len(res.Done) != 0 {
		t.Fatalf("%+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(src, "main.py")); string(b) != "print('v2')\n" {
		t.Fatalf("main.py = %q", b)
	}
}

func TestRestoreFolderMissing(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)
	os.RemoveAll(dir)
	var e map[string]string
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &e); code != http.StatusConflict {
		t.Fatalf("%d %v", code, e)
	}
}

// A file whose current content is saved nowhere (too big now) is listed for
// the trash in the preview and moved there before the old text comes back.
func TestRestoreTrashesUnsavedFileFirst(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	big := "#" + strings.Repeat("x", versions.MaxCodeSize) + "\n"
	editFile(t, dir, "src/main.py", big)
	var prev struct {
		Plan versions.RestorePlan `json:"plan"`
	}
	if code := c.do("GET", appPath(id, "/versions/1/restore-preview"), nil, &prev); code != http.StatusOK {
		t.Fatalf("preview %d", code)
	}
	if len(prev.Plan.Rewrite) != 1 || len(prev.Plan.ToTrash) != 1 || prev.Plan.ToTrash[0] != "src/main.py" {
		t.Fatalf("%+v", prev.Plan)
	}
	var res restoreJSON
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &res); code != http.StatusOK || len(res.Failed) != 0 {
		t.Fatalf("restore %d %+v", code, res)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "src", "main.py")); string(b) != "print('v1')\n" {
		t.Fatalf("main.py = %q", b)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, versions.TrashDir, "*", "src", "main.py"))
	if len(matches) != 1 {
		t.Fatal("the big main.py must be in the trash")
	}
	if b, _ := os.ReadFile(matches[0]); string(b) != big {
		t.Fatal("trash copy must keep the big content")
	}
}
