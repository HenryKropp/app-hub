package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func TestAdminHasManageServer(t *testing.T) {
	c := newClient(t, false)
	p := setupAdmin(t, c)
	if !slices.Contains(p.Permissions, "manage_server") {
		t.Fatalf("admin permissions %v", p.Permissions)
	}
}

func TestHubChangeShowsAsHub(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id := insertApp(t, filesPrepManifest())
	raw, _ := json.Marshal([]manifest.Change{{Field: "shared", Label: "Caiu"}})
	if err := h.q.InsertHubChange(context.Background(), sqlc.InsertHubChangeParams{AppID: id, Changes: raw}); err != nil {
		t.Fatal(err)
	}
	var hist struct {
		Items []struct {
			Ficha *struct {
				Person string `json:"person_name"`
			} `json:"ficha"`
		} `json:"items"`
	}
	if code := c.do("GET", "/api/apps/"+itoa(id)+"/history?type=ficha", nil, &hist); code != http.StatusOK {
		t.Fatalf("history %d", code)
	}
	if len(hist.Items) == 0 || hist.Items[0].Ficha == nil || hist.Items[0].Ficha.Person != "Hub" {
		t.Fatalf("history %+v", hist.Items)
	}
}
