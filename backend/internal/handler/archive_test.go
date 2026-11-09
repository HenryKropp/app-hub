package handler

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// archiveTeam: admin Henry (1), criador Ana (2), usuário Bia (3); one app
// owned by Henry in Research (area 1). It also returns the handler.
func archiveTeam(t *testing.T) (admin, ana, bia *client, h *Handler, appID int64) {
	t.Helper()
	admin, h = newPoolClient(t)
	setupAdmin(t, admin)
	createPersonAs(t, admin, "Ana", "ana@alis.com", "criador")
	createPersonAs(t, admin, "Bia", "bia@alis.com", "usuario")
	ana, bia = admin.fresh(), admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	loginAs(t, bia, "bia@alis.com")
	appID = insertApp(t, filesPrepManifest())
	return
}

type archivedRow struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DaysLeft       int    `json:"days_left"`
	ArchivedByName string `json:"archived_by_name"`
	OwnerName      string `json:"owner_name"`
}

func TestArchivePermissions(t *testing.T) {
	admin, ana, bia, _, id := archiveTeam(t)
	var e struct{ Error string }
	// Bia only uses the app; Ana creates apps but this one is not hers.
	for _, c := range []*client{bia, ana} {
		if code := c.do("POST", appPath(id, "/archive"), nil, &e); code != http.StatusForbidden || !strings.Contains(e.Error, "responsável") {
			t.Fatalf("non-owner archive: %d %q", code, e.Error)
		}
	}
	// A restricted app Bia cannot see answers 404, as everywhere else.
	restrict(t, id)
	if code := bia.do("POST", appPath(id, "/archive"), nil, nil); code != http.StatusNotFound {
		t.Fatalf("hidden app archive: %d", code)
	}
	exec(t, "UPDATE apps SET restricted = false WHERE id = $1", id)
	// The owner may archive their own app.
	ownerOf(t, id, 2)
	var out archivedRow
	if code := ana.do("POST", appPath(id, "/archive"), nil, &out); code != http.StatusOK || out.DaysLeft != 30 || out.ArchivedByName != "Ana" {
		t.Fatalf("owner archive: %d %+v", code, out)
	}
	if code := admin.do("POST", appPath(id, "/archive"), nil, &e); code != http.StatusConflict {
		t.Fatalf("second archive: %d", code)
	}
	// Bringing back follows the same rule.
	if code := bia.do("POST", appPath(id, "/unarchive"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("bia restore: %d", code)
	}
	if code := admin.do("POST", appPath(id, "/unarchive"), nil, nil); code != http.StatusOK {
		t.Fatalf("admin restore: %d", code)
	}
	if code := admin.do("POST", appPath(id, "/unarchive"), nil, &e); code != http.StatusConflict {
		t.Fatalf("restore of a live app: %d", code)
	}
}

func TestArchiveHidesAndRefusesRuns(t *testing.T) {
	admin, _, bia, h, id := archiveTeam(t)
	if code := bia.do("PUT", "/api/favorites/app/"+itoa(id), nil, nil); code != http.StatusNoContent && code != http.StatusOK {
		t.Fatalf("favorite %d", code)
	}
	if code := admin.do("POST", appPath(id, "/archive"), nil, nil); code != http.StatusOK {
		t.Fatalf("archive %d", code)
	}
	// Gone from the catalog and the Home of everyone.
	for _, c := range []*client{admin, bia} {
		var list []appSummary
		c.do("GET", "/api/apps", nil, &list)
		if len(list) != 0 {
			t.Fatalf("catalog still lists it: %+v", list)
		}
	}
	var home struct {
		Favorites []struct{ ID int64 } `json:"favorites"`
	}
	bia.do("GET", "/api/home", nil, &home)
	if len(home.Favorites) != 0 {
		t.Fatalf("home favorites: %+v", home.Favorites)
	}
	// The ficha and running answer the archive message.
	var e struct{ Error string }
	if code := bia.do("GET", appPath(id, ""), nil, &e); code != http.StatusGone || e.Error != msgArchived {
		t.Fatalf("ficha: %d %q", code, e.Error)
	}
	if code := admin.do("POST", appPath(id, "/runs"), map[string]string{}, &e); code != http.StatusGone || !strings.Contains(e.Error, "arquivado") {
		t.Fatalf("run: %d %q", code, e.Error)
	}
	// Runs queued some other way (schedule, Claude, a job already waiting) stop at the policy.
	if reason, err := h.executionBlock(context.Background(), id); err != nil || reason != msgArchivedRun {
		t.Fatalf("execution block: %q %v", reason, err)
	}
	// Only who may bring it back sees it in Arquivados.
	var rows []archivedRow
	admin.do("GET", "/api/apps/archived", nil, &rows)
	if len(rows) != 1 || rows[0].ID != id || rows[0].DaysLeft != 30 || rows[0].ArchivedByName != "Henry" || rows[0].OwnerName == "" {
		t.Fatalf("archived list: %+v", rows)
	}
	bia.do("GET", "/api/apps/archived", nil, &rows)
	if len(rows) != 0 {
		t.Fatalf("bia sees the archived list: %+v", rows)
	}
	// Back in the catalog after "Trazer de volta".
	if code := admin.do("POST", appPath(id, "/unarchive"), nil, nil); code != http.StatusOK {
		t.Fatalf("restore %d", code)
	}
	var list []appSummary
	bia.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || !list[0].Favorite {
		t.Fatalf("catalog after restore: %+v", list)
	}
	if reason, _ := h.executionBlock(context.Background(), id); reason != "" {
		t.Fatalf("still blocked after restore: %q", reason)
	}
}

func TestArchivePausesSchedulesAndKeepsThemOff(t *testing.T) {
	admin, _, _, _, id := archiveTeam(t)
	pairComputer(t, admin, "PC")
	var sid int64
	if err := testPool.QueryRow(context.Background(), `INSERT INTO schedules (app_id, person_id, computer_id, frequency, output_folder, next_run_at)
		VALUES ($1, 1, 1, '{"kind":"daily","at":"07:00"}', 'C:\saida', now()) RETURNING id`, id).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	if code := admin.do("POST", appPath(id, "/archive"), nil, nil); code != http.StatusOK {
		t.Fatalf("archive %d", code)
	}
	active := func() (bool, string) {
		var on bool
		var why string
		if err := testPool.QueryRow(context.Background(), "SELECT active, paused_reason FROM schedules WHERE id = $1", sid).Scan(&on, &why); err != nil {
			t.Fatal(err)
		}
		return on, why
	}
	if on, why := active(); on || why != schedPausedArchive {
		t.Fatalf("schedule after archive: %v %q", on, why)
	}
	var mine []map[string]any
	admin.do("GET", "/api/me/schedules", nil, &mine)
	if len(mine) != 0 {
		t.Fatalf("my schedules list an archived app: %+v", mine)
	}
	if code := admin.do("POST", "/api/schedules/"+itoa(sid)+"/resume", nil, nil); code != http.StatusNotFound {
		t.Fatalf("resume while archived: %d", code)
	}
	admin.do("POST", appPath(id, "/unarchive"), nil, nil)
	if on, _ := active(); on {
		t.Fatal("restoring must leave the schedule off")
	}
}

func TestArchivedFolderIsNotRegisteredAgain(t *testing.T) {
	admin, _, _, _, id := archiveTeam(t)
	dir := t.TempDir()
	exec(t, "UPDATE apps SET folder_path = $1 WHERE id = $2", dir, id)
	if code := admin.do("POST", appPath(id, "/archive"), nil, nil); code != http.StatusOK {
		t.Fatalf("archive %d", code)
	}
	// The scan points at the archived app instead of offering a new one.
	var scan struct {
		Items []scanItem `json:"items"`
	}
	if code := admin.do("POST", "/api/scan", map[string]string{"path": dir, "mode": "single"}, &scan); code != http.StatusOK || len(scan.Items) != 1 {
		t.Fatalf("scan %d %+v", code, scan)
	}
	if it := scan.Items[0]; it.ArchivedAppID != id || it.RegisteredAppID != 0 {
		t.Fatalf("scan item: archived %d registered %d", it.ArchivedAppID, it.RegisteredAppID)
	}
	// Publishing the folder again is refused; nothing in the folder changes.
	body := map[string]any{"folder_path": dir, "owner_id": 1, "manifest": filesPrepManifest()}
	var e struct{ Error, Code string }
	if code := admin.do("POST", "/api/apps", body, &e); code != http.StatusConflict || e.Code != "app_archived" || e.Error != msgArchivedFolder {
		t.Fatalf("register archived folder: %d %+v", code, e)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("the folder was touched: %v", entries)
	}
}

func TestArchivePurgeAfter30Days(t *testing.T) {
	admin, _, bia, h, id := archiveTeam(t)
	recent := insertApp(t, filesPrepManifestWithID("outro-app"))
	bia.do("PUT", "/api/favorites/app/"+itoa(id), nil, nil)
	someonesRun(t, id, 1)
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/app.exe", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	exec(t, "UPDATE apps SET folder_path = $1 WHERE id = $2", dir, id)
	for _, a := range []int64{id, recent} {
		if code := admin.do("POST", appPath(a, "/archive"), nil, nil); code != http.StatusOK {
			t.Fatalf("archive %d", code)
		}
	}
	exec(t, "UPDATE apps SET archived_at = now() - interval '31 days' WHERE id = $1", id)
	exec(t, "UPDATE apps SET archived_at = now() - interval '29 days' WHERE id = $1", recent)
	var rows []archivedRow
	admin.do("GET", "/api/apps/archived", nil, &rows)
	if len(rows) != 2 || rows[0].ID != recent || rows[0].DaysLeft != 1 || rows[1].DaysLeft != 0 {
		t.Fatalf("days left: %+v", rows)
	}
	if n := h.purgeArchived(context.Background(), time.Now()); n != 1 {
		t.Fatalf("purged %d apps, want 1", n)
	}
	count := func(sql string, args ...any) int {
		var n int
		if err := testPool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count("SELECT count(*) FROM apps WHERE id = $1", id) != 0 || count("SELECT count(*) FROM runs WHERE app_id = $1", id) != 0 {
		t.Fatal("the old archived app is still there")
	}
	if count("SELECT count(*) FROM favorites WHERE kind = 'app' AND item_id = $1", id) != 0 {
		t.Fatal("favorites of the purged app remain")
	}
	if count("SELECT count(*) FROM apps WHERE id = $1 AND archived_at IS NOT NULL", recent) != 1 {
		t.Fatal("an app archived 29 days ago was deleted")
	}
	if count("SELECT count(*) FROM security_audit WHERE action = 'archive.purge.completed'") == 0 {
		t.Fatal("purge not audited")
	}
	// The app's folder is never touched.
	if _, err := os.Stat(dir + "/app.exe"); err != nil {
		t.Fatalf("folder touched: %v", err)
	}
	// Brought back just before the cleanup: it stays.
	admin.do("POST", appPath(recent, "/unarchive"), nil, nil)
	exec(t, "UPDATE apps SET archived_at = NULL WHERE id = $1", recent)
	if n := h.purgeArchived(context.Background(), time.Now().Add(60*24*time.Hour)); n != 0 {
		t.Fatalf("purged a restored app: %d", n)
	}
}

func TestArchiveStopsServerVersionAndPurgeCleansIt(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	exec(t, "INSERT INTO app_versions (app_id, number, origin, files, summary) VALUES ($1, 1, 'original', '[]', '{}')", id)
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, nil); code != http.StatusAccepted {
		t.Fatalf("publish %d", code)
	}
	waitShared(t, c, id, "running")
	if code := c.do("POST", appPath(id, "/archive"), nil, nil); code != http.StatusOK {
		t.Fatalf("archive %d", code)
	}
	status := func() string {
		var s string
		if err := testPool.QueryRow(context.Background(), "SELECT status FROM shared_apps WHERE app_id = $1", id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if s := status(); s != "stopped" {
		t.Fatalf("server version after archive: %s", s)
	}
	if _, found, err := h.GatewayBackend().Lookup(context.Background(), "notas", false); err != nil || found {
		t.Fatalf("archived app still has an address: %v %v", found, err)
	}
	// Restoring keeps it off: the person turns it on again.
	if code := c.do("POST", appPath(id, "/unarchive"), nil, nil); code != http.StatusOK {
		t.Fatalf("restore %d", code)
	}
	if s := status(); s != "stopped" {
		t.Fatalf("server version after restore: %s", s)
	}
	// Archived again and past 30 days: the cleanup removes the server version too.
	c.do("POST", appPath(id, "/archive"), nil, nil)
	exec(t, "UPDATE apps SET archived_at = now() - interval '31 days' WHERE id = $1", id)
	if n := h.purgeArchived(context.Background(), time.Now()); n != 1 {
		t.Fatalf("purged %d", n)
	}
	var left int
	_ = testPool.QueryRow(context.Background(), "SELECT count(*) FROM shared_apps WHERE app_id = $1", id).Scan(&left)
	if left != 0 {
		t.Fatal("shared app row remains")
	}
}
