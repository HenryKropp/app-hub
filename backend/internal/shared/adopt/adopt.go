// Package adopt takes over what Kova and the App Hub left under their old
// names (the Postgres container and the shared apps' data folder) by renaming
// it, so the first Ziv start finds the same data. It never creates or deletes anything.
package adopt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// Dir renames oldPath to newPath when only the old one exists. In any other
// case (new already there, or neither) it does nothing.
func Dir(newPath, oldPath string) error {
	if _, err := os.Stat(newPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := os.Stat(oldPath); err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

// Container renames the container oldName to newName when only the old one
// exists (podman renames a running container too). run calls the container
// engine; a failing `container exists` counts as "does not exist". It reports
// whether it renamed.
func Container(ctx context.Context, run func(ctx context.Context, args ...string) ([]byte, error), newName, oldName string) (bool, error) {
	if _, err := run(ctx, "container", "exists", newName); err == nil {
		return false, nil
	}
	if _, err := run(ctx, "container", "exists", oldName); err != nil {
		return false, nil
	}
	if _, err := run(ctx, "rename", oldName, newName); err != nil {
		return false, err
	}
	return true, nil
}

// NewestContainer tries Container with each old name in order (newest first)
// and stops at the first rename. It returns the name it adopted, or "".
func NewestContainer(ctx context.Context, run func(ctx context.Context, args ...string) ([]byte, error), newName string, oldNames ...string) (string, error) {
	for _, old := range oldNames {
		adopted, err := Container(ctx, run, newName, old)
		if err != nil {
			return "", err
		}
		if adopted {
			return old, nil
		}
	}
	return "", nil
}

// NewestDir tries Dir with each old path in order (newest first) and stops
// once newPath exists. It returns the path it adopted, or "".
func NewestDir(newPath string, oldPaths ...string) (string, error) {
	if _, err := os.Stat(newPath); err == nil {
		return "", nil
	}
	for _, old := range oldPaths {
		if err := Dir(newPath, old); err != nil {
			return "", err
		}
		if _, err := os.Stat(newPath); err == nil {
			return old, nil
		}
	}
	return "", nil
}
