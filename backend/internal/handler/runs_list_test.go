package handler

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

func TestMigrationColumnsExist(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns WHERE table_name = 'runs' AND column_name = 'dismissed_at'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("dismissed_at: %d %v", n, err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM run_folders`).Scan(&n); err != nil {
		t.Fatalf("run_folders: %v", err)
	}
}

func finishRun(t *testing.T, c *client, token string) {
	t.Helper()
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	p := "/api/agent/jobs/" + strconv.FormatInt(job.ID, 10) + "/result"
	c.agent("POST", p, token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	c.agent("POST", p, token, agentproto.Result{Event: agentproto.EventFinished}, nil)
}

func TestActiveRunsAndDismiss(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")

	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{"output_folder": `D:\Saida`}, &run)
	var active []activeRunJSON
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 1 || active[0].ID != run.ID || active[0].Status != "waiting" || active[0].AppName != "Eq. Research Files Prep" || active[0].AppFolder != `C:\apps\eq-research-files-prep` || active[0].OutputFolder != `D:\Saida` {
		t.Fatalf("ativa %+v", active)
	}
	finishRun(t, c, token)
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 1 || active[0].Status != "no_files" {
		t.Fatalf("terminada continua até dispensar %+v", active)
	}
	dismiss := "/api/runs/" + strconv.FormatInt(run.ID, 10) + "/dismiss"
	if code := c.do("POST", dismiss, nil, nil); code != http.StatusNoContent {
		t.Fatalf("dismiss %d", code)
	}
	if code := c.do("POST", dismiss, nil, nil); code != http.StatusNoContent {
		t.Fatalf("dismiss again %d", code)
	}
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 0 {
		t.Fatalf("dispensada voltou %+v", active)
	}
}

func TestDismissOnlyOwnRun(t *testing.T) {
	admin, _ := newPoolClient(t)
	setupAdmin(t, admin)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, admin, "PC")
	var run runJSON
	admin.do("POST", appPath(app, "/runs"), map[string]string{}, &run)

	createPersonAs(t, admin, "Ana", "ana@alis.com", "usuario")
	ana := admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("POST", "/api/runs/"+strconv.FormatInt(run.ID, 10)+"/dismiss", nil, nil); code != http.StatusNotFound {
		t.Fatalf("Ana dispensou a do Henry: %d", code)
	}
	if code := ana.do("POST", "/api/runs/999/dismiss", nil, nil); code != http.StatusNotFound {
		t.Fatalf("inexistente %d", code)
	}
	var active []activeRunJSON
	ana.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 0 {
		t.Fatalf("Ana vê faixa do Henry %+v", active)
	}
	admin.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 1 {
		t.Fatalf("faixa do Henry sumiu %+v", active)
	}
}

func TestListRunsScopesAndPages(t *testing.T) {
	admin, h := newPoolClient(t)
	setupAdmin(t, admin)
	h.runsPage = 2
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, admin, "PC")
	for range 3 {
		admin.do("POST", appPath(app, "/runs"), map[string]string{}, nil)
	}
	var page runListJSON
	admin.do("GET", "/api/runs?scope=mine", nil, &page)
	if len(page.Items) != 2 || !page.More || page.Items[0].ID < page.Items[1].ID {
		t.Fatalf("página 1 %+v", page)
	}
	admin.do("GET", "/api/runs?scope=mine&before="+strconv.FormatInt(page.Items[1].ID, 10), nil, &page)
	if len(page.Items) != 1 || page.More {
		t.Fatalf("página 2 %+v", page)
	}

	createPersonAs(t, admin, "Ana", "ana@alis.com", "usuario")
	ana := admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("GET", "/api/runs?scope=all", nil, &page); code != http.StatusOK || len(page.Items) != 0 {
		t.Fatalf("all para usuário %d %+v", code, page)
	}
	ana.do("GET", "/api/runs?scope=mine", nil, &page)
	if len(page.Items) != 0 {
		t.Fatalf("Ana vê do Henry %+v", page)
	}
	admin.do("GET", "/api/runs?scope=all", nil, &page)
	if len(page.Items) != 2 || page.Items[0].PersonName != "Henry" || page.Items[0].PersonID != 1 || page.Items[0].OutputFolder == "" {
		t.Fatalf("all admin %+v", page)
	}
	if code := admin.do("GET", "/api/runs?before=x", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("before inválido %d", code)
	}
}
