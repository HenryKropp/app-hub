package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func makeAppsFolder(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"Rentabilidade fundos/rentabilidade.py": "import pandas as pd\ndf = pd.read_csv('cnpjs.csv')\nif __name__ == '__main__':\n    pass\n",
		"Rentabilidade fundos/cnpjs.csv":        "cnpj\n",
		"Sistema Alis/app.py":                   "import streamlit as st\n",
		"Projeto Sistema Alis/app.py":           "import streamlit as st\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

type scanResp struct {
	Items []scanItem `json:"items"`
}

func findItem(t *testing.T, r scanResp, folder string) scanItem {
	t.Helper()
	for _, it := range r.Items {
		if it.FolderName == folder {
			return it
		}
	}
	t.Fatalf("folder %q not in scan", folder)
	return scanItem{}
}

func TestScanAndPublish(t *testing.T) {
	c := newClient(t, false)
	admin := setupAdmin(t, c)
	var ana personJSON
	c.do("POST", "/api/people", map[string]string{"name": "Ana", "email": "ana@alis.com", "password": "senha-ana-1", "role": "criador"}, &ana)
	root := makeAppsFolder(t)

	// Batch scan finds one app per subfolder and warns about look-alikes.
	var res scanResp
	if code := c.do("POST", "/api/scan", map[string]string{"path": root, "mode": "batch"}, &res); code != http.StatusOK {
		t.Fatalf("scan = %d", code)
	}
	if len(res.Items) != 3 {
		t.Fatalf("items = %d", len(res.Items))
	}
	rent := findItem(t, res, "Rentabilidade fundos")
	if rent.Manifest.Kind != manifest.KindScript || len(rent.Manifest.Inputs) != 1 {
		t.Errorf("rentabilidade diagnosis = %+v", rent.Manifest)
	}
	if sis := findItem(t, res, "Sistema Alis"); len(sis.Similar) != 1 || sis.Similar[0].FolderName != "Projeto Sistema Alis" {
		t.Errorf("similar = %+v", sis.Similar)
	}

	// Publishing without an owner is refused.
	m := rent.Manifest
	m.Area = "Investimentos"
	body := map[string]any{"folder_path": rent.FolderPath, "manifest": m, "owner_id": 0}
	if code := c.do("POST", "/api/apps", body, nil); code != http.StatusBadRequest {
		t.Fatalf("publish without owner = %d", code)
	}

	body["owner_id"] = admin.ID
	body["backup_id"] = ana.ID
	var saved appDetail
	if code := c.do("POST", "/api/apps", body, &saved); code != http.StatusCreated {
		t.Fatalf("publish = %d", code)
	}
	if saved.Manifest.Owner == nil || saved.Manifest.Owner.Email != "henry@alis.com" || saved.Manifest.Backup.Name != "Ana" {
		t.Errorf("people in manifest = %+v / %+v", saved.Manifest.Owner, saved.Manifest.Backup)
	}

	// The manifest and ficha are written next to the app; the code is untouched.
	yaml, err := os.ReadFile(filepath.Join(rent.FolderPath, manifest.FileName))
	if err != nil || !strings.Contains(string(yaml), "name: Rentabilidade fundos") {
		t.Fatalf("manifest file: %v\n%s", err, yaml)
	}
	if _, err := os.Stat(filepath.Join(rent.FolderPath, manifest.FichaName)); err != nil {
		t.Errorf("ficha missing: %v", err)
	}

	// The app shows up in the catalog and can be opened.
	var apps []appSummary
	c.do("GET", "/api/apps", nil, &apps)
	if len(apps) != 1 || apps[0].OwnerName != "Henry" || apps[0].Area != "Investimentos" {
		t.Fatalf("catalog = %+v", apps)
	}
	var detail appDetail
	if code := c.do("GET", "/api/apps/"+strconv.FormatInt(saved.ID, 10), nil, &detail); code != http.StatusOK || detail.Manifest.Entrypoint != "rentabilidade.py" {
		t.Errorf("detail = %d %+v", code, detail)
	}

	// Scanning again marks it as registered and reuses the saved manifest.
	c.do("POST", "/api/scan", map[string]string{"path": root, "mode": "batch"}, &res)
	again := findItem(t, res, "Rentabilidade fundos")
	if again.RegisteredAppID != saved.ID || !again.HasManifest || again.Manifest.Area != "Investimentos" {
		t.Errorf("rescan = %+v", again)
	}

	// Re-publishing the same folder updates instead of duplicating.
	m.Summary = "Calcula a rentabilidade"
	body["manifest"] = m
	c.do("POST", "/api/apps", body, nil)
	c.do("GET", "/api/apps", nil, &apps)
	if len(apps) != 1 || apps[0].Summary != "Calcula a rentabilidade" {
		t.Errorf("after update = %+v", apps)
	}
}

func TestScanSingleAndErrors(t *testing.T) {
	c := newClient(t, false)
	setupAdmin(t, c)
	root := makeAppsFolder(t)

	var res scanResp
	c.do("POST", "/api/scan", map[string]string{"path": filepath.Join(root, "Sistema Alis"), "mode": "single"}, &res)
	if len(res.Items) != 1 || res.Items[0].Manifest.Kind != manifest.KindWeb {
		t.Errorf("single = %+v", res.Items)
	}
	if code := c.do("POST", "/api/scan", map[string]string{"path": filepath.Join(root, "nao-existe")}, nil); code != http.StatusBadRequest {
		t.Errorf("missing folder = %d", code)
	}
	if code := c.fresh().do("POST", "/api/scan", map[string]string{"path": root}, nil); code != http.StatusUnauthorized {
		t.Errorf("anonymous scan = %d", code)
	}
}

func TestPublishRefusesForeignFicha(t *testing.T) {
	c := newClient(t, false)
	admin := setupAdmin(t, c)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifest.FichaName), []byte("ficha feita à mão"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := manifest.Manifest{Name: "Docs", Kind: manifest.KindReference, Area: "Research"}
	code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "manifest": m, "owner_id": admin.ID}, nil)
	if code != http.StatusConflict {
		t.Fatalf("status = %d", code)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, manifest.FichaName)); string(data) != "ficha feita à mão" {
		t.Error("foreign ficha was overwritten")
	}
}

func TestUserCannotRegister(t *testing.T) {
	c := newClient(t, false)
	setupAdmin(t, c)
	c.do("POST", "/api/people", map[string]string{"name": "Bia", "email": "bia@alis.com", "password": "senha-bia-1", "role": "usuario"}, nil)
	bia := c.fresh()
	bia.do("POST", "/api/auth/login", map[string]string{"email": "bia@alis.com", "password": "senha-bia-1"}, nil)
	if code := bia.do("POST", "/api/scan", map[string]string{"path": t.TempDir()}, nil); code != http.StatusForbidden {
		t.Errorf("usuario scan = %d", code)
	}
	if code := bia.do("GET", "/api/apps", nil, nil); code != http.StatusOK {
		t.Errorf("usuario catalog = %d", code)
	}
}
