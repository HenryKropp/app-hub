package hostpath

import "testing"

func TestToLocalAndBack(t *testing.T) {
	cases := map[string]string{
		`C:\Users\Henry\OneDrive - Alis\8. Programação`: "/mnt/c/Users/Henry/OneDrive - Alis/8. Programação",
		`"D:\apps\"`:                         "/mnt/d/apps",
		`\\wsl$\Ubuntu-24.04\home\henry\app`: "/home/henry/app",
		`\\wsl.localhost\Ubuntu\home\x`:      "/home/x",
		"/home/henry/app/":                   "/home/henry/app",
	}
	for in, want := range cases {
		if got := ToLocal(in); got != want {
			t.Errorf("ToLocal(%q) = %q, want %q", in, got, want)
		}
	}
	if got := ToDisplay("/mnt/c/Users/Henry/x"); got != `C:\Users\Henry\x` {
		t.Errorf("ToDisplay = %q", got)
	}
	if got := ToDisplay("/home/x"); got != "/home/x" {
		t.Errorf("ToDisplay = %q", got)
	}
}
