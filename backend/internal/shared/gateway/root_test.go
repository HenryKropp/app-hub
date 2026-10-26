package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func hubMux() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hub")) })
}

func TestRootServesHubWhileHeldAndHoldsAppHosts(t *testing.T) {
	g, b := setup(t)
	g.Hold()
	root := g.Root(hubMux())
	for _, host := range []string{"localhost:8080", "127.0.0.1:8080", "localhost:5173"} {
		if w := get(g, host, "/", nil, nil); w.Code != http.StatusNotFound {
			// sanity: the gateway alone refuses non-app hosts
			t.Fatalf("gateway on %s: %d", host, w.Code)
		}
		r := httptest.NewRequest("GET", "http://"+host+"/up", nil)
		r.Host = host
		w := httptest.NewRecorder()
		root.ServeHTTP(w, r)
		if w.Body.String() != "hub" {
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

func TestRootSendsEveryAppDomainHostToGateway(t *testing.T) {
	g, _ := setup(t)
	root := g.Root(hubMux())
	for _, host := range []string{"x.notas.localhost:8080", "--teste.localhost:8080", "NOTAS.LOCALHOST:8080"} {
		w := serve(root, host, "/")
		if w.Body.String() == "hub" {
			t.Fatalf("%s reached the Hub", host)
		}
		if host != "NOTAS.LOCALHOST:8080" && w.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", host, w.Code)
		}
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

func serve(h http.Handler, host, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://"+host+path, nil)
	r.Host = host
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
