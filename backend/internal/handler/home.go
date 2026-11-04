package handler

import (
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

const (
	homeRecentMax = 6
	homeRunsMax   = 5
	homeRunsFetch = 20 // fetched before hiding runs of apps the person cannot see
)

type homeItemJSON struct {
	Kind    string     `json:"kind"`
	ID      int64      `json:"id"`
	Name    string     `json:"name"`
	Summary string     `json:"summary"`
	Area    string     `json:"area"`
	UsedAt  *time.Time `json:"used_at,omitempty"`
}

type homeWarningJSON struct {
	Kind         string     `json:"kind"`
	At           *time.Time `json:"at,omitempty"` // nil when there is no honest time to show
	AppID        int64      `json:"app_id,omitempty"`
	AppName      string     `json:"app_name,omitempty"`
	RunID        int64      `json:"run_id,omitempty"`
	ComputerName string     `json:"computer_name,omitempty"`
	Count        int        `json:"count,omitempty"`
}

type homeJSON struct {
	Attention []homeWarningJSON `json:"attention"`
	Favorites []homeItemJSON    `json:"favorites"`
	Recent    []homeItemJSON    `json:"recent"`
	Runs      []runListItemJSON `json:"runs"`
}

// homeCatalog holds the apps and artifacts the person may see, by id.
type homeCatalog struct {
	apps      map[int64]sqlc.ListAppsRow
	artifacts map[int64]sqlc.ListArtifactsRow
	appList   []sqlc.ListAppsRow
}

func (h *Handler) homeCatalog(r *http.Request, acc *access.Person) (*homeCatalog, error) {
	apps, err := h.q.ListApps(r.Context())
	if err != nil {
		return nil, err
	}
	arts, err := h.q.ListArtifacts(r.Context())
	if err != nil {
		return nil, err
	}
	c := &homeCatalog{apps: map[int64]sqlc.ListAppsRow{}, artifacts: map[int64]sqlc.ListArtifactsRow{}}
	for _, a := range apps {
		if acc.For(access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted}).View {
			c.apps[a.ID] = a
			c.appList = append(c.appList, a)
		}
	}
	for _, a := range arts {
		if acc.ForArtifact(access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted}).View {
			c.artifacts[a.ID] = a
		}
	}
	return c, nil
}

func (c *homeCatalog) item(kind string, id int64, usedAt *time.Time) (homeItemJSON, bool) {
	if kind == "app" {
		a, ok := c.apps[id]
		return homeItemJSON{Kind: kind, ID: id, Name: a.Name, Summary: a.Summary, Area: a.Area, UsedAt: usedAt}, ok
	}
	a, ok := c.artifacts[id]
	return homeItemJSON{Kind: kind, ID: id, Name: a.Name, Summary: a.Summary, Area: a.AreaName, UsedAt: usedAt}, ok
}

// homeItems are the starred items, in the order they were starred, without
// what is gone or hidden from the person. It also returns the set of stars.
func (h *Handler) homeItems(r *http.Request, me sqlc.Person, c *homeCatalog) ([]homeItemJSON, map[favoriteKey]bool, error) {
	rows, err := h.q.ListFavorites(r.Context(), me.ID)
	if err != nil {
		return nil, nil, err
	}
	out := []homeItemJSON{}
	set := map[favoriteKey]bool{}
	for _, f := range rows {
		set[favoriteKey{f.Kind, f.ItemID}] = true
		if it, ok := c.item(f.Kind, f.ItemID, nil); ok {
			out = append(out, it)
		}
	}
	return out, set, nil
}

