package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// govTeam: admin Henry (1), criador Ana (2), usuário Bia (3, member of
// Research), usuário Leo (4, leader of Research) in a company with
// governance on (workspace set). The company starts light: leader only,
// "só quando muda o risco".
func govTeam(t *testing.T) (h *Handler, admin, ana, bia, leo *client) {
	t.Helper()
	admin, h = newPoolClient(t)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	setupAdmin(t, admin)
	// A new company: governance replaces the old single approval.
	exec(t, "UPDATE security_settings SET require_app_approval = FALSE")
	createPersonAs(t, admin, "Ana", "ana@exemplo.com", "criador")
	createPersonAs(t, admin, "Bia", "bia@exemplo.com", "usuario")
	createPersonAs(t, admin, "Leo", "leo@exemplo.com", "usuario")
	exec(t, "INSERT INTO area_members (area_id, person_id) VALUES (1, 3)")
	makeLeader(t, 1, 4)
	ana, bia, leo = admin.fresh(), admin.fresh(), admin.fresh()
	loginAs(t, ana, "ana@exemplo.com")
	loginAs(t, bia, "bia@exemplo.com")
	loginAs(t, leo, "leo@exemplo.com")
	return
}

type approvalJSON struct {
	ID        int64    `json:"id"`
	Kind      string   `json:"kind"`
	ItemID    int64    `json:"item_id"`
	ItemName  string   `json:"item_name"`
	Reason    string   `json:"reason"`
	RiskAdded []string `json:"risk_added"`
	Status    string   `json:"status"`
	Tested    bool     `json:"tested"`
	Steps     []struct {
		Step     string `json:"step"`
		Decision string `json:"decision"`
		OnBehalf string `json:"on_behalf"`
	} `json:"steps"`
}

func approvals(t *testing.T, c *client) []approvalJSON {
	t.Helper()
	var out []approvalJSON
	if code := c.do("GET", "/api/approvals", nil, &out); code != http.StatusOK {
		t.Fatalf("approvals %d", code)
	}
	return out
}

func approvalPath(id int64, action string) string {
	return "/api/approvals/" + strconv.FormatInt(id, 10) + "/" + action
}

