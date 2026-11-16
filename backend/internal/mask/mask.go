// Package mask hides vault values in text shown to people and to Claude.
package mask

import (
	"sort"
	"strings"
)

// Placeholder replaces each hidden value.
const Placeholder = "••••"

// Values swaps every value with 4 or more characters for ••••, longest first
// so a value that contains another is hidden whole. Shorter values are left:
// hiding "abc" everywhere would garble the text and protect nothing.
func Values(text string, values []string) string {
	if text == "" {
		return text
	}
	vals := make([]string, 0, len(values))
	for _, v := range values {
		if len(v) >= 4 {
			vals = append(vals, v)
		}
	}
	sort.SliceStable(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) })
	for _, v := range vals {
		text = strings.ReplaceAll(text, v, Placeholder)
	}
	return text
}
