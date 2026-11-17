package handler

// Governança (ADR 046): approval requests with steps. The area leader always
// approves (and, for something new, picks who uses it); TI approves too when
// the company turns it on. A version or ficha change asks again according to
// the version rule (company default, an item may set its own).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/governance"
	"github.com/HenryKropp/app-hub/backend/internal/risk"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	msgOnlyClaude     = "Esta empresa só aceita versões enviadas pelo Claude."
	msgWaitingLeader  = "Aguardando o líder de %s."
	msgWaitingIT      = "Aguardando a TI."
	msgRejected       = "Recusado por %s: %s"
	msgNoMCP          = "Seu cargo não tem permissão para conectar o Claude ao Ziv. Fale com quem administra a empresa."
	msgPickAudience   = "Escolha quem vai usar."
	msgGiveReason     = "Diga o motivo."
	msgResolved       = "Este pedido já foi resolvido."
	msgCannotDecide   = "Você não pode decidir este pedido."
	msgLoosenOnlyAdm  = "Só o administrador deixa a regra mais leve que o padrão da empresa."
	msgRequestMissing = "Pedido não encontrado."
)

// errApplyLater marks request kinds whose apply arrives with the paths that
// create them (drafts and area moves, plan Task 5).
var errApplyLater = errors.New("governance: apply not wired for this request")

type audienceIn struct {
	Mode      string  `json:"mode"` // area | company | custom
	AreaIDs   []int64 `json:"area_ids"`
	PersonIDs []int64 `json:"person_ids"`
}

func (a *audienceIn) valid() bool {
	return a != nil && (a.Mode == "area" || a.Mode == "company" || a.Mode == "custom")
}

type requestIn struct {
	Kind              string // app | artifact
	AppID, ArtifactID int64
	AreaID            int64
	Reason            string
	DraftID           int64
	ArtifactVersionID int64
	TargetAreaID      int64
	AppVersion        int32
	Risk              risk.Set
	Added             []string
	Channel           string
	Tested            bool
	RequesterID       int64
}

type requestOut struct {
	ID      int64
	Auto    bool
	Applied bool
	Waiting string
}

func optInt8(n int64) pgtype.Int8 { return pgtype.Int8{Int64: n, Valid: n != 0} }

// itemRule reads whether the item was ever approved and its own rule.
func (h *Handler) itemRule(ctx context.Context, kind string, appID, artifactID int64) (bool, *governance.Rule, error) {
	var rule pgtype.Text
	var ever bool
	if kind == "app" {
		g, err := h.q.GetAppGovernance(ctx, appID)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		} else if err != nil {
			return false, nil, err
		}
		rule, ever = g.VersionRule, g.EverApproved
	} else {
		a, err := h.q.GetArtifact(ctx, artifactID)
		if err != nil {
			return false, nil, err
		}
		rule, ever = a.VersionRule, a.EverApproved
	}
	if !rule.Valid {
		return ever, nil, nil
	}
	r := governance.Rule(rule.String)
	return ever, &r, nil
}

