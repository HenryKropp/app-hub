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
	id, err := p.Start(ctx, StartSpec{Name: "apphub-probe-1", Image: img, Labels: labels, Port: 8080, MemoryMB: 128, CPUs: 0.5,
		Env: map[string]string{"PORT": "8080"}, DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Remove(ctx, id) })
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
	if err := p.Remove(ctx, "app-hub-db"); err == nil {
		t.Fatal("Remove(app-hub-db) was not refused")
	}
	if err := p.Stop(ctx, "app-hub-db"); err == nil {
		t.Fatal("Stop(app-hub-db) was not refused")
	}
	if dbst, _ := p.Inspect(ctx, "app-hub-db"); !dbst.Running {
		t.Fatalf("app-hub-db no longer running: %+v", dbst)
	}
	if err := p.Stop(ctx, "apphub-does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stop missing: %v", err)
	}
	if err := p.Remove(ctx, "apphub-does-not-exist"); err != nil {
		t.Fatalf("remove missing: %v", err)
	}
	list, _ := p.List(ctx)
	found := false
	for _, c := range list {
		found = found || strings.HasPrefix(id, c.ID) || strings.HasPrefix(c.ID, id)
	}
	if !found {
		t.Fatalf("list %+v", list)
	}
}
