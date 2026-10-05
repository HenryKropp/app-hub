package handler

import (
	"context"
	"net/http"
	"slices"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type personJSON struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	RoleID         int64    `json:"role_id"`
	RoleName       string   `json:"role_name"`
	Permissions    []string `json:"permissions"`
	Leads          []int64  `json:"leads"`
	Areas          []int64  `json:"areas"`
	SeesOthersRuns bool     `json:"sees_others_runs"`
}

// personOut is the logged-in person as the frontend needs it: role, what it
// allows, and the areas the person leads or belongs to.
func (h *Handler) personOut(ctx context.Context, p sqlc.Person) (personJSON, error) {
	role, err := h.q.GetRole(ctx, p.RoleID)
	if err != nil {
		return personJSON{}, err
	}
	leads, err := h.q.ListLedAreaIDs(ctx, p.ID)
	if err != nil {
		return personJSON{}, err
	}
	areas, err := h.q.ListMemberAreaIDs(ctx, p.ID)
	if err != nil {
		return personJSON{}, err
	}
	out := personJSON{ID: p.ID, Name: p.Name, Email: p.Email, RoleID: role.ID, RoleName: role.Name,
		Permissions: role.Permissions, Leads: leads, Areas: areas}
	if out.Permissions == nil {
		out.Permissions = []string{}
	}
	if out.Leads == nil {
		out.Leads = []int64{}
	}
	if out.Areas == nil {
		out.Areas = []int64{}
	}
	out.SeesOthersRuns = slices.Contains(out.Permissions, "see_all_runs") || len(out.Leads) > 0
	if !out.SeesOthersRuns {
		grants, err := h.q.ListPersonAppGrants(ctx, p.ID)
		if err != nil {
			return personJSON{}, err
		}
		areaGrants, err := h.q.ListPersonAreaGrants(ctx, p.ID)
		if err != nil {
			return personJSON{}, err
		}
		for _, g := range grants {
			out.SeesOthersRuns = out.SeesOthersRuns || g.Permission == "runs"
		}
		for _, g := range areaGrants {
			out.SeesOthersRuns = out.SeesOthersRuns || g.Permission == "runs"
		}
	}
	return out, nil
}

// loadAccess gathers everything access.Person needs about me, in a few queries.
func (h *Handler) loadAccess(ctx context.Context, me sqlc.Person) (*access.Person, error) {
	role, err := h.q.GetRole(ctx, me.RoleID)
	if err != nil {
		return nil, err
	}
	p := &access.Person{ID: me.ID, Perms: map[string]bool{}, Leads: map[int64]bool{}, Member: map[int64]bool{},
		ViewerApps: map[int64]bool{}, AppGrants: map[int64]map[string]bool{}, AreaGrants: map[int64]map[string]bool{}}
	for _, perm := range role.Permissions {
		p.Perms[perm] = true
	}
	leads, err := h.q.ListLedAreaIDs(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range leads {
		p.Leads[id] = true
	}
	member, err := h.q.ListMemberAreaIDs(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range member {
		p.Member[id] = true
	}
	viewer, err := h.q.ListViewerAppIDs(ctx, pgtype.Int8{Int64: me.ID, Valid: true})
	if err != nil {
		return nil, err
	}
	for _, id := range viewer {
		p.ViewerApps[id] = true
	}
	grants, err := h.q.ListPersonAppGrants(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, g := range grants {
		if p.AppGrants[g.AppID] == nil {
			p.AppGrants[g.AppID] = map[string]bool{}
		}
		p.AppGrants[g.AppID][g.Permission] = true
	}
	areaGrants, err := h.q.ListPersonAreaGrants(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, g := range areaGrants {
		if p.AreaGrants[g.AreaID] == nil {
			p.AreaGrants[g.AreaID] = map[string]bool{}
		}
		p.AreaGrants[g.AreaID][g.Permission] = true
	}
	return p, nil
}

func appRef(a sqlc.App) access.App {
	return access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted}
}

// requirePerm checks a role permission; 403 in plain Portuguese otherwise.
func (h *Handler) requirePerm(w http.ResponseWriter, r *http.Request, perm string) (sqlc.Person, *access.Person, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return me, nil, false
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return me, nil, false
	}
	if !acc.Has(perm) {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return me, acc, false
	}
	return me, acc, true
}

// appFor loads the app in the URL and what me can do with it. An app me
// cannot see answers 404, the same as one that does not exist.
func (h *Handler) appFor(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, *access.Person, access.Can, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return sqlc.App{}, me, nil, access.Can{}, false
	}
	a, ok := h.appFromPath(w, r)
	if !ok {
		return sqlc.App{}, me, nil, access.Can{}, false
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return sqlc.App{}, me, nil, access.Can{}, false
	}
	can := acc.For(appRef(a))
	if !can.View {
		// Zero values, so a careless caller cannot leak the hidden app.
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return sqlc.App{}, me, nil, access.Can{}, false
	}
	return a, me, acc, can, true
}
