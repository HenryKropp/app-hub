//go:build !linux

package versions

import (
	"io/fs"
	"path/filepath"
	"time"
)

func (f LocalFolder) walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error {
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
