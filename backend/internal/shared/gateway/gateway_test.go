package gateway

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fakeBackend struct {
	target  Target
	allowed bool
	woken   []int64
	touched []int64
}

func (f *fakeBackend) Lookup(_ context.Context, slug string, test bool) (Target, bool, error) {
	if slug != "notas" {
		return Target{}, false, nil
	}
	return f.target, true, nil
}
func (f *fakeBackend) Allowed(_ context.Context, pid, aid int64, test bool) (Identity, bool, error) {
	return Identity{ID: pid, Email: "ana@alis.com", Name: "Ana", Role: "Usuário"}, f.allowed, nil
}
func (f *fakeBackend) Wake(id int64)  { f.woken = append(f.woken, id) }
func (f *fakeBackend) Touch(id int64) { f.touched = append(f.touched, id) }

// upstream echoes what the app receives.
func upstream(t *testing.T) (*httptest.Server, int) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s email=%s cookie=%s host=%s", r.URL.RequestURI(), r.Header.Get("X-AppHub-User-Email"), r.Header.Get("Cookie"), r.Host)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return srv, port
}

func setup(t *testing.T) (*Gateway, *fakeBackend) {
	_, port := upstream(t)
	b := &fakeBackend{target: Target{AppID: 7, Name: "Notas", Owner: "Henry", Status: "running", Port: port}, allowed: true}
	return New("localhost:8080", "http://localhost:5173", false, b), b
}

func get(g *Gateway, host, path string, cookie *http.Cookie, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://"+host+path, nil)
	r.Host = host
	r.Header.Set("Accept", "text/html")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	return w
}

// enter does the handshake and returns the app cookie.
func enter(t *testing.T, g *Gateway, host string, test bool) *http.Cookie {
	t.Helper()
	code := g.Codes.Issue(1, 7, test)
	w := get(g, host, "/__apphub/entrar?code="+code+"&volta=/notas?x=1", nil, nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/notas?x=1" {
		t.Fatalf("enter %d %s", w.Code, w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "apphub_app" && c.HttpOnly {
			return c
		}
	}
	t.Fatal("no cookie")
	return nil
}

func TestMatch(t *testing.T) {
	g, _ := setup(t)
	cases := map[string][3]any{
		"notas.localhost:8080":        {"notas", false, true},
		"notas--teste.localhost:8080": {"notas", true, true},
		"localhost:8080":              {"", false, false},
		"a.b.localhost:8080":          {"", false, false},
		"notas.localhost:5173":        {"", false, false},
	}
	for host, want := range cases {
		slug, test, ok := g.Match(host)
		if slug != want[0] || test != want[1] || ok != want[2] {
			t.Errorf("%s: %q %v %v", host, slug, test, ok)
		}
	}
}

func TestNoSessionRedirectsToHub(t *testing.T) {
	g, _ := setup(t)
	w := get(g, "notas.localhost:8080", "/pagina?a=1", nil, nil)
	want := "http://localhost:5173/api/shared/authorize?app=notas&volta=%2Fpagina%3Fa%3D1"
	if w.Code != http.StatusFound || w.Header().Get("Location") != want {
		t.Fatalf("%d %s", w.Code, w.Header().Get("Location"))
	}
	w = get(g, "notas--teste.localhost:8080", "/", nil, nil)
	if !strings.Contains(w.Header().Get("Location"), "teste=1") {
		t.Fatalf("test host: %s", w.Header().Get("Location"))
	}
}

func TestEnterThenProxies(t *testing.T) {
	g, b := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	w := get(g, "notas.localhost:8080", "/notas?x=1", c, nil)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "path=/notas?x=1") || !strings.Contains(body, "email=ana@alis.com") {
		t.Fatalf("%d %s", w.Code, body)
	}
	if strings.Contains(body, "apphub_app") {
		t.Fatal("the app must not see the gateway cookie")
	}
	if !strings.Contains(body, "host=notas.localhost:8080") {
		t.Fatal("the app should see its own address as Host")
	}
	if len(b.touched) == 0 {
		t.Fatal("access not recorded")
	}
}

