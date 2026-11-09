package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type settingsOut struct {
	Items []struct {
		Name     string `json:"name"`
		Label    string `json:"label"`
		Secret   bool   `json:"secret"`
		Required bool   `json:"required"`
		HasValue bool   `json:"has_value"`
		Value    string `json:"value"`
	} `json:"items"`
	CanManage bool     `json:"can_manage"`
	Locked    bool     `json:"locked"`
	Missing   []string `json:"missing"`
}

// Configurações (ADR 032): one list with the secrets and the plain values;
// plain values are saved, shown to who manages, and reach the computer.
func TestAppSettingsListAndPlainValues(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	id := webWithSettings(t, c)
	base := "/api/apps/" + itoa(id) + "/settings"

	var got settingsOut
	if code := c.do("GET", base, nil, &got); code != http.StatusOK || len(got.Items) != 2 || !got.CanManage ||
		got.Items[0].Name != "SENHA_EMAIL" || !got.Items[0].Secret || !got.Items[0].Required ||
		got.Items[1].Name != "MODO" || got.Items[1].Secret || len(got.Missing) != 1 || got.Missing[0] != "Senha do e-mail" {
		t.Fatalf("list %d %+v", code, got)
	}
	var detail struct {
		MissingSettings []string `json:"missing_settings"`
	}
	if code := c.do("GET", "/api/apps/"+itoa(id), nil, &detail); code != http.StatusOK || len(detail.MissingSettings) != 1 {
		t.Fatalf("detail %d %+v", code, detail)
	}

	saved := auditCount(t, "app_setting.saved")
	for _, name := range []string{"MODO", "PASTA_SAIDA"} {
		if code := c.do("PUT", base+"/"+name, map[string]string{"value": `C:\Relatórios`}, nil); code != http.StatusOK {
			t.Fatalf("put %s %d", name, code)
		}
	}
	if n := auditCount(t, "app_setting.saved") - saved; n != 2 {
		t.Fatalf("audit %d", n)
	}
	var e map[string]string
	if code := c.do("PUT", base+"/SENHA_EMAIL", map[string]string{"value": "x"}, &e); code != http.StatusBadRequest || !strings.Contains(e["error"], "senha") {
		t.Fatalf("secret as plain %d %v", code, e)
	}
	for _, bad := range []string{"PORT", "PATH", "nao_vale"} {
		if code := c.do("PUT", base+"/"+bad, map[string]string{"value": "x"}, nil); code != http.StatusBadRequest {
			t.Fatalf("%s %d", bad, code)
		}
	}
	if code := c.do("PUT", base+"/MODO", map[string]string{"value": "a\nb"}, nil); code != http.StatusBadRequest {
		t.Fatalf("two lines %d", code)
	}
	c.do("GET", base, nil, &got)
	if len(got.Items) != 3 || got.Items[1].Value != `C:\Relatórios` || got.Items[2].Name != "PASTA_SAIDA" || !got.Items[2].HasValue {
		t.Fatalf("after put %+v", got)
	}

	a, err := h.q.GetApp(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	values, err := h.keyValues(context.Background(), a)
	if err != nil || values["MODO"] != `C:\Relatórios` || values["PASTA_SAIDA"] != `C:\Relatórios` {
		t.Fatalf("run values %v %v", values, err)
	}

	if code := c.do("DELETE", base+"/PASTA_SAIDA", nil, nil); code != http.StatusOK {
		t.Fatalf("delete %d", code)
	}
	if code := c.do("DELETE", base+"/PASTA_SAIDA", nil, nil); code != http.StatusNotFound {
		t.Fatalf("delete again %d", code)
	}
	if code := c.do("DELETE", base+"/SENHA_EMAIL", nil, nil); code != http.StatusNotFound {
		t.Fatalf("delete a secret here %d", code)
	}
	c.do("GET", base, nil, &got)
	if len(got.Items) != 2 {
		t.Fatalf("after delete %+v", got)
	}
}

// Who only sees the app sees the names and the state, never the values, and
// changes nothing.
func TestAppSettingsOnlyOwnerOrAdmin(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	id := importKeyScript(t, c)
	base := "/api/apps/" + itoa(id) + "/settings"
	if code := c.do("PUT", base+"/PASTA", map[string]string{"value": "C:\\Dados"}, nil); code != http.StatusOK {
		t.Fatalf("put %d", code)
	}

	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	var got settingsOut
	if code := ana.do("GET", base, nil, &got); code != http.StatusOK || got.CanManage || len(got.Items) != 3 {
		t.Fatalf("user list %d %+v", code, got)
	}
	for _, it := range got.Items {
		if it.Value != "" {
			t.Fatalf("user sees a value %+v", it)
		}
	}
	if code := ana.do("PUT", base+"/PASTA", map[string]string{"value": "outra"}, nil); code != http.StatusForbidden {
		t.Fatalf("user put %d", code)
	}
	if code := ana.do("DELETE", base+"/PASTA", nil, nil); code != http.StatusForbidden {
		t.Fatalf("user delete %d", code)
	}
}

// A plain value needs no server key: it reaches the run even with the
// secrets locked, as long as no secret is filled.
func TestPlainValuesWithoutServerKey(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	id := importKeyScript(t, c)
	if code := c.do("PUT", "/api/apps/"+itoa(id)+"/settings/PASTA", map[string]string{"value": "C:\\Dados"}, nil); code != http.StatusOK {
		t.Fatalf("put %d", code)
	}
	sh := h.shared
	sh.Vault = nil
	h.SetShared(sh)
	a, err := h.q.GetApp(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	values, err := h.keyValues(context.Background(), a)
	if err != nil || values["PASTA"] != "C:\\Dados" || len(values) != 1 {
		t.Fatalf("values %v %v", values, err)
	}
	var got settingsOut
	if code := c.do("GET", "/api/apps/"+itoa(id)+"/settings", nil, &got); code != http.StatusOK || !got.Locked {
		t.Fatalf("list %d %+v", code, got)
	}
}
