package gateway

import (
	"net/http"
)

// Hold makes app addresses answer "Ligando o app…" (auto-refresh, no wake)
// until Release: the server holds them while the supervisor reconciles, so
// the gateway never wakes an app the Hub is about to adopt.
func (g *Gateway) Hold() { g.held.Store(true) }

// Release lets app addresses through again.
func (g *Gateway) Release() { g.held.Store(false) }

// Page writes the gateway's plain page (or a short JSON error for API
// calls); the Hub uses it for full-page answers of /api/shared/authorize.
func Page(w http.ResponseWriter, r *http.Request, status int, title, message, hubURL string) {
	show(w, r, status, page{Title: title, Message: message, HubURL: hubURL})
}
