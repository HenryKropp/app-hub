package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// fakeLauncher simulates the app: records where it ran and writes files.
type fakeLauncher struct {
	exe, dir string
	write    map[string]string
	block    chan struct{}
}

func (f *fakeLauncher) Launch(ctx context.Context, exe, dir string) (int, error) {
	f.exe, f.dir = exe, dir
	if f.block != nil {
		<-f.block
	}
	for rel, c := range f.write {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(c), 0o644)
	}
	return 0, nil
}

func appFolder(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), "2026 06 23 - Henry Files Prep")
	write(t, dir, "Ferramenta CVM.exe", "MZ")
	write(t, dir, "PETROBRAS/FRE/velho.pdf", "x")
	return dir
}

func TestRunInAppFolder(t *testing.T) {
	app := appFolder(t)
	l := &fakeLauncher{write: map[string]string{"PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md": "# pack"}}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	var events []string
	res := r.Run(context.Background(), agentproto.RunPayload{RunID: 7, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app},
		func(x agentproto.Result) { events = append(events, x.Event+":"+x.AppVersion) })
	if res.Event != agentproto.EventFinished || len(res.Files) != 1 || res.Files[0].Path != "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md" {
		t.Fatalf("result %+v", res)
	}
	if l.exe != filepath.Join(app, "Ferramenta CVM.exe") || l.dir != app {
		t.Fatalf("launched %q in %q", l.exe, l.dir)
	}
	if len(events) != 1 || !strings.HasPrefix(events[0], "started:") || events[0] == "started:" {
		t.Fatalf("events %v", events)
	}
}

func TestRunInOtherFolderCopiesAndRemovesExe(t *testing.T) {
	app := appFolder(t)
	out := filepath.Join(t.TempDir(), "Saídas Relatórios")
	_ = os.MkdirAll(out, 0o755)
	l := &fakeLauncher{write: map[string]string{"X/FRE/novo.pdf": "p"}}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	res := r.Run(context.Background(), agentproto.RunPayload{RunID: 8, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: out}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFinished || len(res.Files) != 1 || res.Files[0].Path != "X/FRE/novo.pdf" {
		t.Fatalf("result %+v", res)
	}
	if l.exe != filepath.Join(out, "Ferramenta CVM.exe") {
		t.Fatalf("launched %q", l.exe)
	}
	if _, err := os.Stat(filepath.Join(out, "Ferramenta CVM.exe")); !os.IsNotExist(err) {
		t.Fatal("temporary copy must be removed")
	}
}

func TestRunRefusesExistingExeInOtherFolder(t *testing.T) {
	app := appFolder(t)
	out := t.TempDir()
	write(t, out, "Ferramenta CVM.exe", "da pessoa")
	r := &Runner{Launcher: &fakeLauncher{}, StateDir: t.TempDir()}
	res := r.Run(context.Background(), agentproto.RunPayload{RunID: 9, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: out}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Já existe") {
		t.Fatalf("result %+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(out, "Ferramenta CVM.exe"))
	if string(b) != "da pessoa" {
		t.Fatal("person's file was touched")
	}
}

func TestRunMissingFolderOrExe(t *testing.T) {
	app := appFolder(t)
	r := &Runner{Launcher: &fakeLauncher{}, StateDir: t.TempDir()}
	res := r.Run(context.Background(), agentproto.RunPayload{AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: filepath.Join(app, "nao-existe")}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Não consegui usar a pasta") {
		t.Fatalf("missing folder %+v", res)
	}
	res = r.Run(context.Background(), agentproto.RunPayload{AppFolder: app, Exe: "Outro.exe", OutputFolder: app}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Outro.exe") {
		t.Fatalf("missing exe %+v", res)
	}
}

func TestRunBusy(t *testing.T) {
	app := appFolder(t)
	l := &fakeLauncher{block: make(chan struct{})}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	p := agentproto.RunPayload{RunID: 1, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app}
	started := make(chan struct{})
	first := make(chan struct{})
	go func() {
		defer close(first)
		var once sync.Once
		r.Run(context.Background(), p, func(agentproto.Result) { once.Do(func() { close(started) }) })
	}()
	<-started
	res := r.Run(context.Background(), p, func(agentproto.Result) {})
	close(l.block)
	<-first // the first run must end before the test's folders go away
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Já tem um app aberto") {
		t.Fatalf("busy %+v", res)
	}
}
