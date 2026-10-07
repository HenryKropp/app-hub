// Package stage copies an app folder into the Hub's build area, so builds
// never touch (or write into) the app folder itself.
package stage

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

var ErrTooBig = errors.New("a pasta do app passa de 500 MB")

const maxBytes = 500 << 20

func Copy(src versions.Folder, dir string, outs []manifest.Output, rec recipes.Recipe) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	ignore := readIgnore(src)
	var total int64
	err := src.Walk(func(rel string, isDir bool, size int64, _ time.Time) error {
		if path.Base(rel) == ".git" || ignored(ignore, rel, isDir) {
			if isDir {
				return fs.SkipDir
			}
			return nil
		}
		if isDir {
			return nil
		}
		if !versions.Included(rel, outs) {
			return nil
		}
		if total += size; total > maxBytes {
			return ErrTooBig
		}
		r, err := src.Open(rel)
		if err != nil {
			return err
		}
		defer r.Close()
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if f, ok := r.(*os.File); ok { // keep the executable bit (start.sh)
			if st, err := f.Stat(); err == nil && st.Mode().Perm()&0o111 != 0 {
				mode = 0o755
			}
		}
		w, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
			w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return os.Chmod(dst, mode) // the umask may have taken bits off
	})
	if err != nil {
		return "", err
	}
	for name, body := range rec.ExtraFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			return "", err
		}
	}
	if rec.Kind == "dockerfile" {
		return "Dockerfile", nil
	}
	return recipes.DockerfileName, os.WriteFile(filepath.Join(dir, recipes.DockerfileName), []byte(rec.Dockerfile), 0o600)
}

func readIgnore(src versions.Folder) []string {
	r, err := src.Open(".dockerignore")
	if err != nil {
		return nil
	}
	defer r.Close()
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "!") {
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(l, "/"), "/"))
		}
	}
	return out
}

// ignored is a simple .dockerignore: a pattern matches the path, its base
// name, or any parent folder. Negations ("!") are not supported.
func ignored(patterns []string, rel string, isDir bool) bool {
	for _, p := range patterns {
		if ok, _ := path.Match(p, rel); ok {
			return true
		}
		if ok, _ := path.Match(p, path.Base(rel)); ok {
			return true
		}
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}
