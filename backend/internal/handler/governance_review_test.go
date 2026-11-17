package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// Final review of governança (ADR 046): each test reproduces one finding.

// Finding 1: the ficha saved through POST /api/apps (same folder) skipped the
// risk check and the area move.
func TestFichaRiskViaSaveAppNeedsApproval(t *testing.T) {
	h, _, ana, leo, id := approvedScript(t, "risk")
	d := detailOf(t, ana, id)
	d.Manifest.Keys = []manifest.Key{{Name: "API_XP"}}
	body := map[string]any{"folder_path": appFolder(t, id), "owner_id": d.OwnerID, "manifest": d.Manifest}
	if code := ana.do("POST", "/api/apps", body, nil); code != http.StatusCreated {
		t.Fatalf("save %d", code)
	}
	list := approvals(t, leo)
	if len(list) != 1 || list[0].Reason != "ficha" {
		t.Fatalf("approvals %+v", list)
	}
	if r := blockedReason(t, h, id); r == "" {
		t.Fatal("risky ficha runs without approval")
	}
}

func TestAreaMoveViaSaveAppWaits(t *testing.T) {
	_, admin, _, leo, id := approvedScript(t, "risk")
	createPersonAs(t, admin, "Caio", "caio@exemplo.com", "usuario")
	makeLeader(t, 2, 5)
	d := detailOf(t, leo, id)
	d.Manifest.Area = "Investimentos"
	body := map[string]any{"folder_path": appFolder(t, id), "owner_id": d.OwnerID, "manifest": d.Manifest}
	if code := leo.do("POST", "/api/apps", body, nil); code != http.StatusCreated {
		t.Fatalf("save %d", code)
	}
	var area string
	_ = testPool.QueryRow(context.Background(), "SELECT area FROM apps WHERE id = $1", id).Scan(&area)
	if area != "Research" {
		t.Fatalf("moved without the target leader: %s", area)
	}
}

