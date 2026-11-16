package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http/httptest"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func testPayload(source string, keys map[string]string) agentproto.TestPayload {
	return agentproto.TestPayload{DraftID: 7, AppID: 3, AppName: "Relatórios", TimeoutMin: 2, Keys: keys != nil, KeyValues: keys,
		Script: agentproto.ScriptPayload{Manifest: manifest.Manifest{Kind: "script", Entrypoint: "main.py"},
			Sources: map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte(source))}}}}
}

func paths(files []agentproto.FileEntry) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func TestRunTestModeEnv(t *testing.T) {
	needPython(t)
	src := `import os
open('env.txt','w').write(os.environ.get('ZIV_TESTE',''))
open(os.path.join(os.environ['ZIV_PASTA_TESTE'],'b.csv'),'w').write('b')
open('a.csv','w').write('a')
`
	folder := filepath.Join(t.TempDir(), "testes", "app-3", "rascunho-7")
	r := &Runner{StateDir: t.TempDir()}
	res := r.RunTest(context.Background(), testPayload(src, nil), folder)
	if res.Event != agentproto.EventFinished || res.ExitCode != 0 {
		t.Fatalf("%+v", res)
	}
	got := paths(res.Files)
	if !slices.Contains(got, "a.csv") || !slices.Contains(got, "b.csv") {
		t.Fatalf("files = %v", got)
	}
	env, _ := os.ReadFile(filepath.Join(folder, "env.txt"))
	if string(env) != "1" {
		t.Fatalf("ZIV_TESTE = %q", env)
	}
}

func TestRunTestMasksKeys(t *testing.T) {
	needPython(t)
	src := "import os\nprint(os.environ['SENHA'])\nopen('a.txt','w').write('x')\n"
	r := &Runner{StateDir: t.TempDir()}
	res := r.RunTest(context.Background(), testPayload(src, map[string]string{"SENHA": "segredo123"}), filepath.Join(t.TempDir(), "r"))
	if res.Event != agentproto.EventFinished {
		t.Fatalf("%+v", res)
	}
	if !strings.Contains(res.Log, "••••") || strings.Contains(res.Log, "segredo123") {
		t.Fatalf("log = %q", res.Log)
	}
}

func TestRunTestFreshFolder(t *testing.T) {
	needPython(t)
	folder := filepath.Join(t.TempDir(), "rascunho-7")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "velho.pdf"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &Runner{StateDir: t.TempDir()}
	res := r.RunTest(context.Background(), testPayload("open('novo.txt','w').write('n')\n", nil), folder)
	if slices.Contains(paths(res.Files), "velho.pdf") {
		t.Fatalf("old file survived: %v", paths(res.Files))
	}
	if _, err := os.Stat(filepath.Join(folder, "velho.pdf")); !os.IsNotExist(err) {
		t.Fatal("folder not cleaned")
	}
}

func TestEvidenceZipLimit(t *testing.T) {
	folder := t.TempDir()
	os.WriteFile(filepath.Join(folder, ".apphub-console.txt"), []byte("log"), 0o600)
	os.WriteFile(filepath.Join(folder, "a.txt"), bytes.Repeat([]byte("a"), 10), 0o600)
	os.MkdirAll(filepath.Join(folder, "sub"), 0o700)
	os.WriteFile(filepath.Join(folder, "sub", "b.txt"), bytes.Repeat([]byte("b"), 10), 0o600)
	var buf bytes.Buffer
	skipped, err := evidenceZip(&buf, folder, 15)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if !slices.Equal(names, []string{"a.txt"}) || !slices.Equal(skipped, []string{"sub/b.txt"}) {
		t.Fatalf("names=%v skipped=%v", names, skipped)
	}
}

func TestAgentTestJobSendsEvidenceBeforeResult(t *testing.T) {
	needPython(t)
	hub := &fakeHub{results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	hub.jobs = []agentproto.Job{{ID: 11, Kind: agentproto.KindTest, Payload: payload(testPayload("open('a.pdf','w').write('pdf')\nprint('ok')\n", nil))}}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()
	state := t.TempDir()
	a := &Agent{Client: &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}, Runner: &Runner{StateDir: state}, StateDir: state}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Loop(ctx)
	waitFor(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return len(hub.results[11]) == 1
	})
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if !strings.HasPrefix(hub.uploads[11], "evidence:PK") {
		t.Fatalf("evidence not sent before the result: %q", hub.uploads[11])
	}
	r := hub.results[11][0]
	if r.Event != agentproto.EventFinished || r.Log != "ok\n" || !slices.Contains(paths(r.Files), "a.pdf") {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(filepath.Join(TestFolder(state, 3, 7), "a.pdf")); err != nil {
		t.Fatalf("test folder: %v", err)
	}
}
