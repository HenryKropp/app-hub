package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/handler"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

var testCredentials = map[string]any{"name": "Test admin", "email": "admin@example.test", "password": "test-password-not-a-real-secret"}

// fakeShared plugs fake shared-app parts into every company.
func fakeShared(t *testing.T, p *Portal) Option {
	v, _ := vault.New(vault.NewKey())
	return WithShared(func(c Company, h *handler.Handler) {
		slug, pool := c.Tenant.Slug, c.Pool
		eng := engine.NewFake()
		t.Cleanup(eng.Close)
		dbs := appdb.NewFake()
		dir := t.TempDir()
		svc := deploy.New(sqlc.New(pool), eng, dbs, v, deploy.Options{DataDir: dir, Domain: "localhost:8080", Scheme: "http",
			HealthTimeout: time.Second, Enabled: func(ctx context.Context) bool { return p.SharedEnabled(ctx, slug) }})
		h.SetShared(handler.Shared{Svc: svc, Vault: v, DBs: dbs, Eng: eng, Codes: gateway.NewCodes(), DataDir: dir})
	})
}

// twoCompanies sets up the principal admin and companies "teste" and "outra".
func twoCompanies(t *testing.T, srvURL string) *testBrowser {
	b := browser(t, srvURL)
	b.want("POST", "/api/auth/setup", testCredentials, 201)
	for _, c := range [][2]string{{"Teste", "teste"}, {"Outra", "outra"}} {
		b.want("POST", "/api/workspaces", map[string]any{"name": c[0], "slug": c[1], "password": testCredentials["password"]}, 201)
	}
	return b
}

func TestRegistrySeedsSharedApps(t *testing.T) {
	ctx := context.Background()
	p, pool, srv := testPortal(t)
	twoCompanies(t, srv.URL)
	if _, err := pool.Exec(ctx, `ALTER TABLE public.workspace_registry DROP COLUMN shared_apps`); err != nil {
		t.Fatal(err)
	}
	p2, err := New(ctx, pool, p.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if !p2.SharedEnabled(ctx, "principal") || !p2.SharedEnabled(ctx, "teste") || p2.SharedEnabled(ctx, "outra") {
		t.Fatal("seed: principal and teste on, outra off")
	}
	srv2 := httptest.NewServer(p2)
	defer srv2.Close()
	b := browser(t, srv2.URL)
	b.want("POST", "/api/auth/login", map[string]any{"email": testCredentials["email"], "password": testCredentials["password"]}, 200)
	b.want("POST", "/api/workspaces", map[string]any{"name": "Nova", "slug": "nova", "password": testCredentials["password"]}, 201)
	if p2.SharedEnabled(ctx, "nova") {
		t.Fatal("a new company starts with screen apps on")
	}
}

func TestCompanySharedOffAnswers503(t *testing.T) {
	ctx := context.Background()
	p, pool, srv := testPortal(t)
	b := twoCompanies(t, srv.URL)
	b.want("POST", "/w/outra/api/auth/login", testCredentials, 200)
	importApp(t, b, "/w/outra", "Outra app")

	var p2 *Portal
	p2, err := New(ctx, pool, p.cfg, fakeShared(t, p))
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	srv2 := httptest.NewServer(p2)
	defer srv2.Close()
	b2 := browser(t, srv2.URL)
	b2.want("POST", "/w/outra/api/auth/login", testCredentials, 200)
	data := b2.want("GET", "/w/outra/api/apps/1/shared", nil, 503)
	if !bytes.Contains(data, []byte(`"code":"shared_runtime_unavailable"`)) || !bytes.Contains(data, []byte("Peça ao administrador do Ziv para ligá-los")) {
		t.Fatalf("%s", data)
	}
	if err := p2.SetCompanyShared(ctx, "outra", true); err != nil {
		t.Fatal(err)
	}
	if code, data := b2.do("GET", "/w/outra/api/apps/1/shared", nil); code == 503 {
		t.Fatalf("still off: %s", data)
	}
}

func TestAdminCompaniesOnlyInPrincipal(t *testing.T) {
	_, _, srv := testPortal(t)
	b := twoCompanies(t, srv.URL)
	var list []handler.CompanyShared
	if err := json.Unmarshal(b.want("GET", "/api/admin/companies", nil, 200), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Slug != "outra" || list[0].SharedApps || list[1].Slug != "teste" {
		t.Fatalf("%+v", list)
	}
	var one handler.CompanyShared
	if err := json.Unmarshal(b.want("PUT", "/api/admin/companies/outra/shared", map[string]bool{"on": true}, 200), &one); err != nil || !one.SharedApps || one.Name != "Outra" {
		t.Fatalf("%+v %v", one, err)
	}
	b.want("PUT", "/api/admin/companies/principal/shared", map[string]bool{"on": false}, 404)
	b.want("PUT", "/api/admin/companies/nao-existe/shared", map[string]bool{"on": true}, 404)
	b.want("POST", "/w/outra/api/auth/login", testCredentials, 200)
	b.want("GET", "/w/outra/api/admin/companies", nil, 404)
	b.want("PUT", "/w/outra/api/admin/companies/teste/shared", map[string]bool{"on": true}, 404)

	b.want("POST", "/api/people", map[string]any{"name": "Member", "email": "member@example.test", "password": "member-test-password", "role": "usuario"}, 201)
	member := browser(t, srv.URL)
	member.want("POST", "/api/auth/login", map[string]any{"email": "member@example.test", "password": "member-test-password"}, 200)
	member.want("GET", "/api/admin/companies", nil, 403)
	member.want("PUT", "/api/admin/companies/outra/shared", map[string]bool{"on": false}, 403)
}