func blockedReason(t *testing.T, h *Handler, appID int64) string {
	t.Helper()
	r, err := h.executionBlock(context.Background(), appID)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

var areaOnly = map[string]any{"audience": map[string]any{"mode": "area"}}

func TestNewAppOpensLeaderRequest(t *testing.T) {
	h, admin, ana, bia, leo := govTeam(t)
	id := importTestScript(t, ana)
	if r := blockedReason(t, h, id); r != "Aguardando o líder de Research." {
		t.Fatalf("blocked %q", r)
	}
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusNotFound {
		t.Fatalf("bia sees pending app: %d", code)
	}
	if code := ana.do("GET", appPath(id, ""), nil, nil); code != http.StatusOK {
		t.Fatalf("creator does not see own app: %d", code)
	}
	list := approvals(t, leo)
	if len(list) != 1 || list[0].ItemID != id || list[0].Reason != "new" || len(list[0].Steps) != 1 || list[0].Steps[0].Step != "leader" {
		t.Fatalf("leader approvals %+v", list)
	}
	var n struct {
		Count int `json:"count"`
	}
	leo.do("GET", "/api/approvals/count", nil, &n)
	if n.Count != 1 {
		t.Fatalf("count %d", n.Count)
	}
	if got := approvals(t, bia); len(got) != 0 {
		t.Fatalf("bia approvals %+v", got)
	}
	// The area has a leader, so the admin does not decide.
	if got := approvals(t, admin); len(got) != 0 {
		t.Fatalf("admin approvals %+v", got)
	}
}

func TestLeaderApprovesWithAudience(t *testing.T) {
	h, admin, ana, bia, leo := govTeam(t)
	createPersonAs(t, admin, "Caio", "caio@exemplo.com", "usuario")
	caio := admin.fresh()
	loginAs(t, caio, "caio@exemplo.com")
	id := importTestScript(t, ana)
	req := approvals(t, leo)[0]
	if code := leo.do("POST", approvalPath(req.ID, "approve"), areaOnly, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("still blocked %q", r)
	}
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusOK {
		t.Fatalf("area member %d", code)
	}
	if code := caio.do("GET", appPath(id, ""), nil, nil); code != http.StatusNotFound {
		t.Fatalf("outsider %d", code)
	}
	if got := approvals(t, leo); len(got) != 0 {
		t.Fatalf("still listed %+v", got)
	}
}

func TestApproveNeedsAudienceForNew(t *testing.T) {
	h, _, ana, _, leo := govTeam(t)
	id := importTestScript(t, ana)
	req := approvals(t, leo)[0]
	var e struct {
		Error string `json:"error"`
	}
	if code := leo.do("POST", approvalPath(req.ID, "approve"), map[string]any{}, &e); code != http.StatusBadRequest || e.Error != "Escolha quem vai usar." {
		t.Fatalf("no audience %d %q", code, e.Error)
	}
	if r := blockedReason(t, h, id); r == "" {
		t.Fatal("approved without audience")
	}
}

func TestITStepWhenCompanyTurnsItOn(t *testing.T) {
	h, admin, ana, _, leo := govTeam(t)
	exec(t, "UPDATE security_settings SET it_approval = TRUE")
	setRole(t, "admin", "manage_people", "manage_areas", "manage_roles", "approve_it")
	id := importTestScript(t, ana)
	req := approvals(t, leo)[0]
	if len(req.Steps) != 2 {
		t.Fatalf("steps %+v", req.Steps)
	}
	if code := leo.do("POST", approvalPath(req.ID, "approve"), areaOnly, nil); code != http.StatusOK {
		t.Fatalf("leader approve %d", code)
	}
	if r := blockedReason(t, h, id); r != "Aguardando a TI." {
		t.Fatalf("after leader %q", r)
	}
	if got := approvals(t, leo); len(got) != 0 {
		t.Fatalf("leader still sees it %+v", got)
	}
	if got := approvals(t, admin); len(got) != 1 {
		t.Fatalf("TI does not see it %+v", got)
	}
	if code := admin.do("POST", approvalPath(req.ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("TI approve %d", code)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("after TI %q", r)
	}
}

func TestRequesterLeaderAutoStep(t *testing.T) {
	h, _, _, bia, leo := govTeam(t)
	setRole(t, "usuario", "register_apps")
	id := importTestScript(t, leo)
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("leader's own app blocked %q", r)
	}
	// Nobody had to pick the audience: the app keeps the access it was created with.
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusOK {
		t.Fatalf("area member %d", code)
	}
}

func TestOnePersonCompanyOneClick(t *testing.T) {
	admin, h := newPoolClient(t)
	h.SetWorkspace(WorkspaceOptions{Slug: "teste", Name: "Teste"})
	setupAdmin(t, admin)
	// A new company: governance replaces the old single approval.
	exec(t, "UPDATE security_settings SET require_app_approval = FALSE")
	exec(t, "UPDATE security_settings SET it_approval = TRUE")
	setRole(t, "admin", "manage_people", "manage_areas", "manage_roles", "register_apps", "approve_it")
	id := importTestScript(t, admin)
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("alone and still blocked %q", r)
	}
}

func TestAdminApprovesOnlyWithoutLeader(t *testing.T) {
	h, admin, ana, _, leo := govTeam(t)
	id := importTestScript(t, ana)
	req := approvals(t, leo)[0]
	if code := admin.do("POST", approvalPath(req.ID, "approve"), areaOnly, nil); code != http.StatusForbidden {
		t.Fatalf("admin with leader %d", code)
	}
	exec(t, "DELETE FROM area_leaders WHERE area_id = 1")
	if got := approvals(t, admin); len(got) != 1 {
		t.Fatalf("admin list %+v", got)
	}
	if code := admin.do("POST", approvalPath(req.ID, "approve"), areaOnly, nil); code != http.StatusOK {
		t.Fatalf("admin without leader %d", code)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("blocked %q", r)
	}
	var onBehalf string
	if err := testPool.QueryRow(context.Background(), "SELECT on_behalf FROM approval_steps WHERE request_id = $1 AND step = 'leader'", req.ID).Scan(&onBehalf); err != nil {
		t.Fatal(err)
	}
	if onBehalf != "admin_no_leader" {
		t.Fatalf("on_behalf %q", onBehalf)
	}
}

