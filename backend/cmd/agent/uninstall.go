package main

import (
	"context"
	"fmt"
	"log/slog"

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
