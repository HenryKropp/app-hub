package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func writeApp(t *testing.T, root, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func scan(t *testing.T, dir string) Diagnosis {
	t.Helper()
	d, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func hasQuestion(d Diagnosis, field string) bool {
	for _, q := range d.Questions {
		if q.Field == field {
			return true
		}
	}
	return false
}

func TestScriptWithFixedInputAndTimestampedOutput(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "2026 04 02 Python Rentabilidade fundos", map[string]string{
		"rentabilidade.py": `"""Rentabilidade de fundos pela CVM"""
import pandas as pd
import requests
from datetime import datetime

def main():
    df = pd.read_csv("cnpjs.csv")
    out = f"rentabilidade_{datetime.now().strftime('%Y%m%d_%H%M%S')}.xlsx"
    df.to_excel(out)

if __name__ == "__main__":
    main()
`,
		"cnpjs.csv":                     "cnpj\n1\n",
		"rentabilidade_20260402_1.xlsx": "x",
	})
	d := scan(t, dir)
	m := d.Manifest
	if m.Name != "Rentabilidade fundos" {
		t.Errorf("name = %q", m.Name)
	}
	if m.Kind != manifest.KindScript || m.Language != "python" || m.Entrypoint != "rentabilidade.py" {
		t.Errorf("kind/lang/entry = %s/%s/%s", m.Kind, m.Language, m.Entrypoint)
	}
	if m.Run.Command != `python "rentabilidade.py"` {
		t.Errorf("command = %q", m.Run.Command)
	}
	if len(m.Inputs) != 1 || m.Inputs[0].Path != "cnpjs.csv" || m.Inputs[0].Source != "fixed_file" {
		t.Fatalf("inputs = %+v", m.Inputs)
	}
	if len(m.Outputs) != 1 || m.Outputs[0].Pattern != "rentabilidade_*.xlsx" {
		t.Errorf("outputs = %+v", m.Outputs)
	}
	if strings.Join(m.Runtime.Dependencies, ",") != "pandas,requests" {
		t.Errorf("deps = %v", m.Runtime.Dependencies)
	}
	if m.Summary != "Rentabilidade de fundos pela CVM" {
		t.Errorf("summary = %q", m.Summary)
	}
	if !hasQuestion(d, "inputs") || !hasQuestion(d, "area") {
		t.Errorf("questions = %+v", d.Questions)
	}
}

func TestScriptWithDialog(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "Extrair PDF", map[string]string{
		"extrair.py": `import tkinter as tk
from tkinter import filedialog
root = tk.Tk()
root.withdraw()
arq = filedialog.askopenfilename(title="Selecione o PDF")
`,
	})
	m := scan(t, dir).Manifest
	if m.Kind != manifest.KindScript {
		t.Fatalf("kind = %s", m.Kind)
	}
	if len(m.Inputs) != 1 || m.Inputs[0].Source != "dialog" || m.Inputs[0].Label != "Selecione o PDF" || m.Inputs[0].Type != "file" {
		t.Errorf("inputs = %+v", m.Inputs)
	}
	if len(m.Notes) == 0 || !strings.Contains(strings.Join(m.Notes, " "), "janela de seleção") {
		t.Errorf("notes = %v", m.Notes)
	}
}

func TestStreamlitWebApp(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "Sistema", map[string]string{
		"app.py":           "import streamlit as st\nst.title('x')\n",
		"common.py":        "import pandas\n",
		"requirements.txt": "streamlit==1.40\n# comentário\npandas\n",
		"README.md":        "# Sistema\n\nPainel **interno** do fundo.\n",
	})
	m := scan(t, dir).Manifest
	if m.Kind != manifest.KindWeb || m.Entrypoint != "app.py" {
		t.Fatalf("kind/entry = %s/%s", m.Kind, m.Entrypoint)
	}
	if m.Run.Command != `streamlit run "app.py"` || m.Run.URL != "http://localhost:8501" {
		t.Errorf("run = %+v", m.Run)
	}
	if m.Runtime.DependenciesFile != "requirements.txt" || strings.Join(m.Runtime.Dependencies, ",") != "streamlit==1.40,pandas" {
		t.Errorf("runtime = %+v", m.Runtime)
	}
	if m.Summary != "Painel interno do fundo." {
		t.Errorf("summary = %q", m.Summary)
	}
}

func TestDesktopAppStartedByBat(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "Analise", map[string]string{
		"Iniciar.bat":  "@echo off\r\n.venv\\Scripts\\pythonw.exe app.py\r\n",
		"app.py":       "import customtkinter as ctk\nfrom ui import janela\napp = ctk.CTk()\napp.mainloop()\n",
		"ui/janela.py": "import customtkinter\n",
		"tools.py":     "if __name__ == '__main__':\n    pass\n",
	})
	d := scan(t, dir)
	m := d.Manifest
	if m.Kind != manifest.KindDesktop || m.Entrypoint != "app.py" || m.Run.Command != `pythonw "app.py"` {
		t.Fatalf("manifest = %+v", m)
	}
	if hasQuestion(d, "entrypoint") {
		t.Errorf("launcher should make the entrypoint certain: %+v", d.Questions)
	}
	if strings.Join(m.Runtime.Dependencies, ",") != "customtkinter" {
		t.Errorf("deps should exclude local modules: %v", m.Runtime.Dependencies)
	}
}

