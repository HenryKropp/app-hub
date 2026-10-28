// Package gateway is the door of shared apps: it answers on each app's own
// address, checks the person with the Hub, wakes sleeping apps and passes
// requests (WebSocket included) to the app's container.
package gateway

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Identity is who the Hub says the person is.
type Identity struct {
	ID    int64
	Email string
	Name  string
	Role  string
}

// Target is what the gateway needs to know about an app address.
type Target struct {
	AppID     int64
	Name      string // nome do app
	Owner     string // nome de quem cuida
	Status    string // draft, starting, running, sleeping, crashed, stopped
	StoppedBy string
	Port      int // 0 = não há caixa para onde mandar
}

// Backend is what the Hub gives the gateway (implemented by the handler).
type Backend interface {
	Lookup(ctx context.Context, slug string, test bool) (Target, bool, error)
	Allowed(ctx context.Context, personID, appID int64, test bool) (Identity, bool, error) // test: pode publicar
	Wake(appID int64)
	Touch(appID int64)
}

const cookieName = "apphub_app"

type session struct {
	personID, appID int64
	test            bool
	id              Identity
	expires         time.Time
	checked         time.Time
	checking        bool // a recheck is already running
}

type Gateway struct {
	Codes   *Codes
	Recheck time.Duration
	TTL     time.Duration

	domain string
	hubURL string
	secure bool
	b      Backend
	now    func() time.Time
	held   atomic.Bool // Hold/Release

	mu       sync.Mutex
	sessions map[string]*session
}

func New(domain, hubURL string, secure bool, b Backend) *Gateway {
	return &Gateway{Codes: NewCodes(), Recheck: 5 * time.Minute, TTL: 12 * time.Hour,
		domain: domain, hubURL: strings.TrimRight(hubURL, "/"), secure: secure, b: b, now: time.Now,
		sessions: map[string]*session{}}
}

func (g *Gateway) SetNow(fn func() time.Time) { g.now = fn; g.Codes.now = fn }

func (g *Gateway) Match(host string) (string, bool, bool) {
	sub, ok := strings.CutSuffix(strings.ToLower(host), "."+g.domain)
	if !ok || sub == "" || strings.Contains(sub, ".") {
		return "", false, false
	}
	if slug, isTest := strings.CutSuffix(sub, "--teste"); isTest {
		return slug, true, slug != ""
	}
	return sub, false, true
}

// SafeVolta only lets same-site paths through: browsers drop tabs/newlines
// inside URLs, so "/\t/evil.com" would otherwise become "//evil.com".
func SafeVolta(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] < 0x20 || v[i] == 0x7f || v[i] == '\\' {
			return "/"
		}
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil ||
		!strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}
	return v
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, test, matched := g.Match(r.Host)
	if !matched {
		g.show(w, r, http.StatusNotFound, page{Title: "App não encontrado", Message: "Este endereço não é de nenhum app compartilhado do Hub.", HubURL: g.hubURL})
		return
	}
	if g.hubURL == "" {
		g.show(w, r, http.StatusServiceUnavailable, page{Title: "Endereços de apps não configurados", Message: "O Hub não sabe o próprio endereço (BASE_URL), então não consegue fazer você entrar no app. Peça ao administrador para configurá-lo."})
		return
	}
	if g.held.Load() {
		g.show(w, r, http.StatusServiceUnavailable, page{Title: "Ligando o app…", Message: "O Hub acabou de ligar. O app abre sozinho em alguns segundos.", Refresh: true})
		return
	}
	t, ok, err := g.b.Lookup(r.Context(), slug, test)
	if err != nil {
		slog.Error("gateway lookup", "slug", slug, "err", err)
		g.show(w, r, http.StatusInternalServerError, page{Title: "Algo deu errado", Message: "Tente de novo em instantes.", HubURL: g.hubURL})
		return
	}
	if !ok {
		g.show(w, r, http.StatusNotFound, page{Title: "App não encontrado", Message: "Este endereço não é de nenhum app compartilhado do Hub.", HubURL: g.hubURL})
		return
	}
	switch r.URL.Path {
	case "/__apphub/entrar":
		g.enter(w, r, t, slug, test)
		return
	case "/__apphub/sem-acesso":
		g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
		return
	}
	tok, s, found := g.session(r, t.AppID, test)
	if found {
		switch g.recheck(r.Context(), tok, t, test, false) {
		case verdictDenied:
			g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem mais acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
			return
		case verdictGone:
			found = false
		default:
			s, found = g.snapshot(tok)
		}
	}
	if !found {
		if !wantsHTML(r) {
			g.show(w, r, http.StatusUnauthorized, page{Title: "Entre pelo Hub", Message: "Abra o app pelo Hub."})
			return
		}
		q := url.Values{"app": {slug}, "volta": {r.URL.RequestURI()}}
		if test {
			q.Set("teste", "1")
		}
		http.Redirect(w, r, g.hubURL+"/api/shared/authorize?"+q.Encode(), http.StatusFound)
		return
	}
	if t.Port == 0 {
		g.notRunning(w, r, t, test)
		return
	}
	if !test {
		g.b.Touch(t.AppID)
	}
	g.proxy(w, r, t, tok, s)
}

