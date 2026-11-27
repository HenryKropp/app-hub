package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	Port string
	// ListenHost is where the server answers (LISTEN_HOST, default
	// 127.0.0.1): only Caddy on the same machine reaches it, not app boxes.
	ListenHost string
	// OAuthRedirectHosts are extra hosts an OAuth client may return to,
	// besides Claude and this computer (OAUTH_REDIRECT_HOSTS, comma separated).
	OAuthRedirectHosts []string
	DatabaseURL        string
	BaseURL            string
	DevMode            bool
	SetupCode          string // SETUP_CODE: code for the principal's first access

	// E-mail of external link codes (ADR 045); both empty = e-mail off.
	ResendAPIKey string // RESEND_API_KEY
	MailFrom     string // MAIL_FROM, ex.: "Ziv <codigo@ziv.app.br>"

	AgentBinaryPath string

	SecretKey string // HUB_SECRET_KEY
	// OldSecretKeys (HUB_SECRET_KEYS_OLD, comma separated) still open values
	// sealed before a key change, until cmd/trocarchave re-seals them.
	OldSecretKeys   []string
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

	// Screen apps per company (ADR 025). The principal company has no
	// per-company limits; the server-wide ones apply to every company.
	SharedAppsPerCompany     int // SHARED_APPS_PER_COMPANY: published screen apps in a company
	SharedAppMaxMemoryMB     int // SHARED_APP_MAX_MEMORY_MB: memory per screen app in a company
	SharedAppsMaxRunning     int // SHARED_APPS_MAX_RUNNING: screen apps on at once, whole server
	SharedAppsMemoryBudgetMB int // SHARED_APPS_MEMORY_BUDGET_MB: their memory together, whole server

	CompanyLimit         int             // COMPANY_LIMIT: companies in the portal, the main one included
	CompanySignupsPerDay int             // COMPANY_SIGNUPS_PER_DAY: new companies in any 24 hours
	ReservedSlugs        map[string]bool // built-in names plus RESERVED_COMPANY_SLUGS (comma-separated)
}

// builtinReservedSlugs are addresses public signup can never take.
var builtinReservedSlugs = []string{"principal", "admin", "api", "app", "apps", "w", "www", "ziv", "suporte", "ajuda", "entrar", "login", "cadastro", "acesso", "convite", "link"}

func envInt(name string, def int) int {
	var n int
	if _, err := fmt.Sscan(os.Getenv(name), &n); err != nil || n <= 0 {
		return def
	}
	return n
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func Load() (Config, error) {
	cfg := Config{
		Port:         os.Getenv("PORT"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		BaseURL:      os.Getenv("BASE_URL"),
		DevMode:      os.Getenv("DEV_MODE") != "",
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		MailFrom:     os.Getenv("MAIL_FROM"),
		SetupCode:    os.Getenv("SETUP_CODE"),

		AgentBinaryPath: os.Getenv("AGENT_BINARY_PATH"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	cfg.ListenHost = envOr("LISTEN_HOST", "127.0.0.1")
	for _, h := range strings.Split(os.Getenv("OAUTH_REDIRECT_HOSTS"), ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			cfg.OAuthRedirectHosts = append(cfg.OAuthRedirectHosts, h)
		}
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	cfg.SecretKey = os.Getenv("HUB_SECRET_KEY")
	for _, k := range strings.Split(os.Getenv("HUB_SECRET_KEYS_OLD"), ",") {
		if k = strings.TrimSpace(k); k != "" {
			cfg.OldSecretKeys = append(cfg.OldSecretKeys, k)
		}
	}
	cfg.AppsDBAdminURL = envOr("APPS_DB_ADMIN_URL", cfg.DatabaseURL)
	cfg.AppsDBHost = envOr("APPS_DB_HOST_FOR_CONTAINERS", "host.containers.internal:5433")
	cfg.AppsDBContainer = envOr("APPS_DB_CONTAINER", "ziv-db")
	cfg.AppsDBContainerExplicit = os.Getenv("APPS_DB_CONTAINER") != ""
	cfg.SharedDomain = envOr("SHARED_APPS_DOMAIN", "localhost:8080")
	home, _ := os.UserHomeDir()
	cfg.SharedDataDir = envOr("SHARED_DATA_DIR", filepath.Join(home, ".local", "share", "ziv", "shared"))
	cfg.SharedDataDirExplicit = os.Getenv("SHARED_DATA_DIR") != ""
	cfg.ContainerEngine = envOr("CONTAINER_ENGINE", "podman")
	cfg.SharedAppsPerCompany = envInt("SHARED_APPS_PER_COMPANY", 5)
	cfg.SharedAppMaxMemoryMB = envInt("SHARED_APP_MAX_MEMORY_MB", 256)
	cfg.SharedAppsMaxRunning = envInt("SHARED_APPS_MAX_RUNNING", 2)
	cfg.SharedAppsMemoryBudgetMB = envInt("SHARED_APPS_MEMORY_BUDGET_MB", 512)
	cfg.CompanyLimit = envInt("COMPANY_LIMIT", 20)
	cfg.CompanySignupsPerDay = envInt("COMPANY_SIGNUPS_PER_DAY", 10)
	cfg.ReservedSlugs = map[string]bool{}
	for _, s := range append(builtinReservedSlugs, strings.Split(os.Getenv("RESERVED_COMPANY_SLUGS"), ",")...) {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			cfg.ReservedSlugs[s] = true
		}
	}
	return cfg, nil
}

// ListenAddr is the address the HTTP server listens on.
func (c Config) ListenAddr() string { return net.JoinHostPort(c.ListenHost, c.Port) }

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
		"AgentBinaryPath": c.AgentBinaryPath, "SecretKey": hide(c.SecretKey), "OldSecretKeys": len(c.OldSecretKeys),
		"ResendAPIKey": hide(c.ResendAPIKey), "MailFrom": c.MailFrom,
		"AppsDBAdminURL": hideURLPassword(c.AppsDBAdminURL), "AppsDBHost": c.AppsDBHost,
		"AppsDBContainer": c.AppsDBContainer, "SharedDomain": c.SharedDomain,
		"SharedDataDir": c.SharedDataDir, "ContainerEngine": c.ContainerEngine,
		"SharedAppsPerCompany": c.SharedAppsPerCompany, "SharedAppMaxMemoryMB": c.SharedAppMaxMemoryMB,
		"SharedAppsMaxRunning": c.SharedAppsMaxRunning, "SharedAppsMemoryBudgetMB": c.SharedAppsMemoryBudgetMB,
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
