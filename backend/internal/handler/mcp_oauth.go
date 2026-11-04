package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// OAuth 2.1 for claude.ai and the Claude app: dynamic registration, PKCE
// (S256 only), a consent screen in the Ziv and rotating refresh tokens.
const (
	oauthRequestTTL = 10 * time.Minute
	oauthCodeTTL    = 60 * time.Second
	oauthAccessTTL  = time.Hour
	oauthRefreshTTL = 30 * 24 * time.Hour
	oauthMaxName    = 200
)

func (h *Handler) registerMCPOAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.oauthProtectedResource)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", h.oauthProtectedResource)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.oauthServerMetadata)
	mux.HandleFunc("POST /oauth/register", h.oauthRegister)
	mux.HandleFunc("GET /oauth/authorize", h.oauthAuthorize)
	mux.HandleFunc("POST /oauth/token", h.oauthToken)
	mux.HandleFunc("GET /api/oauth/requests/{id}", h.getOAuthRequest)
	mux.HandleFunc("POST /api/oauth/requests/{id}/approve", h.approveOAuthRequest)
	mux.HandleFunc("POST /api/oauth/requests/{id}/deny", h.denyOAuthRequest)
}

func (h *Handler) oauthIssuer() string { return strings.TrimRight(h.cfg.BaseURL, "/") }

func (h *Handler) oauthProtectedResource(w http.ResponseWriter, r *http.Request) {
	iss := h.oauthIssuer()
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 iss + "/mcp",
		"authorization_servers":    []string{iss},
		"bearer_methods_supported": []string{"header"},
	})
}

func (h *Handler) oauthServerMetadata(w http.ResponseWriter, r *http.Request) {
	iss := h.oauthIssuer()
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                iss,
		"authorization_endpoint":                iss + "/oauth/authorize",
		"token_endpoint":                        iss + "/oauth/token",
		"registration_endpoint":                 iss + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	})
}

// oauthError is the RFC 6749 / 7591 error body.
func oauthError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

func randomID(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// validRedirectURI accepts https URLs and http only on this computer.
func validRedirectURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" || u.Host == "" || u.Opaque != "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Hostname() != ""
	case "http":
		host := u.Hostname()
		return host == "localhost" || host == "127.0.0.1"
	}
	return false
}

func (h *Handler) oauthRegister(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "Não entendi os dados enviados.")
		return
	}
	name := strings.TrimSpace(b.ClientName)
	if name == "" {
		name = "Claude"
	}
	if len([]rune(name)) > oauthMaxName {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "O nome pode ter até 200 letras.")
		return
	}
	if len(b.RedirectURIs) == 0 || len(b.RedirectURIs) > 10 {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "Informe de 1 a 10 endereços de retorno.")
		return
	}
	for _, u := range b.RedirectURIs {
		if len(u) > 2000 || !validRedirectURI(u) {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "Endereço de retorno não permitido.")
			return
		}
	}
	id, err := randomID(16)
	if err != nil {
		internalError(w, "oauth client id", err)
		return
	}
	cl, err := h.q.CreateMCPClient(r.Context(), sqlc.CreateMCPClientParams{ID: id, Name: name, RedirectUris: b.RedirectURIs})
	if err != nil {
		internalError(w, "create oauth client", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id": cl.ID, "client_name": cl.Name, "redirect_uris": cl.RedirectUris,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

// oauthBadPage is shown when the request cannot be sent back to the client.
func oauthBadPage(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`<!doctype html><html lang="pt-BR"><meta charset="utf-8"><title>Ziv</title>` +
		`<body style="font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem">` +
		`<h1 style="font-size:1.25rem">Não foi possível conectar o Claude</h1><p>` + msg + `</p></body></html>`))
}

