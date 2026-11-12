package deploy_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/tenant"
)

var company = tenant.Tenant{Slug: "teste", Key: "w0123456789a"}
var deployPrincipal = tenant.Principal

func countDeployments(t *testing.T) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM shared_deployments`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCompanyPublishUsesCompanyNames(t *testing.T) {
	e := setup(t)
	e.S = e.NewTenantService(company, "", 0, 0)
	d := e.Publish(t)
	if d.Status != "live" {
		t.Fatalf("%s: %s", d.Status, d.ErrorMessage)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "ziv-w0123456789a-notas-"+itoa(d.ID) {
		t.Fatal(got)
	}
	for _, c := range e.Eng.Containers {
		if c.Spec.Network != "ziv-apps-w0123456789a-"+itoa(e.App.ID) || c.Spec.Labels[engine.WorkspaceLabel] != "w0123456789a" {
			t.Fatalf("%+v", c.Spec)
		}
	}
	if !strings.HasPrefix(e.Eng.Images[d.ImageID].Tag, "localhost/ziv/w0123456789a/notas:") {
		t.Fatal(e.Eng.Images[d.ImageID].Tag)
	}
	if sa := e.Shared(t); sa.DbName != "app_w0123456789a_"+itoa(e.App.ID) {
		t.Fatal(sa.DbName)
	}
	if u := e.S.AppURL("notas"); u != "http://notas.teste.localhost:8080" {
		t.Fatal(u)
	}
}

func TestFolderOutsideRootRefused(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	root := filepath.Join(company.DataDir(e.DataDir()), "code") // the company's code folder, where CodeDir lives
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	e.S = e.NewTenantService(company, root, 0, 0)
	if _, err := e.S.Publish(ctx, e.App, e.M, 1); !errors.Is(err, deploy.ErrFolderOutside) {
		t.Fatal("publish", err)
	}
	if _, err := e.S.Test(ctx, e.App, e.M, 1); !errors.Is(err, deploy.ErrFolderOutside) {
		t.Fatal("test", err)
	}
	if n := countDeployments(t); n != 0 {
		t.Fatal(n)
	}
	link := filepath.Join(root, "x")
	if err := os.Symlink(e.App.FolderPath, link); err != nil {
		t.Fatal(err)
	}
	e.App.FolderPath = link
	if _, err := e.S.Publish(ctx, e.App, e.M, 1); !errors.Is(err, deploy.ErrFolderOutside) {
		t.Fatal("symlink", err)
	}
	code := e.S.CodeDir(e.App.ID)
	deploytest.CopyDir(t, deploytest.Testdata("streamlit"), code)
	e.App.FolderPath = code
	if d := e.Publish(t); d.Status != "live" {
		t.Fatal(d.Status, d.ErrorMessage)
	}
}

func TestMaxAppsBlocksSecondApp(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.S = e.NewTenantService(company, "", 1, 0)
	if d := e.Publish(t); d.Status != "live" {
		t.Fatal(d.ErrorMessage)
	}
	other := e.AddApp(t, "outra")
	var le *deploy.LimitError
	if _, err := e.S.Publish(ctx, other, e.M, 1); !errors.As(err, &le) {
		t.Fatal("publish", err)
	}
	if _, err := e.S.Test(ctx, other, e.M, 1); !errors.As(err, &le) {
		t.Fatal("test", err)
	}
	if d := e.Publish(t); d.Status != "live" {
		t.Fatal("update blocked", d.ErrorMessage)
	}
}

func TestMaxMemoryOnConfigure(t *testing.T) {
	e := setup(t)
	s := e.NewTenantService(company, "", 0, 256)
	m := e.M
	m.Shared = &manifest.Shared{Enabled: true, MemoryMB: 512}
	var le *deploy.LimitError
	if _, err := s.Configure(context.Background(), e.App, m); !errors.As(err, &le) || !strings.Contains(le.Error(), "256 MB") {
		t.Fatal(err)
	}
}

func TestCompanyUnshareKeepsPrincipalNetwork(t *testing.T) {
	e := setup(t)
	if d := e.Publish(t); d.Status != "live" { // principal box on ziv-apps-<id>
		t.Fatal(d.ErrorMessage)
	}
	cs := e.NewTenantService(company, "", 0, 0)
	_ = cs.Unshare(context.Background(), e.App.ID, 1)
	if !e.Eng.HasNetwork("ziv-apps-" + itoa(e.App.ID)) {
		t.Fatal("principal network removed")
	}
	if got := e.Eng.Running(); len(got) != 1 {
		t.Fatal("principal box touched", got)
	}
}

func TestPublishWhenOff(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	on := true
	e.S = e.NewServiceOpts(deploy.Options{Enabled: func(context.Context) bool { return on }})
	if d := e.Publish(t); d.Status != "live" {
		t.Fatalf("%s: %s", d.Status, d.ErrorMessage)
	}
	if err := e.S.Sleep(ctx, e.App.ID); err != nil {
		t.Fatal(err)
	}
	on = false
	if _, err := e.S.Publish(ctx, e.App, e.M, 1); !errors.Is(err, deploy.ErrSharedOff) {
		t.Fatal("publish", err)
	}
	if _, err := e.S.Test(ctx, e.App, e.M, 1); !errors.Is(err, deploy.ErrSharedOff) {
		t.Fatal("test", err)
	}
	if err := e.S.Run(ctx, e.App.ID); !errors.Is(err, deploy.ErrSharedOff) {
		t.Fatal("run", err)
	}
	if err := e.S.Stop(ctx, e.App.ID, 1); err != nil {
		t.Fatal("stop", err)
	}
	if deploy.ErrSharedOff.Error() != "Os apps com tela não estão ligados para esta empresa. Peça ao administrador do Ziv para ligá-los." {
		t.Fatal(deploy.ErrSharedOff)
	}
}

// An app with the browser gets the browser's memory even where the company's
// cap per app is lower, and even when it had less before.
func TestBrowserAppGetsBrowserMemory(t *testing.T) {
	e := setup(t)
	s := e.NewTenantService(company, "", 0, 256)
	app := e.App
	app.FolderPath = t.TempDir()
	deploytest.CopyDir(t, deploytest.Testdata("playwright-py"), app.FolderPath)
	m := e.M
	for _, mem := range []int{0, 256} {
		m.Shared = &manifest.Shared{Enabled: true, MemoryMB: mem}
		sa, err := s.Configure(context.Background(), app, m)
		if err != nil || sa.MemoryMb != 512 {
			t.Fatalf("memory %d: got %d, %v", mem, sa.MemoryMb, err)
		}
	}
	m.Shared = &manifest.Shared{Enabled: true, MemoryMB: 1024}
	var le *deploy.LimitError
	if _, err := s.Configure(context.Background(), app, m); !errors.As(err, &le) || !strings.Contains(le.Error(), "512 MB") {
		t.Fatal(err)
	}
}
