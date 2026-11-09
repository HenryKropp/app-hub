package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func TestScriptPythonInputsAndIsolation(t *testing.T) {
	if _, _, err := scriptInterpreter("python"); err != nil {
		t.Skip(err)
	}
	source := `import argparse, os
p=argparse.ArgumentParser()
p.add_argument('--file')
p.add_argument('--amount')
a=p.parse_args()
with open(a.file) as f: data=f.read()
with open('result.txt','w') as f: f.write(data + ':' + a.amount)
print('finished')
`
	app := t.TempDir()
	write(t, app, "main.py", source)
	out := t.TempDir()
	m := manifest.Manifest{Kind: manifest.KindScript, Entrypoint: "main.py", Inputs: []manifest.Input{
		{ID: "file", Label: "Arquivo", Type: "file", Source: "argument", Required: true},
		{ID: "amount", Label: "Número", Type: "number", Source: "argument", Required: true},
	}}
	r := &Runner{StateDir: t.TempDir()}
	p := agentproto.RunPayload{RunID: 300, AppFolder: app, Exe: "main.py", OutputFolder: out, Script: &agentproto.ScriptPayload{
		Manifest: m, Inputs: map[string][]string{"amount": {"-3"}}, Files: map[string]manifest.InputFile{"file": {Name: "data.txt", ContentBase64: "aGVsbG8="}},
	}}
	events := 0
	res := r.Run(context.Background(), p, func(agentproto.Result) { events++ })
	if res.Event != agentproto.EventFinished || res.ExitCode != 0 || events != 1 {
		t.Fatalf("%+v", res)
	}
	data, err := os.ReadFile(filepath.Join(out, "Execucao-300", "result.txt"))
	if err != nil || string(data) != "hello:-3" {
		t.Fatalf("%s %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(app, "result.txt")); !os.IsNotExist(err) {
		t.Fatal("original folder changed")
	}
	for _, f := range res.Files {
		if strings.Contains(f.Path, "data.txt") || strings.HasSuffix(f.Path, "main.py") {
			t.Fatalf("input/source leaked %+v", f)
		}
	}
}

func TestScriptDeliveredSourcesAndNonzeroExit(t *testing.T) {
	if _, _, err := scriptInterpreter("node"); err != nil {
		t.Skip(err)
	}
	code := `require('fs').writeFileSync('answer.txt','ok'); process.exit(7)`
	out := t.TempDir()
	r := &Runner{StateDir: t.TempDir()}
	p := agentproto.RunPayload{RunID: 301, AppFolder: `C:\missing`, Exe: "main.js", OutputFolder: out, Script: &agentproto.ScriptPayload{
		Manifest: manifest.Manifest{Kind: "script", Entrypoint: "main.js"},
		Sources:  map[string]manifest.InputFile{"main.js": {Name: "main.js", ContentBase64: base64.StdEncoding.EncodeToString([]byte(code))}},
	}}
	res := r.Run(context.Background(), p, func(agentproto.Result) {})
	if res.Event != agentproto.EventFinished || res.ExitCode != 7 || len(res.Files) != 1 {
		t.Fatalf("%+v", res)
	}
	p.RunID = 302
	p.Script.Sources["../outside.js"] = manifest.InputFile{}
	if res := r.Run(context.Background(), p, func(agentproto.Result) {}); res.Event != agentproto.EventFailed {
		t.Fatalf("unsafe sources %+v", res)
	}
}

func TestScriptRefusesUnsafeInputBeforeStarting(t *testing.T) {
	r := &Runner{StateDir: t.TempDir()}
	events := 0
	res := r.Run(context.Background(), agentproto.RunPayload{Exe: "main.py", Script: &agentproto.ScriptPayload{
		Manifest: manifest.Manifest{Kind: "script", Entrypoint: "../main.py"},
	}}, func(agentproto.Result) { events++ })
	if res.Event != agentproto.EventFailed || events != 0 {
		t.Fatalf("%+v events=%d", res, events)
	}
}

func TestLimitedScriptLog(t *testing.T) {
	var log limitedBuffer
	input := []byte(strings.Repeat("x", 100000))
	if n, err := log.Write(input); n != len(input) || err != nil || log.Len() != 64<<10 {
		t.Fatal("unbounded log")
	}
}

// On Windows Python prints in cp1252 and crashed on "✓"; the Agente forces
// UTF-8 for the console. The parent's cp1252 stands in for Windows here.
func TestScriptPythonPrintsAnyCharacter(t *testing.T) {
	if _, _, err := scriptInterpreter("python"); err != nil {
		t.Skip(err)
	}
	t.Setenv("PYTHONIOENCODING", "cp1252")
	out := t.TempDir()
	r := &Runner{StateDir: t.TempDir()}
	code := "print('Olá ✓ 🙂')\n"
	p := agentproto.RunPayload{RunID: 310, AppFolder: `C:\missing`, Exe: "main.py", OutputFolder: out, Script: &agentproto.ScriptPayload{
		Manifest: manifest.Manifest{Kind: "script", Entrypoint: "main.py"},
		Sources:  map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte(code))}},
	}}
	res := r.Run(context.Background(), p, func(agentproto.Result) {})
	if res.Event != agentproto.EventFinished || res.ExitCode != 0 {
		t.Fatalf("%+v", res)
	}
	log, err := os.ReadFile(filepath.Join(out, "Execucao-310", ".apphub-console.txt"))
	if err != nil || !strings.Contains(string(log), "Olá ✓ 🙂") {
		t.Fatalf("%q %v", log, err)
	}
}

func TestScriptGetsKeysInEnvironment(t *testing.T) {
	if _, _, err := scriptInterpreter("python"); err != nil {
		t.Skip(err)
	}
	const secret = "valor-do-cofre-123"
	source := "import os\nopen(os.path.join(os.environ['APP_HUB_OUTPUT_DIR'],'k.txt'),'w').write(os.environ.get('BTG_SENHA',''))\nprint('ok')\n"
	out := t.TempDir()
	r := &Runner{StateDir: t.TempDir()}
	p := agentproto.RunPayload{RunID: 401, AppFolder: `C:\missing`, Exe: "main.py", OutputFolder: out, Keys: true, KeyValues: map[string]string{"BTG_SENHA": secret},
		Script: &agentproto.ScriptPayload{Manifest: manifest.Manifest{Kind: "script", Entrypoint: "main.py"},
			Sources: map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte(source))}}}}
	if res := r.Run(context.Background(), p, func(agentproto.Result) {}); res.Event != agentproto.EventFinished || res.ExitCode != 0 {
		t.Fatalf("%+v", res)
	}
	got, err := os.ReadFile(filepath.Join(out, "Execucao-401", "k.txt"))
	if err != nil || string(got) != secret {
		t.Fatalf("env %q %v", got, err)
	}
	if b, _ := json.Marshal(p); bytes.Contains(b, []byte(secret)) {
		t.Fatal("key values must never be serialized")
	}
}
