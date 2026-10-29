package agentproto

import "testing"

func TestFoldersOverlap(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{`C:\Dados`, `c:\dados\`, true},
		{`C:\Dados`, `C:\Dados\Março`, true},
		{`C:\Dados\Março`, `C:\Dados`, true},
		{`C:\Dados`, `C:\Dados2`, false},
		{`C:\Dados`, `D:\Dados`, false},
		{"/tmp/x", "/tmp/x/y", true},
	} {
		if got := FoldersOverlap(c.a, c.b); got != c.want {
			t.Errorf("%q %q: %v", c.a, c.b, got)
		}
	}
}
