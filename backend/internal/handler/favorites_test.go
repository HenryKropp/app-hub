package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func countRows(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func appStarred(t *testing.T, c *client, appID int64) bool {
	t.Helper()
	var apps []struct {
		ID       int64 `json:"id"`
		Favorite bool  `json:"favorite"`
	}
	if code := c.do("GET", "/api/apps", nil, &apps); code != http.StatusOK {
		t.Fatalf("list apps = %d", code)
	}
	for _, a := range apps {
		if a.ID == appID {
			return a.Favorite
		}
	}
	t.Fatal("app not listed")
	return false
}

func artifactStarred(t *testing.T, c *client, id int64) bool {
	t.Helper()
	var list []struct {
		ID       int64 `json:"id"`
		Favorite bool  `json:"favorite"`
	}
	if code := c.do("GET", "/api/artifacts", nil, &list); code != http.StatusOK {
		t.Fatalf("list artifacts = %d", code)
	}
	for _, a := range list {
		if a.ID == id {
			return a.Favorite
		}
	}
	t.Fatal("artifact not listed")
	return false
}

func TestFavoriteToggle(t *testing.T) {
	_, ana, bia, appID := team(t)
	fav := fmt.Sprintf("/api/favorites/app/%d", appID)
	if appStarred(t, bia, appID) {
		t.Fatal("starred before PUT")
	}
	for i := 0; i < 2; i++ {
		if code := bia.do("PUT", fav, nil, nil); code != http.StatusNoContent {
			t.Fatalf("PUT = %d", code)
		}
	}
	if !appStarred(t, bia, appID) {
		t.Fatal("not starred after PUT")
	}
	var d struct {
		Favorite bool `json:"favorite"`
	}
	bia.do("GET", fmt.Sprintf("/api/apps/%d", appID), nil, &d)
	if !d.Favorite {
		t.Fatal("detail not starred")
	}
	if appStarred(t, ana, appID) {
		t.Fatal("star leaked to another person")
	}
	if code := bia.do("DELETE", fav, nil, nil); code != http.StatusNoContent {
		t.Fatalf("DELETE = %d", code)
	}
	if appStarred(t, bia, appID) {
		t.Fatal("still starred after DELETE")
	}

	art := createArtifact(t, ana, linkBody("Painel", "https://example.com/x"))
	afav := fmt.Sprintf("/api/favorites/artifact/%d", art.ID)
	for i := 0; i < 2; i++ {
		if code := bia.do("PUT", afav, nil, nil); code != http.StatusNoContent {
			t.Fatalf("PUT artifact = %d", code)
		}
	}
	if !artifactStarred(t, bia, art.ID) {
		t.Fatal("artifact not starred")
	}
	var ad struct {
		Favorite bool `json:"favorite"`
	}
	bia.do("GET", artifactPath(art.ID, ""), nil, &ad)
	if !ad.Favorite {
		t.Fatal("artifact detail not starred")
	}
	if code := bia.do("DELETE", afav, nil, nil); code != http.StatusNoContent {
		t.Fatalf("DELETE artifact = %d", code)
	}
	if artifactStarred(t, bia, art.ID) {
		t.Fatal("artifact still starred")
	}
}

func TestFavoriteNeedsAccess(t *testing.T) {
	_, _, bia, appID := team(t)
	restrict(t, appID)
	if code := bia.do("PUT", fmt.Sprintf("/api/favorites/app/%d", appID), nil, nil); code != http.StatusNotFound {
		t.Fatalf("PUT restricted = %d, want 404", code)
	}
	if code := bia.do("DELETE", fmt.Sprintf("/api/favorites/app/%d", appID), nil, nil); code != http.StatusNotFound {
		t.Fatalf("DELETE restricted = %d, want 404", code)
	}
	if n := countRows(t, "SELECT count(*) FROM favorites"); n != 0 {
		t.Fatalf("favorites = %d, want 0", n)
	}
	if code := bia.do("PUT", "/api/favorites/app/999999", nil, nil); code != http.StatusNotFound {
		t.Fatalf("PUT missing = %d, want 404", code)
	}
	if code := bia.do("PUT", "/api/favorites/foo/1", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("PUT foo = %d, want 400", code)
	}
}

func TestArtifactOpenRecorded(t *testing.T) {
	_, ana, bia, _ := team(t)
	art := createArtifact(t, ana, linkBody("Painel", "https://example.com/x"))
	opens := func() int { return countRows(t, "SELECT count(*) FROM artifact_opens WHERE artifact_id = $1", art.ID) }
	for i := 0; i < 2; i++ {
		if code := bia.do("GET", artifactPath(art.ID, ""), nil, nil); code != http.StatusOK {
			t.Fatalf("GET = %d", code)
		}
	}
	if n := opens(); n != 1 {
		t.Fatalf("opens = %d, want 1", n)
	}
	if code := ana.do("PUT", artifactPath(art.ID, ""), map[string]any{"name": "Painel 2", "summary": "x", "tags": []string{}}, nil); code != http.StatusOK {
		t.Fatalf("PUT = %d", code)
	}
	if n := opens(); n != 1 {
		t.Fatalf("opens after edit = %d, want 1", n)
	}
	exec(t, "UPDATE artifact_opens SET opened_at = now() - interval '31 minutes'")
	bia.do("GET", artifactPath(art.ID, ""), nil, nil)
	if n := opens(); n != 2 {
		t.Fatalf("opens after 31 min = %d, want 2", n)
	}
}

func TestDeleteArtifactClearsFavorites(t *testing.T) {
	_, ana, bia, _ := team(t)
	art := createArtifact(t, ana, linkBody("Painel", "https://example.com/x"))
	if code := bia.do("PUT", fmt.Sprintf("/api/favorites/artifact/%d", art.ID), nil, nil); code != http.StatusNoContent {
		t.Fatalf("PUT = %d", code)
	}
	if code := ana.do("DELETE", artifactPath(art.ID, ""), nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if n := countRows(t, "SELECT count(*) FROM favorites WHERE kind = 'artifact'"); n != 0 {
		t.Fatalf("favorites = %d, want 0", n)
	}
}
