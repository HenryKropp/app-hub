// Package gateway is the door of shared apps: it answers on each app's own
// address, checks the person with the Hub, wakes sleeping apps and passes
// requests (WebSocket included) to the app's container.
package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
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

func safeVolta(v string) string {
	if !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.HasPrefix(v, "/\\") || strings.ContainsAny(v, "\\\r\n") {
		return "/"
	}
	return v
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, test, _ := g.Match(r.Host)
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
	s := g.session(r, t.AppID, test)
	if s == nil {
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
	if g.now().Sub(s.checked) >= g.Recheck {
		id, allowed, err := g.b.Allowed(r.Context(), s.personID, t.AppID, test)
		if err != nil {
			slog.Error("gateway recheck", "app", t.AppID, "err", err)
		} else if !allowed {
			g.drop(r)
			g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem mais acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
			return
		} else {
			g.mu.Lock()
			s.id, s.checked = id, g.now()
			g.mu.Unlock()
		}
	}
	if t.Port == 0 {
		g.notRunning(w, r, t, test)
		return
	}
	if !test {
		g.b.Touch(t.AppID)
	}
	g.proxy(w, r, t, s)
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
	volta := safeVolta(r.URL.Query().Get("volta"))
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
	if err != nil || !allowed {
		g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
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

func (g *Gateway) session(r *http.Request, appID int64, test bool) *session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.sessions[c.Value]
	if s == nil || s.appID != appID || s.test != test || g.now().After(s.expires) {
		return nil
	}
	return s
}

func (g *Gateway) drop(r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		g.mu.Lock()
		delete(g.sessions, c.Value)
		g.mu.Unlock()
	}
}

func (g *Gateway) proxy(w http.ResponseWriter, r *http.Request, t Target, s *session) {
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(t.Port)}
	g.mu.Lock()
	id := s.id
	g.mu.Unlock()
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = pr.In.Host // o app vê o próprio endereço
			pr.SetXForwarded()
			for k := range pr.Out.Header {
				if strings.HasPrefix(strings.ToLower(k), "x-apphub-") {
					pr.Out.Header.Del(k)
				}
			}
			pr.Out.Header.Set("X-AppHub-User-Id", strconv.FormatInt(id.ID, 10))
			pr.Out.Header.Set("X-AppHub-User-Email", id.Email)
			pr.Out.Header.Set("X-AppHub-User-Name", url.QueryEscape(id.Name)) // nomes com acento: o app decodifica
			pr.Out.Header.Set("X-AppHub-User-Role", url.QueryEscape(id.Role))
			stripCookie(pr.Out)
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("gateway proxy", "app", t.AppID, "err", err)
			g.b.Wake(t.AppID)
			g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "Ligando o app…", Message: "O app está reiniciando. Ele abre sozinho em alguns segundos.", Refresh: true})
		},
	}
	rp.ServeHTTP(w, r)
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
