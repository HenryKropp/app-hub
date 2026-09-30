package handler

import (
	"context"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Tests empty tables for real (newPoolClient), so they must never touch the
// database the Hub uses in development.
func TestUsesSeparateDatabase(t *testing.T) {
	var name string
	if err := testPool.QueryRow(context.Background(), "SELECT current_database()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != testDatabaseName {
		t.Fatalf("tests are using database %q, want %q", name, testDatabaseName)
	}
}

// ensureTestDatabase creates testDatabaseName on the same server as devURL
// (if missing) and returns its URL.
func ensureTestDatabase(ctx context.Context, devURL string) (string, error) {
	u, err := url.Parse(devURL)
	if err != nil {
		return "", err
	}
	conn, err := pgx.Connect(ctx, devURL)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", testDatabaseName).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+testDatabaseName); err != nil {
			return "", err
		}
	}
	u.Path = "/" + testDatabaseName
	return u.String(), nil
}
