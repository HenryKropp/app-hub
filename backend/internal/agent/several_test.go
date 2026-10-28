package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// waitLauncher writes saida.txt and stays open until released or stopped.
type waitLauncher struct {
	opened  chan string
	release chan struct{}
}

func (l waitLauncher) Launch(ctx context.Context, exe, dir string) (int, error) {
	_ = os.WriteFile(filepath.Join(dir, "saida.txt"), []byte(dir), 0o644)
	l.opened <- dir
	select {
	case <-l.release:
	case <-ctx.Done():
	}
	return 0, nil
}

func TestTwoAppsOpenTogetherInSeparateFolders(t *testing.T) {
	app := appFolder(t)
	l := waitLauncher{opened: make(chan string, 2), release: make(chan struct{})}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	got := map[int64]chan agentproto.Result{1: make(chan agentproto.Result, 1), 2: make(chan agentproto.Result, 1)}
	for id := range got {
		out := filepath.Join(t.TempDir(), "saida")
		write(t, out, "antigo.txt", "x")
		go func() {
			got[id] <- r.Run(context.Background(), agentproto.RunPayload{RunID: id, AppName: "Ferramenta CVM", AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: out}, func(agentproto.Result) {})
		}()
	}
	for range 2 {
		select {
		case <-l.opened:
		case <-time.After(5 * time.Second):
			t.Fatal("both apps should open")
		}
	}
	// Stopping one leaves the other open.
	if _, ok := r.Stop(1); !ok {
		t.Fatal("stop 1")
	}
	first := <-got[1]
	if !first.Stopped || len(first.Files) != 1 || first.Files[0].Path != "saida.txt" {
		t.Fatalf("first %+v", first)
	}
	select {
	case res := <-got[2]:
		t.Fatalf("second closed with the first: %+v", res)
	case <-time.After(100 * time.Millisecond):
	}
	close(l.release)
	second := <-got[2]
	if second.Stopped || second.Event != agentproto.EventFinished || len(second.Files) != 1 || second.Files[0].Path != "saida.txt" {
		t.Fatalf("second %+v", second)
	}
}

func TestFolderInUseIsRefused(t *testing.T) {
	app := appFolder(t)
	out := t.TempDir()
	l := waitLauncher{opened: make(chan string, 1), release: make(chan struct{})}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	first := make(chan agentproto.Result, 1)
	go func() {
		first <- r.Run(context.Background(), agentproto.RunPayload{RunID: 1, AppName: "Ferramenta CVM", AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: out}, func(agentproto.Result) {})
	}()
	<-l.opened
	for _, folder := range []string{out, filepath.Join(out, "Março")} {
		_ = os.MkdirAll(folder, 0o755)
		res := r.Run(context.Background(), agentproto.RunPayload{RunID: 2, AppName: "Outro", AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: folder}, func(agentproto.Result) { t.Fatal("opened in a folder in use") })
		if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "já está sendo usada pelo Ferramenta CVM") {
			t.Fatalf("%s: %+v", folder, res)
		}
	}
	close(l.release)
	<-first
	if _, err := os.Stat(filepath.Join(out, "Março", "Ferramenta CVM.exe")); err == nil {
		t.Fatal("refused run copied the app")
	}
}

func TestScriptsShareAFolderButNotWithApps(t *testing.T) {
	r := &Runner{}
	run := func(id int64, script bool) *openRun {
		return &openRun{id: id, name: "App", folder: `C:\Dados`, script: script, cancel: func() {}, done: make(chan struct{})}
	}
	if res := r.claim(run(1, true)); res.Event != "" {
		t.Fatalf("script 1 %+v", res)
	}
	if res := r.claim(run(2, true)); res.Event != "" {
		t.Fatalf("script 2 %+v", res)
	}
	if res := r.claim(run(3, false)); res.Event != agentproto.EventFailed {
		t.Fatalf("app beside scripts %+v", res)
	}
}

func TestUpdateWaitsWithoutBlockingNewApps(t *testing.T) {
	r := &Runner{}
	a := &openRun{id: 1, folder: `C:\A`, cancel: func() {}, done: make(chan struct{})}
	b := &openRun{id: 2, folder: `C:\B`, cancel: func() {}, done: make(chan struct{})}
	r.claim(a)
	held := make(chan func(), 1)
	go func() {
		release, _ := r.Hold(context.Background())
		held <- release
	}()
	time.Sleep(50 * time.Millisecond)
	if res := r.claim(b); res.Event != "" {
		t.Fatalf("a waiting update blocked a new app: %+v", res)
	}
	r.release(a)
	select {
	case <-held:
		t.Fatal("updated while an app was open")
	case <-time.After(50 * time.Millisecond):
	}
	r.release(b)
	var release func()
	select {
	case release = <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("update kept waiting after every app closed")
	}
	if res := r.claim(&openRun{id: 3, folder: `C:\C`, done: make(chan struct{})}); !strings.Contains(res.Error, "se atualizando") {
		t.Fatalf("opened during the update: %+v", res)
	}
	release()
	if res := r.claim(&openRun{id: 3, folder: `C:\C`, done: make(chan struct{})}); res.Event != "" {
		t.Fatalf("after release %+v", res)
	}
}

func TestHoldGivesUpWhenAgentCloses(t *testing.T) {
	r := &Runner{}
	r.claim(&openRun{id: 1, folder: `C:\A`, done: make(chan struct{})})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Hold(ctx); err == nil {
		t.Fatal("Hold should give up")
	}
}