func TestRejectNeedsReasonAndKeepsOld(t *testing.T) {
	h, _, ana, _, leo := govTeam(t)
	id := importTestScript(t, ana)
	req := approvals(t, leo)[0]
	var e struct {
		Error string `json:"error"`
	}
	if code := leo.do("POST", approvalPath(req.ID, "reject"), map[string]string{"reason": " "}, &e); code != http.StatusBadRequest || e.Error != "Diga o motivo." {
		t.Fatalf("no reason %d %q", code, e.Error)
	}
	if code := leo.do("POST", approvalPath(req.ID, "reject"), map[string]string{"reason": "falta teste"}, nil); code != http.StatusOK {
		t.Fatalf("reject %d", code)
	}
	if r := blockedReason(t, h, id); r != "Recusado por Leo: falta teste" {
		t.Fatalf("after reject %q", r)
	}
	if code := leo.do("POST", approvalPath(req.ID, "approve"), areaOnly, nil); code != http.StatusConflict {
		t.Fatalf("approve after reject %d", code)
	}
}

func TestRejectedVersionKeepsApprovedOne(t *testing.T) {
	h, _, ana, _, leo := govTeam(t)
	id := importFiles(t, ana, map[string]string{"main.py": "print('v1')"})
	leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), areaOnly, nil)
	// A folder version that adds a site asks again; refusing it holds the
	// new version — the approved one stays recorded.
	ctx := context.Background()
	a, _ := h.q.GetApp(ctx, id)
	if err := os.WriteFile(filepath.Join(a.FolderPath, "main.py"), []byte("print('https://api.exemplo.com.br')"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.checkVersion(ctx, a); err != nil {
		t.Fatal(err)
	}
	list := approvals(t, leo)
	if len(list) != 1 || len(list[0].RiskAdded) != 1 || list[0].RiskAdded[0] != "site: api.exemplo.com.br" {
		t.Fatalf("approvals %+v", list)
	}
	if code := leo.do("POST", approvalPath(list[0].ID, "reject"), map[string]string{"reason": "site novo"}, nil); code != http.StatusOK {
		t.Fatalf("reject %d", code)
	}
	var approved int32
	if err := testPool.QueryRow(ctx, "SELECT approved_version FROM app_governance WHERE app_id = $1", id).Scan(&approved); err != nil {
		t.Fatal(err)
	}
	if approved != 1 {
		t.Fatalf("approved version %d", approved)
	}
	if r := blockedReason(t, h, id); r != "Recusado por Leo: site novo" {
		t.Fatalf("blocked %q", r)
	}
	if k := homeKinds(getHome(t, ana)); k["approval_pending"] != 1 {
		t.Fatalf("home %+v", k)
	}
}

func TestVersionWithoutNewRiskGoesAuto(t *testing.T) {
	h, _, ana, _, leo := govTeam(t)
	id := importTestScript(t, ana)
	leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), areaOnly, nil)
	out, err := h.openRequest(context.Background(), requestIn{Kind: "app", AppID: id, AreaID: 1, Reason: "version", AppVersion: 2, Channel: "manual", RequesterID: 2})
	if err != nil || !out.Auto {
		t.Fatalf("open %+v %v", out, err)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("blocked %q", r)
	}
}

func TestDoubleApproveConflict(t *testing.T) {
	h, _, ana, _, leo := govTeam(t)
	id := importTestScript(t, ana)
	req := approvals(t, leo)[0]
	if code := leo.do("POST", approvalPath(req.ID, "approve"), areaOnly, nil); code != http.StatusOK {
		t.Fatalf("first %d", code)
	}
	var e struct {
		Error string `json:"error"`
	}
	if code := leo.do("POST", approvalPath(req.ID, "approve"), areaOnly, &e); code != http.StatusConflict || e.Error != "Este pedido já foi resolvido." {
		t.Fatalf("second %d %q", code, e.Error)
	}
	var events int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM governance_events WHERE app_id = $1 AND kind = 'audience'", id).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("applied %d times", events)
	}
	_ = h
}

func TestItemRuleCannotLoosenBelowCompany(t *testing.T) {
	_, admin, ana, _, leo := govTeam(t)
	id := importTestScript(t, ana)
	leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), areaOnly, nil)
	if code := leo.do("PUT", appPath(id, "/governance/rule"), map[string]any{"rule": "every"}, nil); code != http.StatusOK {
		t.Fatalf("tighten %d", code)
	}
	var g struct {
		RuleCompany   string `json:"rule_company"`
		RuleItem      string `json:"rule_item"`
		RuleEffective string `json:"rule_effective"`
	}
	leo.do("GET", appPath(id, "/governance"), nil, &g)
	if g.RuleCompany != "risk" || g.RuleItem != "every" || g.RuleEffective != "every" {
		t.Fatalf("governance %+v", g)
	}
	if code := leo.do("PUT", appPath(id, "/governance/rule"), map[string]any{"rule": "first"}, nil); code != http.StatusForbidden {
		t.Fatalf("leader loosens %d", code)
	}
	if code := admin.do("PUT", appPath(id, "/governance/rule"), map[string]any{"rule": "first"}, nil); code != http.StatusOK {
		t.Fatalf("admin loosens %d", code)
	}
	var events int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM governance_events WHERE app_id = $1 AND kind = 'rule'", id).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Fatalf("rule events %d", events)
	}
}

