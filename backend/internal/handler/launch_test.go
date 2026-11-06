package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/jackc/pgx/v5/pgtype"
)

// launchFixture returns an app, the admin and the admin's paired computer,
// as the scheduler would load them.
func launchFixture(t *testing.T, h *Handler, c *client, m manifest.Manifest) (sqlc.App, sqlc.Person, sqlc.Computer) {
	t.Helper()
	setupAdmin(t, c)
	id := insertApp(t, m)
	pairComputer(t, c, "PC")
	ctx := context.Background()
	a, err := h.q.GetApp(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.q.GetPerson(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	comp, err := h.q.GetComputer(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	return a, p, comp
}

func launchReq(a sqlc.App, p sqlc.Person, comp sqlc.Computer) launchRequest {
	var m manifest.Manifest
	_ = json.Unmarshal(a.Manifest, &m)
	return launchRequest{App: a, Person: p, Computer: comp, Manifest: m, OutputFolder: `C:\saida`, Origin: "schedule"}
}

func TestLaunchRunSetsScheduleOrigin(t *testing.T) {
	c, h := newPoolClient(t)
	a, p, comp := launchFixture(t, h, c, filesPrepManifest())
	ctx := context.Background()
	var sid int64
	if err := testPool.QueryRow(ctx, `INSERT INTO schedules (app_id, person_id, computer_id, frequency, output_folder, next_run_at)
		VALUES ($1, 1, 1, '{"kind":"daily","at":"07:00"}', 'C:\saida', now()) RETURNING id`, a.ID).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	req := launchReq(a, p, comp)
	req.ScheduleID = pgtype.Int8{Int64: sid, Valid: true}
	run, lerr, err := h.launchRun(ctx, req)
	if err != nil || lerr != nil {
		t.Fatalf("%v %+v", err, lerr)
	}
	if run.Origin != "schedule" || run.ScheduleID.Int64 != sid || run.ComputerID != comp.ID {
		t.Fatalf("%+v", run)
	}
	var jobs int
	_ = testPool.QueryRow(ctx, `SELECT count(*) FROM agent_jobs WHERE run_id=$1 AND kind='run'`, run.ID).Scan(&jobs)
	if jobs != 1 {
		t.Fatalf("jobs %d", jobs)
	}
}

func TestLaunchRunBlockedWhenAppSuspended(t *testing.T) {
	c, h := newPoolClient(t)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste", BasePath: ""})
	a, p, comp := launchFixture(t, h, c, filesPrepManifest())
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `INSERT INTO app_security (app_id, approved, suspended) VALUES ($1, true, true)
		ON CONFLICT (app_id) DO UPDATE SET suspended = true`, a.ID); err != nil {
		t.Fatal(err)
	}
	_, lerr, err := h.launchRun(ctx, launchReq(a, p, comp))
	if err != nil {
		t.Fatal(err)
	}
	if lerr == nil || lerr.Code != "blocked" || lerr.Msg != "Este app foi suspenso pela empresa." {
		t.Fatalf("%+v", lerr)
	}
}

func TestLaunchRunFolderBusy(t *testing.T) {
	c, h := newPoolClient(t)
	a, p, comp := launchFixture(t, h, c, filesPrepManifest())
	ctx := context.Background()
	if _, lerr, err := h.launchRun(ctx, launchReq(a, p, comp)); err != nil || lerr != nil {
		t.Fatalf("%v %+v", err, lerr)
	}
	_, lerr, err := h.launchRun(ctx, launchReq(a, p, comp))
	if err != nil || lerr == nil || lerr.Code != "folder_busy" {
		t.Fatalf("%v %+v", err, lerr)
	}
}
