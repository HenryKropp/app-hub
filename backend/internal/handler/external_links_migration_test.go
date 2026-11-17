package handler

import (
	"context"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database"
)

// The 029 migration gives the admin role share_external (and only admin).
func TestAdminRoleHasShareExternal(t *testing.T) {
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, resetSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE roles SET permissions = array_remove(permissions, 'share_external')`); err != nil {
		t.Fatal(err)
	}
	_, files, err := database.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	sql, ok := files["migrations/029_external_links.sql"]
	if !ok {
		t.Fatal("migration 029_external_links.sql missing")
	}
	if _, err := tx.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]bool{"admin": true, "criador": false, "usuario": false} {
		var has bool
		if err := tx.QueryRow(ctx, `SELECT 'share_external' = ANY(permissions) FROM roles WHERE key=$1`, key).Scan(&has); err != nil {
			t.Fatal(err)
		}
		if has != want {
			t.Fatalf("role %s share_external=%v, want %v", key, has, want)
		}
	}
}
