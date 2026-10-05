package handler

import (
	"context"
	"slices"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
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

// roleKey is temporary (removed in Task 3): the built-in key of the person's role.
func (h *Handler) roleKey(ctx context.Context, p sqlc.Person) string {
	role, err := h.q.GetRole(ctx, p.RoleID)
	if err != nil || !role.Key.Valid {
		return ""
	}
	return role.Key.String
}