// homeRecent merges the apps the person ran and the artifacts they opened,
// newest first, without the starred ones.
func (h *Handler) homeRecent(r *http.Request, me sqlc.Person, c *homeCatalog, stars map[favoriteKey]bool) ([]homeItemJSON, error) {
	apps, err := h.q.ListMyRecentApps(r.Context(), me.ID)
	if err != nil {
		return nil, err
	}
	arts, err := h.q.ListMyRecentArtifacts(r.Context(), me.ID)
	if err != nil {
		return nil, err
	}
	out := []homeItemJSON{}
	add := func(kind string, id int64, ts pgtype.Timestamptz) {
		if stars[favoriteKey{kind, id}] {
			return
		}
		t := ts.Time
		if it, ok := c.item(kind, id, &t); ok {
			out = append(out, it)
		}
	}
	for _, a := range apps {
		add("app", a.AppID, a.UsedAt)
	}
	for _, a := range arts {
		add("artifact", a.ArtifactID, a.UsedAt)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UsedAt.After(*out[j].UsedAt) })
	if len(out) > homeRecentMax {
		out = out[:homeRecentMax]
	}
	return out, nil
}

// homeAttention builds the warnings: my runs, my computers, and the apps I care for.
func (h *Handler) homeAttention(r *http.Request, me sqlc.Person, acc *access.Person, c *homeCatalog) ([]homeWarningJSON, error) {
	ctx := r.Context()
	out := []homeWarningJSON{}

	runs, err := h.q.ListMyAttentionRuns(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, x := range runs {
		if _, ok := c.apps[x.AppID]; !ok {
			continue
		}
		kind := "run_error"
		switch x.Status {
		case "waiting":
			kind = "run_waiting"
		case "interrupted":
			kind = "run_interrupted"
		}
		out = append(out, homeWarningJSON{Kind: kind, At: &x.CreatedAt.Time, AppID: x.AppID, AppName: x.AppName, RunID: x.ID})
	}

	comps, err := h.q.ListPersonComputers(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	connected := false
	for _, pc := range comps {
		if pc.Connected {
			connected = true
		}
	}
	if len(comps) > 0 && !connected {
		at := comps[0].LastSeenAt
		if !at.Valid {
			at = comps[0].CreatedAt
		}
		out = append(out, homeWarningJSON{Kind: "computer_offline", At: &at.Time, ComputerName: comps[0].Name})
	}
	for _, pc := range comps {
		if pc.AgentVersion != "" && pc.AgentVersion != agentproto.Version {
			at := pc.LastSeenAt
			if !at.Valid {
				at = pc.CreatedAt
			}
			out = append(out, homeWarningJSON{Kind: "agent_outdated", At: &at.Time, ComputerName: pc.Name})
		}
	}

	// Apps the person cares for.
	var care []sqlc.ListAppsRow
	for _, a := range c.appList {
		if a.OwnerID == me.ID || a.BackupID.Int64 == me.ID || acc.Leads[a.AreaID] {
			care = append(care, a)
		}
	}
	if len(care) == 0 {
		return out, nil
	}
	briefs, err := h.sharedBriefs(r)
	if err != nil {
		return nil, err
	}
	errs, err := h.q.CountAppErrorsSince(ctx, pgtype.Timestamptz{Time: time.Now().Add(-7 * 24 * time.Hour), Valid: true})
	if err != nil {
		return nil, err
	}
	errCount := map[int64]int{}
	errAt := map[int64]time.Time{}
	for _, e := range errs {
		errCount[e.AppID] = int(e.Errors)
		errAt[e.AppID] = e.LastAt.Time
	}
	pending := map[int64]bool{}
	if h.workspace != nil {
		sec, err := h.securityConfig(ctx)
		if err != nil {
			return nil, err
		}
		if sec.RequireApproval {
			admin, err := h.adminApps(ctx)
			if err != nil {
				return nil, err
			}
			for _, a := range admin {
				if !a.Approved && !a.Suspended {
					pending[a.ID] = true
				}
			}
		}
	}
	sort.SliceStable(care, func(i, j int) bool { return care[i].Name < care[j].Name })
	for _, a := range care {
		if b := briefs[a.ID]; b != nil && b.Status == "crashed" {
			out = append(out, homeWarningJSON{Kind: "shared_crashed", AppID: a.ID, AppName: a.Name})
		}
		if n := errCount[a.ID]; n > 0 {
			out = append(out, homeWarningJSON{Kind: "app_errors", At: ptrTime(errAt[a.ID]), AppID: a.ID, AppName: a.Name, Count: n})
		}
		if pending[a.ID] {
			out = append(out, homeWarningJSON{Kind: "approval_pending", AppID: a.ID, AppName: a.Name})
		}
	}
	return out, nil
}

// home is the "Meu dia" block of the Home tab: everything is the asker's own.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	// Settle stale waiting runs first, so the warnings below see them as interrupted.
	if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	c, err := h.homeCatalog(r, acc)
	if err != nil {
		internalError(w, "home catalog", err)
		return
	}
	favs, stars, err := h.homeItems(r, me, c)
	if err != nil {
		internalError(w, "home favorites", err)
		return
	}
	recent, err := h.homeRecent(r, me, c, stars)
	if err != nil {
		internalError(w, "home recent", err)
		return
	}
	attention, err := h.homeAttention(r, me, acc, c)
	if err != nil {
		internalError(w, "home attention", err)
		return
	}
	rows, err := h.q.ListRuns(r.Context(), sqlc.ListRunsParams{PersonID: me.ID, AppIds: []int64{}, Lim: homeRunsFetch})
	if err != nil {
		internalError(w, "home runs", err)
		return
	}
	runs := []runListItemJSON{}
	for _, x := range rows {
		if _, ok := c.apps[x.AppID]; !ok {
			continue
		}
		if len(runs) == homeRunsMax {
			break
		}
		runs = append(runs, runListItemJSON{ID: x.ID, AppID: x.AppID, AppName: x.AppName, Status: x.Status, Origin: x.Origin,
			OutputFolder: x.OutputFolder, PersonID: x.PersonID, PersonName: x.PersonName, CreatedAt: x.CreatedAt.Time,
			StartedAt: tsPtr(x.StartedAt), FinishedAt: tsPtr(x.FinishedAt), FileCount: x.FileCount})
	}
	writeJSON(w, http.StatusOK, homeJSON{Attention: attention, Favorites: favs, Recent: recent, Runs: runs})
}

const homeTeamCreatingMax = 20

type teamUsageJSON struct {
	PersonID int64  `json:"person_id"`
	Name     string `json:"name"`
	Runs7d   int64  `json:"runs_7d"`
	Opens7d  int64  `json:"opens_7d"`
}

type teamPersonJSON struct {
	PersonID int64  `json:"person_id"`
	Name     string `json:"name"`
}

type teamCreatingJSON struct {
	Kind       string    `json:"kind"`
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	PersonName string    `json:"person_name"`
	Event      string    `json:"event"`
	Version    int32     `json:"version"`
	At         time.Time `json:"at"`
}

type teamInvitationJSON struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	Expired   bool      `json:"expired"`
}

