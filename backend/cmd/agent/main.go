// Command agent is the Agente do Ziv: it runs invisibly on a person's Windows
// computer and opens apps when the Hub asks.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agent"
	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// hubURL is set at build time (scripts/build-agent.sh); APPHUB_URL overrides it.
var hubURL = "http://localhost:8080"

var lockAddr = "127.0.0.1:47631"
var workspaceID string

// connected turns true once this agent reached the Hub; /status reports it so
// an updating agent knows the new version works.
var connected atomic.Bool

func main() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	dir, err := configureWorkspace(exePath, stateDir())
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		os.Exit(1)
	}
	if f, err := os.OpenFile(dir+string(os.PathSeparator)+"agente.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(f, nil)))
	}

	if len(os.Args) > 1 && os.Args[1] == "--update" {
		runUpdate(dir)
		return
	}

	handedOff, err := install(dir)
	if err != nil {
		slog.Error("install", "err", err)
		os.Exit(1)
	}
	if handedOff {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ln, err := net.Listen("tcp", lockAddr)
	if err != nil {
		slog.Info("another agent is already running")
		return
	}
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("workspace") != workspaceID {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/quit" {
			w.WriteHeader(http.StatusNoContent)
			go func() { time.Sleep(100 * time.Millisecond); os.Exit(0) }()
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/status" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"version": agentproto.Version, "connected": connected.Load()})
			return
		}
		http.NotFound(w, r)
	}))

	name, _ := os.Hostname()
	client := &agent.Client{BaseURL: hubURL, Token: agent.LoadToken(dir), HTTP: &http.Client{Timeout: 0}}
	if client.Token == "" {
		token, err := agent.Pair(ctx, client, name, openBrowser)
		if err != nil {
			slog.Error("pairing", "err", err)
			return
		}
		if err := agent.SaveToken(dir, token); err != nil {
			slog.Error("save token", "err", err)
			return
		}
		client.Token = token
	}
	for {
		err := client.Hello(ctx, name)
		if err == nil {
			break
		}
		if errors.Is(err, agent.ErrUnauthorized) {
			forget(dir)
			return
		}
		slog.Warn("hello", "err", err)
		time.Sleep(5 * time.Second)
	}
	connected.Store(true)
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("cannot locate the running agent; self-update may fail", "err", err)
	}
	runner := &agent.Runner{Launcher: agent.ExecLauncher{}, StateDir: dir}
	a := &agent.Agent{
		Client: client, StateDir: dir, Name: name, ChooseFolder: chooseFolder, Runner: runner,
		Updater: &agent.Updater{Client: client, Runner: runner, StateDir: dir, CurrentExe: exe,
			Start: func(p string, args ...string) error { _, err := startProcess(p, args...); return err }},
	}
	if err := a.Loop(ctx); errors.Is(err, agent.ErrUnauthorized) {
		forget(dir)
	}
}

// forget runs when the computer was disconnected in the Hub: the agent drops
// its token and stops starting with Windows, until it is installed again.
func forget(dir string) {
	slog.Info("computer disconnected in the Hub; forgetting token and autostart")
	_ = agent.SaveToken(dir, "")
	if err := removeAutostart(); err != nil {
		slog.Warn("remove autostart", "err", err)
	}
}

// runUpdate is update mode: this is the downloaded agent, which installs
// itself over the current one and rolls back if it does not connect.
func runUpdate(dir string) {
	self, err := os.Executable()
	if err != nil {
		slog.Error("update: executable", "err", err)
		return
	}
	s := &agent.Swap{
		Self: self, Target: filepath.Join(dir, "AppHubAgente.exe"), Backup: filepath.Join(dir, "AppHubAgente.anterior.exe"),
		QuitRunning: quitRunning,
		Start:       func(exe string) (func(), error) { return startProcess(exe) },
		Connected: func(ctx context.Context) bool {
			return agent.StatusConnected(ctx, localURL("/status"), agentproto.Version)
		},
	}
	switch err := s.Run(context.Background()); {
	case err == nil:
		slog.Info("update done", "version", agentproto.Version)
	case errors.Is(err, agent.ErrRolledBack):
		slog.Warn("update rolled back", "version", agentproto.Version)
	default:
		slog.Error("update", "err", err)
	}
}

// quitRunning asks the agent on this computer to close and gives Windows a
// moment to release its file. The short timeout keeps a hung agent from
// blocking the swap.
func quitRunning() {
	hc := &http.Client{Timeout: 3 * time.Second}
	if resp, err := hc.Post(localURL("/quit"), "", nil); err == nil {
		resp.Body.Close()
		time.Sleep(2 * time.Second)
	}
}

// startProcess opens a program without waiting. stop closes it politely via
// /quit, kills it if it has not exited after a few seconds, and returns only
// once the process is gone so its exe file is released.
func startProcess(exe string, args ...string) (func(), error) {
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	return func() {
		quitRunning()
		select {
		case <-done:
			return
		case <-time.After(3 * time.Second):
		}
		_ = cmd.Process.Kill()
		<-done
	}, nil
}
