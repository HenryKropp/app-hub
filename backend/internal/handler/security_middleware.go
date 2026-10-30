package handler

import (
	"crypto/sha256"
	"encoding/hex"
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

func (l *securityLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = map[string]rateWindow{}
	}
	// Bound attacker-controlled keys and discard expired windows.
	if len(l.entries) >= 4096 {
		for k, v := range l.entries {
			if now.Sub(v.start) >= 15*time.Minute {
				delete(l.entries, k)
			}
		}
		if len(l.entries) >= 4096 {
			return false
		}
	}
	v := l.entries[key]
	if now.Sub(v.start) >= 15*time.Minute {
		v = rateWindow{start: now}
	}
	v.count++
	l.entries[key] = v
	return v.count <= 20
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
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		unsafe := r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS"
		isAgent := strings.HasPrefix(r.URL.Path, "/api/agent/")
		if unsafe && !isAgent {
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
if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/setup" || r.URL.Path == "/api/workspaces" || strings.HasPrefix(r.URL.Path, "/api/entry/") || r.URL.Path == "/api/agent/pair/start" {
			// Never trust arbitrary X-Forwarded-For supplied by clients.
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			sum := sha256.Sum256([]byte(host + "|" + r.URL.Path))
			if !limiter.allow(hex.EncodeToString(sum[:]), time.Now()) {
				w.Header().Set("Retry-After", "900")
				writeError(w, 429, "Muitas tentativas. Aguarde alguns minutos e tente novamente.")
				return
			}
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
		// Persist the attempt before allowing state changes. No body, query,
		// cookie, password, token, domain or source text enters this record.
		var eventID int64
		if err := h.q.SecurityDB().QueryRow(r.Context(), `INSERT INTO security_audit(actor_id,action) VALUES($1,$2) RETURNING id`, actor, "request.started").Scan(&eventID); err != nil {
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
		if _, err := h.q.SecurityDB().Exec(r.Context(), `UPDATE security_audit SET action='request.completed',route=$2,status=$3 WHERE id=$1`, eventID, r.Pattern, status); err != nil {
			slog.Error("audit completion failed", "event_id", eventID)
		}
	})
}
