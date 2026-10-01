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

func TestMaskUnquotedSecrets(t *testing.T) {
	cases := map[string]string{
		"DB_PASSWORD=Sup3rSecret99":          "DB_PASSWORD=••••",
		"api_key: abcd1234efgh":              "api_key: ••••",
		"export TOKEN=abc123def456":          "export TOKEN=••••",
		"  client_secret = s3cr3tValue  # x": "  client_secret = ••••  # x",
		"- token: abcd1234":                  "- token: ••••",
		"PASSWORD=abc":                       "PASSWORD=abc", // too short
		"PASSWORD=":                          "PASSWORD=",
		"HOST=db.local":                      "HOST=db.local", // not a secret name
		"password = os.getenv('PW')":         "password = os.getenv('PW')",
		"token = get_token()":                "token = get_token()",
	}
	for in, want := range cases {
		if got := MaskSecrets(in); got != want {
			t.Errorf("MaskSecrets(%q) = %q, want %q", in, got, want)
		}
	}
	// Quoted values keep the quotes, as before.
	if got := MaskSecrets(`API_KEY = "abc123def456ghi"`); got != `API_KEY = "••••"` {
		t.Errorf("quoted %q", got)
	}
}

func TestMaskSecretsInEnvFiles(t *testing.T) {
	text := "# banco\nHOST=db.local\nexport PORT=5432\nX=ab\nDB_PASSWORD=Sup3rSecret99\n"
	want := "# banco\nHOST=••••\nexport PORT=••••\nX=ab\nDB_PASSWORD=••••\n"
	for _, p := range []string{".env", "config/.env", "prod.env", ".env.local", "sub/.env.production"} {
		if got := MaskSecretsIn(p, text); got != want {
			t.Errorf("%s: got %q", p, got)
		}
	}
	for _, p := range []string{"settings.ini", "environment.py", "dotenv.md"} {
		if got := MaskSecretsIn(p, "HOST=db.local\n"); got != "HOST=db.local\n" {
			t.Errorf("%s must not use the .env rule: %q", p, got)
		}
	}
	if got := MaskSecretsIn("settings.ini", "DB_PASSWORD=Sup3rSecret99"); got != "DB_PASSWORD=••••" {
		t.Errorf("secret names are masked everywhere: %q", got)
	}
}