// openRequest records that something new arrived for an item and whether it
// needs approval. An open request of the same item is replaced. Something an
// item never approved stays a "new" request whatever arrives.
func (h *Handler) openRequest(ctx context.Context, in requestIn) (requestOut, error) {
	gs, err := h.q.GetGovernanceSettings(ctx)
	if err != nil {
		return requestOut{}, err
	}
	ever, itemRule, err := h.itemRule(ctx, in.Kind, in.AppID, in.ArtifactID)
	if err != nil {
		return requestOut{}, err
	}
	if in.Kind == "app" && !ever {
		// Keep the app pending (hidden) until its first approval.
		if _, err := h.q.GetAppGovernance(ctx, in.AppID); errors.Is(err, pgx.ErrNoRows) {
			if err := h.q.UpsertAppGovernance(ctx, sqlc.UpsertAppGovernanceParams{AppID: in.AppID, ApprovedRisk: []byte("{}")}); err != nil {
				return requestOut{}, err
			}
		} else if err != nil {
			return requestOut{}, err
		}
	}
	reason := in.Reason
	if !ever && reason != governance.ReasonArea {
		reason = governance.ReasonNew
	}
	rule := governance.Effective(governance.Rule(gs.VersionRule), itemRule)
	need := governance.Decide(rule, ever, in.Added, gs.ItApproval, reason)

	if in.Kind == "app" {
		_, err = h.q.ReplaceOpenAppRequest(ctx, pgInt8(in.AppID))
	} else {
		_, err = h.q.ReplaceOpenArtifactRequest(ctx, pgInt8(in.ArtifactID))
	}
	if err != nil {
		return requestOut{}, err
	}
	riskJSON, _ := json.Marshal(in.Risk)
	added := in.Added
	if added == nil {
		added = []string{}
	}
	addedJSON, _ := json.Marshal(added)
	status := "open"
	if !need.Required {
		status = "auto"
	}
	channel := in.Channel
	if channel == "" {
		channel = originOf(ctx)
	}
	p := sqlc.InsertApprovalRequestParams{ItemKind: in.Kind, AppID: optInt8(in.AppID), ArtifactID: optInt8(in.ArtifactID),
		Reason: reason, DraftID: optInt8(in.DraftID), ArtifactVersionID: optInt8(in.ArtifactVersionID),
		TargetAreaID: optInt8(in.TargetAreaID), Risk: riskJSON, RiskAdded: addedJSON, Channel: channel,
		Tested: in.Tested, Status: status, RequestedBy: optInt8(in.RequesterID)}
	if in.AppVersion != 0 {
		p.AppVersion = pgtype.Int4{Int32: in.AppVersion, Valid: true}
	}
	req, err := h.q.InsertApprovalRequest(ctx, p)
	if err != nil {
		return requestOut{}, err
	}
	if !need.Required {
		if err := h.applyRequest(ctx, req, nil, in.RequesterID); err != nil {
			return requestOut{}, err
		}
		return requestOut{ID: req.ID, Auto: true, Applied: true}, nil
	}

	areaID, _, hasLeader, err := h.requestArea(ctx, req)
	if err != nil {
		return requestOut{}, err
	}
	auto := map[string]string{}
	if in.RequesterID != 0 {
		if me, err := h.q.GetPerson(ctx, in.RequesterID); err == nil {
			acc, err := h.loadAccess(ctx, me)
			if err != nil {
				return requestOut{}, err
			}
			auto = governance.AutoSteps(need.Steps, areaID, hasLeader, approverOf(acc))
		}
	}
	allAuto := len(auto) == len(need.Steps)
	for _, s := range need.Steps {
		sp := sqlc.InsertApprovalStepParams{RequestID: req.ID, Step: s}
		if ob, ok := auto[s]; ok && !allAuto {
			sp.Decision = pgtype.Text{String: "approved", Valid: true}
			sp.DecidedBy, sp.OnBehalf = optInt8(in.RequesterID), ob
		}
		if err := h.q.InsertApprovalStep(ctx, sp); err != nil {
			return requestOut{}, err
		}
	}
	if allAuto {
		// Apply first: if it fails the steps stay open and the requester,
		// who may decide them, sees the request in Aprovações.
		// Nobody else had to pick the audience: the item keeps the access
		// it was created with; the leader can change it in Acesso.
		if err := h.applyRequest(ctx, req, nil, in.RequesterID); err != nil {
			return requestOut{}, err
		}
		for _, s := range need.Steps {
			if _, err := h.q.DecideApprovalStep(ctx, sqlc.DecideApprovalStepParams{Decision: pgtype.Text{String: "approved", Valid: true},
				DecidedBy: optInt8(in.RequesterID), OnBehalf: governance.OnBehalfSelf, RequestID: req.ID, Step: s}); err != nil {
				return requestOut{}, err
			}
		}
		if _, err := h.q.CloseApprovalRequest(ctx, sqlc.CloseApprovalRequestParams{Status: "approved", ID: req.ID}); err != nil {
			return requestOut{}, err
		}
		return requestOut{ID: req.ID, Applied: true}, nil
	}
	waiting, err := h.requestMessage(ctx, req)
	return requestOut{ID: req.ID, Waiting: waiting}, err
}

func approverOf(acc *access.Person) governance.Approver {
	return governance.Approver{PersonID: acc.ID, Perms: acc.Perms, Leads: acc.Leads}
}

