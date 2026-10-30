package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

func invitationHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func membershipID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "Identificador inválido.")
		return 0, false
	}
	return id, true
}

func (h *Handler) membershipRole(w http.ResponseWriter, r *http.Request, acc *access.Person, id int64) (sqlc.Role, bool) {
	role, err := h.q.GetRole(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 400, "Escolha um cargo válido.")
		return role, false
	}
	if err != nil {
		internalError(w, "membership role", err)
		return role, false
	}
	if !holdsAll(acc, role.Permissions) {
		writeError(w, 403, biggerRole)
		return role, false
	}
	return role, true
}

func (h *Handler) listMembership(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	type invitation struct {
		ID        int64     `json:"id"`
		Email     string    `json:"email"`
		RoleID    int64     `json:"role_id"`
		RoleName  string    `json:"role_name"`
		ExpiresAt time.Time `json:"expires_at"`
		CreatedAt time.Time `json:"created_at"`
	}
	type request struct {
		ID        int64     `json:"id"`
		Name      string    `json:"name"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
	}
	invitations, requests := []invitation{}, []request{}
	rows, err := h.q.SecurityDB().Query(r.Context(), `SELECT i.id,i.email,i.role_id,r.name,i.expires_at,i.created_at
        FROM membership_invitations i JOIN roles r ON r.id=i.role_id
        WHERE i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at>now() ORDER BY i.id DESC`)
	if err != nil {
		internalError(w, "list invitations", err)
		return
	}
	for rows.Next() {
		var i invitation
		if err = rows.Scan(&i.ID, &i.Email, &i.RoleID, &i.RoleName, &i.ExpiresAt, &i.CreatedAt); err != nil {
			rows.Close()
			internalError(w, "scan invitation", err)
			return
		}
		invitations = append(invitations, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		internalError(w, "read invitations", err)
		return
	}
	rows, err = h.q.SecurityDB().Query(r.Context(), `SELECT id,name,email,created_at FROM membership_requests WHERE status='pending' ORDER BY id`)
	if err != nil {
		internalError(w, "list membership requests", err)
		return
	}
	for rows.Next() {
		var i request
		if err = rows.Scan(&i.ID, &i.Name, &i.Email, &i.CreatedAt); err != nil {
			rows.Close()
			internalError(w, "scan membership request", err)
			return
		}
		requests = append(requests, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		internalError(w, "read membership requests", err)
		return
	}
	writeJSON(w, 200, map[string]any{"invitations": invitations, "requests": requests})
}

func (h *Handler) inviteMember(w http.ResponseWriter, r *http.Request) {
	h.issueInvitation(w, r, false)
}
func (h *Handler) approveMembership(w http.ResponseWriter, r *http.Request) {
	h.issueInvitation(w, r, true)
}

// Approval produces a targeted invitation, not an account. The administrator
// must deliver its link to that person's email via a trusted channel.
func (h *Handler) issueInvitation(w http.ResponseWriter, r *http.Request, fromRequest bool) {
	me, acc, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	var b struct {
		Email  string `json:"email"`
		RoleID int64  `json:"role_id"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	role, ok := h.membershipRole(w, r, acc, b.RoleID)
	if !ok {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	var requestID int64
	if fromRequest {
		requestID, ok = membershipID(w, r)
		if !ok {
			return
		}
	} else if !validMemberEmail(b.Email) {
		writeError(w, 400, "Informe um e-mail válido.")
		return
	}
	tx, err := h.membershipTx(r.Context())
	if err != nil {
		internalError(w, "begin invitation", err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(731940218)`); err != nil {
		internalError(w, "lock invitations", err)
		return
	}
	if fromRequest {
		err = tx.QueryRow(r.Context(), `SELECT email FROM membership_requests WHERE id=$1 AND status='pending' FOR UPDATE`, requestID).Scan(&b.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 404, "Pedido não encontrado ou já analisado.")
			return
		}
		if err != nil {
			internalError(w, "get membership request", err)
			return
		}
	}
	var exists bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM people WHERE lower(email)=$1)`, b.Email).Scan(&exists); err != nil {
		internalError(w, "check invitation email", err)
		return
	}
	if exists {
		writeError(w, 409, "Essa pessoa já faz parte da empresa.")
		return
	}
	var count int
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM membership_invitations WHERE accepted_at IS NULL AND revoked_at IS NULL AND expires_at>now()`).Scan(&count); err != nil {
		internalError(w, "count invitations", err)
		return
	}
	if count >= 200 {
		writeError(w, 409, "Revogue convites antigos antes de criar novos.")
		return
	}
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		internalError(w, "invitation randomness", err)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	// Reissuing invalidates previous links for this email in this company.
	if _, err = tx.Exec(r.Context(), `UPDATE membership_invitations SET revoked_at=now() WHERE email=$1 AND accepted_at IS NULL AND revoked_at IS NULL`, b.Email); err != nil {
		internalError(w, "replace invitation", err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO membership_invitations(token_hash,email,role_id,created_by) VALUES($1,$2,$3,$4)`, invitationHash(token), b.Email, role.ID, me.ID); err != nil {
		internalError(w, "issue invitation", err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE membership_requests SET status='approved',reviewed_at=now(),reviewed_by=$2 WHERE email=$1 AND status='pending'`, b.Email, me.ID); err != nil {
		internalError(w, "approve membership request", err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		internalError(w, "commit invitation", err)
		return
	}
	// Fragments do not enter HTTP access logs or Referer headers.
	writeJSON(w, 201, map[string]string{"url": strings.TrimRight(h.cfg.BaseURL, "/") + "/convite#token=" + token})
}

func (h *Handler) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	id, ok := membershipID(w, r)
	if !ok {
		return
	}
	tag, err := h.q.SecurityDB().Exec(r.Context(), `UPDATE membership_invitations SET revoked_at=now() WHERE id=$1 AND accepted_at IS NULL AND revoked_at IS NULL`, id)
	if err != nil {
		internalError(w, "revoke invitation", err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "Convite não encontrado ou já utilizado.")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) rejectMembership(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	id, ok := membershipID(w, r)
	if !ok {
		return
	}
	tag, err := h.q.SecurityDB().Exec(r.Context(), `UPDATE membership_requests SET status='rejected',reviewed_at=now(),reviewed_by=$2 WHERE id=$1 AND status='pending'`, id, me.ID)
	if err != nil {
		internalError(w, "reject membership request", err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "Pedido não encontrado ou já analisado.")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) previewInvitation(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token string `json:"token"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if len(b.Token) != 43 {
		writeError(w, 410, "Convite inválido, expirado ou já utilizado.")
		return
	}
	var email string
	err := h.q.SecurityDB().QueryRow(r.Context(), `SELECT email FROM membership_invitations WHERE token_hash=$1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at>now()`, invitationHash(b.Token)).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 410, "Convite inválido, expirado ou já utilizado.")
		return
	}
	if err != nil {
		internalError(w, "preview invitation", err)
		return
	}
	name := "Minha empresa"
	if h.workspace != nil {
		name = h.workspace.Name
	}
	if progress, e := h.q.OnboardingProgress(r.Context(), 0); e == nil && progress.CompanyName != "" {
		name = progress.CompanyName
	}
	writeJSON(w, 200, map[string]string{"email": email, "company_name": name})
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token    string `json:"token"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if len(b.Token) != 43 {
		writeError(w, 410, "Convite inválido, expirado ou já utilizado.")
		return
	}
	if msg := validatePerson(credentials{Name: b.Name, Email: "invited@example.com", Password: b.Password}); msg != "" {
		writeError(w, 400, msg)
		return
	}
	if utf8.RuneCountInString(b.Name) > 120 {
		writeError(w, 400, "O nome pode ter até 120 caracteres.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(b.Password), 12)
	if err != nil {
		internalError(w, "hash invited person", err)
		return
	}
	tx, err := h.membershipTx(r.Context())
	if err != nil {
		internalError(w, "begin accept invitation", err)
		return
	}
	defer tx.Rollback(r.Context())
	var email string
	var id, roleID int64
	err = tx.QueryRow(r.Context(), `SELECT id,email,role_id FROM membership_invitations WHERE token_hash=$1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, invitationHash(b.Token)).Scan(&id, &email, &roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 410, "Convite inválido, expirado ou já utilizado.")
		return
	}
	if err != nil {
		internalError(w, "get accepted invitation", err)
		return
	}
	person, err := h.q.WithTx(tx).CreatePerson(r.Context(), sqlc.CreatePersonParams{Name: b.Name, Email: email, PasswordHash: string(hash), RoleID: roleID})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		writeError(w, 409, "Você já tem uma conta nesta empresa. Entre com seu e-mail e senha.")
		return
	}
	if err != nil {
		internalError(w, "create invited person", err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE membership_invitations SET accepted_at=now() WHERE id=$1`, id); err != nil {
		internalError(w, "consume invitation", err)
		return
	}
	// Persist the session in the same transaction as membership and consumption.
	sessionHandler := &Handler{q: h.q.WithTx(tx), cfg: h.cfg, workspace: h.workspace}
	// Buffer Set-Cookie until commit succeeds, so rollback cannot log anyone in.
	headers := &sessionHeaders{header: make(http.Header)}
	if err = sessionHandler.startSession(headers, r, person.ID); err != nil {
		internalError(w, "start invited session", err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		internalError(w, "commit accepted invitation", err)
		return
	}
	for _, cookie := range headers.header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", cookie)
	}
	writeJSON(w, 201, map[string]string{"url": h.cookiePath()})
}

type sessionHeaders struct{ header http.Header }

func (s *sessionHeaders) Header() http.Header         { return s.header }
func (s *sessionHeaders) Write(b []byte) (int, error) { return len(b), nil }
func (s *sessionHeaders) WriteHeader(int)             {}
