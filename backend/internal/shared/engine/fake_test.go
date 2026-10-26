package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
)

func TestFakeStartServesAndCrashes(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	img, _, err := f.Build(ctx, BuildSpec{Tag: "localhost/apphub/x:1", Labels: map[string]string{Label: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.Start(ctx, StartSpec{Name: "apphub-x-1", Image: img, Port: 8501, Labels: map[string]string{Label: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	st, _ := f.Inspect(ctx, id)
	if !st.Running || st.HostPort == 0 {
		t.Fatalf("state %+v", st)
	}
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", st.HostPort))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != "ok apphub-x-1" {
		t.Fatalf("body %q", body)
	}
	f.Crash(id)
	st, _ = f.Inspect(ctx, id)
	if st.Running || st.ExitCode != 1 {
		t.Fatalf("after crash %+v", st)
	}
	if err := f.Remove(ctx, id); err != nil {
		t.Fatal(err)
	}
	if st, _ := f.Inspect(ctx, id); st.Exists {
		t.Fatal("still exists")
	}
}

func TestFakeBuildErrOnce(t *testing.T) {
	f := NewFake()
	f.BuildErr = fmt.Errorf("pip falhou")
	if _, _, err := f.Build(context.Background(), BuildSpec{Tag: "a"}); err == nil {
		t.Fatal("want error")
	}
	if _, _, err := f.Build(context.Background(), BuildSpec{Tag: "a"}); err != nil {
		t.Fatal("second build should work")
	}
}

func TestFakeListOnlyLabelled(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	f.Start(ctx, StartSpec{Name: "a", Labels: map[string]string{Label: "1"}})
	f.Start(ctx, StartSpec{Name: "b"}) // sem rótulo: não pode aparecer
	list, _ := f.List(ctx)
	if len(list) != 1 || list[0].Name != "a" {
		t.Fatalf("%+v", list)
	}
}

func TestFakeIDsUniqueAfterRemove(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	defer f.Close()
	a, _ := f.Start(ctx, StartSpec{Name: "a"})
	b, _ := f.Start(ctx, StartSpec{Name: "a"})
	f.Remove(ctx, a)
	c, _ := f.Start(ctx, StartSpec{Name: "a"})
	if c == b || c == a || a == b {
		t.Fatalf("ids collide: %s %s %s", a, b, c)
	}
	i1, _, _ := f.Build(ctx, BuildSpec{Tag: "x"})
	i2, _, _ := f.Build(ctx, BuildSpec{Tag: "x"})
	f.RemoveImage(ctx, i1)
	i3, _, _ := f.Build(ctx, BuildSpec{Tag: "x"})
	if i3 == i2 || i3 == i1 {
		t.Fatalf("image ids collide: %s %s %s", i1, i2, i3)
	}
	if len(f.Running()) != 2 {
		t.Fatalf("running %v", f.Running())
	}
}

func TestFakeSetHandler(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	defer f.Close()
	id, _ := f.Start(ctx, StartSpec{Name: "a"})
	f.SetHandler(id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	st, _ := f.Inspect(ctx, id)
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", st.HostPort))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 500 {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestFakeStopUnknown(t *testing.T) {
	if err := NewFake().Stop(context.Background(), "nope"); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestFakeClose(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	id, _ := f.Start(ctx, StartSpec{Name: "a"})
	st, _ := f.Inspect(ctx, id)
	f.Close()
	if len(f.Running()) != 0 {
		t.Fatal("still running")
	}
	if _, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", st.HostPort)); err == nil {
		t.Fatal("server still up")
	}
}

func TestFakeConcurrent(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	defer f.Close()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				id, _ := f.Start(ctx, StartSpec{Name: "c"})
				f.SetHandler(id, http.NotFoundHandler())
				f.Inspect(ctx, id)
				f.Available(ctx)
				f.Stop(ctx, id)
				f.CallsCopy()
				f.Remove(ctx, id)
			}
		}()
	}
	wg.Wait()
}
