package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const pageHTML = "<html><body>Oi</body></html>"

func linkBody(name, url string) map[string]any {
	return map[string]any{"name": name, "summary": "Resumo", "tags": []string{"vendas", "semanal"}, "area_id": 1, "kind": "link", "url": url}
}

func fileBody(name, fileName string, content []byte) map[string]any {
	return map[string]any{"name": name, "summary": "Resumo", "tags": []string{}, "area_id": 1, "kind": "file",
		"file_name": fileName, "content_base64": base64.StdEncoding.EncodeToString(content)}
}

func createArtifact(t *testing.T, c *client, body map[string]any) artifactDetailJSON {
	t.Helper()
	var d artifactDetailJSON
	if code := c.do("POST", "/api/artifacts", body, &d); code != http.StatusCreated {
		t.Fatalf("create artifact = %d", code)
	}
	return d
}

func artifactPath(id int64, rest string) string { return fmt.Sprintf("/api/artifacts/%d%s", id, rest) }

// getContent does a GET and returns status, headers and body (for the content endpoint).
func (c *client) getContent(path string) (int, http.Header, string) {
	c.t.Helper()
	resp, err := c.c.Get(c.srv.URL + path)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func countArtifacts(t *testing.T) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM artifacts").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateLinkArtifact(t *testing.T) {
	_, ana, _, _ := team(t)
	d := createArtifact(t, ana, linkBody("Painel de vendas", "https://exemplo.com/painel"))
	if d.Kind != "link" || d.Current != 1 || len(d.Versions) != 1 || d.Versions[0].Kind != "link" || d.Versions[0].Number != 1 {
		t.Fatalf("detail %+v", d)
	}
	if d.OwnerID != 2 || d.OwnerName != "Ana" || d.Area != "Research" || strings.Join(d.Tags, ",") != "vendas,semanal" {
		t.Fatalf("detail fields %+v", d)
	}
	var list []artifactSummaryJSON
	if code := ana.do("GET", "/api/artifacts", nil, &list); code != http.StatusOK || len(list) != 1 || list[0].ID != d.ID || list[0].Version != 1 {
		t.Fatalf("list %d %+v", code, list)
	}
}

func TestCreateFileArtifactAndServeContent(t *testing.T) {
	_, ana, _, _ := team(t)
	d := createArtifact(t, ana, fileBody("Relatório", "relatorio.html", []byte(pageHTML)))
	if d.Kind != "file" || d.Versions[0].FileName != "relatorio.html" || d.Versions[0].Size != int64(len(pageHTML)) {
		t.Fatalf("detail %+v", d)
	}
	code, h, body := ana.getContent(artifactPath(d.ID, "/content"))
	if code != http.StatusOK || body != pageHTML {
		t.Fatalf("content %d %q", code, body)
	}
	csp := h.Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox allow-scripts") || strings.Contains(csp, "allow-same-origin") {
		t.Fatalf("csp %q", csp)
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(h.Get("Content-Type"), "text/html") {
		t.Fatalf("headers %v", h)
	}
}

func TestArtifactContentIsSandboxed(t *testing.T) {
	_, ana, _, _ := team(t)
	d := createArtifact(t, ana, fileBody("Relatório", "relatorio.html", []byte(pageHTML)))
	code, h, _ := ana.getContent(artifactPath(d.ID, "/content?v=1&download=1"))
	if code != http.StatusOK || !strings.Contains(h.Get("Content-Disposition"), "attachment") ||
		!strings.Contains(h.Get("Content-Disposition"), "relatorio.html") {
		t.Fatalf("download %d %v", code, h)
	}
	if csp := h.Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox allow-scripts") || strings.Contains(csp, "allow-same-origin") {
		t.Fatalf("download csp %q", csp)
	}
	l := createArtifact(t, ana, linkBody("Link", "https://exemplo.com"))
	if code, _, _ := ana.getContent(artifactPath(l.ID, "/content")); code != http.StatusNotFound {
		t.Fatalf("content of a link = %d", code)
	}
	if code, _, _ := ana.getContent(artifactPath(d.ID, "/content?v=9")); code != http.StatusNotFound {
		t.Fatalf("missing version = %d", code)
	}
}

