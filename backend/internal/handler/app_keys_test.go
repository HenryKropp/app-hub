package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

const keyValue = "s3nha-do-BTG-que-nunca-aparece"

// auditCount counts security history rows for an action (the table is not
// emptied between tests, so tests compare before and after).
func auditCount(t *testing.T, action string) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM security_audit WHERE action=$1", action).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func withKeyVault(t *testing.T, h *Handler) {
	t.Helper()
	v, err := vault.New(vault.NewKey())
	if err != nil {
		t.Fatal(err)
	}
	sh := h.shared
	sh.Vault = v
	h.SetShared(sh)
}

// importKeyScript registers a script whose vault declares BTG_SENHA (required) and EMAIL.
func importKeyScript(t *testing.T, c *client) int64 {
	t.Helper()
	m := manifest.Manifest{Name: "Baixador", Summary: "Baixa extratos", Kind: "script", Entrypoint: "main.py", Area: "Research",
		Keys: []manifest.Key{{Name: "BTG_SENHA", Label: "Senha do BTG", Required: true}, {Name: "EMAIL", Label: "E-mail"}}}
	var out struct {
		ID int64 `json:"id"`
	}
	body := map[string]any{"manifest": m, "files": map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte("import os\nprint(len(os.environ['BTG_SENHA']))\n"))}}}
	if code := c.do("POST", "/api/scripts/import", body, &out); code != http.StatusCreated {
		t.Fatalf("import %d", code)
	}
	t.Cleanup(func() {
		var p string
		_ = testPool.QueryRow(context.Background(), "SELECT folder_path FROM apps WHERE id=$1", out.ID).Scan(&p)
		if strings.Contains(p, "apphub-imports") {
			_ = os.RemoveAll(p)
		}
	})
	return out.ID
}

type keysOut struct {
	Keys []struct {
		Name      string `json:"name"`
		Label     string `json:"label"`
		Required  bool   `json:"required"`
		HasValue  bool   `json:"has_value"`
		UpdatedBy string `json:"updated_by"`
	} `json:"keys"`
	CanManage bool     `json:"can_manage"`
	Ready     bool     `json:"ready"`
	Missing   []string `json:"missing"`
}

func TestAppKeysFillRevealDelete(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	id := importKeyScript(t, c)
	base := "/api/apps/" + itoa(id) + "/keys"

	var got keysOut
	if code := c.do("GET", base, nil, &got); code != http.StatusOK || len(got.Keys) != 2 || !got.CanManage || got.Ready || len(got.Missing) != 1 || got.Missing[0] != "BTG_SENHA" {
		t.Fatalf("list %d %+v", code, got)
	}
	if code := c.do("PUT", base+"/BTG_SENHA", map[string]string{"value": keyValue}, nil); code != http.StatusOK {
		t.Fatalf("put %d", code)
	}
	resp := c.raw("GET", base, nil)
	raw := new(bytes.Buffer)
	_, _ = raw.ReadFrom(resp.Body)
	resp.Body.Close()
	if strings.Contains(raw.String(), keyValue) {
		t.Fatal("list leaked the value")
	}
	_ = json.Unmarshal(raw.Bytes(), &got)
	if !got.Keys[0].HasValue || !got.Ready || got.Keys[0].UpdatedBy != "Henry" {
		t.Fatalf("after put %+v", got)
	}
	var cipher []byte
	var plain *string
	if err := testPool.QueryRow(context.Background(), "SELECT value_cipher, value_plain FROM app_config WHERE app_id=$1 AND name='BTG_SENHA'", id).Scan(&cipher, &plain); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cipher, []byte(keyValue)) || (plain != nil && *plain != "") {
		t.Fatal("value not sealed")
	}
	revealed := auditCount(t, "app_key.revealed")
	var shown struct {
		Value string `json:"value"`
	}
	if code := c.do("POST", base+"/BTG_SENHA/reveal", nil, &shown); code != http.StatusOK || shown.Value != keyValue {
		t.Fatalf("reveal %d %q", code, shown.Value)
	}
	if n := auditCount(t, "app_key.revealed") - revealed; n != 1 {
		t.Fatalf("reveal audit rows %d", n)
	}
	if code := c.do("PUT", base+"/nao%20vale", map[string]string{"value": "x"}, nil); code != http.StatusBadRequest {
		t.Fatalf("bad name %d", code)
	}
	if code := c.do("PUT", base+"/EMAIL", map[string]string{"value": "a\nb"}, nil); code != http.StatusBadRequest {
		t.Fatalf("newline %d", code)
	}
	if code := c.do("DELETE", base+"/BTG_SENHA", nil, nil); code != http.StatusOK {
		t.Fatalf("delete %d", code)
	}
	c.do("GET", base, nil, &got)
	if got.Keys[0].HasValue || got.Ready {
		t.Fatalf("after delete %+v", got)
	}
}

func TestAppKeysOnlyOwnerOrAdmin(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	id := importKeyScript(t, c)
	base := "/api/apps/" + itoa(id) + "/keys"
	c.do("PUT", base+"/BTG_SENHA", map[string]string{"value": keyValue}, nil)

	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	var got keysOut
	if code := ana.do("GET", base, nil, &got); code != http.StatusOK || got.CanManage || !got.Keys[0].HasValue {
		t.Fatalf("user list %d %+v", code, got)
	}
	if code := ana.do("POST", base+"/BTG_SENHA/reveal", nil, nil); code != http.StatusForbidden {
		t.Fatalf("user reveal %d", code)
	}
	if code := ana.do("PUT", base+"/BTG_SENHA", map[string]string{"value": "outra"}, nil); code != http.StatusForbidden {
		t.Fatalf("user put %d", code)
	}
	if code := ana.do("DELETE", base+"/BTG_SENHA", nil, nil); code != http.StatusForbidden {
		t.Fatalf("user delete %d", code)
	}
}

