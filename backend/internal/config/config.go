package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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
	// AppsDBContainerExplicit is true when APPS_DB_CONTAINER was set: the
	// server then never adopts the App Hub's app-hub-db (ADR 013).
	AppsDBContainerExplicit bool
	SharedDomain            string // SHARED_APPS_DOMAIN
	SharedDataDir           string // SHARED_DATA_DIR
	// SharedDataDirExplicit is true when SHARED_DATA_DIR was set (no adoption).
	SharedDataDirExplicit bool
	ContainerEngine       string // CONTAINER_ENGINE
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
	cfg.AppsDBContainer = envOr("APPS_DB_CONTAINER", "kova-db")
	cfg.AppsDBContainerExplicit = os.Getenv("APPS_DB_CONTAINER") != ""
	cfg.SharedDomain = envOr("SHARED_APPS_DOMAIN", "localhost:8080")
	home, _ := os.UserHomeDir()
	cfg.SharedDataDir = envOr("SHARED_DATA_DIR", filepath.Join(home, ".local", "share", "kova", "shared"))
	cfg.SharedDataDirExplicit = os.Getenv("SHARED_DATA_DIR") != ""
	cfg.ContainerEngine = envOr("CONTAINER_ENGINE", "podman")
	return cfg, nil
}

// SecureCookies is true when the Hub is served over HTTPS.
func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.BaseURL, "https://")
}

// redacted is a copy of the config that is safe to print: the secret key and
// any password inside a database address are hidden.
func (c Config) redacted() map[string]any {
	hide := func(s string) string {
		if s == "" {
			return ""
		}
		return "[escondido]"
	}
	return map[string]any{
		"Port": c.Port, "DatabaseURL": hideURLPassword(c.DatabaseURL), "BaseURL": c.BaseURL, "DevMode": c.DevMode,
		"AgentBinaryPath": c.AgentBinaryPath, "SecretKey": hide(c.SecretKey),
		"AppsDBAdminURL": hideURLPassword(c.AppsDBAdminURL), "AppsDBHost": c.AppsDBHost,
		"AppsDBContainer": c.AppsDBContainer, "SharedDomain": c.SharedDomain,
		"SharedDataDir": c.SharedDataDir, "ContainerEngine": c.ContainerEngine,
	}
}

func hideURLPassword(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		if s == "" {
			return ""
		}
		return "[escondido]"
	}
	if _, has := u.User.Password(); has {
		u.User = url.UserPassword(u.User.Username(), "xxxxx")
	}
	q := u.Query()
	if q.Has("password") {
		q.Set("password", "xxxxx")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// String, GoString and LogValue keep secrets out of logs and error messages.
func (c Config) String() string {
	r := c.redacted()
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%s:%v", k, r[k])
	}
	b.WriteString("}")
	return b.String()
}

func (c Config) GoString() string { return "config.Config" + c.String() }

func (c Config) LogValue() slog.Value {
	r := c.redacted()
	attrs := make([]slog.Attr, 0, len(r))
	for k, v := range r {
		attrs = append(attrs, slog.Any(k, v))
	}
	return slog.GroupValue(attrs...)
}

// Format makes %v, %+v, %#v and %s all go through the hidden copy.
func (c Config) Format(f fmt.State, verb rune) {
	if verb == 'v' && f.Flag('#') {
		fmt.Fprint(f, c.GoString())
		return
	}
	fmt.Fprint(f, c.String())
}
