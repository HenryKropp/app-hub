package handler

import (
	"context"
	"net/http"
	"slices"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// artifactAccessOf mirrors accessOf for an artifact: who sees it, its own
// extra accesses and the area-wide ones (which also reach the area's apps).
func (h *Handler) artifactAccessOf(ctx context.Context, artifactID int64) (appAccessJSON, error) {
	a, err := h.q.GetArtifact(ctx, artifactID)
	if err != nil {
		return appAccessJSON{}, err
	}
	out := appAccessJSON{Restricted: a.Restricted, Areas: []nameRef{}, People: []nameRef{}, Grants: []grantJSON{}, AreaGrants: []grantJSON{}}
	viewers, err := h.q.ListArtifactViewers(ctx, artifactID)
	if err != nil {
		return out, err
	}
	for _, v := range viewers {
		if v.AreaID.Valid {
			out.Areas = append(out.Areas, nameRef{ID: v.AreaID.Int64, Name: v.AreaName.String})
		} else {
			out.People = append(out.People, nameRef{ID: v.PersonID.Int64, Name: v.PersonName.String})
		}
	}
	g, err := h.q.ListArtifactGrants(ctx, artifactID)
	if err != nil {
		return out, err
	}
	out.Grants = groupGrants(len(g), func(i int) (int64, string, string) { return g[i].PersonID, g[i].PersonName, g[i].Permission })
	ag, err := h.q.ListAreaGrants(ctx, a.AreaID)
	if err != nil {
		return out, err
	}
	out.AreaGrants = groupGrants(len(ag), func(i int) (int64, string, string) { return ag[i].PersonID, ag[i].PersonName, ag[i].Permission })
	return out, nil
}

func (h *Handler) writeArtifactAccess(w http.ResponseWriter, r *http.Request, artifactID int64) {
	out, err := h.artifactAccessOf(r.Context(), artifactID)
	if err != nil {
		internalError(w, "artifact access", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getArtifactAccess(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !can.Tech {
		writeError(w, http.StatusForbidden, "Você não pode ver o acesso deste artifact.")
		return
	}
	h.writeArtifactAccess(w, r, a.ID)
}

func (h *Handler) setArtifactAccess(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !can.Access {
		writeError(w, http.StatusForbidden, "Você não pode mudar quem vê este artifact.")
		return
	}
	var b struct {
		Restricted bool    `json:"restricted"`
		AreaIDs    []int64 `json:"area_ids"`
		PersonIDs  []int64 `json:"person_ids"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	ctx := r.Context()
	for _, id := range b.AreaIDs {
		if _, err := h.q.GetArea(ctx, id); err != nil {
			writeError(w, http.StatusBadRequest, "Escolha as áreas na lista.")
			return
		}
	}
	for _, id := range b.PersonIDs {
		if _, err := h.q.GetPerson(ctx, id); err != nil {
			writeError(w, http.StatusBadRequest, "Escolha as pessoas na lista.")
			return
		}
	}
	err := h.q.InTx(ctx, func(q *sqlc.Queries) error {
		if err := q.SetArtifactRestricted(ctx, sqlc.SetArtifactRestrictedParams{ID: a.ID, Restricted: b.Restricted}); err != nil {
			return err
		}
		if err := q.ClearArtifactViewers(ctx, a.ID); err != nil {
			return err
		}
		for _, id := range b.AreaIDs {
			if err := q.AddArtifactViewerArea(ctx, sqlc.AddArtifactViewerAreaParams{ArtifactID: a.ID, AreaID: pgInt8(id)}); err != nil {
				return err
			}
		}
		for _, id := range b.PersonIDs {
			if err := q.AddArtifactViewerPerson(ctx, sqlc.AddArtifactViewerPersonParams{ArtifactID: a.ID, PersonID: pgInt8(id)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		internalError(w, "set artifact access", err)
		return
	}
	h.writeArtifactAccess(w, r, a.ID)
}

func (h *Handler) setArtifactGrants(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	if !can.Grants {
		writeError(w, http.StatusForbidden, "Só o líder da área dá acessos extras.")
		return
	}
	var b struct {
		PersonID    int64    `json:"person_id"`
		Permissions []string `json:"permissions"`
		WholeArea   bool     `json:"whole_area"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	for _, p := range b.Permissions {
		if !slices.Contains(access.ArtifactGrants, p) {
			writeError(w, http.StatusBadRequest, "Acesso desconhecido.")
			return
		}
	}
	ctx := r.Context()
	if _, err := h.q.GetPerson(ctx, b.PersonID); err != nil {
		writeError(w, http.StatusBadRequest, "Escolha a pessoa.")
		return
	}
	// Only the scope being saved is replaced: this artifact's accesses, or the
	// whole area's (shared with the area's apps), as for apps.
	err := h.q.InTx(ctx, func(q *sqlc.Queries) error {
		var err error
		perms := b.Permissions
		if b.WholeArea {
			// The area's tech and runs accesses (app-only) are not offered
			// here, so they are kept as they were.
			current, lerr := q.ListAreaGrants(ctx, a.AreaID)
			if lerr != nil {
				return lerr
			}
			for _, g := range current {
				if g.PersonID == b.PersonID && !slices.Contains(access.ArtifactGrants, g.Permission) {
					perms = append(perms, g.Permission)
				}
			}
			err = q.ClearPersonAreaGrants(ctx, sqlc.ClearPersonAreaGrantsParams{AreaID: a.AreaID, PersonID: b.PersonID})
		} else {
			err = q.ClearPersonArtifactGrants(ctx, sqlc.ClearPersonArtifactGrantsParams{ArtifactID: a.ID, PersonID: b.PersonID})
		}
		if err != nil {
			return err
		}
		for _, p := range perms {
			var err error
			if b.WholeArea {
				err = q.AddAreaGrant(ctx, sqlc.AddAreaGrantParams{AreaID: a.AreaID, PersonID: b.PersonID, Permission: p})
			} else {
				err = q.AddArtifactGrant(ctx, sqlc.AddArtifactGrantParams{ArtifactID: a.ID, PersonID: b.PersonID, Permission: p})
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		internalError(w, "set artifact grants", err)
		return
	}
	h.writeArtifactAccess(w, r, a.ID)
}
