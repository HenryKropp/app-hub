package versions

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var ErrFolderMissing = errors.New("a pasta do app não foi encontrada ou não pode ser lida")

// Folder is how versions reach an app folder. Today the Hub reads the disk
// directly (WSL); when the Hub moves to a central server, the Agent will
// implement the same interface. Paths are relative, with forward slashes.
type Folder interface {
	// Walk calls fn for every directory (dir=true) and regular file. fn may
	// return fs.SkipDir for a directory. Any read error aborts the walk.
	Walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error
	Open(rel string) (io.ReadCloser, error)
	// Write replaces a file atomically, creating parent folders.
	Write(rel string, data []byte) error
	// Move renames a file, creating the destination's parent folders.
	Move(from, to string) error
}

type LocalFolder struct{ Root string }

func (f LocalFolder) abs(rel string) string { return filepath.Join(f.Root, filepath.FromSlash(rel)) }

func (f LocalFolder) Walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error {
	if info, err := os.Stat(f.Root); err != nil || !info.IsDir() {
		return ErrFolderMissing
	}
	return filepath.WalkDir(f.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == f.Root {
			return nil
		}
		rel, err := filepath.Rel(f.Root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			return fn(rel, true, 0, time.Time{})
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return fn(rel, false, info.Size(), info.ModTime())
	})
}

func (f LocalFolder) Open(rel string) (io.ReadCloser, error) { return os.Open(f.abs(rel)) }

func (f LocalFolder) Write(rel string, data []byte) error {
	p := f.abs(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + tempSuffix
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (f LocalFolder) Move(from, to string) error {
	dst := f.abs(to)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(f.abs(from), dst)
}
