package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

// setRole replaces the permissions of a built-in role (admin, criador, usuario).
func setRole(t *testing.T, key string, perms ...string) {
	t.Helper()
	if perms == nil {
		perms = []string{}
	}
	exec(t, "UPDATE roles SET permissions = $2 WHERE key = $1", key, perms)
}

func makeLeader(t *testing.T, areaID, personID int64) {
	t.Helper()
	exec(t, "INSERT INTO area_leaders (area_id, person_id) VALUES ($1, $2)", areaID, personID)
}

func grant(t *testing.T, appID, personID int64, perm string) {
	t.Helper()
	exec(t, "INSERT INTO app_grants (app_id, person_id, permission) VALUES ($1, $2, $3)", appID, personID, perm)
}

func restrict(t *testing.T, appID int64) {
	t.Helper()
	exec(t, "UPDATE apps SET restricted = true WHERE id = $1", appID)
}

func ownerOf(t *testing.T, appID, personID int64) {
	t.Helper()
	exec(t, "UPDATE apps SET owner_id = $2 WHERE id = $1", appID, personID)
}

// someonesRun stores a finished run of personID (with a computer of theirs).
func someonesRun(t *testing.T, appID, personID int64) {
	t.Helper()
	var comp int64
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO computers (person_id, name, token_hash) VALUES ($1, 'PC', md5(random()::text)) RETURNING id`, personID).Scan(&comp); err != nil {
		t.Fatal(err)
	}
	exec(t, `INSERT INTO runs (app_id, person_id, computer_id, status, output_folder) VALUES ($1, $2, $3, 'done', 'C:\x')`, appID, personID, comp)
}

// team: admin Henry (1), criador Ana (2), usuário Bia (3); one app owned by Henry in Research (area 1).
func team(t *testing.T) (admin, ana, bia *client, appID int64) {
	t.Helper()
	admin, _ = newPoolClient(t)
	setupAdmin(t, admin)
	createPersonAs(t, admin, "Ana", "ana@alis.com", "criador")
	createPersonAs(t, admin, "Bia", "bia@alis.com", "usuario")
	ana, bia = admin.fresh(), admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	loginAs(t, bia, "bia@alis.com")
	appID = insertApp(t, filesPrepManifest())
	return
}

func TestRestrictedAppHidden(t *testing.T) {
	admin, _, bia, id := team(t)
	var list []appSummary
	bia.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || !list[0].Can.View || list[0].Can.Edit {
		t.Fatalf("open app for bia: %+v", list)
	}
	// Bia has the ficha open; then the app is restricted: the next request is 404.
	restrict(t, id)
	list = nil
	bia.do("GET", "/api/apps", nil, &list)
	if len(list) != 0 {
		t.Fatalf("restricted app listed: %+v", list)
	}
	for _, path := range []string{appPath(id, ""), appPath(id, "/runs"), appPath(id, "/history"), appPath(id, "/versions/1")} {
		if code := bia.do("GET", path, nil, nil); code != http.StatusNotFound {
			t.Fatalf("GET %s = %d", path, code)
		}
	}
	if code := bia.do("POST", appPath(id, "/check"), nil, nil); code != http.StatusNotFound {
		t.Fatalf("check restricted = %d", code)
	}
	if code := bia.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\x`}, nil); code != http.StatusNotFound {
		t.Fatalf("run restricted = %d", code)
	}
	exec(t, "INSERT INTO app_viewers (app_id, person_id) VALUES ($1, 3)", id)
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusOK {
		t.Fatalf("listed viewer = %d", code)
	}
	admin.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || !list[0].Restricted || !list[0].Can.Edit {
		t.Fatalf("admin list %+v", list)
	}
}

