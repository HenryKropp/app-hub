package handler

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestMigrationAccessTables(t *testing.T) {
	ctx := context.Background()
	for _, table := range []string{"roles", "areas", "area_leaders", "area_members", "app_viewers", "app_grants", "area_grants"} {
		var ok bool
		if err := testPool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil || !ok {
			t.Fatalf("table %s missing (%v)", table, err)
		}
	}
	var n int
	if err := testPool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE key IN ('admin','criador','usuario')").Scan(&n); err != nil || n != 3 {
		t.Fatalf("built-in roles = %d (%v)", n, err)
	}
}

func TestLoggedPersonCarriesPermissions(t *testing.T) {
	c := newClient(t, false)
	admin := setupAdmin(t, c)
	if admin.RoleName != "Administrador" || !slices.Contains(admin.Permissions, "manage_roles") || !admin.SeesOthersRuns {
		t.Fatalf("admin = %+v", admin)
	}
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	var st struct {
		Person personJSON `json:"person"`
	}
	if code := bia.do("GET", "/api/auth/status", nil, &st); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if st.Person.RoleName != "Usuário" || len(st.Person.Permissions) != 0 || st.Person.SeesOthersRuns {
		t.Fatalf("bia = %+v", st.Person)
	}
}

func TestPublishNeedsAreaFromList(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	dir := t.TempDir()
	m := map[string]any{"name": "X", "kind": "reference", "area": "Inventada"}
	var out map[string]any
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &out); code != http.StatusBadRequest || out["error"] != "Escolha a área do app na lista." {
		t.Fatalf("code %d %v", code, out)
	}
}

func TestMigrationMovesOldData(t *testing.T) {
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Old shape: a role text column and free-text areas.
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE old_people (id int, role text);
		INSERT INTO old_people VALUES (1, 'criador');
		CREATE TEMP TABLE old_apps (id int, area text);
		INSERT INTO old_apps VALUES (1, ''), (2, 'Mesa de Operações');`); err != nil {
		t.Fatal(err)
	}
	var roleName string
	if err := tx.QueryRow(ctx, `SELECT r.name FROM old_people p JOIN roles r ON r.key = p.role`).Scan(&roleName); err != nil || roleName != "Criador" {
		t.Fatalf("role mapping %q %v", roleName, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO areas (name) SELECT DISTINCT area FROM old_apps WHERE area <> '' ON CONFLICT (name) DO NOTHING;
		INSERT INTO areas (name) SELECT 'Sem área' WHERE EXISTS (SELECT 1 FROM old_apps WHERE area = '') ON CONFLICT (name) DO NOTHING;`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM areas WHERE name IN ('Sem área', 'Mesa de Operações')`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("areas from old data = %d %v", n, err)
	}
}
