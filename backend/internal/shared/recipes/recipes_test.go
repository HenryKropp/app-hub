package recipes

import (
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func detect(t *testing.T, dir string, m manifest.Manifest) Recipe {
	t.Helper()
	if m.Shared == nil {
		m.Shared = &manifest.Shared{Enabled: true}
	}
	r, err := Detect(versions.LocalFolder{Root: "../../../testdata/shared/" + dir}, m)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStreamlit(t *testing.T) {
	r := detect(t, "streamlit", manifest.Manifest{Entrypoint: "app.py"})
	if r.Kind != "streamlit" || r.Port != 8501 || r.NeedsConfirm {
		t.Fatalf("%+v", r)
	}
	for _, want := range []string{"FROM docker.io/library/python:3.12-slim", "pip install --no-cache-dir -r requirements.txt",
		"streamlit run app.py", "--server.address 0.0.0.0", "--server.headless true"} {
		if !strings.Contains(r.Dockerfile, want) {
			t.Errorf("missing %q in\n%s", want, r.Dockerfile)
		}
	}
}

func TestStreamlitEntrypointFoundWithoutManifest(t *testing.T) {
	r := detect(t, "streamlit", manifest.Manifest{})
	if !strings.Contains(r.Dockerfile, "streamlit run app.py") {
		t.Fatal(r.Dockerfile)
	}
}

func TestStreamlitWithoutRequirementsGeneratesList(t *testing.T) {
	r := detect(t, "streamlit-noreq", manifest.Manifest{Entrypoint: "app.py"})
	req := r.ExtraFiles["requirements.txt"]
	if !strings.Contains(req, "streamlit") || !strings.Contains(req, "pandas") || len(r.Requirements) != 2 {
		t.Fatalf("%+v", r)
	}
}

func TestPythonVersionFromManifest(t *testing.T) {
	r := detect(t, "streamlit", manifest.Manifest{Entrypoint: "app.py", Shared: &manifest.Shared{Enabled: true, Python: "3.11"}})
	if !strings.Contains(r.Dockerfile, "python:3.11-slim") {
		t.Fatal(r.Dockerfile)
	}
}

func TestFlaskAndFastAPI(t *testing.T) {
	f := detect(t, "flask", manifest.Manifest{Entrypoint: "app.py"})
	if f.Kind != "flask" || !f.NeedsConfirm || !strings.Contains(f.Command, "gunicorn -b 0.0.0.0:$PORT app:app") {
		t.Fatalf("%+v", f)
	}
	a := detect(t, "fastapi", manifest.Manifest{Entrypoint: "main.py"})
	if a.Kind != "fastapi" || !strings.Contains(a.Command, "uvicorn main:app --host 0.0.0.0 --port $PORT") {
		t.Fatalf("%+v", a)
	}
}

func TestConfirmedCommandWins(t *testing.T) {
	r := detect(t, "flask", manifest.Manifest{Entrypoint: "app.py", Shared: &manifest.Shared{Enabled: true, Command: "gunicorn -b 0.0.0.0:$PORT wsgi:app"}})
	if r.NeedsConfirm || !strings.Contains(r.Dockerfile, "wsgi:app") {
		t.Fatalf("%+v", r)
	}
}

func TestNode(t *testing.T) {
	r := detect(t, "node", manifest.Manifest{})
	if r.Kind != "node" || r.Port != 3000 {
		t.Fatalf("%+v", r)
	}
	for _, want := range []string{"FROM docker.io/library/node:22-slim", "npm ci", "npm run build --if-present", `CMD ["npm", "start"]`} {
		if !strings.Contains(r.Dockerfile, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestOwnDockerfile(t *testing.T) {
	r := detect(t, "dockerfile", manifest.Manifest{})
	if r.Kind != "dockerfile" || r.Port != 9000 || r.Dockerfile != "" {
		t.Fatalf("%+v", r)
	}
	r = detect(t, "dockerfile", manifest.Manifest{Shared: &manifest.Shared{Enabled: true, Port: 9100}})
	if r.Port != 9100 {
		t.Fatalf("manifest port should win: %+v", r)
	}
}

func TestUnknown(t *testing.T) {
	r := detect(t, "script", manifest.Manifest{})
	if r.Kind != "" {
		t.Fatalf("%+v", r)
	}
}

func TestGeneratedRequirementsSkipLocalAndStdlib(t *testing.T) {
	r := detect(t, "pyimports", manifest.Manifest{Entrypoint: "app.py"})
	if len(r.Requirements) != 2 || r.Requirements[0] != "numpy" || r.Requirements[1] != "streamlit" {
		t.Fatalf("%+v", r.Requirements)
	}
}

func TestSkippedFoldersAndDepthCutoffAreIgnored(t *testing.T) {
	// .venv, venv, node_modules and a/b/c all hold "import streamlit" (wins over flask):
	// reading any of them would change the kind from flask.
	r := detect(t, "skipdirs", manifest.Manifest{Entrypoint: "app.py"})
	if r.Kind != "flask" {
		t.Fatalf("%+v", r)
	}
}

func TestEntrypointInSubfolder(t *testing.T) {
	r := detect(t, "srcapp", manifest.Manifest{})
	if r.Kind != "streamlit" || !strings.Contains(r.Command, "streamlit run src/app.py") {
		t.Fatalf("%+v", r)
	}
}

func TestDjango(t *testing.T) {
	r := detect(t, "django", manifest.Manifest{})
	if r.Kind != "django" || !r.NeedsConfirm || r.Port != 8000 || !strings.Contains(r.Command, "gunicorn -b 0.0.0.0:$PORT mysite.wsgi") {
		t.Fatalf("%+v", r)
	}
}

func TestCommandWithNewlineRefused(t *testing.T) {
	m := manifest.Manifest{Entrypoint: "app.py", Shared: &manifest.Shared{Enabled: true, Command: "gunicorn app:app\nRUN rm -rf /"}}
	if _, err := Detect(versions.LocalFolder{Root: "../../../testdata/shared/flask"}, m); err == nil || err.Error() != "O comando de início precisa estar numa linha só." {
		t.Fatalf("%v", err)
	}
}

// The start command runs through a shell (so $PORT expands), but with exec:
// the app itself is PID 1 and gets the stop signal, instead of a shell that
// ignores it until Podman kills everything 10 s later.
func TestStartCommandRunsWithExec(t *testing.T) {
	cases := []struct {
		dir, entry, command, want string
	}{
		{"streamlit", "app.py", "", "\nCMD exec streamlit run app.py --server.port $PORT"},
		{"flask", "app.py", "", "\nCMD exec gunicorn -b 0.0.0.0:$PORT app:app\n"},
		{"fastapi", "main.py", "", "\nCMD exec uvicorn main:app --host 0.0.0.0 --port $PORT\n"},
		{"flask", "app.py", "gunicorn -b 0.0.0.0:$PORT wsgi:app", "\nCMD exec gunicorn -b 0.0.0.0:$PORT wsgi:app\n"},
		// already exec: not doubled (exec exec would look for a program called exec)
		{"flask", "app.py", "exec gunicorn -b 0.0.0.0:$PORT wsgi:app", "\nCMD exec gunicorn -b 0.0.0.0:$PORT wsgi:app\n"},
		// several commands or a leading VAR=x: exec in front would break them, so they stay as written
		{"flask", "app.py", "cd src && gunicorn -b 0.0.0.0:$PORT app:app", "\nCMD cd src && gunicorn -b 0.0.0.0:$PORT app:app\n"},
		{"flask", "app.py", "WORKERS=2 gunicorn -b 0.0.0.0:$PORT app:app", "\nCMD WORKERS=2 gunicorn -b 0.0.0.0:$PORT app:app\n"},
	}
	for _, c := range cases {
		m := manifest.Manifest{Entrypoint: c.entry, Shared: &manifest.Shared{Enabled: true, Command: c.command}}
		r := detect(t, c.dir, m)
		if !strings.Contains(r.Dockerfile, c.want) {
			t.Errorf("%s %q: want %q in\n%s", c.dir, c.command, c.want, r.Dockerfile)
		}
	}
}
