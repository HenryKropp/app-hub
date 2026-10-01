package versions

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// abs joins rel to Root and refuses anything that is absolute or escapes Root.
func (f LocalFolder) abs(rel string) (string, error) {
	r := filepath.FromSlash(rel)
	if filepath.IsAbs(r) || strings.HasPrefix(r, string(filepath.Separator)) {
		return "", errors.New("caminho fora da pasta do app")
	}
	p := filepath.Join(f.Root, r)
	back, err := filepath.Rel(f.Root, p)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(filepath.Separator)) {
		return "", errors.New("caminho fora da pasta do app")
	}
	return p, nil
}

func (f LocalFolder) Walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error {
	if info, err := os.Stat(f.Root); err != nil || !info.IsDir() {
		return ErrFolderMissing
	}
	return f.walk(fn)
}

func (f LocalFolder) Open(rel string) (io.ReadCloser, error) {
	p, err := f.abs(rel)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (f LocalFolder) Write(rel string, data []byte) error {
	p, err := f.abs(rel)
	if err != nil {
		return err
	}
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
	dst, err := f.abs(to)
	if err != nil {
		return err
	}
	src, err := f.abs(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}
