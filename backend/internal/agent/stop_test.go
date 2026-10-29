package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// openLauncher writes a file and stays open until the run is stopped.
type openLauncher struct{ opened chan struct{} }

func (l openLauncher) Launch(ctx context.Context, exe, dir string) (int, error) {
	_ = os.WriteFile(filepath.Join(dir, "parcial.xlsx"), []byte("x"), 0o644)
	close(l.opened)
	<-ctx.Done()
	return 1, nil
}

func TestStopClosesAppAndKeepsItsFiles(t *testing.T) {
	app := appFolder(t)
	l := openLauncher{opened: make(chan struct{})}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	got := make(chan agentproto.Result, 1)
	go func() {
		got <- r.Run(context.Background(), agentproto.RunPayload{RunID: 9, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app}, func(agentproto.Result) {})
	}()
	<-l.opened
	if _, ok := r.Stop(8); ok {
		t.Fatal("stopped a run that is not open")
	}
	done, ok := r.Stop(9)
	if !ok {
		t.Fatal("could not stop the open run")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not wrap up")
	}
	res := <-got
	if res.Event != agentproto.EventFinished || !res.Stopped || len(res.Files) != 1 || res.Files[0].Path != "parcial.xlsx" {
		t.Fatalf("result %+v", res)
	}
	if _, ok := r.Stop(9); ok {
		t.Fatal("stopped a run that already closed")
	}
}

func TestRunNotStoppedIsNotMarked(t *testing.T) {
	app := appFolder(t)
	r := &Runner{Launcher: &fakeLauncher{}, StateDir: t.TempDir()}
	res := r.Run(context.Background(), agentproto.RunPayload{RunID: 1, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFinished || res.Stopped {
		t.Fatalf("result %+v", res)
	}
}

func TestRunTreeClosesWhatTheAppOpened(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	mark := filepath.Join(t.TempDir(), "filho-vivo")
	// The child outlives its parent's kill unless the whole group is closed.
	cmd := exec.Command("sh", "-c", "(sleep 1; touch "+mark+") & sleep 30")
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- runTree(ctx, cmd) }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case <-errc:
	case <-time.After(5 * time.Second):
		t.Fatal("runTree did not return after cancel")
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("the app's child kept running")
	}
}

func TestStopScriptHandsOverWhatItWrote(t *testing.T) {
	if _, _, err := scriptInterpreter("python"); err != nil {
		t.Skip(err)
	}
	app := t.TempDir()
	write(t, app, "main.py", "import time\nopen('parcial.txt','w').write('ok')\nprint('comecei', flush=True)\ntime.sleep(60)\n")
	out := t.TempDir()
	m := manifest.Manifest{Kind: manifest.KindScript, Entrypoint: "main.py"}
	r := &Runner{StateDir: t.TempDir()}
	started := make(chan struct{})
	got := make(chan agentproto.Result, 1)
	go func() {
		got <- r.Run(context.Background(), agentproto.RunPayload{RunID: 301, AppFolder: app, Exe: "main.py", OutputFolder: out,
			Script: &agentproto.ScriptPayload{Manifest: m}}, func(agentproto.Result) { close(started) })
	}()
	<-started
	// Wait for the script to write its file before stopping it.
	for i := 0; i < 100; i++ {
		if matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "apphub-script-*", "app", "parcial.txt")); len(matches) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, ok := r.Stop(301); !ok {
		t.Fatal("could not stop the script")
	}
	var res agentproto.Result
	select {
	case res = <-got:
	case <-time.After(10 * time.Second):
		t.Fatal("script did not stop")
	}
	if res.Event != agentproto.EventFinished || !res.Stopped {
		t.Fatalf("result %+v", res)
	}
	var paths []string
	for _, f := range res.Files {
		paths = append(paths, f.Path)
	}
	if !strings.Contains(strings.Join(paths, ","), "Execucao-301/parcial.txt") {
		t.Fatalf("files %v", paths)
	}
}
