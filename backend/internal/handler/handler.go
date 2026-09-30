package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

const (
	cookieName  = "hub_session"
	sessionTTL  = 30 * 24 * time.Hour
	maxBodySize = 1 << 20
)

type Handler struct {
	q   *sqlc.Queries
	cfg config.Config

	// Waits used by the agent endpoints; tests shorten them.
	waitNext   time.Duration
	waitFile   time.Duration
	waitFolder time.Duration
	tick       time.Duration

	uploads *uploadHub
}

func New(q *sqlc.Queries, cfg config.Config) *Handler {
	return &Handler{
		q: q, cfg: cfg,
		waitNext: 25 * time.Second, waitFile: 60 * time.Second, waitFolder: 5 * time.Minute, tick: 300 * time.Millisecond,
		uploads: newUploadHub(),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /up", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	mux.HandleFunc("GET /api/auth/status", h.authStatus)
	mux.HandleFunc("POST /api/auth/setup", h.setup)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	if h.cfg.DevMode {
		mux.HandleFunc("POST /api/dev/login", h.devLogin)
	}

	mux.HandleFunc("GET /api/people", h.listPeople)
	mux.HandleFunc("POST /api/people", h.createPerson)

	mux.HandleFunc("POST /api/scan", h.scan)
	mux.HandleFunc("GET /api/apps", h.listApps)
	mux.HandleFunc("GET /api/apps/{id}", h.getApp)
	mux.HandleFunc("POST /api/apps", h.saveApp)
	mux.HandleFunc("POST /api/apps/{id}/runs", h.startRun)
	mux.HandleFunc("GET /api/apps/{id}/runs", h.listAppRuns)
	mux.HandleFunc("GET /api/runs/{id}", h.getRun)
	mux.HandleFunc("POST /api/choose-folder", h.chooseFolder)
	mux.HandleFunc("GET /api/runs/{id}/files/{fileId}", h.downloadFile)
	mux.HandleFunc("GET /api/runs/{id}/groups/{index}/zip", h.downloadGroup)
	mux.HandleFunc("GET /api/runs/{id}/selected", h.downloadSelected)
	mux.HandleFunc("POST /api/agent/jobs/{id}/upload", h.agentUpload)

	mux.HandleFunc("POST /api/agent/pair/start", h.pairStart)
	mux.HandleFunc("POST /api/agent/pair/poll", h.pairPoll)
	mux.HandleFunc("GET /api/pairings/{code}", h.getPairing)
	mux.HandleFunc("POST /api/pairings/{code}/approve", h.approvePairing)
	mux.HandleFunc("GET /api/computers/me", h.myComputers)
	mux.HandleFunc("DELETE /api/computers/{id}", h.revokeComputer)
	mux.HandleFunc("POST /api/agent/hello", h.agentHello)
	mux.HandleFunc("GET /api/agent/download", h.agentDownload)
	mux.HandleFunc("GET /api/agent/binary", h.agentBinary)
	mux.HandleFunc("GET /api/agent/next", h.agentNext)
	mux.HandleFunc("POST /api/agent/jobs/{id}/result", h.agentResult)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return false
	}
	return true
}

func internalError(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, "err", err)
	writeError(w, http.StatusInternalServerError, "Algo deu errado do nosso lado. Tente de novo.")
}

type personJSON struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func toPersonJSON(p sqlc.Person) personJSON {
	return personJSON{ID: p.ID, Name: p.Name, Email: p.Email, Role: p.Role}
}

// currentPerson returns the logged-in person, or writes 401 and returns false.
func (h *Handler) currentPerson(w http.ResponseWriter, r *http.Request) (sqlc.Person, bool) {
	c, err := r.Cookie(cookieName)
	if err == nil && c.Value != "" {
		p, err := h.q.GetSessionPerson(r.Context(), c.Value)
		if err == nil {
			return p, true
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get session", err)
			return sqlc.Person{}, false
		}
	}
	writeError(w, http.StatusUnauthorized, "Entre no Hub para continuar.")
	return sqlc.Person{}, false
}

// requireRole checks that the logged-in person has one of the roles.
func (h *Handler) requireRole(w http.ResponseWriter, r *http.Request, roles ...string) (sqlc.Person, bool) {
	p, ok := h.currentPerson(w, r)
	if !ok {
		return p, false
	}
	for _, role := range roles {
		if p.Role == role {
			return p, true
		}
	}
	writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
	return p, false
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, personID int64) error {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	expires := time.Now().Add(sessionTTL)
	if err := h.q.CreateSession(r.Context(), sqlc.CreateSessionParams{
		Token: token, PersonID: personID, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
	}); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.cfg.SecureCookies(),
	})
	return nil
}

