// Command agent is the Agente do Hub: it runs invisibly on a person's Windows
// computer and opens apps when the Hub asks.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agent"
)

// hubURL is set at build time (scripts/build-agent.sh); APPHUB_URL overrides it.
var hubURL = "http://localhost:8080"

const lockAddr = "127.0.0.1:47631"

func main() {
	if v := os.Getenv("APPHUB_URL"); v != "" {
		hubURL = v
	}
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		os.Exit(1)
	}
	if f, err := os.OpenFile(dir+string(os.PathSeparator)+"agente.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(f, nil)))
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
		if r.Method == http.MethodPost && r.URL.Path == "/quit" {
			w.WriteHeader(http.StatusNoContent)
			go func() { time.Sleep(100 * time.Millisecond); os.Exit(0) }()
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
	a := &agent.Agent{
		Client: client, StateDir: dir, Name: name, ChooseFolder: chooseFolder,
		Runner: &agent.Runner{Launcher: agent.ExecLauncher{}, StateDir: dir},
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
