package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/governance"
)

// channelAllowed: may code or files arrive by this request? A company set
// to "só pelo Claude" takes them only through the MCP (ADR 046).
func (h *Handler) channelAllowed(ctx context.Context) (bool, error) {
	if viaMCP(ctx) {
		return true, nil
	}
	gs, err := h.q.GetGovernanceSettings(ctx)
	if err != nil {
		return false, err
	}
	return gs.CodeChannel != "mcp", nil
}

// refuseOffChannel answers 403 msgOnlyClaude when the channel is closed;
// true means the request was answered.
func (h *Handler) refuseOffChannel(w http.ResponseWriter, r *http.Request) bool {
	ok, err := h.channelAllowed(r.Context())
	if err != nil {
		internalError(w, "code channel", err)
		return true
	}
	if !ok {
		writeError(w, http.StatusForbidden, msgOnlyClaude)
		return true
	}
	return false
}

// offChannel: req is a version found in the PC folder of a company that
// only takes code through Claude. It waits and cannot be approved; a new
// version sent by Claude replaces it.
func (h *Handler) offChannel(ctx context.Context, req sqlc.ApprovalRequest) (bool, error) {
	if req.ItemKind != "app" || req.Reason != governance.ReasonVersion || req.DraftID.Valid || req.Channel == "mcp" || !req.AppVersion.Valid {
		return false, nil
	}
	gs, err := h.q.GetGovernanceSettings(ctx)
	if err != nil || gs.CodeChannel != "mcp" {
		return false, err
	}
	v, err := h.q.GetAppVersion(ctx, sqlc.GetAppVersionParams{AppID: req.AppID.Int64, Number: req.AppVersion.Int32})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v.Origin == "folder", nil
}

// mcpAllowed: may this person hold a connection to the Ziv MCP?
func (h *Handler) mcpAllowed(ctx context.Context, p sqlc.Person) (bool, error) {
	gs, err := h.q.GetGovernanceSettings(ctx)
	if err != nil {
		return false, err
	}
	if !gs.McpRestricted {
		return true, nil
	}
	acc, err := h.loadAccess(ctx, p)
	if err != nil {
		return false, err
	}
	return acc.Has(access.UseMCP), nil
}

type companyMCPJSON struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	Name       string     `json:"name"`
	PersonID   int64      `json:"person_id"`
	PersonName string     `json:"person_name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	Revoked    bool       `json:"revoked"`
	ExpiresAt  *time.Time `json:"expires_at"` // "código" only
}

func (h *Handler) listCompanyMCP(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	rows, err := h.q.ListCompanyMCPConnections(r.Context())
	if err != nil {
		internalError(w, "list company mcp", err)
		return
	}
	out := make([]companyMCPJSON, 0, len(rows))
	for _, c := range rows {
		out = append(out, companyMCPJSON{ID: c.ID, Kind: c.Kind, Name: c.Name, PersonID: c.PersonID, PersonName: c.PersonName,
			CreatedAt: c.CreatedAt.Time, LastUsedAt: tsPtr(c.LastUsedAt), Revoked: c.RevokedAt.Valid, ExpiresAt: tsPtr(c.ExpiresAt)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) revokeCompanyMCP(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Conexão não encontrada.")
		return
	}
	ctx := r.Context()
	c, err := h.q.RevokeAnyMCPConnection(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Conexão não encontrada.")
		return
	}
	if err != nil {
		internalError(w, "revoke company mcp", err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"connection_id": id, "person_id": c.PersonID, "name": c.Name})
	if err := h.q.InsertGovernanceEvent(ctx, sqlc.InsertGovernanceEventParams{Kind: "mcp", ActorID: optInt8(me.ID), Detail: detail}); err != nil {
		logGov("mcp event", id, err)
	}
	w.WriteHeader(http.StatusNoContent)
}
