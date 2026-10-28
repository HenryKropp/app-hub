// Package versions keeps the history of an app's code: what enters a version,
// how a folder is compared with the last version, line diffs and restores.
// It knows nothing about HTTP or the database.
package versions

import (
	"path"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/outputs"
)

const (
	MaxCodeSize = 1 << 20
	TrashDir    = ".app-hub-lixeira"
	KindCode    = "code"
	KindBinary  = "binary"
	tempSuffix  = ".app-hub-tmp"
)

var junkDirs = map[string]bool{"__pycache__": true, ".venv": true, "venv": true, "node_modules": true, ".git": true, TrashDir: true}

// hubFiles are written by the Hub; their changes enter the history through the ficha.
var hubFiles = map[string]bool{"app-hub.yaml": true, "FICHA.md": true}

var codeExts = map[string]bool{
	".py": true, ".pyw": true, ".ipynb": true, ".r": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true,
	".tsx": true, ".jsx": true, ".go": true, ".java": true, ".cs": true, ".rb": true, ".php": true, ".sql": true,
	".bat": true, ".cmd": true, ".ps1": true, ".sh": true, ".vbs": true, ".spec": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true, ".env": true,
	".txt": true, ".md": true, ".html": true, ".htm": true, ".css": true, ".xml": true,
}

// Included reports whether a file (path relative to the app folder, forward
// slashes) belongs in a version.
func Included(rel string, outs []manifest.Output) bool {
	if strings.HasSuffix(rel, tempSuffix) {
		return false
	}
	segs := strings.Split(rel, "/")
	for _, s := range segs[:len(segs)-1] {
		if junkDirs[s] {
			return false
		}
	}
	if len(segs) == 1 && hubFiles[rel] {
		return false
	}
	idx, _ := outputs.Match(outs, rel)
	return idx < 0
}

// SecretFile reports whether a file holds passwords and keys (.env, prod.env,
// .env.local…). Its values never go into the history: they stay only in the
// app's folder. Examples (.env.example, .env.sample, .env.template) do go in.
func SecretFile(rel string) bool {
	base := strings.ToLower(path.Base(rel))
	for _, ex := range []string{".example", ".sample", ".template"} {
		if strings.HasSuffix(base, ex) {
			return false
		}
	}
	return base == ".env" || strings.HasSuffix(base, ".env") || strings.HasPrefix(base, ".env.")
}

func isCodeName(rel string) bool {
	return codeExts[strings.ToLower(path.Ext(rel))] || strings.HasPrefix(strings.ToLower(path.Base(rel)), ".env.")
}