// Finding 2: an artifact moved to another area waits for that area's leader.
func TestArtifactAreaMoveWaitsForTargetLeader(t *testing.T) {
	_, ana, _, _, id := approvedArtifact(t, "risk")
	admin := ana.fresh()
	loginAs(t, admin, "henry@exemplo.com")
	createPersonAs(t, admin, "Caio", "caio@exemplo.com", "usuario")
	makeLeader(t, 2, 5)
	caio := ana.fresh()
	loginAs(t, caio, "caio@exemplo.com")
	if code := ana.do("PUT", "/api/artifacts/"+itoa(id), map[string]any{"name": "Relatório", "area_id": 2}, nil); code != http.StatusOK {
		t.Fatalf("put %d", code)
	}
	var area int64
	ctx := context.Background()
	_ = testPool.QueryRow(ctx, "SELECT area_id FROM artifacts WHERE id = $1", id).Scan(&area)
	if area != 1 {
		t.Fatalf("moved before approval: %d", area)
	}
	list := approvals(t, caio)
	if len(list) != 1 || list[0].Reason != "area" || list[0].Kind != "artifact" {
		t.Fatalf("target leader approvals %+v", list)
	}
	if code := caio.do("POST", approvalPath(list[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	_ = testPool.QueryRow(ctx, "SELECT area_id FROM artifacts WHERE id = $1", id).Scan(&area)
	if area != 2 {
		t.Fatalf("not moved: %d", area)
	}
}

// Finding 3: an app with no governance row in a governed company is held.
func TestAppWithoutGovernanceRowIsHeld(t *testing.T) {
	h, _, ana, _, _ := govTeam(t)
	id := importTestScript(t, ana)
	exec(t, "DELETE FROM approval_requests WHERE app_id = $1", id)
	exec(t, "DELETE FROM app_governance WHERE app_id = $1", id)
	if r := blockedReason(t, h, id); r == "" {
		t.Fatal("app without governance row runs")
	}
}

// Finding 4: a waiting draft replaced by a new draft closes its request, and a
// version request that lost its draft never approves whatever is latest.
func TestCancelledWaitingDraftClosesRequest(t *testing.T) {
	_, _, ana, leo, id := approvedScript(t, "every")
	sendWaiting(t, ana, id, map[string]string{"main.py": "print('v2')"})
	if code := ana.do("POST", appPath(id, "/drafts"), map[string]any{"files": b64files(map[string]string{"main.py": "print('v3')"})}, nil); code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("draft %d", code)
	}
	if got := approvals(t, leo); len(got) != 0 {
		t.Fatalf("request of the cancelled draft still open %+v", got)
	}
}

func TestRequestThatLostItsDraftDoesNotApproveLatest(t *testing.T) {
	_, _, ana, leo, id := approvedScript(t, "every")
	out := sendWaiting(t, ana, id, map[string]string{"main.py": "print('v2')"})
	exec(t, "UPDATE app_drafts SET status = 'cancelada' WHERE id = $1", out.DraftID)
	exec(t, "DELETE FROM app_drafts WHERE id = $1", out.DraftID)
	// A folder version arrives and is held under its own request.
	if err := os.WriteFile(filepath.Join(appFolder(t, id), "main.py"), []byte("print('pasta')"), 0o600); err != nil {
		t.Fatal(err)
	}
	exec(t, "UPDATE approval_requests SET status = 'open', closed_at = NULL WHERE app_id = $1 AND draft_id IS NULL AND reason = 'version' AND app_version IS NULL", id)
	var orphan int64
	if err := testPool.QueryRow(context.Background(), "SELECT id FROM approval_requests WHERE app_id = $1 AND status = 'open' AND draft_id IS NULL AND app_version IS NULL", id).Scan(&orphan); err != nil {
		t.Fatal(err)
	}
	if code := leo.do("POST", approvalPath(orphan, "approve"), nil, nil); code == http.StatusOK {
		var approved int32
		_ = testPool.QueryRow(context.Background(), "SELECT COALESCE(approved_version, 0) FROM app_governance WHERE app_id = $1", id).Scan(&approved)
		t.Fatalf("orphan request approved; approved_version now %d", approved)
	}
}

// Finding 5: a request replaced while the approver decides is not applied.
func TestReplacedRequestIsNotApplied(t *testing.T) {
	h, _, ana, leo, id := approvedScript(t, "every")
	d := detailOf(t, ana, id)
	d.Manifest.Keys = []manifest.Key{{Name: "API_XP"}}
	if code, out := putApp(ana, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	r1 := approvals(t, leo)[0]
	// The ficha request pins the live version.
	var pinned int32
	if err := testPool.QueryRow(context.Background(), "SELECT COALESCE(app_version, 0) FROM approval_requests WHERE id = $1", r1.ID).Scan(&pinned); err != nil || pinned != 1 {
		t.Fatalf("ficha request pinned %d %v", pinned, err)
	}
	// A folder version v2 arrives; approving the ficha does not approve v2.
	if err := os.WriteFile(filepath.Join(appFolder(t, id), "main.py"), []byte("print('pasta')"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, _ := h.q.GetApp(context.Background(), id)
	if _, _, err := h.checkVersion(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if code := leo.do("POST", approvalPath(r1.ID, "approve"), nil, nil); code != http.StatusConflict {
		t.Fatalf("replaced request approve %d", code)
	}
	if r := blockedReason(t, h, id); r == "" {
		t.Fatal("v2 runs without approval")
	}
}

// Finding 6: restoring a folder version that was never approved, in a "só
// pelo Claude" company, is refused.
func TestOnlyClaudeRestoreOfFolderVersionRefused(t *testing.T) {
	h, _, ana, _, id := approvedScript(t, "risk")
	onlyClaude(t)
	ctx := context.Background()
	folder := appFolder(t, id)
	for _, s := range []string{"print('pasta 1')", "print('pasta 2')"} {
		if err := os.WriteFile(filepath.Join(folder, "main.py"), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
		a, _ := h.q.GetApp(ctx, id)
		if _, _, err := h.checkVersion(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	var e struct {
		Error string `json:"error"`
	}
	if code := ana.do("POST", appPath(id, "/versions/2/restore"), nil, &e); code != http.StatusForbidden || e.Error != msgOnlyClaude {
		t.Fatalf("restore %d %q", code, e.Error)
	}
}

// Finding 7: tightening the company reaches items that set a looser rule.
func TestCompanyTighteningReachesItemRule(t *testing.T) {
	_, admin, _, leo, id := approvedScript(t, "first")
	if code := leo.do("PUT", appPath(id, "/governance/rule"), map[string]any{"rule": "risk"}, nil); code != http.StatusOK {
		t.Fatalf("rule %d", code)
	}
	var s map[string]any
	admin.do("GET", "/api/governance/settings", nil, &s)
	s["version_rule"] = "every"
	if code := admin.do("PUT", "/api/governance/settings", s, nil); code != http.StatusOK {
		t.Fatalf("settings %d", code)
	}
	var g struct {
		RuleEffective string `json:"rule_effective"`
	}
	leo.do("GET", appPath(id, "/governance"), nil, &g)
	if g.RuleEffective != "every" {
		t.Fatalf("effective %q", g.RuleEffective)
	}
}

// Finding 10: a version found in the folder is not approved in the owner's
// name, even when the owner leads the area.
func TestFolderVersionNotSelfApprovedByLeaderOwner(t *testing.T) {
	h, _, _, leo, id := approvedScript(t, "every")
	exec(t, "UPDATE apps SET owner_id = 4 WHERE id = $1", id)
	if err := os.WriteFile(filepath.Join(appFolder(t, id), "main.py"), []byte("print('pasta')"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, _ := h.q.GetApp(context.Background(), id)
	if _, _, err := h.checkVersion(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if r := blockedReason(t, h, id); r == "" {
		t.Fatal("folder version approved in the leader's name")
	}
	if got := approvals(t, leo); len(got) != 1 {
		t.Fatalf("leader approvals %+v", got)
	}
}

// Finding 9: the artifact page asks the server; without a company there is
// no governance to show.
func TestArtifactGovernanceOnlyInACompany(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id := createArtifact(t, c, fileBody("Relatório", "rel.html", []byte(pageHTML))).ID
	if code := c.do("GET", "/api/artifacts/"+itoa(id)+"/governance", nil, nil); code != http.StatusNotFound {
		t.Fatalf("ungoverned %d", code)
	}
}
