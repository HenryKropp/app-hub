package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakePodman writes a stand-in podman binary: `container inspect` answers
// with an empty label (an unlabelled container, like app-hub-db) and every
// call is appended to the returned log file.
func fakePodman(t *testing.T) (bin, logFile string) {
	t.Helper()
	dir := t.TempDir()
	logFile = filepath.Join(dir, "calls.log")
	bin = filepath.Join(dir, "podman")
	script := "#!/bin/sh\necho \"$@\" >> " + logFile + "\nexit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, logFile
}

func calls(t *testing.T, logFile string) string {
	b, _ := os.ReadFile(logFile)
	return string(b)
}

func TestExecAllowlistSkipsGuardOnlyForExec(t *testing.T) {
	ctx := context.Background()
	bin, logFile := fakePodman(t)
	p := NewPodman(bin, WithExecAllowed("app-hub-db"))

	if err := p.Exec(ctx, "app-hub-db", nil, &strings.Builder{}, "pg_dump", "x"); err != nil {
		t.Fatalf("allowlisted exec refused: %v", err)
	}
	got := calls(t, logFile)
	if !strings.Contains(got, "exec -i app-hub-db pg_dump x") || strings.Contains(got, "inspect") {
		t.Fatalf("calls: %q", got)
	}

	// Not on the list and unlabelled: still refused, and nothing executed.
	os.Remove(logFile)
	if err := p.Exec(ctx, "sistema-alis-db", nil, &strings.Builder{}, "ls"); !errors.Is(err, errNotHub) {
		t.Fatalf("non-allowlisted exec: %v", err)
	}
	if strings.Contains(calls(t, logFile), "exec -i") {
		t.Fatalf("exec ran: %q", calls(t, logFile))
	}
	// Exact match only.
	if err := p.Exec(ctx, "app-hub-db2", nil, &strings.Builder{}, "ls"); !errors.Is(err, errNotHub) {
		t.Fatalf("prefix name: %v", err)
	}
	// Default Podman has no allowlist.
	if err := NewPodman(bin).Exec(ctx, "app-hub-db", nil, &strings.Builder{}, "ls"); !errors.Is(err, errNotHub) {
		t.Fatalf("no allowlist: %v", err)
	}
}

func TestExecAllowlistDoesNotWeakenDestructiveCalls(t *testing.T) {
	ctx := context.Background()
	bin, logFile := fakePodman(t)
	p := NewPodman(bin, WithExecAllowed("app-hub-db"))

	if err := p.Stop(ctx, "app-hub-db"); !errors.Is(err, errNotHub) {
		t.Fatalf("stop: %v", err)
	}
	if err := p.Remove(ctx, "app-hub-db"); !errors.Is(err, errNotHub) {
		t.Fatalf("remove: %v", err)
	}
	if _, err := p.Logs(ctx, "app-hub-db", 10); !errors.Is(err, errNotHub) {
		t.Fatalf("logs: %v", err)
	}
	// RemoveImage guards by image labels/tags; an image id equal to the
	// allowlisted name has neither, so it is refused as well.
	if err := p.RemoveImage(ctx, "app-hub-db"); err == nil {
		t.Fatal("removeimage not refused")
	}
	got := calls(t, logFile)
	for _, bad := range []string{"stop ", "rm ", "logs ", "rmi "} {
		if strings.Contains(got, bad) {
			t.Fatalf("destructive call ran (%q): %q", bad, got)
		}
	}
}
