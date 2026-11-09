package tenant

import "testing"

func TestKeyOf(t *testing.T) {
	k, err := KeyOf("tenant_0123456789abcdef01234567")
	if err != nil || k != "w0123456789a" {
		t.Fatal(k, err)
	}
	if k, err := KeyOf("public"); err != nil || k != "" {
		t.Fatal(k, err)
	}
	for _, bad := range []string{"", "tenant_xyz", "tenant_0123456789ABCDEF01234567", "other"} {
		if _, err := KeyOf(bad); err == nil {
			t.Fatal(bad)
		}
	}
}

func TestPrincipalNamesUnchanged(t *testing.T) {
	p := Principal
	if p.AppsDomain("zivapp.duckdns.org") != "zivapp.duckdns.org" || p.DataDir("/d") != "/d" || p.DBPrefix() != "app_" ||
		p.ContainerName("notas", 7, false) != "ziv-notas-7" || p.ContainerName("notas", 7, true) != "ziv-notas-teste-7" ||
		p.ImageTag("notas", 7) != "localhost/ziv/notas:7" || !p.Principal() {
		t.Fatal("principal names changed")
	}
}

func TestCompanyNames(t *testing.T) {
	c := Tenant{Slug: "teste", Key: "w0123456789a"}
	if c.AppsDomain("zivapp.duckdns.org") != "teste.zivapp.duckdns.org" || c.DataDir("/d") != "/d/workspaces/w0123456789a" ||
		c.DBPrefix() != "app_w0123456789a_" || c.ContainerName("notas", 7, false) != "ziv-w0123456789a-notas-7" ||
		c.ContainerName("notas", 7, true) != "ziv-w0123456789a-notas-teste-7" ||
		c.ImageTag("notas", 7) != "localhost/ziv/w0123456789a/notas:7" || c.Principal() {
		t.Fatal("company names")
	}
}
