package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// withBinary points the Hub at a fake agent binary and returns its digest.
func withBinary(t *testing.T, h *Handler, body string) (string, int64) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "AppHubAgente.exe")
	if err := os.WriteFile(bin, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h.cfg.AgentBinaryPath = bin
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:]), int64(len(body))
}

func nextJob(t *testing.T, c *client, token string) (agentproto.Job, int) {
	t.Helper()
	var j agentproto.Job
	code := c.agent("GET", "/api/agent/next", token, nil, &j)
	return j, code
}

func updateJobs(t *testing.T) (total int) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM agent_jobs WHERE kind = 'update'").Scan(&total); err != nil {
		t.Fatal(err)
	}
	return total
}

func jobStatus(t *testing.T, id int64) (status string) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(), "SELECT status FROM agent_jobs WHERE id = $1", id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestUpdateQueuedOnHello(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	sum, size := withBinary(t, h, "MZ-new")

	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	if n := updateJobs(t); n != 1 {
		t.Fatalf("want 1 update job, got %d", n)
	}
	j, code := nextJob(t, c, token)
	if code != http.StatusOK || j.Kind != agentproto.KindUpdate {
		t.Fatalf("next %d %+v", code, j)
	}
	var p agentproto.UpdatePayload
	_ = json.Unmarshal(j.Payload, &p)
	if p.Version != agentproto.Version || p.SHA256 != sum || p.Size != size {
		t.Fatalf("payload %+v", p)
	}
}

func TestNoUpdateWhenNotPossible(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	// No binary on this Hub.
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	// Old agent that cannot update.
	withBinary(t, h, "MZ")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.1"}, nil)
	// Already on the Hub's version.
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true}, nil)
	if n := updateJobs(t); n != 0 {
		t.Fatalf("want no update job, got %d", n)
	}
	if _, code := nextJob(t, c, token); code != http.StatusNoContent {
		t.Fatalf("next %d", code)
	}
}

// Review Focus 1: the Hub restarted with a new version; the agent keeps
// long-polling without a new hello.
func TestUpdateQueuedOnNext(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true}, nil)
	if _, err := testPool.Exec(context.Background(), "UPDATE computers SET agent_version = '0.1.9'"); err != nil {
		t.Fatal(err)
	}
	if j, code := nextJob(t, c, token); code != http.StatusOK || j.Kind != agentproto.KindUpdate {
		t.Fatalf("next %d %+v", code, j)
	}
}

func TestHelloClosesUpdate(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	old := agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}

	c.agent("POST", "/api/agent/hello", token, old, nil)
	j, _ := nextJob(t, c, token)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true}, nil)
	if s := jobStatus(t, j.ID); s != "done" {
		t.Fatalf("worked update: %s", s)
	}
}

func TestUpdateGivesUpAfterThreeFailures(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	old := agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}

	c.agent("POST", "/api/agent/hello", token, old, nil)
	for i := range 3 {
		j, code := nextJob(t, c, token)
		if code != http.StatusOK || j.Kind != agentproto.KindUpdate {
			t.Fatalf("try %d: next %d %+v", i, code, j)
		}
		c.agent("POST", "/api/agent/hello", token, old, nil) // rolled back
		if s := jobStatus(t, j.ID); s != "failed" {
			t.Fatalf("try %d: %s", i, s)
		}
	}
	if _, code := nextJob(t, c, token); code != http.StatusNoContent {
		t.Fatalf("after 3 failures next %d", code)
	}
}

func TestExpiredUpdateDoesNotCount(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	if _, err := testPool.Exec(context.Background(), "UPDATE agent_jobs SET created_at = now() - interval '3 minutes'"); err != nil {
		t.Fatal(err)
	}
	// next expires the old one and queues a fresh one.
	j, code := nextJob(t, c, token)
	if code != http.StatusOK || j.Kind != agentproto.KindUpdate || j.ID != 2 {
		t.Fatalf("next %d %+v", code, j)
	}
}

func TestAgentBinaryNeedsComputerToken(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ-new")

	if code := c.agent("GET", "/api/agent/binary", "", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("no token %d", code)
	}
	req, _ := http.NewRequest("GET", c.srv.URL+"/api/agent/binary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "MZ-new" {
		t.Fatalf("%d %q", resp.StatusCode, b)
	}
}
