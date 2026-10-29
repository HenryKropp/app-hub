package brand

import "testing"

func TestNames(t *testing.T) {
	if Name != "Ziv" {
		t.Errorf("Name = %q", Name)
	}
	if AgentFile != "ZivAgente.exe" {
		t.Errorf("AgentFile = %q", AgentFile)
	}
}
