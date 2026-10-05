package handler

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database"
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

// TestMigrationMovesOldData runs the real migrations 001-006, fills the old
// shape (role text, free-text areas), then applies 007 and checks the result.
// Everything happens in a scratch schema inside a transaction that is undone.
func TestMigrationMovesOldData(t *testing.T) {
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `CREATE SCHEMA scratch_007; SET LOCAL search_path = scratch_007, public`); err != nil {
		t.Fatal(err)
	}
	names, sqls, err := database.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	var m007 string
	for _, n := range names {
		if strings.HasSuffix(n, "007_access.sql") {
			m007 = sqls[n]
			continue
		}
		if _, err := tx.Exec(ctx, sqls[n]); err != nil {
			t.Fatalf("%s: %v", n, err)
		}
	}
	if m007 == "" {
		t.Fatal("007_access.sql not found")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO people (name, email, password_hash, role) VALUES
			('Ada', 'ada@x.com', '', 'admin'), ('Cris', 'cris@x.com', '', 'criador'), ('Ugo', 'ugo@x.com', '', 'usuario');
		INSERT INTO apps (slug, name, kind, area, folder_path, owner_id, manifest, created_by) VALUES
			('sem', 'Sem', 'reference', '', '/a/sem', 1, '{"area": ""}', 1),
			('mesa', 'Mesa', 'reference', 'Mesa de Operações', '/a/mesa', 1, '{"area": "Mesa de Operações"}', 1),
			('res', 'Res', 'reference', 'Research', '/a/res', 1, '{"name": "Res"}', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, m007); err != nil {
		t.Fatalf("007: %v", err)
	}

	rows, err := tx.Query(ctx, `SELECT p.email, r.key FROM people p JOIN roles r ON r.id = p.role_id ORDER BY p.id`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var e, k string
		if err := rows.Scan(&e, &k); err != nil {
			t.Fatal(err)
		}
		got = append(got, e+"="+k)
	}
	rows.Close()
	if want := []string{"ada@x.com=admin", "cris@x.com=criador", "ugo@x.com=usuario"}; !slices.Equal(got, want) {
		t.Fatalf("roles = %v", got)
	}

	check := func(slug, area string) {
		t.Helper()
		var col, mf, linked string
		var restricted bool
		err := tx.QueryRow(ctx, `SELECT a.area, COALESCE(a.manifest->>'area', '<missing>'), ar.name, a.restricted
			FROM apps a JOIN areas ar ON ar.id = a.area_id WHERE a.slug = $1`, slug).Scan(&col, &mf, &linked, &restricted)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if col != area || mf != area || linked != area || restricted {
			t.Fatalf("%s: area=%q manifest=%q linked=%q restricted=%v, want %q", slug, col, mf, linked, restricted, area)
		}
	}
	check("sem", "Sem área")
	check("mesa", "Mesa de Operações")
	check("res", "Research")
}
