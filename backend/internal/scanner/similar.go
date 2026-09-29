package scanner

import (
	"regexp"
	"strings"
)

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

var stopWords = map[string]bool{
	"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true, "a": true, "o": true, "the": true,
	"para": true, "em": true, "com": true, "app": true, "python": true, "v1": true, "v2": true, "v3": true, "files": true,
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(foldAccents(s)), -1) {
		if stopWords[w] || len(w) < 2 || strings.Trim(w, "0123456789") == "" {
			continue
		}
		out[w] = true
	}
	return out
}

// Similarity compares two app names (and optional summaries) by shared words, 0..1.
func Similarity(a, b string) float64 {
	ta, tb := tokens(a), tokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	shared := 0
	for w := range ta {
		if tb[w] {
			shared++
		}
	}
	smaller := len(ta)
	if len(tb) < smaller {
		smaller = len(tb)
	}
	return float64(shared) / float64(smaller)
}

// LooksDuplicate is the rule used for the "app parecido" warning.
func LooksDuplicate(a, b string) bool {
	return Similarity(a, b) >= 0.66
}
