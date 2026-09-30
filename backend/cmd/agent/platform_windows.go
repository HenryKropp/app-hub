//go:build windows

package main

import (
	"context"
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
	if resp, err := http.Post("http://"+lockAddr+"/quit", "", nil); err == nil {
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
	if err := k.SetStringValue("AppHubAgente", `"`+target+`"`); err != nil {
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

// folderScript shows the Windows folder picker on top of other windows and
// prints the chosen path as UTF-8 (paths like "8. Programação").
const folderScript = `
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form -Property @{TopMost = $true}
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = 'Onde salvar os resultados'
$d.ShowNewFolderButton = $true
if ($env:APPHUB_INITIAL -and (Test-Path -LiteralPath $env:APPHUB_INITIAL)) { $d.SelectedPath = $env:APPHUB_INITIAL }
if ($d.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($d.SelectedPath) }
`

func chooseFolder(ctx context.Context, initial string) (string, bool, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", folderScript)
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
