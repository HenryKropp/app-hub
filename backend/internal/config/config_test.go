package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestConfigHidesSecrets(t *testing.T) {
	c := Config{Port: "9", SecretKey: "topsecretkey", AppsDBAdminURL: "postgres://admin:dbpass123@db:5432/x",
		DatabaseURL: "postgres://hub:hubpass456@db:5432/hub"}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("cfg", "c", c)
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("cfg", "c", c)
	outs := []string{fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s", c, c, c, c), fmt.Sprintf("%v %+v", &c, &c), buf.String()}
	for _, out := range outs {
		for _, secret := range []string{"topsecretkey", "dbpass123", "hubpass456"} {
			if strings.Contains(out, secret) {
				t.Fatalf("secret %q shown in %q", secret, out)
			}
		}
	}
	if !strings.Contains(fmt.Sprint(c), "9") {
		t.Fatalf("non-secret fields should still show: %q", fmt.Sprint(c))
	}
}
