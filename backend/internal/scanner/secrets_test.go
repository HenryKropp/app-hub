package scanner

import "testing"

func TestMaskSecrets(t *testing.T) {
	in := "API_KEY = \"abc123def456ghi\"\nchave = os.getenv('X')\nclient = Anthropic(api_key=\"sk-ant-abcdefghijklmnopqrstuvwxyz\")\nprint('oi')\n"
	got := MaskSecrets(in)
	want := "API_KEY = \"••••\"\nchave = os.getenv('X')\nclient = Anthropic(api_key=\"••••\")\nprint('oi')\n"
	if got != want {
		t.Fatalf("got\n%s", got)
	}
	if MaskSecrets("token: \"${TOKEN}\"") != "token: \"${TOKEN}\"" {
		t.Fatal("placeholders stay")
	}
}