func TestGatewayStripsForgedHeaders(t *testing.T) {
	g, _ := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	w := get(g, "notas.localhost:8080", "/", c, map[string]string{"X-AppHub-User-Email": "chefe@alis.com", "X-Apphub-User-Role": "Administrador"})
	if !strings.Contains(w.Body.String(), "email=ana@alis.com") {
		t.Fatalf("%s", w.Body.String())
	}
	// sem sessão, o cabeçalho forjado também não passa: vai para o Hub
	if w := get(g, "notas.localhost:8080", "/", nil, map[string]string{"X-AppHub-User-Email": "chefe@alis.com"}); w.Code != http.StatusFound {
		t.Fatalf("forged header without session got %d", w.Code)
	}
}

func TestCodeSingleUseAndExpires(t *testing.T) {
	g, _ := setup(t)
	code := g.Codes.Issue(1, 7, false)
	if _, ok := g.Codes.Redeem(code, 8, false); ok {
		t.Fatal("code for app 7 worked on app 8")
	}
	if _, ok := g.Codes.Redeem(code, 7, false); !ok {
		t.Fatal("valid code refused")
	}
	if _, ok := g.Codes.Redeem(code, 7, false); ok {
		t.Fatal("code used twice")
	}
	now := time.Now()
	g.Codes.now = func() time.Time { return now }
	old := g.Codes.Issue(1, 7, false)
	g.Codes.now = func() time.Time { return now.Add(61 * time.Second) }
	if _, ok := g.Codes.Redeem(old, 7, false); ok {
		t.Fatal("expired code worked")
	}
}

func TestUnsafeVoltaGoesHome(t *testing.T) {
	g, _ := setup(t)
	for _, volta := range []string{"//evil.com/x", "https://evil.com", "/\\evil.com"} {
		code := g.Codes.Issue(1, 7, false)
		w := get(g, "notas.localhost:8080", "/__apphub/entrar?code="+code+"&volta="+url.QueryEscape(volta), nil, nil)
		if w.Header().Get("Location") != "/" {
			t.Errorf("%s → %s", volta, w.Header().Get("Location"))
		}
	}
}

func TestGatewayRechecksAccess(t *testing.T) {
	g, b := setup(t)
	now := time.Now()
	g.SetNow(func() time.Time { return now })
	c := enter(t, g, "notas.localhost:8080", false)
	b.allowed = false // a pessoa perdeu o acesso
	if w := get(g, "notas.localhost:8080", "/", c, nil); w.Code != 200 {
		t.Fatalf("within 5 min should still pass, got %d", w.Code)
	}
	now = now.Add(5*time.Minute + time.Second)
	w := get(g, "notas.localhost:8080", "/", c, nil)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "Sem acesso") || !strings.Contains(w.Body.String(), "Henry") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// e a sessão morreu: mesmo voltando o acesso, passa pelo Hub de novo
	b.allowed = true
	if w := get(g, "notas.localhost:8080", "/", c, nil); w.Code != http.StatusFound {
		t.Fatalf("dead session got %d", w.Code)
	}
}

func TestCookieOfOtherAppDoesNotWork(t *testing.T) {
	g, _ := setup(t)
	c := enter(t, g, "notas--teste.localhost:8080", true) // sessão da caixa de teste
	if w := get(g, "notas.localhost:8080", "/", c, nil); w.Code != http.StatusFound {
		t.Fatalf("test session opened the live app: %d", w.Code)
	}
}

func TestSleepingWakes(t *testing.T) {
	g, b := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	b.target.Status, b.target.Port = "sleeping", 0
	w := get(g, "notas.localhost:8080", "/", c, nil)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Ligando o app") ||
		!strings.Contains(w.Body.String(), `http-equiv="refresh"`) || len(b.woken) != 1 {
		t.Fatalf("%d %s woken=%v", w.Code, w.Body.String(), b.woken)
	}
	r := httptest.NewRequest("GET", "http://notas.localhost:8080/api/dados", nil)
	r.Host = "notas.localhost:8080"
	r.Header.Set("Accept", "application/json")
	r.AddCookie(c)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, r)
	if rec.Code != 503 || rec.Header().Get("Retry-After") == "" || strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("api while sleeping: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStoppedAndCrashedPages(t *testing.T) {
	g, b := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	b.target.Status, b.target.Port, b.target.StoppedBy = "stopped", 0, "Maria"
	if w := get(g, "notas.localhost:8080", "/", c, nil); !strings.Contains(w.Body.String(), "Parado por Maria") {
		t.Fatalf("%s", w.Body.String())
	}
	b.target.Status = "crashed"
	if w := get(g, "notas.localhost:8080", "/", c, nil); !strings.Contains(w.Body.String(), "App com problema") || len(b.woken) != 0 {
		t.Fatalf("%s woken=%v", w.Body.String(), b.woken)
	}
}