func TestArtifactUploadLimits(t *testing.T) {
	_, ana, _, _ := team(t)
	cases := []struct {
		body map[string]any
		msg  string
	}{
		{fileBody("Grande", "grande.html", bytes.Repeat([]byte("a"), 16<<20+1)), "O arquivo passa de 16 MB."},
		{fileBody("PDF", "x.pdf", []byte(pageHTML)), "Envie um arquivo .html."},
		{fileBody("Binário", "x.html", []byte("<html>\x00</html>")), "Esse arquivo não parece uma página HTML."},
		{fileBody("Latin1", "x.html", []byte("<html>\xe9</html>")), "Esse arquivo não parece uma página HTML."},
		{linkBody("Sem https", "http://example.com"), "Cole um link que comece com https://."},
		{linkBody("Script", "javascript:alert(1)"), "Cole um link que comece com https://."},
		{linkBody("   ", "https://example.com"), "Dê um nome ao artifact."},
		{map[string]any{"name": "X", "area_id": 1, "kind": "zip"}, "Escolha link ou arquivo."},
		{map[string]any{"name": "X", "area_id": 99, "kind": "link", "url": "https://example.com"}, "Escolha uma área existente."},
	}
	for _, tc := range cases {
		var e struct {
			Error string `json:"error"`
		}
		if code := ana.do("POST", "/api/artifacts", tc.body, &e); code != http.StatusBadRequest || e.Error != tc.msg {
			t.Fatalf("%v: %d %q", tc.body["name"], code, e.Error)
		}
	}
	if n := countArtifacts(t); n != 0 {
		t.Fatalf("%d artifacts stored", n)
	}
}

func TestUserCannotCreateArtifact(t *testing.T) {
	_, _, bia, _ := team(t)
	if code := bia.do("POST", "/api/artifacts", linkBody("X", "https://example.com"), nil); code != http.StatusForbidden {
		t.Fatalf("usuário create = %d", code)
	}
}

func TestRestrictedArtifactHidden(t *testing.T) {
	admin, _, bia, _ := team(t)
	d := createArtifact(t, admin, fileBody("Segredo", "s.html", []byte(pageHTML)))
	if code := admin.do("PUT", artifactPath(d.ID, "/access"), map[string]any{"restricted": true}, nil); code != http.StatusOK {
		t.Fatalf("restrict = %d", code)
	}
	var list []artifactSummaryJSON
	bia.do("GET", "/api/artifacts", nil, &list)
	if len(list) != 0 {
		t.Fatalf("restricted listed for bia: %+v", list)
	}
	checks := []struct{ method, path string }{
		{"GET", artifactPath(d.ID, "")},
		{"GET", artifactPath(d.ID, "/content")},
		{"GET", artifactPath(d.ID, "/access")},
		{"PUT", artifactPath(d.ID, "")},
		{"PUT", artifactPath(d.ID, "/access")},
		{"PUT", artifactPath(d.ID, "/grants")},
		{"POST", artifactPath(d.ID, "/versions")},
		{"POST", artifactPath(d.ID, "/versions/1/restore")},
		{"DELETE", artifactPath(d.ID, "")},
		{"GET", artifactPath(999, "")},
	}
	for _, c := range checks {
		var e struct {
			Error string `json:"error"`
		}
		if code := bia.do(c.method, c.path, map[string]any{"name": "x", "kind": "link", "url": "https://e.com"}, &e); code != http.StatusNotFound || e.Error != "Artifact não encontrado." {
			t.Fatalf("%s %s = %d %q", c.method, c.path, code, e.Error)
		}
	}
	exec(t, "INSERT INTO area_members (area_id, person_id) VALUES (1, 3)")
	if code := admin.do("PUT", artifactPath(d.ID, "/access"), map[string]any{"restricted": true, "area_ids": []int64{1}}, nil); code != http.StatusOK {
		t.Fatalf("add viewer area = %d", code)
	}
	var got artifactDetailJSON
	if code := bia.do("GET", artifactPath(d.ID, ""), nil, &got); code != http.StatusOK || !got.Restricted || got.Can.Edit {
		t.Fatalf("viewer detail = %d %+v", code, got)
	}
}

func TestArtifactVersionsAndRestore(t *testing.T) {
	_, ana, _, _ := team(t)
	d := createArtifact(t, ana, linkBody("Painel", "https://exemplo.com"))
	var d2 artifactDetailJSON
	v2 := map[string]any{"kind": "file", "file_name": "p.html", "content_base64": base64.StdEncoding.EncodeToString([]byte(pageHTML)), "note": "Agora arquivo"}
	if code := ana.do("POST", artifactPath(d.ID, "/versions"), v2, &d2); code != http.StatusOK || d2.Current != 2 || d2.Kind != "file" {
		t.Fatalf("version 2 = %d %+v", code, d2)
	}
	var d3 artifactDetailJSON
	if code := ana.do("POST", artifactPath(d.ID, "/versions/1/restore"), nil, &d3); code != http.StatusOK {
		t.Fatalf("restore = %d", code)
	}
	if len(d3.Versions) != 3 || d3.Current != 3 {
		t.Fatalf("versions %+v", d3.Versions)
	}
	v := d3.Versions[0]
	if v.Number != 3 || v.Kind != "link" || v.RestoredFrom == nil || *v.RestoredFrom != 1 || v.URL != "https://exemplo.com" {
		t.Fatalf("version 3 %+v", v)
	}
	if d3.Versions[1].Note != "Agora arquivo" || d3.Versions[1].AuthorName != "Ana" {
		t.Fatalf("version 2 %+v", d3.Versions[1])
	}
	if code := ana.do("POST", artifactPath(d.ID, "/versions/9/restore"), nil, nil); code != http.StatusNotFound {
		t.Fatalf("restore missing = %d", code)
	}
}

