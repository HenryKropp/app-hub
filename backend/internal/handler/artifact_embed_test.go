package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/config"
)

func TestFramingAllowed(t *testing.T) {
	cases := []struct {
		name string
		h    map[string][]string
		want bool
	}{
		{"no headers", nil, true},
		{"xfo deny", map[string][]string{"X-Frame-Options": {"DENY"}}, false},
		{"xfo sameorigin", map[string][]string{"X-Frame-Options": {"SAMEORIGIN"}}, false},
		{"ancestors self", map[string][]string{"Content-Security-Policy": {"default-src 'self'; frame-ancestors 'self'"}}, false},
		{"ancestors none", map[string][]string{"Content-Security-Policy": {"frame-ancestors 'none'"}}, false},
		{"ancestors named site", map[string][]string{"Content-Security-Policy": {"frame-ancestors https://teams.microsoft.com"}}, false},
		{"ancestors star", map[string][]string{"Content-Security-Policy": {"script-src 'self'; frame-ancestors *"}}, true},
		{"ancestors https", map[string][]string{"Content-Security-Policy": {"Frame-Ancestors https:"}}, true},
		{"ancestors star wins over xfo", map[string][]string{"Content-Security-Policy": {"frame-ancestors *"}, "X-Frame-Options": {"DENY"}}, true},
		{"one of two policies closed", map[string][]string{"Content-Security-Policy": {"frame-ancestors *", "frame-ancestors 'self'"}}, false},
		{"csp without ancestors keeps xfo", map[string][]string{"Content-Security-Policy": {"default-src 'self'"}, "X-Frame-Options": {"DENY"}}, false},
	}
	for _, c := range cases {
		h := http.Header{}
		for k, vs := range c.h {
			for _, v := range vs {
				h.Add(k, v)
			}
		}
		if got := framingAllowed(h); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPublicAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"8.8.8.8": true, "2606:4700::1111": true,
		"127.0.0.1": false, "10.1.2.3": false, "192.168.0.10": false, "172.16.0.1": false,
		"169.254.169.254": false, "100.64.0.1": false, "0.0.0.0": false, "::1": false, "fe80::1": false,
		"::ffff:127.0.0.1": false, "fd00::1": false,
		"64:ff9b::a9fe:a9fe": false, "::127.0.0.1": false, "2002:7f00:1::1": false, "0.1.2.3": false,
		"192.0.0.192": false, "198.18.0.1": false, "240.0.0.1": false, "2001:0:4136:e378::1": false,
	} {
		if got := publicAddr(netip.MustParseAddr(addr)); got != want {
			t.Errorf("%s: got %v, want %v", addr, got, want)
		}
	}
}

// The real check never reaches the server's own network, even when the site
// would allow framing.
func TestCheckLinkEmbeddableRefusesLocalAddresses(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	if checkLinkEmbeddable(context.Background(), srv.URL, nil) {
		t.Fatal("a loopback address must not be probed")
	}
	if checkLinkEmbeddable(context.Background(), "http://example.com", nil) {
		t.Fatal("only https links are checked")
	}
}

func TestLinkArtifactEmbeddable(t *testing.T) {
	_, ana, _, _ := team(t)
	d := createArtifact(t, ana, linkBody("Power BI", "https://app.exemplo.com/embed/painel"))
	if !d.Versions[0].Embeddable {
		t.Fatalf("embed link should be embeddable: %+v", d.Versions[0])
	}
	var v artifactDetailJSON
	ana.do("POST", artifactPath(d.ID, "/versions"), map[string]any{"kind": "link", "url": "https://exemplo.com/painel"}, &v)
	if v.Versions[0].Embeddable || !v.Versions[1].Embeddable {
		t.Fatalf("versions %+v", v.Versions)
	}
	var r artifactDetailJSON
	if code := ana.do("POST", artifactPath(d.ID, "/versions/1/restore"), nil, &r); code != http.StatusOK || r.Current != 3 || !r.Versions[0].Embeddable {
		t.Fatalf("restore keeps embeddable: %d %+v", code, r.Versions)
	}
	own := createArtifact(t, ana, linkBody("Ziv", "https://empresa.hub.test/embed/x"))
	if own.Versions[0].Embeddable {
		t.Fatal("a link to the Ziv itself must never be framed")
	}
	f := createArtifact(t, ana, fileBody("Arquivo", "a.html", []byte(pageHTML)))
	if f.Versions[0].Embeddable {
		t.Fatal("file versions do not use embeddable")
	}
}

func TestZivRefusesBeingFramed(t *testing.T) {
	h := New(nil, config.Config{BaseURL: "http://hub.test"})
	rec := httptest.NewRecorder()
	h.Secure(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/status", nil))
	if rec.Header().Get("Content-Security-Policy") != "frame-ancestors 'none'" || rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("headers %v", rec.Header())
	}
	// The artifact content keeps its own policy, framable only by the Ziv.
	rec = httptest.NewRecorder()
	h.Secure(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", artifactCSP+"; frame-ancestors 'self'")
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/api/artifacts/1/content", nil))
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") || !strings.Contains(csp, "frame-ancestors 'self'") {
		t.Fatalf("content csp %q", csp)
	}
}

// A link whose redirect lands on the Ziv is never framed.
func TestCheckLinkEmbeddableStopsAtOwnHost(t *testing.T) {
	own := func(u string) bool { return strings.Contains(u, "hub.test") }
	if checkLinkEmbeddable(context.Background(), "https://hub.test/x", own) {
		t.Fatal("own host")
	}
}
