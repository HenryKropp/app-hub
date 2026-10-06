package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
	BaseURL     string
	DevMode     bool

	AgentBinaryPath string

	SecretKey       string // HUB_SECRET_KEY
	AppsDBAdminURL  string // APPS_DB_ADMIN_URL; default: DATABASE_URL
	AppsDBHost      string // APPS_DB_HOST_FOR_CONTAINERS
	AppsDBContainer string // APPS_DB_CONTAINER (copies via exec)
	SharedDomain    string // SHARED_APPS_DOMAIN
	SharedDataDir   string // SHARED_DATA_DIR
	ContainerEngine string // CONTAINER_ENGINE
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func Load() (Config, error) {
	cfg := Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BaseURL:     os.Getenv("BASE_URL"),
		DevMode:     os.Getenv("DEV_MODE") != "",

		AgentBinaryPath: os.Getenv("AGENT_BINARY_PATH"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	cfg.SecretKey = os.Getenv("HUB_SECRET_KEY")
	cfg.AppsDBAdminURL = envOr("APPS_DB_ADMIN_URL", cfg.DatabaseURL)
	cfg.AppsDBHost = envOr("APPS_DB_HOST_FOR_CONTAINERS", "host.containers.internal:5433")
	cfg.AppsDBContainer = envOr("APPS_DB_CONTAINER", "app-hub-db")
	cfg.SharedDomain = envOr("SHARED_APPS_DOMAIN", "localhost:8080")
	home, _ := os.UserHomeDir()
	cfg.SharedDataDir = envOr("SHARED_DATA_DIR", filepath.Join(home, ".local", "share", "app-hub", "shared"))
	cfg.ContainerEngine = envOr("CONTAINER_ENGINE", "podman")
	return cfg, nil
}

// SecureCookies is true when the Hub is served over HTTPS.
func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.BaseURL, "https://")
}
