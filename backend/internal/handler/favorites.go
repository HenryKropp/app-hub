package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

const msgFavoriteNotFound = "Não encontrado."

type favoriteKey struct {
	kind string
	id   int64
}

// favoriteSet is the set of items me starred, from one query.
func (h *Handler) favoriteSet(ctx context.Context, personID int64) (map[favoriteKey]bool, error) {
	rows, err := h.q.ListFavorites(ctx, personID)
	if err != nil {
		return nil, err
	}
	set := make(map[favoriteKey]bool, len(rows))
	for _, f := range rows {
		set[favoriteKey{f.Kind, f.ItemID}] = true
	}
	return set, nil
}

// isFavorite tells whether me starred one item (detail pages).
func (h *Handler) isFavorite(ctx context.Context, personID int64, kind string, id int64) (bool, error) {
	set, err := h.favoriteSet(ctx, personID)
	return set[favoriteKey{kind, id}], err
}

// canViewItem tells whether the person may see the app or artifact. A missing
// item counts as not viewable.
func (h *Handler) canViewItem(ctx context.Context, acc *access.Person, kind string, id int64) (bool, error) {
	if kind == "app" {
		a, err := h.q.GetApp(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return acc.For(appRef(a)).View, nil
	}
	a, err := h.q.GetArtifact(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return acc.ForArtifact(artifactRef(a)).View, nil
}

// favoriteTarget validates the URL and the access; it answers the error itself.
func (h *Handler) favoriteTarget(w http.ResponseWriter, r *http.Request) (sqlc.Person, string, int64, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return me, "", 0, false
	}
	kind := r.PathValue("kind")
	if kind != "app" && kind != "artifact" {
		writeError(w, http.StatusBadRequest, "Tipo inválido.")
		return me, "", 0, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, msgFavoriteNotFound)
		return me, "", 0, false
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return me, "", 0, false
	}
	can, err := h.canViewItem(r.Context(), acc, kind, id)
	if err != nil {
		internalError(w, "favorite access", err)
		return me, "", 0, false
	}
	if !can {
		writeError(w, http.StatusNotFound, msgFavoriteNotFound)
		return me, "", 0, false
	}
	return me, kind, id, true
}

func (h *Handler) putFavorite(w http.ResponseWriter, r *http.Request) {
	me, kind, id, ok := h.favoriteTarget(w, r)
	if !ok {
		return
	}
	if err := h.q.AddFavorite(r.Context(), sqlc.AddFavoriteParams{PersonID: me.ID, Kind: kind, ItemID: id}); err != nil {
		internalError(w, "add favorite", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteFavorite(w http.ResponseWriter, r *http.Request) {
	me, kind, id, ok := h.favoriteTarget(w, r)
	if !ok {
		return
	}
	if err := h.q.RemoveFavorite(r.Context(), sqlc.RemoveFavoriteParams{PersonID: me.ID, Kind: kind, ItemID: id}); err != nil {
		internalError(w, "remove favorite", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
