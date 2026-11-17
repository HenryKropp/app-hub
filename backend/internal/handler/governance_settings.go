package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/governance"
	"github.com/jackc/pgx/v5/pgtype"
)

// governanceSettingsJSON is how the company runs approvals (ADR 046).
type governanceSettingsJSON struct {
	ITApproval    bool   `json:"it_approval"`
	VersionRule   string `json:"version_rule"`
	CodeChannel   string `json:"code_channel"`
	MCPRestricted bool   `json:"mcp_restricted"`
}

func settingsJSON(gs sqlc.GetGovernanceSettingsRow) governanceSettingsJSON {
	return governanceSettingsJSON{ITApproval: gs.ItApproval, VersionRule: gs.VersionRule, CodeChannel: gs.CodeChannel, MCPRestricted: gs.McpRestricted}
}

func (h *Handler) getGovernanceSettings(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	gs, err := h.q.GetGovernanceSettings(r.Context())
	if err != nil {
		internalError(w, "governance settings", err)
		return
	}
	writeJSON(w, http.StatusOK, settingsJSON(gs))
}

func (h *Handler) updateGovernanceSettings(w http.ResponseWriter, r *http.Request) {
	me, _, ok := h.requirePerm(w, r, access.ManageRoles)
	if !ok {
		return
	}
	var b governanceSettingsJSON
	if !readJSON(w, r, &b) {
		return
	}
	if !governance.Valid(b.VersionRule) {
		writeError(w, http.StatusBadRequest, "Escolha quando as versões passam por aprovação.")
		return
	}
	if b.CodeChannel != "any" && b.CodeChannel != "mcp" {
		writeError(w, http.StatusBadRequest, "Escolha por onde o código pode chegar.")
		return
	}
	ctx := r.Context()
	before, err := h.q.GetGovernanceSettings(ctx)
	if err != nil {
		internalError(w, "governance settings", err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"before": settingsJSON(before), "after": b})
	err = h.q.InTx(ctx, func(q *sqlc.Queries) error {
		if err := q.UpdateGovernanceSettings(ctx, sqlc.UpdateGovernanceSettingsParams{ItApproval: b.ITApproval,
			VersionRule: b.VersionRule, CodeChannel: b.CodeChannel, McpRestricted: b.MCPRestricted}); err != nil {
			return err
		}
		if b.ITApproval && !before.ItApproval {
			// What is already waiting now also waits for TI.
			if err := q.AddITStepToOpen(ctx); err != nil {
				return err
			}
		}
		return q.InsertGovernanceEvent(ctx, sqlc.InsertGovernanceEventParams{Kind: "settings", ActorID: optInt8(me.ID), Detail: detail})
	})
	if err != nil {
		internalError(w, "update governance settings", err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type govEventJSON struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	ItemKind  string          `json:"item_kind"`
	ItemID    int64           `json:"item_id"`
	Actor     string          `json:"actor"`
	Detail    json.RawMessage `json:"detail"`
	CreatedAt time.Time       `json:"created_at"`
}

type historyEntryJSON struct {
	Type    string        `json:"type"` // request | event
	At      time.Time     `json:"at"`
	Request *requestJSON  `json:"request,omitempty"`
	Event   *govEventJSON `json:"event,omitempty"`
}

// governanceHistory answers "who approved what, when": requests with their
// steps and the other governance events, newest first, for one item
// (?item=app:12 or artifact:3) or the whole company.
func (h *Handler) governanceHistory(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	ctx := r.Context()
	var appID, artID pgtype.Int8
	if item := r.URL.Query().Get("item"); item != "" {
		kind, raw, _ := strings.Cut(item, ":")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || (kind != "app" && kind != "artifact") {
			writeError(w, http.StatusBadRequest, "Item inválido.")
			return
		}
		if kind == "app" {
			appID = pgInt8(id)
		} else {
			artID = pgInt8(id)
		}
	}
	var reqs []sqlc.ApprovalRequest
	switch {
	case appID.Valid:
		rows, err := h.q.ListAppRequests(ctx, appID)
		if err != nil {
			internalError(w, "history requests", err)
			return
		}
		for _, x := range rows {
			reqs = append(reqs, sqlc.ApprovalRequest{ID: x.ID, ItemKind: x.ItemKind, AppID: x.AppID, ArtifactID: x.ArtifactID,
				Reason: x.Reason, DraftID: x.DraftID, AppVersion: x.AppVersion, ArtifactVersionID: x.ArtifactVersionID,
				TargetAreaID: x.TargetAreaID, Risk: x.Risk, RiskAdded: x.RiskAdded, Channel: x.Channel, Tested: x.Tested,
				Status: x.Status, RequestedBy: x.RequestedBy, CreatedAt: x.CreatedAt, ClosedAt: x.ClosedAt, Audience: x.Audience})
		}
	case artID.Valid:
		rows, err := h.q.ListArtifactRequests(ctx, artID)
		if err != nil {
			internalError(w, "history requests", err)
			return
		}
		for _, x := range rows {
			reqs = append(reqs, sqlc.ApprovalRequest{ID: x.ID, ItemKind: x.ItemKind, AppID: x.AppID, ArtifactID: x.ArtifactID,
				Reason: x.Reason, DraftID: x.DraftID, AppVersion: x.AppVersion, ArtifactVersionID: x.ArtifactVersionID,
				TargetAreaID: x.TargetAreaID, Risk: x.Risk, RiskAdded: x.RiskAdded, Channel: x.Channel, Tested: x.Tested,
				Status: x.Status, RequestedBy: x.RequestedBy, CreatedAt: x.CreatedAt, ClosedAt: x.ClosedAt, Audience: x.Audience})
		}
	default:
		rows, err := h.q.ListRecentRequests(ctx)
		if err != nil {
			internalError(w, "history requests", err)
			return
		}
		for _, x := range rows {
			reqs = append(reqs, sqlc.ApprovalRequest{ID: x.ID, ItemKind: x.ItemKind, AppID: x.AppID, ArtifactID: x.ArtifactID,
				Reason: x.Reason, DraftID: x.DraftID, AppVersion: x.AppVersion, ArtifactVersionID: x.ArtifactVersionID,
				TargetAreaID: x.TargetAreaID, Risk: x.Risk, RiskAdded: x.RiskAdded, Channel: x.Channel, Tested: x.Tested,
				Status: x.Status, RequestedBy: x.RequestedBy, CreatedAt: x.CreatedAt, ClosedAt: x.ClosedAt, Audience: x.Audience})
		}
	}
	out := []historyEntryJSON{}
	for _, x := range reqs {
		v, err := h.requestView(ctx, x)
		if err != nil {
			internalError(w, "history request", err)
			return
		}
		at := x.CreatedAt.Time
		if x.ClosedAt.Valid {
			at = x.ClosedAt.Time
		}
		out = append(out, historyEntryJSON{Type: "request", At: at, Request: &v})
	}
	events, err := h.q.ListGovernanceEvents(ctx, sqlc.ListGovernanceEventsParams{AppID: appID, ArtifactID: artID})
	if err != nil {
		internalError(w, "history events", err)
		return
	}
	for _, e := range events {
		ev := govEventJSON{ID: e.ID, Kind: e.Kind, ItemKind: e.ItemKind.String, Actor: e.ActorName, Detail: e.Detail, CreatedAt: e.CreatedAt.Time}
		if e.AppID.Valid {
			ev.ItemID = e.AppID.Int64
		} else if e.ArtifactID.Valid {
			ev.ItemID = e.ArtifactID.Int64
		}
		out = append(out, historyEntryJSON{Type: "event", At: ev.CreatedAt, Event: &ev})
	}
	sortHistory(out)
	if len(out) > 200 {
		out = out[:200]
	}
	writeJSON(w, http.StatusOK, out)
}

func sortHistory(out []historyEntryJSON) {
	slices.SortStableFunc(out, func(a, b historyEntryJSON) int { return b.At.Compare(a.At) })
}

type audienceState struct {
	Restricted bool    `json:"restricted"`
	AreaIDs    []int64 `json:"area_ids"`
	PersonIDs  []int64 `json:"person_ids"`
}

func audienceOf(restricted bool, areas, people []nameRef) audienceState {
	s := audienceState{Restricted: restricted, AreaIDs: []int64{}, PersonIDs: []int64{}}
	for _, a := range areas {
		s.AreaIDs = append(s.AreaIDs, a.ID)
	}
	for _, p := range people {
		s.PersonIDs = append(s.PersonIDs, p.ID)
	}
	return s
}

// logAudience records who changed who uses an item, with before and after.
func (h *Handler) logAudience(ctx context.Context, kind string, id, actorID int64, before, after audienceState) {
	detail, _ := json.Marshal(map[string]any{"before": before, "after": after})
	if err := h.q.InsertGovernanceEvent(ctx, eventFor(kind, id, "audience", actorID, detail)); err != nil {
		logGov("audience event", id, err)
	}
}

// artifactWaiting: why the artifact's newest version waits ("" = it doesn't).
func (h *Handler) artifactWaiting(ctx context.Context, id int64) string {
	req, err := h.q.GetOpenRequestForArtifact(ctx, pgInt8(id))
	if err != nil {
		return ""
	}
	msg, err := h.requestMessage(ctx, req)
	if err != nil {
		logGov("artifact waiting", id, err)
	}
	return msg
}
