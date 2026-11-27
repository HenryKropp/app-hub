package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

func personID(t *testing.T, email string) int64 {
	t.Helper()
	var id int64
	if err := testPool.QueryRow(context.Background(), `SELECT id FROM people WHERE email=$1`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// Disabling a person ends everything at once: the browser session, the
// password, Claude's connection and the paired computer. Enabling lets the
// person sign in again.
func TestDisablePersonCutsEverything(t *testing.T) {
	admin, h := newPoolClient(t)
	setupAdmin(t, admin)
	createPersonAs(t, admin, "Ana", "ana@exemplo.com", "usuario")
	ana := admin.fresh()
	loginAs(t, ana, "ana@exemplo.com")
	token := pairComputer(t, ana, "PC-ANA")
	var code mcpCodeOut
	if st := ana.do("POST", "/api/mcp/codes", map[string]string{"name": "Note"}, &code); st != http.StatusCreated {
		t.Fatalf("code %d", st)
	}
	id := personID(t, "ana@exemplo.com")

	if st := admin.do("POST", fmt.Sprintf("/api/people/%d/disable", id), nil, nil); st != http.StatusOK {
		t.Fatalf("disable = %d", st)
	}
	if st := ana.do("GET", "/api/computers/me", nil, nil); st != http.StatusUnauthorized {
		t.Fatalf("session after disable = %d", st)
	}
	if st := ana.fresh().do("POST", "/api/auth/login", map[string]string{"email": "ana@exemplo.com", "password": "senha-forte-1"}, nil); st != http.StatusUnauthorized {
		t.Fatalf("login after disable = %d", st)
	}
	if _, _, err := h.mcpConnection(context.Background(), code.Code); !errors.Is(err, errMCPUnauthorized) {
		t.Fatalf("mcp after disable: %v", err)
	}
	hello := agentproto.Hello{Name: "PC-ANA", Version: agentproto.Version}
	if st := ana.agent("POST", "/api/agent/hello", token, hello, nil); st != http.StatusUnauthorized {
		t.Fatalf("agent after disable = %d", st)
	}
	var people []map[string]any
	admin.do("GET", "/api/people", nil, &people)
	for _, p := range people {
		if p["email"] == "ana@exemplo.com" {
			t.Fatal("disabled person still offered in /api/people")
		}
	}
	people = nil
	admin.do("GET", "/api/people?todas=1", nil, &people)
	found := false
	for _, p := range people {
		if p["email"] == "ana@exemplo.com" {
			found = p["disabled"] == true
		}
	}
	if !found {
		t.Fatalf("admin list must show Ana as disabled: %v", people)
	}

	if st := admin.do("POST", fmt.Sprintf("/api/people/%d/enable", id), nil, nil); st != http.StatusOK {
		t.Fatalf("enable = %d", st)
	}
	loginAs(t, ana.fresh(), "ana@exemplo.com")
}

func TestDisablePersonRefusals(t *testing.T) {
	admin, _ := newPoolClient(t)
	me := setupAdmin(t, admin)
	if st := admin.do("POST", fmt.Sprintf("/api/people/%d/disable", me.ID), nil, nil); st != http.StatusBadRequest {
		t.Fatalf("disable self = %d", st)
	}
	createPersonAs(t, admin, "Bia", "bia@exemplo.com", "usuario")
	bia := admin.fresh()
	loginAs(t, bia, "bia@exemplo.com")
	if st := bia.do("POST", fmt.Sprintf("/api/people/%d/disable", me.ID), nil, nil); st != http.StatusForbidden {
		t.Fatalf("disable without permission = %d", st)
	}
	if st := admin.do("POST", "/api/people/999999/disable", nil, nil); st != http.StatusNotFound {
		t.Fatalf("disable unknown = %d", st)
	}
}

// A session falls after 7 days without use; using it moves the clock.
func TestSessionFallsAfterSevenIdleDays(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `UPDATE sessions SET last_used_at = now() - interval '6 days'`); err != nil {
		t.Fatal(err)
	}
	if st := c.do("GET", "/api/computers/me", nil, nil); st != http.StatusOK {
		t.Fatalf("6 idle days = %d", st)
	}
	var last time.Time
	if err := testPool.QueryRow(ctx, `SELECT max(last_used_at) FROM sessions`).Scan(&last); err != nil || time.Since(last) > time.Minute {
		t.Fatalf("session not touched: %v %v", last, err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE sessions SET last_used_at = now() - interval '8 days'`); err != nil {
		t.Fatal(err)
	}
	if st := c.do("GET", "/api/computers/me", nil, nil); st != http.StatusUnauthorized {
		t.Fatalf("8 idle days = %d", st)
	}
}

// A computer silent for 90 days loses its access; one seen recently keeps it.
func TestIdleComputersAreRevoked(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	old := pairComputer(t, c, "VELHO")
	recent := pairComputer(t, c, "NOVO")
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `UPDATE computers SET last_seen_at = now() - interval '91 days' WHERE name='VELHO'`); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE computers SET last_seen_at = now() - interval '89 days' WHERE name='NOVO'`); err != nil {
		t.Fatal(err)
	}
	h.revokeIdleComputers(ctx, time.Now())
	hello := agentproto.Hello{Name: "x", Version: agentproto.Version}
	if st := c.agent("POST", "/api/agent/hello", old, hello, nil); st != http.StatusUnauthorized {
		t.Fatalf("idle computer = %d", st)
	}
	if st := c.agent("POST", "/api/agent/hello", recent, hello, nil); st != http.StatusNoContent {
		t.Fatalf("recent computer = %d", st)
	}
}
