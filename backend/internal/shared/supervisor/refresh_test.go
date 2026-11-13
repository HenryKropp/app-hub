package supervisor

import (
	"context"
	"testing"
	"time"
)

// waitIdle waits for the app's background job (the refresh) to finish.
func waitIdle(t *testing.T, busy func() bool) {
	t.Helper()
	for i := 0; i < 200 && busy(); i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if busy() {
		t.Fatal("refresh never finished")
	}
}

// A running app whose version in the air was built with an older recipe is
// rebuilt by the Ziv, once; then nothing more happens.
func TestRefreshRebuildsOldRecipe(t *testing.T) {
	e, s, _ := live(t)
	ctx := context.Background()
	old := e.Shared(t).CurrentDeploymentID.Int64
	if _, err := pool.Exec(ctx, "UPDATE shared_deployments SET recipe = 'FROM receita-antiga' WHERE id = $1", old); err != nil {
		t.Fatal(err)
	}
	s.Step(ctx)
	waitIdle(t, func() bool { return e.S.Busy(e.App.ID) })
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 == old || sa.Status != "running" {
		t.Fatalf("not rebuilt: %+v", sa)
	}
	d, err := e.Q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil {
		t.Fatal(err)
	}
	if d.CreatedBy.Valid || d.Recipe == "FROM receita-antiga" {
		t.Fatalf("%+v", d)
	}
	var n int
	s.Step(ctx)
	waitIdle(t, func() bool { return e.S.Busy(e.App.ID) })
	pool.QueryRow(ctx, "SELECT count(*) FROM shared_deployments WHERE app_id = $1", e.App.ID).Scan(&n)
	if n != 2 {
		t.Fatalf("%d deployments", n)
	}
}

// New code waiting to be published is the person's call: no refresh.
func TestRefreshSkipsUnpublishedCode(t *testing.T) {
	e, s, _ := live(t)
	ctx := context.Background()
	old := e.Shared(t).CurrentDeploymentID.Int64
	if _, err := pool.Exec(ctx, "UPDATE shared_deployments SET recipe = 'FROM receita-antiga' WHERE id = $1", old); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app_versions (app_id, number, origin, files, summary)
		VALUES ($1, 99, 'folder', '[]', '{}')`, e.App.ID); err != nil {
		t.Fatal(err)
	}
	s.Step(ctx)
	waitIdle(t, func() bool { return e.S.Busy(e.App.ID) })
	if e.Shared(t).CurrentDeploymentID.Int64 != old {
		t.Fatal("rebuilt code the person did not publish")
	}
}
