package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
