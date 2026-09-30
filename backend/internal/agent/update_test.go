package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

func digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

// binaryHub serves body at /api/agent/binary.
func binaryHub(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/binary" || r.Header.Get("Authorization") != "Bearer t" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}
}

type startRec struct {
	mu    sync.Mutex
	calls [][]string
}

func (s *startRec) start(exe string, args ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, append([]string{exe}, args...))
	return nil
}

func (s *startRec) n() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.calls) }

func newUpdater(t *testing.T, c *Client) (*Updater, *startRec) {
	t.Helper()
	state := t.TempDir()
	cur := filepath.Join(state, "AppHubAgente.exe")
	_ = os.WriteFile(cur, []byte("MZ-old"), 0o755)
	rec := &startRec{}
	return &Updater{Client: c, Runner: &Runner{}, StateDir: state, CurrentExe: cur, Start: rec.start}, rec
}

func TestUpdaterAppliesVerifiedDownload(t *testing.T) {
	u, rec := newUpdater(t, binaryHub(t, "MZ-new"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	if err := u.Apply(ctx, agentproto.UpdatePayload{Version: "0.2.1", SHA256: digest("MZ-new"), Size: 6}); err != nil {
		t.Fatal(err)
	}
	dl := filepath.Join(u.StateDir, "update", "AppHubAgente.exe")
	if b, _ := os.ReadFile(dl); string(b) != "MZ-new" {
		t.Fatalf("download %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(u.StateDir, "AppHubAgente.anterior.exe")); string(b) != "MZ-old" {
		t.Fatalf("backup %q", b)
	}
	if rec.n() != 1 || rec.calls[0][0] != dl || rec.calls[0][1] != "--update" {
		t.Fatalf("start %v", rec.calls)
	}
	// Handed over: no app may open until this agent is closed.
	if u.Runner.mu.TryLock() {
		t.Fatal("runner released after a successful hand-over")
	}
}

// Review Focus 5: a cut-short or different download never starts.
func TestUpdaterRejectsBadDownload(t *testing.T) {
	for name, p := range map[string]agentproto.UpdatePayload{
		"short":    {SHA256: digest("MZ-new-and-more"), Size: 15},
		"tampered": {SHA256: digest("MZ-evil"), Size: 6},
		"oversize": {SHA256: digest("MZ"), Size: 2},
	} {
		t.Run(name, func(t *testing.T) {
			u, rec := newUpdater(t, binaryHub(t, "MZ-new"))
			if err := u.Apply(context.Background(), p); !errors.Is(err, ErrBadDownload) {
				t.Fatalf("got %v", err)
			}
			if _, err := os.Stat(filepath.Join(u.StateDir, "update", "AppHubAgente.exe")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("download left behind: %v", err)
			}
			if rec.n() != 0 {
				t.Fatal("started a bad download")
			}
		})
	}
}

func TestUpdaterWaitsForOpenApp(t *testing.T) {
	u, rec := newUpdater(t, binaryHub(t, "MZ-new"))
	u.Runner.mu.Lock() // an app is open
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { time.Sleep(400 * time.Millisecond); cancel() }()
	go func() { done <- u.Apply(ctx, agentproto.UpdatePayload{SHA256: digest("MZ-new"), Size: 6}) }()
	time.Sleep(100 * time.Millisecond)
	if rec.n() != 0 {
		t.Fatal("updated while an app was open")
	}
	u.Runner.mu.Unlock()
	if err := <-done; err != nil || rec.n() != 1 {
		t.Fatalf("err %v starts %d", err, rec.n())
	}
}

// Review Focus 4: Rodar during an update says so.
func TestRunDuringUpdate(t *testing.T) {
	r := &Runner{StateDir: t.TempDir()}
	release := r.Hold()
	defer release()
	res := r.Run(context.Background(), agentproto.RunPayload{}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "se atualizando") {
		t.Fatalf("%+v", res)
	}
}

func TestAgentReportsFailedUpdate(t *testing.T) {
	hub := &fakeHub{results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	hub.jobs = []agentproto.Job{{ID: 9, Kind: agentproto.KindUpdate, Payload: payload(agentproto.UpdatePayload{SHA256: "x", Size: 1})}}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}
	u, _ := newUpdater(t, client)
	a := &Agent{Client: client, Runner: u.Runner, StateDir: u.StateDir, Updater: u}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Loop(ctx)
	waitFor(t, func() bool { hub.mu.Lock(); defer hub.mu.Unlock(); return len(hub.results[9]) == 1 })
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if r := hub.results[9][0]; r.Event != agentproto.EventFailed || r.Error != "O arquivo baixado veio diferente do esperado." {
		t.Fatalf("%+v", r)
	}
}
