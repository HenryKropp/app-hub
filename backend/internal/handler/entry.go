package handler

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrWorkspaceCapacity = errors.New("workspace capacity reached")
var companySlug = regexp.MustCompile(`^[a-z][a-z0-9-]{2,39}$`)

func (h *Handler) registerEntry(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/entry/companies", h.registerCompany)
	mux.HandleFunc("GET /api/entry/company", h.workspaceInfo)
	mux.HandleFunc("POST /api/entry/requests", h.requestMembership)
	mux.HandleFunc("POST /api/entry/invitations/preview", h.previewInvitation)
	mux.HandleFunc("POST /api/entry/invitations/accept", h.acceptInvitation)
	mux.HandleFunc("GET /api/membership", h.listMembership)
	mux.HandleFunc("POST /api/membership/invitations", h.inviteMember)
	mux.HandleFunc("POST /api/membership/invitations/{id}/revoke", h.revokeInvitation)
	mux.HandleFunc("POST /api/membership/requests/{id}/approve", h.approveMembership)
	mux.HandleFunc("POST /api/membership/requests/{id}/reject", h.rejectMembership)
}

func validMemberEmail(email string) bool {
	if len(email) > 254 || strings.TrimSpace(email) != email {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && strings.Contains(email, "@")
}

// Public registration bootstraps only a NEW namespace. It never uses the
// unauthenticated root setup route or modifies any existing company's accounts.
func (h *Handler) registerCompany(w http.ResponseWriter, r *http.Request) {
	if h.workspace == nil || h.workspace.Create == nil || h.workspace.BasePath != "" {
		writeError(w, 403, "Cadastre sua empresa na entrada principal do Ziv.")
		return
	}
	var b struct {
		CompanyName string `json:"company_name"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	c := credentials{Name: b.Name, Email: b.Email, Password: b.Password}
	c.normalize()
	b.CompanyName = strings.TrimSpace(b.CompanyName)
	b.Slug = strings.ToLower(strings.TrimSpace(b.Slug))
	if msg := validatePerson(c); msg != "" {
		writeError(w, 400, msg)
		return
	}
	if !validMemberEmail(c.Email) || utf8.RuneCountInString(c.Name) > 120 || b.CompanyName == "" || utf8.RuneCountInString(b.CompanyName) > 120 || !companySlug.MatchString(b.Slug) || b.Slug == "principal" {
		writeError(w, 400, "Informe seus dados, o nome da empresa e um endereço de 3 a 40 letras minúsculas, números ou hífens. O endereço principal é reservado.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), 12)
	if err != nil {
		internalError(w, "hash company admin", err)
		return
	}
	target, err := h.workspace.Create(r.Context(), b.CompanyName, b.Slug, sqlc.Person{Name: c.Name, Email: c.Email, PasswordHash: string(hash)})
	if errors.Is(err, ErrWorkspaceExists) {
		writeError(w, 409, "Esse endereço de empresa já está em uso.")
		return
	}
	if errors.Is(err, ErrWorkspaceCapacity) {
		writeError(w, 503, "O limite de empresas deste portal foi atingido. Fale com o responsável pelo Ziv.")
		return
	}
	if err != nil {
		internalError(w, "register company", err)
		return
	}
	writeJSON(w, 201, map[string]string{"url": target, "slug": b.Slug, "name": b.CompanyName})
}

// The executor must be the workspace-bound pool (or a test transaction).
// Never open another connection from a caller-supplied company/schema.
func (h *Handler) membershipTx(ctx context.Context) (pgx.Tx, error) {
	db, ok := h.q.SecurityDB().(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, errors.New("membership transactions unavailable")
	}
	return db.Begin(ctx)
}

func (h *Handler) requestMembership(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	if b.Name == "" || utf8.RuneCountInString(b.Name) > 120 || !validMemberEmail(b.Email) {
		writeError(w, 400, "Informe seu nome e um e-mail válido.")
		return
	}
	tx, err := h.membershipTx(r.Context())
	if err != nil {
		internalError(w, "begin membership request", err)
		return
	}
	defer tx.Rollback(r.Context())
	// Bound public submissions across processes, not just per HTTP handler.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(731940217)`); err != nil {
		internalError(w, "lock membership requests", err)
		return
	}
	var count int
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM membership_requests WHERE status='pending'`).Scan(&count); err != nil {
		internalError(w, "count membership requests", err)
		return
	}
	if count >= 200 {
		writeError(w, 429, "A empresa tem muitos pedidos pendentes. Peça um convite ao administrador.")
		return
	}
	// Do not disclose whether an account, invitation or request already exists.
	_, err = tx.Exec(r.Context(), `INSERT INTO membership_requests(name,email)
        SELECT $1,$2 WHERE NOT EXISTS(SELECT 1 FROM people WHERE lower(email)=$2)
        ON CONFLICT (email) WHERE status='pending' DO NOTHING`, b.Name, b.Email)
	if err != nil {
		internalError(w, "request membership", err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		internalError(w, "commit membership request", err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Pedido recebido. O administrador precisa aprovar e enviar um convite ao seu e-mail antes de você entrar."})
}
