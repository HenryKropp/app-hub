package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// sessionHub fakes the Hub calls a Session makes: pairing and hello.
type sessionHub struct {
	mu          sync.Mutex
	pairStarts  int
	helloCodes  []int // answered in order; 204 once exhausted
	approve     bool
	nextFailing int // GET /api/agent/next answers 503 this many times
}

func (h *sessionHub) server(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/agent/pair/start", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.pairStarts++
		h.mu.Unlock()
		_ = json.NewEncoder(w).Encode(agentproto.PairStartResponse{Code: "ABC", Secret: "s", ApproveURL: "http://ziv/aprovar"})
	})
	mux.HandleFunc("POST /api/agent/pair/poll", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.approve {
			_ = json.NewEncoder(w).Encode(agentproto.PairPollResponse{Status: "approved", Token: "novo"})
			return
		}
		_ = json.NewEncoder(w).Encode(agentproto.PairPollResponse{Status: "pending"})
	})
	mux.HandleFunc("POST /api/agent/hello", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		code := http.StatusNoContent
		if len(h.helloCodes) > 0 {
			code, h.helloCodes = h.helloCodes[0], h.helloCodes[1:]
		}
		h.mu.Unlock()
		w.WriteHeader(code)
	})
	mux.HandleFunc("GET /api/agent/next", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		failing := h.nextFailing > 0
		if failing {
			h.nextFailing--
		}
		h.mu.Unlock()
		if failing {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// states records every state a Status reports.
type states struct {
	mu   sync.Mutex
	seen []State
}

func (s *states) add(st State) { s.mu.Lock(); s.seen = append(s.seen, st); s.mu.Unlock() }
func (s *states) list() []State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]State(nil), s.seen...)
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type sessionRig struct {
	hub       *sessionHub
	sess      *Session
	seen      *states
	reconnect chan struct{}
	mu        sync.Mutex
	opened    []string
}

func (r *sessionRig) openedURLs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.opened...)
}

func newSessionRig(t *testing.T, hub *sessionHub, run func(ctx context.Context) error) *sessionRig {
	srv := hub.server(t)
	r := &sessionRig{hub: hub, seen: &states{}, reconnect: make(chan struct{}, 4)}
	status := &Status{}
	status.Subscribe(r.seen.add)
	if run == nil {
		run = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	}
	r.sess = &Session{
		Client:    &Client{BaseURL: srv.URL, HTTP: srv.Client()},
		Name:      "PC-1",
		StateDir:  t.TempDir(),
		Open:      func(u string) error { r.mu.Lock(); r.opened = append(r.opened, u); r.mu.Unlock(); return nil },
		Status:    status,
		Reconnect: r.reconnect,
		Run:       run,
		Retry:     10 * time.Millisecond,
	}
	return r
}

func (r *sessionRig) start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = r.sess.Start(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
}

func TestSessionStartsDisconnectedWhenMarked(t *testing.T) {
	r := newSessionRig(t, &sessionHub{}, nil)
	if err := MarkDisconnected(r.sess.StateDir); err != nil {
		t.Fatal(err)
	}
	r.start(t)
	waitUntil(t, "disconnected", func() bool { return r.sess.Status.Get() == StateDisconnected })
	time.Sleep(300 * time.Millisecond)
	r.hub.mu.Lock()
	starts := r.hub.pairStarts
	r.hub.mu.Unlock()
	if starts != 0 || len(r.openedURLs()) != 0 {
		t.Fatalf("paired on its own: starts=%d opened=%v", starts, r.openedURLs())
	}
	r.reconnect <- struct{}{}
	waitUntil(t, "browser opened", func() bool { return len(r.openedURLs()) == 1 })
	if got := r.openedURLs()[0]; got != "http://ziv/aprovar" {
		t.Fatalf("opened %q", got)
	}
	r.hub.mu.Lock()
	r.hub.approve = true
	r.hub.mu.Unlock()
	waitUntil(t, "connected", func() bool { return r.sess.Status.Get() == StateConnected })
	if IsMarkedDisconnected(r.sess.StateDir) {
		t.Fatal("mark still there after pairing")
	}
	if tok := LoadToken(r.sess.StateDir); tok != "novo" {
		t.Fatalf("token %q", tok)
	}
}

func TestSessionOfflineThenBack(t *testing.T) {
	r := newSessionRig(t, &sessionHub{helloCodes: []int{503, 503}}, nil)
	if err := SaveToken(r.sess.StateDir, "tok"); err != nil {
		t.Fatal(err)
	}
	r.start(t)
	waitUntil(t, "connected", func() bool { return r.sess.Status.Get() == StateConnected })
	seen := r.seen.list()
	offline := -1
	for i, s := range seen {
		if s == StateOffline && offline < 0 {
			offline = i
		}
	}
	if offline < 0 || seen[len(seen)-1] != StateConnected {
		t.Fatalf("states %v", seen)
	}
}

func TestSessionRevokedGoesDisconnected(t *testing.T) {
	r := newSessionRig(t, &sessionHub{}, func(ctx context.Context) error { return ErrUnauthorized })
	if err := SaveToken(r.sess.StateDir, "tok"); err != nil {
		t.Fatal(err)
	}
	r.start(t)
	waitUntil(t, "disconnected", func() bool { return r.sess.Status.Get() == StateDisconnected })
	if !IsMarkedDisconnected(r.sess.StateDir) {
		t.Fatal("no disconnected mark")
	}
	if tok := LoadToken(r.sess.StateDir); tok != "" {
		t.Fatalf("token kept: %q", tok)
	}
	if len(r.openedURLs()) != 0 {
		t.Fatal("opened the browser on its own")
	}
}

func TestSessionReconnectIgnoredWhilePairing(t *testing.T) {
	r := newSessionRig(t, &sessionHub{}, nil)
	r.start(t)
	waitUntil(t, "first pairing", func() bool { return len(r.openedURLs()) == 1 })
	r.reconnect <- struct{}{}
	r.reconnect <- struct{}{}
	time.Sleep(300 * time.Millisecond)
	r.hub.mu.Lock()
	defer r.hub.mu.Unlock()
	if r.hub.pairStarts != 1 {
		t.Fatalf("pair starts %d", r.hub.pairStarts)
	}
}

func TestLoopReportsReachable(t *testing.T) {
	hub := &sessionHub{nextFailing: 1}
	srv := hub.server(t)
	var mu sync.Mutex
	var got []bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &Agent{
		Client: &Client{BaseURL: srv.URL, Token: "tok", HTTP: srv.Client()},
		OnReachable: func(ok bool) {
			mu.Lock()
			got = append(got, ok)
			mu.Unlock()
			if ok {
				cancel()
			}
		},
	}
	_ = a.Loop(ctx)
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0] != false || got[1] != true {
		t.Fatalf("reachable reports %v", got)
	}
}

func TestClientGoodbye(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/agent/goodbye" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		if gotAuth != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Token: "tok", HTTP: srv.Client()}
	if err := c.Goodbye(context.Background()); err != nil {
		t.Fatalf("goodbye: %v", err)
	}
	c.Token = "outro"
	if err := c.Goodbye(context.Background()); err != ErrUnauthorized {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}
