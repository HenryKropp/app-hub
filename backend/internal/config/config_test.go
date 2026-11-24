package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestConfigHidesSecrets(t *testing.T) {
	c := Config{Port: "9", SecretKey: "topsecretkey", ResendAPIKey: "re_supersecret", AppsDBAdminURL: "postgres://admin:dbpass123@db:5432/x",
		DatabaseURL: "postgres://hub:hubpass456@db:5432/hub"}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("cfg", "c", c)
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("cfg", "c", c)
	outs := []string{fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s", c, c, c, c), fmt.Sprintf("%v %+v", &c, &c), buf.String()}
	for _, out := range outs {
		for _, secret := range []string{"topsecretkey", "dbpass123", "hubpass456", "re_supersecret"} {
			if strings.Contains(out, secret) {
				t.Fatalf("secret %q shown in %q", secret, out)
			}
		}
	}
	if !strings.Contains(fmt.Sprint(c), "9") {
		t.Fatalf("non-secret fields should still show: %q", fmt.Sprint(c))
	}
}

func TestDefaultsUseZivNamesAndAreNotExplicit(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("APPS_DB_CONTAINER", "")
	t.Setenv("SHARED_DATA_DIR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppsDBContainer != "ziv-db" || cfg.AppsDBContainerExplicit {
		t.Errorf("container %q explicit=%v", cfg.AppsDBContainer, cfg.AppsDBContainerExplicit)
	}
	if !strings.HasSuffix(cfg.SharedDataDir, "/.local/share/ziv/shared") || cfg.SharedDataDirExplicit {
		t.Errorf("data dir %q explicit=%v", cfg.SharedDataDir, cfg.SharedDataDirExplicit)
	}
	t.Setenv("APPS_DB_CONTAINER", "meu-db")
	if cfg, _ := Load(); cfg.AppsDBContainer != "meu-db" || !cfg.AppsDBContainerExplicit {
		t.Errorf("explicit: %+v", cfg)
	}
}

func TestLinkIsReservedSlug(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ReservedSlugs["link"] {
		t.Fatal(`"link" must be a reserved company address`)
	}
}

// The server answers only on this machine unless LISTEN_HOST opens it, so
// app boxes cannot reach Ziv through host.containers.internal (Fase 0).
func TestListenHostDefaultsToLoopback(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("LISTEN_HOST", "")
	if cfg, _ := Load(); cfg.ListenHost != "127.0.0.1" || cfg.ListenAddr() != "127.0.0.1:8080" {
		t.Fatalf("default listen %q %q", cfg.ListenHost, cfg.ListenAddr())
	}
	t.Setenv("LISTEN_HOST", "0.0.0.0")
	if cfg, _ := Load(); cfg.ListenAddr() != "0.0.0.0:8080" {
		t.Fatalf("explicit listen %q", cfg.ListenAddr())
	}
}

func TestOAuthRedirectHosts(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("OAUTH_REDIRECT_HOSTS", " ok.example , outro.example ,")
	cfg, _ := Load()
	if len(cfg.OAuthRedirectHosts) != 2 || cfg.OAuthRedirectHosts[0] != "ok.example" || cfg.OAuthRedirectHosts[1] != "outro.example" {
		t.Fatalf("hosts %q", cfg.OAuthRedirectHosts)
	}
}
