package handler

import (
	"context"
	"net/http"
	"slices"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type grantJSON struct {
	PersonID    int64    `json:"person_id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type appAccessJSON struct {
	Restricted bool        `json:"restricted"`
	Areas      []nameRef   `json:"areas"`
	People     []nameRef   `json:"people"`
	Grants     []grantJSON `json:"grants"`
	AreaGrants []grantJSON `json:"area_grants"`
}

func pgInt8(n int64) pgtype.Int8 { return pgtype.Int8{Int64: n, Valid: true} }

// groupGrants folds rows (already ordered by person) into one entry per person.
func groupGrants(n int, row func(i int) (personID int64, name, permission string)) []grantJSON {
	out := []grantJSON{}
	for i := 0; i < n; i++ {
		id, name, perm := row(i)
		if k := len(out); k > 0 && out[k-1].PersonID == id {
			out[k-1].Permissions = append(out[k-1].Permissions, perm)
			continue
		}
		out = append(out, grantJSON{PersonID: id, Name: name, Permissions: []string{perm}})
	}
	return out
}

func (h *Handler) accessOf(ctx context.Context, appID, areaID int64) (appAccessJSON, error) {
	a, err := h.q.GetApp(ctx, appID)
	if err != nil {
		return appAccessJSON{}, err
	}
	out := appAccessJSON{Restricted: a.Restricted, Areas: []nameRef{}, People: []nameRef{}, Grants: []grantJSON{}, AreaGrants: []grantJSON{}}
	viewers, err := h.q.ListAppViewers(ctx, appID)
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
	g, err := h.q.ListAppGrants(ctx, appID)
	if err != nil {
		return out, err
	}
	out.Grants = groupGrants(len(g), func(i int) (int64, string, string) { return g[i].PersonID, g[i].PersonName, g[i].Permission })
	ag, err := h.q.ListAreaGrants(ctx, areaID)
	if err != nil {
		return out, err
	}
	out.AreaGrants = groupGrants(len(ag), func(i int) (int64, string, string) { return ag[i].PersonID, ag[i].PersonName, ag[i].Permission })
	return out, nil
}

func (h *Handler) getAppAccess(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Tech {
		writeError(w, http.StatusForbidden, "Você não pode ver o acesso deste app.")
		return
	}
	out, err := h.accessOf(r.Context(), a.ID, a.AreaID)
	if err != nil {
		internalError(w, "app access", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setAppAccess(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Access {
		writeError(w, http.StatusForbidden, "Você não pode mudar quem vê este app.")
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
		if err := q.SetAppRestricted(ctx, sqlc.SetAppRestrictedParams{ID: a.ID, Restricted: b.Restricted}); err != nil {
			return err
		}
		if err := q.ClearAppViewers(ctx, a.ID); err != nil {
			return err
		}
		for _, id := range b.AreaIDs {
			if err := q.AddAppViewerArea(ctx, sqlc.AddAppViewerAreaParams{AppID: a.ID, AreaID: pgInt8(id)}); err != nil {
				return err
			}
		}
		for _, id := range b.PersonIDs {
			if err := q.AddAppViewerPerson(ctx, sqlc.AddAppViewerPersonParams{AppID: a.ID, PersonID: pgInt8(id)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		internalError(w, "set app access", err)
		return
	}
	out, err := h.accessOf(ctx, a.ID, a.AreaID)
	if err != nil {
		internalError(w, "app access", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setAppGrants(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.appFor(w, r)
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
		if !slices.Contains(access.Grants, p) {
			writeError(w, http.StatusBadRequest, "Acesso desconhecido.")
			return
		}
	}
	ctx := r.Context()
	if _, err := h.q.GetPerson(ctx, b.PersonID); err != nil {
		writeError(w, http.StatusBadRequest, "Escolha a pessoa.")
		return
	}
	// One place per person: choosing "whole area" moves the accesses there, and back.
	err := h.q.InTx(ctx, func(q *sqlc.Queries) error {
		if err := q.ClearPersonAppGrants(ctx, sqlc.ClearPersonAppGrantsParams{AppID: a.ID, PersonID: b.PersonID}); err != nil {
			return err
		}
		if err := q.ClearPersonAreaGrants(ctx, sqlc.ClearPersonAreaGrantsParams{AreaID: a.AreaID, PersonID: b.PersonID}); err != nil {
			return err
		}
		for _, p := range b.Permissions {
			var err error
			if b.WholeArea {
				err = q.AddAreaGrant(ctx, sqlc.AddAreaGrantParams{AreaID: a.AreaID, PersonID: b.PersonID, Permission: p})
			} else {
				err = q.AddAppGrant(ctx, sqlc.AddAppGrantParams{AppID: a.ID, PersonID: b.PersonID, Permission: p})
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		internalError(w, "set grants", err)
		return
	}
	out, err := h.accessOf(ctx, a.ID, a.AreaID)
	if err != nil {
		internalError(w, "app access", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
