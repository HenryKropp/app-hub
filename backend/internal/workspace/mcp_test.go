package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

func TestSplitWorkspacePath(t *testing.T) {
	cases := []struct{ in, slug, rest string }{
		{"/mcp", "principal", "/mcp"},
		{"/w/teste/mcp", "teste", "/mcp"},
		{"/w/teste", "teste", "/"},
		{"/w/teste/oauth/token", "teste", "/oauth/token"},
		{"/.well-known/oauth-protected-resource/mcp", "principal", "/.well-known/oauth-protected-resource/mcp"},
		{"/.well-known/oauth-protected-resource/w/teste/mcp", "teste", "/.well-known/oauth-protected-resource/mcp"},
		{"/.well-known/oauth-authorization-server/w/teste", "teste", "/.well-known/oauth-authorization-server"},
		{"/.well-known/oauth-authorization-server/w/teste/", "teste", "/.well-known/oauth-authorization-server/"},
		{"/.well-known/other/w/teste/mcp", "principal", "/.well-known/other/w/teste/mcp"},
	}
	for _, c := range cases {
		slug, rest := splitWorkspacePath(c.in)
		if slug != c.slug || rest != c.rest {
			t.Errorf("%s -> (%q, %q), want (%q, %q)", c.in, slug, rest, c.slug, c.rest)
		}
	}
}

func mcpPost(t *testing.T, url, token, origin string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestPortalRoutesMCPAndWellKnown(t *testing.T) {
	p, _, srv := testPortal(t)
	b := browser(t, srv.URL)
	b.want("POST", "/api/auth/setup", map[string]any{"name": "Admin", "email": "admin@example.test", "password": "test-password-not-a-real-secret"}, 201)
	var principal struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(b.want("POST", "/api/mcp/codes", map[string]string{"name": "Notebook"}, 201), &principal); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := p.create(ctx, "Teste", "teste", sqlc.Person{Name: "Ana", Email: "ana@example.com", PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	const tenantToken = "ziv_tenant-test-token"
	if _, err := p.entries["teste"].pool.Exec(ctx, `INSERT INTO mcp_connections(person_id,kind,name,token_hash) SELECT id,'code','Notebook',$1 FROM people LIMIT 1`, hashForTest(tenantToken)); err != nil {
		t.Fatal(err)
	}

	// /w/teste/mcp reaches teste: its 401 points at teste's metadata.
	resp := mcpPost(t, srv.URL+"/w/teste/mcp", "", "")
	if resp.StatusCode != 401 || resp.Header.Get("WWW-Authenticate") != `Bearer resource_metadata="http://localhost/.well-known/oauth-protected-resource/w/teste/mcp"` {
		t.Fatalf("no token: %d %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}
	// Claude sends no cookies and may send its own Origin; Secure lets it through.
	if resp := mcpPost(t, srv.URL+"/w/teste/mcp", tenantToken, "https://claude.ai"); resp.StatusCode != 200 {
		t.Fatalf("teste token on teste: %d", resp.StatusCode)
	}
	if resp := mcpPost(t, srv.URL+"/mcp", principal.Code, ""); resp.StatusCode != 200 {
		t.Fatalf("principal token on principal: %d", resp.StatusCode)
	}
	// A token never crosses companies.
	if resp := mcpPost(t, srv.URL+"/w/teste/mcp", principal.Code, ""); resp.StatusCode != 401 {
		t.Fatalf("principal token on teste: %d", resp.StatusCode)
	}
	if resp := mcpPost(t, srv.URL+"/mcp", tenantToken, ""); resp.StatusCode != 401 {
		t.Fatalf("teste token on principal: %d", resp.StatusCode)
	}
	// The root /.well-known/<name>/w/<slug>/... form goes to the company
	// handler (its routes come with OAuth), never to the web app page.
	for _, path := range []string{"/.well-known/oauth-protected-resource/w/teste/mcp", "/w/teste/.well-known/oauth-authorization-server", "/w/teste/oauth/token"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		// The web app page fallback is the only answer marked no-cache.
		if resp.StatusCode != 404 || resp.Header.Get("Cache-Control") == "no-cache" {
			t.Fatalf("%s: %d, Cache-Control %q", path, resp.StatusCode, resp.Header.Get("Cache-Control"))
		}
	}
	if status, _ := b.do("GET", "/.well-known/oauth-protected-resource/w/nobody-here/mcp", nil); status != 404 {
		t.Fatalf("unknown company: %d", status)
	}
}
