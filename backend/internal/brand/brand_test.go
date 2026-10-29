package brand

import "testing"

func TestNames(t *testing.T) {
	if Name != "Kova" {
		t.Errorf("Name = %q", Name)
	}
	if AgentFile != "KovaAgente.exe" {
		t.Errorf("AgentFile = %q", AgentFile)
	}
}
