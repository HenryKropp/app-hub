// Package sharedtest gives each shared/* package its own test database
// (the caller picks the name, e.g. "apphub_test_appdb"), so packages never
// race each other or the handler tests over apphub_test when `go test ./...`
// runs them in parallel.
package sharedtest

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database"
)

// Reset empties the tables the shared packages use and puts back the
// built-in admin (id 1, "Henry") and area Research (id 1).
const Reset = `
TRUNCATE shared_usage, shared_backups, app_config, shared_apps, shared_deployments, app_changes, app_versions,
         code_blobs, app_grants, app_viewers, area_grants, area_leaders, area_members, apps, sessions, people, areas
         RESTART IDENTITY CASCADE;
INSERT INTO areas (name) VALUES ('Research');
INSERT INTO people (name, email, password_hash, role_id)
SELECT 'Henry', 'henry@alis.com', 'x', id FROM roles WHERE key = 'admin';
`

var safeDBName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,40}$`)

// lockKey is the advisory lock that serializes create+migrate across packages.
const lockKey = 7301006

// Pool opens (creating and migrating if needed) the test database called
// name. It returns nil, nil when DATABASE_URL is not set, so callers can skip.
// Concurrent calls (same or different names) are serialized by an advisory
// lock on the admin connection.
func Pool(ctx context.Context, name string) (*pgxpool.Pool, error) {
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		return nil, nil
	}
	if !safeDBName.MatchString(name) {
		return nil, fmt.Errorf("sharedtest: invalid database name %q", name)
	}
	u, err := url.Parse(dev)
	if err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(ctx, dev)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return nil, err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockKey)

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
			var pe *pgconn.PgError
			if !errors.As(err, &pe) || pe.Code != "42P04" {
				return nil, err
			}
		}
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
