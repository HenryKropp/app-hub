package handler

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// remoteHost is the visitor's address. Behind Caddy every request comes from
// loopback, so there the address Caddy puts in X-Real-IP is used; from
// anywhere else the header is ignored (anyone could send it).
func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(real) != nil {
			return real
		}
	}
	return host
}

const auditUserAgentMax = 300

// auditOrigin is what the security record keeps about where a request came
// from: the real address and the browser's name, cut to 300 characters.
func auditOrigin(r *http.Request) (ip, userAgent string) {
	ua := []rune(r.UserAgent())
	if len(ua) > auditUserAgentMax {
		ua = ua[:auditUserAgentMax]
	}
	return remoteHost(r), string(ua)
}

type auditOriginKey struct{}
type auditOriginValue struct{ ip, userAgent string }

// withAuditOrigin keeps the request's origin in its context, so every
// security record written while serving it (also from MCP tools) carries it.
func withAuditOrigin(ctx context.Context, r *http.Request) context.Context {
	ip, ua := auditOrigin(r)
	return context.WithValue(ctx, auditOriginKey{}, auditOriginValue{ip: ip, userAgent: ua})
}
