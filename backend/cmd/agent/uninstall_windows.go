//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\ZivAgente`

// registerAppEntry lists the Agente in Settings → Apps → Installed apps for
// this Windows user, with Windows' own Uninstall button (no admin needed).
func registerAppEntry(dir string) error {
	exe := filepath.Join(dir, "AppHubAgente.exe")
	k, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	for name, v := range map[string]string{
		"DisplayName":     appDisplayName,
		"Publisher":       "Ziv",
		"DisplayVersion":  agentproto.Version,
		"DisplayIcon":     exe,
		"InstallLocation": dir,
		"UninstallString": `"` + exe + `" --desinstalar`,
	} {
		if err := k.SetStringValue(name, v); err != nil {
			return err
		}
	}
	for _, name := range []string{"NoModify", "NoRepair"} {
		if err := k.SetDWordValue(name, 1); err != nil {
			return err
		}
	}
	return nil
}

func removeAppEntry() error {
	err := registry.DeleteKey(registry.CURRENT_USER, uninstallKey)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

// removeFolder deletes the Agente's folder after this process exits: a
// running program cannot delete its own file. ping is the wait because
// timeout needs a console.
func removeFolder(dir string) error {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
		CmdLine:       `cmd.exe /c ping -n 4 127.0.0.1 >nul & rmdir /s /q "` + dir + `"`,
	}
	cmd.Dir = os.TempDir()
	return cmd.Start()
}

const (
	mbYesNo        = 0x00000004
	mbIconQuestion = 0x00000020
	mbIconInfo     = 0x00000040
	mbForeground   = 0x00010000
	mbTopmost      = 0x00040000
	idYes          = 6
)

// messageBox shows a Windows dialog; with yesNo it returns true for "Sim".
func messageBox(text string, yesNo bool) bool {
	flags := uint32(mbIconInfo | mbForeground | mbTopmost)
	if yesNo {
		flags = mbYesNo | mbIconQuestion | mbForeground | mbTopmost
	}
	t, _ := windows.UTF16PtrFromString(text)
	c, _ := windows.UTF16PtrFromString(appDisplayName)
	ret, _ := windows.MessageBox(0, t, c, flags)
	return !yesNo || ret == idYes
}
