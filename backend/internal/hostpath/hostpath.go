// Package hostpath translates between the Windows paths people type and the
// paths the Hub server can open. The server runs inside WSL, where C:\ is
// mounted at /mnt/c; on other systems paths pass through untouched.
package hostpath

import (
	"path"
	"regexp"
	"runtime"
	"strings"
)

var (
	drivePath = regexp.MustCompile(`^([A-Za-z]):[\\/]?(.*)$`)
	wslShare  = regexp.MustCompile(`(?i)^[\\/]{2}wsl(?:\$|\.localhost)[\\/][^\\/]+[\\/]?(.*)$`)
	mntPath   = regexp.MustCompile(`^/mnt/([a-z])(?:/(.*))?$`)
)

// ToLocal converts "C:\Users\x" into "/mnt/c/Users/x" and "\\wsl$\Ubuntu\home\x"
// into "/home/x". Surrounding quotes and spaces are removed.
func ToLocal(p string) string {
	p = strings.Trim(strings.TrimSpace(p), `"'`)
	if runtime.GOOS == "windows" || p == "" {
		return p
	}
	if m := wslShare.FindStringSubmatch(p); m != nil {
		return path.Clean("/" + strings.ReplaceAll(m[1], `\`, "/"))
	}
	if m := drivePath.FindStringSubmatch(p); m != nil {
		rest := strings.ReplaceAll(m[2], `\`, "/")
		return path.Clean("/mnt/" + strings.ToLower(m[1]) + "/" + rest)
	}
	return path.Clean(p)
}

// ToDisplay converts "/mnt/c/Users/x" back into "C:\Users\x" for people.
func ToDisplay(p string) string {
	if m := mntPath.FindStringSubmatch(p); m != nil {
		return strings.ToUpper(m[1]) + `:\` + strings.ReplaceAll(m[2], "/", `\`)
	}
	return p
}