func TestArtifactRestoreSwitchesKind(t *testing.T) {
	_, ana, _, _ := team(t)
	d := createArtifact(t, ana, fileBody("Página", "p.html", []byte(pageHTML)))
	kind := func() string {
		var got artifactDetailJSON
		ana.do("GET", artifactPath(d.ID, ""), nil, &got)
		return got.Kind
	}
	if k := kind(); k != "file" {
		t.Fatalf("v1 kind %s", k)
	}
	ana.do("POST", artifactPath(d.ID, "/versions"), map[string]any{"kind": "link", "url": "https://exemplo.com"}, nil)
	if k := kind(); k != "link" {
		t.Fatalf("v2 kind %s", k)
	}
	ana.do("POST", artifactPath(d.ID, "/versions/1/restore"), nil, nil)
	if k := kind(); k != "file" {
		t.Fatalf("v3 kind %s", k)
	}
	if code, _, body := ana.getContent(artifactPath(d.ID, "/content")); code != http.StatusOK || body != pageHTML {
		t.Fatalf("restored content %d %q", code, body)
	}
}

func TestAreaGrantEditsArtifact(t *testing.T) {
	admin, _, bia, _ := team(t)
	d := createArtifact(t, admin, linkBody("Painel", "https://exemplo.com"))
	ficha := map[string]any{"name": "Painel novo", "summary": "Outro", "tags": []string{" Vendas ", "vendas", ""}}
	if code := bia.do("PUT", artifactPath(d.ID, ""), ficha, nil); code != http.StatusForbidden {
		t.Fatalf("bia edit without grant = %d", code)
	}
	exec(t, "INSERT INTO area_grants (area_id, person_id, permission) VALUES (1, 3, 'edit')")
	var got artifactDetailJSON
	if code := bia.do("PUT", artifactPath(d.ID, ""), ficha, &got); code != http.StatusOK || got.Name != "Painel novo" || strings.Join(got.Tags, ",") != "vendas" {
		t.Fatalf("bia edit with grant = %d %+v", code, got)
	}
	if code := bia.do("PUT", artifactPath(d.ID, ""), map[string]any{"name": "X", "owner_id": 3}, nil); code != http.StatusForbidden {
		t.Fatalf("bia hands over = %d", code)
	}
	if code := bia.do("POST", artifactPath(d.ID, "/versions/1/restore"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("bia restore = %d", code)
	}
}

func TestArtifactGrantsAndAccess(t *testing.T) {
	admin, _, bia, _ := team(t)
	d := createArtifact(t, admin, linkBody("Painel", "https://exemplo.com"))
	if code := admin.do("PUT", artifactPath(d.ID, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"runs"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("runs grant = %d", code)
	}
	if code := admin.do("PUT", artifactPath(d.ID, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"restore"}}, nil); code != http.StatusOK {
		t.Fatalf("restore grant = %d", code)
	}
	if code := bia.do("POST", artifactPath(d.ID, "/versions/1/restore"), nil, nil); code != http.StatusOK {
		t.Fatalf("bia restore with grant = %d", code)
	}
	var acc struct {
		Grants []grantJSON `json:"grants"`
	}
	if code := bia.do("GET", artifactPath(d.ID, "/access"), nil, &acc); code != http.StatusOK || len(acc.Grants) != 1 {
		t.Fatalf("access with tech = %d %+v", code, acc)
	}
	if code := bia.do("PUT", artifactPath(d.ID, "/access"), map[string]any{"restricted": true}, nil); code != http.StatusForbidden {
		t.Fatalf("bia restricts = %d", code)
	}
}

func TestArtifactSecretsFollowCompanyPolicy(t *testing.T) {
	_, ana, _, _ := team(t)
	html := []byte(`<script>const apiKey = "sk-ant-` + strings.Repeat("a", 30) + `";</script>`)
	body := fileBody("Com chave", "k.html", html)
	var e struct {
		Code        string `json:"code"`
		SecretCount int    `json:"secret_count"`
	}
	if code := ana.do("POST", "/api/artifacts", body, &e); code != http.StatusConflict || e.Code != "artifact_secrets_confirmation_required" || e.SecretCount == 0 {
		t.Fatalf("no confirm = %d %+v", code, e)
	}
	if n := countArtifacts(t); n != 0 {
		t.Fatalf("stored before confirmation: %d", n)
	}
	body["confirm_secrets"] = true
	createArtifact(t, ana, body)
}

func TestDeleteArtifactNeedsAccess(t *testing.T) {
	admin, ana, _, _ := team(t)
	d := createArtifact(t, admin, linkBody("Painel", "https://exemplo.com"))
	if code := ana.do("DELETE", artifactPath(d.ID, ""), nil, nil); code != http.StatusForbidden {
		t.Fatalf("criador delete = %d", code)
	}
	if code := admin.do("DELETE", artifactPath(d.ID, ""), nil, nil); code != http.StatusNoContent {
		t.Fatalf("owner delete = %d", code)
	}
	if n := countArtifacts(t); n != 0 {
		t.Fatalf("%d left", n)
	}
}

// A whole-area grant saved from an artifact keeps the area's app-only
// accesses (tech, runs), which the artifact screen does not offer.
func TestArtifactWholeAreaGrantKeepsAppOnlyAccesses(t *testing.T) {
	admin, _, bia, _ := team(t)
	d := createArtifact(t, admin, linkBody("Painel", "https://exemplo.com"))
	exec(t, "INSERT INTO area_grants (area_id, person_id, permission) VALUES (1, 3, 'runs')")
	if code := admin.do("PUT", artifactPath(d.ID, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"edit"}, "whole_area": true}, nil); code != http.StatusOK {
		t.Fatalf("whole area grant = %d", code)
	}
	var perms []string
	rows, err := testPool.Query(context.Background(), "SELECT permission FROM area_grants WHERE person_id = 3 ORDER BY permission")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		perms = append(perms, p)
	}
	rows.Close()
	if strings.Join(perms, ",") != "edit,runs" {
		t.Fatalf("area grants %v", perms)
	}
	if code := bia.do("PUT", artifactPath(d.ID, ""), map[string]any{"name": "Novo"}, nil); code != http.StatusOK {
		t.Fatalf("bia edit = %d", code)
	}
}

