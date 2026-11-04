package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/HenryKropp/app-hub/backend/internal/config"
)

// bearerTransport adds the personal code to every MCP request.
type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// mcpSession connects a go-sdk client to url with a personal code.
func mcpSession(t *testing.T, url, token string) (*mcp.ClientSession, error) {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "teste", Version: "1"}, nil)
	return client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             url,
		HTTPClient:           &http.Client{Transport: bearerTransport{token}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
}

func newMCPCode(t *testing.T, c *client) mcpCodeOut {
	t.Helper()
	var out mcpCodeOut
	if code := c.do("POST", "/api/mcp/codes", map[string]string{"name": "Notebook"}, &out); code != http.StatusCreated {
		t.Fatalf("create code = %d", code)
	}
	return out
}

func postMCP(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
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

func TestMCPRequiresBearer(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	for _, token := range []string{"", "ziv_desconhecido"} {
		resp := postMCP(t, c.srv.URL+"/mcp", token)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("token %q: status %d, want 401", token, resp.StatusCode)
		}
		got := resp.Header.Get("WWW-Authenticate")
		want := `Bearer resource_metadata="http://hub.test/.well-known/oauth-protected-resource/mcp"`
		if got != want {
			t.Fatalf("WWW-Authenticate = %q, want %q", got, want)
		}
	}
}

func TestMCPResourceMetadataHasCompanyPath(t *testing.T) {
	c, h := newPoolClient(t)
	h.cfg.BaseURL = "https://ziv.example/w/teste"
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste", BasePath: "/w/teste"})
	resp := postMCP(t, c.srv.URL+"/mcp", "")
	want := `Bearer resource_metadata="https://ziv.example/.well-known/oauth-protected-resource/w/teste/mcp"`
	if got := resp.Header.Get("WWW-Authenticate"); resp.StatusCode != 401 || got != want {
		t.Fatalf("status %d, WWW-Authenticate = %q", resp.StatusCode, got)
	}
}

func TestMCPWithCodeListsTools(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	code := newMCPCode(t, c)
	s, err := mcpSession(t, c.srv.URL+"/mcp", code.Code)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.InitializeResult(); got == nil || got.ServerInfo.Name != "ziv" || !strings.Contains(got.Instructions, "Ziv") {
		t.Fatalf("initialize = %+v", got)
	}
	res, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "buscar") {
		t.Fatalf("tools = %v", names)
	}
}

func TestMCPRevokedCodeRejected(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	code := newMCPCode(t, c)
	if resp := postMCP(t, c.srv.URL+"/mcp", code.Code); resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("live code rejected")
	}
	if st := c.do("DELETE", fmt.Sprintf("/api/mcp/connections/%d", code.ID), nil, nil); st != http.StatusNoContent {
		t.Fatalf("revoke = %d", st)
	}
	if resp := postMCP(t, c.srv.URL+"/mcp", code.Code); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked code status %d, want 401", resp.StatusCode)
	}
}

// Tools see the person behind the code, and callAPI acts as that person.
func TestMCPToolsActAsPerson(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	code := newMCPCode(t, c)
	s := h.newMCPServer()
	type none struct{}
	mcp.AddTool(s, &mcp.Tool{Name: "quem"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ none) (*mcp.CallToolResult, any, error) {
		status, raw, err := h.callAPI(ctx, "GET", "/api/auth/status", nil)
		text := fmt.Sprintf("%d|%d|%v|%s", mcpPersonFrom(ctx), status, err, raw)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
	srv := httptest.NewServer(h.mcpHTTPFor(s))
	defer srv.Close()
	sess, err := mcpSession(t, srv.URL, code.Code)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{Name: "quem", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.HasPrefix(text, fmt.Sprintf("%d|200|<nil>|", admin.ID)) || !strings.Contains(text, `"email":"henry@alis.com"`) {
		t.Fatalf("tool saw %q", text)
	}
}

func TestCallAPIAndAPIError(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	ctx := withMCPPerson(context.Background(), admin.ID)
	status, raw, err := h.callAPI(ctx, "POST", "/api/artifacts", map[string]string{"name": ""})
	if err != nil || status != http.StatusBadRequest || apiError(raw) != "Dê um nome ao artifact." {
		t.Fatalf("status %d, err %v, error %q", status, err, apiError(raw))
	}
	if status, _, err := h.callAPI(context.Background(), "GET", "/api/apps?area=1", nil); err != nil || status != http.StatusUnauthorized {
		t.Fatalf("no person: status %d, err %v", status, err)
	}
	if got := apiError([]byte("not json")); got != "" {
		t.Fatalf("apiError(non-json) = %q", got)
	}
}

// Claude's endpoints skip the browser checks; OAuth ones are limited per address.
func TestSecureLetsClaudeEndpointsThrough(t *testing.T) {
	h := New(nil, config.Config{BaseURL: "http://hub.test"})
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	sec := h.Secure(ok)
	call := func(path, ct string) int {
		req := httptest.NewRequest("POST", path, strings.NewReader("grant_type=x"))
		req.Header.Set("Origin", "https://claude.ai")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		sec.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, path := range []string{"/mcp", "/oauth/token", "/oauth/register"} {
		req := httptest.NewRequest("POST", path, strings.NewReader("grant_type=x"))
		req.Header.Set("Origin", "https://claude.ai")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		sec.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s = %d, Cache-Control %q", path, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
	if got := call("/api/mcp/codes", "application/json"); got != http.StatusForbidden {
		t.Fatalf("browser route from another origin = %d, want 403", got)
	}
	for _, path := range []string{"/oauth/token", "/oauth/register"} {
		last := 0
		for i := 0; i < addressAttempts; i++ {
			last = call(path, "application/json")
		}
		if last != http.StatusTooManyRequests {
			t.Fatalf("%s after %d attempts = %d, want 429", path, addressAttempts+1, last)
		}
	}
}

// callAPI reaches only reviewed routes: never the folder scan, never MCP codes.
func TestCallAPIRefusesUnlistedRoutes(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	ctx := withMCPPerson(context.Background(), admin.ID)
	for _, r := range [][2]string{
		{"POST", "/api/scan"},
		{"POST", "/api/apps"},
		{"POST", "/api/mcp/codes"},
		{"GET", "/api/mcp/connections"},
		{"DELETE", "/api/apps/1"},
		{"GET", "/api/apps/abc"},
		{"GET", "/api/apps/../scan"},
		{"GET", "/api/apps/1/history"},
	} {
		status, raw, err := h.callAPI(ctx, r[0], r[1], map[string]string{"root": "/tmp"})
		if !errors.Is(err, errMCPRoute) || status != 0 || raw != nil {
			t.Fatalf("%s %s: status %d, err %v", r[0], r[1], status, err)
		}
	}
	for _, r := range [][2]string{{"GET", "/api/apps/12"}, {"POST", "/api/apps/7/runs"}, {"POST", "/api/artifacts/3/versions"}, {"GET", "/api/runs?page=2"}} {
		if _, _, err := h.callAPI(ctx, r[0], r[1], nil); err != nil {
			t.Fatalf("%s %s refused: %v", r[0], r[1], err)
		}
	}
}

func TestMCPBearerSchemeIgnoresCase(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	code := newMCPCode(t, c)
	req, _ := http.NewRequest("POST", c.srv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "bearer "+code.Code)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lowercase bearer = %d", resp.StatusCode)
	}
}