func (g *Gateway) notRunning(w http.ResponseWriter, r *http.Request, t Target, test bool) {
	switch {
	case test:
		g.show(w, r, http.StatusNotFound, page{App: t.Name, Title: "O teste terminou", Message: "A caixa de teste foi desligada. Faça um teste novo pela ficha do app.", HubURL: g.hubURL})
	case t.Status == "stopped":
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "App parado", Message: "Parado por " + t.StoppedBy + ". Peça para " + owner(t) + " ligá-lo de novo.", HubURL: g.hubURL})
	case t.Status == "crashed":
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "App com problema", Message: "O app parou de responder e o Hub não conseguiu religá-lo. " + owner(t) + " já vê o aviso na ficha do app.", HubURL: g.hubURL})
	case t.Status == "draft":
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "Ainda não publicado", Message: "Este app ainda não foi publicado. Fale com " + owner(t) + ".", HubURL: g.hubURL})
	default: // sleeping, starting, running sem porta (Hub acabou de ligar)
		g.b.Wake(t.AppID)
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "Ligando o app…", Message: "O app estava desligado para economizar. Ele abre sozinho em alguns segundos.", Refresh: true})
	}
}

func (g *Gateway) enter(w http.ResponseWriter, r *http.Request, t Target, slug string, test bool) {
	personID, ok := g.Codes.Redeem(r.URL.Query().Get("code"), t.AppID, test)
	volta := SafeVolta(r.URL.Query().Get("volta"))
	if !ok {
		// código vencido ou já usado: passa pelo Hub de novo
		q := url.Values{"app": {slug}, "volta": {volta}}
		if test {
			q.Set("teste", "1")
		}
		http.Redirect(w, r, g.hubURL+"/api/shared/authorize?"+q.Encode(), http.StatusFound)
		return
	}
	id, allowed, err := g.b.Allowed(r.Context(), personID, t.AppID, test)
	if err != nil {
		slog.Error("gateway enter", "app", t.AppID, "err", err)
		g.show(w, r, http.StatusInternalServerError, page{App: t.Name, Title: "Algo deu errado", Message: "Tente de novo em instantes.", HubURL: g.hubURL})
		return
	}
	if !allowed {
		g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
		return
	}
	if !id.valid() {
		slog.Error("gateway enter: identity without id or e-mail", "app", t.AppID, "person", personID)
		g.show(w, r, http.StatusInternalServerError, page{App: t.Name, Title: "Algo deu errado", Message: "Tente de novo em instantes.", HubURL: g.hubURL})
		return
	}
	tok := token()
	now := g.now()
	g.mu.Lock()
	for k, s := range g.sessions {
		if now.After(s.expires) {
			delete(g.sessions, k)
		}
	}
	g.sessions[tok] = &session{personID: personID, appID: t.AppID, test: test, id: id, expires: now.Add(g.TTL), checked: now}
	g.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: tok, Path: "/", MaxAge: int(g.TTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: g.secure})
	http.Redirect(w, r, volta, http.StatusFound)
}

// session finds the first valid gateway cookie (a hostile sibling site could
// plant extra cookies of the same name) and returns a copy of its session.
func (g *Gateway) session(r *http.Request, appID int64, test bool) (string, session, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, c := range r.Cookies() {
		if c.Name != cookieName {
			continue
		}
		s := g.sessions[c.Value]
		if s == nil || s.appID != appID || s.test != test || g.now().After(s.expires) {
			continue
		}
		return c.Value, *s, true
	}
	return "", session{}, false
}

// snapshot copies the session; ok is false when it is gone (or carries no
// identity to give the app), and the person must enter again.
func (g *Gateway) snapshot(tok string) (session, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.sessions[tok]
	if s == nil || !s.id.valid() {
		return session{}, false
	}
	return *s, true
}

// valid: the app is only ever told about a real person.
func (id Identity) valid() bool { return id.ID != 0 && id.Email != "" }

type verdict int

const (
	verdictOK verdict = iota
	verdictDenied
	verdictGone
)

