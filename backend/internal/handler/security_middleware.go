package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type rateWindow struct {
	start time.Time
	count int
}
type securityLimiter struct {
	mu      sync.Mutex
	entries map[string]rateWindow
}

const (
	rateWindowLen   = 15 * time.Minute
	rateMaxKeys     = 4096
	accountFailures = 10  // wrong passwords per e-mail and route
	addressAttempts = 300 // any attempt per network address; people often share one behind a proxy
	signupAttempts  = 20  // sign-ups and setup per network address
)

// over reports whether key already used its limit in the current window.
func (l *securityLimiter) over(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.entries[key]
	return ok && now.Sub(v.start) < rateWindowLen && v.count >= limit
}

// add counts one attempt for key. When the table is full it drops expired
// windows and then the oldest one, so attacker-made keys never lock others out.
func (l *securityLimiter) add(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = map[string]rateWindow{}
	}
	if _, ok := l.entries[key]; !ok && len(l.entries) >= rateMaxKeys {
		oldest, first := "", true
		for k, v := range l.entries {
			if now.Sub(v.start) >= rateWindowLen {
				delete(l.entries, k)
				continue
			}
			if first || v.start.Before(l.entries[oldest].start) {
				oldest, first = k, false
			}
		}
		if len(l.entries) >= rateMaxKeys {
			delete(l.entries, oldest)
		}
	}
	v := l.entries[key]
	if now.Sub(v.start) >= rateWindowLen {
		v = rateWindow{start: now}
	}
	v.count++
	l.entries[key] = v
}

// allow counts one attempt and reports whether it is within limit.
func (l *securityLimiter) allow(key string, limit int, now time.Time) bool {
	l.add(key, now)
	return !l.over(key, limit+1, now)
}

func rateKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// loginEmail reads the e-mail from a login body and puts the body back.
func loginEmail(r *http.Request) string {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var b struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(body, &b)
	return strings.ToLower(strings.TrimSpace(b.Email))
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(n int) {
	if w.status == 0 {
		w.status = n
		w.ResponseWriter.WriteHeader(n)
	}
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}

// Secure adds workspace-local defenses. Authentication/authorization remain in
// each route, including every agent endpoint; URL workspace selection is NOT auth.
func (h *Handler) Secure(next http.Handler) http.Handler {
	limiter := &securityLimiter{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		NoFraming(w.Header())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/oauth/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		unsafe := r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS"
		isAgent := strings.HasPrefix(r.URL.Path, "/api/agent/")
		// MCP and the OAuth token/registration endpoints are called by Claude,
		// not a browser session: a bearer token or PKCE protects them, no cookie.
		isClaude := r.URL.Path == "/mcp" || r.URL.Path == "/oauth/token" || r.URL.Path == "/oauth/register"
		if unsafe && !isAgent && !isClaude {
			origin := r.Header.Get("Origin")
			if origin != "" {
				want, err := url.Parse(h.cfg.BaseURL)
				got, e := url.Parse(origin)
				if err != nil || e != nil || got.Scheme != want.Scheme || !strings.EqualFold(got.Host, want.Host) || got.User != nil || got.Path != "" {
					writeError(w, 403, "Origem da solicitação não autorizada.")
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeError(w, 403, "Solicitação externa bloqueada.")
				return
			}
			if r.ContentLength != 0 {
				ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if ct != "application/json" {
					writeError(w, 415, "Envie dados no formato JSON.")
					return
				}
			}
		}
		limited := unsafe && (r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/setup" || r.URL.Path == "/api/workspaces" || strings.HasPrefix(r.URL.Path, "/api/entry/") || r.URL.Path == "/api/agent/pair/start" || r.URL.Path == "/oauth/register" || r.URL.Path == "/oauth/token")
		// /oauth/authorize is a GET with no login that writes a pending row, so it
		// is limited per address too.
		limited = limited || r.URL.Path == "/oauth/authorize"
		accountKey := ""
		if limited {
			// Never trust arbitrary X-Forwarded-For supplied by clients.
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			limit := signupAttempts
			if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/entry/invitations/preview" || r.URL.Path == "/api/agent/pair/start" || r.URL.Path == "/oauth/token" || r.URL.Path == "/oauth/authorize" {
				limit = addressAttempts
			}
			if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/entry/login" {
				// Wrong passwords count per account, so one person mistyping
				// does not lock out everyone behind the same proxy.
				accountKey = rateKey("account", loginEmail(r), r.URL.Path)
			}
			if !limiter.allow(rateKey(host, r.URL.Path), limit, time.Now()) || (accountKey != "" && limiter.over(accountKey, accountFailures, time.Now())) {
				w.Header().Set("Retry-After", "900")
				writeError(w, 429, "Muitas tentativas. Aguarde alguns minutos e tente novamente.")
				return
			}
		}
		if accountKey != "" {
			sw := &statusWriter{ResponseWriter: w}
			defer func() {
				if sw.status == http.StatusUnauthorized {
					limiter.add(accountKey, time.Now())
				}
			}()
			w = sw
		}
		if h.workspace != nil && ((r.URL.Path == "/api/scan" && r.Method == "POST") || (r.URL.Path == "/api/apps" && r.Method == "POST")) {
			writeError(w, 403, "No portal multiempresa, importe os arquivos pelo navegador. A leitura de pastas do servidor foi desativada.")
			return
		}
		audit := (unsafe && !isAgent && strings.HasPrefix(r.URL.Path, "/api/")) || r.URL.Path == "/api/security/export"
		if !audit {
			next.ServeHTTP(w, r)
			return
		}
		var actor *int64
		if c, err := r.Cookie(cookieName); err == nil {
			if p, err := h.q.GetSessionPerson(r.Context(), c.Value); err == nil {
				id := p.ID
				actor = &id
			}
		}
		// Anonymous calls are audited only on the rate-limited entry routes;
		// every other route refuses them, and auditing them let anyone grow
		// the table without limit and push real events out of view.
		if actor == nil && !limited {
			next.ServeHTTP(w, r)
			return
		}
		// Persist the attempt before allowing state changes. No body, query,
		// cookie, password, token, domain or source text enters this record.
		eventID, err := h.auditStart(r.Context(), actor, "request.started")
		if err != nil {
			writeError(w, 503, "Não foi possível registrar a operação com segurança.")
			return
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		status := sw.status
		if status == 0 {
			status = 200
		}
		// r.Pattern is a server-defined route template, never a raw URL.
		h.auditFinish(r.Context(), eventID, "request.completed", r.Pattern, status)
	})
}

// auditStart records an attempt before it may change anything and returns
// the event id. No body, query, cookie, password, token or source text goes in.
func (h *Handler) auditStart(ctx context.Context, actor *int64, action string) (int64, error) {
	var eventID int64
	err := h.q.SecurityDB().QueryRow(ctx, `INSERT INTO security_audit(actor_id,action) VALUES($1,$2) RETURNING id`, actor, action).Scan(&eventID)
	return eventID, err
}

// auditFinish completes the event with the server-defined route template
// (never a raw URL) and the answer's status. A failure is only logged.
func (h *Handler) auditFinish(ctx context.Context, eventID int64, action, route string, status int) {
	if _, err := h.q.SecurityDB().Exec(ctx, `UPDATE security_audit SET action=$2,route=$3,status=$4 WHERE id=$1`, eventID, action, route, status); err != nil {
		slog.Error("audit completion failed", "event_id", eventID)
	}
}
