package handler

import (
	"net/http"
	"strings"
	"testing"
)

type teamT struct {
	Scope string   `json:"scope"`
	Areas []string `json:"areas"`
	Usage []struct {
		PersonID int64  `json:"person_id"`
		Name     string `json:"name"`
		Runs7d   int    `json:"runs_7d"`
		Opens7d  int    `json:"opens_7d"`
	} `json:"usage"`
	NeverUsed []struct {
		PersonID int64  `json:"person_id"`
		Name     string `json:"name"`
	} `json:"never_used"`
	Creating []struct {
		Kind       string `json:"kind"`
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		PersonName string `json:"person_name"`
		Event      string `json:"event"`
		Version    int    `json:"version"`
	} `json:"creating"`
	Pending *struct {
		Invitations []struct {
			ID      int64 `json:"id"`
			Expired bool  `json:"expired"`
		} `json:"invitations"`
		Requests    []struct{ ID int64 } `json:"requests"`
		WithoutArea []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"without_area"`
	} `json:"pending"`
}

func getTeam(t *testing.T, c *client) teamT {
	t.Helper()
	var out teamT
	if code := c.do("GET", "/api/home/team", nil, &out); code != http.StatusOK {
		t.Fatalf("GET /api/home/team = %d", code)
	}
	return out
}

func (x teamT) usageOf(name string) (runs, opens int, ok bool) {
	for _, u := range x.Usage {
		if u.Name == name {
			return u.Runs7d, u.Opens7d, true
		}
	}
	return 0, 0, false
}

func (x teamT) neverUsed(name string) bool {
	for _, u := range x.NeverUsed {
		if u.Name == name {
			return true
		}
	}
	return false
}

func TestTeamNotForEveryone(t *testing.T) {
	_, _, bia, _ := team(t)
	if code := bia.do("GET", "/api/home/team", nil, nil); code != http.StatusNotFound {
		t.Fatalf("bia = %d, want 404", code)
	}
	makeLeader(t, 1, 3)
	got := getTeam(t, bia)
	if got.Scope != "areas" || len(got.Areas) != 1 || got.Areas[0] != "Research" {
		t.Fatalf("scope/areas = %q %v", got.Scope, got.Areas)
	}
}

func TestTeamScopedToLedAreas(t *testing.T) {
	admin, _, bia, app1 := team(t)
	app2 := insertApp(t, filesPrepManifestWithID("outro-app"))
	exec(t, "UPDATE apps SET area_id = 2 WHERE id = $1", app2)
	exec(t, "INSERT INTO area_members (area_id, person_id) VALUES (1, 2), (2, 3)")
	makeLeader(t, 2, 3)
	someonesRun(t, app1, 2)
	someonesRun(t, app2, 3)
	a := createArtifact(t, admin, linkBody("Relatório da Research", "https://example.com/a"))
	got := getTeam(t, bia)
	if got.Scope != "areas" || len(got.Areas) != 1 || got.Areas[0] != "Investimentos" {
		t.Fatalf("scope/areas = %q %v", got.Scope, got.Areas)
	}
	if _, _, ok := got.usageOf("Ana"); ok || got.neverUsed("Ana") {
		t.Fatalf("Ana leaked: %+v", got)
	}
	if runs, _, ok := got.usageOf("Bia"); !ok || runs != 1 {
		t.Fatalf("Bia usage = %+v", got.Usage)
	}
	for _, c := range got.Creating {
		if c.Kind == "artifact" && c.ID == a.ID || c.Kind == "app" && c.ID == app1 {
			t.Fatalf("area 1 item leaked: %+v", c)
		}
	}
	if got.Pending != nil {
		t.Fatalf("pending should be null: %+v", got.Pending)
	}
}

func TestTeamCompanyForAdmin(t *testing.T) {
	admin, _, _, app := team(t)
	someonesRun(t, app, 2)
	someonesRun(t, app, 1)
	exec(t, "UPDATE runs SET created_at = now() - interval '8 days' WHERE person_id = 1")
	a := createArtifact(t, admin, linkBody("Painel", "https://example.com/p"))
	exec(t, `INSERT INTO artifact_versions (artifact_id, number, kind, url, author_id) VALUES ($1, 2, 'link', 'https://example.com/p2', 1)`, a.ID)
	got := getTeam(t, admin)
	if got.Scope != "company" {
		t.Fatalf("scope = %q", got.Scope)
	}
	if !got.neverUsed("Bia") {
		t.Fatalf("Bia should be never_used: %+v", got.NeverUsed)
	}
	if got.neverUsed("Ana") {
		t.Fatalf("Ana used something")
	}
	if _, _, ok := got.usageOf("Henry"); ok {
		t.Fatalf("an 8 day old run must not count: %+v", got.Usage)
	}
	if got.neverUsed("Henry") {
		t.Fatalf("Henry did run once")
	}
	if runs, _, ok := got.usageOf("Ana"); !ok || runs != 1 {
		t.Fatalf("Ana usage = %+v", got.Usage)
	}
	seen := map[string]bool{}
	for _, c := range got.Creating {
		seen[c.Kind+"/"+c.Event] = true
		if c.Kind == "artifact" && c.Event == "new_version" && c.Version != 2 {
			t.Fatalf("version = %d", c.Version)
		}
	}
	if !seen["app/created"] || !seen["artifact/created"] || !seen["artifact/new_version"] {
		t.Fatalf("creating = %+v", got.Creating)
	}
}

func TestTeamPending(t *testing.T) {
	admin, _, _, _ := team(t)
	exec(t, "INSERT INTO area_members (area_id, person_id) VALUES (1, 1), (1, 2)")
	exec(t, `INSERT INTO membership_invitations (token_hash, email, role_id, created_by, accepted_at)
	         SELECT 'h2', 'b@x.com', id, 1, now() FROM roles WHERE key = 'usuario'`)
	exec(t, `INSERT INTO membership_invitations (token_hash, email, role_id, created_by)
	         SELECT 'h1', 'a@x.com', id, 1 FROM roles WHERE key = 'usuario'`)
	exec(t, `INSERT INTO membership_invitations (token_hash, email, role_id, created_by, expires_at)
	         SELECT 'h3', 'c@x.com', id, 1, now() - interval '1 day' FROM roles WHERE key = 'usuario'`)
	exec(t, "INSERT INTO membership_requests (name, email) VALUES ('Caio', 'caio@x.com')")
	got := getTeam(t, admin)
	if got.Pending == nil || len(got.Pending.Invitations) != 2 || len(got.Pending.Requests) != 1 {
		t.Fatalf("pending = %+v", got.Pending)
	}
	expired := 0
	for _, i := range got.Pending.Invitations {
		if i.Expired {
			expired++
		}
	}
	if expired != 1 {
		t.Fatalf("expired = %d", expired)
	}
	if len(got.Pending.WithoutArea) != 1 || got.Pending.WithoutArea[0].Name != "Bia" {
		t.Fatalf("without_area = %+v", got.Pending.WithoutArea)
	}
}

func TestTeamHidesRestrictedCreations(t *testing.T) {
	admin, _, bia, _ := team(t)
	makeLeader(t, 1, 3)
	in := createArtifact(t, admin, linkBody("Restrito da Research", "https://example.com/1"))
	b2 := linkBody("Restrito dos Investimentos", "https://example.com/2")
	b2["area_id"] = 2
	out := createArtifact(t, admin, b2)
	exec(t, "UPDATE artifacts SET restricted = true")
	got := getTeam(t, bia)
	var ids []int64
	for _, c := range got.Creating {
		if c.Kind == "artifact" {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) != 1 || ids[0] != in.ID {
		t.Fatalf("bia sees artifacts %v, want only %d (not %d)", ids, in.ID, out.ID)
	}

	// An admin without see_all_apps and manage_areas does not see what others restricted.
	setRole(t, "admin", "manage_roles", "manage_people")
	exec(t, "UPDATE artifacts SET owner_id = 2 WHERE id = $1", out.ID)
	exec(t, "UPDATE artifacts SET restricted = false WHERE id = $1", in.ID)
	got = getTeam(t, admin)
	for _, c := range got.Creating {
		if c.Kind == "artifact" && c.ID == out.ID || strings.Contains(c.Name, "Restrito dos") {
			t.Fatalf("restricted item leaked to admin: %+v", c)
		}
	}
	seen := false
	for _, c := range got.Creating {
		if c.Kind == "artifact" && c.ID == in.ID {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("visible artifact missing: %+v", got.Creating)
	}
}
