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
	s        *Swap
	starts   int
	started  []string
	stopped  int
	startErr func(exe string) error
	order    []string
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
		QuitRunning: func() { e.order = append(e.order, "quit") },
		Start: func(exe string) (func(), error) {
			e.started = append(e.started, exe)
			e.starts++
			if e.startErr != nil {
				if err := e.startErr(exe); err != nil {
					return nil, err
				}
			}
			return func() { e.stopped++ }, nil
		},
		Connected: func(context.Context) bool {
			polls++
			return connectAfter >= 0 && polls > connectAfter
		},
		Wait: 200 * time.Millisecond, Poll: 10 * time.Millisecond, RetryWait: time.Millisecond, Tries: 3,
	}
	return e
}

func content(t *testing.T, p string) string { b, _ := os.ReadFile(p); return string(b) }

func TestSwapInstallsNewVersion(t *testing.T) {
	e := newSwap(t, 2)
	if err := e.s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if content(t, e.s.Target) != "MZ-new" || !sameSeq(e.started, e.s.Target) || e.stopped != 0 {
		t.Fatalf("target %q starts %d stopped %d", content(t, e.s.Target), e.starts, e.stopped)
	}
}

func TestSwapRollsBack(t *testing.T) {
	e := newSwap(t, -1) // never connects
	if err := e.s.Run(context.Background()); !errors.Is(err, ErrRolledBack) {
		t.Fatalf("got %v", err)
	}
	if content(t, e.s.Target) != "MZ-old" || !sameSeq(e.started, e.s.Target, e.s.Target) || e.stopped != 1 {
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
	if !sameSeq(e.started, e.s.Target) || content(t, e.s.Target) != "MZ-old" {
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

func sameSeq(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSwapQuitsBeforeFirstCopy(t *testing.T) {
	e := newSwap(t, 0)
	e.s.Copy = func(src, dst string) error {
		e.order = append(e.order, "copy")
		return replaceFile(src, dst)
	}
	if err := e.s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(e.order) < 2 || e.order[0] != "quit" || e.order[1] != "copy" {
		t.Fatalf("order %v", e.order)
	}
}

func failRestoreCopy(e *swapEnv) {
	e.s.Copy = func(src, dst string) error {
		if src == e.s.Backup {
			return fmt.Errorf("file in use")
		}
		return replaceFile(src, dst)
	}
}

func TestSwapRestoreCopyFailsStartsBackup(t *testing.T) {
	e := newSwap(t, -1)
	failRestoreCopy(e)
	err := e.s.Run(context.Background())
	if !errors.Is(err, ErrRolledBack) {
		t.Fatalf("got %v", err)
	}
	if !sameSeq(e.started, e.s.Target, e.s.Backup) {
		t.Fatalf("started %v", e.started)
	}
}

func TestSwapRestoreCopyAndBackupFailStartsTarget(t *testing.T) {
	e := newSwap(t, -1)
	failRestoreCopy(e)
	e.startErr = func(exe string) error {
		if exe == e.s.Backup {
			return fmt.Errorf("cannot start")
		}
		return nil
	}
	err := e.s.Run(context.Background())
	if !errors.Is(err, ErrRolledBack) {
		t.Fatalf("got %v", err)
	}
	if !sameSeq(e.started, e.s.Target, e.s.Backup, e.s.Target) {
		t.Fatalf("started %v", e.started)
	}
}

func TestSwapRestoredTargetFailsStartsBackup(t *testing.T) {
	e := newSwap(t, -1)
	n := 0
	e.startErr = func(exe string) error {
		n++
		if n == 2 { // restored target
			return fmt.Errorf("cannot start")
		}
		return nil
	}
	if err := e.s.Run(context.Background()); !errors.Is(err, ErrRolledBack) {
		t.Fatalf("got %v", err)
	}
	if !sameSeq(e.started, e.s.Target, e.s.Target, e.s.Backup) {
		t.Fatalf("started %v", e.started)
	}
}

func TestSwapInstallCopyFailsAndTargetFailsStartsBackup(t *testing.T) {
	e := newSwap(t, 0)
	e.s.Copy = func(string, string) error { return fmt.Errorf("file in use") }
	e.startErr = func(exe string) error {
		if exe == e.s.Target {
			return fmt.Errorf("cannot start")
		}
		return nil
	}
	err := e.s.Run(context.Background())
	if !errors.Is(err, ErrRolledBack) {
		t.Fatalf("got %v", err)
	}
	if !sameSeq(e.started, e.s.Target, e.s.Backup) {
		t.Fatalf("started %v", e.started)
	}
}

func TestStatusConnectedRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"version":"1","connected":true}`)
	}))
	defer srv.Close()
	if StatusConnected(context.Background(), srv.URL, "1") {
		t.Fatal("500 counted as connected")
	}
}
