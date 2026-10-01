package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// codeApp creates a real folder with code and registers it through the API,
// so versão 1 and "Ficha criada" are recorded like in production.
func codeApp(t *testing.T, c *client) (int64, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.py"), []byte("print('v1')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := manifest.Manifest{Name: "App Teste", Summary: "Teste", Kind: manifest.KindScript, Language: "python", Entrypoint: "src/main.py", Area: "Research"}
	var d appDetail
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &d); code != http.StatusCreated {
		t.Fatalf("publish = %d", code)
	}
	return d.ID, dir
}

func editFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute) // make sure size or mtime differ
	_ = os.Chtimes(p, later, later)
}

func TestVersionOneOnPublishAndNewVersionOnChange(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)

	var d appDetail
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version == nil || d.Version.Number != 1 || !d.FolderReadable {
		t.Fatalf("detail %+v", d)
	}
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("POST", appPath(id, "/check"), nil, &d)
	if d.Version.Number != 2 {
		t.Fatalf("after change %+v", d.Version)
	}
	c.do("POST", appPath(id, "/check"), nil, &d)
	if d.Version.Number != 2 {
		t.Fatal("no change must not create a version")
	}
	var origin string
	var probable bool
	var author int64
	testPool.QueryRow(context.Background(), `SELECT origin, author_probable, author_id FROM app_versions WHERE app_id=$1 AND number=2`, id).Scan(&origin, &probable, &author)
	if origin != "folder" || !probable || author != 1 {
		t.Fatalf("%s %v %d", origin, probable, author)
	}
	var raw []byte
	testPool.QueryRow(context.Background(), `SELECT summary FROM app_versions WHERE app_id=$1 AND number=2`, id).Scan(&raw)
	var summary versions.Summary
	if err := json.Unmarshal(raw, &summary); err != nil || summary.Modified != 1 || summary.Added != 0 || len(summary.Binaries) != 0 {
		t.Fatalf("summary %s", raw)
	}
}

func TestCheckVersionIgnoresTouch(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	later := time.Now().Add(time.Hour).Truncate(time.Second)
	_ = os.Chtimes(filepath.Join(dir, "src", "main.py"), later, later)
	var d appDetail
	c.do("POST", appPath(id, "/check"), nil, &d)
	if d.Version.Number != 1 {
		t.Fatalf("touch created version %d", d.Version.Number)
	}
	// The scan cache remembers the new mtime, so the next check does not re-read the file.
	h.scansMu.Lock()
	sc := h.scans[id]
	h.scansMu.Unlock()
	if sc.number != 1 || len(sc.snap) != 1 || !sc.snap[0].Modified.Equal(later) {
		t.Fatalf("cache %+v, want mtime %v", sc, later)
	}
}

func TestConcurrentChecksCreateOneVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.do("POST", appPath(id, "/check"), nil, nil) }()
	}
	wg.Wait()
	var n int
	testPool.QueryRow(context.Background(), `SELECT count(*) FROM app_versions WHERE app_id=$1`, id).Scan(&n)
	if n != 2 {
		t.Fatalf("versions = %d", n)
	}
}

func TestGetAppWithMissingFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest()) // folder /mnt/c/apps/files-prep does not exist
	var d appDetail
	if code := c.do("GET", appPath(app, ""), nil, &d); code != http.StatusOK || d.Version != nil {
		t.Fatalf("ficha must open: %d %+v", code, d)
	}
	if code := c.do("POST", appPath(app, "/check"), nil, &d); code != http.StatusOK || d.FolderReadable || d.Version != nil {
		t.Fatalf("check %d %+v", code, d)
	}
}

func TestRunLinksToVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Ferramenta CVM.exe"), []byte("MZ"), 0o644)
	m := filesPrepManifest()
	m.Area = "Research"
	var d appDetail
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &d); code != http.StatusCreated {
		t.Fatalf("publish %d", code)
	}
	pairComputer(t, c, "PC")
	os.WriteFile(filepath.Join(dir, "Ferramenta CVM.exe"), []byte("MZ2"), 0o644)
	var run runJSON
	if code := c.do("POST", appPath(d.ID, "/runs"), map[string]string{"output_folder": `D:\Saídas`}, &run); code != http.StatusCreated {
		t.Fatalf("run %d", code)
	}
	if run.VersionNumber == nil || *run.VersionNumber != 2 {
		t.Fatalf("version %v", run.VersionNumber)
	}
}

func TestFichaChangesRecorded(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	m := manifest.Manifest{Name: "App Teste", Summary: "Teste", Kind: manifest.KindScript, Language: "python", Entrypoint: "src/main.py", Area: "Equity"}
	c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, nil)
	c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, nil) // no change
	rows, _ := testPool.Query(context.Background(), `SELECT changes FROM app_changes WHERE app_id=$1 ORDER BY id`, id)
	var all []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		all = append(all, s)
	}
	if len(all) != 2 {
		t.Fatalf("changes %v", all)
	}
	var first, second []manifest.Change
	json.Unmarshal([]byte(all[0]), &first)
	json.Unmarshal([]byte(all[1]), &second)
	if first[0].Field != "created" || len(second) != 1 || second[0].Field != "area" || second[0].After != "Equity" {
		t.Fatalf("%+v %+v", first, second)
	}
}

// A folder the Hub cannot read gives no confirmed version: the run has none.
func TestRunWithUnreadableFolderHasNoVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Ferramenta CVM.exe"), []byte("MZ"), 0o644)
	m := filesPrepManifest()
	m.Area = "Research"
	var d appDetail
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &d); code != http.StatusCreated {
		t.Fatalf("publish %d", code)
	}
	pairComputer(t, c, "PC")
	os.RemoveAll(dir)
	var run runJSON
	if code := c.do("POST", appPath(d.ID, "/runs"), map[string]string{"output_folder": `D:\Saídas`}, &run); code != http.StatusCreated {
		t.Fatalf("run %d", code)
	}
	if run.VersionNumber != nil {
		t.Fatalf("run must not claim version %d", *run.VersionNumber)
	}
}

func TestGetAppDoesNotCheckFolderButCheckDoes(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c) // publicar registra a versão 1
	editFile(t, dir, "src/main.py", "print('v2')\n")

	var d appDetail
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version == nil || d.Version.Number != 1 {
		t.Fatalf("GET não deve conferir a pasta: %+v", d.Version)
	}
	var chk struct {
		FolderReadable bool        `json:"folder_readable"`
		Version        *versionRef `json:"version"`
	}
	if code := c.do("POST", appPath(id, "/check"), nil, &chk); code != http.StatusOK || !chk.FolderReadable || chk.Version == nil || chk.Version.Number != 2 {
		t.Fatalf("check %d %+v", code, chk)
	}
}

func TestCheckWithMissingFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	_ = os.RemoveAll(dir)
	var chk struct {
		FolderReadable bool `json:"folder_readable"`
	}
	if code := c.do("POST", appPath(id, "/check"), nil, &chk); code != http.StatusOK || chk.FolderReadable {
		t.Fatalf("check %d %+v", code, chk)
	}
}
