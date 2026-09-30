// Package outputs sorts a run's files into the groups the manifest describes.
// Patterns are globs over paths relative to the output folder; "**" stands
// for any number of folders.
package outputs

import (
	"path"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

const OtherLabel = "Outros arquivos"

// Match returns the manifest output a relative path (forward slashes) belongs
// to, or -1 and OtherLabel. The most specific matching pattern wins.
func Match(outs []manifest.Output, rel string) (int, string) {
	best, bestScore := -1, -1
	for i, o := range outs {
		target := rel
		if !strings.Contains(o.Pattern, "/") {
			target = path.Base(rel)
		}
		if !matchPath(o.Pattern, target) {
			continue
		}
		if score := literalChars(o.Pattern); score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		return -1, OtherLabel
	}
	return best, outs[best].Label
}

// matchPath matches slash-separated paths segment by segment. "*" does not
// cross "/", and a "**" segment matches any number of folders (also none).
func matchPath(pattern, rel string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func matchSegments(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegments(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	return err == nil && ok && matchSegments(pat[1:], segs[1:])
}

func literalChars(p string) int {
	n := 0
	for _, r := range p {
		if r != '*' && r != '?' && r != '[' && r != ']' {
			n++
		}
	}
	return n
}
