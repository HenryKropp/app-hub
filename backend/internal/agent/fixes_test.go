package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// The Hub may be restarted while an app is open; the "finished" report must
// not be lost.
func TestReportRetriesUntilHubAccepts(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	a := &Agent{Client: &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}, RetryBase: 10 * time.Millisecond}
	a.report(context.Background(), 1, agentproto.Result{Event: agentproto.EventFinished})
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestReportGivesUpWhenJobIsUnknown(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	a := &Agent{Client: &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}, RetryBase: 10 * time.Millisecond}
	a.report(context.Background(), 1, agentproto.Result{Event: agentproto.EventFinished})
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

// A half-open connection must not freeze the agent.
func TestClientTimesOutHangingRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)
	c := &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client(), RequestTimeout: 100 * time.Millisecond}
	start := time.Now()
	if _, err := c.Next(context.Background()); err == nil {
		t.Fatal("want timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("request did not time out")
	}
}

// copyFile must never delete or change a file that was already there.
func TestCopyFileKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "src.exe", "novo")
	write(t, dir, "dst.exe", "da pessoa")
	if err := copyFile(filepath.Join(dir, "src.exe"), filepath.Join(dir, "dst.exe")); err == nil {
		t.Fatal("want error for existing file")
	}
	b, err := os.ReadFile(filepath.Join(dir, "dst.exe"))
	if err != nil || string(b) != "da pessoa" {
		t.Fatalf("person's file changed: %q %v", b, err)
	}
}
