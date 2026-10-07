package handler

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

type sharedDeps struct {
	svc   *deploy.Service
	eng   *engine.Fake
	dbs   *appdb.Fake
	codes *gateway.Codes
}

// withShared plugs fake shared-app parts into h (use with newPoolClient, since
// the deploy service works in the background).
func withShared(t *testing.T, h *Handler) sharedDeps {
	t.Helper()
	v, _ := vault.New(vault.NewKey())
	d := sharedDeps{eng: engine.NewFake(), dbs: appdb.NewFake(), codes: gateway.NewCodes()}
	t.Cleanup(d.eng.Close)
	d.svc = deploy.New(h.q, d.eng, d.dbs, v, deploy.Options{DataDir: t.TempDir(), Domain: "localhost:8080", Scheme: "http", HealthTimeout: time.Second})
	h.SetShared(Shared{Svc: d.svc, Vault: v, DBs: d.dbs, Eng: d.eng, Codes: d.codes, DataDir: t.TempDir()})
	return d
}

// sharedApp registers a web app (folder = copy of testdata/shared/streamlit) and marks it shared.
func sharedApp(t *testing.T, h *Handler, d sharedDeps, slug string) int64 {
	t.Helper()
	dir := t.TempDir()
	deploytest.CopyDir(t, "../../testdata/shared/streamlit", dir)
	m := manifest.Manifest{ManifestVersion: 1, ID: slug, Name: "Notas", Kind: manifest.KindWeb, Entrypoint: "app.py",
		Shared: &manifest.Shared{Enabled: true}}
	id := insertApp(t, m)
	if _, err := testPool.Exec(context.Background(), "UPDATE apps SET folder_path = $1 WHERE id = $2", dir, id); err != nil {
		t.Fatal(err)
	}
	app, err := h.q.GetApp(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.svc.Configure(context.Background(), app, m); err != nil {
		t.Fatal(err)
	}
	return id
}

// noRedirect makes the client stop at the first redirect, to read Location.
func (c *client) noRedirect() {
	c.c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
}

// raw sends a request and returns the response as is.
func (c *client) raw(method, path string, body io.Reader) *http.Response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.srv.URL+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { res.Body.Close() })
	return res
}