// recheck asks the Hub again whether the person still has access, at most
// every Recheck, with one question in flight per session. If the Hub cannot
// answer for longer than 2*Recheck the session is dropped (fail closed).
func (g *Gateway) recheck(ctx context.Context, tok string, t Target, test, force bool) verdict {
	g.mu.Lock()
	s := g.sessions[tok]
	now := g.now()
	if s == nil || now.After(s.expires) {
		delete(g.sessions, tok)
		g.mu.Unlock()
		return verdictGone
	}
	if (!force && now.Sub(s.checked) < g.Recheck) || s.checking {
		g.mu.Unlock()
		return verdictOK
	}
	s.checking = true
	personID := s.personID
	g.mu.Unlock()

	id, allowed, err := g.b.Allowed(ctx, personID, t.AppID, test)

	g.mu.Lock()
	defer g.mu.Unlock()
	s.checking = false
	if err != nil {
		slog.Error("gateway recheck", "app", t.AppID, "err", err)
		if g.now().Sub(s.checked) > 2*g.Recheck {
			delete(g.sessions, tok)
			return verdictGone
		}
		return verdictOK
	}
	if !allowed {
		delete(g.sessions, tok)
		return verdictDenied
	}
	s.id, s.checked = id, g.now()
	return verdictOK
}

// hijackTrack remembers the connection a WebSocket upgrade takes over, so the
// gateway can keep checking access and close it when access is gone.
type hijackTrack struct {
	http.ResponseWriter
	mu   sync.Mutex
	conn net.Conn
}

func (h *hijackTrack) Unwrap() http.ResponseWriter { return h.ResponseWriter }

func (h *hijackTrack) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := h.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	c, rw, err := hj.Hijack()
	if err == nil {
		h.mu.Lock()
		h.conn = c
		h.mu.Unlock()
	}
	return c, rw, err
}

func (h *hijackTrack) Flush() {
	if f, ok := h.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *hijackTrack) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conn != nil {
		_ = h.conn.Close()
	}
}

// watch runs while an upgraded connection is open: every Recheck it asks the
// Hub again (closing the connection if access is gone) and records use.
func (g *Gateway) watch(h *hijackTrack, done <-chan struct{}, tok string, t Target, test bool) {
	tk := time.NewTicker(g.Recheck)
	defer tk.Stop()
	for {
		select {
		case <-done:
			return
		case <-tk.C:
			if g.recheck(context.Background(), tok, t, test, true) != verdictOK {
				h.close()
				return
			}
			if !test {
				g.b.Touch(t.AppID)
			}
		}
	}
}

func (g *Gateway) proxy(w http.ResponseWriter, r *http.Request, t Target, tok string, s session) {
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(t.Port)}
	id, test := s.id, s.test
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = pr.In.Host // o app vê o próprio endereço
			pr.SetXForwarded()
			for k := range pr.Out.Header {
				if hubHeader(k) {
					delete(pr.Out.Header, k) // Del would canonicalize "X_AppHub_…" and miss it
				}
			}
			pr.Out.Header.Set("X-AppHub-User-Id", strconv.FormatInt(id.ID, 10))
			pr.Out.Header.Set("X-AppHub-User-Email", id.Email)
			pr.Out.Header.Set("X-AppHub-User-Name", url.PathEscape(id.Name)) // nomes com acento: o app decodifica com unquote
			pr.Out.Header.Set("X-AppHub-User-Role", url.PathEscape(id.Role))
			stripCookie(pr.Out)
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, context.Canceled) {
				return // the browser went away; nothing to wake or show
			}
			slog.Warn("gateway proxy", "app", t.AppID, "err", err)
			if test {
				g.show(w, r, http.StatusNotFound, page{App: t.Name, Title: "O teste terminou", Message: "A caixa de teste foi desligada. Faça um teste novo pela ficha do app.", HubURL: g.hubURL})
				return
			}
			g.b.Wake(t.AppID)
			g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "Ligando o app…", Message: "O app está reiniciando. Ele abre sozinho em alguns segundos.", Refresh: true})
		},
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		h := &hijackTrack{ResponseWriter: w}
		done := make(chan struct{})
		defer close(done)
		go g.watch(h, done, tok, t, test)
		rp.ServeHTTP(h, r)
		return
	}
	rp.ServeHTTP(w, r)
}

// hubHeader reports whether a header name is (or could pass for) one of the
// Hub's: case-insensitive, with "_" read as "-" (some servers do).
func hubHeader(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.ReplaceAll(name, "_", "-")), "x-apphub-")
}

// stripCookie removes the gateway cookie, keeping the app's own cookies.
func stripCookie(r *http.Request) {
	cs := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cs {
		if c.Name != cookieName {
			r.AddCookie(c)
		}
	}
}
