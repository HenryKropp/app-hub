package versions

import "strings"

// Line is one line of a diff: " " kept, "-" removed, "+" added. Old and New
// are 1-based line numbers, 0 when the line does not exist on that side.
type Line struct {
	Op   string `json:"op"`
	Text string `json:"text"`
	Old  int    `json:"old"`
	New  int    `json:"new"`
}

type Hunk struct {
	Lines []Line `json:"lines"`
}

// maxCells bounds the LCS table (int32 cells, ~16 MB). Beyond it the changed
// block is shown as fully removed and fully added.
const maxCells = 4_000_000

// DiffLines compares two texts line by line and returns the changes with
// `context` unchanged lines around them, grouped into hunks.
func DiffLines(a, b string, context int) []Hunk {
	all := diffOps(splitLines(a), splitLines(b))
	keep := make([]bool, len(all))
	for i, l := range all {
		if l.Op == " " {
			continue
		}
		for k := max(0, i-context); k <= min(len(all)-1, i+context); k++ {
			keep[k] = true
		}
	}
	var hunks []Hunk
	open := false
	for i, l := range all {
		if !keep[i] {
			open = false
			continue
		}
		if !open {
			hunks = append(hunks, Hunk{})
			open = true
		}
		last := len(hunks) - 1
		hunks[last].Lines = append(hunks[last].Lines, l)
	}
	return hunks
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func diffOps(a, b []string) []Line {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	var out []Line
	for i := 0; i < pre; i++ {
		out = append(out, Line{Op: " ", Text: a[i], Old: i + 1, New: i + 1})
	}
	out = append(out, middleOps(a[pre:len(a)-suf], b[pre:len(b)-suf], pre)...)
	for i := 0; i < suf; i++ {
		ai, bi := len(a)-suf+i, len(b)-suf+i
		out = append(out, Line{Op: " ", Text: a[ai], Old: ai + 1, New: bi + 1})
	}
	return out
}

func middleOps(a, b []string, off int) []Line {
	n, m := len(a), len(b)
	var out []Line
	if n*m > maxCells {
		for i, s := range a {
			out = append(out, Line{Op: "-", Text: s, Old: off + i + 1})
		}
		for j, s := range b {
			out = append(out, Line{Op: "+", Text: s, New: off + j + 1})
		}
		return out
	}
	// lcs[i][j] is the longest common subsequence of a[i:] and b[j:].
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			out = append(out, Line{Op: " ", Text: a[i], Old: off + i + 1, New: off + j + 1})
			i++
			j++
		case i < n && (j == m || lcs[i+1][j] >= lcs[i][j+1]):
			out = append(out, Line{Op: "-", Text: a[i], Old: off + i + 1})
			i++
		default:
			out = append(out, Line{Op: "+", Text: b[j], New: off + j + 1})
			j++
		}
	}
	return out
}
