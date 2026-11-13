package deploy_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

func boxes(t *testing.T, f *engine.Fake, labels ...map[string]string) {
	t.Helper()
	for i, l := range labels {
		if _, err := f.Start(context.Background(), engine.StartSpec{Name: "box-" + itoa(int64(i)), Labels: l, Network: engine.AppNetwork("", int64(900+i))}); err != nil {
			t.Fatal(err)
		}
	}
}

func box(mem string, more ...string) map[string]string {
	l := map[string]string{engine.Label: "9"}
	if mem != "" {
		l[engine.MemoryLabel] = mem
	}
	for i := 0; i+1 < len(more); i += 2 {
		l[more[i]] = more[i+1]
	}
	return l
}

func TestSlotsFullByCount(t *testing.T) {
	ctx := context.Background()
	f := engine.NewFake()
	t.Cleanup(f.Close)
	boxes(t, f, box("256"), box("256"))
	var le *deploy.LimitError
	if _, err := deploy.NewSlots(f, 2, 0).Acquire(ctx, 128, nil); !errors.As(err, &le) || !strings.Contains(le.Error(), "(2)") {
		t.Fatal(err)
	}
	if release, err := deploy.NewSlots(f, 2, 0).Acquire(ctx, 128, &deploy.Owner{AppID: 9}); err != nil {
		t.Fatal(err)
	} else {
		release()
	}
}

func TestSlotsFullByMemory(t *testing.T) {
	ctx := context.Background()
	f := engine.NewFake()
	t.Cleanup(f.Close)
	boxes(t, f, box("256"))
	var le *deploy.LimitError
	if _, err := deploy.NewSlots(f, 0, 512).Acquire(ctx, 300, nil); !errors.As(err, &le) {
		t.Fatal(err)
	}
	if _, err := deploy.NewSlots(f, 0, 512).Acquire(ctx, 256, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSlotsCountsInflight(t *testing.T) {
	ctx := context.Background()
	f := engine.NewFake()
	t.Cleanup(f.Close)
	s := deploy.NewSlots(f, 1, 0)
	release, err := s.Acquire(ctx, 128, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Acquire(ctx, 128, nil); err == nil {
		t.Fatal("second reservation passed")
	}
	release()
	if _, err := s.Acquire(ctx, 128, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSlotsMissingLabelCountsDefault(t *testing.T) {
	f := engine.NewFake()
	t.Cleanup(f.Close)
	boxes(t, f, box(""))
	if _, err := deploy.NewSlots(f, 0, 600).Acquire(context.Background(), 128, nil); err == nil {
		t.Fatal("old box counted as less than 512 MB")
	}
}

func TestNilSlotsNoLimit(t *testing.T) {
	var s *deploy.Slots
	release, err := s.Acquire(context.Background(), 4096, nil)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestSlotsCountsAllCompanies(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	boxes(t, e.Eng, box("128", engine.WorkspaceLabel, "wA"), box("128", engine.WorkspaceLabel, "wB"))
	e.S = e.NewTenantServiceSlots(company, deploy.NewSlots(e.Eng, 2, 0))
	d, err := e.S.Publish(ctx, e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if d = e.WaitDeployment(t, d.ID); d.Status != "failed" || !strings.Contains(d.ErrorMessage, "no limite") {
		t.Fatalf("%s: %s", d.Status, d.ErrorMessage)
	}
}

func TestWakeWhenFullSleepsWithMessage(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	if d := e.Publish(t); d.Status != "live" {
		t.Fatal(d.ErrorMessage)
	}
	if err := e.S.Sleep(ctx, e.App.ID); err != nil {
		t.Fatal(err)
	}
	boxes(t, e.Eng, box("128"), box("128"))
	e.S = e.NewTenantServiceSlots(deployPrincipal, deploy.NewSlots(e.Eng, 2, 0))
	var le *deploy.LimitError
	if err := e.S.Run(ctx, e.App.ID); !errors.As(err, &le) {
		t.Fatal(err)
	}
	if sa := e.Shared(t); sa.Status != "sleeping" || !strings.Contains(sa.StatusMessage, "no limite") {
		t.Fatalf("%s %q", sa.Status, sa.StatusMessage)
	}
}

// A box that replaces the app's own may keep its memory over the budget, but
// growing must fit: only the extra memory over its own box counts.
func TestSlotsReplacingChecksGrowth(t *testing.T) {
	ctx := context.Background()
	f := engine.NewFake()
	t.Cleanup(f.Close)
	boxes(t, f, box("256"), box("256", engine.Label, "7"), box("256", engine.Label, "8"))
	s := deploy.NewSlots(f, 0, 768)
	var le *deploy.LimitError
	if _, err := s.Acquire(ctx, 512, &deploy.Owner{AppID: 9}); !errors.As(err, &le) || !strings.Contains(le.Error(), "memória") {
		t.Fatalf("growing past the budget: %v", err)
	}
	release, err := s.Acquire(ctx, 256, &deploy.Owner{AppID: 9})
	if err != nil {
		t.Fatalf("same memory: %v", err)
	}
	release()
	// another company's app 9 is not this app's own box
	if _, err := s.Acquire(ctx, 256, &deploy.Owner{Workspace: "outra", AppID: 9}); err == nil {
		t.Fatal("another company's box counted as own")
	}
}

// Refused for memory, the message says memory, not the count of apps.
func TestSlotsMemoryMessage(t *testing.T) {
	f := engine.NewFake()
	t.Cleanup(f.Close)
	boxes(t, f, box("512"))
	var le *deploy.LimitError
	if _, err := deploy.NewSlots(f, 2, 768).Acquire(context.Background(), 512, nil); !errors.As(err, &le) ||
		!strings.Contains(le.Error(), "sem memória") || strings.Contains(le.Error(), "(2)") {
		t.Fatal(err)
	}
}
