package handler

import (
	"net/http"

	"github.com/HenryKropp/app-hub/backend/internal/config"
)

// StrictTransport tells browsers to reach this Ziv over HTTPS only for a year
// (HSTS), when it is served over HTTPS. Subdomains are left out: a company's
// app addresses get their own certificate on demand.
func StrictTransport(cfg config.Config, next http.Handler) http.Handler {
	if !cfg.SecureCookies() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}

// siteCSP is the policy of the Ziv site (React): only its own scripts run.
// Styles allow inline (React style props); frames may show https dashboards
// (link artifacts) and the Ziv's own artifact pages.
const siteCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'self' https:; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// SiteCSP sets the site's Content-Security-Policy, replacing the framing-only
// one NoFraming sets (it keeps frame-ancestors 'none').
func SiteCSP(h http.Header) { h.Set("Content-Security-Policy", siteCSP) }
