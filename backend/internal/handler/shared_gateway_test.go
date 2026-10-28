package handler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSharedAuthorizeIssuesCode(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&volta=%2Fpagina", nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != http.StatusFound || loc.Host != "notas.localhost:8080" || loc.Path != "/__apphub/entrar" || loc.Query().Get("volta") != "/pagina" {
		t.Fatalf("%d %s", res.StatusCode, loc)
	}
	if _, ok := d.codes.Redeem(loc.Query().Get("code"), id, false); !ok {
		t.Fatal("code not valid for the app")
	}
}

func TestSharedAuthorizeUnsafeVoltaGoesHome(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	sharedApp(t, h, d, "notas")
	c.noRedirect()
	for _, v := range []string{"//evil.com", "/\t/evil.com", "https://evil.com/x", "/\\evil.com", "pagina", ""} {
		res := c.raw("GET", "/api/shared/authorize?app=notas&volta="+url.QueryEscape(v), nil)
		loc, _ := url.Parse(res.Header.Get("Location"))
		if res.StatusCode != http.StatusFound || loc.Path != "/__apphub/entrar" || loc.Query().Get("volta") != "/" {
			t.Fatalf("volta %q: %d %s", v, res.StatusCode, loc)
		}
	}
}

func TestSharedAuthorizeWithoutSessionGoesToLogin(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	sharedApp(t, h, d, "notas")
	c.do("POST", "/api/auth/logout", nil, nil)
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&volta=%2F", nil)
	if res.StatusCode != http.StatusFound || !strings.HasPrefix(res.Header.Get("Location"), "/entrar?volta=%2Fapi%2Fshared%2Fauthorize") {
		t.Fatalf("%d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestSharedAuthorizeRestrictedApp(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	if code := c.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{}, "person_ids": []int64{}}, nil); code != 200 {
		t.Fatalf("restrict %d", code)
	}
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	loginAs(t, c, "ana@alis.com")
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&volta=%2F", nil)
	if res.StatusCode != http.StatusFound || !strings.HasSuffix(res.Header.Get("Location"), "notas.localhost:8080/__apphub/sem-acesso") {
		t.Fatalf("%d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestSharedAuthorizeTestBoxNeedsRestore(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	sharedApp(t, h, d, "notas")
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	loginAs(t, c, "ana@alis.com")
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&teste=1&volta=%2F", nil)
	if res.StatusCode != http.StatusFound || !strings.HasSuffix(res.Header.Get("Location"), "notas--teste.localhost:8080/__apphub/sem-acesso") {
		t.Fatalf("%d %s", res.StatusCode, res.Header.Get("Location"))
	}
	// The same person can open the live app (not restricted).
	res = c.raw("GET", "/api/shared/authorize?app=notas&volta=%2F", nil)
	if loc, _ := url.Parse(res.Header.Get("Location")); loc.Host != "notas.localhost:8080" || loc.Path != "/__apphub/entrar" {
		t.Fatalf("live: %s", loc)
	}
}

func TestSharedAuthorizeUnknownAppAndNoServer(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	c.noRedirect()
	page := func(path string) (int, string, string) {
		req, _ := http.NewRequest("GET", c.srv.URL+path, nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		res, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Content-Type"), string(b)
	}
	if code, ct, body := page("/api/shared/authorize?app=notas&volta=%2F"); code != http.StatusServiceUnavailable ||
		!strings.HasPrefix(ct, "text/html") || !strings.Contains(body, "O servidor de apps não está disponível") {
		t.Fatalf("without shared parts: %d %s %s", code, ct, body)
	}
	withShared(t, h)
	if code, ct, body := page("/api/shared/authorize?app=nada&volta=%2F"); code != http.StatusNotFound ||
		!strings.HasPrefix(ct, "text/html") || !strings.Contains(body, "App não encontrado") {
		t.Fatalf("unknown app: %d %s %s", code, ct, body)
	}
}

func TestGatewayBackendAllowed(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	b := h.GatewayBackend()
	ident, ok, err := b.Allowed(context.Background(), admin.ID, id, false)
	if err != nil || !ok || ident.Email != admin.Email || ident.Role != "Administrador" || ident.Name != "Henry" {
		t.Fatalf("%+v %v %v", ident, ok, err)
	}
	if _, ok, _ := b.Allowed(context.Background(), admin.ID, id, true); !ok {
		t.Fatal("admin cannot open the test box")
	}
	tg, found, _ := b.Lookup(context.Background(), "notas", false)
	if !found || tg.AppID != id || tg.Status != "draft" || tg.Owner == "" || tg.Name != "Notas" {
		t.Fatalf("%+v", tg)
	}
	if _, found, _ := b.Lookup(context.Background(), "nada", false); found {
		t.Fatal("found an app that does not exist")
	}
	if _, ok, _ := b.Allowed(context.Background(), 9999, id, false); ok {
		t.Fatal("unknown person allowed")
	}
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	var ana int64
	if err := testPool.QueryRow(context.Background(), "SELECT id FROM people WHERE email = 'ana@alis.com'").Scan(&ana); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := b.Allowed(context.Background(), ana, id, true); ok {
		t.Fatal("plain user allowed into the test box")
	}
	if _, ok, _ := b.Allowed(context.Background(), ana, id, false); !ok {
		t.Fatal("plain user cannot open an unrestricted app")
	}
}
