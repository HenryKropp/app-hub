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
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/handler"
	"github.com/HenryKropp/app-hub/backend/internal/shared/adopt"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
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
	// Shared apps (design 2026-10-06) run for the main company only: their
	// containers, databases and app addresses use global names, so other
	// companies keep them off until those are company-scoped.
	var gw *gateway.Gateway
	var sup *supervisor.Supervisor
	portal, err := workspace.New(ctx, pool, cfg, workspace.WithPrincipal(func(h *handler.Handler) {
		gw, sup = startShared(ctx, pool, cfg, h)
	}))
	if err != nil {
		slog.Error("workspace initialization", "err", err)
		os.Exit(1)
	}
	defer portal.Close()
	// The Hub (UI, API, Agente) answers right away; app addresses answer
	// "Ligando o app…" until the supervisor has reconciled, so the gateway
	// never wakes an app the Hub is about to adopt.
	gw.Hold()
	go func() {
		if err := sup.Reconcile(ctx); err != nil {
			slog.Warn("supervisor reconcile", "err", err)
		}
		gw.Release()
		sup.Loop(ctx)
	}()

	// Hosts under SHARED_APPS_DOMAIN go to the gateway; the rest is the portal.
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: gw.Root(portal), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
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

// startShared builds the shared-app parts and plugs them into the main
// company's handler. Without HUB_SECRET_KEY or Podman the Hub still starts;
// only shared apps become unavailable.
func startShared(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, h *handler.Handler) (*gateway.Gateway, *supervisor.Supervisor) {
	q := sqlc.New(pool)
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
	dbs := appdb.New(adminPool, cfg.AppsDBHost, eng, cfg.AppsDBContainer, adminUser)
	if err := dbs.LockDown(ctx, pool.Config().ConnConfig.Database); err != nil {
		slog.Warn("lock hub database", "err", err)
	}
	scheme := "http"
	if cfg.SecureCookies() {
		scheme = "https"
	}
	svc := deploy.New(q, eng, dbs, v, deploy.Options{DataDir: cfg.SharedDataDir, Domain: cfg.SharedDomain, Scheme: scheme})
	sup := supervisor.New(q, eng, svc)
	svc.SealOldBackups(ctx)
	if cfg.BaseURL == "" {
		slog.Warn("BASE_URL is empty: app addresses cannot send people to the Hub sign-in")
	}
	gw := gateway.New(cfg.SharedDomain, cfg.BaseURL, cfg.SecureCookies(), h.GatewayBackend())
	h.SetShared(handler.Shared{Svc: svc, Sup: sup, Vault: v, DBs: dbs, Eng: eng, Codes: gw.Codes, DataDir: cfg.SharedDataDir})
	return gw, sup
}
