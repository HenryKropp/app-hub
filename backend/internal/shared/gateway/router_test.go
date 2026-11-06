package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func hubMux() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hub")) })
}

func serve(h http.Handler, host, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://"+host+path, nil)
	r.Host = host
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// companyGateway is a gateway for company slug, under localhost:8080.
func companyGateway(t *testing.T, slug string) (*Gateway, *fakeBackend) {
	_, port := upstream(t)
	b := &fakeBackend{target: Target{AppID: 1, Name: "Notas", Owner: "Henry", Status: "running", Port: port}, allowed: true}
	return New(slug+".localhost:8080", "http://localhost:5173/w/"+slug, false, b), b
}

func TestRouterPrincipalHostUnchanged(t *testing.T) {
	g, b := setup(t)
	r := NewRouter("localhost:8080")
	r.Set("principal", g)
	serve(r.Root(hubMux()), "notas.localhost:8080", "/")
	if got := b.lookups(); len(got) != 1 || got[0] != "notas" {
		t.Fatal(got)
	}
}

func TestRouterCompanyHost(t *testing.T) {
	g, pb := setup(t)
	cg, cb := companyGateway(t, "teste")
	r := NewRouter("localhost:8080")
	r.Set("principal", g)
	r.Set("teste", cg)
	root := r.Root(hubMux())
	serve(root, "notas.teste.localhost:8080", "/")
	serve(root, "notas--teste.teste.localhost:8080", "/")
	if got := cb.lookups(); len(got) != 2 || got[0] != "notas" || got[1] != "notas--teste" {
		t.Fatal(got)
	}
	if got := pb.lookups(); len(got) != 0 {
		t.Fatal("principal saw", got)
	}
}

func TestRouterRejects(t *testing.T) {
	g, pb := setup(t)
	cg, cb := companyGateway(t, "teste")
	r := NewRouter("localhost:8080")
	r.Set("principal", g)
	r.Set("teste", cg)
	var loaded []string
	r.SetLoader(func(_ context.Context, company string) bool { loaded = append(loaded, company); return false })
	root := r.Root(hubMux())
	for _, host := range []string{"notas.principal.localhost:8080", "a.b.c.localhost:8080", "notas.ab.localhost:8080", "notas.nao-existe.localhost:8080"} {
		w := serve(root, host, "/")
		if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "App não encontrado") {
			t.Fatalf("%s: %d %s", host, w.Code, w.Body.String())
		}
	}
	if len(pb.lookups()) != 0 || len(cb.lookups()) != 0 {
		t.Fatal(pb.lookups(), cb.lookups())
	}
	if len(loaded) != 1 || loaded[0] != "nao-existe" {
		t.Fatal(loaded)
	}
	serve(root, "NOTAS.TESTE.LOCALHOST:8080", "/")
	if got := cb.lookups(); len(got) != 1 || got[0] != "notas" {
		t.Fatal(got)
	}
}

func TestRouterLoadsCompany(t *testing.T) {
	gw, b := companyGateway(t, "nova")
	r := NewRouter("localhost:8080")
	r.SetLoader(func(_ context.Context, company string) bool {
		if company != "nova" {
			return false
		}
		r.Set("nova", gw)
		return true
	})
	serve(r.Root(hubMux()), "x.nova.localhost:8080", "/")
	if got := b.lookups(); len(got) != 1 || got[0] != "x" {
		t.Fatal(got)
	}
}

