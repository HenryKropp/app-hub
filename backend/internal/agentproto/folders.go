package agentproto

import "strings"

// FoldersOverlap tells whether two Windows folders are the same or one is
// inside the other, ignoring case, slash style and a trailing slash. Two
// apps watching overlapping folders cannot tell whose new files are whose.
func FoldersOverlap(a, b string) bool {
	a, b = folderKey(a), folderKey(b)
	if len(a) > len(b) {
		a, b = b, a
	}
	return a == b || strings.HasPrefix(b, a+"/")
}

func folderKey(p string) string {
	return strings.TrimRight(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p), `\`, "/")), "/")
}
