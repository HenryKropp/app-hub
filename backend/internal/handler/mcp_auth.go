package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// mcpCtxKey is unexported, so only this package can put an MCP identity in a
// request context; nothing coming from outside can.
type mcpCtxKey struct{}

var errMCPUnauthorized = errors.New("mcp: token inválido")

// withMCPPerson marks the context as a request made by Claude for this person.
func withMCPPerson(ctx context.Context, personID int64) context.Context {
	return context.WithValue(ctx, mcpCtxKey{}, personID)
}

func mcpPersonID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(mcpCtxKey{}).(int64)
	return id, ok
}

func viaMCP(ctx context.Context) bool {
	_, ok := mcpPersonID(ctx)
	return ok
}

// originOf is what gets stored in "origin" columns for things made in ctx.
func originOf(ctx context.Context) string {
	if viaMCP(ctx) {
		return "mcp"
	}
	return "manual"
}

// newToken makes (hashToken, in agents.go, is the SHA-256 hex) a "ziv_" token; only its SHA-256 hex is ever stored.
func newToken() (plain, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = "ziv_" + base64.RawURLEncoding.EncodeToString(buf)
	return plain, hashToken(plain), nil
}

// mcpConnection finds the live connection and person for a bearer token.
func (h *Handler) mcpConnection(ctx context.Context, bearer string) (sqlc.McpConnection, sqlc.Person, error) {
	if bearer == "" {
		return sqlc.McpConnection{}, sqlc.Person{}, errMCPUnauthorized
	}
	conn, err := h.q.GetMCPConnectionByTokenHash(ctx, hashToken(bearer))
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.McpConnection{}, sqlc.Person{}, errMCPUnauthorized
	}
	if err != nil {
		return sqlc.McpConnection{}, sqlc.Person{}, err
	}
	if conn.RevokedAt.Valid || (conn.AccessExpiresAt.Valid && !conn.AccessExpiresAt.Time.After(time.Now())) {
		return sqlc.McpConnection{}, sqlc.Person{}, errMCPUnauthorized
	}
	p, err := h.q.GetPerson(ctx, conn.PersonID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.McpConnection{}, sqlc.Person{}, errMCPUnauthorized
	}
	if err != nil {
		return sqlc.McpConnection{}, sqlc.Person{}, err
	}
	if err := h.q.TouchMCPConnection(ctx, conn.ID); err != nil {
		return sqlc.McpConnection{}, sqlc.Person{}, err
	}
	return conn, p, nil
}

func (h *Handler) registerMCPAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mcp/connections", h.listMCPConnections)
	mux.HandleFunc("POST /api/mcp/codes", h.createMCPCode)
	mux.HandleFunc("DELETE /api/mcp/connections/{id}", h.revokeMCPConnection)
}

type mcpConnectionJSON struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func (h *Handler) listMCPConnections(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListMCPConnections(r.Context(), me.ID)
	if err != nil {
		internalError(w, "list mcp connections", err)
		return
	}
	out := make([]mcpConnectionJSON, 0, len(rows))
	for _, c := range rows {
		out = append(out, mcpConnectionJSON{ID: c.ID, Kind: c.Kind, Name: c.Name, CreatedAt: c.CreatedAt.Time, LastUsedAt: tsPtr(c.LastUsedAt)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createMCPCode(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	var b struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	name := strings.TrimSpace(b.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "Dê um nome para este código, por exemplo o nome do seu computador.")
		return
	}
	if len([]rune(name)) > 80 {
		writeError(w, http.StatusBadRequest, "O nome pode ter até 80 letras.")
		return
	}
	plain, hash, err := newToken()
	if err != nil {
		internalError(w, "new mcp token", err)
		return
	}
	conn, err := h.q.CreateMCPConnection(r.Context(), sqlc.CreateMCPConnectionParams{
		PersonID: me.ID, Kind: "code", Name: name, TokenHash: hash,
	})
	if err != nil {
		internalError(w, "create mcp code", err)
		return
	}
	base := strings.TrimRight(h.cfg.BaseURL, "/")
	cmd := `claude mcp add --transport http ziv ` + base + `/mcp --header "Authorization: Bearer ` + plain + `"`
	writeJSON(w, http.StatusCreated, map[string]any{"id": conn.ID, "name": conn.Name, "code": plain, "command": cmd})
}

func (h *Handler) revokeMCPConnection(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Conexão não encontrada.")
		return
	}
	n, err := h.q.RevokeMCPConnection(r.Context(), sqlc.RevokeMCPConnectionParams{ID: id, PersonID: me.ID})
	if err != nil {
		internalError(w, "revoke mcp connection", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "Conexão não encontrada.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
