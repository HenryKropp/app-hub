package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/jackc/pgx/v5/pgtype"
)

type draftOut struct {
	DraftID       int64  `json:"draft_id"`
	ID            int64  `json:"id"`
	Status        string `json:"status"`
	StatusReason  string `json:"status_reason"`
	VersionNumber *int32 `json:"version_number"`
	Note          string `json:"note"`
	Version       int32  `json:"version"`
	Code          string `json:"code"`
	Error         string `json:"error"`
}

func sendDraft(t *testing.T, c *client, id int64, files map[string]string, extra map[string]any) (int, draftOut) {
	t.Helper()
	body := map[string]any{"files": b64files(files)}
	for k, v := range extra {
		body[k] = v
	}
	var out draftOut
	code := c.do("POST", appPath(id, "/drafts"), body, &out)
	return code, out
}

func publishDraftReq(c *client, id, draft int64) (int, draftOut) {
	var out draftOut
	code := c.do("POST", appPath(id, "/drafts/"+itoa(draft)+"/publish"), map[string]any{}, &out)
	return code, out
}

func getDraft(t *testing.T, c *client, id, draft int64) draftOut {
	t.Helper()
	var out draftOut
	if code := c.do("GET", appPath(id, "/drafts/"+itoa(draft)), nil, &out); code != http.StatusOK {
		t.Fatalf("get draft %d %+v", code, out)
	}
	return out
}