type teamRequestJSON struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type teamNoAreaJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type teamPendingJSON struct {
	Invitations []teamInvitationJSON `json:"invitations"`
	Requests    []teamRequestJSON    `json:"requests"`
	WithoutArea []teamNoAreaJSON     `json:"without_area"`
}

type teamHomeJSON struct {
	Scope     string             `json:"scope"`
	Areas     []string           `json:"areas"`
	Usage     []teamUsageJSON    `json:"usage"`
	NeverUsed []teamPersonJSON   `json:"never_used"`
	Creating  []teamCreatingJSON `json:"creating"`
	Pending   *teamPendingJSON   `json:"pending"`
}

// homeTeam is the "Meu time" block: the whole company for manage_roles,
// otherwise the areas the asker leads; everyone else gets 404.
func (h *Handler) homeTeam(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	acc, err := h.loadAccess(ctx, me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	all := acc.Has(access.ManageRoles)
	if !all && len(acc.Leads) == 0 {
		writeError(w, http.StatusNotFound, msgFavoriteNotFound)
		return
	}
	out := teamHomeJSON{Scope: "company", Areas: []string{}, Usage: []teamUsageJSON{}, NeverUsed: []teamPersonJSON{},
		Creating: []teamCreatingJSON{}}
	areaIDs := []int64{}
	if !all {
		out.Scope = "areas"
		areas, err := h.q.ListAreas(ctx)
		if err != nil {
			internalError(w, "team areas", err)
			return
		}
		for _, a := range areas {
			if acc.Leads[a.ID] {
				areaIDs = append(areaIDs, a.ID)
				out.Areas = append(out.Areas, a.Name)
			}
		}
	}

	people, err := h.q.TeamPeople(ctx, sqlc.TeamPeopleParams{AllAreas: all, AreaIds: areaIDs})
	if err != nil {
		internalError(w, "team people", err)
		return
	}
	ids := make([]int64, 0, len(people))
	for _, p := range people {
		ids = append(ids, p.ID)
	}
	usage, err := h.q.TeamUsage(ctx, sqlc.TeamUsageParams{AllAreas: all, AreaIds: areaIDs, PersonIds: ids})
	if err != nil {
		internalError(w, "team usage", err)
		return
	}
	byID := map[int64]sqlc.TeamUsageRow{}
	for _, u := range usage {
		byID[u.PersonID] = u
	}
	for _, p := range people {
		u := byID[p.ID]
		if u.Runs7d+u.Opens7d > 0 {
			out.Usage = append(out.Usage, teamUsageJSON{PersonID: p.ID, Name: p.Name, Runs7d: u.Runs7d, Opens7d: u.Opens7d})
		}
		if !u.EverUsed {
			out.NeverUsed = append(out.NeverUsed, teamPersonJSON{PersonID: p.ID, Name: p.Name})
		}
	}
	sort.SliceStable(out.Usage, func(i, j int) bool {
		a, b := out.Usage[i], out.Usage[j]
		return a.Runs7d+a.Opens7d > b.Runs7d+b.Opens7d
	})

	events, err := h.q.TeamCreations(ctx, sqlc.TeamCreationsParams{AllAreas: all, AreaIds: areaIDs})
	if err != nil {
		internalError(w, "team creations", err)
		return
	}
	for _, e := range events {
		item := access.App{ID: e.ID, AreaID: e.AreaID, OwnerID: e.OwnerID, BackupID: e.BackupID.Int64, Restricted: e.Restricted}
		var can access.Can
		if e.Kind == "app" {
			can = acc.For(item)
		} else {
			can = acc.ForArtifact(item)
		}
		if !can.View {
			continue
		}
		out.Creating = append(out.Creating, teamCreatingJSON{Kind: e.Kind, ID: e.ID, Name: e.Name,
			PersonName: e.PersonName, Event: e.Event, Version: e.Version, At: e.At.Time})
		if len(out.Creating) == homeTeamCreatingMax {
			break
		}
	}

	if acc.Has(access.ManagePeople) {
		p := &teamPendingJSON{Invitations: []teamInvitationJSON{}, Requests: []teamRequestJSON{}, WithoutArea: []teamNoAreaJSON{}}
		invs, err := h.q.ListPendingInvitations(ctx)
		if err != nil {
			internalError(w, "team invitations", err)
			return
		}
		for _, i := range invs {
			p.Invitations = append(p.Invitations, teamInvitationJSON{ID: i.ID, Email: i.Email, CreatedAt: i.CreatedAt.Time, Expired: i.Expired})
		}
		reqs, err := h.q.ListPendingRequests(ctx)
		if err != nil {
			internalError(w, "team requests", err)
			return
		}
		for _, q := range reqs {
			p.Requests = append(p.Requests, teamRequestJSON{ID: q.ID, Name: q.Name, Email: q.Email, CreatedAt: q.CreatedAt.Time})
		}
		none, err := h.q.ListPeopleWithoutArea(ctx)
		if err != nil {
			internalError(w, "team without area", err)
			return
		}
		for _, n := range none {
			p.WithoutArea = append(p.WithoutArea, teamNoAreaJSON{ID: n.ID, Name: n.Name})
		}
		out.Pending = p
	}
	writeJSON(w, http.StatusOK, out)
}

func ptrTime(t time.Time) *time.Time { return &t }
