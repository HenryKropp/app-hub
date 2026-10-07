package stage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopySkipsJunkAndOutputs(t *testing.T) {
	src := t.TempDir()
	write(t, src, "app.py", "import streamlit")
	write(t, src, ".git/config", "x")
	write(t, src, ".venv/lib/a.py", "x")
	write(t, src, "node_modules/a/index.js", "x")
	write(t, src, "saida/relatorio.xlsx", "x")
	write(t, src, "segredos/chave.txt", "x")
	write(t, src, ".dockerignore", "segredos/\n*.log\n")
	write(t, src, "debug.log", "x")
	dst := filepath.Join(t.TempDir(), "stage")
	rec := recipes.Recipe{Kind: "streamlit", Dockerfile: "FROM x\n", ExtraFiles: map[string]string{"requirements.txt": "streamlit\n"}}
	df, err := Copy(versions.LocalFolder{Root: src}, dst, []manifest.Output{{Pattern: "saida/*.xlsx"}}, rec)
	if err != nil {
		t.Fatal(err)
	}
	if df != recipes.DockerfileName {
		t.Fatalf("dockerfile %q", df)
	}
	for _, want := range []string{"app.py", "requirements.txt", recipes.DockerfileName} {
		if _, err := os.Stat(filepath.Join(dst, want)); err != nil {
			t.Errorf("missing %s", want)
		}
	}
	for _, gone := range []string{".git", ".venv", "node_modules", "saida/relatorio.xlsx", "segredos/chave.txt", "debug.log"} {
		if _, err := os.Stat(filepath.Join(dst, gone)); err == nil {
			t.Errorf("should skip %s", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(src, recipes.DockerfileName)); err == nil {
		t.Fatal("wrote into the app folder")
	}
}

func TestCopyOwnDockerfile(t *testing.T) {
	src := t.TempDir()
	write(t, src, "Dockerfile", "FROM y\n")
	dst := filepath.Join(t.TempDir(), "stage")
	df, err := Copy(versions.LocalFolder{Root: src}, dst, nil, recipes.Recipe{Kind: "dockerfile"})
	if err != nil || df != "Dockerfile" {
		t.Fatalf("%q %v", df, err)
	}
}
