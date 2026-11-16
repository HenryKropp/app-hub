package mask

import "testing"

func TestMaskValues(t *testing.T) {
	got := Values("abc xsegredo123y segredo", []string{"abc", "segredo123", "segredo"})
	if got != "abc x••••y ••••" {
		t.Errorf("got %q", got)
	}
	if Values("", []string{"segredo"}) != "" {
		t.Error("empty stays empty")
	}
}