// The artifact's access screen shows only the area-wide accesses that reach
// artifacts (edit, restore); people with only tech/runs are left out.
func TestArtifactAccessShowsOnlyArtifactAreaGrants(t *testing.T) {
	admin, _, _, _ := team(t)
	d := createArtifact(t, admin, linkBody("Painel", "https://exemplo.com"))
	exec(t, "INSERT INTO area_grants (area_id, person_id, permission) VALUES (1, 3, 'runs'), (1, 3, 'tech'), (1, 3, 'edit'), (1, 2, 'runs')")
	var acc struct {
		AreaGrants []grantJSON `json:"area_grants"`
	}
	if code := admin.do("GET", artifactPath(d.ID, "/access"), nil, &acc); code != http.StatusOK {
		t.Fatalf("access = %d", code)
	}
	if len(acc.AreaGrants) != 1 || acc.AreaGrants[0].PersonID != 3 || strings.Join(acc.AreaGrants[0].Permissions, ",") != "edit" {
		t.Fatalf("area grants %+v", acc.AreaGrants)
	}
}

// Uploads that arrive at the same time each get their own version number
// (the artifact row is locked) instead of one failing with 500.
func TestArtifactConcurrentVersions(t *testing.T) {
	_, ana, _, _ := team(t)
	d := createArtifact(t, ana, linkBody("Painel", "https://exemplo.com"))
	const n = 6
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"kind":"link","url":"https://exemplo.com/%d"}`, i)
			resp, err := ana.c.Post(ana.srv.URL+artifactPath(d.ID, "/versions"), "application/json", strings.NewReader(body))
			if err != nil {
				codes <- 0
				return
			}
			resp.Body.Close()
			codes <- resp.StatusCode
		}(i)
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != http.StatusOK {
			t.Fatalf("concurrent version = %d", c)
		}
	}
	var got artifactDetailJSON
	if code := ana.do("GET", artifactPath(d.ID, ""), nil, &got); code != http.StatusOK || got.Current != n+1 || len(got.Versions) != n+1 {
		t.Fatalf("after concurrent uploads = %d current %d versions %d", code, got.Current, len(got.Versions))
	}
}
