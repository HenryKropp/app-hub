package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const testVerifier = "verificador-de-teste-com-mais-de-quarenta-e-tres-letras-0123456789"

func testChallenge(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func getJSON(t *testing.T, url string, out any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_ = json.NewDecoder(resp.Body).Decode(out)
	return resp.StatusCode
}

func TestOAuthMetadata(t *testing.T) {
	c, h := newPoolClient(t)
	h.cfg.BaseURL = "https://ziv.example/w/teste/"
	var res struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		BearerMethods        []string `json:"bearer_methods_supported"`
	}
	for _, p := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
		if st := getJSON(t, c.srv.URL+p, &res); st != 200 {
			t.Fatalf("%s = %d", p, st)
		}
		if res.Resource != "https://ziv.example/w/teste/mcp" || len(res.AuthorizationServers) != 1 || res.AuthorizationServers[0] != "https://ziv.example/w/teste" || len(res.BearerMethods) != 1 || res.BearerMethods[0] != "header" {
			t.Fatalf("%s = %+v", p, res)
		}
	}
	var as struct {
		Issuer         string   `json:"issuer"`
		Authorization  string   `json:"authorization_endpoint"`
		Token          string   `json:"token_endpoint"`
		Registration   string   `json:"registration_endpoint"`
		ResponseTypes  []string `json:"response_types_supported"`
		GrantTypes     []string `json:"grant_types_supported"`
		Challenge      []string `json:"code_challenge_methods_supported"`
		TokenAuthMeths []string `json:"token_endpoint_auth_methods_supported"`
	}
	if st := getJSON(t, c.srv.URL+"/.well-known/oauth-authorization-server", &as); st != 200 {
		t.Fatalf("as metadata = %d", st)
	}
	base := "https://ziv.example/w/teste"
	if as.Issuer != base || as.Authorization != base+"/oauth/authorize" || as.Token != base+"/oauth/token" || as.Registration != base+"/oauth/register" {
		t.Fatalf("endpoints = %+v", as)
	}
	if strings.Join(as.Challenge, ",") != "S256" || strings.Join(as.ResponseTypes, ",") != "code" ||
		strings.Join(as.GrantTypes, ",") != "authorization_code,refresh_token" || strings.Join(as.TokenAuthMeths, ",") != "none" {
		t.Fatalf("supported = %+v", as)
	}
}

type oauthClient struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
	AuthMethod   string   `json:"token_endpoint_auth_method"`
}

type tokenOut struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
}

const testRedirect = "https://claude.ai/api/mcp/auth_callback"

func registerClient(t *testing.T, c *client, redirects ...string) (int, oauthClient) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"client_name": "Claude", "redirect_uris": redirects})
	resp, err := http.Post(c.srv.URL+"/oauth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out oauthClient
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

var oauthNoRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// authorize calls /oauth/authorize and returns the status and Location.
func authorize(t *testing.T, c *client, q url.Values) (int, string) {
	t.Helper()
	resp, err := oauthNoRedirect.Get(c.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}

func authorizeQuery(clientID, redirect string) url.Values {
	return url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect},
		"code_challenge": {testChallenge(testVerifier)}, "code_challenge_method": {"S256"}, "state": {"estado 1&2"},
	}
}

func postToken(t *testing.T, c *client, form url.Values) (int, tokenOut) {
	t.Helper()
	resp, err := http.PostForm(c.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out tokenOut
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// approvedCode runs register → authorize → approve and returns the client and code.
func approvedCode(t *testing.T, c *client) (oauthClient, string) {
	t.Helper()
	st, cl := registerClient(t, c, testRedirect)
	if st != http.StatusCreated || cl.ClientID == "" || cl.AuthMethod != "none" || cl.ClientName != "Claude" {
		t.Fatalf("register = %d %+v", st, cl)
	}
	st, loc := authorize(t, c, authorizeQuery(cl.ClientID, testRedirect))
	if st != http.StatusFound || !strings.HasPrefix(loc, "/conectar-claude/autorizar?pedido=") {
		t.Fatalf("authorize = %d %q", st, loc)
	}
	id := strings.TrimPrefix(loc, "/conectar-claude/autorizar?pedido=")
	var info struct {
		ClientName   string `json:"client_name"`
		RedirectHost string `json:"redirect_host"`
	}
	if st := c.do("GET", "/api/oauth/requests/"+id, nil, &info); st != 200 || info.ClientName != "Claude" || info.RedirectHost != "claude.ai" {
		t.Fatalf("request info = %d %+v", st, info)
	}
	var appr struct {
		Redirect string `json:"redirect"`
	}
	if st := c.do("POST", "/api/oauth/requests/"+id+"/approve", nil, &appr); st != 200 {
		t.Fatalf("approve = %d", st)
	}
	u, err := url.Parse(appr.Redirect)
	if err != nil || !strings.HasPrefix(appr.Redirect, testRedirect+"?") || u.Query().Get("state") != "estado 1&2" || u.Query().Get("code") == "" {
		t.Fatalf("approve redirect = %q", appr.Redirect)
	}
	// Used: a second approval or a lookup is 404.
	if st := c.do("POST", "/api/oauth/requests/"+id+"/approve", nil, nil); st != http.StatusNotFound {
		t.Fatalf("second approve = %d", st)
	}
	return cl, u.Query().Get("code")
}

func codeForm(cl oauthClient, code, verifier, redirect string) url.Values {
	return url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}, "client_id": {cl.ClientID}, "code_verifier": {verifier}}
}

