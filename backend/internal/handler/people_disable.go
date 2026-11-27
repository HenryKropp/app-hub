package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// idleComputerLimit is how long a paired computer may stay silent before it
// must be paired again (Fase 0 de segurança).
const idleComputerLimit = 90 * 24 * time.Hour

// disablePerson takes a person out of the company without deleting anything:
// no sign-in, and every session, Claude connection and paired computer ends.
func (h *Handler) disablePerson(w http.ResponseWriter, r *http.Request) {
	me, p, ok := h.personToToggle(w, r)
	if !ok {
		return
	}
	if p.ID == me.ID {
		writeError(w, http.StatusBadRequest, "Você não pode desativar a si mesmo.")
		return
	}
	cur, err := h.q.GetRole(r.Context(), p.RoleID)
	if err != nil {
		internalError(w, "get role", err)
		return
	}
	if isManager(cur.Permissions) {
		n, err := h.q.CountManagersExceptPerson(r.Context(), p.ID)
		if err != nil {
			internalError(w, "count managers", err)
			return
		}
		if n == 0 {
			writeError(w, http.StatusConflict, lastManager)
			return
		}
	}
	err = h.q.InTx(r.Context(), func(q *sqlc.Queries) error {
		if _, err := q.SetPersonDisabled(r.Context(), sqlc.SetPersonDisabledParams{ID: p.ID, Disabled: true}); err != nil {
			return err
		}
		if err := q.DeletePersonSessions(r.Context(), p.ID); err != nil {
			return err
		}
		if err := q.RevokePersonComputers(r.Context(), p.ID); err != nil {
			return err
		}
		return q.RevokePersonMCPConnections(r.Context(), p.ID)
	})
	if err != nil {
		internalError(w, "disable person", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": p.ID, "disabled": true})
}

// enablePerson lets a disabled person sign in again; computers and Claude
// connections stay ended and are connected anew.
func (h *Handler) enablePerson(w http.ResponseWriter, r *http.Request) {
	_, p, ok := h.personToToggle(w, r)
	if !ok {
		return
	}
	if _, err := h.q.SetPersonDisabled(r.Context(), sqlc.SetPersonDisabledParams{ID: p.ID, Disabled: false}); err != nil {
		internalError(w, "enable person", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": p.ID, "disabled": false})
}

// personToToggle checks that the caller manages people and holds every
// permission of the person's role, like changing the role does.
func (h *Handler) personToToggle(w http.ResponseWriter, r *http.Request) (sqlc.Person, sqlc.Person, bool) {
	me, acc, ok := h.requirePerm(w, r, access.ManagePeople)
	if !ok {
		return sqlc.Person{}, sqlc.Person{}, false
	}
	id, _ := pathID(r)
	p, err := h.q.GetPerson(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Pessoa não encontrada.")
		return sqlc.Person{}, sqlc.Person{}, false
	}
	if err != nil {
		internalError(w, "get person", err)
		return sqlc.Person{}, sqlc.Person{}, false
	}
	role, err := h.q.GetRole(r.Context(), p.RoleID)
	if err != nil {
		internalError(w, "get role", err)
		return sqlc.Person{}, sqlc.Person{}, false
	}
	if !holdsAll(acc, role.Permissions) {
		writeError(w, http.StatusForbidden, "Você não pode desativar quem tem mais permissões que você.")
		return sqlc.Person{}, sqlc.Person{}, false
	}
	return me, p, true
}

// revokeIdleComputers ends the access of computers silent for 90 days.
func (h *Handler) revokeIdleComputers(ctx context.Context, now time.Time) {
	n, err := h.q.RevokeIdleComputers(ctx, ts(now.Add(-idleComputerLimit)))
	if err != nil {
		slog.Error("revoke idle computers", "err", err)
		return
	}
	if n > 0 {
		slog.Info("idle computers revoked", "count", n)
	}
}