func appManifestRaw(t *testing.T, id int64) string {
	t.Helper()
	var m string
	if err := testPool.QueryRow(context.Background(), "SELECT manifest::text FROM apps WHERE id=$1", id).Scan(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDraftDoesNotChangeLiveApp(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	before := appManifestRaw(t, id)
	code, out := sendDraft(t, c, id, map[string]string{"main.py": "print('v2')"},
		map[string]any{"ficha": map[string]any{"summary": "Outro resumo"}})
	if code != http.StatusCreated || out.DraftID == 0 || out.Status != "rascunho" {
		t.Fatalf("%d %+v", code, out)
	}
	if got := storedNames(t, id)["main.py"]; got != "print('v1')" {
		t.Fatal("stored code changed:", got)
	}
	data, err := os.ReadFile(filepath.Join(appFolder(t, id), "main.py"))
	if err != nil || string(data) != "print('v1')" {
		t.Fatalf("folder changed: %q %v", data, err)
	}
	if n := versionCount(t, id); n != 1 {
		t.Fatal("versions:", n)
	}
	if after := appManifestRaw(t, id); after != before {
		t.Fatal("manifest changed")
	}
}

func TestDraftAcceptedDuringRun(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	token := pairComputer(t, c, "PC")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	if code := c.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\R`, "inputs": map[string]string{"amount": "2"}}, nil); code != http.StatusCreated {
		t.Fatal(code)
	}
	code, out := sendDraft(t, c, id, map[string]string{"main.py": "print(2)"}, nil)
	if code != http.StatusCreated {
		t.Fatalf("%d %+v", code, out)
	}
	pc, pout := publishDraftReq(c, id, out.DraftID)
	if pc != http.StatusConflict || !strings.Contains(pout.Error, "Espere a execução terminar") {
		t.Fatalf("%d %+v", pc, pout)
	}
	if storedNames(t, id)["main.py"] != "print(1)" {
		t.Fatal("sources changed")
	}
	if d := getDraft(t, c, id, out.DraftID); d.Status != "rascunho" {
		t.Fatal("draft status:", d.Status)
	}
}

func TestDraftPublishMakesVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print('v1')"})
	_, out := sendDraft(t, c, id, map[string]string{"main.py": "print('v2')"}, map[string]any{"note": "Corrige"})
	code, pout := publishDraftReq(c, id, out.DraftID)
	if code != http.StatusOK || pout.Version != 2 {
		t.Fatalf("%d %+v", code, pout)
	}
	if n := versionCount(t, id); n != 2 {
		t.Fatal("versions:", n)
	}
	var origin, desc string
	var tested pgtype.Bool
	testPool.QueryRow(context.Background(), "SELECT origin, description, tested FROM app_versions WHERE app_id=$1 AND number=2", id).Scan(&origin, &desc, &tested)
	if origin != "upload" || desc != "Corrige" || !tested.Valid || tested.Bool {
		t.Fatal(origin, desc, tested)
	}
	if storedNames(t, id)["main.py"] != "print('v2')" {
		t.Fatal("not live")
	}
	d := getDraft(t, c, id, out.DraftID)
	if d.Status != "no_ar" || d.VersionNumber == nil || *d.VersionNumber != 2 || d.Note != "Corrige" {
		t.Fatalf("%+v", d)
	}
	if code, again := publishDraftReq(c, id, out.DraftID); code != http.StatusConflict || again.Code != "draft_closed" {
		t.Fatalf("%d %+v", code, again)
	}
}

func TestNewDraftCancelsOpenOne(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	_, a := sendDraft(t, c, id, map[string]string{"main.py": "print('a')"}, nil)
	_, b := sendDraft(t, c, id, map[string]string{"main.py": "print('b')"}, nil)
	if a.DraftID == b.DraftID {
		t.Fatal("same draft")
	}
	da := getDraft(t, c, id, a.DraftID)
	if da.Status != "cancelada" || da.StatusReason != "substituída por versão mais nova" {
		t.Fatalf("%+v", da)
	}
	if db := getDraft(t, c, id, b.DraftID); db.Status != "rascunho" {
		t.Fatalf("%+v", db)
	}
	if code, out := publishDraftReq(c, id, a.DraftID); code != http.StatusConflict || out.Code != "draft_closed" {
		t.Fatalf("%d %+v", code, out)
	}
}

func TestDraftFichaAppliesOverCurrent(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	_, out := sendDraft(t, c, id, map[string]string{"main.py": "print(2)"},
		map[string]any{"ficha": map[string]any{"summary": "Novo resumo"}})
	// Someone edits another part of the ficha on the site before publishing.
	d := detailOf(t, c, id)
	d.Manifest.Notes = []string{"Precisa da VPN."}
	if code, res := putApp(c, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, res)
	}
	if code, pout := publishDraftReq(c, id, out.DraftID); code != http.StatusOK {
		t.Fatalf("%d %+v", code, pout)
	}
	after := detailOf(t, c, id)
	if after.Manifest.Summary != "Novo resumo" || len(after.Manifest.Notes) != 1 || after.Manifest.Notes[0] != "Precisa da VPN." {
		t.Fatalf("%+v", after.Manifest)
	}
}

func TestDraftFolderAppRefused(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := insertApp(t, filesPrepManifest())
	code, out := sendDraft(t, c, id, map[string]string{"main.py": "print(1)"}, nil)
	if code != http.StatusConflict || out.Code != "no_stored_code" {
		t.Fatalf("%d %+v", code, out)
	}
}

func TestDraftSecretsNeedConfirm(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	secret := map[string]string{"main.py": "KEY = 'AKIAIOSFODNN7EXAMPLQ'\nprint(KEY)"}
	if code, out := sendDraft(t, c, id, secret, nil); code != http.StatusConflict || out.Code != "script_secrets_confirmation_required" {
		t.Fatalf("%d %+v", code, out)
	}
	var n int
	testPool.QueryRow(context.Background(), "SELECT count(*) FROM app_drafts WHERE app_id=$1", id).Scan(&n)
	if n != 0 {
		t.Fatal("draft saved:", n)
	}
	if code, out := sendDraft(t, c, id, secret, map[string]any{"confirm_secrets": true}); code != http.StatusCreated {
		t.Fatalf("%d %+v", code, out)
	}
}

func TestDraftNeedsEdit(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := importFiles(t, c, map[string]string{"main.py": "print(1)"})
	_, own := sendDraft(t, c, id, map[string]string{"main.py": "print(2)"}, nil)
	c.do("POST", "/api/people", map[string]string{"name": "Pessoa", "email": "user@example.test", "password": "test-password", "role": "usuario"}, nil)
	other := c.fresh()
	other.do("POST", "/api/auth/login", map[string]string{"email": "user@example.test", "password": "test-password"}, nil)
	if code, out := sendDraft(t, other, id, map[string]string{"main.py": "print(3)"}, nil); code != http.StatusForbidden {
		t.Fatalf("send %d %+v", code, out)
	}
	if code, out := publishDraftReq(other, id, own.DraftID); code != http.StatusForbidden {
		t.Fatalf("publish %d %+v", code, out)
	}
}
