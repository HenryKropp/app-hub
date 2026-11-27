package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// Only Claude and this computer may receive an OAuth code, unless the server
// lists more hosts (OAUTH_REDIRECT_HOSTS).
func TestValidRedirectURIAllowlist(t *testing.T) {
	for _, ok := range []string{"http://localhost:33418/callback", "http://127.0.0.1:5000/x", "https://claude.ai/api/mcp/auth_callback", "https://x.claude.com/cb", "https://claude.com/cb"} {
		if !validRedirectURI(ok, nil) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"https://evil.com/cb", "https://claude.ai.evil.com/cb", "http://claude.ai/cb", "https://notclaude.ai/cb", "https://ok.example/cb"} {
		if validRedirectURI(bad, nil) {
			t.Errorf("%q accepted", bad)
		}
	}
	if !validRedirectURI("https://ok.example/cb", []string{"ok.example"}) || validRedirectURI("http://ok.example/cb", []string{"ok.example"}) {
		t.Error("extra host must be accepted over https only")
	}
}

// A client registered before the list existed, with another address, can
// neither start a login nor renew one.
func TestOAuthOldClientElsewhereIsRefused(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	ctx := context.Background()
	const elsewhere = "https://antigo.example/cb"
	cl, err := h.q.CreateMCPClient(ctx, sqlc.CreateMCPClientParams{ID: "antigo", Name: "Antigo", RedirectUris: []string{elsewhere}})
	if err != nil {
		t.Fatal(err)
	}
	if st, loc := authorize(t, c, authorizeQuery(cl.ID, elsewhere)); st != http.StatusBadRequest || loc != "" {
		t.Fatalf("authorize old client = %d %q", st, loc)
	}
	refresh := "ziv_refresh_antigo"
	if _, err := h.q.CreateMCPConnection(ctx, sqlc.CreateMCPConnectionParams{
		PersonID: admin.ID, Kind: "oauth", Name: "Antigo", ClientID: pgtype.Text{String: cl.ID, Valid: true}, TokenHash: hashToken("ziv_access_antigo"),
		AccessExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		RefreshHash:      pgtype.Text{String: hashToken(refresh), Valid: true},
		RefreshExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if st, _ := postToken(t, c, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {cl.ID}}); st != http.StatusBadRequest {
		t.Fatalf("refresh old client = %d", st)
	}
}

// A "código" connection lasts 90 days, then stops working.
func TestMCPCodeExpiresIn90Days(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	var out mcpCodeOut
	if code := c.do("POST", "/api/mcp/codes", map[string]string{"name": "Notebook"}, &out); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	ctx := context.Background()
	var exp time.Time
	if err := testPool.QueryRow(ctx, `SELECT access_expires_at FROM mcp_connections WHERE id=$1`, out.ID).Scan(&exp); err != nil {
		t.Fatal(err)
	}
	if d := time.Until(exp); d < 89*24*time.Hour || d > 91*24*time.Hour {
		t.Fatalf("expires in %v", d)
	}
	var list []map[string]any
	c.do("GET", "/api/mcp/connections", nil, &list)
	if len(list) != 1 || list[0]["expires_at"] == nil {
		t.Fatalf("list lacks expires_at: %v", list)
	}
	if _, err := testPool.Exec(ctx, `UPDATE mcp_connections SET access_expires_at=now()-interval '1 minute' WHERE id=$1`, out.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.mcpConnection(ctx, out.Code); !errors.Is(err, errMCPUnauthorized) {
		t.Fatalf("expired code err = %v", err)
	}
}

// Codes made before this version get 90 days from the upgrade, not zero.
func TestMigrationGivesOldCodes90Days(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	ctx := context.Background()
	conn, err := h.q.CreateMCPConnection(ctx, sqlc.CreateMCPConnectionParams{PersonID: admin.ID, Kind: "code", Name: "Antigo", TokenHash: hashToken("ziv_antigo")})
	if err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("../database/migrations/034_mcp_code_expiry.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	var exp time.Time
	if err := testPool.QueryRow(ctx, `SELECT access_expires_at FROM mcp_connections WHERE id=$1`, conn.ID).Scan(&exp); err != nil {
		t.Fatal(err)
	}
	if d := time.Until(exp); d < 89*24*time.Hour || d > 91*24*time.Hour {
		t.Fatalf("old code expires in %v", d)
	}
}
