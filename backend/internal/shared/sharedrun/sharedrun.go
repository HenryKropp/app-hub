// Package sharedrun plugs the shared-app parts (screen apps) into each
// company's handler: its own scoped engine view, databases, data folder,
// deploy service, supervisor and gateway (ADR 025).
package sharedrun

import (
	"context"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/handler"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/shared/supervisor"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/HenryKropp/app-hub/backend/internal/workspace"
)

// Deps are the server-wide pieces every company shares.
type Deps struct {
	Engine                           engine.Engine // unscoped; each company sees it through engine.Scoped
	AdminPool                        *pgxpool.Pool
	DBHost, DBContainer, DBAdminUser string
	NewDBs                           func(prefix string) appdb.DBs // tests; nil = appdb.New with Prefix
	Vault                            *vault.Vault                  // nil without HUB_SECRET_KEY
	Router                           *gateway.Router
	Group                            *supervisor.Group
	DataRoot, Domain, Scheme         string
	Secure                           bool
	MaxApps, MaxMemoryMB             int // per company other than the principal
	Slots                            *deploy.Slots
	Enabled                          func(ctx context.Context, slug string) bool
	HealthTimeout                    time.Duration // 0 = the service's default
}

// Attach builds company c's shared-app parts and plugs them into h.
func (d Deps) Attach(c workspace.Company, h *handler.Handler) { d.attach(c, h) }

func (d Deps) attach(c workspace.Company, h *handler.Handler) *deploy.Service {
	t := c.Tenant
	eng := engine.Scoped(d.Engine, t.Key)
	var dbs appdb.DBs
	if d.NewDBs != nil {
		dbs = d.NewDBs(t.DBPrefix())
	} else {
		a := appdb.New(d.AdminPool, d.DBHost, d.Engine, d.DBContainer, d.DBAdminUser)
		a.Prefix = t.DBPrefix()
		dbs = a
	}
	dataDir := t.DataDir(d.DataRoot)
	enabled := func(ctx context.Context) bool { return d.Enabled == nil || d.Enabled(ctx, t.Slug) }
	opt := deploy.Options{DataDir: dataDir, Domain: t.AppsDomain(d.Domain), Scheme: d.Scheme, HealthTimeout: d.HealthTimeout,
		Tenant: t, Slots: d.Slots, Enabled: enabled}
	if !t.Principal() {
		opt.FolderRoot = filepath.Join(dataDir, "code")
		opt.MaxApps, opt.MaxMemoryMB = d.MaxApps, d.MaxMemoryMB
	}
	q := sqlc.New(c.Pool)
	svc := deploy.New(q, eng, dbs, d.Vault, opt)
	svc.SealOldBackups(context.Background())
	sup := supervisor.New(q, eng, svc)
	gw := gateway.New(t.AppsDomain(d.Domain), c.BaseURL, d.Secure, h.GatewayBackend())
	h.SetShared(handler.Shared{Svc: svc, Sup: sup, Vault: d.Vault, DBs: dbs, Eng: eng, Codes: gw.Codes, DataDir: dataDir})
	zone := t.Slug
	if t.Principal() {
		zone = "principal"
	}
	d.Router.Set(zone, gw)
	d.Group.Add(sup, gw, enabled)
	return svc
}
