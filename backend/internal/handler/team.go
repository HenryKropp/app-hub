package handler

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

const lastManager = "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos."

func isManager(perms []string) bool {
	return slices.Contains(perms, access.ManagePeople) && slices.Contains(perms, access.ManageRoles)
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

type roleJSON struct {
	ID          int64    `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	PeopleCount int64    `json:"people_count"`
}

type roleBody struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

func (b *roleBody) check() string {
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		return "Dê um nome."
	}
	if b.Permissions == nil {
		b.Permissions = []string{}
	}
	for _, p := range b.Permissions {
		if !slices.Contains(access.RolePerms, p) {
			return "Permissão desconhecida."
		}
	}
	return ""
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	if !acc.Has(access.ManageRoles) && !acc.Has(access.ManagePeople) {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return
	}
	rows, err := h.q.ListRoles(r.Context())
	if err != nil {
		internalError(w, "list roles", err)
		return
	}
	out := make([]roleJSON, 0, len(rows))
	for _, x := range rows {
		out = append(out, roleJSON{ID: x.ID, Key: x.Key.String, Name: x.Name, Permissions: orEmpty(x.Permissions), PeopleCount: x.PeopleCount})
	}
	writeJSON(w, http.StatusOK, out)
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	var b roleBody
	if !readJSON(w, r, &b) {
		return
	}
	if msg := b.check(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	x, err := h.q.CreateRole(r.Context(), sqlc.CreateRoleParams{Name: b.Name, Permissions: b.Permissions})
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe um cargo com esse nome.")
		return
	}
	if err != nil {
		internalError(w, "create role", err)
		return
	}
	writeJSON(w, http.StatusCreated, roleJSON{ID: x.ID, Key: x.Key.String, Name: x.Name, Permissions: orEmpty(x.Permissions)})
}

func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "Cargo não encontrado.")
		return
	}
	var b roleBody
	if !readJSON(w, r, &b) {
		return
	}
	if msg := b.check(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	cur, err := h.q.GetRole(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cargo não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get role", err)
		return
	}
	if isManager(cur.Permissions) && !isManager(b.Permissions) {
		n, err := h.q.CountManagersExceptRole(r.Context(), id)
		if err != nil {
			internalError(w, "count managers", err)
			return
		}
		if n == 0 {
			writeError(w, http.StatusConflict, lastManager)
			return
		}
	}
	x, err := h.q.UpdateRole(r.Context(), sqlc.UpdateRoleParams{ID: id, Name: b.Name, Permissions: b.Permissions})
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe um cargo com esse nome.")
		return
	}
	if err != nil {
		internalError(w, "update role", err)
		return
	}
	writeJSON(w, http.StatusOK, roleJSON{ID: x.ID, Key: x.Key.String, Name: x.Name, Permissions: orEmpty(x.Permissions)})
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	id, _ := pathID(r)
	rows, err := h.q.ListRoles(r.Context())
	if err != nil {
		internalError(w, "list roles", err)
		return
	}
	for _, x := range rows {
		if x.ID != id {
			continue
		}
		switch {
		case x.Key.Valid:
			writeError(w, http.StatusConflict, "Os cargos prontos não podem ser apagados.")
		case x.PeopleCount > 0:
			writeError(w, http.StatusConflict, "Esse cargo ainda tem pessoas. Troque o cargo delas antes de apagar.")
		default:
			if err := h.q.DeleteRole(r.Context(), id); err != nil {
				internalError(w, "delete role", err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}
	writeError(w, http.StatusNotFound, "Cargo não encontrado.")
}

func (h *Handler) setPersonRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManagePeople); !ok {
		return
	}
	id, _ := pathID(r)
	var b struct {
		RoleID int64 `json:"role_id"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	p, err := h.q.GetPerson(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Pessoa não encontrada.")
		return
	}
	if err != nil {
		internalError(w, "get person", err)
		return
	}
	next, err := h.q.GetRole(r.Context(), b.RoleID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Escolha o cargo da pessoa.")
		return
	}
	cur, err := h.q.GetRole(r.Context(), p.RoleID)
	if err != nil {
		internalError(w, "get role", err)
		return
	}
	if isManager(cur.Permissions) && !isManager(next.Permissions) {
		n, err := h.q.CountManagersExceptPerson(r.Context(), id)
		if err != nil {
			internalError(w, "count managers", err)
			return
		}
		if n == 0 {
			writeError(w, http.StatusConflict, lastManager)
			return
		}
	}
	if err := h.q.SetPersonRole(r.Context(), sqlc.SetPersonRoleParams{ID: id, RoleID: next.ID}); err != nil {
		internalError(w, "set role", err)
		return
	}
	writeJSON(w, http.StatusOK, personRow{ID: p.ID, Name: p.Name, Email: p.Email, RoleID: next.ID, RoleName: next.Name, AreaIDs: []int64{}})
}

type nameRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type areaJSON struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	AppCount int64     `json:"app_count"`
	Leaders  []nameRef `json:"leaders"`
	Members  []nameRef `json:"members"`
}

