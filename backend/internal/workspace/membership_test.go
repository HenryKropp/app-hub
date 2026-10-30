package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func memberJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func invitationToken(t *testing.T, raw []byte, prefix string) string {
	t.Helper()
	out := memberJSON(t, raw)
	link, _ := out["url"].(string)
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal("invalid invitation URL")
	}
	token := strings.TrimPrefix(u.Fragment, "token=")
	if u.Path != prefix+"/convite" || len(token) != 43 || !strings.HasPrefix(u.Fragment, "token=") || u.RawQuery != "" {
		t.Fatalf("invalid invitation link: path=%s", u.Path)
	}
	return token
}

func TestPortalPublicCompanyAndMembership(t *testing.T) {
	p, pool, srv := testPortal(t)
	ctx := context.Background()
	root := browser(t, srv.URL)
	root.want("POST", "/api/auth/setup", map[string]string{"name": "Owner", "email": "owner@example.com", "password": "owner-password"}, 201)
	visitor := browser(t, srv.URL)
	registration := map[string]string{"company_name": "Equipe Aurora", "slug": "aurora", "name": "Lia", "email": "lia@example.com", "password": "aurora-password"}
	out := memberJSON(t, visitor.want("POST", "/api/entry/companies", registration, 201))
	if out["url"] != "/w/aurora/" {
		t.Fatalf("registration redirect=%v", out["url"])
	}
	visitor.want("POST", "/api/entry/companies", registration, 409)
	registration["slug"] = "principal"
	visitor.want("POST", "/api/entry/companies", registration, 400)
	registration["slug"] = "other"
	visitor.want("POST", "/w/aurora/api/entry/companies", registration, 403)
	visitor.want("GET", "/w/nonexistent/api/workspace", nil, 404)
	var rootPeople int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM people`).Scan(&rootPeople); err != nil || rootPeople != 1 {
		t.Fatalf("root people changed: %d %v", rootPeople, err)
	}

	prefix := "/w/aurora"
	manager := browser(t, srv.URL)
	manager.want("POST", prefix+"/api/auth/login", map[string]string{"email": "lia@example.com", "password": "aurora-password"}, 200)
	visitor.want("GET", prefix+"/api/membership", nil, 401)
	request := map[string]string{"name": "Guest", "email": "guest@example.com"}
	visitor.want("POST", prefix+"/api/entry/requests", request, 200)
	visitor.want("POST", prefix+"/api/entry/requests", request, 200)
	visitor.want("POST", prefix+"/api/entry/requests", map[string]string{"name": "Bad", "email": "bad email"}, 400)
	info := memberJSON(t, manager.want("GET", prefix+"/api/membership", nil, 200))
	requests := info["requests"].([]any)
	if len(requests) != 1 {
		t.Fatalf("duplicate pending requests: %d", len(requests))
	}
	requestID := int64(requests[0].(map[string]any)["id"].(float64))
	rootInfo := memberJSON(t, root.want("GET", "/api/membership", nil, 200))
	if len(rootInfo["requests"].([]any)) != 0 {
		t.Fatal("tenant requests leaked to root")
	}
	e := p.entries["aurora"]
	var userRole int64
	if err := e.pool.QueryRow(ctx, `SELECT id FROM roles WHERE key='usuario'`).Scan(&userRole); err != nil {
		t.Fatal(err)
	}
	approval := fmt.Sprintf("/api/membership/requests/%d/approve", requestID)
	root.want("POST", approval, map[string]int64{"role_id": userRole}, 404)
	manager.want("POST", prefix+approval, map[string]int64{"role_id": 0}, 400)
	token := invitationToken(t, manager.want("POST", prefix+approval, map[string]int64{"role_id": userRole}, 201), prefix)
	manager.want("POST", prefix+approval, map[string]int64{"role_id": userRole}, 404)
	info = memberJSON(t, manager.want("GET", prefix+"/api/membership", nil, 200))
	if len(info["requests"].([]any)) != 0 || len(info["invitations"].([]any)) != 1 {
		t.Fatal("approval did not move request to invitation")
	}
	var people int
	e.pool.QueryRow(ctx, `SELECT count(*) FROM people`).Scan(&people)
	if people != 1 {
		t.Fatal("approval granted membership before invitation accepted")
	}
	var stored string
	if err := e.pool.QueryRow(ctx, `SELECT token_hash FROM membership_invitations`).Scan(&stored); err != nil || stored != hashForTest(token) {
		t.Fatal("invitation token is not stored as a hash")
	}
	visitor.want("POST", "/api/entry/invitations/preview", map[string]string{"token": token}, 410)
	preview := memberJSON(t, visitor.want("POST", prefix+"/api/entry/invitations/preview", map[string]string{"token": token}, 200))
	if preview["email"] != request["email"] || preview["company_name"] != "Equipe Aurora" {
		t.Fatal("wrong invitation preview")
	}
	visitor.want("POST", prefix+"/api/entry/invitations/accept", map[string]string{"token": token, "name": "Guest", "password": "short"}, 400)
	accepted := memberJSON(t, visitor.want("POST", prefix+"/api/entry/invitations/accept", map[string]string{"token": token, "name": "Guest", "password": "guest-password", "email": "attacker@example.com"}, 201))
	if accepted["url"] != prefix+"/" {
		t.Fatal("acceptance crossed company")
	}
	status := memberJSON(t, visitor.want("GET", prefix+"/api/auth/status", nil, 200))
	person := status["person"].(map[string]any)
	if person["email"] != request["email"] {
		t.Fatal("client overrode invitation email")
	}
	rootStatus := memberJSON(t, visitor.want("GET", "/api/auth/status", nil, 200))
	if rootStatus["person"] != nil {
		t.Fatal("tenant session authenticated root")
	}
	visitor.want("GET", prefix+"/api/membership", nil, 403)
	visitor.want("POST", prefix+"/api/membership/invitations", map[string]any{"email": "other@example.com", "role_id": userRole}, 403)
	visitor.want("POST", prefix+"/api/entry/invitations/accept", map[string]string{"token": token, "name": "Guest", "password": "guest-password"}, 410)
	manager.want("POST", prefix+"/api/membership/invitations", map[string]any{"email": "guest@example.com", "role_id": userRole}, 409)

	// Reissue replaces the old bearer link; revocation and expiry are enforced.
	body := map[string]any{"email": "next@example.com", "role_id": userRole}
	old := invitationToken(t, manager.want("POST", prefix+"/api/membership/invitations", body, 201), prefix)
	fresh := invitationToken(t, manager.want("POST", prefix+"/api/membership/invitations", body, 201), prefix)
	visitor.want("POST", prefix+"/api/entry/invitations/preview", map[string]string{"token": old}, 410)
	var freshID int64
	e.pool.QueryRow(ctx, `SELECT id FROM membership_invitations WHERE token_hash=$1`, hashForTest(fresh)).Scan(&freshID)
	manager.want("POST", fmt.Sprintf("%s/api/membership/invitations/%d/revoke", prefix, freshID), map[string]any{}, 200)
	visitor.want("POST", prefix+"/api/entry/invitations/preview", map[string]string{"token": fresh}, 410)
	fresh = invitationToken(t, manager.want("POST", prefix+"/api/membership/invitations", body, 201), prefix)
	e.pool.Exec(ctx, `UPDATE membership_invitations SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, hashForTest(fresh))
	visitor.want("POST", prefix+"/api/entry/invitations/accept", map[string]string{"token": fresh, "name": "Next", "password": "next-password"}, 410)
	visitor.want("POST", prefix+"/api/entry/requests", map[string]string{"name": "Rejected", "email": "reject@example.com"}, 200)
	var rejected int64
	e.pool.QueryRow(ctx, `SELECT id FROM membership_requests WHERE email='reject@example.com'`).Scan(&rejected)
	manager.want("POST", fmt.Sprintf("%s/api/membership/requests/%d/reject", prefix, rejected), map[string]any{}, 200)
	var pending int
	e.pool.QueryRow(ctx, `SELECT count(*) FROM membership_requests WHERE status='pending'`).Scan(&pending)
	if pending != 0 {
		t.Fatal("rejection left request pending")
	}

	// Bodies and tokens never appear in audit metadata.
	var leaked bool
	if err := e.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM security_audit WHERE (route || action) LIKE $1)`, "%"+token+"%").Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked {
		t.Fatal("raw invitation token in audit")
	}
}

func TestPortalInvitationOneConcurrentUse(t *testing.T) {
	p, _, srv := testPortal(t)
	manager := browser(t, srv.URL)
	manager.want("POST", "/api/auth/setup", map[string]string{"name": "Owner", "email": "owner@example.com", "password": "owner-password"}, 201)
	var roleID int64
	if err := p.root.QueryRow(context.Background(), `SELECT id FROM roles WHERE key='usuario'`).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	token := invitationToken(t, manager.want("POST", "/api/membership/invitations", map[string]any{"email": "race@example.com", "role_id": roleID}, 201), "")
	browsers := []*testBrowser{browser(t, srv.URL), browser(t, srv.URL)}
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for _, b := range browsers {
		wg.Add(1)
		go func(b *testBrowser) {
			defer wg.Done()
			code, _ := b.do("POST", "/api/entry/invitations/accept", map[string]string{"token": token, "name": "Race", "password": "race-password"})
			results <- code
		}(b)
	}
	wg.Wait()
	close(results)
	success, expired := 0, 0
	for code := range results {
		if code == 201 {
			success++
		} else if code == 410 {
			expired++
		} else {
			t.Fatalf("unexpected concurrent status %d", code)
		}
	}
	if success != 1 || expired != 1 {
		t.Fatalf("concurrent uses: success=%d expired=%d", success, expired)
	}
}

func TestPortalEntryRejectsCrossOriginAndLimitsAttempts(t *testing.T) {
	_, _, srv := testPortal(t)
	b := browser(t, srv.URL)
	reqURL := srv.URL + "/api/entry/requests"
	crossOrigin, _ := http.NewRequest("POST", reqURL, strings.NewReader(`{"name":"x","email":"x@example.com"}`))
	crossOrigin.Header.Set("Content-Type", "application/json")
	crossOrigin.Header.Set("Origin", "https://external.example.com")
	blocked, err := b.c.Do(crossOrigin)
	if err != nil {
		t.Fatal(err)
	}
	blocked.Body.Close()
	if blocked.StatusCode != 403 {
		t.Fatalf("cross origin status=%d", blocked.StatusCode)
	}
	resp, err := b.c.Post(reqURL, "text/plain", strings.NewReader(`{"name":"x","email":"x@example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 415 {
		t.Fatalf("unsafe content type=%d", resp.StatusCode)
	}
	for i := 0; i < 20; i++ {
		b.want("POST", "/api/entry/requests", map[string]string{"name": "X", "email": "x@example.com"}, 200)
	}
	b.want("POST", "/api/entry/requests", map[string]string{"name": "X", "email": "x@example.com"}, 429)
	// Invalid invite is explicitly unavailable, not a public lookup endpoint.
	b.want("POST", "/api/entry/invitations/preview", map[string]string{"token": strings.Repeat("a", 43)}, 410)
}
