package handler

// The pages people outside the company see at link.<domain>/<company>/<token>
// (ADR 045). Plain HTML from the server, with no JavaScript and no Ziv
// cookie: the address is its own origin. The artifact itself is served with
// the same sandbox as inside the Ziv (ADR 018).

import (
	"crypto/subtle"
	_ "embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/extlink"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

//go:embed external_page.html
var externalPageHTML string

var externalPage = template.Must(template.New("page").Parse(externalPageHTML))

const (
	linkCookie          = "ziv_link"
	linkCodesPerLink    = 5   // code requests per link per window
	linkCodesPerAddress = 20  // code requests per network address per window
	linkCodesPerDay     = 10  // codes one link may send in 24 hours
	linkTriesPerDay     = 20  // code tries one link may take in 24 hours, across its codes
	companyCodesPerDay  = 200 // codes one company may send in 24 hours
	externalPageCSP     = "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; frame-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
)

type externalView struct {
	Mode         string // content, ask, code, limit, unavailable
	HomeURL      string
	CompanyName  string
	ArtifactName string
	Base         string // /<company>/<token>
	Masked       string
	Error        string
	CodeDead     bool
}

// SplitLinkPath reads /<company>/<token>[/<action>].
func SplitLinkPath(p string) (slug, token, action string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	if len(parts) == 3 {
		if parts[2] == "" {
			return "", "", "", false
		}
		action = parts[2]
	}
	return parts[0], parts[1], action, true
}

func externalHeaders(w http.ResponseWriter) {
	hd := w.Header()
	hd.Set("Content-Security-Policy", externalPageCSP)
	hd.Set("X-Frame-Options", "DENY")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Referrer-Policy", "same-origin") // the token is in the address: never sent to another site
	hd.Set("Cache-Control", "private, no-store")
	hd.Set("Content-Type", "text/html; charset=utf-8")
}

func renderExternal(w http.ResponseWriter, status int, v externalView) {
	externalHeaders(w)
	w.WriteHeader(status)
	if err := externalPage.Execute(w, v); err != nil {
		slog.Error("external page", "err", err)
	}
}

// ExternalUnavailable is the one page for every link that does not open:
// unknown, expired, cancelled or closed look exactly the same.
// home is the Ziv's home page (HomeFor), where the logo leads.
func ExternalUnavailable(w http.ResponseWriter, home string) {
	renderExternal(w, http.StatusNotFound, externalView{Mode: "unavailable", HomeURL: home})
}

// HomeFor turns a BASE_URL (perhaps with /w/<company>) into the Ziv's home
// page. On the link address "/" would stay on link.<domain>, so it never is.
func HomeFor(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return "/"
	}
	return u.Scheme + "://" + u.Host + "/"
}

func (h *Handler) homeURL() string { return HomeFor(h.cfg.BaseURL) }

func (h *Handler) companyName(r *http.Request) string {
	name := "Uma empresa"
	if h.workspace != nil && h.workspace.Name != "" {
		name = h.workspace.Name
	}
	if p, err := h.q.OnboardingProgress(r.Context(), 0); err == nil && p.CompanyName != "" {
		name = p.CompanyName
	}
	return name
}

