package readiness

import (
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func check(t *testing.T, dir string, m manifest.Manifest, secrets []scanner.SecretFinding, acks ...string) Report {
	t.Helper()
	if m.Shared == nil {
		m.Shared = &manifest.Shared{Enabled: true}
	}
	m.Kind = manifest.KindWeb
	r, err := Check(versions.LocalFolder{Root: "../../../testdata/shared/" + dir}, m, secrets, acks)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func item(r Report, key string) Item {
	for _, it := range r.Items {
		if it.Key == key {
			return it
		}
	}
	return Item{}
}

func TestStreamlitIsReady(t *testing.T) {
	r := check(t, "streamlit", manifest.Manifest{Entrypoint: "app.py"}, nil)
	if !r.Ready || item(r, "recipe").Status != "auto" || item(r, "port").Status != "auto" {
		t.Fatalf("%+v", r)
	}
}

func TestUnknownNeedsDockerfile(t *testing.T) {
	r := check(t, "script", manifest.Manifest{}, nil)
	it := item(r, "recipe")
	if r.Ready || it.Status != "missing" || !strings.Contains(it.ClaudeText, "Dockerfile") {
		t.Fatalf("%+v", it)
	}
}

func TestDockerfileWithoutPortAsksPort(t *testing.T) {
	// a pasta "dockerfile" tem EXPOSE; aqui o manifesto não diz a porta e o Dockerfile vem sem EXPOSE
	r := check(t, "dockerfile-noexpose", manifest.Manifest{}, nil)
	if r.Ready || item(r, "port").Status != "missing" {
		t.Fatalf("%+v", item(r, "port"))
	}
}

func TestSQLiteWarnsUntilAcked(t *testing.T) {
	r := check(t, "sqlite", manifest.Manifest{Entrypoint: "app.py"}, nil)
	it := item(r, "file_data")
	if r.Ready || it.Status != "warn" || !strings.Contains(it.ClaudeText, "DATABASE_URL") {
		t.Fatalf("%+v", it)
	}
	r = check(t, "sqlite", manifest.Manifest{Entrypoint: "app.py"}, nil, "file_data")
	if !r.Ready || !item(r, "file_data").Acked {
		t.Fatalf("acked: %+v", r)
	}
}

func TestWindowsPaths(t *testing.T) {
	it := item(check(t, "windows-path", manifest.Manifest{Entrypoint: "app.py"}, nil), "windows_paths")
	if it.Status != "warn" || !strings.Contains(it.ClaudeText, "APPHUB_DATA_DIR") || !strings.Contains(it.Detail, "app.py") {
		t.Fatalf("%+v", it)
	}
}

func TestSettingsReadInCode(t *testing.T) {
	r := check(t, "env-reads", manifest.Manifest{Entrypoint: "app.py"}, nil)
	if !r.WantsDB || item(r, "database").Status != "auto" {
		t.Fatalf("DATABASE_URL should mean a database: %+v", r)
	}
	var names []string
	for _, e := range r.SuggestEnv {
		names = append(names, e.Name)
	}
	if strings.Join(names, ",") != "ADMIN_EMAILS,SMTP_PASSWORD" {
		t.Fatalf("suggest %v", names)
	}
	if !r.SuggestEnv[1].Secret {
		t.Fatal("SMTP_PASSWORD should be a secret")
	}
	// já declaradas no manifesto: não sugere de novo
	m := manifest.Manifest{Entrypoint: "app.py", Shared: &manifest.Shared{Enabled: true,
		Env: []manifest.EnvVar{{Name: "ADMIN_EMAILS", Label: "x"}, {Name: "SMTP_PASSWORD", Label: "y", Secret: true}}}}
	if r := check(t, "env-reads", m, nil); len(r.SuggestEnv) != 0 || item(r, "settings").Status != "ok" {
		t.Fatalf("%+v", r.SuggestEnv)
	}
}

func TestSecretInCode(t *testing.T) {
	r := check(t, "streamlit", manifest.Manifest{Entrypoint: "app.py"}, []scanner.SecretFinding{{Name: "API_KEY", File: "app.py", Line: 3}})
	it := item(r, "secrets")
	if it.Status != "warn" || !strings.Contains(it.ClaudeText, "API_KEY") {
		t.Fatalf("%+v", it)
	}
}

func TestPlaywrightOutsideRecipe(t *testing.T) {
	it := item(check(t, "playwright", manifest.Manifest{Entrypoint: "app.py"}, nil), "container_limits")
	if it.Status != "warn" || !strings.Contains(it.Detail, "gerar_pdf.js:1") || !strings.Contains(it.ClaudeText, "Playwright") {
		t.Fatalf("%+v", it)
	}
}

func TestEscapesAreNotWindowsPaths(t *testing.T) {
	if it := item(check(t, "escapes", manifest.Manifest{Entrypoint: "app.py"}, nil), "windows_paths"); it.Key != "" {
		t.Fatalf("false positive: %+v", it)
	}
	if it := item(check(t, "windows-path", manifest.Manifest{Entrypoint: "app.py"}, nil), "windows_paths"); it.Status != "warn" {
		t.Fatalf("%+v", it)
	}
}

func TestNeedsConfirmIsMissing(t *testing.T) {
	r := check(t, "flask", manifest.Manifest{Entrypoint: "app.py"}, nil)
	it := item(r, "recipe")
	if r.Ready || it.Status != "missing" || !strings.Contains(it.Detail, "gunicorn") {
		t.Fatalf("%+v", it)
	}
}

func TestDockerfileAppWithBrowserDoesNotWarn(t *testing.T) {
	r := check(t, "dockerfile-browser", manifest.Manifest{}, nil)
	if it := item(r, "container_limits"); it.Key != "" || !r.Ready {
		t.Fatalf("%+v", r)
	}
}

func TestWhereCutsOff(t *testing.T) {
	it := item(check(t, "winpath-many", manifest.Manifest{Entrypoint: "app.py"}, nil), "windows_paths")
	if !strings.Contains(it.Detail, "app.py:2, app.py:3, app.py:4, e mais 2") {
		t.Fatalf("%+v", it)
	}
}

func TestUnknownFrameworkHasNoPortItem(t *testing.T) {
	if it := item(check(t, "script", manifest.Manifest{}, nil), "port"); it.Key != "" {
		t.Fatalf("%+v", it)
	}
}

func TestWinPathRegexForms(t *testing.T) {
	for _, s := range []string{`open("C:\\Users\\henry\\x.xlsx")`, `r"C:\Users\henry"`, `"D:\Temp\a"`, `/mnt/c/Users`, `OneDrive`} {
		if !winPathRe.MatchString(s) {
			t.Errorf("should match %s", s)
		}
	}
	for _, s := range []string{`"Total:\n"`, `"Nome:\t"`, `"e:\n"`, `"a:\r"`} {
		if winPathRe.MatchString(s) {
			t.Errorf("should not match %s", s)
		}
	}
}
