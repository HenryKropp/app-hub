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
	if len(r.Requirements) != 1 || r.Requirements[0] != "streamlit" {
		t.Fatalf("%+v", r.Requirements)
	}
}

func TestSkippedFoldersAndDepthCutoffAreIgnored(t *testing.T) {
	// .venv has "import streamlit", node_modules and a/b/c have "import django":
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
