package gateway

import (
	"context"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// AskPath is where Caddy's on-demand TLS asks whether an address deserves a
// certificate (only from the same machine).
const AskPath = "/__ziv/tls-ask"

// principalZone is the name the principal company's gateway is set under: its
// apps live one label below the apps domain.
const principalZone = "principal"

var companySlug = regexp.MustCompile(`^[a-z][a-z0-9-]{2,39}$`)

// Router picks the company's gateway from the address: <app>.<domain> is the
// principal company, <app>.<company>.<domain> is that company. Everything
// that is not under the apps domain goes to the Hub.
type Router struct {
	domain string

	mu       sync.RWMutex
	gateways map[string]*Gateway
	loader   func(ctx context.Context, company string) bool
}

func NewRouter(domain string) *Router {
	return &Router{domain: strings.ToLower(domain), gateways: map[string]*Gateway{}}
}

// Set registers company's gateway ("principal" for the one-label zone).
func (r *Router) Set(company string, g *Gateway) {
	r.mu.Lock()
	r.gateways[company] = g
	r.mu.Unlock()
}

// SetLoader is called once per request for a company with no gateway yet; it
// attaches the company (whose hook calls Set) and says whether it exists.
func (r *Router) SetLoader(fn func(ctx context.Context, company string) bool) {
	r.mu.Lock()
	r.loader = fn
	r.mu.Unlock()
}

func (r *Router) gateway(ctx context.Context, company string) *Gateway {
	r.mu.RLock()
	g, load := r.gateways[company], r.loader
	r.mu.RUnlock()
	if g != nil || load == nil || company == principalZone {
		return g
	}
	if !load(ctx, company) {
		return nil
	}
	r.mu.RLock()
	g = r.gateways[company]
	r.mu.RUnlock()
	return g
}

// route answers which gateway serves host, and whether host is under the
// apps domain at all.
func (r *Router) route(ctx context.Context, host string) (*Gateway, bool) {
	sub, ok := strings.CutSuffix(strings.ToLower(host), "."+r.domain)
	if !ok {
		return nil, false
	}
	labels := strings.Split(sub, ".")
	switch {
	case len(labels) == 1 && labels[0] != "":
		return r.gateway(ctx, principalZone), true
	case len(labels) == 2 && labels[0] != "" && labels[1] != principalZone && companySlug.MatchString(labels[1]):
		return r.gateway(ctx, labels[1]), true
	}
	return nil, true
}

// Root sends app addresses to their company's gateway, answers the TLS ask,
// and sends every other request to hub.
func (r *Router) Root(hub http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == AskPath {
			r.ask(w, req)
			return
		}
		if g, isApp := r.route(req.Context(), req.Host); isApp {
			if g == nil {
				Page(w, req, http.StatusNotFound, "App não encontrado", "Este endereço não é de nenhum app compartilhado do Ziv.", "")
				return
			}
			g.ServeHTTP(w, req)
			return
		}
		hub.ServeHTTP(w, req)
	})
}

// ask answers 200 only to the same machine (no proxy in between) and only
// for the address of an app that exists.
func (r *Router) ask(w http.ResponseWriter, req *http.Request) {
	if strings.HasSuffix(strings.ToLower(req.Host), "."+r.domain) { // asked on an app address
		http.NotFound(w, req)
		return
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() || req.Header.Get("X-Forwarded-For") != "" {
		http.NotFound(w, req)
		return
	}
	domain := req.URL.Query().Get("domain")
	if g, _ := r.route(req.Context(), domain); g != nil && g.Known(req.Context(), domain) {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.NotFound(w, req)
}
