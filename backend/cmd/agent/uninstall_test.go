package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agent"
)

func TestUninstallStepsContinueWhenGoodbyeFails(t *testing.T) {
	var calls []string
	u := uninstallSteps{
		QuitRunning:     func() { calls = append(calls, "quit") },
		Goodbye:         func(context.Context) error { calls = append(calls, "goodbye"); return errors.New("sem internet") },
		RemoveAutostart: func() error { calls = append(calls, "autostart"); return nil },
		RemoveAppEntry:  func() error { calls = append(calls, "entry"); return nil },
		RemoveFolder:    func(dir string) error { calls = append(calls, "folder:"+dir); return nil },
	}
	errs := u.run(context.Background(), `C:\pasta`)
	want := []string{"quit", "goodbye", "autostart", "entry", `folder:C:\pasta`}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls %v", calls)
	}
	if len(errs) != 1 {
		t.Fatalf("errors %v", errs)
	}
}

func TestTrayLabel(t *testing.T) {
	cases := map[agent.State]string{
		agent.StateConnected:    "Ziv Agente 0.5.0 — Conectado (PC-1)",
		agent.StateOffline:      "Ziv Agente 0.5.0 — Sem conexão, tentando de novo sozinho",
		agent.StateDisconnected: "Ziv Agente 0.5.0 — Desconectado do Ziv",
		agent.StateConnecting:   "Ziv Agente 0.5.0 — Conectando...",
	}
	for st, want := range cases {
		if got := trayLabel(st, "0.5.0", "PC-1"); got != want {
			t.Errorf("state %d: %q, want %q", st, got, want)
		}
	}
}