func TestUnknownApp(t *testing.T) {
	g, _ := setup(t)
	if w := get(g, "outro.localhost:8080", "/", nil, nil); w.Code != 404 || !strings.Contains(w.Body.String(), "não encontrado") {
		t.Fatalf("%d", w.Code)
	}
}

func TestWebSocketPassesThrough(t *testing.T) {
	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			w.WriteHeader(400)
			return
		}
		conn, buf, _ := w.(http.Hijacker).Hijack()
		defer conn.Close()
		buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		buf.Flush()
		line, _ := buf.ReadString('\n')
		buf.WriteString("eco:" + line)
		buf.Flush()
	}))
	defer ws.Close()
	u, _ := url.Parse(ws.URL)
	port, _ := strconv.Atoi(u.Port())
	b := &fakeBackend{target: Target{AppID: 7, Status: "running", Port: port}, allowed: true}
	g := New("localhost:8080", "http://localhost:5173", false, b)
	c := enter(t, g, "notas.localhost:8080", false)
	front := httptest.NewServer(g)
	defer front.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(front.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET /_stcore/stream HTTP/1.1\r\nHost: notas.localhost:8080\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nCookie: apphub_app=%s\r\n\r\n", c.Value)
	rd := bufio.NewReader(conn)
	status, _ := rd.ReadString('\n')
	if !strings.Contains(status, "101") {
		t.Fatalf("status %q", status)
	}
	for l, _ := rd.ReadString('\n'); l != "\r\n"; l, _ = rd.ReadString('\n') {
	}
	fmt.Fprint(conn, "oi\n")
	got, _ := rd.ReadString('\n')
	if got != "eco:oi\n" {
		t.Fatalf("got %q", got)
	}
}

func TestCookieFlags(t *testing.T) {
	_, port := upstream(t)
	b := &fakeBackend{target: Target{AppID: 7, Status: "running", Port: port}, allowed: true}
	for _, secure := range []bool{false, true} {
		g := New("localhost:8080", "http://localhost:5173", secure, b)
		c := enter(t, g, "notas.localhost:8080", false)
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure != secure || c.Domain != "" || c.MaxAge != 12*3600 || c.Path != "/" {
			t.Errorf("secure=%v cookie %+v", secure, c)
		}
	}
}

func TestEnterWithBadCodeGoesToHubAndSetsNoCookie(t *testing.T) {
	g, _ := setup(t)
	w := get(g, "notas--teste.localhost:8080", "/__apphub/entrar?code=nada&volta=/x", nil, nil)
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.Contains(loc, "authorize?app=notas&") || !strings.Contains(loc, "teste=1") || len(w.Result().Cookies()) != 0 {
		t.Fatalf("%d %s", w.Code, loc)
	}
}

func TestEnterRefusedWhenNotAllowed(t *testing.T) {
	g, b := setup(t)
	b.allowed = false
	code := g.Codes.Issue(1, 7, false)
	w := get(g, "notas.localhost:8080", "/__apphub/entrar?code="+code, nil, nil)
	if w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0 {
		t.Fatalf("%d", w.Code)
	}
}

func TestSessionExpiresAfterTTL(t *testing.T) {
	g, _ := setup(t)
	now := time.Now()
	g.SetNow(func() time.Time { return now })
	c := enter(t, g, "notas.localhost:8080", false)
	now = now.Add(12*time.Hour + time.Second)
	if w := get(g, "notas.localhost:8080", "/", c, nil); w.Code != http.StatusFound {
		t.Fatalf("expired session got %d", w.Code)
	}
}

func TestReservedPathsNeverReachTheApp(t *testing.T) {
	g, _ := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	w := get(g, "notas.localhost:8080", "/__apphub/sem-acesso", c, nil)
	if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "path=") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
