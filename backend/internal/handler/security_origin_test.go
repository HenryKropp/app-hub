package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/config"
)

// Behind Caddy every request comes from loopback; the limit must follow the
// visitor's real address (X-Real-IP), so one attacker cannot lock out everyone.
func TestSecureLimitsPerRealAddress(t *testing.T) {
	h := New(nil, config.Config{BaseURL: "http://hub.test"})
	sec := h.Secure(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	call := func(remote, realIP string) int {
		req := httptest.NewRequest("POST", "/oauth/register", strings.NewReader("{}"))
		req.RemoteAddr = remote
		req.Header.Set("Content-Type", "application/json")
		if realIP != "" {
			req.Header.Set("X-Real-IP", realIP)
		}
		rec := httptest.NewRecorder()
		sec.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < signupAttempts; i++ {
		if got := call("127.0.0.1:5000", "203.0.113.9"); got != http.StatusOK {
			t.Fatalf("attempt %d = %d, want 200", i+1, got)
		}
	}
	if got := call("127.0.0.1:5000", "203.0.113.9"); got != http.StatusTooManyRequests {
		t.Fatalf("over the limit = %d, want 429", got)
	}
	if got := call("127.0.0.1:5000", "203.0.113.10"); got != http.StatusOK {
		t.Fatalf("another visitor behind Caddy = %d, want 200", got)
	}
	// Straight to the server (not loopback): a made-up X-Real-IP is ignored,
	// so changing it does not buy fresh attempts.
	for i := 0; i < signupAttempts; i++ {
		call("198.51.100.7:4000", "192.0.2.1")
	}
	if got := call("198.51.100.7:4000", "192.0.2.2"); got != http.StatusTooManyRequests {
		t.Fatalf("spoofed X-Real-IP from outside = %d, want 429", got)
	}
}

func TestAuditOrigin(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/apps", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-Real-IP", "203.0.113.9")
	r.Header.Set("User-Agent", strings.Repeat("a", 400))
	ip, ua := auditOrigin(r)
	if ip != "203.0.113.9" || len([]rune(ua)) != 300 {
		t.Fatalf("auditOrigin = %q, %d runes", ip, len([]rune(ua)))
	}
}

// Every recorded action keeps the address and browser name it came from.
func TestAuditRecordsOrigin(t *testing.T) {
	_, h := newPoolClient(t)
	r := httptest.NewRequest("POST", "/api/artifacts", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-Real-IP", "203.0.113.9")
	r.Header.Set("User-Agent", "Navegador/1.0")
	id, err := h.auditStart(withAuditOrigin(context.Background(), r), nil, "request.started")
	if err != nil {
		t.Fatal(err)
	}
	var ip, ua string
	if err := testPool.QueryRow(context.Background(), `SELECT ip,user_agent FROM security_audit WHERE id=$1`, id).Scan(&ip, &ua); err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.9" || ua != "Navegador/1.0" {
		t.Fatalf("ip %q, user_agent %q", ip, ua)
	}
	var e []auditEvent
	if e, err = h.auditEvents(context.Background()); err != nil || len(e) == 0 || e[0].IP != "203.0.113.9" {
		t.Fatalf("export %v %v", e, err)
	}
}
