package handler

import (
	"context"
	"testing"
)

// The schedules table exists, and runs accept the "schedule" origin and the
// "missed" status.
func TestSchedulesTableAndMissedRun(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	ctx := context.Background()
	var sid int64
	if err := testPool.QueryRow(ctx, `INSERT INTO schedules (app_id, person_id, computer_id, frequency, output_folder, next_run_at)
		VALUES ($1, 1, 1, '{"kind":"daily","at":"07:00"}', 'C:\out', now()) RETURNING id`, app).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO runs (app_id, person_id, computer_id, output_folder, status, origin, schedule_id)
		VALUES ($1, 1, 1, 'C:\out', 'missed', 'schedule', $2)`, app, sid); err != nil {
		t.Fatal(err)
	}
}
