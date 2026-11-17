package risk

import (
	"reflect"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func TestOfReadsKeysDomainsBrowser(t *testing.T) {
	m := manifest.Manifest{Keys: []manifest.Key{{Name: "BTG_SENHA"}}}
	files := map[string][]byte{
		"main.py": []byte("from playwright.sync_api import sync_playwright\nrequests.get(\"https://research.btgpactual.com/x\")\n"),
		"logo.png": []byte("https://ignored.example.org"),
	}
	s := Of(m, files)
	if !reflect.DeepEqual(s.Keys, []string{"BTG_SENHA"}) || !reflect.DeepEqual(s.Domains, []string{"research.btgpactual.com"}) || !s.Browser {
		t.Fatalf("Of = %+v", s)
	}
}

func TestOfReadsSharedSecretsAndAlwaysOn(t *testing.T) {
	m := manifest.Manifest{Shared: &manifest.Shared{Enabled: true, Env: []manifest.EnvVar{{Name: "API_TOKEN", Secret: true}, {Name: "PAGE_SIZE"}}}}
	s := Of(m, nil)
	if !reflect.DeepEqual(s.Keys, []string{"API_TOKEN"}) || !s.AlwaysOn {
		t.Fatalf("Of = %+v", s)
	}
}

func TestOfIgnoresLocalhost(t *testing.T) {
	files := map[string][]byte{"app.js": []byte(`fetch("http://localhost:8080/x"); fetch("http://127.0.0.1/y"); x="https://www.w3.org/2000/svg"; y="https://example.com"`)}
	if s := Of(manifest.Manifest{}, files); len(s.Domains) != 0 {
		t.Fatalf("domains = %v", s.Domains)
	}
}

func TestDomainsAreLowercaseAndUnique(t *testing.T) {
	files := map[string][]byte{"a.py": []byte(`"https://API.XP.com/a" "https://api.xp.com/b"`), "b.yaml": []byte("url: https://b3.com.br")}
	if s := Of(manifest.Manifest{}, files); !reflect.DeepEqual(s.Domains, []string{"api.xp.com", "b3.com.br"}) {
		t.Fatalf("domains = %v", s.Domains)
	}
}

func TestAddedListsOnlyNew(t *testing.T) {
	prev := Set{Domains: []string{"a.com", "b.com"}}
	now := Set{Domains: []string{"a.com", "xp.com"}}
	if got := now.Added(prev); !reflect.DeepEqual(got, []string{"site: xp.com"}) {
		t.Fatalf("Added = %v", got)
	}
}

func TestAddedWording(t *testing.T) {
	now := Set{Keys: []string{"XP_SENHA"}, Browser: true, AlwaysOn: true, SendsOut: true}
	want := []string{"chave do Cofre: XP_SENHA", "usa navegador", "roda 24h no servidor", "manda coisas para fora"}
	if got := now.Added(Set{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Added = %v", got)
	}
}

func TestMergeKeepsCodeRisk(t *testing.T) {
	prev := Set{Keys: []string{"A"}, Domains: []string{"xp.com"}, Browser: true, SendsOut: true}
	ficha := Set{Keys: []string{"A", "B"}, AlwaysOn: true}
	got := Merge(ficha, prev)
	want := Set{Keys: []string{"A", "B"}, Domains: []string{"xp.com"}, Browser: true, AlwaysOn: true, SendsOut: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge = %+v", got)
	}
}

func TestSendsOutSmtplib(t *testing.T) {
	if !Of(manifest.Manifest{}, map[string][]byte{"m.py": []byte("import smtplib\n")}).SendsOut {
		t.Fatal("smtplib not detected")
	}
	if Of(manifest.Manifest{}, map[string][]byte{"m.py": []byte("import os\n")}).SendsOut {
		t.Fatal("false positive")
	}
}
