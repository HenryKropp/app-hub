package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/database"
	"github.com/HenryKropp/app-hub/backend/internal/workspace"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()

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

	portal, err := workspace.New(ctx, pool, cfg)
	if err != nil {
		slog.Error("workspace initialization", "err", err)
		os.Exit(1)
	}
	defer portal.Close()
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: portal, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	slog.Info("server starting", "port", cfg.Port, "dev_mode", cfg.DevMode)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
