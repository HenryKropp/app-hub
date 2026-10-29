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

func TestNetworksOnlyTheHubs(t *testing.T) {
	ctx := context.Background()
	bin, logFile := fakePodman(t)
	p := NewPodman(bin)
	for _, name := range []string{"podman", "app-hub-apps", "app-hub-apps-", "app-hub-apps-1x", "other-net"} {
		if err := p.RemoveNetwork(ctx, name); err == nil {
			t.Errorf("RemoveNetwork(%q) not refused", name)
		}
	}
	if got := calls(t, logFile); got != "" {
		t.Fatalf("podman called for a foreign network: %q", got)
	}
	// The right name but no Kova label (the fake answers an empty label): refused.
	if err := p.RemoveNetwork(ctx, "app-hub-apps-5"); err == nil {
		t.Fatal("unlabelled network removed")
	}
	if got := calls(t, logFile); strings.Contains(got, "network rm") {
		t.Fatalf("network rm ran: %q", got)
	}
	// Start refuses a network that is not an app's.
	if _, err := p.Start(ctx, StartSpec{Name: "apphub-x-1", Labels: map[string]string{Label: "5"}, Network: "podman"}); err == nil {
		t.Fatal("start on a foreign network")
	}
	if AppNetwork(42) != "kova-apps-42" || !HubNetwork(AppNetwork(42)) {
		t.Fatal(AppNetwork(42))
	}
}

// legacyPodman answers every inspect as an App Hub object: no kova.app label,
// app-hub.app=5 (the format asks for both, separated by "|").
func legacyPodman(t *testing.T) (bin, logFile string) {
	t.Helper()
	dir := t.TempDir()
	logFile = filepath.Join(dir, "calls.log")
	bin = filepath.Join(dir, "podman")
	script := "#!/bin/sh\necho \"$@\" >> " + logFile + "\ncase \"$*\" in *inspect*) echo '|5';; esac\nexit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, logFile
}

func TestGuardAcceptsLegacyLabel(t *testing.T) {
	ctx := context.Background()
	bin, logFile := legacyPodman(t)
	p := NewPodman(bin)
	if err := p.Stop(ctx, "velho"); err != nil {
		t.Fatalf("stop legacy container: %v", err)
	}
	if err := p.Remove(ctx, "velho"); err != nil {
		t.Fatalf("remove legacy container: %v", err)
	}
	if err := p.RemoveNetwork(ctx, "app-hub-apps-5"); err != nil {
		t.Fatalf("remove legacy network: %v", err)
	}
	got := calls(t, logFile)
	if !strings.Contains(got, "stop") || !strings.Contains(got, "network rm -- app-hub-apps-5") {
		t.Fatalf("calls: %q", got)
	}
}

func TestLabelsReadNewThenLegacy(t *testing.T) {
	if id, ok := AppLabel(map[string]string{"app-hub.app": "5"}); !ok || id != "5" {
		t.Errorf("legacy: %q %v", id, ok)
	}
	if id, _ := AppLabel(map[string]string{"app-hub.app": "5", "kova.app": "7"}); id != "7" {
		t.Errorf("both: %q", id)
	}
	if _, ok := AppLabel(map[string]string{"x": "1"}); ok {
		t.Error("foreign container read as Kova's")
	}
	if DeploymentID(map[string]string{"app-hub.deployment": "9"}) != "9" || DeploymentID(map[string]string{"kova.deployment": "3", "app-hub.deployment": "9"}) != "3" {
		t.Error("DeploymentID")
	}
	if !HubNetwork("app-hub-apps-5") || !HubNetwork("kova-apps-5") || AppNetwork(5) != "kova-apps-5" || LegacyAppNetwork(5) != "app-hub-apps-5" {
		t.Error("networks")
	}
	if !imageAllowed("", []string{"localhost/apphub/notas:3"}) || !imageAllowed("", []string{"localhost/kova/notas:3"}) || imageAllowed("", []string{"docker.io/x"}) {
		t.Error("imageAllowed")
	}
}

func TestParsePSKeepsLegacyAndNew(t *testing.T) {
	raw := []byte(`[{"Id":"a","Names":["apphub-x-1"],"State":"running","Labels":{"app-hub.app":"1"}},{"Id":"b","Names":["kova-x-2"],"State":"running","Labels":{"kova.app":"1"}},{"Id":"c","Names":["alheio"],"State":"running","Labels":{}}]`)
	got, err := parsePS(raw)
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
}