// approvedScript: Ana's script, approved by Leo for the area, under rule.
func approvedScript(t *testing.T, rule string) (h *Handler, admin, ana, leo *client, id int64) {
	t.Helper()
	h, admin, ana, _, leo = govTeam(t)
	id = importFiles(t, ana, map[string]string{"main.py": "print('v1')"})
	if code := leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), areaOnly, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	exec(t, "UPDATE security_settings SET version_rule = $1", rule)
	return
}

func latestVersion(t *testing.T, id int64) int32 {
	t.Helper()
	var n int32
	if err := testPool.QueryRow(context.Background(), "SELECT COALESCE(max(number), 0) FROM app_versions WHERE app_id = $1", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type waitingOut struct {
	Version int32  `json:"version"`
	Changed bool   `json:"changed"`
	Waiting string `json:"waiting"`
	DraftID int64  `json:"draft_id"`
}

func sendWaiting(t *testing.T, c *client, id int64, files map[string]string) waitingOut {
	t.Helper()
	var out waitingOut
	if code := c.do("POST", appPath(id, "/sources"), map[string]any{"files": b64files(files)}, &out); code != http.StatusOK {
		t.Fatalf("sources %d", code)
	}
	return out
}

func TestVersionUnderEveryWaitsAndOldKeepsRunning(t *testing.T) {
	h, _, ana, leo, id := approvedScript(t, "every")
	out := sendWaiting(t, ana, id, map[string]string{"main.py": "print('v2')"})
	if out.Waiting != "Aguardando o líder de Research." || out.Changed || out.DraftID == 0 {
		t.Fatalf("send %+v", out)
	}
	if latestVersion(t, id) != 1 || storedNames(t, id)["main.py"] != "print('v1')" {
		t.Fatal("waiting version went live")
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("old version stopped %q", r)
	}
	list := approvals(t, leo)
	if len(list) != 1 || list[0].Reason != "version" {
		t.Fatalf("approvals %+v", list)
	}
	if code := leo.do("POST", approvalPath(list[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	if latestVersion(t, id) != 2 || storedNames(t, id)["main.py"] != "print('v2')" {
		t.Fatal("approved version not live")
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("blocked %q", r)
	}
}

func TestVersionUnderRiskWithoutNewRiskGoesLive(t *testing.T) {
	h, _, ana, leo, id := approvedScript(t, "risk")
	out := sendWaiting(t, ana, id, map[string]string{"main.py": "print('v2')"})
	if out.Waiting != "" || !out.Changed || out.Version != 2 {
		t.Fatalf("send %+v", out)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("blocked %q", r)
	}
	list := approvals(t, leo)
	if len(list) != 1 || list[0].Status != "auto" {
		t.Fatalf("auto notice %+v", list)
	}
}

func TestVersionAddingDomainNeedsApproval(t *testing.T) {
	h, _, ana, leo, id := approvedScript(t, "risk")
	out := sendWaiting(t, ana, id, map[string]string{"main.py": "import requests\nrequests.get('https://xp.com/api')"})
	if out.Waiting == "" {
		t.Fatalf("send %+v", out)
	}
	list := approvals(t, leo)
	if len(list) != 1 || len(list[0].RiskAdded) != 1 || list[0].RiskAdded[0] != "site: xp.com" {
		t.Fatalf("approvals %+v", list)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("old version stopped %q", r)
	}
}

func TestNewVersionReplacesOpenRequest(t *testing.T) {
	_, _, ana, leo, id := approvedScript(t, "every")
	first := sendWaiting(t, ana, id, map[string]string{"main.py": "print('v2')"})
	second := sendWaiting(t, ana, id, map[string]string{"main.py": "print('v3')"})
	list := approvals(t, leo)
	if len(list) != 1 {
		t.Fatalf("approvals %+v", list)
	}
	var status string
	if err := testPool.QueryRow(context.Background(), "SELECT status FROM app_drafts WHERE id = $1", first.DraftID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "cancelada" {
		t.Fatalf("first draft %s", status)
	}
	if code := leo.do("POST", approvalPath(list[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	if latestVersion(t, id) != 2 || storedNames(t, id)["main.py"] != "print('v3')" || second.DraftID == first.DraftID {
		t.Fatalf("live %d %q", latestVersion(t, id), storedNames(t, id)["main.py"])
	}
}

func TestScheduledRunBlockedWhileWaiting(t *testing.T) {
	h, _, _, _, _ := govTeam(t)
	now := time.Now()
	sid, app, _ := dueSchedule(t, h, now, true)
	ownerOf(t, app, 3)
	if _, err := h.openRequest(context.Background(), requestIn{Kind: "app", AppID: app, AreaID: 1, Reason: "new", RequesterID: 3}); err != nil {
		t.Fatal(err)
	}
	h.schedulerTick(context.Background(), now)
	s := readSchedule(t, sid)
	if s.Runs != 0 || !s.Active || s.Situation != "Aguardando o líder de Research." {
		t.Fatalf("%+v", s)
	}
}

func TestFolderVersionBlocksUntilApproved(t *testing.T) {
	h, _, _, leo, id := approvedScript(t, "every")
	a, err := h.q.GetApp(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.FolderPath, "main.py"), []byte("print('pasta')"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.checkVersion(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if r := blockedReason(t, h, id); r != "Aguardando o líder de Research." {
		t.Fatalf("folder version runs %q", r)
	}
	if code := leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("blocked %q", r)
	}
}

func TestFichaTextEditNoApproval(t *testing.T) {
	h, _, ana, leo, id := approvedScript(t, "every")
	d := detailOf(t, ana, id)
	d.Manifest.Summary = "Resumo novo"
	if code, out := putApp(ana, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("blocked %q", r)
	}
	if got := approvals(t, leo); len(got) != 0 {
		t.Fatalf("approvals %+v", got)
	}
}

func TestFichaNewKeyNeedsApproval(t *testing.T) {
	h, _, ana, leo, id := approvedScript(t, "risk")
	d := detailOf(t, ana, id)
	d.Manifest.Keys = []manifest.Key{{Name: "API_XP"}}
	if code, out := putApp(ana, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	list := approvals(t, leo)
	if len(list) != 1 || list[0].Reason != "ficha" || len(list[0].RiskAdded) != 1 || list[0].RiskAdded[0] != "chave do Cofre: API_XP" {
		t.Fatalf("approvals %+v", list)
	}
	if r := blockedReason(t, h, id); r != "Aguardando o líder de Research." {
		t.Fatalf("blocked %q", r)
	}
	if code := leo.do("POST", approvalPath(list[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("still blocked %q", r)
	}
}

func TestAreaMoveWaitsForTargetLeader(t *testing.T) {
	h, admin, _, leo, id := approvedScript(t, "risk")
	createPersonAs(t, admin, "Caio", "caio@exemplo.com", "usuario")
	makeLeader(t, 2, 5)
	caio := admin.fresh()
	loginAs(t, caio, "caio@exemplo.com")
	d := detailOf(t, leo, id)
	d.Manifest.Area = "Investimentos"
	if code, out := putApp(leo, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	var area string
	ctx := context.Background()
	_ = testPool.QueryRow(ctx, "SELECT area FROM apps WHERE id = $1", id).Scan(&area)
	if area != "Research" {
		t.Fatalf("moved before approval: %s", area)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("area move stops the app %q", r)
	}
	list := approvals(t, caio)
	if len(list) != 1 || list[0].Reason != "area" {
		t.Fatalf("target leader approvals %+v", list)
	}
	if code := caio.do("POST", approvalPath(list[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	_ = testPool.QueryRow(ctx, "SELECT area FROM apps WHERE id = $1", id).Scan(&area)
	if area != "Investimentos" {
		t.Fatalf("not moved: %s", area)
	}
}

func TestTestedDraftWaitsWhenRuleSaysSo(t *testing.T) {
	h, _, _, leo, id := approvedScript(t, "every")
	ctx := context.Background()
	files, _ := json.Marshal(b64files(map[string]string{"main.py": "print('testada')"}))
	d, err := h.q.InsertDraft(ctx, sqlc.InsertDraftParams{AppID: id, AuthorID: pgInt8(2), Channel: "mcp", Files: files})
	if err != nil {
		t.Fatal(err)
	}
	exec(t, "UPDATE app_drafts SET status = 'aprovada' WHERE id = $1", d.ID)
	d, _ = h.q.GetDraftByID(ctx, d.ID)
	a, _ := h.q.GetApp(ctx, id)
	if _, err := h.publishTested(ctx, a, d); err != nil {
		t.Fatal(err)
	}
	var status string
	_ = testPool.QueryRow(ctx, "SELECT status FROM app_drafts WHERE id = $1", d.ID).Scan(&status)
	if status != "aguardando" || latestVersion(t, id) != 1 {
		t.Fatalf("draft %s version %d", status, latestVersion(t, id))
	}
	list := approvals(t, leo)
	if len(list) != 1 || !list[0].Tested {
		t.Fatalf("approvals %+v", list)
	}
	if code := leo.do("POST", approvalPath(list[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	var tested bool
	_ = testPool.QueryRow(ctx, "SELECT COALESCE(tested, false) FROM app_versions WHERE app_id = $1 AND number = 2", id).Scan(&tested)
	if latestVersion(t, id) != 2 || !tested {
		t.Fatalf("version %d tested %v", latestVersion(t, id), tested)
	}
}

// --- Task 6: só pelo Claude e chaves do MCP ---

func onlyClaude(t *testing.T) {
	t.Helper()
	exec(t, "UPDATE security_settings SET code_channel = 'mcp'")
}

func TestOnlyClaudeRefusesSiteUpload(t *testing.T) {
	_, _, ana, _, _ := govTeam(t)
	onlyClaude(t)
	var e struct {
		Error string `json:"error"`
	}
	if code := ana.do("POST", "/api/scripts/import", importBody("Pelo site"), &e); code != http.StatusForbidden || e.Error != msgOnlyClaude {
		t.Fatalf("import %d %q", code, e.Error)
	}
}

func TestOnlyClaudeAcceptsMCP(t *testing.T) {
	h, _, _, _, _ := govTeam(t)
	onlyClaude(t)
	code, raw, err := h.callAPI(withMCPPerson(context.Background(), 2), "POST", "/api/scripts/import", importBody("Pelo Claude"))
	if err != nil || code != http.StatusCreated {
		t.Fatalf("%d %s %v", code, raw, err)
	}
}

func TestOnlyClaudeKeepsAppRunning(t *testing.T) {
	h, _, ana, _, id := approvedScript(t, "risk")
	onlyClaude(t)
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("approved app blocked %q", r)
	}
	// Sending a new version from the site is refused; the old one keeps running.
	var e struct {
		Error string `json:"error"`
	}
	if code := ana.do("POST", appPath(id, "/sources"), map[string]any{"files": b64files(map[string]string{"main.py": "print(2)"})}, &e); code != http.StatusForbidden || e.Error != msgOnlyClaude {
		t.Fatalf("sources %d %q", code, e.Error)
	}
	if code := ana.do("POST", appPath(id, "/drafts"), map[string]any{"files": b64files(map[string]string{"main.py": "print(2)"})}, &e); code != http.StatusForbidden || e.Error != msgOnlyClaude {
		t.Fatalf("draft %d %q", code, e.Error)
	}
	if r := blockedReason(t, h, id); r != "" {
		t.Fatalf("blocked %q", r)
	}
}

func TestOnlyClaudeFolderVersionHeld(t *testing.T) {
	h, _, _, leo, id := approvedScript(t, "risk")
	onlyClaude(t)
	ctx := context.Background()
	a, _ := h.q.GetApp(ctx, id)
	if err := os.WriteFile(filepath.Join(a.FolderPath, "main.py"), []byte("print('pasta')"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.checkVersion(ctx, a); err != nil {
		t.Fatal(err)
	}
	if r := blockedReason(t, h, id); r != msgOnlyClaude {
		t.Fatalf("folder version %q", r)
	}
	list := approvals(t, leo)
	if len(list) != 1 {
		t.Fatalf("approvals %+v", list)
	}
	var e struct {
		Error string `json:"error"`
	}
	if code := leo.do("POST", approvalPath(list[0].ID, "approve"), nil, &e); code != http.StatusConflict || e.Error != msgOnlyClaude {
		t.Fatalf("approve %d %q", code, e.Error)
	}
}

func TestOnlyClaudeArtifact(t *testing.T) {
	h, _, ana, _, _ := govTeam(t)
	onlyClaude(t)
	var e struct {
		Error string `json:"error"`
	}
	if code := ana.do("POST", "/api/artifacts", linkBody("Site", "https://exemplo.com/a"), &e); code != http.StatusForbidden || e.Error != msgOnlyClaude {
		t.Fatalf("artifact %d %q", code, e.Error)
	}
	code, raw, err := h.callAPI(withMCPPerson(context.Background(), 2), "POST", "/api/artifacts", linkBody("Claude", "https://exemplo.com/b"))
	if err != nil || code != http.StatusCreated {
		t.Fatalf("mcp artifact %d %s %v", code, raw, err)
	}
	var d struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(raw, &d)
	if code := ana.do("POST", "/api/artifacts/"+itoa(d.ID)+"/versions", linkBody("Claude", "https://exemplo.com/c"), &e); code != http.StatusForbidden || e.Error != msgOnlyClaude {
		t.Fatalf("artifact version %d %q", code, e.Error)
	}
	var ch string
	if err := testPool.QueryRow(context.Background(), "SELECT channel FROM artifact_versions WHERE artifact_id = $1", d.ID).Scan(&ch); err != nil || ch != "mcp" {
		t.Fatalf("channel %q %v", ch, err)
	}
}

func TestMCPRestrictedRefusesWithoutPerm(t *testing.T) {
	_, admin, _, bia, _ := govTeam(t)
	exec(t, "UPDATE security_settings SET mcp_restricted = TRUE")
	setRole(t, "admin", "manage_roles", "manage_people", "manage_areas", "use_mcp")
	var e struct {
		Error string `json:"error"`
	}
	if code := bia.do("POST", "/api/mcp/codes", map[string]string{"name": "PC"}, &e); code != http.StatusForbidden || e.Error != msgNoMCP {
		t.Fatalf("code %d %q", code, e.Error)
	}
	newMCPCode(t, admin)
}

func TestMCPConnectionStopsWhenPermRemoved(t *testing.T) {
	h, _, _, bia, _ := govTeam(t)
	out := newMCPCode(t, bia)
	ctx := context.Background()
	if _, _, err := h.mcpConnection(ctx, out.Code); err != nil {
		t.Fatal(err)
	}
	exec(t, "UPDATE security_settings SET mcp_restricted = TRUE")
	if _, _, err := h.mcpConnection(ctx, out.Code); !errors.Is(err, errMCPUnauthorized) {
		t.Fatalf("still connects: %v", err)
	}
}

func TestAdminListsAndRevokesMCP(t *testing.T) {
	_, admin, _, bia, _ := govTeam(t)
	newMCPCode(t, bia)
	var list []struct {
		ID         int64  `json:"id"`
		PersonName string `json:"person_name"`
		Name       string `json:"name"`
		Revoked    bool   `json:"revoked"`
	}
	if code := bia.do("GET", "/api/admin/mcp-connections", nil, nil); code != http.StatusForbidden {
		t.Fatalf("member lists %d", code)
	}
	if code := admin.do("GET", "/api/admin/mcp-connections", nil, &list); code != http.StatusOK || len(list) != 1 || list[0].PersonName != "Bia" || list[0].Name != "Notebook" || list[0].Revoked {
		t.Fatalf("list %d %+v", code, list)
	}
	if code := admin.do("DELETE", "/api/admin/mcp-connections/"+itoa(list[0].ID), nil, nil); code != http.StatusNoContent {
		t.Fatalf("revoke %d", code)
	}
	admin.do("GET", "/api/admin/mcp-connections", nil, &list)
	if !list[0].Revoked {
		t.Fatalf("not revoked %+v", list)
	}
	var n int
	testPool.QueryRow(context.Background(), "SELECT count(*) FROM governance_events WHERE kind = 'mcp'").Scan(&n)
	if n != 1 {
		t.Fatalf("events %d", n)
	}
}

// --- Task 7: artifacts e links externos ---

func addArtifactVersionT(t *testing.T, c *client, id int64, content string) {
	t.Helper()
	body := fileBody("Relatório", "rel.html", []byte(content))
	if code := c.do("POST", "/api/artifacts/"+itoa(id)+"/versions", body, nil); code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("add version %d", code)
	}
}

func artifactCurrent(t *testing.T, c *client, id int64) int32 {
	t.Helper()
	var d artifactDetailJSON
	if code := c.do("GET", "/api/artifacts/"+itoa(id), nil, &d); code != http.StatusOK {
		t.Fatalf("get artifact %d", code)
	}
	return d.Current
}

// approvedArtifact: Ana's HTML artifact, approved by Leo for the area.
func approvedArtifact(t *testing.T, rule string) (h *Handler, ana, bia, leo *client, id int64) {
	t.Helper()
	h, _, ana, bia, leo = govTeam(t)
	h.cfg.SharedDomain = "ziv.test"
	setRole(t, "criador", "register_apps", "share_external")
	id = createArtifact(t, ana, fileBody("Relatório", "rel.html", []byte(pageHTML))).ID
	if code := leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), areaOnly, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	exec(t, "UPDATE security_settings SET version_rule = $1", rule)
	return
}

func TestNewArtifactWaitsForLeader(t *testing.T) {
	_, _, ana, bia, leo := govTeam(t)
	id := createArtifact(t, ana, fileBody("Relatório", "rel.html", []byte(pageHTML))).ID
	if code := bia.do("GET", "/api/artifacts/"+itoa(id), nil, nil); code != http.StatusNotFound {
		t.Fatalf("pending artifact visible %d", code)
	}
	list := approvals(t, leo)
	if len(list) != 1 || list[0].Kind != "artifact" || list[0].Reason != "new" {
		t.Fatalf("approvals %+v", list)
	}
	if code := leo.do("POST", approvalPath(list[0].ID, "approve"), areaOnly, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	if n := artifactCurrent(t, bia, id); n != 1 {
		t.Fatalf("current %d", n)
	}
}

func TestArtifactVersionRiskOnlyWithExternalLink(t *testing.T) {
	_, ana, bia, leo, id := approvedArtifact(t, "risk")
	addArtifactVersionT(t, ana, id, "<html>v2</html>")
	if n := artifactCurrent(t, bia, id); n != 2 {
		t.Fatalf("v2 without link should go live, current %d", n)
	}
	if code := ana.do("POST", linksPath(id), openLinkBody(), nil); code != http.StatusCreated {
		t.Fatalf("link %d", code)
	}
	addArtifactVersionT(t, ana, id, "<html>v3</html>")
	if n := artifactCurrent(t, bia, id); n != 2 {
		t.Fatalf("v3 with link should wait, current %d", n)
	}
	list := approvals(t, leo)
	if len(list) != 2 || list[0].Status != "open" || len(list[0].RiskAdded) != 1 || list[0].RiskAdded[0] != "tem link externo ativo" || list[1].Status != "auto" {
		t.Fatalf("approvals %+v", list)
	}
}

func TestExternalLatestShowsLastApproved(t *testing.T) {
	h, ana, _, leo, id := approvedArtifact(t, "every")
	if code := ana.do("POST", linksPath(id), openLinkBody(), nil); code != http.StatusCreated {
		t.Fatalf("link %d", code)
	}
	addArtifactVersionT(t, ana, id, "<html>v2</html>")
	ctx := context.Background()
	var linkID int64
	testPool.QueryRow(ctx, "SELECT id FROM external_links WHERE artifact_id = $1", id).Scan(&linkID)
	l, err := h.q.GetExternalLink(ctx, linkID)
	if err != nil {
		t.Fatal(err)
	}
	if _, v, err := h.linkState(ctx, l, time.Now()); err != nil || v.Number != 1 {
		t.Fatalf("link shows %d %v", v.Number, err)
	}
	if code := leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), nil, nil); code != http.StatusOK {
		t.Fatalf("approve %d", code)
	}
	if _, v, err := h.linkState(ctx, l, time.Now()); err != nil || v.Number != 2 {
		t.Fatalf("link shows %d %v", v.Number, err)
	}
}

func TestNoExternalLinkBeforeApproval(t *testing.T) {
	h, _, ana, _, _ := govTeam(t)
	h.cfg.SharedDomain = "ziv.test"
	setRole(t, "criador", "register_apps", "share_external")
	id := createArtifact(t, ana, fileBody("Relatório", "rel.html", []byte(pageHTML))).ID
	var e struct {
		Error string `json:"error"`
	}
	if code := ana.do("POST", linksPath(id), openLinkBody(), &e); code != http.StatusConflict || e.Error != "Este artifact ainda espera aprovação." {
		t.Fatalf("link %d %q", code, e.Error)
	}
}
