package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func valid() Manifest {
	return Manifest{
		Name: "Rentabilidade", Kind: KindScript, Entrypoint: "r.py",
		Owner:   &PersonRef{Name: "Henry", Email: "h@alis"},
		Inputs:  []Input{{ID: "cnpjs", Label: "CNPJs", Type: "file", Source: "fixed_file", Path: "cnpjs.csv", Required: true}},
		Secrets: []SecretRef{{Name: "API_KEY", File: "config.json", Line: 2}},
	}
}

func TestValidate(t *testing.T) {
	if errs := valid().Validate(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	m := valid()
	m.Owner = nil
	m.Entrypoint = ""
	m.Kind = "x"
	if errs := m.Validate(); len(errs) != 2 {
		t.Errorf("errs = %v", errs)
	}
	m = valid()
	m.Backup = &PersonRef{Name: "Henry", Email: "h@alis"}
	if errs := m.Validate(); len(errs) != 1 {
		t.Errorf("same owner and backup should fail: %v", errs)
	}
	m = valid()
	m.Kind = KindReference
	m.Entrypoint = ""
	if errs := m.Validate(); len(errs) != 0 {
		t.Errorf("reference apps need no entrypoint: %v", errs)
	}
}

func TestSlugify(t *testing.T) {
	if got := Slugify("Análise FIAs (v2)"); got != "analise-fias-v2" {
		t.Errorf("got %q", got)
	}
	if got := Slugify("!!!"); got != "app" {
		t.Errorf("got %q", got)
	}
}

func TestWriteToFolderRoundTripAndSafety(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "r.py"), []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteToFolder(dir, valid()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(data)
	if err != nil || back.Name != "Rentabilidade" || back.ManifestVersion != Version || back.Inputs[0].Path != "cnpjs.csv" {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	ficha, _ := os.ReadFile(filepath.Join(dir, FichaName))
	if !strings.Contains(string(ficha), "# Rentabilidade") || !strings.Contains(string(ficha), "API_KEY") {
		t.Errorf("ficha = %s", ficha)
	}
	// Writing again replaces the Hub's own files.
	if err := WriteToFolder(dir, valid()); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// The app's code is untouched.
	if code, _ := os.ReadFile(filepath.Join(dir, "r.py")); string(code) != "print(1)" {
		t.Error("app code changed")
	}
	// A FICHA.md written by someone else is never overwritten.
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, FichaName), []byte("minha ficha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteToFolder(other, valid()); !errors.Is(err, ErrForeignFile) {
		t.Fatalf("want ErrForeignFile, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, FileName)); !os.IsNotExist(err) {
		t.Error("manifest must not be written when ficha is foreign")
	}
}

func TestProtocolV02Fields(t *testing.T) {
	m := valid()
	m.ID = "files-prep"
	m.Kind = KindDesktop
	m.Inputs = []Input{{ID: "docs", Label: "Documentos", Type: "choice", Source: "window", Options: []string{"FRE", "DFP"}, Multiple: true}}
	m.DependsOn = []string{"cvm-downloader"}
	m.Build = &Build{Source: "src/app.py", Command: "build.bat"}
	if errs := m.Validate(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(data)
	if err != nil || !back.Inputs[0].Multiple || back.DependsOn[0] != "cvm-downloader" || back.Build.Command != "build.bat" {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	ficha := RenderFicha(m)
	for _, want := range []string{"pode marcar várias", "campo na janela do app", "FRE, DFP", "cvm-downloader", "build.bat"} {
		if !strings.Contains(ficha, want) {
			t.Errorf("ficha missing %q:\n%s", want, ficha)
		}
	}

	bad := m
	bad.Inputs = []Input{{ID: "x", Label: "X", Type: "text", Source: "window", Multiple: true}}
	bad.DependsOn = []string{"files-prep"}
	bad.Build = &Build{Command: "build.bat"}
	if errs := bad.Validate(); len(errs) != 3 {
		t.Errorf("want 3 errors, got %v", errs)
	}
}
