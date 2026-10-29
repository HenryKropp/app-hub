package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

var prepOutputs = []manifest.Output{
	{Label: "Documentos", Pattern: "*/*/**/*.pdf"},
	{Label: "Data Pack", Pattern: "*/MD_Output/*_DataPack_Consolidado.md"},
	{Label: "FRE cortado", Pattern: "*_secoes.pdf"},
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func filesPrepFolder(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "Código-fonte/Files Prep.py", "print('oi')\n")
	write(t, root, "Código-fonte/requirements.txt", "pypdf\n")
	write(t, root, "Ferramenta CVM.exe", "MZ\x00\x01binary")
	write(t, root, "app-hub.yaml", "id: x\n")
	write(t, root, "FICHA.md", "# x\n")
	write(t, root, "PETROBRAS/5. FRE/2025/FRE.pdf", "%PDF")
	write(t, root, "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", "# pack\n")
	write(t, root, "FRE_secoes.pdf", "%PDF")
	write(t, root, "Old/__pycache__/x.pyc", "\x00")
	write(t, root, ".app-hub-lixeira/2026-10-01 120000/velho.py", "x\n")
	return root
}

func paths(s Snapshot) []string {
	out := []string{}
	for _, f := range s {
		out = append(out, f.Path+":"+f.Kind)
	}
	return out
}

func TestScanRules(t *testing.T) {
	root := filesPrepFolder(t)
	snap, contents, err := Scan(LocalFolder{Root: root}, prepOutputs, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := paths(snap)
	want := []string{"Código-fonte/Files Prep.py:code", "Código-fonte/requirements.txt:code", "Ferramenta CVM.exe:binary"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if len(contents) != 2 {
		t.Fatalf("contents %d", len(contents))
	}
	if string(contents[snap[0].SHA256]) != "print('oi')\n" {
		t.Fatalf("content %q", contents[snap[0].SHA256])
	}
	if snap[2].SHA256 == "" || snap[2].Size != int64(len("MZ\x00\x01binary")) {
		t.Fatalf("binary fingerprint %+v", snap[2])
	}
}

func TestScanCodeTooBigOrWithNulIsBinary(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, MaxCodeSize+1)
	for i := range big {
		big[i] = 'a'
	}
	write(t, root, "big.py", string(big))
	write(t, root, "nul.py", "a\x00b")
	snap, contents, err := Scan(LocalFolder{Root: root}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 2 || snap[0].Kind != KindBinary || snap[1].Kind != KindBinary || len(contents) != 0 {
		t.Fatalf("%+v %d", snap, len(contents))
	}
	if snap[0].Size != MaxCodeSize+1 {
		t.Fatalf("size %d", snap[0].Size)
	}
}

func TestScanTrustsUnchangedFiles(t *testing.T) {
	root := filesPrepFolder(t)
	first, _, _ := Scan(LocalFolder{Root: root}, prepOutputs, nil)
	// A previous snapshot with a fake hash but the same size and time is trusted (no read).
	prev := append(Snapshot{}, first...)
	prev[0].SHA256 = "trusted"
	again, contents, err := Scan(LocalFolder{Root: root}, prepOutputs, prev)
	if err != nil {
		t.Fatal(err)
	}
	if again[0].SHA256 != "trusted" || len(contents) != 0 {
		t.Fatalf("%+v %d", again[0], len(contents))
	}
}

func TestScanTouchedFileSameContent(t *testing.T) {
	root := filesPrepFolder(t)
	first, _, _ := Scan(LocalFolder{Root: root}, prepOutputs, nil)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "Código-fonte", "Files Prep.py"), later, later); err != nil {
		t.Fatal(err)
	}
	again, _, err := Scan(LocalFolder{Root: root}, prepOutputs, first)
	if err != nil {
		t.Fatal(err)
	}
	if !Same(first, again) {
		t.Fatal("touch without change must be the same version")
	}
}

func TestScanFailsInsteadOfDroppingFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	root := filesPrepFolder(t)
	locked := filepath.Join(root, "Código-fonte")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	if _, _, err := Scan(LocalFolder{Root: root}, prepOutputs, nil); err == nil {
		t.Fatal("an unreadable folder must fail the scan, not look like removed files")
	}
	if _, _, err := Scan(LocalFolder{Root: filepath.Join(root, "nao-existe")}, nil, nil); err == nil {
		t.Fatal("missing folder must fail")
	}
}

func TestChangesAndSummary(t *testing.T) {
	old := Snapshot{
		{Path: "a.py", SHA256: "1", Kind: KindCode},
		{Path: "app.exe", SHA256: "x", Kind: KindBinary, Size: 10},
		{Path: "b.py", SHA256: "2", Kind: KindCode},
	}
	now := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	cur := Snapshot{
		{Path: "a.py", SHA256: "1b", Kind: KindCode, Modified: now},
		{Path: "app.exe", SHA256: "y", Kind: KindBinary, Size: 12, Modified: now.Add(-time.Hour)},
		{Path: "c.py", SHA256: "3", Kind: KindCode, Modified: now.Add(-2 * time.Hour)},
	}
	ch := Changes(old, cur)
	if len(ch) != 4 || ch[0].Path != "a.py" || ch[0].Status != "modified" || ch[2].Path != "b.py" || ch[2].Status != "removed" || ch[3].Status != "added" {
		t.Fatalf("%+v", ch)
	}
	s := Summarize(ch)
	if s.Added != 1 || s.Removed != 1 || s.Modified != 1 || len(s.Binaries) != 1 || s.Binaries[0] != "app.exe" {
		t.Fatalf("%+v", s)
	}
	if at, ok := LastModified(ch); !ok || !at.Equal(now) {
		t.Fatalf("%v %v", at, ok)
	}
	if Same(old, cur) || !Same(cur, append(Snapshot{}, cur...)) {
		t.Fatal("Same")
	}
}

func TestIncludedSkipsLeftoverTempFile(t *testing.T) {
	for _, rel := range []string{"a.py.app-hub-tmp", "src/b.py.app-hub-tmp"} {
		if Included(rel, nil) {
			t.Fatalf("%s must not enter a version", rel)
		}
	}
	if !Included("a.py", nil) {
		t.Fatal("a.py must be included")
	}
}

// Files with passwords stay out of the history; examples go in.
func TestScanLeavesSecretFilesOut(t *testing.T) {
	root := t.TempDir()
	write(t, root, "main.py", "print('oi')\n")
	write(t, root, ".env", "API_KEY=sk-segredo\n")
	write(t, root, "config/prod.env", "DB_PASSWORD=segredo\n")
	write(t, root, ".env.local", "X=segredo\n")
	write(t, root, ".env.example", "API_KEY=\n")
	snap, contents, err := Scan(LocalFolder{Root: root}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, want := paths(snap), []string{".env.example:code", "main.py:code"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, data := range contents {
		if strings.Contains(string(data), "segredo") {
			t.Fatalf("secret stored: %q", data)
		}
	}
}

func TestSecretFile(t *testing.T) {
	cases := []struct {
		rel  string
		want bool
	}{
		{".env", true},
		{"sub/.ENV", true},
		{"prod.env", true},
		{".env.local", true},
		{".env.example", false},
		{".env.sample", false},
		{"env.py", false},
		{"environment.yml", false},
	}
	for _, c := range cases {
		if got := SecretFile(c.rel); got != c.want {
			t.Errorf("SecretFile(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}

func TestIncludedSkipsAllManifestNames(t *testing.T) {
	for _, rel := range []string{"ziv.yaml", "kova.yaml", "app-hub.yaml", "FICHA.md"} {
		if Included(rel, nil) {
			t.Errorf("%s is written by Ziv and must not enter a version as code", rel)
		}
	}
}

func TestIncludedSkipsAllTrashAndTempNames(t *testing.T) {
	for _, rel := range []string{".app-hub-lixeira/x.py", ".kova-lixeira/x.py", ".ziv-lixeira/x.py", "a.py.app-hub-tmp", "a.py.kova-tmp", "a.py.ziv-tmp"} {
		if Included(rel, nil) {
			t.Errorf("%s must not enter a version", rel)
		}
	}
}
