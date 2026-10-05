package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func detailOf(t *testing.T, c *client, id int64) appDetail {
	t.Helper()
	var d appDetail
	if code := c.do("GET", appPath(id, ""), nil, &d); code != http.StatusOK {
		t.Fatalf("get app %d", code)
	}
	return d
}

func putApp(c *client, id int64, d appDetail) (int, map[string]any) {
	var out map[string]any
	code := c.do("PUT", appPath(id, ""), map[string]any{
		"owner_id": d.OwnerID, "backup_id": d.BackupID, "manifest": d.Manifest, "updated_at": d.UpdatedAt,
	}, &out)
	return code, out
}

func TestUpdateAppOnePart(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	d := detailOf(t, c, id)
	d.Manifest.Summary = "Resumo novo"
	d.Manifest.Notes = []string{"Precisa da VPN."}
	if code, out := putApp(c, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "app-hub.yaml"))
	if !strings.Contains(string(yaml), "Resumo novo") || !strings.Contains(string(yaml), "Precisa da VPN.") {
		t.Fatalf("yaml not rewritten:\n%s", yaml)
	}
	var h struct {
		Items []historyItem `json:"items"`
	}
	c.do("GET", appPath(id, "/history?type=ficha"), nil, &h)
	if len(h.Items) < 2 {
		t.Fatalf("history should record the change: %+v", h.Items)
	}
}

func TestUpdateAppConflict(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	stale := detailOf(t, c, id)
	fresh := stale
	fresh.Manifest.Summary = "Primeiro"
	if code, _ := putApp(c, id, fresh); code != http.StatusOK {
		t.Fatal("first save")
	}
	stale.Manifest.Summary = "Segundo"
	code, out := putApp(c, id, stale)
	if code != http.StatusConflict || out["error"] != "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova." {
		t.Fatalf("conflict %d %v", code, out)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "app-hub.yaml"))
	if strings.Contains(string(yaml), "Segundo") {
		t.Fatal("stale save reached the folder")
	}
}

func TestUpdateAppPermissionAndFolder(t *testing.T) {
	admin, ana, bia, _ := team(t)
	id, dir := codeApp(t, admin)
	d := detailOf(t, admin, id)
	if code, _ := putApp(ana, id, d); code != http.StatusForbidden {
		t.Fatalf("criador on someone else's app = %d", code)
	}
	restrict(t, id)
	if code, _ := putApp(bia, id, d); code != http.StatusNotFound {
		t.Fatalf("restricted = %d", code)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	code, out := putApp(admin, id, d)
	if code != http.StatusConflict || !strings.HasPrefix(out["error"].(string), "Não consegui ler a pasta do app agora.") {
		t.Fatalf("missing folder %d %v", code, out)
	}
}

func TestUpdateAppValidation(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, _ := codeApp(t, c)
	d := detailOf(t, c, id)
	d.Manifest.DependsOn = []string{d.Manifest.ID}
	code, out := putApp(c, id, d)
	if code != http.StatusBadRequest || out["error"] != "O app não pode depender dele mesmo." {
		t.Fatalf("self dependency %d %v", code, out)
	}
}

func TestDependsLinks(t *testing.T) {
	admin, ana, _, fp := team(t) // fp: slug eq-research-files-prep
	id, _ := codeApp(t, admin)
	d := detailOf(t, admin, id)
	d.Manifest.DependsOn = []string{"eq-research-files-prep", "nao-existe"}
	if code, out := putApp(admin, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	d = detailOf(t, admin, id)
	if len(d.DependsOn) != 2 || d.DependsOn[0].Name == "" || d.DependsOn[1].ID != 0 {
		t.Fatalf("depends = %+v", d.DependsOn)
	}
	var other appDetail
	admin.do("GET", appPath(fp, ""), nil, &other)
	if len(other.UsedBy) != 1 || other.UsedBy[0].ID != id {
		t.Fatalf("used_by = %+v", other.UsedBy)
	}
	if d := detailOf(t, ana, id); d.DependsOn != nil {
		t.Fatal("someone without tech must not get the links")
	}
}

func TestUpdateAppKeepsSlug(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, _ := codeApp(t, c)
	d := detailOf(t, c, id)
	slug := d.Manifest.ID
	d.Manifest.ID = "outro-id"
	d.Manifest.DependsOn = []string{slug}
	code, out := putApp(c, id, d)
	if code != http.StatusBadRequest || out["error"] != "O app não pode depender dele mesmo." {
		t.Fatalf("disguised self dependency %d %v", code, out)
	}
	d.Manifest.DependsOn = nil
	d.Manifest.Name = "Nome novo"
	if code, out := putApp(c, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	if got := detailOf(t, c, id); got.Manifest.ID != slug {
		t.Fatalf("slug changed: %q -> %q", slug, got.Manifest.ID)
	}
}

func TestDependsLinksHideRestricted(t *testing.T) {
	admin, ana, _, fp := team(t)
	id, _ := codeApp(t, admin)
	d := detailOf(t, admin, id)
	d.Manifest.DependsOn = []string{"eq-research-files-prep"}
	if code, out := putApp(admin, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	ownerOf(t, id, 2) // Ana owns the depending app: she sees its technical part
	restrict(t, fp)   // ...but cannot see the app it depends on
	got := detailOf(t, ana, id)
	if len(got.DependsOn) != 1 || got.DependsOn[0].Visible {
		t.Fatalf("restricted target must be Visible=false: %+v", got.DependsOn)
	}
	// Reverse: Ana owns the target, the dependent app is restricted.
	ownerOf(t, fp, 2)
	exec(t, "UPDATE apps SET restricted = false WHERE id = $1", fp)
	ownerOf(t, id, 1)
	restrict(t, id)
	var other appDetail
	ana.do("GET", appPath(fp, ""), nil, &other)
	if len(other.UsedBy) != 1 || other.UsedBy[0].Visible {
		t.Fatalf("restricted dependent must be Visible=false: %+v", other.UsedBy)
	}
}
