package workspace

import (
	"encoding/json"
	"fmt"
	"testing"
)

// The Admin dashboard lists the company's apps with their status, and each
// app's page approves or suspends it.
func TestPortalAdminDashboardAndAppControl(t *testing.T) {
	_, _, srv := testPortal(t)
	b := browser(t, srv.URL)
	credentials := map[string]string{"name": "Admin", "email": "admin@example.test", "password": "test-password-not-a-real-credential"}
	b.want("POST", "/api/auth/setup", credentials, 201)
	b.want("POST", "/api/workspaces", map[string]string{"name": "Admin company", "slug": "admin-company", "password": credentials["password"]}, 201)
	const prefix = "/w/admin-company"
	b.want("POST", prefix+"/api/auth/login", credentials, 200)
	b.want("PUT", prefix+"/api/security/settings", map[string]any{"retention_days": 30, "require_app_approval": true, "block_embedded_secrets": true, "local_execution_enabled": true}, 200)
	id := importApp(t, b, prefix, "Dashboard app")
	app := fmt.Sprintf("%s/api/apps/%d", prefix, id)

	var overview struct {
		Apps []struct {
			Name      string `json:"name"`
			Approved  bool   `json:"approved"`
			Suspended bool   `json:"suspended"`
		} `json:"apps"`
		Activity []struct {
			Kind string `json:"kind"`
		} `json:"activity"`
	}
	if err := json.Unmarshal(b.want("GET", prefix+"/api/admin/overview", nil, 200), &overview); err != nil {
		t.Fatal(err)
	}
	if len(overview.Apps) != 1 || overview.Apps[0].Name != "Dashboard app" || overview.Apps[0].Approved {
		t.Fatalf("a new app should wait for approval: %+v", overview.Apps)
	}
	if len(overview.Activity) == 0 {
		t.Fatal("registering an app should show in the activity")
	}

	control := func() (c struct {
		Approved, Suspended, CanManage bool
	}) {
		var d struct {
			Control *struct {
				Approved  bool `json:"approved"`
				Suspended bool `json:"suspended"`
				CanManage bool `json:"can_manage"`
			} `json:"control"`
		}
		if err := json.Unmarshal(b.want("GET", app, nil, 200), &d); err != nil || d.Control == nil {
			t.Fatalf("app page has no control: %v", err)
		}
		c.Approved, c.Suspended, c.CanManage = d.Control.Approved, d.Control.Suspended, d.Control.CanManage
		return
	}
	if c := control(); c.Approved || !c.CanManage {
		t.Fatalf("before approval: %+v", c)
	}
	b.want("POST", app+"/runs", map[string]any{"output_folder": `C:\Results`}, 403)
	b.want("PUT", app+"/control", map[string]bool{"approved": true}, 200)
	if c := control(); !c.Approved {
		t.Fatal("approval did not stick")
	}
	b.want("POST", app+"/runs", map[string]any{"output_folder": `C:\Results`}, 409) // approved; no computer connected
	b.want("PUT", app+"/control", map[string]bool{"approved": true, "suspended": true}, 200)
	b.want("POST", app+"/runs", map[string]any{"output_folder": `C:\Results`}, 403)

	b.want("POST", prefix+"/api/people", map[string]any{"name": "Member", "email": "member@example.test", "password": "member-test-password", "role": "usuario"}, 201)
	member := browser(t, srv.URL)
	member.want("POST", prefix+"/api/auth/login", map[string]any{"email": "member@example.test", "password": "member-test-password"}, 200)
	member.want("GET", prefix+"/api/admin/overview", nil, 403)
	member.want("PUT", app+"/control", map[string]bool{"approved": true}, 403)
}
