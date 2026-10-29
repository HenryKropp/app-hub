//go:build podman

package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPodmanRoundTrip(t *testing.T) {
	ctx := context.Background()
	p := NewPodman("")
	if err := p.Available(ctx); err != nil {
		t.Skip("podman indisponível")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM docker.io/library/busybox:latest\nCMD httpd -f -p $PORT\n"), 0o644)
	labels := map[string]string{Label: "999999"}
	img, log, err := p.Build(ctx, BuildSpec{ContextDir: dir, Tag: "localhost/apphub/probe:1", Labels: labels})
	if err != nil {
		t.Fatalf("%v\n%s", err, log)
	}
	t.Cleanup(func() { p.RemoveImage(ctx, img) })
	net := AppNetwork(999999)
	id, err := p.Start(ctx, StartSpec{Name: "apphub-probe-1", Image: img, Labels: labels, Port: 9090, MemoryMB: 128, CPUs: 0.5,
		Env: map[string]string{"PORT": "9090"}, DataDir: filepath.Join(dir, "data"), Network: net})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Remove(ctx, id)
		if err := p.RemoveNetwork(ctx, net); err != nil {
			t.Errorf("remove network: %v", err)
		}
	})
	st, _ := p.Inspect(ctx, id)
	if !st.Running || st.HostPort == 0 {
		t.Fatalf("%+v", st)
	}
	var code int
	for i := 0; i < 20; i++ {
		res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", st.HostPort))
		if err == nil {
			code = res.StatusCode
			res.Body.Close()
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if code == 0 {
		t.Fatal("never answered")
	}
	u, err := p.Stats(ctx, []string{id})
	if err != nil || len(u) != 1 || u[0].MemBytes == 0 {
		t.Fatalf("stats %+v %v", u, err)
	}
	var out strings.Builder
	if err := p.Exec(ctx, id, nil, &out, "nc", "-z", "-w", "3", "host.containers.internal", "5433"); err != nil {
		t.Fatalf("container cannot reach Postgres on host.containers.internal:5433: %v", err)
	}
	// Safety guard: a container without the Hub label must be refused.
	if err := p.Remove(ctx, "kova-db"); err == nil {
		t.Fatal("Remove(kova-db) was not refused")
	}
	if err := p.Stop(ctx, "kova-db"); err == nil {
		t.Fatal("Stop(kova-db) was not refused")
	}
	if dbst, err := p.Inspect(ctx, "kova-db"); err != nil || !dbst.Running {
		t.Fatalf("kova-db no longer running: %+v", dbst)
	}
	if err := p.Stop(ctx, "apphub-does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stop missing: %v", err)
	}
	if err := p.Remove(ctx, "apphub-does-not-exist"); err != nil {
		t.Fatalf("remove missing: %v", err)
	}
	// Same name twice: second Start must fail and leave the first running.
	if _, err := p.Start(ctx, StartSpec{Name: "apphub-probe-1", Image: img, Labels: labels, Port: 9090, MemoryMB: 128, CPUs: 0.5, Network: net}); err == nil {
		t.Fatal("second Start with the same name succeeded")
	}
	if st2, err := p.Inspect(ctx, id); err != nil || !st2.Running {
		t.Fatalf("first container disturbed: %+v %v", st2, err)
	}
	list, err := p.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range list {
		found = found || strings.HasPrefix(id, c.ID) || strings.HasPrefix(c.ID, id)
	}
	if !found {
		t.Fatalf("list %+v", list)
	}
}
