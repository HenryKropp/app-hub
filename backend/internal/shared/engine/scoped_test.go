package engine

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func startOn(t *testing.T, f *Fake, name string, labels map[string]string) string {
	t.Helper()
	id, err := f.Start(context.Background(), StartSpec{Name: name, Labels: labels, Network: AppNetwork("", 1)})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func names(t *testing.T, e Engine) []string {
	t.Helper()
	l, err := e.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range l {
		out = append(out, c.Name)
	}
	slices.Sort(out)
	return out
}

func TestScopedListPrincipalKeepsUnlabelled(t *testing.T) {
	f := NewFake()
	startOn(t, f, "a", map[string]string{Label: "1"})
	startOn(t, f, "b", map[string]string{"app-hub.app": "2"})
	startOn(t, f, "c", map[string]string{Label: "1", WorkspaceLabel: "w0123456789a"})
	if got := names(t, Scoped(f, "")); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatal(got)
	}
	if got := names(t, Scoped(f, "w0123456789a")); !slices.Equal(got, []string{"c"}) {
		t.Fatal(got)
	}
}

func TestScopedRefusesOtherCompanysBox(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	idA := startOn(t, f, "a", map[string]string{Label: "1", WorkspaceLabel: "wA"})
	b := Scoped(f, "wB")
	if err := b.Remove(ctx, idA); !errors.Is(err, ErrOtherWorkspace) {
		t.Fatal("remove", err)
	}
	if err := b.Stop(ctx, idA); !errors.Is(err, ErrOtherWorkspace) {
		t.Fatal("stop", err)
	}
	if _, err := b.Logs(ctx, idA, 10); !errors.Is(err, ErrOtherWorkspace) {
		t.Fatal("logs", err)
	}
	if err := Scoped(f, "").Remove(ctx, idA); !errors.Is(err, ErrOtherWorkspace) {
		t.Fatal("principal remove", err)
	}
	if !slices.Contains(f.Running(), "a") {
		t.Fatal("A's box was touched", f.Running())
	}
}

func TestScopedStartLabelsAndChecksNetwork(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	b := Scoped(f, "wb0000000000")
	id, err := b.Start(ctx, StartSpec{Name: "x", Labels: map[string]string{Label: "1"}, Network: AppNetwork("wb0000000000", 1)})
	if err != nil {
		t.Fatal(err)
	}
	if f.Containers[id].Spec.Labels[WorkspaceLabel] != "wb0000000000" {
		t.Fatal(f.Containers[id].Spec.Labels)
	}
	if _, err := b.Start(ctx, StartSpec{Name: "y", Labels: map[string]string{Label: "1"}, Network: AppNetwork("", 1)}); err == nil {
		t.Fatal("started on principal's network")
	}
	if _, err := Scoped(f, "").Start(ctx, StartSpec{Name: "z", Labels: map[string]string{Label: "1"}, Network: AppNetwork("wb0000000000", 1)}); err == nil {
		t.Fatal("principal started on a company's network")
	}
}

func TestScopedBuildLabels(t *testing.T) {
	f := NewFake()
	if _, _, err := Scoped(f, "wB").Build(context.Background(), BuildSpec{Tag: "t", Labels: map[string]string{Label: "1"}}); err != nil {
		t.Fatal(err)
	}
	for _, b := range f.Images {
		if b.Labels[WorkspaceLabel] != "wB" {
			t.Fatal(b.Labels)
		}
	}
}

func TestScopedRemoveNetwork(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	for _, n := range []string{"ziv-apps-1", "app-hub-apps-1"} {
		if err := Scoped(f, "wb0000000000").RemoveNetwork(ctx, n); !errors.Is(err, ErrOtherWorkspace) {
			t.Fatal(n, err)
		}
	}
	if err := Scoped(f, "").RemoveNetwork(ctx, "ziv-apps-wb0000000000-1"); !errors.Is(err, ErrOtherWorkspace) {
		t.Fatal(err)
	}
	if err := Scoped(f, "wb0000000000").RemoveNetwork(ctx, "ziv-apps-wb0000000000-1"); err != nil {
		t.Fatal(err)
	}
}

func TestCompanyNetworkNames(t *testing.T) {
	if !HubNetwork("ziv-apps-w0123456789a-5") || HubNetwork("ziv-apps-w0123-5") || AppNetwork("w0123456789a", 5) != "ziv-apps-w0123456789a-5" {
		t.Fatal("company networks")
	}
	if ImagePrefix != "localhost/ziv/" { // tenant keeps a copy of this prefix
		t.Fatal(ImagePrefix)
	}
	if WorkspaceOf(map[string]string{WorkspaceLabel: "wX"}) != "wX" || WorkspaceOf(nil) != "" {
		t.Fatal("WorkspaceOf")
	}
}