func TestCreatorEditsOnlyOwnApps(t *testing.T) {
	_, ana, _, id := team(t)
	var d appDetail
	ana.do("GET", appPath(id, ""), nil, &d)
	if d.Can.Edit || d.Can.Tech {
		t.Fatalf("criador on someone else's app: %+v", d.Can)
	}
	if d.AreaID != 1 || d.Restricted {
		t.Fatalf("detail area/restricted: %+v", d)
	}
	if code := ana.do("GET", appPath(id, "/history"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("history without tech = %d", code)
	}
	ownerOf(t, id, 2)
	ana.do("GET", appPath(id, ""), nil, &d)
	if !d.Can.Edit || !d.Can.Tech || d.Can.Runs {
		t.Fatalf("owner: %+v", d.Can)
	}
}

func TestGrantGivesOneThing(t *testing.T) {
	_, _, bia, id := team(t)
	grant(t, id, 3, "tech")
	var d appDetail
	bia.do("GET", appPath(id, ""), nil, &d)
	if !d.Can.Tech || d.Can.Edit || d.Can.Restore {
		t.Fatalf("tech grant: %+v", d.Can)
	}
	if code := bia.do("GET", appPath(id, "/history"), nil, nil); code != http.StatusOK {
		t.Fatalf("history with tech = %d", code)
	}
}

func TestLeaderRunsVisibility(t *testing.T) {
	_, ana, bia, id := team(t)
	someonesRun(t, id, 3)
	var runs []runSummaryJSON
	ana.do("GET", appPath(id, "/runs"), nil, &runs)
	if len(runs) != 0 {
		t.Fatal("criador sees bia's run")
	}
	if code := ana.do("GET", "/api/runs/1", nil, nil); code != http.StatusNotFound {
		t.Fatalf("criador opens bia's run = %d", code)
	}
	makeLeader(t, 1, 2)
	ana.do("GET", appPath(id, "/runs"), nil, &runs)
	if len(runs) != 1 {
		t.Fatal("leader must see the area's runs")
	}
	if code := ana.do("GET", "/api/runs/1", nil, nil); code != http.StatusOK {
		t.Fatalf("leader opens bia's run = %d", code)
	}
	var all runListJSON
	if code := ana.do("GET", "/api/runs?scope=all", nil, &all); code != http.StatusOK || len(all.Items) != 1 {
		t.Fatalf("leader scope=all %d %+v", code, all)
	}
	// Bia sees no one else's runs: "everyone" is just her own, not an error.
	if code := bia.do("GET", "/api/runs?scope=all", nil, &all); code != http.StatusOK || len(all.Items) != 1 || all.Items[0].PersonID != 3 {
		t.Fatalf("bia scope=all = %d %+v", code, all)
	}
	var mine runListJSON
	ana.do("GET", "/api/runs", nil, &mine)
	if len(mine.Items) != 0 {
		t.Fatalf("scope=mine shows others' runs: %+v", mine)
	}
}

func TestRunsGrantOnlyThatApp(t *testing.T) {
	_, ana, _, id := team(t)
	other := filesPrepManifest()
	other.ID, other.Name = "outro-app", "Outro App"
	id2 := insertApp(t, other)
	someonesRun(t, id, 3)  // run 1
	someonesRun(t, id2, 3) // run 2
	grant(t, id, 2, "runs")
	var all runListJSON
	if code := ana.do("GET", "/api/runs?scope=all", nil, &all); code != http.StatusOK || len(all.Items) != 1 || all.Items[0].AppID != id {
		t.Fatalf("runs grant scope=all %d %+v", code, all)
	}
	if code := ana.do("GET", "/api/runs/2", nil, nil); code != http.StatusNotFound {
		t.Fatalf("run of another app = %d", code)
	}
	var h historyJSON
	if code := ana.do("GET", appPath(id, "/history?type=runs"), nil, &h); code != http.StatusForbidden {
		t.Fatalf("history needs tech even with runs = %d", code)
	}
	grant(t, id, 2, "tech")
	ana.do("GET", appPath(id, "/history?type=runs"), nil, &h)
	if len(h.Items) != 1 {
		t.Fatalf("history with runs grant: %+v", h)
	}
}

func TestOwnerDoesNotSeeOthersRuns(t *testing.T) {
	_, ana, _, id := team(t)
	ownerOf(t, id, 2)
	someonesRun(t, id, 3)
	var runs []runSummaryJSON
	ana.do("GET", appPath(id, "/runs"), nil, &runs)
	if len(runs) != 0 {
		t.Fatal("owner without the runs access sees others' runs")
	}
	var h historyJSON
	ana.do("GET", appPath(id, "/history?type=runs"), nil, &h)
	if len(h.Items) != 0 {
		t.Fatalf("owner history shows others' runs: %+v", h)
	}
	var run map[string]any
	if code := ana.do("GET", "/api/runs/1", nil, &run); code != http.StatusNotFound {
		t.Fatalf("someone else's run = %d", code)
	}
}

func TestRegisterNeedsPermissionOrLeadership(t *testing.T) {
	_, _, bia, _ := team(t)
	if code := bia.do("POST", "/api/scan", map[string]string{"path": t.TempDir(), "mode": "single"}, nil); code != http.StatusForbidden {
		t.Fatalf("usuario scan = %d", code)
	}
	m := manifest.Manifest{Name: "Novo", Kind: manifest.KindScript, Language: "python", Entrypoint: "main.py", Area: "Research"}
	if code := bia.do("POST", "/api/apps", map[string]any{"folder_path": t.TempDir(), "owner_id": 3, "manifest": m}, nil); code != http.StatusForbidden {
		t.Fatalf("usuario publish = %d", code)
	}
	makeLeader(t, 2, 3)
	if code := bia.do("POST", "/api/scan", map[string]string{"path": t.TempDir(), "mode": "single"}, nil); code != http.StatusOK {
		t.Fatalf("leader scan = %d", code)
	}
	setRole(t, "usuario", "register_apps")
	bia2 := bia.fresh()
	loginAs(t, bia2, "bia@alis.com")
	exec(t, "DELETE FROM area_leaders")
	if code := bia2.do("POST", "/api/scan", map[string]string{"path": t.TempDir(), "mode": "single"}, nil); code != http.StatusOK {
		t.Fatalf("role with register_apps scan = %d", code)
	}
}

func TestScanHidesRestrictedSimilar(t *testing.T) {
	_, ana, _, id := team(t)
	dir := t.TempDir()
	m := filesPrepManifest()
	m.ID = ""
	if err := manifest.WriteToFolder(dir, m); err != nil {
		t.Fatal(err)
	}
	var res scanResp
	ana.do("POST", "/api/scan", map[string]string{"path": dir, "mode": "single"}, &res)
	if len(res.Items) != 1 || len(res.Items[0].Similar) != 1 {
		t.Fatalf("open similar app must show: %+v", res)
	}
	restrict(t, id)
	ana.do("POST", "/api/scan", map[string]string{"path": dir, "mode": "single"}, &res)
	if len(res.Items) != 1 || len(res.Items[0].Similar) != 0 {
		t.Fatalf("restricted similar app leaked: %+v", res.Items[0].Similar)
	}
}

func TestRepublishNeedsEdit(t *testing.T) {
	admin, ana, _, _ := team(t)
	_, dir := codeApp(t, admin) // owned by Henry (1)
	m := manifest.Manifest{Name: "App Teste", Kind: manifest.KindScript, Language: "python", Entrypoint: "src/main.py", Area: "Research"}
	if code := ana.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 2, "manifest": m}, nil); code != http.StatusForbidden {
		t.Fatalf("criador republishing someone else's app = %d", code)
	}
	var a struct{ OwnerID int64 }
	if err := testPool.QueryRow(context.Background(), "SELECT owner_id FROM apps WHERE folder_path = $1", dir).Scan(&a.OwnerID); err != nil || a.OwnerID != 1 {
		t.Fatalf("app changed: %v %d", err, a.OwnerID)
	}
}

