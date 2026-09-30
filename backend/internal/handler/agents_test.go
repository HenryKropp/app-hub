package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestNextLongPollAndChooseFolderResult(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	// Nothing queued: 204 after the wait.
	if code := c.agent("GET", "/api/agent/next", token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("empty next %d", code)
	}

	ctx := context.Background()
	job, err := h.enqueue(ctx, 1, nil, agentproto.KindChooseFolder, agentproto.ChooseFolderPayload{Initial: `C:\apps`})
	if err != nil {
		t.Fatal(err)
	}
	var got agentproto.Job
	if code := c.agent("GET", "/api/agent/next", token, nil, &got); code != http.StatusOK || got.ID != job.ID || got.Kind != agentproto.KindChooseFolder {
		t.Fatalf("next %d %+v", code, got)
	}
	// Claimed once only.
	if code := c.agent("GET", "/api/agent/next", token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("second next %d", code)
	}

	res := agentproto.Result{Event: agentproto.EventDone, Folder: `D:\Saídas`}
	if code := c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, res, nil); code != http.StatusNoContent {
		t.Fatalf("result %d", code)
	}
	done, ok := h.waitJob(ctx, job.ID, time.Second)
	if !ok || done.Status != "done" || !strings.Contains(string(done.Result), `D:\\Saídas`) {
		t.Fatalf("job %+v", done)
	}
}

func TestResultForOtherComputersJobIsRejected(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC1")
	token2 := pairComputer(t, c, "PC2")
	job, err := h.enqueue(context.Background(), 1, nil, agentproto.KindChooseFolder, agentproto.ChooseFolderPayload{})
	if err != nil {
		t.Fatal(err)
	}
	res := agentproto.Result{Event: agentproto.EventDone, Canceled: true}
	if code := c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token2, res, nil); code != http.StatusNotFound {
		t.Fatalf("foreign result %d", code)
	}
}

func TestAgentDownload(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	if code := c.do("GET", "/api/agent/download", nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing binary %d", code)
	}
	bin := filepath.Join(t.TempDir(), "AppHubAgente.exe")
	_ = os.WriteFile(bin, []byte("MZ"), 0o644)
	h.cfg.AgentBinaryPath = bin
	resp, err := c.c.Get(c.srv.URL + "/api/agent/download")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Disposition"), "AppHubAgente.exe") {
		t.Fatalf("%d %q", resp.StatusCode, resp.Header.Get("Content-Disposition"))
	}
	if code := c.fresh().do("GET", "/api/agent/download", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous %d", code)
	}
}