// withParams adds query values to a redirect URI, keeping the ones it has.
func withParams(raw string, kv ...string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			q.Set(kv[i], kv[i+1])
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func validChallenge(s string) bool {
	if len(s) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(s)
	return err == nil
}

func (h *Handler) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID, redirect, state := q.Get("client_id"), q.Get("redirect_uri"), q.Get("state")
	if clientID == "" {
		oauthBadPage(w, "O pedido veio sem a identificação do aplicativo. Tente conectar de novo pelo Claude.")
		return
	}
	cl, err := h.q.GetMCPClient(r.Context(), clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		oauthBadPage(w, "Este aplicativo não está registrado no Ziv. Tente conectar de novo pelo Claude.")
		return
	}
	if err != nil {
		internalError(w, "get oauth client", err)
		return
	}
	if redirect == "" || !slices.Contains(cl.RedirectUris, redirect) {
		oauthBadPage(w, "O endereço de retorno não confere com o registrado. Tente conectar de novo pelo Claude.")
		return
	}
	fail := func(code, desc string) {
		http.Redirect(w, r, withParams(redirect, "error", code, "error_description", desc, "state", state), http.StatusFound)
	}
	if q.Get("response_type") != "code" {
		fail("unsupported_response_type", "Use response_type=code.")
		return
	}
	if q.Get("code_challenge_method") != "S256" || !validChallenge(q.Get("code_challenge")) {
		fail("invalid_request", "PKCE com S256 é obrigatório.")
		return
	}
	if len(state) > 1000 {
		fail("invalid_request", "state longo demais.")
		return
	}
	if err := h.q.DeleteStaleMCPAuthRequests(r.Context()); err != nil {
		internalError(w, "clean oauth requests", err)
		return
	}
	id, err := randomID(24)
	if err != nil {
		internalError(w, "oauth request id", err)
		return
	}
	if err := h.q.CreateMCPAuthRequest(r.Context(), sqlc.CreateMCPAuthRequestParams{
		ID:            id,
		ClientID:      pgtype.Text{String: cl.ID, Valid: true},
		RedirectUri:   pgtype.Text{String: redirect, Valid: true},
		CodeChallenge: pgtype.Text{String: q.Get("code_challenge"), Valid: true},
		State:         pgtype.Text{String: state, Valid: state != ""},
		ExpiresAt:     ts(time.Now().Add(oauthRequestTTL)),
	}); err != nil {
		internalError(w, "create oauth request", err)
		return
	}
	base := ""
	if h.workspace != nil {
		base = h.workspace.BasePath
	}
	http.Redirect(w, r, base+"/conectar-claude/autorizar?pedido="+url.QueryEscape(id), http.StatusFound)
}

// --- consent API (cookie session) ---

func (h *Handler) getOAuthRequest(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	req, err := h.q.GetPendingMCPAuthRequest(r.Context(), r.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Este pedido de conexão expirou. Comece de novo pelo Claude.")
		return
	}
	if err != nil {
		internalError(w, "get oauth request", err)
		return
	}
	host := ""
	if u, err := url.Parse(req.RedirectUri.String); err == nil {
		host = u.Hostname()
	}
	writeJSON(w, http.StatusOK, map[string]string{"client_name": req.ClientName, "redirect_host": host})
}

func (h *Handler) approveOAuthRequest(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	code, err := randomID(32)
	if err != nil {
		internalError(w, "oauth code", err)
		return
	}
	req, err := h.q.ApproveMCPAuthRequest(r.Context(), sqlc.ApproveMCPAuthRequestParams{
		ID:            r.PathValue("id"),
		PersonID:      pgtype.Int8{Int64: me.ID, Valid: true},
		CodeHash:      pgtype.Text{String: hashToken(code), Valid: true},
		CodeExpiresAt: ts(time.Now().Add(oauthCodeTTL)),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Este pedido de conexão expirou. Comece de novo pelo Claude.")
		return
	}
	if err != nil {
		internalError(w, "approve oauth request", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"redirect": withParams(req.RedirectUri.String, "code", code, "state", req.State.String)})
}

func (h *Handler) denyOAuthRequest(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	req, err := h.q.DenyMCPAuthRequest(r.Context(), r.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Este pedido de conexão expirou. Comece de novo pelo Claude.")
		return
	}
	if err != nil {
		internalError(w, "deny oauth request", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"redirect": withParams(req.RedirectUri.String, "error", "access_denied", "state", req.State.String)})
}

// --- token endpoint ---

func (h *Handler) oauthToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Não entendi os dados enviados.")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		h.oauthCodeGrant(w, r)
	case "refresh_token":
		h.oauthRefreshGrant(w, r)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "Tipo de concessão não aceito.")
	}
}

func pkceMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

// issueTokens makes a new access and refresh token pair.
func issueTokens() (access, accessHash, refresh, refreshHash string, err error) {
	if access, accessHash, err = newToken(); err != nil {
		return
	}
	refresh, refreshHash, err = newToken()
	return
}

func writeTokens(w http.ResponseWriter, access, refresh string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access, "token_type": "Bearer",
		"expires_in": int(oauthAccessTTL / time.Second), "refresh_token": refresh,
	})
}

