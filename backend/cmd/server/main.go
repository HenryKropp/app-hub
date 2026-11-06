package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/shared/adopt"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/shared/sharedrun"
	"github.com/HenryKropp/app-hub/backend/internal/shared/supervisor"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/HenryKropp/app-hub/backend/internal/workspace"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var pool *pgxpool.Pool
	for i := range 6 {
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
			pool.Close()
		}
		delay := time.Second * (1 << i)
		slog.Warn("database not ready, retrying", "attempt", i+1, "delay", delay, "err", err)
		time.Sleep(delay)
	}
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
	// Screen apps (shared apps) run in every company, each with its own
	// containers, databases, folders and app addresses (ADR 025). One
	// supervisor group walks the companies; one router picks the company's
	// gateway from the address.
	deps := startShared(ctx, pool, cfg)
	router := gateway.NewRouter(cfg.SharedDomain)
	group := supervisor.NewGroup(deps.Engine)
	deps.Router, deps.Group = router, group
	deps.Slots = deploy.NewSlots(deps.Engine, cfg.SharedAppsMaxRunning, cfg.SharedAppsMemoryBudgetMB)
	// Attach runs inside workspace.New, before the portal exists: until then
	// only the principal company counts as on.
	var portalRef atomic.Pointer[workspace.Portal]
	deps.Enabled = func(ctx context.Context, slug string) bool {
		if p := portalRef.Load(); p != nil {
			return p.SharedEnabled(ctx, slug)
		}
		return slug == "principal"
	}
	portal, err := workspace.New(ctx, pool, cfg, workspace.WithShared(deps.Attach))
	if err != nil {
		slog.Error("workspace initialization", "err", err)
		os.Exit(1)
	}
	defer portal.Close()
	portalRef.Store(portal)
	router.SetLoader(portal.Load)
	// The Hub (UI, API, Agente) answers right away; each company's app
	// addresses answer "Ligando o app…" until the group has reconciled it.
	go group.Loop(ctx)

	// Hosts under SHARED_APPS_DOMAIN go to their company's gateway; the rest is the portal.
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router.Root(portal), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("server starting", "port", cfg.Port, "dev_mode", cfg.DevMode, "shared_domain", cfg.SharedDomain)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
	<-stopped
}

// startShared does the once-only, server-wide steps (vault, adopting old
// containers and folders, the container engine, the apps' database admin)
// and returns what every company's shared-app parts are built from. Without
// HUB_SECRET_KEY or Podman the Hub still starts; only screen apps become
// unavailable.
func startShared(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) sharedrun.Deps {
	v, err := vault.New(cfg.SecretKey)
	if err != nil {
		slog.Warn("shared apps without vault (set HUB_SECRET_KEY)", "err", err)
		v = nil
	}
	// Ziv was Kova, and Kova was the App Hub: take over the newest of their
	// Postgres containers and data folders by renaming them (never
	// recreating), unless the names were set explicitly.
	if !cfg.AppsDBContainerExplicit {
		run := func(ctx context.Context, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, cfg.ContainerEngine, args...).Output()
		}
		if old, err := adopt.NewestContainer(ctx, run, cfg.AppsDBContainer, "kova-db", "app-hub-db"); err != nil {
			slog.Warn("adopt old database container", "err", err)
		} else if old != "" {
			slog.Info("adopted the old database container", "from", old, "to", cfg.AppsDBContainer)
		}
	}
	if !cfg.SharedDataDirExplicit {
		home, _ := os.UserHomeDir()
		olds := []string{
			filepath.Join(home, ".local", "share", "kova", "shared"),
			filepath.Join(home, ".local", "share", "app-hub", "shared"),
		}
		if old, err := adopt.NewestDir(cfg.SharedDataDir, olds...); err != nil {
			slog.Warn("adopt shared data dir", "err", err)
		} else if old != "" {
			slog.Info("adopted the old shared data folder", "from", old, "to", cfg.SharedDataDir)
		}
		// Backup rows keep absolute paths; point the ones under an old folder at
		// the adopted one. Idempotent, so it runs on every start.
		for _, old := range olds {
			if n, err := deploy.MoveBackupPaths(ctx, pool, old, cfg.SharedDataDir); err != nil {
				slog.Warn("move backup paths", "from", old, "err", err)
			} else if n > 0 {
				slog.Info("moved backup paths to the adopted folder", "from", old, "rows", n)
			}
		}
	}
	// Exec (pg_dump/pg_restore) may enter the Hub's own Postgres container;
	// everything else stays limited to containers labelled by the Hub.
	eng := engine.NewPodman(cfg.ContainerEngine, engine.WithExecAllowed(cfg.AppsDBContainer))
	adminPool := pool
	if cfg.AppsDBAdminURL != cfg.DatabaseURL {
		if adminPool, err = pgxpool.New(ctx, cfg.AppsDBAdminURL); err != nil {
			slog.Error("apps db admin", "err", err)
			os.Exit(1)
		}
		go func() { <-ctx.Done(); adminPool.Close() }()
	}
	adminUser := ""
	if adminURL, err := url.Parse(cfg.AppsDBAdminURL); err == nil && adminURL.User != nil {
		adminUser = adminURL.User.Username()
	}
	if err := appdb.New(adminPool, cfg.AppsDBHost, eng, cfg.AppsDBContainer, adminUser).LockDown(ctx, pool.Config().ConnConfig.Database); err != nil {
		slog.Warn("lock hub database", "err", err)
	}
	scheme := "http"
	if cfg.SecureCookies() {
		scheme = "https"
	}
	if cfg.BaseURL == "" {
		slog.Warn("BASE_URL is empty: app addresses cannot send people to the Hub sign-in")
	}
	return sharedrun.Deps{Engine: eng, AdminPool: adminPool, DBHost: cfg.AppsDBHost, DBContainer: cfg.AppsDBContainer, DBAdminUser: adminUser,
		Vault: v, DataRoot: cfg.SharedDataDir, Domain: cfg.SharedDomain, Scheme: scheme, Secure: cfg.SecureCookies(),
		MaxApps: cfg.SharedAppsPerCompany, MaxMemoryMB: cfg.SharedAppMaxMemoryMB}
}
