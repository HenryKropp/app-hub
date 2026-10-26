package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// The ZIP distributed by a company contains a non-secret endpoint sidecar.
// Persist it alongside the installed agent so auto-updates retain the company.
func configureWorkspace(executable, root string) (string, error) {
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(executable), "apphub-url.txt")); err == nil {
		if len(b) > 2048 {
			return "", errors.New("invalid Hub endpoint")
		}
		hubURL = strings.TrimSpace(string(b))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if v := os.Getenv("APPHUB_URL"); v != "" {
		hubURL = v
	}
	u, err := url.Parse(hubURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return "", errors.New("Hub endpoint must be HTTPS")
	}
	hubURL = strings.TrimRight(hubURL, "/")
	dir := root
	if strings.Trim(u.Path, "/") != "" {
		sum := sha256.Sum256([]byte(hubURL))
		workspaceID = hex.EncodeToString(sum[:8])
		dir = filepath.Join(root, "workspaces", workspaceID)
		// Root company keeps its legacy port and install directory.
		port := 20000 + (int(sum[0])<<8|int(sum[1]))%25000
		lockAddr = fmt.Sprintf("127.0.0.1:%d", port)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "apphub-url.txt"), []byte(hubURL+"\n"), 0o600); err != nil {
		return "", err
	}
	return dir, nil
}

func localURL(endpoint string) string {
	return "http://" + lockAddr + endpoint + "?workspace=" + workspaceID
}

func autostartName() string {
	if workspaceID == "" {
		return "AppHubAgente"
	}
	return "AppHubAgente-" + workspaceID
}