func TestAppKeysNoVault(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importKeyScript(t, c)
	var out map[string]string
	if code := c.do("PUT", "/api/apps/"+itoa(id)+"/keys/BTG_SENHA", map[string]string{"value": keyValue}, &out); code != http.StatusServiceUnavailable || !strings.Contains(out["error"], "cofre") {
		t.Fatalf("no vault %d %v", code, out)
	}
}

func TestAgentGetsKeysOncePerRun(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	id := importKeyScript(t, c)
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	body := map[string]any{"output_folder": `C:\Resultados`}
	var refused map[string]string
	if code := c.do("POST", appPath(id, "/runs"), body, &refused); code != http.StatusConflict || !strings.Contains(refused["error"], "BTG_SENHA") {
		t.Fatalf("missing key %d %v", code, refused)
	}
	c.do("PUT", "/api/apps/"+itoa(id)+"/keys/BTG_SENHA", map[string]string{"value": keyValue}, nil)
	if code := c.do("POST", appPath(id, "/runs"), body, nil); code != http.StatusCreated {
		t.Fatalf("run %d", code)
	}
	keysPath := func(jobID int64) string { return "/api/agent/jobs/" + itoa(jobID) + "/keys" }
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	if bytes.Contains(job.Payload, []byte(keyValue)) {
		t.Fatal("key in the job payload")
	}
	var p agentproto.RunPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || !p.Keys {
		t.Fatalf("payload must flag keys: %+v %v", p, err)
	}
	other := pairComputer(t, c, "OUTRO")
	if code := c.agent("GET", keysPath(job.ID), other, nil, nil); code != http.StatusNotFound {
		t.Fatalf("other computer %d", code)
	}
	delivered := auditCount(t, "app_key.delivered")
	var keys map[string]string
	if code := c.agent("GET", keysPath(job.ID), token, nil, &keys); code != http.StatusOK || keys["BTG_SENHA"] != keyValue || len(keys) != 1 {
		t.Fatalf("keys %d %v", code, keys)
	}
	if code := c.agent("GET", keysPath(job.ID), token, nil, nil); code != http.StatusNotFound {
		t.Fatalf("second fetch %d", code)
	}
	if n := auditCount(t, "app_key.delivered") - delivered; n != 1 {
		t.Fatalf("delivery audit %d", n)
	}
}

// A key added on the site (not in the ficha) joins the vault, goes to the
// run and leaves the list when deleted.
func TestAppKeysAddedOnSite(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	id := importKeyScript(t, c)
	base := "/api/apps/" + itoa(id) + "/keys"

	if code := c.do("PUT", base+"/API_TOKEN", map[string]string{"value": keyValue}, nil); code != http.StatusOK {
		t.Fatalf("add %d", code)
	}
	for _, bad := range []string{"PATH", "PYTHONPATH", "token"} {
		if code := c.do("PUT", base+"/"+bad, map[string]string{"value": "x"}, nil); code != http.StatusBadRequest {
			t.Fatalf("%s %d", bad, code)
		}
	}
	var got keysOut
	c.do("GET", base, nil, &got)
	if len(got.Keys) != 3 || got.Keys[2].Name != "API_TOKEN" || !got.Keys[2].HasValue || got.Keys[2].Required {
		t.Fatalf("list %+v", got)
	}
	a, err := h.q.GetApp(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	values, err := h.keyValues(context.Background(), a)
	if err != nil || values["API_TOKEN"] != keyValue {
		t.Fatalf("run values %v %v", err, values)
	}
	var shown struct {
		Value string `json:"value"`
	}
	if code := c.do("POST", base+"/API_TOKEN/reveal", nil, &shown); code != http.StatusOK || shown.Value != keyValue {
		t.Fatalf("reveal %d", code)
	}
	if code := c.do("DELETE", base+"/API_TOKEN", nil, nil); code != http.StatusOK {
		t.Fatalf("delete %d", code)
	}
	c.do("GET", base, nil, &got)
	if len(got.Keys) != 2 {
		t.Fatalf("after delete %+v", got)
	}
	if code := c.do("DELETE", base+"/API_TOKEN", nil, nil); code != http.StatusNotFound {
		t.Fatalf("delete again %d", code)
	}
}

// The vault caps keys, declared plus added, at manifest.MaxKeys.
func TestAppKeysAddedCap(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withKeyVault(t, h)
	id := importKeyScript(t, c) // 2 declared
	base := "/api/apps/" + itoa(id) + "/keys"
	for i := 0; i < manifest.MaxKeys-2; i++ {
		if code := c.do("PUT", base+"/EXTRA_"+itoa(int64(i)), map[string]string{"value": "x"}, nil); code != http.StatusOK {
			t.Fatalf("add %d: %d", i, code)
		}
	}
	if code := c.do("PUT", base+"/EXTRA_X", map[string]string{"value": "x"}, nil); code != http.StatusBadRequest {
		t.Fatalf("over cap %d", code)
	}
	if code := c.do("PUT", base+"/EXTRA_0", map[string]string{"value": "y"}, nil); code != http.StatusOK {
		t.Fatalf("replace at cap %d", code)
	}
}
