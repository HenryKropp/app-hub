//go:build windows

package main

import (
	"context"
	_ "embed"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

const createNoWindow = 0x08000000

func stateDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "AppHub")
}

// install copies this program to %LOCALAPPDATA%\AppHub, registers it to start
// with Windows and starts the installed copy. It returns true when the
// installed copy took over.
func install(dir string) (bool, error) {
	self, err := os.Executable()
	if err != nil {
		return false, err
	}
	target := filepath.Join(dir, "AppHubAgente.exe")
	if strings.EqualFold(filepath.Clean(self), filepath.Clean(target)) {
		return false, nil
	}
	// An older agent may be running from the target; ask it to quit.
	if resp, err := http.Post(localURL("/quit"), "", nil); err == nil {
		resp.Body.Close()
		time.Sleep(2 * time.Second)
	}
	var copyErr error
	for range 5 {
		if copyErr = copySelf(self, target); copyErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if copyErr != nil {
		return false, copyErr
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	if err := k.SetStringValue(autostartName(), `"`+target+`"`); err != nil {
		return false, err
	}
	return true, exec.Command(target).Start()
}

func copySelf(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

//go:embed folderpicker.ps1
var folderScript []byte

// chooseFolder runs the folder picker script (written next to the agent) and
// returns the folder the person chose.
func chooseFolder(ctx context.Context, initial string) (string, bool, error) {
	script := filepath.Join(stateDir(), "escolher-pasta.ps1")
	// UTF-8 with BOM so Windows PowerShell reads the accents in the script right.
	if err := os.WriteFile(script, append([]byte("\ufeff"), folderScript...), 0o600); err != nil {
		return "", false, err
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-File", script)
	cmd.Env = append(os.Environ(), "APPHUB_INITIAL="+initial)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return "", false, err
	}
	folder := strings.TrimSpace(strings.TrimPrefix(string(out), "\ufeff"))
	return folder, folder != "", nil
}

func openBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func removeAutostart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.DeleteValue(autostartName())
}