func TestRestoreAndDescribeNeedPermission(t *testing.T) {
	admin, ana, _, _ := team(t)
	id, _ := codeApp(t, admin)
	if code := ana.do("GET", appPath(id, "/versions/1/restore-preview"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("restore preview = %d", code)
	}
	if code := ana.do("POST", appPath(id, "/versions/1/restore"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("restore = %d", code)
	}
	if code := ana.do("PATCH", appPath(id, "/versions/1"), map[string]any{"description": "x"}, nil); code != http.StatusForbidden {
		t.Fatalf("describe = %d", code)
	}
	for _, s := range []string{"/versions/1", "/versions/1/changes", "/versions/1/files", "/versions/1/content?path=src/main.py"} {
		if code := ana.do("GET", appPath(id, s), nil, nil); code != http.StatusForbidden {
			t.Fatalf("GET %s without tech = %d", s, code)
		}
	}
	grant(t, id, 2, "restore")
	if code := ana.do("GET", appPath(id, "/versions/1/restore-preview"), nil, nil); code == http.StatusForbidden || code == http.StatusNotFound {
		t.Fatalf("restore with grant = %d", code)
	}
	if code := ana.do("GET", appPath(id, "/versions/1/files"), nil, nil); code != http.StatusOK {
		t.Fatalf("restore implies tech = %d", code)
	}
	if code := ana.do("PATCH", appPath(id, "/versions/1"), map[string]any{"description": "x"}, nil); code != http.StatusForbidden {
		t.Fatalf("restore grant does not allow describe = %d", code)
	}
}

func TestOnlyManagersCreatePeopleOrRevokeOthersComputers(t *testing.T) {
	_, ana, bia, _ := team(t)
	body := map[string]string{"name": "Caio", "email": "caio@alis.com", "password": "senha-forte-1", "role": "usuario"}
	if code := ana.do("POST", "/api/people", body, nil); code != http.StatusForbidden {
		t.Fatalf("criador creates person = %d", code)
	}
	someonesRun(t, 1, 3) // gives Bia a computer (id 1)
	if code := ana.do("DELETE", "/api/computers/1", nil, nil); code != http.StatusNotFound {
		t.Fatalf("revoke someone else's computer = %d", code)
	}
	setRole(t, "criador", "manage_people")
	if code := ana.do("POST", "/api/people", body, nil); code != http.StatusCreated {
		t.Fatalf("manage_people creates person = %d", code)
	}
	if code := ana.do("DELETE", "/api/computers/1", nil, nil); code != http.StatusNoContent {
		t.Fatalf("manager revokes computer = %d", code)
	}
	_ = bia
}

// A role with company-wide boxes but not see_all_apps does not reach a
// restricted app: not in the catalog, not openable, and its runs stay out of
// "De todos".
func TestCompanyWideRoleDoesNotRevealRestricted(t *testing.T) {
	_, ana, _, id := team(t)
	other := filesPrepManifest()
	other.ID, other.Name = "outro-app", "Outro App"
	open := insertApp(t, other)
	someonesRun(t, id, 3)   // run 1, restricted app
	someonesRun(t, open, 3) // run 2, open app
	restrict(t, id)
	setRole(t, "criador", "edit_any_app", "restore_any_app", "see_tech_any_app", "see_all_runs")
	var list []appSummary
	ana.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || list[0].ID != open || !list[0].Can.Edit || !list[0].Can.Runs {
		t.Fatalf("catalog: %+v", list)
	}
	if code := ana.do("GET", appPath(id, ""), nil, nil); code != http.StatusNotFound {
		t.Fatalf("restricted ficha = %d", code)
	}
	if code := ana.do("GET", "/api/runs/1", nil, nil); code != http.StatusNotFound {
		t.Fatalf("run of restricted app = %d", code)
	}
	var all runListJSON
	if code := ana.do("GET", "/api/runs?scope=all", nil, &all); code != http.StatusOK || len(all.Items) != 1 || all.Items[0].AppID != open {
		t.Fatalf("scope=all %d %+v", code, all)
	}
	// Listed as a viewer: now the boxes apply to it too.
	exec(t, "INSERT INTO app_viewers (app_id, person_id) VALUES ($1, 2)", id)
	ana.do("GET", "/api/apps", nil, &list)
	if len(list) != 2 {
		t.Fatalf("viewer catalog: %+v", list)
	}
	if code := ana.do("GET", "/api/runs?scope=all", nil, &all); code != http.StatusOK || len(all.Items) != 2 {
		t.Fatalf("viewer scope=all %d %+v", code, all)
	}
}

// Asking for everyone's runs with none in reach is an empty list, not an error.
func TestScopeAllWithNothingInReach(t *testing.T) {
	_, ana, _, id := team(t)
	someonesRun(t, id, 3)
	var areaID int64
	if err := testPool.QueryRow(context.Background(), "INSERT INTO areas (name) VALUES ('Vazia') RETURNING id").Scan(&areaID); err != nil {
		t.Fatal(err)
	}
	makeLeader(t, areaID, 2) // leads an area with no apps
	var all runListJSON
	if code := ana.do("GET", "/api/runs?scope=all", nil, &all); code != http.StatusOK || len(all.Items) != 0 {
		t.Fatalf("leader of an empty area scope=all %d %+v", code, all)
	}
}

func TestScanHidesRestrictedRegisteredID(t *testing.T) {
	_, ana, _, id := team(t)
	dir := t.TempDir()
	m := filesPrepManifest()
	if err := manifest.WriteToFolder(dir, m); err != nil {
		t.Fatal(err)
	}
	exec(t, "UPDATE apps SET folder_path = $2 WHERE id = $1", id, dir)
	var res scanResp
	ana.do("POST", "/api/scan", map[string]string{"path": dir, "mode": "single"}, &res)
	if len(res.Items) != 1 || res.Items[0].RegisteredAppID != id {
		t.Fatalf("open app registered id: %+v", res)
	}
	restrict(t, id)
	ana.do("POST", "/api/scan", map[string]string{"path": dir, "mode": "single"}, &res)
	if len(res.Items) != 1 || res.Items[0].RegisteredAppID != 0 {
		t.Fatalf("restricted app id leaked: %+v", res.Items)
	}
}
