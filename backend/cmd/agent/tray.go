package main

import "github.com/HenryKropp/app-hub/backend/internal/agent"

// trayActions is what the icon's menu does; main wires it (ADR 049).
type trayActions struct {
	Status    *agent.Status
	Version   string
	Name      string
	Open      func() // Abrir o Ziv
	Reconnect func() // Conectar de novo
	Quit      func() // Sair (already confirmed)
	Uninstall func() // Desinstalar (already confirmed)
}
