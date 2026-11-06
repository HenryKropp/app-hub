package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func importBody(name string) map[string]any {
	m := manifest.Manifest{Name: name, Summary: "Teste", Kind: "script", Entrypoint: "main.py", Area: "Research"}
	return map[string]any{"manifest": m, "files": map[string]manifest.InputFile{"main.py": {Name: "main.py", ContentBase64: base64.StdEncoding.EncodeToString([]byte("print('ok')"))}}}
}

func firstVersion(t *testing.T, id int64) (origin, channel string, author int64) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(), `SELECT origin, channel, author_id FROM app_versions WHERE app_id=$1 AND number=1`, id).Scan(&origin, &channel, &author); err != nil {
		t.Fatal(err)
	}
	return
}

func TestImportViaClaudeMarksChannel(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	code, raw, err := h.callAPI(withMCPPerson(context.Background(), admin.ID), "POST", "/api/scripts/import", importBody("Pelo Claude"))
	if err != nil || code != http.StatusCreated {
		t.Fatalf("%d %s %v", code, raw, err)
	}
	var out struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(raw, &out)
	if o, ch, a := firstVersion(t, out.ID); o != "original" || ch != "mcp" || a != admin.ID {
		t.Fatalf("%s %s %d", o, ch, a)
	}
	var site struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", "/api/scripts/import", importBody("Pelo site"), &site); code != http.StatusCreated {
		t.Fatal(code)
	}
	if _, ch, _ := firstVersion(t, site.ID); ch != "manual" {
		t.Fatal(ch)
	}
}

func TestImportAnswersBlockedReason(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	var before bool
	_ = testPool.QueryRow(context.Background(), `SELECT require_app_approval FROM security_settings WHERE id = TRUE`).Scan(&before)
	exec(t, `UPDATE security_settings SET require_app_approval = true WHERE id = TRUE`)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE security_settings SET require_app_approval = $1 WHERE id = TRUE`, before)
	})
	var out struct {
		ID            int64  `json:"id"`
		Version       int32  `json:"version"`
		BlockedReason string `json:"blocked_reason"`
	}
	var e map[string]any
	if code := c.do("POST", "/api/scripts/import", importBody("Precisa aprovar"), &e); code != http.StatusCreated {
		t.Fatal(code, e)
	}
	b, _ := json.Marshal(e)
	json.Unmarshal(b, &out)
	if out.Version != 1 || out.BlockedReason != "Este app está esperando a aprovação de um administrador." {
		t.Fatalf("%+v", out)
	}
}
