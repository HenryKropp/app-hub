package readiness

import (
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// MCP F2: which settings the code reads, and whether it needs them.
//
// Only a name written between quotes counts (os.environ["X"], os.getenv("X"),
// os.environ.get("X")): a constant (os.environ.get(VAR_DEMO)) is not a key.
// A read with a default (os.getenv("X", "a"), os.environ.get("X", ""),
// process.env.X || "a") is optional; the others are required.
var (
	pyEnvRe = regexp.MustCompile(`os\.(?:getenv|environ\.get)\(\s*["']([A-Z][A-Z0-9_]*)["']\s*(,|\))|os\.environ\[\s*["']([A-Z][A-Z0-9_]*)["']\s*\]`)
	jsEnvRe = regexp.MustCompile(`process\.env(?:\.([A-Z][A-Z0-9_]*)|\[\s*["']([A-Z][A-Z0-9_]*)["']\s*\])(\s*(?:\|\||\?\?))?`)
)

// envReadsIn adds to reads the names one line reads; a name is required when
// any read of it has no default.
func envReadsIn(line string, reads map[string]bool) {
	for _, ix := range pyEnvRe.FindAllStringSubmatchIndex(line, -1) {
		switch {
		case ix[2] >= 0:
			name := line[ix[2]:ix[3]]
			reads[name] = reads[name] || line[ix[4]:ix[5]] == ")" // ")" = no default
		case ix[6] >= 0:
			if assigns(line[ix[1]:]) { // os.environ["X"] = … sets it, does not read it
				continue
			}
			reads[line[ix[6]:ix[7]]] = true
		}
	}
	for _, m := range jsEnvRe.FindAllStringSubmatch(line, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		reads[name] = reads[name] || m[3] == ""
	}
}

// assigns: the text after os.environ["X"] makes it an assignment.
func assigns(rest string) bool {
	rest = strings.TrimLeft(rest, " \t")
	return strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==")
}

// EnvReads lists the settings an app's code reads: name → required.
func EnvReads(src versions.Folder) (map[string]bool, error) {
	reads := map[string]bool{}
	err := src.Walk(func(rel string, dir bool, _ int64, _ time.Time) error {
		if dir {
			if skipDir(rel) {
				return fs.SkipDir
			}
			return nil
		}
		if !codeExt[path.Ext(rel)] {
			return nil
		}
		f, err := src.Open(rel)
		if err != nil {
			return nil
		}
		b, _ := io.ReadAll(io.LimitReader(f, 1<<20))
		f.Close()
		for _, line := range strings.Split(string(b), "\n") {
			envReadsIn(line, reads)
		}
		return nil
	})
	return reads, err
}

func skipDir(rel string) bool {
	switch path.Base(rel) {
	case ".git", "node_modules", ".venv", "venv", "__pycache__":
		return true
	}
	return false
}
