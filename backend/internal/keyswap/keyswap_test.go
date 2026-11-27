package keyswap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

const testDB = "ziv_keyswap_test"

// testPool opens a database of its own on the dev server, with one company
// schema holding the two places the vault writes to.
func testPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		t.Skip("DATABASE_URL not configured")
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dev)
	if err != nil {
		t.Fatal(err)
	}
	var exists bool
	_ = c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, testDB).Scan(&exists)
	if !exists {
		if _, err := c.Exec(ctx, "CREATE DATABASE "+testDB); err != nil {
			t.Fatal(err)
		}
	}
	c.Close(ctx)
	u, _ := url.Parse(dev)
	u.Path = "/" + testDB
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("kswap_%d", time.Now().UnixNano())
	s := pgx.Identifier{schema}.Sanitize()
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS public.workspace_registry (slug TEXT PRIMARY KEY, name TEXT NOT NULL, schema_name TEXT UNIQUE NOT NULL, ready BOOLEAN NOT NULL DEFAULT FALSE)`,
		`CREATE SCHEMA ` + s,
		`CREATE TABLE ` + s + `.app_config (app_id BIGINT NOT NULL, name TEXT NOT NULL, value_cipher BYTEA, PRIMARY KEY (app_id, name))`,
		`CREATE TABLE ` + s + `.shared_backups (id BIGSERIAL PRIMARY KEY, path TEXT NOT NULL)`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.workspace_registry(slug,name,schema_name,ready) VALUES($1,$1,$1,TRUE)`, schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DROP SCHEMA `+s+` CASCADE`)
		pool.Exec(context.Background(), `DELETE FROM public.workspace_registry WHERE slug=$1`, schema)
	})
	return pool, schema
}

// After the key change every value and copy opens with the new key alone,
// and a second run has nothing left to do.
func TestRewrapMovesEverythingToTheNewKey(t *testing.T) {
	pool, schema := testPool(t)
	ctx := context.Background()
	a, b := vault.NewKey(), vault.NewKey()
	va, _ := vault.New(a)
	s := pgx.Identifier{schema}.Sanitize()
	for i, plain := range []string{"senha-1", "senha-2"} {
		sealed, _ := va.Seal(plain)
		if _, err := pool.Exec(ctx, `INSERT INTO `+s+`.app_config VALUES($1,'K',$2)`, i+1, sealed); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO `+s+`.app_config VALUES(9,'PLAIN',NULL)`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	copyPath := filepath.Join(dir, "copia.dump")
	var buf bytes.Buffer
	w, _ := va.SealStream(&buf)
	w.Write([]byte("conteúdo da cópia"))
	w.Close()
	os.WriteFile(copyPath, buf.Bytes(), 0o600)
	missing := filepath.Join(dir, "sumiu.dump")
	for _, p := range []string{copyPath, missing} {
		if _, err := pool.Exec(ctx, `INSERT INTO `+s+`.shared_backups(path) VALUES($1)`, p); err != nil {
			t.Fatal(err)
		}
	}

	both, _ := vault.New(b, a)
	rep, err := Rewrap(ctx, pool, both)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Values != 2 || rep.Copies != 1 {
		t.Fatalf("report %+v", rep)
	}

	onlyB, _ := vault.New(b)
	rows, _ := pool.Query(ctx, `SELECT value_cipher FROM `+s+`.app_config WHERE value_cipher IS NOT NULL ORDER BY app_id`)
	var got []string
	for rows.Next() {
		var c []byte
		rows.Scan(&c)
		plain, err := onlyB.Open(c)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, plain)
	}
	if len(got) != 2 || got[0] != "senha-1" || got[1] != "senha-2" {
		t.Fatalf("values after change %v", got)
	}
	f, _ := os.Open(copyPath)
	r, err := onlyB.OpenStream(f)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(r)
	f.Close()
	if string(content) != "conteúdo da cópia" {
		t.Fatalf("copy after change %q", content)
	}
	if again, err := Rewrap(ctx, pool, both); err != nil || again.Values != 0 || again.Copies != 0 {
		t.Fatalf("second run %+v %v", again, err)
	}
}
