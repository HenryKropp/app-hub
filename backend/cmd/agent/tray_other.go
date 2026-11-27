//go:build !windows

package main

import "context"

// runTray has no icon off Windows: it waits for the Agente to stop.
func runTray(ctx context.Context, _ trayActions) { <-ctx.Done() }
