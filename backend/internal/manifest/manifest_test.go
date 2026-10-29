package manifest

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestChoiceNeedsOptions(t *testing.T) {
	m := valid()
	m.Inputs = []Input{{ID: "docs", Label: "Quais documentos", Type: "choice", Source: "window", Required: true}}
	errs := m.Validate()
	if len(errs) == 0 || errs[0] != `Escreva ao menos uma opção para "Quais documentos".` {
		t.Fatalf("errs = %v", errs)
	}
	m.Inputs[0].Options = []string{"FRE"}
	m.Inputs[0].Multiple = true
	if errs := m.Validate(); len(errs) != 0 {
		t.Fatalf("valid choice: %v", errs)
	}
}

func TestSharedValidation(t *testing.T) {
	base := func() Manifest {
		return Manifest{ManifestVersion: 1, ID: "x", Name: "X", Kind: KindWeb, Entrypoint: "app.py", Owner: &PersonRef{Name: "Henry", Email: "h@a.com"}, Area: "Research",
			Shared: &Shared{Enabled: true}}
	}
	cases := []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{"ok", func(*Manifest) {}, ""},
		{"only web", func(m *Manifest) { m.Kind = KindScript }, "Só apps com tela no navegador podem ser compartilhados."},
		{"bad db", func(m *Manifest) { m.Shared.Database = "mysql" }, "O banco do app compartilhado precisa ser postgres."},
		{"bad port", func(m *Manifest) { m.Shared.Port = 70000 }, "A porta do app precisa estar entre 1 e 65535."},
		{"memory", func(m *Manifest) { m.Shared.MemoryMB = 64 }, "A memória do app precisa estar entre 128 e 16384 MB."},
		{"reserved env", func(m *Manifest) { m.Shared.Env = []EnvVar{{Name: "DATABASE_URL", Label: "x"}} }, "DATABASE_URL é preenchida pelo Kova; tire da lista de configurações."},
		{"bad env name", func(m *Manifest) { m.Shared.Env = []EnvVar{{Name: "minha var", Label: "x"}} }, "O nome da configuração \"minha var\" só pode ter letras maiúsculas, números e _."},
		{"underscore start", func(m *Manifest) { m.Shared.Env = []EnvVar{{Name: "_db_password", Label: "x"}} }, "só pode ter letras maiúsculas"},
		{"dup env", func(m *Manifest) { m.Shared.Env = []EnvVar{{Name: "A", Label: "a"}, {Name: "A", Label: "b"}} }, "A configuração A aparece duas vezes."},
		{"health", func(m *Manifest) { m.Shared.HealthPath = "health" }, "O endereço de saúde precisa começar com /."},
		{"command newline", func(m *Manifest) { m.Shared.Command = "gunicorn app:app\nRUN curl evil" }, "O comando de início precisa estar numa linha só."},
		{"command cr", func(m *Manifest) { m.Shared.Command = "gunicorn app:app\rx" }, "O comando de início precisa estar numa linha só."},
	}
	for _, c := range cases {
		m := base()
		c.edit(&m)
		got := strings.Join(m.Validate(), " | ")
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestSharedDefaults(t *testing.T) {
	d := (&Shared{Enabled: true}).WithDefaults()
	if d.HealthPath != "/" || d.IdleMin != 30 || d.MemoryMB != 512 || d.CPUs != 1 || d.Python != "3.12" || d.Node != "22" {
		t.Fatalf("%+v", d)
	}
}

func TestSharedRoundTripYAML(t *testing.T) {
	m := Manifest{ManifestVersion: 1, ID: "x", Name: "X", Kind: KindWeb, Shared: &Shared{Enabled: true, Database: "postgres",
		Env: []EnvVar{{Name: "SMTP_PASSWORD", Label: "Senha do e-mail", Secret: true}}}}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "secret: true") || strings.Contains(string(out), "value") {
		t.Fatalf("yaml:\n%s", out)
	}
}

func TestDiffShared(t *testing.T) {
	a := valid()
	b := valid()
	b.Shared = &Shared{Enabled: true}
	for _, c := range Diff(a, b) {
		if c.Field == "shared" && c.Long {
			return
		}
	}
	t.Fatal("no shared change")
}

// legacyManifest is an app-hub.yaml as the old App Hub wrote it.
func legacyManifest(t *testing.T, dir string) {
	t.Helper()
	data, err := Marshal(valid())
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(headerMarker), []byte(legacyHeaderMarker), 1)
	if err := os.WriteFile(filepath.Join(dir, LegacyFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FichaName), []byte(legacyFichaMarker+"\n# ficha antiga\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteToFolder_LegacyOnly_KeepsLegacy(t *testing.T) {
	dir := t.TempDir()
	legacyManifest(t, dir)
	m := valid()
	m.Name = "Nome novo"
	if err := WriteToFolder(dir, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, LegacyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if back, _ := Parse(data); back.Name != "Nome novo" {
		t.Errorf("legacy file not updated: %s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Error("kova.yaml must not be created next to an existing app-hub.yaml")
	}
}

func TestWriteToFolder_NewApp_WritesKova(t *testing.T) {
	dir := t.TempDir()
	if err := WriteToFolder(dir, valid()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "kova.yaml"))
	if err != nil || !strings.HasPrefix(string(data), "# Kova — manifesto do app\n") {
		t.Fatalf("kova.yaml = %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app-hub.yaml")); !os.IsNotExist(err) {
		t.Error("new apps get no app-hub.yaml")
	}
	if ficha, _ := os.ReadFile(filepath.Join(dir, FichaName)); !strings.HasPrefix(string(ficha), "<!-- kova:ficha -->") {
		t.Errorf("ficha = %q", ficha)
	}
}

func TestLocate_BothPresent_PrefersKova(t *testing.T) {
	dir := t.TempDir()
	if got := Locate(dir); got != filepath.Join(dir, "kova.yaml") {
		t.Errorf("empty dir: %s", got)
	}
	os.WriteFile(filepath.Join(dir, "app-hub.yaml"), []byte("x"), 0o644)
	if got := Locate(dir); got != filepath.Join(dir, "app-hub.yaml") {
		t.Errorf("legacy only: %s", got)
	}
	os.WriteFile(filepath.Join(dir, "kova.yaml"), []byte("x"), 0o644)
	if got := Locate(dir); got != filepath.Join(dir, "kova.yaml") {
		t.Errorf("both: %s", got)
	}
}

func TestWriteToFolder_ForeignKovaYaml_Refuses(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kova.yaml"), []byte("name: outro projeto\n"), 0o644)
	if err := WriteToFolder(dir, valid()); !errors.Is(err, ErrForeignFile) {
		t.Fatalf("want ErrForeignFile, got %v", err)
	}
}

func TestIsManifestName(t *testing.T) {
	for name, want := range map[string]bool{"kova.yaml": true, "app-hub.yaml": true, "KOVA.YAML": true, "App-Hub.yaml": true, "x.yaml": false, "FICHA.md": false} {
		if IsManifestName(name) != want {
			t.Errorf("%s: want %v", name, want)
		}
	}
}