func (h *Handler) listAreas(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	areas, err := h.q.ListAreas(r.Context())
	if err != nil {
		internalError(w, "list areas", err)
		return
	}
	leaders, err := h.q.ListAreaLeaders(r.Context())
	if err != nil {
		internalError(w, "area leaders", err)
		return
	}
	members, err := h.q.ListAreaMembers(r.Context())
	if err != nil {
		internalError(w, "area members", err)
		return
	}
	out := make([]areaJSON, 0, len(areas))
	for _, a := range areas {
		x := areaJSON{ID: a.ID, Name: a.Name, AppCount: a.AppCount, Leaders: []nameRef{}, Members: []nameRef{}}
		for _, l := range leaders {
			if l.AreaID == a.ID {
				x.Leaders = append(x.Leaders, nameRef{ID: l.ID, Name: l.Name})
			}
		}
		for _, m := range members {
			if m.AreaID == a.ID {
				x.Members = append(x.Members, nameRef{ID: m.ID, Name: m.Name})
			}
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createArea(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	var b struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if b.Name = strings.TrimSpace(b.Name); b.Name == "" {
		writeError(w, http.StatusBadRequest, "Dê um nome.")
		return
	}
	a, err := h.q.CreateArea(r.Context(), b.Name)
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe uma área com esse nome.")
		return
	}
	if err != nil {
		internalError(w, "create area", err)
		return
	}
	writeJSON(w, http.StatusCreated, areaJSON{ID: a.ID, Name: a.Name, Leaders: []nameRef{}, Members: []nameRef{}})
}

// renameArea also renames it in every app of the area; each folder's
// app-hub.yaml catches up the next time that ficha is saved.
func (h *Handler) renameArea(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	id, _ := pathID(r)
	var b struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if b.Name = strings.TrimSpace(b.Name); b.Name == "" {
		writeError(w, http.StatusBadRequest, "Dê um nome.")
		return
	}
	a, err := h.q.RenameArea(r.Context(), sqlc.RenameAreaParams{ID: id, Name: b.Name})
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe uma área com esse nome.")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Área não encontrada.")
		return
	}
	if err != nil {
		internalError(w, "rename area", err)
		return
	}
	if err := h.q.RenameAppsArea(r.Context(), sqlc.RenameAppsAreaParams{AreaID: a.ID, Name: a.Name}); err != nil {
		internalError(w, "rename apps area", err)
		return
	}
	writeJSON(w, http.StatusOK, areaJSON{ID: a.ID, Name: a.Name, Leaders: []nameRef{}, Members: []nameRef{}})
}

func (h *Handler) deleteArea(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	id, _ := pathID(r)
	areas, err := h.q.ListAreas(r.Context())
	if err != nil {
		internalError(w, "list areas", err)
		return
	}
	for _, a := range areas {
		if a.ID != id {
			continue
		}
		if a.AppCount > 0 {
			writeError(w, http.StatusConflict, "Essa área ainda tem apps. Mude a área deles antes de apagar.")
			return
		}
		if err := h.q.DeleteArea(r.Context(), id); err != nil {
			internalError(w, "delete area", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "Área não encontrada.")
}

func (h *Handler) setAreaLeaders(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	h.replaceAreaPeople(w, r, h.q.ClearAreaLeaders, func(area, person int64) error {
		return h.q.AddAreaLeader(r.Context(), sqlc.AddAreaLeaderParams{AreaID: area, PersonID: person})
	})
}

func (h *Handler) setAreaMembers(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	id, _ := pathID(r)
	if !acc.Has(access.ManageAreas) && !acc.Leads[id] {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return
	}
	h.replaceAreaPeople(w, r, h.q.ClearAreaMembers, func(area, person int64) error {
		return h.q.AddAreaMember(r.Context(), sqlc.AddAreaMemberParams{AreaID: area, PersonID: person})
	})
}

func (h *Handler) replaceAreaPeople(w http.ResponseWriter, r *http.Request, clear func(context.Context, int64) error, add func(area, person int64) error) {
	id, _ := pathID(r)
	if _, err := h.q.GetArea(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "Área não encontrada.")
		return
	}
	var b struct {
		PersonIDs []int64 `json:"person_ids"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if err := clear(r.Context(), id); err != nil {
		internalError(w, "clear area people", err)
		return
	}
	for _, p := range b.PersonIDs {
		if err := add(id, p); err != nil {
			internalError(w, "add area person", err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
