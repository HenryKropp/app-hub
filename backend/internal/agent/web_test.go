package agent

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// serveApp is a tiny screen app: it records what it got from the vault and
// the Agente, then answers on PORT until stopped.
const serveApp = `import os, http.server
d = os.environ["APPHUB_DATA_DIR"]
open(os.path.join(d, "visto.txt"), "w").write(os.environ["SENHA_EMAIL"] + "|" + os.environ["BASE_URL"] + "|" + os.path.basename(os.getcwd()))
http.server.HTTPServer(("127.0.0.1", int(os.environ["PORT"])), http.server.SimpleHTTPRequestHandler).serve_forever()
`

func webPayload(runID int64, out string, code string) agentproto.RunPayload {
	return agentproto.RunPayload{RunID: runID, OutputFolder: out, AppName: "Notícias", Keys: true,
		KeyValues: map[string]string{"SENHA_EMAIL": "s3gredo"},
		Web: &agentproto.WebPayload{AppID: 42, Runtime: "python", Args: []string{"app.py"},
			Sources: map[string]manifest.InputFile{"app.py": {Name: "app.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte(code))}}}}
}

func needPython(t *testing.T) {
	if _, _, err := scriptInterpreter("python"); err != nil {
		t.Skip("python not installed")
	}
}

func TestRunWebAppOpensBrowserWithVault(t *testing.T) {
	needPython(t)
	out := t.TempDir()
	r := &Runner{StateDir: t.TempDir()}
	opened := make(chan string, 1)
	r.OpenURL = func(url string) error {
		opened <- url
		go r.Stop(7)
		return nil
	}
	var started string
	res := r.Run(context.Background(), webPayload(7, out, serveApp), func(x agentproto.Result) { started = x.AppVersion })
	if res.Event != agentproto.EventFinished {
		t.Fatalf("result %+v", res)
	}
	url := <-opened
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.HasSuffix(started, url) {
		t.Fatalf("url %q started %q", url, started)
	}
	seen, err := os.ReadFile(filepath.Join(out, "visto.txt"))
	if err != nil || string(seen) != "s3gredo|"+url+"|codigo" {
		t.Fatalf("app saw %q %v", seen, err)
	}
	if len(res.Files) != 1 || res.Files[0].Path != "visto.txt" {
		t.Fatalf("files %+v", res.Files)
	}
	// No console log is left when the person stopped the app.
	if _, err := os.Stat(filepath.Join(out, webLogName(7))); err == nil {
		t.Fatal("log written after stop")
	}
}

func TestRunWebAppThatCrashesLeavesItsConsole(t *testing.T) {
	needPython(t)
	out := t.TempDir()
	r := &Runner{StateDir: t.TempDir(), OpenURL: func(string) error { return nil }}
	res := r.Run(context.Background(), webPayload(8, out, "import sys\nprint('faltou a planilha')\nsys.exit(3)\n"), func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "código 3") || !strings.Contains(res.Error, webLogName(8)) {
		t.Fatalf("result %+v", res)
	}
	log, _ := os.ReadFile(filepath.Join(out, webLogName(8)))
	if !strings.Contains(string(log), "faltou a planilha") {
		t.Fatalf("log %q", log)
	}
}

func TestRunWebAppRefusesBadPayload(t *testing.T) {
	r := &Runner{StateDir: t.TempDir()}
	p := webPayload(9, t.TempDir(), "x")
	p.Web.Runtime = "ruby"
	if res := r.Run(context.Background(), p, func(agentproto.Result) {}); res.Event != agentproto.EventFailed {
		t.Fatalf("result %+v", res)
	}
	p = webPayload(10, t.TempDir(), "x")
	p.Web.Requirements = []string{"--index-url=http://mal"}
	if _, err := exec.LookPath("python3"); err == nil {
		if res := r.Run(context.Background(), p, func(agentproto.Result) {}); res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "inválida") {
			t.Fatalf("option smuggled: %+v", res)
		}
	}
}
