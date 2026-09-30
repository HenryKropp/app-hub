// Package outputs sorts a run's files into the groups the manifest describes.
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
		ok, err := path.Match(o.Pattern, target)
		if err != nil || !ok {
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

func literalChars(p string) int {
	n := 0
	for _, r := range p {
		if r != '*' && r != '?' && r != '[' && r != ']' {
			n++
		}
	}
	return n
}
