package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

type homeItemT struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type homeT struct {
	Attention []struct {
		Kind  string `json:"kind"`
		RunID int64  `json:"run_id"`
		Count int    `json:"count"`
	} `json:"attention"`
	Favorites []homeItemT `json:"favorites"`
	Recent    []homeItemT `json:"recent"`
	Runs      []struct {
		ID       int64 `json:"id"`
		PersonID int64 `json:"person_id"`
	} `json:"runs"`
}

func getHome(t *testing.T, c *client) (homeT, string) {
	t.Helper()
	var out homeT
	if code := c.do("GET", "/api/home", nil, &out); code != http.StatusOK {
		t.Fatalf("GET /api/home = %d", code)
	}
	return out, ""
}

func homeKinds(h homeT) map[string]int {
	m := map[string]int{}
	for _, a := range h.Attention {
		m[a.Kind]++
	}
	return m
}

// runAt stores a run with a status and age; returns its id.
func runAt(t *testing.T, appID, personID int64, status string, age string, dismissed bool) int64 {
	t.Helper()
	var comp int64
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO computers (person_id, name, token_hash, last_seen_at) VALUES ($1, 'PC', md5(random()::text), now()) RETURNING id`, personID).Scan(&comp); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO runs (app_id, person_id, computer_id, status, output_folder, created_at, dismissed_at)
		 VALUES ($1, $2, $3, $4, 'C:\x', now() - $5::interval, CASE WHEN $6 THEN now() END) RETURNING id`,
		appID, personID, comp, status, age, dismissed).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestHomeFavoritesAndRecent(t *testing.T) {
	_, ana, bia, appID := team(t)
	a := createArtifact(t, ana, linkBody("Artifact A", "https://example.com/x"))
	b := createArtifact(t, ana, linkBody("Artifact B", "https://example.com/x"))
	if code := bia.do("PUT", "/api/favorites/artifact/"+itoa(a.ID), nil, nil); code != 204 {
		t.Fatalf("fav a = %d", code)
	}
	time.Sleep(10 * time.Millisecond)
	if code := bia.do("PUT", "/api/favorites/app/"+itoa(appID), nil, nil); code != 204 {
		t.Fatalf("fav app = %d", code)
	}
	bia.do("GET", artifactPath(b.ID, ""), nil, nil)
	someonesRun(t, appID, 3)
	h, _ := getHome(t, bia)
	if len(h.Favorites) != 2 || h.Favorites[0].Kind != "artifact" || h.Favorites[0].ID != a.ID || h.Favorites[1].Kind != "app" || h.Favorites[1].ID != appID {
		t.Fatalf("favorites = %+v", h.Favorites)
	}
	if len(h.Recent) != 1 || h.Recent[0].Kind != "artifact" || h.Recent[0].ID != b.ID {
		t.Fatalf("recent = %+v", h.Recent)
	}
	for i := 0; i < 8; i++ {
		x := createArtifact(t, ana, linkBody("Extra "+itoa(int64(i)), "https://example.com/x"))
		bia.do("GET", artifactPath(x.ID, ""), nil, nil)
	}
	h, _ = getHome(t, bia)
	if len(h.Recent) != 6 {
		t.Fatalf("recent len = %d", len(h.Recent))
	}
}

func TestHomeHidesInvisibleFavorites(t *testing.T) {
	_, ana, bia, appID := team(t)
	a := createArtifact(t, ana, linkBody("Segredo Artifact", "https://example.com/x"))
	bia.do("PUT", "/api/favorites/app/"+itoa(appID), nil, nil)
	bia.do("PUT", "/api/favorites/artifact/"+itoa(a.ID), nil, nil)
	restrict(t, appID)
	exec(t, "DELETE FROM artifacts WHERE id = $1", a.ID)
	h, _ := getHome(t, bia)
	if len(h.Favorites) != 0 || len(h.Recent) != 0 {
		t.Fatalf("favorites = %+v recent = %+v", h.Favorites, h.Recent)
	}
	code, _, text := bia.getContent("/api/home")
	if code != 200 || strings.Contains(text, "Segredo Artifact") || strings.Contains(text, "\"kind\":\"app\"") {
		t.Fatalf("leak: %d %s", code, text)
	}
}

func TestHomeAttentionIsMine(t *testing.T) {
	_, _, bia, appID := team(t)
	runAt(t, appID, 2, "error", "1 hour", false)
	e := runAt(t, appID, 3, "error", "1 hour", false)
	runAt(t, appID, 3, "error", "8 days", false)
	runAt(t, appID, 3, "error", "2 hours", true)
	w := runAt(t, appID, 3, "waiting", "10 minutes", false)
	runAt(t, appID, 3, "waiting", "2 minutes", false)
	h, _ := getHome(t, bia)
	got := map[string]int64{}
	n := 0
	for _, a := range h.Attention {
		if a.Kind == "run_error" || a.Kind == "run_waiting" {
			got[a.Kind] = a.RunID
			n++
		}
	}
	if n != 2 || got["run_error"] != e || got["run_waiting"] != w {
		t.Fatalf("attention = %+v (want error %d, waiting %d)", h.Attention, e, w)
	}
}

func TestHomeComputerWarnings(t *testing.T) {
	_, _, bia, _ := team(t)
	h, _ := getHome(t, bia)
	if k := homeKinds(h); k["computer_offline"]+k["agent_outdated"] != 0 {
		t.Fatalf("no computers: %+v", h.Attention)
	}
	exec(t, `INSERT INTO computers (person_id, name, token_hash, last_seen_at) VALUES (3, 'PC', 'x1', now() - interval '1 hour')`)
	h, _ = getHome(t, bia)
	if k := homeKinds(h); k["computer_offline"] != 1 {
		t.Fatalf("offline: %+v", h.Attention)
	}
	exec(t, `UPDATE computers SET last_seen_at = now(), agent_version = '0.0.1' WHERE token_hash = 'x1'`)
	h, _ = getHome(t, bia)
	if k := homeKinds(h); k["computer_offline"] != 0 || k["agent_outdated"] != 1 {
		t.Fatalf("outdated: %+v", h.Attention)
	}
}

func TestHomeCareWarnings(t *testing.T) {
	_, ana, bia, appID := team(t)
	ownerOf(t, appID, 2)
	runAt(t, appID, 3, "error", "1 hour", false)
	h, _ := getHome(t, ana)
	found := false
	for _, a := range h.Attention {
		if a.Kind == "app_errors" && a.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("ana attention = %+v", h.Attention)
	}
	h, _ = getHome(t, bia)
	if homeKinds(h)["app_errors"] != 0 {
		t.Fatalf("bia attention = %+v", h.Attention)
	}
}

func TestHomeLatestRuns(t *testing.T) {
	_, _, bia, appID := team(t)
	for i := 0; i < 7; i++ {
		someonesRun(t, appID, 3)
	}
	someonesRun(t, appID, 2)
	h, _ := getHome(t, bia)
	if len(h.Runs) != 5 {
		t.Fatalf("runs = %+v", h.Runs)
	}
	for i, r := range h.Runs {
		if r.PersonID != 3 || (i > 0 && r.ID > h.Runs[i-1].ID) {
			t.Fatalf("runs = %+v", h.Runs)
		}
	}
}
