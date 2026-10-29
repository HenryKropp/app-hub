package adopt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDir(t *testing.T) {
	root := t.TempDir()
	old, nw := filepath.Join(root, "app-hub", "shared"), filepath.Join(root, "ziv", "shared")

	// Neither exists: nothing happens.
	if err := Dir(nw, old); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(nw); !os.IsNotExist(err) {
		t.Fatal("created the new dir out of nothing")
	}

	// Only the old one: renamed, contents kept.
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "dados.txt"), []byte("x"), 0o644)
	if err := Dir(nw, old); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(nw, "dados.txt")); err != nil || string(b) != "x" {
		t.Fatalf("not adopted: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old dir still there")
	}

	// Both exist: nothing changes.
	os.MkdirAll(old, 0o755)
	if err := Dir(nw, old); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("touched the old dir while the new one exists")
	}
}

// fakeRun answers `container exists` from a set and records every call.
func fakeRun(existing ...string) (func(context.Context, ...string) ([]byte, error), *[]string) {
	var calls []string
	set := map[string]bool{}
	for _, n := range existing {
		set[n] = true
	}
	return func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		if len(args) == 3 && args[0] == "container" && args[1] == "exists" && !set[args[2]] {
			return nil, errors.New("exit status 1")
		}
		return nil, nil
	}, &calls
}

func TestContainerRenamesOld(t *testing.T) {
	run, calls := fakeRun("kova-db")
	adopted, err := Container(context.Background(), run, "ziv-db", "kova-db")
	if err != nil || !adopted {
		t.Fatalf("adopted=%v err=%v", adopted, err)
	}
	if last := (*calls)[len(*calls)-1]; last != "rename kova-db ziv-db" {
		t.Fatalf("calls %q", *calls)
	}
}

func TestContainerLeavesNewAlone(t *testing.T) {
	for _, existing := range [][]string{{"ziv-db"}, {"ziv-db", "kova-db"}, nil} {
		run, calls := fakeRun(existing...)
		if adopted, err := Container(context.Background(), run, "ziv-db", "kova-db"); err != nil || adopted {
			t.Fatalf("%v: adopted=%v err=%v", existing, adopted, err)
		}
		for _, c := range *calls {
			if strings.HasPrefix(c, "rename") {
				t.Fatalf("%v: renamed: %q", existing, *calls)
			}
		}
	}
}

func TestNewestContainerPrefersKova(t *testing.T) {
	for _, c := range []struct {
		existing []string
		want     string
	}{
		{[]string{"kova-db", "app-hub-db"}, "kova-db"},
		{[]string{"app-hub-db"}, "app-hub-db"},
		{[]string{"ziv-db", "kova-db"}, ""},
		{nil, ""},
	} {
		run, calls := fakeRun(c.existing...)
		got, err := NewestContainer(context.Background(), run, "ziv-db", "kova-db", "app-hub-db")
		if err != nil || got != c.want {
			t.Fatalf("%v: got %q err %v (calls %q)", c.existing, got, err, *calls)
		}
		renames := 0
		for _, call := range *calls {
			if strings.HasPrefix(call, "rename") {
				renames++
			}
		}
		if (c.want == "") != (renames == 0) || renames > 1 {
			t.Fatalf("%v: calls %q", c.existing, *calls)
		}
	}
}

func TestNewestDirPrefersKova(t *testing.T) {
	root := t.TempDir()
	nw := filepath.Join(root, "ziv", "shared")
	kova, hub := filepath.Join(root, "kova", "shared"), filepath.Join(root, "app-hub", "shared")
	os.MkdirAll(kova, 0o755)
	os.WriteFile(filepath.Join(kova, "de.txt"), []byte("kova"), 0o644)
	os.MkdirAll(hub, 0o755)
	got, err := NewestDir(nw, kova, hub)
	if err != nil || got != kova {
		t.Fatalf("got %q %v", got, err)
	}
	if b, _ := os.ReadFile(filepath.Join(nw, "de.txt")); string(b) != "kova" {
		t.Fatal("did not adopt the kova folder")
	}
	if _, err := os.Stat(hub); err != nil {
		t.Fatal("touched the app-hub folder")
	}
	if got, _ := NewestDir(nw, kova, hub); got != "" {
		t.Fatalf("second run adopted %q", got)
	}

	// Only the App Hub folder: adopted.
	nw2 := filepath.Join(root, "ziv2", "shared")
	if got, err := NewestDir(nw2, filepath.Join(root, "nada"), hub); err != nil || got != hub {
		t.Fatalf("got %q %v", got, err)
	}
}
