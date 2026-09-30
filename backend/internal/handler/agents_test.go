package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// pairComputer runs the whole pairing flow for the logged-in client c.
func pairComputer(t *testing.T, c *client, name string) string {
	t.Helper()
	var start agentproto.PairStartResponse
	if code := c.agent("POST", "/api/agent/pair/start", "", agentproto.PairStartRequest{ComputerName: name}, &start); code != http.StatusOK {
		t.Fatalf("pair start %d", code)
	}
	if code := c.do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil); code != http.StatusNoContent {
		t.Fatalf("approve %d", code)
	}
	var poll agentproto.PairPollResponse
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, &poll); code != http.StatusOK || poll.Token == "" {
		t.Fatalf("poll %d %+v", code, poll)
	}
	return poll.Token
}

func TestPairingFlow(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)

	var start agentproto.PairStartResponse
	c.agent("POST", "/api/agent/pair/start", "", agentproto.PairStartRequest{ComputerName: "NOTE-HENRY"}, &start)
	if !strings.HasPrefix(start.ApproveURL, "http://hub.test/conectar?codigo="+start.Code) {
		t.Fatalf("approve url %q", start.ApproveURL)
	}

	// Before approval the agent keeps waiting.
	var poll agentproto.PairPollResponse
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, &poll); code != http.StatusOK || poll.Status != "pending" {
		t.Fatalf("pending poll %d %+v", code, poll)
	}

	// The page shows the computer name; approving needs a login.
	var p struct {
		ComputerName string `json:"computer_name"`
		Expired      bool   `json:"expired"`
	}
	c.do("GET", "/api/pairings/"+start.Code, nil, &p)
	if p.ComputerName != "NOTE-HENRY" || p.Expired {
		t.Fatalf("pairing %+v", p)
	}
	if code := c.fresh().do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous approve %d", code)
	}
	c.do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil)

	c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, &poll)
	if poll.Status != "approved" || poll.Token == "" {
		t.Fatalf("approved poll %+v", poll)
	}
	// Single use.
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, nil); code != http.StatusGone {
		t.Fatalf("second poll %d", code)
	}

	var me struct {
		Computers []computerJSON `json:"computers"`
		Expected  string         `json:"expected_version"`
	}
	c.do("GET", "/api/computers/me", nil, &me)
	if len(me.Computers) != 1 || !me.Computers[0].Connected || me.Computers[0].Name != "NOTE-HENRY" || me.Expected != agentproto.Version {
		t.Fatalf("computers %+v", me)
	}
}

func TestPairingExpired(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	var start agentproto.PairStartResponse
	c.agent("POST", "/api/agent/pair/start", "", agentproto.PairStartRequest{ComputerName: "PC"}, &start)
	if _, err := testPool.Exec(context.Background(), "UPDATE pairings SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if code := c.do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil); code != http.StatusGone {
		t.Fatalf("approve expired %d", code)
	}
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, nil); code != http.StatusGone {
		t.Fatalf("poll expired %d", code)
	}
}

func TestComputerAuthAndRevoke(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	hello := agentproto.Hello{Name: "PC", Version: "0.0.9"}
	if code := c.agent("POST", "/api/agent/hello", "errado", hello, nil); code != http.StatusUnauthorized {
		t.Fatalf("bad token %d", code)
	}
	if code := c.agent("POST", "/api/agent/hello", token, hello, nil); code != http.StatusNoContent {
		t.Fatalf("hello %d", code)
	}
	var me struct {
		Computers []computerJSON `json:"computers"`
	}
	c.do("GET", "/api/computers/me", nil, &me)
	if !me.Computers[0].Outdated {
		t.Fatalf("want outdated: %+v", me.Computers[0])
	}

	// Someone else cannot disconnect it.
	other := c.fresh()
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	loginAs(t, other, "ana@alis.com")
	if code := other.do("DELETE", "/api/computers/1", nil, nil); code != http.StatusNotFound {
		t.Fatalf("other revoke %d", code)
	}
	if code := c.do("DELETE", "/api/computers/1", nil, nil); code != http.StatusNoContent {
		t.Fatalf("revoke %d", code)
	}
	if code := c.agent("POST", "/api/agent/hello", token, hello, nil); code != http.StatusUnauthorized {
		t.Fatalf("revoked token %d", code)
	}
}

func createPersonAs(t *testing.T, admin *client, name, email, role string) {
	t.Helper()
	body := map[string]string{"name": name, "email": email, "password": "senha-forte-1", "role": role}
	if code := admin.do("POST", "/api/people", body, nil); code != http.StatusCreated {
		t.Fatalf("create person %d", code)
	}
}

func loginAs(t *testing.T, c *client, email string) {
	t.Helper()
	if code := c.do("POST", "/api/auth/login", map[string]string{"email": email, "password": "senha-forte-1"}, nil); code != http.StatusOK {
		t.Fatalf("login %d", code)
	}
}