func TestCodeFromOtherCompanyRefused(t *testing.T) {
	gwA, _ := companyGateway(t, "a00")
	gwB, _ := companyGateway(t, "b00")
	r := NewRouter("localhost:8080")
	r.Set("a00", gwA)
	r.Set("b00", gwB)
	root := r.Root(hubMux())
	codeA := gwA.Codes.Issue(7, 1, false)
	w := serve(root, "notas.b00.localhost:8080", "/__apphub/entrar?code="+codeA)
	if w.Code != http.StatusFound || !strings.HasPrefix(w.Header().Get("Location"), "http://localhost:5173/w/b00/api/shared/authorize") {
		t.Fatalf("%d %s", w.Code, w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			t.Fatal("cookie set by B for A's code")
		}
	}

	// A session obtained on A does not open B.
	code := gwA.Codes.Issue(7, 1, false)
	wa := serve(root, "notas.a00.localhost:8080", "/__apphub/entrar?code="+code)
	var cookie *http.Cookie
	for _, c := range wa.Result().Cookies() {
		if c.Name == cookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatalf("no session on A: %d %s", wa.Code, wa.Body.String())
	}
	req := httptest.NewRequest("GET", "http://notas.b00.localhost:8080/", nil)
	req.Host = "notas.b00.localhost:8080"
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	wb := httptest.NewRecorder()
	root.ServeHTTP(wb, req)
	if wb.Code != http.StatusFound || !strings.Contains(wb.Header().Get("Location"), "/w/b00/api/shared/authorize") {
		t.Fatalf("A's session on B: %d %s", wb.Code, wb.Header().Get("Location"))
	}
}

func ask(root http.Handler, remote, domain string, hdr map[string]string) int {
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080"+AskPath+"?domain="+domain, nil)
	r.Host = "127.0.0.1:8080"
	r.RemoteAddr = remote
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	root.ServeHTTP(w, r)
	return w.Code
}

func TestAsk(t *testing.T) {
	cg, _ := companyGateway(t, "teste")
	r := NewRouter("localhost:8080")
	r.Set("teste", cg)
	root := r.Root(hubMux())
	if c := ask(root, "127.0.0.1:5555", "notas.teste.localhost:8080", nil); c != http.StatusOK {
		t.Fatal("known app", c)
	}
	if c := ask(root, "127.0.0.1:5555", "zzz.teste.localhost:8080", nil); c != http.StatusNotFound {
		t.Fatal("unknown app", c)
	}
	if c := ask(root, "127.0.0.1:5555", "notas.teste.localhost:8080", map[string]string{"X-Forwarded-For": "1.2.3.4"}); c != http.StatusNotFound {
		t.Fatal("forwarded", c)
	}
	if c := ask(root, "10.0.0.5:1", "notas.teste.localhost:8080", nil); c != http.StatusNotFound {
		t.Fatal("not loopback", c)
	}
	w := serve(root, "notas.teste.localhost:8080", AskPath+"?domain=notas.teste.localhost:8080")
	if w.Code != http.StatusNotFound {
		t.Fatal("ask on an app host", w.Code)
	}
}

func TestRootServesHubWhileHeldAndHoldsAppHosts(t *testing.T) {
	g, b := setup(t)
	g.Hold()
	r := NewRouter("localhost:8080")
	r.Set("principal", g)
	root := r.Root(hubMux())
	for _, host := range []string{"localhost:8080", "127.0.0.1:8080", "localhost:5173"} {
		if w := serve(root, host, "/up"); w.Body.String() != "hub" {
			t.Fatalf("hub host %s not served while held: %d %s", host, w.Code, w.Body.String())
		}
	}
	w := serve(root, "notas.localhost:8080", "/")
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Ligando o app") || !strings.Contains(w.Body.String(), `http-equiv="refresh"`) {
		t.Fatalf("held app host: %d %s", w.Code, w.Body.String())
	}
	b.mu.Lock()
	woken := len(b.woken)
	b.mu.Unlock()
	if woken != 0 {
		t.Fatal("app woken while the Hub is still reconciling")
	}
	g.Release()
	if w := serve(root, "notas.localhost:8080", "/"); w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "/api/shared/authorize") {
		t.Fatalf("after release: %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestEmptyHubURLShowsNotConfigured(t *testing.T) {
	_, port := upstream(t)
	b := &fakeBackend{target: Target{AppID: 7, Name: "Notas", Status: "running", Port: port}, allowed: true}
	g := New("localhost:8080", "", false, b)
	w := get(g, "notas.localhost:8080", "/", nil, nil)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Endereços de apps não configurados") || w.Header().Get("Location") != "" {
		t.Fatalf("%d %s %s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
}

func TestPageWritesHTML(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/shared/authorize", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	Page(w, r, http.StatusNotFound, "App não encontrado", "Confira o endereço.", "/")
	if w.Code != 404 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") || !strings.Contains(w.Body.String(), "Confira o endereço.") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
