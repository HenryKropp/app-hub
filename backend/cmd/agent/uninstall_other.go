//go:build !windows

package main

import "log/slog"

func registerAppEntry(string) error { return nil }
func removeAppEntry() error         { return nil }
func removeFolder(string) error     { return nil }

// messageBox has no dialog off Windows: it logs and answers yes.
func messageBox(text string, yesNo bool) bool {
	slog.Info("message", "text", text)
	return true
}
