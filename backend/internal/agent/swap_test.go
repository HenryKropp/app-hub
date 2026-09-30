package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type swapEnv struct {
	s       *Swap
	starts  int
	stopped int
}

func newSwap(t *testing.T, connectAfter int) *swapEnv {
	t.Helper()
	dir := t.TempDir()
	e := &swapEnv{}
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte(body), 0o755)
		return p
	}
	polls := 0
	e.s = &Swap{
		Self: write("novo.exe", "MZ-new"), Target: write("AppHubAgente.exe", "MZ-old"), Backup: write("AppHubAgente.anterior.exe", "MZ-old"),
		QuitRunning: func() {},
		Start: func(string) (func(), error) {
			e.starts++
			return func() { e.stopped++ }, nil
		},
		Connected: func(context.Context) bool {
			polls++
			return connectAfter >= 0 && polls > connectAfter
		},
		Wait: 200 * time.Millisecond, Poll: 10 * time.Millisecond, RetryWait: time.Millisecond,
	}
	return e
}

func content(t *testing.T, p string) string { b, _ := os.ReadFile(p); return string(b) }

func TestSwapInstallsNewVersion(t *testing.T) {
	e := newSwap(t, 2)
	if err := e.s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if content(t, e.s.Target) != "MZ-new" || e.starts != 1 || e.stopped != 0 {
		t.Fatalf("target %q starts %d stopped %d", content(t, e.s.Target), e.starts, e.stopped)
	}
}

func TestSwapRollsBack(t *testing.T) {
	e := newSwap(t, -1) // never connects
	if err := e.s.Run(context.Background()); !errors.Is(err, ErrRolledBack) {
		t.Fatalf("got %v", err)
	}
	if content(t, e.s.Target) != "MZ-old" || e.starts != 2 || e.stopped != 1 {
		t.Fatalf("target %q starts %d stopped %d", content(t, e.s.Target), e.starts, e.stopped)
	}
}

// Review Focus 2: Windows holds the old exe for a moment after /quit.
func TestSwapRetriesLockedFile(t *testing.T) {
	e := newSwap(t, 0)
	fails := 2
	e.s.Copy = func(src, dst string) error {
		if fails > 0 {
			fails--
			return fmt.Errorf("file in use")
		}
		return replaceFile(src, dst)
	}
	if err := e.s.Run(context.Background()); err != nil || content(t, e.s.Target) != "MZ-new" {
		t.Fatalf("err %v target %q", err, content(t, e.s.Target))
	}
}

func TestSwapCopyNeverWorksRestartsOld(t *testing.T) {
	e := newSwap(t, 0)
	e.s.Copy = func(string, string) error { return fmt.Errorf("file in use") }
	if err := e.s.Run(context.Background()); err == nil {
		t.Fatal("want error")
	}
	if e.starts != 1 || content(t, e.s.Target) != "MZ-old" {
		t.Fatalf("old agent not restarted: starts %d target %q", e.starts, content(t, e.s.Target))
	}
}

// Review Focus 3: the old agent may still answer /status.
func TestStatusConnectedChecksVersion(t *testing.T) {
	version := "0.2.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":%q,"connected":true}`, version)
	}))
	defer srv.Close()
	if StatusConnected(context.Background(), srv.URL, "0.2.1") {
		t.Fatal("old version counted as connected")
	}
	version = "0.2.1"
	if !StatusConnected(context.Background(), srv.URL, "0.2.1") {
		t.Fatal("new version not seen")
	}
	if StatusConnected(context.Background(), "http://127.0.0.1:1", "0.2.1") {
		t.Fatal("nothing listening counted as connected")
	}
}
