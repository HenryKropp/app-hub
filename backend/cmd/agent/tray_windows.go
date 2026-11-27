//go:build windows

package main

import (
	"context"
	_ "embed"
	"runtime"

	"fyne.io/systray"

	"github.com/HenryKropp/app-hub/backend/internal/agent"
)

//go:embed icons/conectado.ico
var iconConnected []byte

//go:embed icons/sem-conexao.ico
var iconOffline []byte

// runTray shows the icon next to the clock until ctx ends. It must run on
// the main goroutine (Windows sends the icon's messages to that thread).
func runTray(ctx context.Context, t trayActions) {
	runtime.LockOSThread()
	systray.Run(func() {
		status := systray.AddMenuItem("", "")
		status.Disable()
		systray.AddSeparator()
		open := systray.AddMenuItem("Abrir o Ziv", "")
		reconnect := systray.AddMenuItem("Conectar de novo", "")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Sair", "")
		uninstall := systray.AddMenuItem("Desinstalar...", "")

		show := func(st agent.State) {
			label := trayLabel(st, t.Version, t.Name)
			status.SetTitle(label)
			systray.SetTooltip(label)
			if st == agent.StateConnected {
				systray.SetIcon(iconConnected)
			} else {
				systray.SetIcon(iconOffline)
			}
			if st == agent.StateDisconnected {
				reconnect.Show()
			} else {
				reconnect.Hide()
			}
		}
		t.Status.Subscribe(show)
		show(t.Status.Get())

		go func() {
			for {
				select {
				case <-ctx.Done():
					systray.Quit()
					return
				case <-open.ClickedCh:
					t.Open()
				case <-reconnect.ClickedCh:
					t.Reconnect()
				case <-quit.ClickedCh:
					if messageBox(confirmQuit, true) {
						t.Quit()
					}
				case <-uninstall.ClickedCh:
					if messageBox(confirmUninstall, true) {
						t.Uninstall()
					}
				}
			}
		}()
	}, func() {})
}