func (h *Handler) oauthCodeGrant(w http.ResponseWriter, r *http.Request) {
	f := r.PostForm
	code, redirect, clientID, verifier := f.Get("code"), f.Get("redirect_uri"), f.Get("client_id"), f.Get("code_verifier")
	if code == "" || redirect == "" || clientID == "" || verifier == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Faltam code, redirect_uri, client_id ou code_verifier.")
		return
	}
	// Deleting the row is what makes the code single use, even on failure.
	req, err := h.q.ConsumeMCPAuthCode(r.Context(), pgtype.Text{String: hashToken(code), Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Código inválido ou já usado.")
		return
	}
	if err != nil {
		internalError(w, "consume oauth code", err)
		return
	}
	if !req.PersonID.Valid || !req.CodeExpiresAt.Valid || !req.CodeExpiresAt.Time.After(time.Now()) ||
		req.ClientID.String != clientID || req.RedirectUri.String != redirect || !pkceMatches(verifier, req.CodeChallenge.String) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Código inválido ou já usado.")
		return
	}
	cl, err := h.q.GetMCPClient(r.Context(), clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Aplicativo não registrado.")
		return
	}
	if err != nil {
		internalError(w, "get oauth client", err)
		return
	}
	if _, err := h.q.GetPerson(r.Context(), req.PersonID.Int64); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			oauthError(w, http.StatusBadRequest, "invalid_grant", "A pessoa não está mais no Ziv.")
			return
		}
		internalError(w, "get oauth person", err)
		return
	}
	access, accessHash, refresh, refreshHash, err := issueTokens()
	if err != nil {
		internalError(w, "oauth tokens", err)
		return
	}
	now := time.Now()
	if _, err := h.q.CreateMCPConnection(r.Context(), sqlc.CreateMCPConnectionParams{
		PersonID:         req.PersonID.Int64,
		Kind:             "oauth",
		Name:             cl.Name,
		ClientID:         pgtype.Text{String: cl.ID, Valid: true},
		TokenHash:        accessHash,
		AccessExpiresAt:  ts(now.Add(oauthAccessTTL)),
		RefreshHash:      pgtype.Text{String: refreshHash, Valid: true},
		RefreshExpiresAt: ts(now.Add(oauthRefreshTTL)),
	}); err != nil {
		internalError(w, "create oauth connection", err)
		return
	}
	writeTokens(w, access, refresh)
}

func (h *Handler) oauthRefreshGrant(w http.ResponseWriter, r *http.Request) {
	old, clientID := r.PostForm.Get("refresh_token"), r.PostForm.Get("client_id")
	if old == "" || clientID == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Faltam refresh_token ou client_id.")
		return
	}
	oldHash := pgtype.Text{String: hashToken(old), Valid: true}
	conn, err := h.q.GetMCPConnectionByRefreshHash(r.Context(), oldHash)
	if errors.Is(err, pgx.ErrNoRows) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Conexão expirada. Conecte o Claude ao Ziv de novo.")
		return
	}
	if err != nil {
		internalError(w, "get oauth refresh", err)
		return
	}
	if conn.Kind != "oauth" || conn.RevokedAt.Valid || conn.ClientID.String != clientID ||
		!conn.RefreshExpiresAt.Valid || !conn.RefreshExpiresAt.Time.After(time.Now()) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Conexão expirada. Conecte o Claude ao Ziv de novo.")
		return
	}
	if _, err := h.q.GetPerson(r.Context(), conn.PersonID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			oauthError(w, http.StatusBadRequest, "invalid_grant", "A pessoa não está mais no Ziv.")
			return
		}
		internalError(w, "get oauth person", err)
		return
	}
	access, accessHash, refresh, refreshHash, err := issueTokens()
	if err != nil {
		internalError(w, "oauth tokens", err)
		return
	}
	now := time.Now()
	n, err := h.q.RotateMCPConnectionTokens(r.Context(), sqlc.RotateMCPConnectionTokensParams{
		ID:               conn.ID,
		RefreshHash:      oldHash,
		TokenHash:        accessHash,
		AccessExpiresAt:  ts(now.Add(oauthAccessTTL)),
		RefreshHash_2:    pgtype.Text{String: refreshHash, Valid: true},
		RefreshExpiresAt: ts(now.Add(oauthRefreshTTL)),
	})
	if err != nil {
		internalError(w, "rotate oauth tokens", err)
		return
	}
	if n == 0 {
		// Another request used this refresh token first.
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Conexão expirada. Conecte o Claude ao Ziv de novo.")
		return
	}
	writeTokens(w, access, refresh)
}
