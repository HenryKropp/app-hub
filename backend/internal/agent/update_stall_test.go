package agent

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// A new agent that never closes this one must not leave the runner held.
func TestUpdaterReleasesWhenHandOverStalls(t *testing.T) {
	hub := &fakeHub{results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	hub.jobs = []agentproto.Job{{ID: 9, Kind: agentproto.KindUpdate, Payload: payload(agentproto.UpdatePayload{SHA256: digest("MZ-new"), Size: 6})}}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}
	u, rec := newUpdater(t, client)
	u.HandOver = 50 * time.Millisecond
	a := &Agent{Client: client, Runner: u.Runner, StateDir: u.StateDir, Updater: u}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Loop(ctx)
	waitFor(t, func() bool { hub.mu.Lock(); defer hub.mu.Unlock(); return len(hub.results[9]) == 1 })
	hub.mu.Lock()
	r := hub.results[9][0]
	hub.mu.Unlock()
	if r.Event != agentproto.EventFailed || r.Error != "Não consegui atualizar o Agente." || rec.n() != 1 {
		t.Fatalf("%+v starts %d", r, rec.n())
	}
	res := u.Runner.Run(ctx, agentproto.RunPayload{}, func(agentproto.Result) {})
	if strings.Contains(res.Error, "se atualizando") || strings.Contains(res.Error, "Já tem um app") {
		t.Fatalf("runner still held: %+v", res)
	}
}
