package handler

import (
	"context"
	"testing"
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
