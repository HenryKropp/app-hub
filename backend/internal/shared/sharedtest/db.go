// Package sharedtest gives the shared/* packages their own test database,
// so they never race the handler tests over apphub_test.
package sharedtest

import (
	"context"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database"
)

const Name = "apphub_test_shared"

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

// Pool opens (creating and migrating if needed) the shared test database.
// It returns nil when DATABASE_URL is not set, so callers can skip.
func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		return nil, nil
	}
	u, err := url.Parse(dev)
	if err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(ctx, dev)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", Name).Scan(&exists); err != nil {
		conn.Close(ctx)
		return nil, err
	}
	if !exists {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+Name); err != nil {
			conn.Close(ctx)
			return nil, err
		}
	}
	conn.Close(ctx)
	u.Path = "/" + Name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return nil, err
	}
	return pool, database.Migrate(ctx, pool)
}
