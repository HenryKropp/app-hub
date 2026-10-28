package handler

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database"
)

// TestMigrationDropsSecretFilesFromHistory fills versions with .env files,
// applies 010 and checks they and their contents are gone, examples stay.
// Everything happens in a scratch schema inside a transaction that is undone.
func TestMigrationDropsSecretFilesFromHistory(t *testing.T) {
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `CREATE SCHEMA scratch_010; SET LOCAL search_path = scratch_010, public`); err != nil {
		t.Fatal(err)
	}
	names, sqls, err := database.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	var m010 string
	for _, n := range names {
		if strings.HasSuffix(n, "010_versions_without_secrets.sql") {
			m010 = sqls[n]
			continue
		}
		if _, err := tx.Exec(ctx, sqls[n]); err != nil {
			t.Fatalf("%s: %v", n, err)
		}
	}
	if m010 == "" {
		t.Fatal("010_versions_without_secrets.sql not found")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO people (name, email, password_hash, role_id) VALUES ('Ada', 'ada@x.com', '', (SELECT id FROM roles WHERE key = 'admin'));
		INSERT INTO apps (slug, name, kind, folder_path, owner_id, manifest, created_by, area_id)
			VALUES ('a', 'A', 'reference', '/a', 1, '{}', 1, (SELECT id FROM areas LIMIT 1));
		INSERT INTO code_blobs (sha256, content, size) VALUES ('main', 'print()', 7), ('env', 'KEY=segredo', 11), ('ex', 'KEY=', 4);
		INSERT INTO app_versions (app_id, number, origin, files, summary) VALUES
			(1, 1, 'original', '[{"path":"main.py","sha256":"main"},{"path":".env","sha256":"env"},{"path":".env.example","sha256":"ex"}]', '{}'),
			(1, 2, 'folder', '[{"path":"cfg/Prod.env","sha256":"env"}]', '{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, m010); err != nil {
		t.Fatalf("010: %v", err)
	}
	var v1, v2 string
	if err := tx.QueryRow(ctx, `SELECT
		(SELECT string_agg(f->>'path', ',') FROM app_versions v, jsonb_array_elements(v.files) f WHERE v.number = 1),
		(SELECT files::text FROM app_versions WHERE number = 2)`).Scan(&v1, &v2); err != nil {
		t.Fatal(err)
	}
	if v1 != "main.py,.env.example" || v2 != "[]" {
		t.Fatalf("v1 %q v2 %q", v1, v2)
	}
	rows, err := tx.Query(ctx, `SELECT sha256 FROM code_blobs ORDER BY sha256`)
	if err != nil {
		t.Fatal(err)
	}
	var blobs []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		blobs = append(blobs, s)
	}
	rows.Close()
	if want := []string{"ex", "main"}; !slices.Equal(blobs, want) {
		t.Fatalf("blobs %v, want %v", blobs, want)
	}
}
