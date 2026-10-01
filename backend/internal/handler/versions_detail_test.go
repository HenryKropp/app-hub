package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionChanges(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "API_KEY = \"abc123def456ghi\"\nprint('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)

	var res struct {
		Version versionItem      `json:"version"`
		Files   []fileChangeJSON `json:"files"`
	}
	if code := c.do("GET", appPath(id, "/versions/2/changes"), nil, &res); code != http.StatusOK {
		t.Fatalf("%d", code)
	}
	if res.Version.Number != 2 || len(res.Files) != 1 || res.Files[0].Status != "modified" || res.Files[0].Kind != "code" {
		t.Fatalf("%+v", res)
	}
	var text strings.Builder
	for _, hk := range res.Files[0].Hunks {
		for _, l := range hk.Lines {
			text.WriteString(l.Op + l.Text + "\n")
		}
	}
	if text.String() != "-print('v1')\n+API_KEY = \"••••\"\n+print('v2')\n" {
		t.Fatalf("diff\n%s", text.String())
	}
	var v1 struct {
		Files []fileChangeJSON `json:"files"`
	}
	c.do("GET", appPath(id, "/versions/1/changes"), nil, &v1)
	if len(v1.Files) != 1 || v1.Files[0].Status != "added" {
		t.Fatalf("v1 %+v", v1.Files)
	}
	if code := c.do("GET", appPath(id, "/versions/9/changes"), nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing version %d", code)
	}
}

func TestVersionFileContent(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "token = \"abcdefghijklmnop\"\n")
	c.do("GET", appPath(id, ""), nil, nil)

	var files []versionFileJSON
	c.do("GET", appPath(id, "/versions/1/files"), nil, &files)
	if len(files) != 1 || files[0].Path != "src/main.py" || files[0].Kind != "code" {
		t.Fatalf("%+v", files)
	}
	var got struct{ Path, Text string }
	c.do("GET", appPath(id, "/versions/1/content?path="+url.QueryEscape("src/main.py")), nil, &got)
	if got.Text != "print('v1')\n" {
		t.Fatalf("v1 content %q", got.Text)
	}
	c.do("GET", appPath(id, "/versions/2/content?path="+url.QueryEscape("src/main.py")), nil, &got)
	if got.Text != "token = \"••••\"\n" {
		t.Fatalf("masked %q", got.Text)
	}
	for _, bad := range []string{"../etc/passwd", "/etc/passwd", "nao-existe.py"} {
		if code := c.do("GET", appPath(id, "/versions/1/content?path="+url.QueryEscape(bad)), nil, nil); code != http.StatusNotFound {
			t.Fatalf("%s = %d", bad, code)
		}
	}
}

func TestDescribeVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, _ := codeApp(t, c)
	createPersonAs(t, c, "Ana", "ana@alis.com", "criador")
	const anaID = 2 // setupAdmin is person 1 (RESTART IDENTITY)
	var v versionItem
	body := map[string]any{"description": "  Primeira versão  ", "author_id": anaID}
	if code := c.do("PATCH", appPath(id, "/versions/1"), body, &v); code != http.StatusOK {
		t.Fatalf("%d", code)
	}
	if v.Description != "Primeira versão" || v.AuthorName != "Ana" || v.AuthorProbable {
		t.Fatalf("%+v", v)
	}
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	if code := bia.do("PATCH", appPath(id, "/versions/1"), body, nil); code != http.StatusForbidden {
		t.Fatalf("usuario describe %d", code)
	}
	if code := c.do("PATCH", appPath(id, "/versions/1"), map[string]any{"description": strings.Repeat("x", 2001)}, nil); code != http.StatusBadRequest {
		t.Fatalf("too long %d", code)
	}
}

func TestVersionContentMasksEnvFile(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("HOST=db.local\nDB_PASSWORD=Sup3rSecret99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.do("GET", appPath(id, ""), nil, nil)
	var got struct{ Path, Text string }
	if code := c.do("GET", appPath(id, "/versions/2/content?path="+url.QueryEscape(".env")), nil, &got); code != http.StatusOK {
		t.Fatalf("content %d", code)
	}
	if got.Text != "HOST=••••\nDB_PASSWORD=••••\n" {
		t.Fatalf("masked %q", got.Text)
	}
	var ch json.RawMessage
	if code := c.do("GET", appPath(id, "/versions/2/changes"), nil, &ch); code != http.StatusOK {
		t.Fatalf("changes %d", code)
	}
	if b := string(ch); !strings.Contains(b, ".env") || strings.Contains(b, "Sup3rSecret99") || strings.Contains(b, "db.local") {
		t.Fatalf("changes must list .env without its values: %s", b)
	}
}