// sameSite refuses posts sent by another site (the forms post to themselves).
// Sec-Fetch-Site is set by the browser and cannot be forged by a page, so it
// decides when present; some browsers send "Origin: null" on their own form
// posts, which is fine when Sec-Fetch-Site says same-origin.
func sameSite(r *http.Request) bool {
	if s := r.Header.Get("Sec-Fetch-Site"); s != "" {
		return s == "same-origin" || s == "none"
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	return o != "null" && err == nil && strings.EqualFold(u.Host, r.Host)
}

// linkCodeBudgetLeft keeps the daily budgets in the database, so a restart
// does not reset them: per link, codes sent and tries taken; per company,
// codes sent (one shared e-mail account serves every company).
func (h *Handler) linkCodeBudgetLeft(r *http.Request, l sqlc.ExternalLink) bool {
	use, err := h.q.ExternalLinkCodeUseToday(r.Context(), l.ID)
	if err != nil || use.Codes >= linkCodesPerDay || use.Tries >= linkTriesPerDay {
		return false
	}
	n, err := h.q.CountCompanyLinkCodesToday(r.Context())
	return err == nil && n < companyCodesPerDay
}

// ServeExternal answers one request on the link address for this company.
func (h *Handler) ServeExternal(w http.ResponseWriter, r *http.Request, slug, token, action string) {
	if slug != h.companySlug() {
		ExternalUnavailable(w, h.homeURL())
		return
	}
	l, err := h.q.GetExternalLinkByToken(r.Context(), token)
	if errors.Is(err, pgx.ErrNoRows) {
		ExternalUnavailable(w, h.homeURL())
		return
	}
	if err != nil {
		slog.Error("external link", "err", err)
		ExternalUnavailable(w, h.homeURL())
		return
	}
	now := time.Now()
	state, version, err := h.linkState(r.Context(), l, now)
	if err != nil {
		slog.Error("external link state", "err", err)
		ExternalUnavailable(w, h.homeURL())
		return
	}
	if !extlink.Open(state) {
		ExternalUnavailable(w, h.homeURL())
		return
	}
	a, err := h.q.GetArtifact(r.Context(), l.ArtifactID)
	if err != nil {
		ExternalUnavailable(w, h.homeURL())
		return
	}
	v := externalView{HomeURL: h.homeURL(), CompanyName: h.companyName(r), ArtifactName: a.Name,
		Base: "/" + slug + "/" + token, Masked: extlink.MaskEmail(l.RecipientEmail.String)}
	session, hasSession := h.linkSession(r, l)

	switch {
	case action == "" && r.Method == http.MethodGet:
		v.Mode = "content"
		if l.Kind == "email" && !hasSession {
			v.Mode = "ask"
		}
		renderExternal(w, http.StatusOK, v)
	case action == "conteudo" && r.Method == http.MethodGet:
		if l.Kind == "email" && !hasSession {
			v.Mode = "ask"
			renderExternal(w, http.StatusForbidden, v)
			return
		}
		h.serveLinkContent(w, r, l, version, session)
	case action == "codigo" && r.Method == http.MethodPost && l.Kind == "email":
		h.sendLinkCode(w, r, l, v)
	case action == "confirmar" && r.Method == http.MethodPost && l.Kind == "email":
		h.confirmLinkCode(w, r, l, v)
	default:
		ExternalUnavailable(w, h.homeURL())
	}
}

// linkSession returns the e-mail of a browser let in on this link.
func (h *Handler) linkSession(r *http.Request, l sqlc.ExternalLink) (string, bool) {
	c, err := r.Cookie(linkCookie)
	if err != nil || c.Value == "" {
		return "", false
	}
	s, err := h.q.GetExternalLinkSession(r.Context(), sqlc.GetExternalLinkSessionParams{SecretHash: extlink.Hash(c.Value), LinkID: l.ID})
	if err != nil {
		return "", false
	}
	return s.Email, true
}

func (h *Handler) serveLinkContent(w http.ResponseWriter, r *http.Request, l sqlc.ExternalLink, v sqlc.ArtifactVersion, email string) {
	content, err := h.q.GetCodeBlob(r.Context(), v.BlobSha256.String)
	if err != nil {
		slog.Error("external content", "err", err)
		ExternalUnavailable(w, h.homeURL())
		return
	}
	if err := h.q.RecordExternalLinkOpen(r.Context(), sqlc.RecordExternalLinkOpenParams{LinkID: l.ID, Email: pgtype.Text{String: email, Valid: email != ""}}); err != nil {
		slog.Error("record external open", "err", err)
	}
	hd := w.Header()
	hd.Set("Content-Type", "text/html; charset=utf-8")
	hd.Set("Content-Security-Policy", artifactCSP+"; frame-ancestors 'self'")
	hd.Set("X-Frame-Options", "SAMEORIGIN")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Referrer-Policy", "no-referrer")
	hd.Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *Handler) sendLinkCode(w http.ResponseWriter, r *http.Request, l sqlc.ExternalLink, v externalView) {
	if !sameSite(r) {
		http.Error(w, "Pedido recusado.", http.StatusForbidden)
		return
	}
	now := time.Now()
	perLink := !h.linkLimiter.allow(rateKey("link-code", l.Token), linkCodesPerLink, now)
	perAddress := !h.linkLimiter.allow(rateKey("link-code-address", remoteHost(r)), linkCodesPerAddress, now)
	if perLink || perAddress || !h.linkCodeBudgetLeft(r, l) {
		v.Mode = "limit"
		renderExternal(w, http.StatusTooManyRequests, v)
		return
	}
	if h.mail == nil {
		ExternalUnavailable(w, h.homeURL())
		return
	}
	code := extlink.NewCode()
	if _, err := h.q.CreateExternalLinkCode(r.Context(), sqlc.CreateExternalLinkCodeParams{LinkID: l.ID, CodeHash: extlink.Hash(code),
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(extlink.CodeTTL), Valid: true}}); err != nil {
		slog.Error("create link code", "err", err)
		ExternalUnavailable(w, h.homeURL())
		return
	}
	subject := "Seu código para abrir " + v.ArtifactName
	text := v.CompanyName + " compartilhou com você \"" + v.ArtifactName + "\" pelo Ziv.\n\n" +
		"Seu código: " + code + "\n\nEle vale por 10 minutos. Se não foi você que pediu, ignore este e-mail."
	if err := h.mail.Send(r.Context(), l.RecipientEmail.String, subject, text); err != nil {
		slog.Error("send link code", "err", err)
		v.Mode, v.Error = "code", "Não conseguimos mandar o e-mail agora. Tente de novo em alguns minutos."
		renderExternal(w, http.StatusBadGateway, v)
		return
	}
	v.Mode = "code"
	renderExternal(w, http.StatusOK, v)
}

