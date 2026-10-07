package gateway

import (
	"net/http"
	"strings"
)

// Hold makes app addresses answer "Ligando o app…" (auto-refresh, no wake)
// until Release: the server holds them while the supervisor reconciles, so
// the gateway never wakes an app the Hub is about to adopt.
func (g *Gateway) Hold() { g.held.Store(true) }

// Release lets app addresses through again.
func (g *Gateway) Release() { g.held.Store(false) }

// IsAppHost reports whether host is under the apps domain (it may still be
// an address that Match refuses; the gateway answers those with a 404).
func (g *Gateway) IsAppHost(host string) bool {
	return strings.HasSuffix(strings.ToLower(host), "."+g.domain)
}

// Root sends app addresses to the gateway and every other host to hub.
func (g *Gateway) Root(hub http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.IsAppHost(r.Host) {
			g.ServeHTTP(w, r)
			return
		}
		hub.ServeHTTP(w, r)
	})
}

// Page writes the gateway's plain page (or a short JSON error for API
// calls); the Hub uses it for full-page answers of /api/shared/authorize.
func Page(w http.ResponseWriter, r *http.Request, status int, title, message, hubURL string) {
	show(w, r, status, page{Title: title, Message: message, HubURL: hubURL})
}
