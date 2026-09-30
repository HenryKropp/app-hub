package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// fakeHub serves queued jobs and records results and uploads.
type fakeHub struct {
	mu      sync.Mutex
	jobs    []agentproto.Job
	results map[int64][]agentproto.Result
	uploads map[int64]string
	revoked bool
}

func (f *fakeHub) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/agent/next", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.revoked {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if len(f.jobs) == 0 {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		j := f.jobs[0]
		f.jobs = f.jobs[1:]
		_ = json.NewEncoder(w).Encode(j)
	})
	mux.HandleFunc("POST /api/agent/jobs/{id}/result", func(w http.ResponseWriter, r *http.Request) {
		var res agentproto.Result
		_ = json.NewDecoder(r.Body).Decode(&res)
		var id int64
		_, _ = fmtSscan(r.PathValue("id"), &id)
		f.mu.Lock()
		f.results[id] = append(f.results[id], res)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/agent/jobs/{id}/upload", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var id int64
		_, _ = fmtSscan(r.PathValue("id"), &id)
		f.mu.Lock()
		f.uploads[id] = string(b)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func payload(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestAgentLoop(t *testing.T) {
	app := appFolder(t)
	state := t.TempDir()
	hub := &fakeHub{results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	hub.jobs = []agentproto.Job{
		{ID: 1, Kind: agentproto.KindChooseFolder, Payload: payload(agentproto.ChooseFolderPayload{Initial: app})},
		{ID: 2, Kind: agentproto.KindRun, Payload: payload(agentproto.RunPayload{RunID: 3, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app})},
	}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()

	a := &Agent{
		Client:   &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()},
		Runner:   &Runner{Launcher: &fakeLauncher{write: map[string]string{"novo.md": "# oi"}}, StateDir: state},
		StateDir: state,
		ChooseFolder: func(ctx context.Context, initial string) (string, bool, error) {
			return `C:\Relatórios`, true, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go a.Loop(ctx)

	waitFor(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return len(hub.results[2]) == 2
	})
	hub.mu.Lock()
	if r := hub.results[1]; len(r) != 1 || r[0].Folder != `C:\Relatórios` {
		t.Fatalf("folder result %+v", r)
	}
	if r := hub.results[2]; r[0].Event != agentproto.EventStarted || r[1].Event != agentproto.EventFinished || len(r[1].Files) != 1 {
		t.Fatalf("run results %+v", r)
	}
	hub.jobs = append(hub.jobs, agentproto.Job{ID: 4, Kind: agentproto.KindSendFile, Payload: payload(agentproto.SendFilePayload{RunID: 3, Paths: []string{"novo.md"}})},
		agentproto.Job{ID: 5, Kind: agentproto.KindSendFile, Payload: payload(agentproto.SendFilePayload{RunID: 3, Paths: []string{"Ferramenta CVM.exe"}})})
	hub.mu.Unlock()

	waitFor(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return hub.uploads[4] != "" && len(hub.results[5]) == 1
	})
	hub.mu.Lock()
	if hub.uploads[4] != "# oi" || hub.results[5][0].Event != agentproto.EventFailed {
		t.Fatalf("uploads %v results5 %+v", hub.uploads, hub.results[5])
	}
	hub.revoked = true
	hub.mu.Unlock()
	cancel()
}

func TestAgentLoopStopsWhenRevoked(t *testing.T) {
	hub := &fakeHub{revoked: true, results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()
	a := &Agent{Client: &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}, Runner: &Runner{}, StateDir: t.TempDir()}
	if err := a.Loop(context.Background()); err != ErrUnauthorized {
		t.Fatalf("got %v", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func fmtSscan(s string, v *int64) (int, error) { return fmt.Sscan(s, v) }
