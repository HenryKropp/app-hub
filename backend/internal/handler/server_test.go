package handler

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestServerNeedsManageServer(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	if code := c.do("GET", "/api/server", nil, nil); code != 200 {
		t.Fatalf("admin %d", code)
	}
	createPersonAs(t, c, "Ana", "ana@alis.com", "criador")
	loginAs(t, c, "ana@alis.com")
	if code := c.do("GET", "/api/server", nil, nil); code != http.StatusForbidden {
		t.Fatalf("criador %d", code)
	}
	for _, p := range [][2]string{{"PUT", "/api/server/cost"}, {"PUT", "/api/server/apps/1"}, {"POST", "/api/server/apps/1/stop"}, {"POST", "/api/server/apps/1/start"}} {
		if code := c.do(p[0], p[1], map[string]int64{"monthly_cost_cents": 1}, nil); code != http.StatusForbidden {
			t.Fatalf("criador %s %s %d", p[0], p[1], code)
		}
	}
}

func TestServerCostAndApps(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	if code := c.do("PUT", "/api/server/cost", map[string]int64{"monthly_cost_cents": 30000}, nil); code != 200 {
		t.Fatalf("cost %d", code)
	}
	if code := c.do("PUT", "/api/server/cost", map[string]int64{"monthly_cost_cents": -1}, nil); code != 400 {
		t.Fatalf("negative cost %d", code)
	}
	// A body without the field (a typo, a wrong name) must not save R$ 0.
	for _, body := range []any{map[string]int64{"cents": 1}, map[string]any{}, map[string]any{"monthly_cost_cents": nil}} {
		var e struct {
			Error string `json:"error"`
		}
		if code := c.do("PUT", "/api/server/cost", body, &e); code != 400 || !strings.Contains(e.Error, "custo") {
			t.Fatalf("body %v: %d %q", body, code, e.Error)
		}
	}
	testPool.Exec(t.Context(), "INSERT INTO shared_usage (app_id, at, mem_bytes, cpu_pct) SELECT $1, date_trunc('minute', now()) - (g || ' minutes')::interval, 200000000, 3 FROM generate_series(0, 59) g", id)
	var out struct {
		MonthlyCostCents int64 `json:"monthly_cost_cents"`
		Apps             []struct {
			AppID     int64   `json:"app_id"`
			HoursOn   float64 `json:"hours_on"`
			CostCents int64   `json:"cost_cents"`
			MaxMemMB  int64   `json:"max_mem_mb"`
		} `json:"apps"`
	}
	c.do("GET", "/api/server", nil, &out)
	if out.MonthlyCostCents != 30000 || len(out.Apps) != 1 || out.Apps[0].HoursOn != 1 || out.Apps[0].MaxMemMB != 190 || out.Apps[0].CostCents <= 0 {
		t.Fatalf("%+v", out)
	}
}

func TestServerStopsAnyApp(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	waitShared(t, c, id, "running")
	if code := c.do("POST", "/api/server/apps/"+itoa(id)+"/stop", nil, nil); code != 200 {
		t.Fatalf("stop %d", code)
	}
	waitShared(t, c, id, "stopped")
}

func TestServerAppSettings(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	path := "/api/server/apps/" + itoa(id)
	if code := c.do("PUT", path, map[string]any{"memory_mb": 64, "cpus": 1, "idle_minutes": 30}, nil); code != 400 {
		t.Fatalf("too little memory %d", code)
	}
	var out struct {
		NeedsRestart bool `json:"needs_restart"`
	}
	if code := c.do("PUT", path, map[string]any{"memory_mb": 1024, "cpus": 2, "always_on": true, "idle_minutes": 60}, &out); code != 200 || out.NeedsRestart {
		t.Fatalf("save %d %+v", code, out)
	}
	var mb, idle int
	var always bool
	testPool.QueryRow(t.Context(), "SELECT memory_mb, always_on, idle_minutes FROM shared_apps WHERE app_id = $1", id).Scan(&mb, &always, &idle)
	if mb != 1024 || !always || idle != 60 {
		t.Fatalf("row %d %v %d", mb, always, idle)
	}
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	waitShared(t, c, id, "running")
	if code := c.do("PUT", path, map[string]any{"memory_mb": 768, "cpus": 1, "idle_minutes": 30}, &out); code != 200 || !out.NeedsRestart {
		t.Fatalf("running %d %+v", code, out)
	}
}

func TestServerLogsDBSizeError(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	testPool.Exec(t.Context(), "UPDATE shared_apps SET db_name = 'app_notas' WHERE app_id = $1", id)
	d.dbs.FailSize = errors.New("size boom")
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)
	if code := c.do("GET", "/api/server", nil, nil); code != 200 {
		t.Fatalf("server %d", code)
	}
	if !strings.Contains(buf.String(), "size boom") {
		t.Fatalf("Size error not logged: %q", buf.String())
	}
}

// A limits save never turns the database flag off while the app has a database.
func TestServerSettingsKeepWantsDB(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	if _, err := testPool.Exec(t.Context(), "UPDATE shared_apps SET db_name = 'app_notas', wants_db = true WHERE app_id = $1", id); err != nil {
		t.Fatal(err)
	}
	if code := c.do("PUT", "/api/server/apps/"+itoa(id), map[string]any{"memory_mb": 1024, "cpus": 1, "idle_minutes": 30}, nil); code != 200 {
		t.Fatalf("save %d", code)
	}
	var wants bool
	testPool.QueryRow(t.Context(), "SELECT wants_db FROM shared_apps WHERE app_id = $1", id).Scan(&wants)
	if !wants {
		t.Fatal("wants_db turned off while the app has a database")
	}
}