// requestArea is the area whose leader decides: the target for an area move,
// else the item's.
func (h *Handler) requestArea(ctx context.Context, req sqlc.ApprovalRequest) (int64, string, bool, error) {
	var areaID int64
	switch {
	case req.Reason == governance.ReasonArea && req.TargetAreaID.Valid:
		areaID = req.TargetAreaID.Int64
	case req.ItemKind == "app":
		a, err := h.q.GetApp(ctx, req.AppID.Int64)
		if err != nil {
			return 0, "", false, err
		}
		areaID = a.AreaID
	default:
		a, err := h.q.GetArtifact(ctx, req.ArtifactID.Int64)
		if err != nil {
			return 0, "", false, err
		}
		areaID = a.AreaID
	}
	area, err := h.q.GetArea(ctx, areaID)
	if err != nil {
		return 0, "", false, err
	}
	has, err := h.q.AreaHasLeader(ctx, areaID)
	return areaID, area.Name, has, err
}

// requestMessage: why the request still holds the item — waiting for the
// leader, for TI, or refused.
func (h *Handler) requestMessage(ctx context.Context, req sqlc.ApprovalRequest) (string, error) {
	steps, err := h.q.ListApprovalSteps(ctx, req.ID)
	if err != nil {
		return "", err
	}
	for _, s := range steps {
		if s.Decision.String == "rejected" {
			return fmt.Sprintf(msgRejected, s.DecidedByName, s.Reason), nil
		}
	}
	if req.Status != "open" {
		return "", nil
	}
	for _, s := range steps {
		if s.Decision.Valid {
			continue
		}
		if s.Step == governance.StepIT {
			return msgWaitingIT, nil
		}
		_, name, _, err := h.requestArea(ctx, req)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(msgWaitingLeader, name), nil
	}
	return "", nil
}

