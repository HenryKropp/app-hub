package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/agent"
)

// Texts the person sees (ADR 049).
const (
	confirmQuit      = "Fechar o Ziv Agente?\n\nApps agendados neste computador não vão rodar até você entrar de novo no Windows."
	confirmUninstall = "Desinstalar o Ziv Agente deste computador?\n\nEle para de rodar, sai do início do Windows e este computador é desconectado do Ziv."
	alreadyRunning   = "O Ziv Agente já está rodando. Veja o ícone perto do relógio."
	appDisplayName   = "Ziv Agente"
)

// trayLabel is the first line of the icon's menu (and its tooltip).
func trayLabel(st agent.State, version, name string) string {
	prefix := "Ziv Agente " + version + " — "
	switch st {
	case agent.StateConnected:
		return prefix + "Conectado (" + name + ")"
	case agent.StateOffline:
		return prefix + "Sem conexão, tentando de novo sozinho"
	case agent.StateDisconnected:
		return prefix + "Desconectado do Ziv"
	default:
		return prefix + "Conectando..."
	}
}

// uninstallSteps removes the Agente from this computer. Every step runs even
// when an earlier one fails: without internet the Ziv is not told, but the
// computer is still cleaned.
type uninstallSteps struct {
	QuitRunning     func()
	Goodbye         func(ctx context.Context) error
	RemoveAutostart func() error
	RemoveAppEntry  func() error
	RemoveFolder    func(dir string) error
}

func (u uninstallSteps) run(ctx context.Context, dir string) []error {
	var errs []error
	note := func(step string, err error) {
		if err != nil {
			slog.Warn("uninstall", "step", step, "err", err)
			errs = append(errs, fmt.Errorf("%s: %w", step, err))
		}
	}
	u.QuitRunning()
	note("goodbye", u.Goodbye(ctx))
	note("autostart", u.RemoveAutostart())
	note("app entry", u.RemoveAppEntry())
	note("folder", u.RemoveFolder(dir))
	return errs
}

// appEntryName is the Installed apps key of this company's Agente: one per
// company, like autostartName (ADR 049).
func appEntryName(workspace string) string {
	if workspace == "" {
		return "ZivAgente"
	}
	return "ZivAgente-" + workspace
}

// appEntryLabel names the entry with the company's Ziv address, so two
// companies' Agentes are told apart in Settings.
func appEntryLabel(hub string) string {
	u, err := url.Parse(hub)
	if err != nil || u.Host == "" {
		return appDisplayName
	}
	return appDisplayName + " (" + u.Host + strings.TrimRight(u.Path, "/") + ")"
}

// removalCommand is the cmd.exe line that deletes the Agente's folder after
// it exits (ping is the wait: timeout needs a console). The root company's
// folder holds the other companies' Agentes under workspaces\, so for it
// (keepWorkspaces) only its own files go, and the folder itself only when
// nothing else is left.
func removalCommand(dir string, keepWorkspaces bool) (string, error) {
	parts := []string{"ping -n 4 127.0.0.1 >nul"}
	if !keepWorkspaces {
		return strings.Join(append(parts, `rmdir /s /q "`+dir+`"`), " & "), nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.Name() == "workspaces" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			parts = append(parts, `rmdir /s /q "`+p+`"`)
		} else {
			parts = append(parts, `del /f /q "`+p+`"`)
		}
	}
	// Plain rmdir removes the folder only when it is empty.
	parts = append(parts, `rmdir "`+dir+`"`)
	return strings.Join(parts, " & "), nil
}