func TestOAuthFullFlow(t *testing.T) {
	c, _ := newPoolClient(t)
	admin := setupAdmin(t, c)
	cl, code := approvedCode(t, c)
	st, tok := postToken(t, c, codeForm(cl, code, testVerifier, testRedirect))
	if st != 200 || !strings.HasPrefix(tok.AccessToken, "ziv_") || !strings.HasPrefix(tok.RefreshToken, "ziv_") || tok.TokenType != "Bearer" || tok.ExpiresIn != 3600 {
		t.Fatalf("token = %d %+v", st, tok)
	}
	var kind, name string
	var person int64
	if err := testPool.QueryRow(context.Background(), "SELECT kind, name, person_id FROM mcp_connections").Scan(&kind, &name, &person); err != nil || kind != "oauth" || name != "Claude" || person != admin.ID {
		t.Fatalf("connection = %s %s %d %v", kind, name, person, err)
	}
	if resp := postMCP(t, c.srv.URL+"/mcp", tok.AccessToken); resp.StatusCode != http.StatusOK {
		t.Fatalf("mcp with access token = %d", resp.StatusCode)
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {cl.ClientID}}
	st, tok2 := postToken(t, c, refresh)
	if st != 200 || tok2.AccessToken == "" || tok2.AccessToken == tok.AccessToken || tok2.RefreshToken == tok.RefreshToken || tok2.ExpiresIn != 3600 {
		t.Fatalf("refresh = %d %+v", st, tok2)
	}
	if st, again := postToken(t, c, refresh); st != http.StatusBadRequest || again.Error != "invalid_grant" || again.AccessToken != "" {
		t.Fatalf("old refresh = %d %+v", st, again)
	}
	if resp := postMCP(t, c.srv.URL+"/mcp", tok.AccessToken); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old access token = %d", resp.StatusCode)
	}
	if resp := postMCP(t, c.srv.URL+"/mcp", tok2.AccessToken); resp.StatusCode != http.StatusOK {
		t.Fatalf("new access token = %d", resp.StatusCode)
	}
	if st, _ := postToken(t, c, url.Values{"grant_type": {"password"}}); st != http.StatusBadRequest {
		t.Fatalf("unsupported grant = %d", st)
	}
}

func TestOAuthDeny(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	_, cl := registerClient(t, c, testRedirect)
	q := authorizeQuery(cl.ClientID, testRedirect)
	q.Del("state")
	_, loc := authorize(t, c, q)
	id := strings.TrimPrefix(loc, "/conectar-claude/autorizar?pedido=")
	if st := c.fresh().do("GET", "/api/oauth/requests/"+id, nil, nil); st != http.StatusUnauthorized {
		t.Fatalf("logged-out lookup = %d", st)
	}
	var out struct {
		Redirect string `json:"redirect"`
	}
	if st := c.do("POST", "/api/oauth/requests/"+id+"/deny", nil, &out); st != 200 || out.Redirect != testRedirect+"?error=access_denied" {
		t.Fatalf("deny = %d %q", st, out.Redirect)
	}
	if st := c.do("GET", "/api/oauth/requests/"+id, nil, nil); st != http.StatusNotFound {
		t.Fatalf("denied lookup = %d", st)
	}
	if st := c.do("GET", "/api/oauth/requests/nao-existe", nil, nil); st != http.StatusNotFound {
		t.Fatalf("missing lookup = %d", st)
	}
}

