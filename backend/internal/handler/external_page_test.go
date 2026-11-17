package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/extlink"
)

// outside is a browser of someone outside the company, talking to the link
// address of h (path /<slug>/<token>[/<action>]).
type outside struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
}

func newOutside(t *testing.T, h *Handler) *outside {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug, token, action, ok := SplitLinkPath(r.URL.Path)
		if !ok {
			ExternalUnavailable(w, h.homeURL())
			return
		}
		h.ServeExternal(w, r, slug, token, action)
	}))
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	return &outside{t: t, srv: srv, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (o *outside) get(path string) (int, http.Header, string) {
	o.t.Helper()
	resp, err := o.c.Get(o.srv.URL + path)
	if err != nil {
		o.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func (o *outside) post(path string, form url.Values) (int, http.Header, string) {
	o.t.Helper()
	resp, err := o.c.PostForm(o.srv.URL+path, form)
	if err != nil {
		o.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

// makeLink creates a link as c and returns its id and path on the link address.
func makeLink(t *testing.T, c *client, artifactID int64, body map[string]any) (int64, string) {
	t.Helper()
	var l linkJSON
	if code := c.do("POST", linksPath(artifactID), body, &l); code != http.StatusCreated {
		t.Fatalf("create link = %d", code)
	}
	u, err := url.Parse(l.URL)
	if err != nil {
		t.Fatal(err)
	}
	return l.ID, u.Path
}

func countOpens(t *testing.T, linkID int64) (n int, email string) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*), COALESCE(max(email),'') FROM external_link_opens WHERE link_id=$1`, linkID).Scan(&n, &email); err != nil {
		t.Fatal(err)
	}
	return
}

const unavailableText = "Este link não está mais disponível"

func TestOpenLinkServesPageAndContent(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.cfg.BaseURL = "https://ziv.test/w/piloto"
	h.workspace = &WorkspaceOptions{Slug: "piloto", Name: "Empresa Piloto"}
	linkID, path := makeLink(t, ana, id, openLinkBody())
	o := newOutside(t, h)
	code, hd, body := o.get(path)
	if code != http.StatusOK || !strings.Contains(body, "Empresa Piloto compartilhou com você") ||
		!strings.Contains(body, "Relatório mensal") || !strings.Contains(body, `href="https://ziv.test/"`) ||
		!strings.Contains(body, `src="`+path+`/conteudo"`) || strings.Contains(body, "<script") {
		t.Fatalf("page %d %s", code, body)
	}
	if csp := hd.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("page csp %q", csp)
	}
	if hd.Get("Referrer-Policy") != "same-origin" {
		t.Fatal("token would leak in Referer (or the browser would post Origin: null)")
	}
	if n, _ := countOpens(t, linkID); n != 0 {
		t.Fatal("the page itself is not an opening")
	}
	code, hd, body = o.get(path + "/conteudo")
	csp := hd.Get("Content-Security-Policy")
	if code != http.StatusOK || body != pageHTML || !strings.Contains(csp, "sandbox") || strings.Contains(csp, "allow-same-origin") {
		t.Fatalf("content %d %q csp %q", code, body, csp)
	}
	if n, email := countOpens(t, linkID); n != 1 || email != "" {
		t.Fatalf("opens %d %q", n, email)
	}
}

func TestLatestFollowsNewVersion(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	_, path := makeLink(t, ana, id, openLinkBody())
	ana.do("POST", artifactPath(id, "/versions"), fileBody("Relatório mensal", "rel.html", []byte("<p>v2</p>")), nil)
	if _, _, body := newOutside(t, h).get(path + "/conteudo"); body != "<p>v2</p>" {
		t.Fatalf("latest shows %q", body)
	}
}

func TestPinnedVersionStays(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	body := openLinkBody()
	body["content"], body["version"] = "version", 1
	_, path := makeLink(t, ana, id, body)
	ana.do("POST", artifactPath(id, "/versions"), fileBody("Relatório mensal", "rel.html", []byte("<p>v2</p>")), nil)
	if _, _, got := newOutside(t, h).get(path + "/conteudo"); got != pageHTML {
		t.Fatalf("pinned shows %q", got)
	}
}

func TestClosedLinkReasonsAllLookTheSame(t *testing.T) {
	admin, ana, _, h, id := linksTeam(t)
	o := newOutside(t, h)
	var bodies []string
	check := func(name, path string) {
		t.Helper()
		for _, p := range []string{path, path + "/conteudo"} {
			code, _, body := o.get(p)
			if code != http.StatusNotFound || !strings.Contains(body, unavailableText) {
				t.Fatalf("%s %s: %d %s", name, p, code, body)
			}
			bodies = append(bodies, body)
		}
	}
	cancelledID, cancelled := makeLink(t, ana, id, openLinkBody())
	ana.do("POST", fmt.Sprintf("/api/links/%d/cancel", cancelledID), nil, nil)
	check("cancelled", cancelled)

	expiredID, expired := makeLink(t, ana, id, openLinkBody())
	exec(t, "UPDATE external_links SET expires_at = now() - interval '1 second' WHERE id=$1", expiredID)
	check("expired", expired)

	check("unknown token", "/principal/naoexiste")

	_, other := makeLink(t, ana, id, openLinkBody())
	check("other company in path", strings.Replace(other, "/principal/", "/outra/", 1))

	_, lostShare := makeLink(t, ana, id, openLinkBody())
	setRole(t, "criador", "register_apps")
	check("creator lost share_external", lostShare)
	setRole(t, "criador", "register_apps", "share_external")

	var henry personJSON
	admin.do("GET", "/api/auth/status", nil, &henry)
	_, gone := makeLink(t, admin, id, openLinkBody())
	exec(t, "UPDATE external_links SET created_by = NULL WHERE token = $1", gone[strings.LastIndex(gone, "/")+1:])
	check("creator gone", gone)

	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Fatalf("closed pages differ:\n%s\n---\n%s", bodies[0], b)
		}
	}
}

func emailLinkBody() map[string]any {
	return map[string]any{"kind": "email", "email": "henry@cliente.com", "content": "latest", "expires_days": 30}
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

func TestPageMasksEmail(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	_, path := makeLink(t, ana, id, emailLinkBody())
	code, _, body := newOutside(t, h).get(path)
	if code != http.StatusOK || !strings.Contains(body, "h***@cliente.com") || strings.Contains(body, "henry@") || !strings.Contains(body, "Enviar código") {
		t.Fatalf("page %d %s", code, body)
	}
}

func TestEmailLinkCodeFlow(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	linkID, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	if code, _, _ := o.get(path + "/conteudo"); code != http.StatusForbidden {
		t.Fatalf("content without code = %d", code)
	}
	if code, _, body := o.post(path+"/codigo", nil); code != http.StatusOK || !strings.Contains(body, `name="codigo"`) {
		t.Fatalf("ask code %d %s", code, body)
	}
	if len(fm.sent) != 1 || fm.sent[0].to != "henry@cliente.com" || !strings.Contains(fm.sent[0].subject, "Relatório mensal") {
		t.Fatalf("sent %+v", fm.sent)
	}
	right := sixDigits.FindString(fm.sent[0].text)
	wrong := "000000"
	if right == wrong {
		wrong = "111111"
	}
	if code, _, body := o.post(path+"/confirmar", url.Values{"codigo": {wrong}}); code != http.StatusBadRequest || !strings.Contains(body, "Código errado") {
		t.Fatalf("wrong code %d %s", code, body)
	}
	code, hd, _ := o.post(path+"/confirmar", url.Values{"codigo": {right}})
	if code != http.StatusSeeOther {
		t.Fatalf("right code = %d", code)
	}
	cookie := hd.Get("Set-Cookie")
	if !strings.Contains(cookie, "Path="+path) || !strings.Contains(cookie, "HttpOnly") {
		t.Fatalf("cookie %q", cookie)
	}
	if code, _, body := o.get(path + "/conteudo"); code != http.StatusOK || body != pageHTML {
		t.Fatalf("content after code %d %q", code, body)
	}
	if n, email := countOpens(t, linkID); n != 1 || email != "henry@cliente.com" {
		t.Fatalf("opens %d %q", n, email)
	}
	if code, _, _ := o.post(path+"/confirmar", url.Values{"codigo": {right}}); code != http.StatusBadRequest {
		t.Fatalf("used code again = %d", code)
	}
}

func TestCodeDiesAfterFiveWrongTries(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	_, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	o.post(path+"/codigo", nil)
	right := sixDigits.FindString(fm.sent[0].text)
	wrong := "000000"
	if right == wrong {
		wrong = "111111"
	}
	var body string
	for range 5 {
		_, _, body = o.post(path+"/confirmar", url.Values{"codigo": {wrong}})
	}
	if !strings.Contains(body, "Peça um código novo") {
		t.Fatalf("after 5 wrong: %s", body)
	}
	if code, _, _ := o.post(path+"/confirmar", url.Values{"codigo": {right}}); code != http.StatusBadRequest {
		t.Fatalf("right code after 5 wrong = %d", code)
	}
}

func TestExpiredCodeRefused(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	linkID, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	o.post(path+"/codigo", nil)
	exec(t, "UPDATE external_link_codes SET expires_at = now() - interval '1 second' WHERE link_id=$1", linkID)
	if code, _, _ := o.post(path+"/confirmar", url.Values{"codigo": {sixDigits.FindString(fm.sent[0].text)}}); code != http.StatusBadRequest {
		t.Fatalf("expired code = %d", code)
	}
}

func TestSessionDiesWithLink(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	linkID, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	o.post(path+"/codigo", nil)
	o.post(path+"/confirmar", url.Values{"codigo": {sixDigits.FindString(fm.sent[0].text)}})
	if code, _, _ := o.get(path + "/conteudo"); code != http.StatusOK {
		t.Fatalf("before cancel = %d", code)
	}
	ana.do("POST", fmt.Sprintf("/api/links/%d/cancel", linkID), nil, nil)
	if code, _, body := o.get(path + "/conteudo"); code != http.StatusNotFound || !strings.Contains(body, unavailableText) {
		t.Fatalf("after cancel %d %s", code, body)
	}
}

func TestSessionOfOneLinkDoesNotOpenAnother(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	_, first := makeLink(t, ana, id, emailLinkBody())
	_, second := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	o.post(first+"/codigo", nil)
	_, hd, _ := o.post(first+"/confirmar", url.Values{"codigo": {sixDigits.FindString(fm.sent[0].text)}})
	// Even sent by hand to the other path, the cookie belongs to the first link.
	req, _ := http.NewRequest("GET", o.srv.URL+second+"/conteudo", nil)
	req.Header.Set("Cookie", strings.Split(hd.Get("Set-Cookie"), ";")[0])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("other link with first link's cookie = %d", resp.StatusCode)
	}
}

func TestCodeRequestRateLimited(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	_, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	for i := range 5 {
		if code, _, _ := o.post(path+"/codigo", nil); code != http.StatusOK {
			t.Fatalf("request %d = %d", i+1, code)
		}
	}
	if code, _, body := o.post(path+"/codigo", nil); code != http.StatusTooManyRequests || !strings.Contains(body, "Muitas tentativas. Espere alguns minutos.") {
		t.Fatalf("6th = %d %s", code, body)
	}
}

func TestCodeRequestOnOpenLinkRefused(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	_, path := makeLink(t, ana, id, openLinkBody())
	if code, _, _ := newOutside(t, h).post(path+"/codigo", nil); code != http.StatusNotFound {
		t.Fatalf("code on open link = %d", code)
	}
}

func TestPostFromOtherSiteRefused(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	_, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	req, _ := http.NewRequest("POST", o.srv.URL+path+"/codigo", nil)
	req.Header.Set("Origin", "https://malicioso.example")
	resp, err := o.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || len(fm.sent) != 0 {
		t.Fatalf("cross-site post = %d, sent %d", resp.StatusCode, len(fm.sent))
	}
}

// Chrome sends "Origin: null" on a form post from a no-referrer page; the
// browser-set Sec-Fetch-Site is what tells same-site from cross-site.
func TestBrowserPostWithNullOriginAccepted(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	_, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	for _, c := range []struct {
		site string
		want int
	}{{"same-origin", http.StatusOK}, {"cross-site", http.StatusForbidden}} {
		req, _ := http.NewRequest("POST", o.srv.URL+path+"/codigo", nil)
		req.Header.Set("Origin", "null")
		req.Header.Set("Sec-Fetch-Site", c.site)
		resp, err := o.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Fatalf("Origin null, Sec-Fetch-Site %s = %d, want %d", c.site, resp.StatusCode, c.want)
		}
	}
	if len(fm.sent) != 1 {
		t.Fatalf("sent %d codes, want 1", len(fm.sent))
	}
}

func TestSplitLinkPath(t *testing.T) {
	for p, want := range map[string][3]string{
		"/piloto/tok":          {"piloto", "tok", ""},
		"/piloto/tok/conteudo": {"piloto", "tok", "conteudo"},
	} {
		s, tk, a, ok := SplitLinkPath(p)
		if !ok || s != want[0] || tk != want[1] || a != want[2] {
			t.Errorf("%s → %q %q %q %v", p, s, tk, a, ok)
		}
	}
	for _, p := range []string{"/", "/piloto", "/piloto/", "/piloto/tok/conteudo/mais", "//tok"} {
		if _, _, _, ok := SplitLinkPath(p); ok {
			t.Errorf("%s should not split", p)
		}
	}
	_ = time.Now
}

// Final review C1: a burst of simultaneous guesses must not get more than
// five comparisons out of one code.
func TestConcurrentGuessesCappedAtFive(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	fm := &fakeMail{}
	h.mail = fm
	linkID, path := makeLink(t, ana, id, emailLinkBody())
	o := newOutside(t, h)
	o.post(path+"/codigo", nil)
	wrong := "000000"
	if sixDigits.FindString(fm.sent[0].text) == wrong {
		wrong = "111111"
	}
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if resp, err := o.c.PostForm(o.srv.URL+path+"/confirmar", url.Values{"codigo": {wrong}}); err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	var tries int
	if err := testPool.QueryRow(context.Background(), "SELECT wrong_tries FROM external_link_codes WHERE link_id=$1", linkID).Scan(&tries); err != nil {
		t.Fatal(err)
	}
	if tries != extlink.MaxWrongTries {
		t.Fatalf("wrong_tries after 40 simultaneous guesses = %d, want %d", tries, extlink.MaxWrongTries)
	}
}

// Final review I1: a link has a daily budget of wrong guesses and of codes,
// kept in the database (survives restarts), across all its codes.
func TestLinkDailyGuessBudget(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	linkID, path := makeLink(t, ana, id, emailLinkBody())
	exec(t, "INSERT INTO external_link_codes (link_id, code_hash, expires_at, wrong_tries) SELECT $1, 'x', now(), 5 FROM generate_series(1, 4)", linkID)
	if code, _, body := newOutside(t, h).post(path+"/codigo", nil); code != http.StatusTooManyRequests || !strings.Contains(body, "Muitas tentativas") {
		t.Fatalf("after 20 wrong guesses today = %d", code)
	}
}

func TestLinkDailyCodeBudget(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	linkID, path := makeLink(t, ana, id, emailLinkBody())
	exec(t, "INSERT INTO external_link_codes (link_id, code_hash, expires_at) SELECT $1, 'x', now() FROM generate_series(1, 10)", linkID)
	if code, _, _ := newOutside(t, h).post(path+"/codigo", nil); code != http.StatusTooManyRequests {
		t.Fatalf("after 10 codes today = %d", code)
	}
}

// Final review I2: behind Caddy every request comes from loopback; the real
// address is in X-Real-IP, trusted only from loopback.
func TestRemoteHostBehindCaddy(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "127.0.0.1:5555"
	r.Header.Set("X-Real-IP", "203.0.113.9")
	if got := remoteHost(r); got != "203.0.113.9" {
		t.Fatalf("from Caddy = %q", got)
	}
	r.RemoteAddr = "198.51.100.7:5555"
	if got := remoteHost(r); got != "198.51.100.7" {
		t.Fatalf("forged header from outside = %q", got)
	}
}

func TestCompanyDailyCodeCeiling(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	other, _ := makeLink(t, ana, id, emailLinkBody())
	_, path := makeLink(t, ana, id, emailLinkBody())
	exec(t, "INSERT INTO external_link_codes (link_id, code_hash, expires_at) SELECT $1, 'x', now() FROM generate_series(1, 200)", other)
	if code, _, _ := newOutside(t, h).post(path+"/codigo", nil); code != http.StatusTooManyRequests {
		t.Fatalf("after 200 codes today in the company = %d", code)
	}
}

// Henry 2026-11-17: the whole Ziv logo, a quiet bar under the content and a
// bigger card on the screens where the person waits.
func TestOutsidePagesShowZivBrand(t *testing.T) {
	_, ana, _, h, id := linksTeam(t)
	h.mail = &fakeMail{}
	o := newOutside(t, h)
	_, open := makeLink(t, ana, id, openLinkBody())
	_, body := func() (int, string) { c, _, b := o.get(open); return c, b }()
	for _, want := range []string{`class="wm"`, "Compartilhado com segurança pelo Ziv", `<a class="more" href="http://hub.test/">Conheça o Ziv</a>`} {
		if !strings.Contains(body, want) {
			t.Fatalf("content page lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `class="promo"`) {
		t.Fatal("content page must not carry the big card")
	}
	_, email := makeLink(t, ana, id, emailLinkBody())
	_, _, ask := o.get(email)
	_, _, gone := o.get("/principal/naoexiste")
	for name, b := range map[string]string{"ask": ask, "unavailable": gone} {
		if !strings.Contains(b, `class="promo"`) || !strings.Contains(b, "Apps e relatórios da empresa num lugar só") || strings.Contains(b, `class="bar"`) {
			t.Fatalf("%s page brand:\n%s", name, b)
		}
		if !strings.Contains(b, `class="logo" href="http://hub.test/"`) {
			t.Fatalf("%s page logo must go to the Ziv home:\n%s", name, b)
		}
	}
}