func TestScheduledTask(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "News", map[string]string{
		"scraper.py":         "import feedparser\nif __name__ == '__main__':\n    pass\n",
		"agendar_tarefa.bat": `schtasks /Create /SC MINUTE /MO 30 /TN "News" /TR "python scraper.py"`,
	})
	m := scan(t, dir).Manifest
	if m.Kind != manifest.KindScheduled || m.Schedule != "A cada 30 minutos" {
		t.Errorf("kind/schedule = %s/%q", m.Kind, m.Schedule)
	}
}

func TestExecutableOnly(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "CVM", map[string]string{"dist/Ferramenta.exe": "MZ"})
	m := scan(t, dir).Manifest
	if m.Kind != manifest.KindExecutable || m.Entrypoint != "dist/Ferramenta.exe" {
		t.Errorf("kind/entry = %s/%s", m.Kind, m.Entrypoint)
	}
}

func TestDocumentsAndShortcut(t *testing.T) {
	root := t.TempDir()
	docs := scan(t, writeApp(t, root, "Projeto", map[string]string{"Guia.docx": "x"})).Manifest
	if docs.Kind != manifest.KindReference || !strings.Contains(docs.Summary, "Guia.docx") {
		t.Errorf("docs = %+v", docs)
	}
	link := scan(t, writeApp(t, root, "Atalho", map[string]string{"Abrir.url": "[InternetShortcut]\r\nURL=https://exemplo.com\r\n"})).Manifest
	if link.Kind != manifest.KindReference || link.Run.URL != "https://exemplo.com" {
		t.Errorf("link = %+v", link)
	}
}

func TestComplexSystem(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "Sistema-X", map[string]string{
		"backend/go.mod":        "module x",
		"backend/main.go":       "package main",
		"frontend/package.json": "{}",
		"Dockerfile":            "FROM alpine",
	})
	if k := scan(t, dir).Manifest.Kind; k != manifest.KindSystem {
		t.Errorf("kind = %s", k)
	}
}

func TestSecretsAreFoundButNeverStored(t *testing.T) {
	value := "sk-ant-" + strings.Repeat("a1B2", 10)
	dir := writeApp(t, t.TempDir(), "Research", map[string]string{
		"config.json": `{"api_key": "` + strings.Repeat("Zx9", 20) + `"}`,
		"app.py":      "import anthropic\nKEY = '" + value + "'\nsenha = os.environ.get('SENHA')\nempty_token = ''\n",
	})
	d := scan(t, dir)
	if len(d.Secrets) != 2 {
		t.Fatalf("secrets = %+v", d.Secrets)
	}
	manifestBytes, err := manifest.Marshal(d.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifestBytes), value) || strings.Contains(string(manifestBytes), "Zx9Zx9") {
		t.Fatal("secret value leaked into manifest")
	}
	if strings.Contains(strings.Join(d.Findings, " "), value) {
		t.Fatal("secret value leaked into findings")
	}
}

func TestAmbiguousEntrypointAsksQuestion(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "Duas", map[string]string{
		"a.py": "if __name__ == '__main__':\n    pass\n",
		"b.py": "if __name__ == '__main__':\n    pass\n",
	})
	d := scan(t, dir)
	if !hasQuestion(d, "entrypoint") || len(d.EntrypointOptions) != 2 {
		t.Errorf("questions=%+v options=%v", d.Questions, d.EntrypointOptions)
	}
}

func TestExistingManifestIsReused(t *testing.T) {
	dir := writeApp(t, t.TempDir(), "Pronto", map[string]string{"run.py": "print(1)\n"})
	m := manifest.Manifest{Name: "Nome escolhido", Kind: manifest.KindScript, Entrypoint: "run.py", Owner: &manifest.PersonRef{Name: "A", Email: "a@x"}}
	if err := manifest.WriteToFolder(dir, m); err != nil {
		t.Fatal(err)
	}
	d := scan(t, dir)
	if !d.HasManifest || d.Manifest.Name != "Nome escolhido" {
		t.Errorf("diagnosis = %+v", d)
	}
}

func TestScanBatchSkipsHiddenAndFlagsArchive(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "App Um", map[string]string{"main.py": "print(1)"})
	writeApp(t, root, "0. Old", map[string]string{"x.docx": "x"})
	writeApp(t, root, ".claude", map[string]string{"x.json": "{}"})
	writeApp(t, root, "App Dois", map[string]string{"node_modules/x.js": "", "index.js": ""})
	res, err := ScanBatch(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("got %d results", len(res))
	}
	for _, d := range res {
		if d.FolderName == "0. Old" && !d.SuggestSkip {
			t.Error("archive folder should be flagged")
		}
		if d.FolderName == "App Um" && d.SuggestSkip {
			t.Error("normal folder should not be flagged")
		}
		if d.FolderName == "App Dois" && d.FileCount != 1 {
			t.Errorf("node_modules should be skipped, files=%d", d.FileCount)
		}
	}
}

func TestLooksDuplicate(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Sistema-Alis", "Projeto Sistema Alis", true},
		{"Sistema Alis V2", "Sistema-Alis", true},
		{"Rentabilidade fundos", "Carteira detalhada dos fundos", false},
		{"Research Platform", "Research Shortcut", false},
	}
	for _, c := range cases {
		if got := LooksDuplicate(c.a, c.b); got != c.want {
			t.Errorf("LooksDuplicate(%q, %q) = %v", c.a, c.b, got)
		}
	}
}