func TestOAuthRejects(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	count := func() int {
		var n int
		_ = testPool.QueryRow(context.Background(), "SELECT count(*) FROM mcp_connections").Scan(&n)
		return n
	}
	for _, bad := range []string{"http://evil.com/cb", "javascript:alert(1)", "https:///semhost", "http://localhost.evil.com/cb"} {
		if st, _ := registerClient(t, c, bad); st != http.StatusBadRequest {
			t.Fatalf("register %q = %d", bad, st)
		}
	}
	if st, _ := registerClient(t, c); st != http.StatusBadRequest {
		t.Fatalf("register without redirect = %d", st)
	}
	if st, _ := registerClient(t, c, "http://localhost:3334/cb", "http://127.0.0.1/cb"); st != http.StatusCreated {
		t.Fatalf("register localhost = %d", st)
	}

	// Authorize: unknown client or unregistered redirect never redirects.
	_, cl := registerClient(t, c, testRedirect)
	if st, loc := authorize(t, c, authorizeQuery("desconhecido", testRedirect)); st != http.StatusBadRequest || loc != "" {
		t.Fatalf("unknown client = %d %q", st, loc)
	}
	if st, loc := authorize(t, c, authorizeQuery(cl.ClientID, "https://evil.com/cb")); st != http.StatusBadRequest || loc != "" {
		t.Fatalf("unregistered redirect = %d %q", st, loc)
	}
	q := authorizeQuery(cl.ClientID, testRedirect)
	q.Set("code_challenge_method", "plain")
	if st, loc := authorize(t, c, q); st != http.StatusFound || !strings.Contains(loc, "error=invalid_request") || !strings.HasPrefix(loc, testRedirect) {
		t.Fatalf("plain challenge = %d %q", st, loc)
	}

	// Wrong verifier burns the code.
	cl, code := approvedCode(t, c)
	if st, tok := postToken(t, c, codeForm(cl, code, testVerifier+"x", testRedirect)); st != http.StatusBadRequest || tok.Error != "invalid_grant" || tok.AccessToken != "" {
		t.Fatalf("wrong verifier = %d %+v", st, tok)
	}
	if st, tok := postToken(t, c, codeForm(cl, code, testVerifier, testRedirect)); st != http.StatusBadRequest || tok.AccessToken != "" {
		t.Fatalf("code after failure = %d %+v", st, tok)
	}
	// Reused code.
	cl, code = approvedCode(t, c)
	if st, _ := postToken(t, c, codeForm(cl, code, testVerifier, testRedirect)); st != 200 {
		t.Fatalf("first use = %d", st)
	}
	if st, tok := postToken(t, c, codeForm(cl, code, testVerifier, testRedirect)); st != http.StatusBadRequest || tok.Error != "invalid_grant" || tok.AccessToken != "" {
		t.Fatalf("reused code = %d %+v", st, tok)
	}
	// Mismatched redirect_uri.
	cl, code = approvedCode(t, c)
	if st, tok := postToken(t, c, codeForm(cl, code, testVerifier, testRedirect+"/outro")); st != http.StatusBadRequest || tok.Error != "invalid_grant" || tok.AccessToken != "" {
		t.Fatalf("mismatched redirect = %d %+v", st, tok)
	}
	// Missing fields.
	if st, tok := postToken(t, c, url.Values{"grant_type": {"authorization_code"}}); st != http.StatusBadRequest || tok.Error != "invalid_request" {
		t.Fatalf("missing fields = %d %+v", st, tok)
	}
	if n := count(); n != 1 {
		t.Fatalf("connections = %d, want only the one good exchange", n)
	}
}

func TestOAuthExpiredCode(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	cl, code := approvedCode(t, c)
	if _, err := testPool.Exec(context.Background(), "UPDATE mcp_auth_requests SET code_expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if st, tok := postToken(t, c, codeForm(cl, code, testVerifier, testRedirect)); st != http.StatusBadRequest || tok.Error != "invalid_grant" {
		t.Fatalf("expired code = %d %+v", st, tok)
	}
}

func TestOAuthExpiredAccessToken(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	cl, code := approvedCode(t, c)
	_, tok := postToken(t, c, codeForm(cl, code, testVerifier, testRedirect))
	if resp := postMCP(t, c.srv.URL+"/mcp", tok.AccessToken); resp.StatusCode != http.StatusOK {
		t.Fatalf("fresh token = %d", resp.StatusCode)
	}
	if _, err := testPool.Exec(context.Background(), "UPDATE mcp_connections SET access_expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if resp := postMCP(t, c.srv.URL+"/mcp", tok.AccessToken); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired token = %d", resp.StatusCode)
	}
	// An expired refresh token is refused too.
	if _, err := testPool.Exec(context.Background(), "UPDATE mcp_connections SET refresh_expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if st, _ := postToken(t, c, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {cl.ClientID}}); st != http.StatusBadRequest {
		t.Fatalf("expired refresh = %d", st)
	}
}

func TestOAuthAuthorizeUsesCompanyPath(t *testing.T) {
	c, h := newPoolClient(t)
	h.cfg.BaseURL = "https://ziv.example/w/teste"
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste", BasePath: "/w/teste"})
	_, cl := registerClient(t, c, testRedirect)
	if st, loc := authorize(t, c, authorizeQuery(cl.ClientID, testRedirect)); st != http.StatusFound || !strings.HasPrefix(loc, "/w/teste/conectar-claude/autorizar?pedido=") {
		t.Fatalf("authorize = %d %q", st, loc)
	}
}
