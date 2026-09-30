//go:build !windows

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".apphub-agente")
}

func install(string) (bool, error) { return false, nil }

func chooseFolder(context.Context, string) (string, bool, error) {
	return "", false, errors.New("folder dialog only on Windows")
}

func openBrowser(url string) error {
	slog.Info("open this page to connect", "url", url)
	return nil
}