// --- auth ---

func (h *Handler) authStatus(w http.ResponseWriter, r *http.Request) {
	count, err := h.q.CountPeople(r.Context())
	if err != nil {
		internalError(w, "count people", err)
		return
	}
	resp := map[string]any{"needs_setup": count == 0, "person": nil}
	if c, err := r.Cookie(cookieName); err == nil {
		if p, err := h.q.GetSessionPerson(r.Context(), c.Value); err == nil {
			resp["person"] = toPersonJSON(p)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (c *credentials) normalize() {
	c.Name = strings.TrimSpace(c.Name)
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
}

func validatePerson(c credentials) string {
	switch {
	case c.Name == "":
		return "Informe o nome."
	case !strings.Contains(c.Email, "@"):
		return "Informe um e-mail válido."
	case len(c.Password) < 8:
		return "A senha precisa ter pelo menos 8 caracteres."
	}
	return ""
}

// setup creates the first person (administrator). Only works while nobody exists.
func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	c.normalize()
	if msg := validatePerson(c); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	count, err := h.q.CountPeople(r.Context())
	if err != nil {
		internalError(w, "count people", err)
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "O Hub já foi configurado. Entre com seu e-mail e senha.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		internalError(w, "hash", err)
		return
	}
	p, err := h.q.CreatePerson(r.Context(), sqlc.CreatePersonParams{Name: c.Name, Email: c.Email, PasswordHash: string(hash), Role: "admin"})
	if err != nil {
		internalError(w, "create person", err)
		return
	}
	if err := h.startSession(w, r, p.ID); err != nil {
		internalError(w, "session", err)
		return
	}
	writeJSON(w, http.StatusCreated, toPersonJSON(p))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	c.normalize()
	p, err := h.q.GetPersonByEmail(r.Context(), c.Email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		internalError(w, "get person", err)
		return
	}
	if err != nil || p.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(p.PasswordHash), []byte(c.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "E-mail ou senha não conferem.")
		return
	}
	if err := h.startSession(w, r, p.ID); err != nil {
		internalError(w, "session", err)
		return
	}
	writeJSON(w, http.StatusOK, toPersonJSON(p))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if err := h.q.DeleteSession(r.Context(), c.Value); err != nil {
			slog.Warn("delete session", "err", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}

// devLogin is registered only with DEV_MODE; it signs in a test administrator.
func (h *Handler) devLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(&body)
	if body.Email == "" {
		body.Email = "dev@apphub.local"
	}
	p, err := h.q.UpsertDevPerson(r.Context(), sqlc.UpsertDevPersonParams{Name: "Pessoa de teste", Email: strings.ToLower(body.Email)})
	if err != nil {
		internalError(w, "dev person", err)
		return
	}
	if err := h.startSession(w, r, p.ID); err != nil {
		internalError(w, "session", err)
		return
	}
	writeJSON(w, http.StatusOK, toPersonJSON(p))
}

// --- people ---

func (h *Handler) listPeople(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	people, err := h.q.ListPeople(r.Context())
	if err != nil {
		internalError(w, "list people", err)
		return
	}
	out := make([]personJSON, 0, len(people))
	for _, p := range people {
		out = append(out, personJSON{ID: p.ID, Name: p.Name, Email: p.Email, Role: p.Role})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createPerson(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireRole(w, r, "admin"); !ok {
		return
	}
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	c.normalize()
	if msg := validatePerson(c); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if c.Role != "admin" && c.Role != "criador" && c.Role != "usuario" {
		writeError(w, http.StatusBadRequest, "Escolha o perfil da pessoa.")
		return
	}
	if _, err := h.q.GetPersonByEmail(r.Context(), c.Email); err == nil {
		writeError(w, http.StatusConflict, "Já existe uma pessoa com esse e-mail.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		internalError(w, "hash", err)
		return
	}
	p, err := h.q.CreatePerson(r.Context(), sqlc.CreatePersonParams{Name: c.Name, Email: c.Email, PasswordHash: string(hash), Role: c.Role})
	if err != nil {
		internalError(w, "create person", err)
		return
	}
	writeJSON(w, http.StatusCreated, toPersonJSON(p))
}
