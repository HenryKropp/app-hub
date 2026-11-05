package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var ErrWorkspaceCapacity = errors.New("workspace capacity reached")
var companySlug = regexp.MustCompile(`^[a-z][a-z0-9-]{2,39}$`)

// slugFromName turns a company name into its address: "Ação & Cia." →
// "acao-cia". Names that give fewer than 3 letters get an "empresa-" prefix.
func slugFromName(name string) string {
	plain, _, _ := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn))), name)
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(plain) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	slug := strings.TrimLeft(b.String(), "0123456789-")
	if len(slug) < 3 {
		slug = strings.Trim("empresa-"+b.String(), "-")
	}
	if len(slug) > 34 {
		slug = strings.TrimRight(slug[:34], "-")
	}
	return slug
}

func (h *Handler) registerEntry(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/entry/companies", h.registerCompany)
	mux.HandleFunc("POST /api/entry/login", h.findLogins)
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
	if msg := validatePerson(c); msg != "" {
		writeError(w, 400, msg)
		return
	}
	if !validMemberEmail(c.Email) || utf8.RuneCountInString(c.Name) > 120 || b.CompanyName == "" || utf8.RuneCountInString(b.CompanyName) > 120 {
		writeError(w, 400, "Informe seus dados e o nome da empresa.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), 12)
	if err != nil {
		internalError(w, "hash company admin", err)
		return
	}
	// The address always comes from the company name; a taken or reserved
	// one gets the next free number ("horizonte-2").
	base := slugFromName(b.CompanyName)
	var target string
	err = ErrWorkspaceExists
	for n := 1; n <= 20 && errors.Is(err, ErrWorkspaceExists); n++ {
		b.Slug = base
		if n > 1 {
			b.Slug = fmt.Sprintf("%s-%d", base, n)
		}
		if b.Slug == "principal" || h.cfg.ReservedSlugs[b.Slug] {
			continue
		}
		target, err = h.workspace.Create(r.Context(), b.CompanyName, b.Slug, sqlc.Person{Name: c.Name, Email: c.Email, PasswordHash: string(hash)})
	}
	if errors.Is(err, ErrWorkspaceExists) {
		writeError(w, 409, "Já existem muitas empresas com esse nome. Use um nome mais específico.")
		return
	}
	if errors.Is(err, ErrWorkspaceCapacity) {
		writeError(w, 503, "O Ziv não está aceitando novas empresas agora. Tente de novo amanhã ou fale com o responsável pelo Ziv.")
		return
	}
	if err != nil {
		internalError(w, "register company", err)
		return
	}
	writeJSON(w, 201, map[string]string{"url": target, "slug": b.Slug, "name": b.CompanyName})
}

// findLogins answers the main entry's "e-mail and password" form with the
// companies where they sign in. It opens no session: the browser then signs
// in at the chosen company's own /w/<slug>/api/auth/login.
func (h *Handler) findLogins(w http.ResponseWriter, r *http.Request) {
	if h.workspace == nil || h.workspace.FindLogins == nil || h.workspace.BasePath != "" {
		writeError(w, 403, "Entre pela entrada principal do Ziv.")
		return
	}
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	c.normalize()
	found, err := h.workspace.FindLogins(r.Context(), c.Email, c.Password)
	if err != nil {
		internalError(w, "find logins", err)
		return
	}
	if len(found) == 0 {
		writeError(w, http.StatusUnauthorized, "E-mail ou senha não conferem.")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]LoginMatch{"companies": found})
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