func (h *Handler) confirmLinkCode(w http.ResponseWriter, r *http.Request, l sqlc.ExternalLink, v externalView) {
	if !sameSite(r) {
		http.Error(w, "Pedido recusado.", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	typed := strings.TrimSpace(r.PostFormValue("codigo"))
	v.Mode = "code"
	now := time.Now()
	dead := func() {
		v.Error, v.CodeDead = "Este código não vale mais. Peça um código novo.", true
		renderExternal(w, http.StatusBadRequest, v)
	}
	// The try is taken before comparing (ReserveExternalLinkCodeTry), so no
	// burst of simultaneous guesses gets more than MaxWrongTries comparisons.
	c, err := h.q.ReserveExternalLinkCodeTry(r.Context(), sqlc.ReserveExternalLinkCodeTryParams{LinkID: l.ID, MaxTries: extlink.MaxWrongTries})
	if err != nil {
		dead()
		return
	}
	if subtle.ConstantTimeCompare([]byte(extlink.Hash(typed)), []byte(c.CodeHash)) != 1 {
		if c.WrongTries >= extlink.MaxWrongTries {
			dead()
			return
		}
		v.Error = "Código errado. Confira e tente de novo."
		renderExternal(w, http.StatusBadRequest, v)
		return
	}
	if n, err := h.q.UseExternalLinkCode(r.Context(), c.ID); err != nil || n != 1 {
		dead()
		return
	}
	secret := extlink.NewToken()
	expires := extlink.SessionExpiry(now, l.ExpiresAt.Time)
	if err := h.q.CreateExternalLinkSession(r.Context(), sqlc.CreateExternalLinkSessionParams{LinkID: l.ID, SecretHash: extlink.Hash(secret),
		Email: l.RecipientEmail.String, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}}); err != nil {
		slog.Error("create link session", "err", err)
		ExternalUnavailable(w, h.homeURL())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: linkCookie, Value: secret, Path: v.Base, Expires: expires, HttpOnly: true,
		Secure: h.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, v.Base, http.StatusSeeOther)
}
