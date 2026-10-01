package versions

import (
	"strings"
	"testing"
)

func render(h []Hunk) string {
	var b strings.Builder
	for i, hk := range h {
		if i > 0 {
			b.WriteString("@@\n")
		}
		for _, l := range hk.Lines {
			b.WriteString(l.Op + l.Text + "\n")
		}
	}
	return b.String()
}

func TestDiffLinesChangeWithContext(t *testing.T) {
	a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	b := "1\n2\n3\n4\nCINCO\n6\n7\n8\n9\n10\n"
	got := render(DiffLines(a, b, 2))
	want := " 3\n 4\n-5\n+CINCO\n 6\n 7\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	h := DiffLines(a, b, 2)
	if h[0].Lines[2].Old != 5 || h[0].Lines[3].New != 5 || h[0].Lines[4].Old != 6 || h[0].Lines[4].New != 6 {
		t.Fatalf("numbers %+v", h[0].Lines)
	}
}

func TestDiffLinesSeparateHunksAndCRLF(t *testing.T) {
	a := "a\r\nb\r\nc\r\nd\r\ne\r\nf\r\ng\r\nh\r\n"
	b := "A\nb\nc\nd\ne\nf\ng\nH\n"
	got := render(DiffLines(a, b, 1))
	want := "-a\n+A\n b\n@@\n g\n-h\n+H\n"
	if got != want {
		t.Fatalf("got\n%s", got)
	}
}

func TestDiffLinesAddedRemovedAndEqual(t *testing.T) {
	if got := render(DiffLines("", "x\ny\n", 3)); got != "+x\n+y\n" {
		t.Fatalf("added %q", got)
	}
	if got := render(DiffLines("x\n", "", 3)); got != "-x\n" {
		t.Fatalf("removed %q", got)
	}
	if h := DiffLines("same\n", "same\n", 3); len(h) != 0 {
		t.Fatalf("equal %v", h)
	}
}

func TestDiffLinesHugeFallsBack(t *testing.T) {
	var a, b strings.Builder
	for i := 0; i < 3000; i++ {
		a.WriteString("a" + strings.Repeat("x", i%7) + "\n")
		b.WriteString("b" + strings.Repeat("y", i%5) + "\n")
	}
	h := DiffLines(a.String(), b.String(), 3)
	minus, plus := 0, 0
	for _, hk := range h {
		for _, l := range hk.Lines {
			if l.Op == "-" {
				minus++
			} else if l.Op == "+" {
				plus++
			}
		}
	}
	if minus != 3000 || plus != 3000 {
		t.Fatalf("%d %d", minus, plus)
	}
}