// governanceBlock: why an app may not run because of approvals ("" = may).
// A never-approved app waits for its first approval; an approved one is held
// only by a folder version (no draft) still waiting or refused.
func (h *Handler) governanceBlock(ctx context.Context, appID int64) (string, error) {
	g, err := h.q.GetAppGovernance(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	req, err := h.q.LatestAppRequest(ctx, pgInt8(appID))
	if errors.Is(err, pgx.ErrNoRows) {
		if !g.EverApproved {
			return "Este app está esperando aprovação.", nil
		}
		return "", nil
	} else if err != nil {
		return "", err
	}
	if g.EverApproved && (req.DraftID.Valid || req.Reason == governance.ReasonNew || req.Reason == governance.ReasonArea) {
		return "", nil
	}
	if req.Status != "open" && req.Status != "rejected" {
		if !g.EverApproved {
			return "Este app está esperando aprovação.", nil
		}
		return "", nil
	}
	return h.requestMessage(ctx, req)
}

// applyRequest makes an approved request real. A nil aud keeps the item's
// current access.
func (h *Handler) applyRequest(ctx context.Context, req sqlc.ApprovalRequest, aud *audienceIn, actorID int64) error {
	if req.DraftID.Valid || req.Reason == governance.ReasonArea {
		return errApplyLater
	}
	if req.ItemKind == "app" {
		appID := req.AppID.Int64
		approvedRisk := req.Risk
		if len(approvedRisk) == 0 {
			approvedRisk = []byte("{}")
		}
		if err := h.q.UpsertAppGovernance(ctx, sqlc.UpsertAppGovernanceParams{AppID: appID, ApprovedVersion: req.AppVersion,
			ApprovedRisk: approvedRisk, EverApproved: true}); err != nil {
			return err
		}
		if req.Reason == governance.ReasonNew {
			a, err := h.q.GetApp(ctx, appID)
			if err != nil {
				return err
			}
			if aud == nil {
				return nil
			}
			return h.setAudience(ctx, "app", appID, a.AreaID, aud, actorID)
		}
		return nil
	}
	artID := req.ArtifactID.Int64
	if req.ArtifactVersionID.Valid {
		if err := h.q.SetArtifactVersionApproved(ctx, sqlc.SetArtifactVersionApprovedParams{ID: req.ArtifactVersionID.Int64, Approved: true}); err != nil {
			return err
		}
	}
	if err := h.q.SetArtifactEverApproved(ctx, sqlc.SetArtifactEverApprovedParams{ID: artID, EverApproved: true}); err != nil {
		return err
	}
	if req.Reason == governance.ReasonNew {
		a, err := h.q.GetArtifact(ctx, artID)
		if err != nil {
			return err
		}
		if aud == nil {
			return nil
		}
		return h.setAudience(ctx, "artifact", artID, a.AreaID, aud, actorID)
	}
	return nil
}

// setAudience applies who uses an item: só a área, the whole company, or the
// area plus chosen areas and people. It records a governance event.
func (h *Handler) setAudience(ctx context.Context, kind string, id, areaID int64, aud *audienceIn, actorID int64) error {
	restricted := aud.Mode != "company"
	areas := []int64{}
	people := []int64{}
	if restricted {
		areas = append(areas, areaID)
	}
	if aud.Mode == "custom" {
		for _, a := range aud.AreaIDs {
			if a != areaID {
				areas = append(areas, a)
			}
		}
		people = append(people, aud.PersonIDs...)
	}
	err := h.q.InTx(ctx, func(q *sqlc.Queries) error {
		if kind == "app" {
			if err := q.SetAppRestricted(ctx, sqlc.SetAppRestrictedParams{ID: id, Restricted: restricted}); err != nil {
				return err
			}
			if err := q.ClearAppViewers(ctx, id); err != nil {
				return err
			}
			for _, a := range areas {
				if err := q.AddAppViewerArea(ctx, sqlc.AddAppViewerAreaParams{AppID: id, AreaID: pgInt8(a)}); err != nil {
					return err
				}
			}
			for _, p := range people {
				if err := q.AddAppViewerPerson(ctx, sqlc.AddAppViewerPersonParams{AppID: id, PersonID: pgInt8(p)}); err != nil {
					return err
				}
			}
			return nil
		}
		if err := q.SetArtifactRestricted(ctx, sqlc.SetArtifactRestrictedParams{ID: id, Restricted: restricted}); err != nil {
			return err
		}
		if err := q.ClearArtifactViewers(ctx, id); err != nil {
			return err
		}
		for _, a := range areas {
			if err := q.AddArtifactViewerArea(ctx, sqlc.AddArtifactViewerAreaParams{ArtifactID: id, AreaID: pgInt8(a)}); err != nil {
				return err
			}
		}
		for _, p := range people {
			if err := q.AddArtifactViewerPerson(ctx, sqlc.AddArtifactViewerPersonParams{ArtifactID: id, PersonID: pgInt8(p)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"mode": aud.Mode, "area_ids": areas, "person_ids": people})
	return h.q.InsertGovernanceEvent(ctx, eventFor(kind, id, "audience", actorID, detail))
}

func eventFor(kind string, id int64, what string, actorID int64, detail []byte) sqlc.InsertGovernanceEventParams {
	e := sqlc.InsertGovernanceEventParams{ItemKind: pgtype.Text{String: kind, Valid: true}, Kind: what, ActorID: optInt8(actorID), Detail: detail}
	if kind == "app" {
		e.AppID = pgInt8(id)
	} else {
		e.ArtifactID = pgInt8(id)
	}
	return e
}

// ---- routes ----

type stepJSON struct {
	Step          string     `json:"step"`
	Decision      string     `json:"decision"`
	DecidedBy     string     `json:"decided_by"`
	DecidedAt     *time.Time `json:"decided_at"`
	Reason        string     `json:"reason"`
	OnBehalf      string     `json:"on_behalf"`
}

type requestJSON struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	ItemID      int64      `json:"item_id"`
	ItemName    string     `json:"item_name"`
	Area        string     `json:"area"`
	Reason      string     `json:"reason"`
	RiskAdded   []string   `json:"risk_added"`
	Channel     string     `json:"channel"`
	Tested      bool       `json:"tested"`
	Status      string     `json:"status"`
	RequestedBy string     `json:"requested_by"`
	CreatedAt   time.Time  `json:"created_at"`
	Steps       []stepJSON `json:"steps"`
	Waiting     string     `json:"waiting,omitempty"`
}

func (h *Handler) requestView(ctx context.Context, req sqlc.ApprovalRequest) (requestJSON, error) {
	out := requestJSON{ID: req.ID, Kind: req.ItemKind, Reason: req.Reason, Channel: req.Channel, Tested: req.Tested,
		Status: req.Status, CreatedAt: req.CreatedAt.Time, RiskAdded: []string{}, Steps: []stepJSON{}}
	_ = json.Unmarshal(req.RiskAdded, &out.RiskAdded)
	if req.ItemKind == "app" {
		a, err := h.q.GetApp(ctx, req.AppID.Int64)
		if err != nil {
			return out, err
		}
		out.ItemID, out.ItemName = a.ID, a.Name
	} else {
		a, err := h.q.GetArtifact(ctx, req.ArtifactID.Int64)
		if err != nil {
			return out, err
		}
		out.ItemID, out.ItemName = a.ID, a.Name
	}
	_, area, _, err := h.requestArea(ctx, req)
	if err != nil {
		return out, err
	}
	out.Area = area
	if req.RequestedBy.Valid {
		if p, err := h.q.GetPerson(ctx, req.RequestedBy.Int64); err == nil {
			out.RequestedBy = p.Name
		}
	}
	steps, err := h.q.ListApprovalSteps(ctx, req.ID)
	if err != nil {
		return out, err
	}
	for _, s := range steps {
		j := stepJSON{Step: s.Step, Decision: s.Decision.String, DecidedBy: s.DecidedByName, Reason: s.Reason,
			OnBehalf: s.OnBehalf}
		if s.DecidedAt.Valid {
			t := s.DecidedAt.Time
			j.DecidedAt = &t
		}
		out.Steps = append(out.Steps, j)
	}
	if req.Status == "open" || req.Status == "rejected" {
		out.Waiting, err = h.requestMessage(ctx, req)
	}
	return out, err
}

// mySteps: the undecided steps of req that acc may decide, with on_behalf,
// and whether another undecided step remains for someone else.
func (h *Handler) mySteps(ctx context.Context, req sqlc.ApprovalRequest, acc *access.Person) (map[string]string, bool, error) {
	areaID, _, hasLeader, err := h.requestArea(ctx, req)
	if err != nil {
		return nil, false, err
	}
	steps, err := h.q.ListApprovalSteps(ctx, req.ID)
	if err != nil {
		return nil, false, err
	}
	mine, others := map[string]string{}, false
	for _, s := range steps {
		if s.Decision.Valid {
			continue
		}
		if ok, ob := governance.MayDecide(s.Step, areaID, hasLeader, approverOf(acc)); ok {
			mine[s.Step] = ob
		} else {
			others = true
		}
	}
	return mine, others, nil
}

func (h *Handler) decidableRequests(ctx context.Context, acc *access.Person) ([]sqlc.ApprovalRequest, error) {
	open, err := h.q.ListOpenRequests(ctx)
	if err != nil {
		return nil, err
	}
	var out []sqlc.ApprovalRequest
	for _, req := range open {
		mine, _, err := h.mySteps(ctx, req, acc)
		if err != nil {
			return nil, err
		}
		if len(mine) > 0 {
			out = append(out, req)
		}
	}
	return out, nil
}

// listApprovals answers GET /api/approvals: what I may decide now, then what
// went live on its own in the last 30 days (for leaders and TI to look at).
func (h *Handler) listApprovals(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	acc, err := h.loadAccess(ctx, me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	reqs, err := h.decidableRequests(ctx, acc)
	if err != nil {
		internalError(w, "approvals", err)
		return
	}
	out := []requestJSON{}
	for _, req := range reqs {
		v, err := h.requestView(ctx, req)
		if err != nil {
			internalError(w, "approval view", err)
			return
		}
		out = append(out, v)
	}
	if len(acc.Leads) > 0 || acc.Has(access.ApproveIT) || acc.Has(access.ManageRoles) {
		autos, err := h.q.ListAutoRequestsSince(ctx, pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -30), Valid: true})
		if err != nil {
			internalError(w, "auto approvals", err)
			return
		}
		for _, req := range autos {
			areaID, _, _, err := h.requestArea(ctx, req)
			if err != nil {
				internalError(w, "auto area", err)
				return
			}
			if !acc.Leads[areaID] && !acc.Has(access.ApproveIT) && !acc.Has(access.ManageRoles) {
				continue
			}
			v, err := h.requestView(ctx, req)
			if err != nil {
				internalError(w, "approval view", err)
				return
			}
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) countApprovals(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	reqs, err := h.decidableRequests(r.Context(), acc)
	if err != nil {
		internalError(w, "approvals count", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": len(reqs)})
}

// decisionStart loads the request under its lock (one decision at a time per
// request). The returned unlock must be called; ok false means w answered.
func (h *Handler) decisionStart(w http.ResponseWriter, r *http.Request, body any) (sqlc.Person, *access.Person, sqlc.ApprovalRequest, map[string]string, bool, func(), bool) {
	noop := func() {}
	me, ok := h.currentPerson(w, r)
	if !ok {
		return me, nil, sqlc.ApprovalRequest{}, nil, false, noop, false
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, msgRequestMissing)
		return me, nil, sqlc.ApprovalRequest{}, nil, false, noop, false
	}
	if err := json.NewDecoder(r.Body).Decode(body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Pedido inválido.")
		return me, nil, sqlc.ApprovalRequest{}, nil, false, noop, false
	}
	ctx := r.Context()
	acc, err := h.loadAccess(ctx, me)
	if err != nil {
		internalError(w, "load access", err)
		return me, nil, sqlc.ApprovalRequest{}, nil, false, noop, false
	}
	// Negative keys: lockApp's ids are app ids, requests use their own space.
	unlock := h.lockApp(-id)
	fail := func() (sqlc.Person, *access.Person, sqlc.ApprovalRequest, map[string]string, bool, func(), bool) {
		unlock()
		return me, nil, sqlc.ApprovalRequest{}, nil, false, noop, false
	}
	req, err := h.q.GetApprovalRequest(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, msgRequestMissing)
		return fail()
	} else if err != nil {
		internalError(w, "get request", err)
		return fail()
	}
	if req.Status != "open" {
		writeError(w, http.StatusConflict, msgResolved)
		return fail()
	}
	mine, others, err := h.mySteps(ctx, req, acc)
	if err != nil {
		internalError(w, "request steps", err)
		return fail()
	}
	if len(mine) == 0 {
		writeError(w, http.StatusForbidden, msgCannotDecide)
		return fail()
	}
	return me, acc, req, mine, others, unlock, true
}

func (h *Handler) approveRequest(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Audience *audienceIn `json:"audience"`
	}
	me, _, req, mine, others, unlock, ok := h.decisionStart(w, r, &b)
	defer unlock()
	if !ok {
		return
	}
	ctx := r.Context()
	_, leaderStep := mine[governance.StepLeader]
	if b.Audience != nil && !b.Audience.valid() {
		b.Audience = nil
	}
	if req.Reason == governance.ReasonNew && leaderStep && b.Audience == nil {
		writeError(w, http.StatusBadRequest, msgPickAudience)
		return
	}
	if b.Audience != nil {
		for _, id := range b.Audience.AreaIDs {
			if _, err := h.q.GetArea(ctx, id); err != nil {
				writeError(w, http.StatusBadRequest, "Escolha as áreas na lista.")
				return
			}
		}
		for _, id := range b.Audience.PersonIDs {
			if _, err := h.q.GetPerson(ctx, id); err != nil {
				writeError(w, http.StatusBadRequest, "Escolha as pessoas na lista.")
				return
			}
		}
	}
	aud := b.Audience
	if aud == nil && len(req.Audience) > 0 {
		aud = &audienceIn{}
		_ = json.Unmarshal(req.Audience, aud)
	}
	if !others {
		// Apply before recording: if it fails, nothing is decided.
		if err := h.applyRequest(ctx, req, aud, me.ID); err != nil {
			internalError(w, "apply request", err)
			return
		}
	} else if b.Audience != nil {
		raw, _ := json.Marshal(b.Audience)
		if err := h.q.SetRequestAudience(ctx, sqlc.SetRequestAudienceParams{ID: req.ID, Audience: raw}); err != nil {
			internalError(w, "request audience", err)
			return
		}
	}
	for step, ob := range mine {
		if _, err := h.q.DecideApprovalStep(ctx, sqlc.DecideApprovalStepParams{Decision: pgtype.Text{String: "approved", Valid: true},
			DecidedBy: pgInt8(me.ID), OnBehalf: ob, RequestID: req.ID, Step: step}); err != nil {
			internalError(w, "decide step", err)
			return
		}
	}
	status := "open"
	if !others {
		status = "approved"
		if _, err := h.q.CloseApprovalRequest(ctx, sqlc.CloseApprovalRequestParams{Status: status, ID: req.ID}); err != nil {
			internalError(w, "close request", err)
			return
		}
	}
	req.Status = status
	waiting, err := h.requestMessage(ctx, req)
	if err != nil {
		internalError(w, "request message", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status, "waiting": waiting})
}

func (h *Handler) rejectRequest(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Reason string `json:"reason"`
	}
	me, _, req, mine, _, unlock, ok := h.decisionStart(w, r, &b)
	defer unlock()
	if !ok {
		return
	}
	reason := strings.TrimSpace(b.Reason)
	if reason == "" {
		writeError(w, http.StatusBadRequest, msgGiveReason)
		return
	}
	ctx := r.Context()
	step, ob := governance.StepLeader, ""
	if o, ok := mine[governance.StepLeader]; ok {
		ob = o
	} else {
		step, ob = governance.StepIT, mine[governance.StepIT]
	}
	if _, err := h.q.DecideApprovalStep(ctx, sqlc.DecideApprovalStepParams{Decision: pgtype.Text{String: "rejected", Valid: true},
		DecidedBy: pgInt8(me.ID), Reason: reason, OnBehalf: ob, RequestID: req.ID, Step: step}); err != nil {
		internalError(w, "decide step", err)
		return
	}
	if _, err := h.q.CloseApprovalRequest(ctx, sqlc.CloseApprovalRequestParams{Status: "rejected", ID: req.ID}); err != nil {
		internalError(w, "close request", err)
		return
	}
	if req.DraftID.Valid {
		if err := h.q.FailDraft(ctx, sqlc.FailDraftParams{ID: req.DraftID.Int64, Reason: fmt.Sprintf(msgRejected, me.Name, reason)}); err != nil {
			internalError(w, "reject draft", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// ---- per-item governance ----

type itemGovernanceJSON struct {
	RuleCompany   string        `json:"rule_company"`
	RuleItem      string        `json:"rule_item"`
	RuleEffective string        `json:"rule_effective"`
	ITOn          bool          `json:"it_on"`
	OpenRequest   *requestJSON  `json:"open_request"`
	History       []requestJSON `json:"history"`
	CanLoosen     bool          `json:"can_loosen"`
	CanChange     bool          `json:"can_change"`
}

func (h *Handler) itemGovernance(ctx context.Context, kind string, id int64, acc *access.Person, can access.Can) (itemGovernanceJSON, error) {
	gs, err := h.q.GetGovernanceSettings(ctx)
	if err != nil {
		return itemGovernanceJSON{}, err
	}
	var appID, artID int64
	if kind == "app" {
		appID = id
	} else {
		artID = id
	}
	_, rule, err := h.itemRule(ctx, kind, appID, artID)
	if err != nil {
		return itemGovernanceJSON{}, err
	}
	out := itemGovernanceJSON{RuleCompany: gs.VersionRule, ITOn: gs.ItApproval, History: []requestJSON{},
		CanLoosen: governance.CanLoosen(acc.Perms), CanChange: can.Access || can.Audience}
	if rule != nil {
		out.RuleItem = string(*rule)
	}
	out.RuleEffective = string(governance.Effective(governance.Rule(gs.VersionRule), rule))
	var reqs []sqlc.ApprovalRequest
	if kind == "app" {
		rows, err := h.q.ListAppRequests(ctx, pgInt8(id))
		if err != nil {
			return out, err
		}
		for _, r := range rows {
			reqs = append(reqs, sqlc.ApprovalRequest{ID: r.ID, ItemKind: r.ItemKind, AppID: r.AppID, ArtifactID: r.ArtifactID,
				Reason: r.Reason, DraftID: r.DraftID, AppVersion: r.AppVersion, ArtifactVersionID: r.ArtifactVersionID,
				TargetAreaID: r.TargetAreaID, Risk: r.Risk, RiskAdded: r.RiskAdded, Channel: r.Channel, Tested: r.Tested,
				Status: r.Status, RequestedBy: r.RequestedBy, CreatedAt: r.CreatedAt, ClosedAt: r.ClosedAt, Audience: r.Audience})
		}
	} else {
		rows, err := h.q.ListArtifactRequests(ctx, pgInt8(id))
		if err != nil {
			return out, err
		}
		for _, r := range rows {
			reqs = append(reqs, sqlc.ApprovalRequest{ID: r.ID, ItemKind: r.ItemKind, AppID: r.AppID, ArtifactID: r.ArtifactID,
				Reason: r.Reason, DraftID: r.DraftID, AppVersion: r.AppVersion, ArtifactVersionID: r.ArtifactVersionID,
				TargetAreaID: r.TargetAreaID, Risk: r.Risk, RiskAdded: r.RiskAdded, Channel: r.Channel, Tested: r.Tested,
				Status: r.Status, RequestedBy: r.RequestedBy, CreatedAt: r.CreatedAt, ClosedAt: r.ClosedAt, Audience: r.Audience})
		}
	}
	for _, r := range reqs {
		v, err := h.requestView(ctx, r)
		if err != nil {
			return out, err
		}
		if r.Status == "open" && out.OpenRequest == nil {
			vv := v
			out.OpenRequest = &vv
		}
		out.History = append(out.History, v)
	}
	return out, nil
}

func (h *Handler) getAppGovernance(w http.ResponseWriter, r *http.Request) {
	a, _, acc, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	out, err := h.itemGovernance(r.Context(), "app", a.ID, acc, can)
	if err != nil {
		internalError(w, "app governance", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getArtifactGovernance(w http.ResponseWriter, r *http.Request) {
	a, _, acc, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	out, err := h.itemGovernance(r.Context(), "artifact", a.ID, acc, can)
	if err != nil {
		internalError(w, "artifact governance", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// setItemRule saves an item's own version rule (null = follow the company).
// Who cares for it or its leader may make it stricter; only the admin makes
// it lighter than the company.
func (h *Handler) setItemRule(w http.ResponseWriter, r *http.Request, kind string, id int64, me sqlc.Person, acc *access.Person, can access.Can) {
	if !(can.Access || can.Audience) {
		writeError(w, http.StatusForbidden, "Você não pode mudar a regra deste item.")
		return
	}
	var b struct {
		Rule *string `json:"rule"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if b.Rule != nil && !governance.Valid(*b.Rule) {
		writeError(w, http.StatusBadRequest, "Regra desconhecida.")
		return
	}
	ctx := r.Context()
	gs, err := h.q.GetGovernanceSettings(ctx)
	if err != nil {
		internalError(w, "governance settings", err)
		return
	}
	company := governance.Rule(gs.VersionRule)
	if b.Rule != nil {
		next := governance.Rule(*b.Rule)
		if next != company && governance.Stricter(company, next) == company && !governance.CanLoosen(acc.Perms) {
			writeError(w, http.StatusForbidden, msgLoosenOnlyAdm)
			return
		}
	}
	var appID, artID int64
	if kind == "app" {
		appID = id
	} else {
		artID = id
	}
	_, before, err := h.itemRule(ctx, kind, appID, artID)
	if err != nil {
		internalError(w, "item rule", err)
		return
	}
	val := pgtype.Text{}
	if b.Rule != nil {
		val = pgtype.Text{String: *b.Rule, Valid: true}
	}
	if kind == "app" {
		err = h.q.SetAppVersionRule(ctx, sqlc.SetAppVersionRuleParams{AppID: id, VersionRule: val})
	} else {
		err = h.q.SetArtifactVersionRule(ctx, sqlc.SetArtifactVersionRuleParams{ID: id, VersionRule: val})
	}
	if err != nil {
		internalError(w, "set item rule", err)
		return
	}
	beforeS := ""
	if before != nil {
		beforeS = string(*before)
	}
	detail, _ := json.Marshal(map[string]string{"before": beforeS, "after": val.String})
	if err := h.q.InsertGovernanceEvent(ctx, eventFor(kind, id, "rule", me.ID, detail)); err != nil {
		internalError(w, "rule event", err)
		return
	}
	out, err := h.itemGovernance(ctx, kind, id, acc, can)
	if err != nil {
		internalError(w, "item governance", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setAppRule(w http.ResponseWriter, r *http.Request) {
	a, me, acc, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	h.setItemRule(w, r, "app", a.ID, me, acc, can)
}

func (h *Handler) setArtifactRule(w http.ResponseWriter, r *http.Request) {
	a, me, acc, can, ok := h.artifactFor(w, r)
	if !ok {
		return
	}
	h.setItemRule(w, r, "artifact", a.ID, me, acc, can)
}
