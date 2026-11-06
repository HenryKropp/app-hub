package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

const serverSecret = "ZIV_SEGREDO_123"

// secretDir is a server folder that is not a company's: it holds a secret
// and a main.py so it looks like code.
func secretDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"segredo.txt": serverSecret, "main.py": "print('oi')\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// appAtFolder inserts an app row pointing at folder, as a company could get
// one saved when its PC path happens to exist on the server.
func appAtFolder(t *testing.T, slug, folder string) int64 {
	t.Helper()
	m := manifest.Manifest{ManifestVersion: 1, ID: slug, Name: slug, Kind: manifest.KindScript, Entrypoint: "main.py", Area: "Research"}
	raw, _ := json.Marshal(m)
	var id int64
	if err := testPool.QueryRow(context.Background(), `INSERT INTO apps (slug, name, kind, area, area_id, folder_path, owner_id, manifest, created_by)
		VALUES ($1, $1, 'script', 'Research', 1, $2, 1, $3, 1) RETURNING id`, slug, folder, raw).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func versionCount(t *testing.T, id int64) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM app_versions WHERE app_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func getText(t *testing.T, c *client, path string) (int, string) {
	t.Helper()
	res := c.raw("GET", path, nil)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func companyClient(t *testing.T) *client {
	t.Helper()
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	return c
}

func wantNotOnServer(t *testing.T, c *client, method, path string, body any) {
	t.Helper()
	var e map[string]any
	if code := c.do(method, path, body, &e); code != http.StatusConflict || e["code"] != "folder_not_on_server" {
		t.Fatalf("%s %s = %d %+v", method, path, code, e)
	}
}

func TestCompanyCannotReadServerFolderThroughVersions(t *testing.T) {
	c := companyClient(t)
	dir := secretDir(t)
	m := manifest.Manifest{Name: "Espiao", Summary: "x", Kind: manifest.KindScript, Language: "python", Entrypoint: "main.py", Area: "Research"}
	wantNotOnServer(t, c, "POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m})
	wantNotOnServer(t, c, "POST", "/api/scan", map[string]string{"path": dir, "mode": "single"})
	if _, err := os.Stat(filepath.Join(dir, manifest.FileName)); err == nil {
		t.Fatal("manifest written into a server folder")
	}

	id := appAtFolder(t, "espiao", dir)
	var d appDetail
	if code := c.do("POST", appPath(id, "/check"), nil, &d); code != http.StatusOK || d.FolderReadable || d.Version != nil {
		t.Fatalf("check %d %+v", code, d)
	}
	if n := versionCount(t, id); n != 0 {
		t.Fatalf("versions = %d", n)
	}
	code, raw := getText(t, c, appPath(id, "/versions/1/content?path=segredo.txt"))
	if (code != http.StatusNotFound && code != http.StatusConflict) || strings.Contains(raw, serverSecret) {
		t.Fatalf("content %d %s", code, raw)
	}
}

func TestCompanyCannotRestoreIntoServerFolder(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c) // versão 1, recorded in principal
	editFile(t, dir, "src/main.py", "print('v2')\n")
	var d appDetail
	c.do("POST", appPath(id, "/check"), nil, &d)
	if d.Version == nil || d.Version.Number != 2 {
		t.Fatalf("setup %+v", d.Version)
	}
	before, _ := os.ReadDir(dir)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	wantNotOnServer(t, c, "POST", appPath(id, "/versions/1/restore"), map[string]any{})
	if got, _ := os.ReadFile(filepath.Join(dir, "src", "main.py")); string(got) != "print('v2')\n" {
		t.Fatalf("folder changed: %q", got)
	}
	if after, _ := os.ReadDir(dir); len(after) != len(before) {
		t.Fatalf("folder entries %d → %d", len(before), len(after))
	}
}

func TestCompanySymlinkIntoImportsRefused(t *testing.T) {
	c := companyClient(t)
	dir := secretDir(t)
	imports := filepath.Join(os.TempDir(), "apphub-imports")
	if err := os.MkdirAll(imports, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(imports, strings.Repeat("a", 43))
	_ = os.Remove(link)
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	id := appAtFolder(t, "atalho", link)
	c.do("POST", appPath(id, "/check"), nil, nil)
	if n := versionCount(t, id); n != 0 {
		t.Fatalf("versions = %d", n)
	}
}

func TestCompanyImportedScriptStillVersions(t *testing.T) {
	c := companyClient(t)
	id := importTestScript(t, c)
	var d appDetail
	if code := c.do("POST", appPath(id, "/check"), nil, &d); code != http.StatusOK || d.Version == nil || d.Version.Number != 1 {
		t.Fatalf("check %d %+v", code, d.Version)
	}
	if code, raw := getText(t, c, appPath(id, "/versions/1/content?path=main.py")); code != http.StatusOK || !bytes.Contains([]byte(raw), []byte("print('ok')")) {
		t.Fatalf("content %d %s", code, raw)
	}
}

func TestPrincipalFolderUnchanged(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	dir := secretDir(t)
	m := manifest.Manifest{Name: "Do servidor", Summary: "x", Kind: manifest.KindScript, Language: "python", Entrypoint: "main.py", Area: "Research"}
	var d appDetail
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &d); code != http.StatusCreated {
		t.Fatalf("publish = %d", code)
	}
	if n := versionCount(t, d.ID); n != 1 {
		t.Fatalf("versions = %d", n)
	}
}
