package appdb

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

func admin(t *testing.T) (*Admin, string) {
	t.Helper()
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dev)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	u, _ := url.Parse(dev)
	a := New(pool, u.Host, engine.NewPodman("", engine.WithExecAllowed("ziv-db")), "ziv-db", u.User.Username())
	a.Prefix = "apptest_"
	return a, dev
}

// connectAs tries to open db as user (on the host port, as a test can't use host.containers.internal).
func connectAs(dev, user, pw, db string) error {
	u, _ := url.Parse(dev)
	u.User = url.UserPassword(user, pw)
	u.Path = "/" + db
	c, err := pgx.Connect(context.Background(), u.String())
	if err == nil {
		c.Close(context.Background())
	}
	return err
}

func TestEnsureIsolatesApps(t *testing.T) {
	ctx := context.Background()
	a, dev := admin(t)
	one, two := a.Name(1), a.Name(2)
	pw1, pw2 := NewPassword(), NewPassword()
	for name, pw := range map[string]string{one: pw1, two: pw2} {
		if err := a.Ensure(ctx, name, pw); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { a.Drop(ctx, name) })
	}
	if err := a.Ensure(ctx, one, pw1); err != nil { // de novo: não pode falhar
		t.Fatal(err)
	}
	if err := connectAs(dev, one, pw1, one); err != nil {
		t.Fatalf("own db: %v", err)
	}
	if err := connectAs(dev, one, pw1, two); err == nil {
		t.Fatal("app 1 opened app 2's database")
	}
	hub := strings.TrimPrefix(mustURL(dev).Path, "/")
	if err := a.LockDown(ctx, hub); err != nil {
		t.Fatal(err)
	}
	if err := connectAs(dev, one, pw1, hub); err == nil {
		t.Fatal("app 1 opened the Hub's database")
	}
}

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }

func TestRejectsBadNames(t *testing.T) {
	a, _ := admin(t)
	if err := a.Ensure(context.Background(), "x; DROP DATABASE apphub", "p"); err == nil {
		t.Fatal("accepted a dangerous name")
	}
}

func TestDumpRestore(t *testing.T) {
	ctx := context.Background()
	a, dev := admin(t)
	if a.eng.Available(ctx) != nil {
		t.Skip("podman indisponível")
	}
	name, pw := a.Name(3), NewPassword()
	if err := a.Ensure(ctx, name, pw); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Drop(ctx, name) })
	u := mustURL(dev)
	u.User, u.Path = url.UserPassword(name, pw), "/"+name
	c, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	c.Exec(ctx, "CREATE TABLE notas (t text); INSERT INTO notas VALUES ('antes')")
	var buf bytes.Buffer
	if err := a.Dump(ctx, name, &buf); err != nil || buf.Len() == 0 {
		t.Fatalf("dump %v (%d bytes)", err, buf.Len())
	}
	c.Exec(ctx, "UPDATE notas SET t = 'depois'")
	c.Close(ctx)
	if err := a.Restore(ctx, name, &buf); err != nil {
		t.Fatal(err)
	}
	c, _ = pgx.Connect(ctx, u.String())
	defer c.Close(ctx)
	var got string
	c.QueryRow(ctx, "SELECT t FROM notas").Scan(&got)
	if got != "antes" {
		t.Fatalf("after restore %q", got)
	}
	if n, err := a.Size(ctx, name); err != nil || n == 0 {
		t.Fatalf("size %d %v", n, err)
	}
}

func TestFakeRoundTrip(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	f.Ensure(ctx, "app_1", "pw")
	f.Data["app_1"] = "x"
	var buf bytes.Buffer
	if err := f.Dump(ctx, "app_1", &buf); err != nil {
		t.Fatal(err)
	}
	f.Data["app_1"] = "y"
	if err := f.Restore(ctx, "app_1", &buf); err != nil || f.Data["app_1"] != "x" {
		t.Fatalf("restore %v %q", err, f.Data["app_1"])
	}
	f.Drop(ctx, "app_1")
	if len(f.DBs) != 0 {
		t.Fatal("not dropped")
	}
}

// A copy is loaded all or nothing: one transaction, stopping at the first error.
func TestRestoreIsAllOrNothing(t *testing.T) {
	f := engine.NewFake()
	a := New(nil, "db:5432", f, "ziv-db", "postgres")
	if err := a.Restore(context.Background(), "app_1", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	calls := f.CallsCopy()
	if len(calls) != 1 || !strings.Contains(calls[0], "pg_restore") || !strings.Contains(calls[0], " --single-transaction") || !strings.Contains(calls[0], " --exit-on-error") {
		t.Fatalf("%v", calls)
	}
}
