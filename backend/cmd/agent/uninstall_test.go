package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestRemovalCommandKeepsOtherCompanies(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"workspaces", "runs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "AppHubAgente.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd, err := removalCommand(root, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`rmdir /s /q "` + filepath.Join(root, "runs") + `"`,
		`del /f /q "` + filepath.Join(root, "AppHubAgente.exe") + `"`,
		`rmdir "` + root + `"`,
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command lacks %q:\n%s", want, cmd)
		}
	}
	if strings.Contains(cmd, "workspaces") {
		t.Errorf("command touches other companies:\n%s", cmd)
	}

	company := filepath.Join(root, "workspaces", "abc")
	cmd, err = removalCommand(company, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cmd, `rmdir /s /q "`+company+`"`) {
		t.Errorf("company command:\n%s", cmd)
	}
}

func TestAppEntryPerCompany(t *testing.T) {
	if got := appEntryName(""); got != "ZivAgente" {
		t.Errorf("root entry %q", got)
	}
	if got := appEntryName("abc123"); got != "ZivAgente-abc123" {
		t.Errorf("company entry %q", got)
	}
	if got := appEntryLabel("https://zivapp.duckdns.org/w/teste"); got != "Ziv Agente (zivapp.duckdns.org/w/teste)" {
		t.Errorf("label %q", got)
	}
}
