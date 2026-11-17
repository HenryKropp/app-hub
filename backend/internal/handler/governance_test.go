package handler

import (
	"context"
	"net/http"
	"strconv"
	"testing"
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
	id := importTestScript(t, ana)
	leo.do("POST", approvalPath(approvals(t, leo)[0].ID, "approve"), areaOnly, nil)
	// A new folder version that adds risk asks again; rejecting it blocks the
	// new version only — the approved one stays recorded.
	ctx := context.Background()
	out, err := h.openRequest(ctx, requestIn{Kind: "app", AppID: id, AreaID: 1, Reason: "version", AppVersion: 2,
		Added: []string{"site: api.exemplo.com.br"}, Channel: "manual", RequesterID: 2})
	if err != nil || out.Auto || out.Waiting != "Aguardando o líder de Research." {
		t.Fatalf("open %+v %v", out, err)
	}
	if code := leo.do("POST", approvalPath(out.ID, "reject"), map[string]string{"reason": "site novo"}, nil); code != http.StatusOK {
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
