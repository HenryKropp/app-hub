package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5"
)

// Avisos (ADR 044): "<Nome> versão <N> no ar — testada: …", for the app's
// owner and the company admins, on the Home.

type noticeOut struct {
	ID        int64      `json:"id"`
	AppID     int64      `json:"app_id"`
	AppName   string     `json:"app_name"`
	Version   int32      `json:"version"`
	Text      string     `json:"text"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
}

// listNotices is GET /api/notices: the 20 newest.
func (h *Handler) listNotices(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	rows, err := h.q.ListNotices(r.Context(), sqlc.ListNoticesParams{Admin: acc.Has(access.ManageRoles), PersonID: me.ID})
	if err != nil {
		internalError(w, "list notices", err)
		return
	}
	out := make([]noticeOut, 0, len(rows))
	for _, n := range rows {
		o := noticeOut{ID: n.ID, AppID: n.AppID, AppName: n.AppName, Version: n.Version, Text: n.Text, CreatedAt: n.CreatedAt.Time}
		if n.ReadAt.Valid {
			o.ReadAt = &n.ReadAt.Time
		}
		out = append(out, o)
	}
	writeJSON(w, http.StatusOK, out)
}

// readNotice is POST /api/notices/{id}/read. A notice of someone else's app
// answers 404, like one that does not exist.
func (h *Handler) readNotice(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Aviso não encontrado.")
		return
	}
	n, err := h.q.GetNotice(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Aviso não encontrado.")
		return
	} else if err != nil {
		internalError(w, "get notice", err)
		return
	}
	a, err := h.q.GetApp(ctx, n.AppID)
	if err != nil {
		internalError(w, "get app", err)
		return
	}
	acc, err := h.loadAccess(ctx, me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	if !ownerOrAdmin(acc, me, a) {
		writeError(w, http.StatusNotFound, "Aviso não encontrado.")
		return
	}
	if err := h.q.ReadNotice(ctx, id); err != nil {
		internalError(w, "read notice", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
