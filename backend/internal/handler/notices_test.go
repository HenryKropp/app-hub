package handler

import (
	"context"
	"net/http"
	"testing"
)

type noticeJSON struct {
	ID      int64   `json:"id"`
	AppID   int64   `json:"app_id"`
	AppName string  `json:"app_name"`
	Text    string  `json:"text"`
	ReadAt  *string `json:"read_at"`
}

func TestNoticesScope(t *testing.T) {
	admin, ana, bia, appID := team(t)
	ctx := context.Background()
	_ = ctx
	exec(t, "INSERT INTO notices (app_id, version, text) VALUES ($1, 2, 'Prep versão 2 no ar — testada: 1 arquivo salvo')", appID)
	var list []noticeJSON
	if code := admin.do("GET", "/api/notices", nil, &list); code != http.StatusOK || len(list) != 1 || list[0].AppName == "" {
		t.Fatalf("admin %d %+v", code, list)
	}
	for name, cl := range map[string]*client{"ana": ana, "bia": bia} {
		var other []noticeJSON
		if code := cl.do("GET", "/api/notices", nil, &other); code != http.StatusOK || len(other) != 0 {
			t.Fatalf("%s sees %d %+v", name, code, other)
		}
		if code := cl.do("POST", "/api/notices/"+itoa(list[0].ID)+"/read", nil, nil); code != http.StatusNotFound {
			t.Fatalf("%s read %d", name, code)
		}
	}
	// The owner sees the notices of their app.
	var anaID int64
	testPool.QueryRow(ctx, "SELECT id FROM people WHERE email='ana@exemplo.com'").Scan(&anaID)
	ownerOf(t, appID, anaID)
	var mine []noticeJSON
	if ana.do("GET", "/api/notices", nil, &mine); len(mine) != 1 {
		t.Fatalf("owner %+v", mine)
	}
	if code := ana.do("POST", "/api/notices/"+itoa(list[0].ID)+"/read", nil, nil); code != http.StatusNoContent {
		t.Fatalf("read %d", code)
	}
	mine = nil
	ana.do("GET", "/api/notices", nil, &mine)
	if len(mine) != 1 || mine[0].ReadAt == nil {
		t.Fatalf("after read %+v", mine)
	}
}
