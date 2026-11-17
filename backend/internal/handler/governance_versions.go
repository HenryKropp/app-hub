package handler

// Governança (ADR 046) on the paths that bring a new version of an app: code
// sent by the site or Claude, a draft put live, a change in the folder, a
// restore, a ficha that adds risk, an area move.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/governance"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/risk"
	"github.com/jackc/pgx/v5"
)

type govApplyingKey struct{}

// withGovApplying marks ctx as an approval being applied: the version it
// records is already approved and opens no request of its own.
func withGovApplying(ctx context.Context) context.Context {
	return context.WithValue(ctx, govApplyingKey{}, true)
}

func govApplying(ctx context.Context) bool {
	v, _ := ctx.Value(govApplyingKey{}).(bool)
	return v
}

// governed: this company runs approvals and ctx is not applying one.
func (h *Handler) governed(ctx context.Context) bool {
	return h.workspace != nil && !govApplying(ctx)
}

var skipDirs = map[string]bool{"node_modules": true, ".venv": true, "venv": true, "__pycache__": true, ".git": true}

// riskOfFolder reads the risk of an app's code folder (text files only).
func riskOfFolder(folder string, m manifest.Manifest) risk.Set {
	files := map[string][]byte{}
	_ = filepath.WalkDir(folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != folder && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(files) >= 2000 {
			return filepath.SkipAll
		}
		if info, err := d.Info(); err != nil || info.Size() > 200<<10 {
			return nil
		}
		if b, err := os.ReadFile(p); err == nil {
			rel, _ := filepath.Rel(folder, p)
			files[filepath.ToSlash(rel)] = b
		}
		return nil
	})
	return risk.Of(m, files)
}

func decodeInputFiles(files map[string]manifest.InputFile) map[string][]byte {
	out := map[string][]byte{}
	for name, f := range files {
		if b, err := base64.StdEncoding.DecodeString(f.ContentBase64); err == nil {
			out[path.Clean(name)] = b
		}
	}
	return out
}

// approvedRisk is what the app had when last approved.
func (h *Handler) approvedRisk(ctx context.Context, appID int64) (risk.Set, error) {
	var s risk.Set
	g, err := h.q.GetAppGovernance(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	} else if err != nil {
		return s, err
	}
	_ = json.Unmarshal(g.ApprovedRisk, &s)
	return s, nil
}

// versionNeed says, without writing anything, whether a version with risk rs
// needs approval before going live. ever is false for a pending app (its new
// versions go live in its folder and wait with the "new" request).
func (h *Handler) versionNeed(ctx context.Context, appID int64, rs risk.Set, reason string) (governance.Need, bool, []string, error) {
	gs, err := h.q.GetGovernanceSettings(ctx)
	if err != nil {
		return governance.Need{}, false, nil, err
	}
	ever, rule, err := h.itemRule(ctx, "app", appID, 0)
	if err != nil {
		return governance.Need{}, false, nil, err
	}
	prev, err := h.approvedRisk(ctx, appID)
	if err != nil {
		return governance.Need{}, false, nil, err
	}
	added := rs.Added(prev)
	need := governance.Decide(governance.Effective(governance.Rule(gs.VersionRule), rule), ever, added, gs.ItApproval, reason)
	return need, ever, added, nil
}

// governVersion: version v was just recorded and is live in the folder. It
// opens the request the rule asks for (auto when none is needed). A failure
// leaves the version unapproved, so the app stays held (fail closed).
func (h *Handler) governVersion(ctx context.Context, a sqlc.App, v sqlc.AppVersion, requester int64, tested, preapproved bool) {
	if !h.governed(ctx) {
		return
	}
	var m manifest.Manifest
	_ = json.Unmarshal(a.Manifest, &m)
	var rs risk.Set
	if h.serverPath(a.FolderPath, a.ID) == nil {
		rs = riskOfFolder(a.FolderPath, m)
	} else {
		rs = risk.Of(m, nil)
	}
	prev, err := h.approvedRisk(ctx, a.ID)
	if err != nil {
		logGov("approved risk", a.ID, err)
		return
	}
	off := false
	if v.Origin == "folder" && v.Channel != "mcp" {
		gs, err := h.q.GetGovernanceSettings(ctx)
		if err != nil {
			logGov("settings", a.ID, err)
			return
		}
		off = gs.CodeChannel == "mcp"
	}
	if _, err := h.openRequest(ctx, requestIn{Kind: "app", AppID: a.ID, AreaID: a.AreaID, Reason: governance.ReasonVersion,
		AppVersion: v.Number, Risk: rs, Added: rs.Added(prev), Channel: v.Channel, Tested: tested, RequesterID: requester, Preapproved: preapproved, OffChannel: off}); err != nil {
		logGov("version request", a.ID, err)
	}
}

// draftRisk reads the risk of draft d: its files with its ficha over the app.
func draftRisk(a sqlc.App, d sqlc.AppDraft) (risk.Set, error) {
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return risk.Set{}, err
	}
	var files map[string]manifest.InputFile
	if err := json.Unmarshal(d.Files, &files); err != nil {
		return risk.Set{}, err
	}
	if len(d.Ficha) > 0 {
		f := &fichaUpdate{}
		if err := json.Unmarshal(d.Ficha, f); err != nil {
			return risk.Set{}, err
		}
		m = applyFicha(m, f)
	}
	return risk.Of(m, decodeInputFiles(files)), nil
}

// holdDraft: when the rule asks for approval, draft d waits ("aguardando")
// with an open request instead of going live. held is false when it may go
// live now (then the version it records opens an "auto" request).
func (h *Handler) holdDraft(ctx context.Context, a sqlc.App, d sqlc.AppDraft, tested bool, requester int64) (held bool, waiting string, err error) {
	if !h.governed(ctx) {
		return false, "", nil
	}
	rs, err := draftRisk(a, d)
	if err != nil {
		return false, "", err
	}
	need, ever, added, err := h.versionNeed(ctx, a.ID, rs, governance.ReasonVersion)
	if err != nil || !need.Required || !ever {
		return false, "", err
	}
	if err := h.q.SetDraftWaiting(ctx, sqlc.SetDraftWaitingParams{ID: d.ID}); err != nil {
		return false, "", err
	}
	out, err := h.openRequest(ctx, requestIn{Kind: "app", AppID: a.ID, AreaID: a.AreaID, Reason: governance.ReasonVersion,
		DraftID: d.ID, Risk: rs, Added: added, Channel: d.Channel, Tested: tested, RequesterID: requester})
	if err != nil {
		return true, "", err
	}
	if out.Applied {
		// The requester could decide every step: it is already live.
		return false, "", nil
	}
	if out.Waiting != "" {
		// Claude's ver_versao reads why it waits from here.
		if err := h.q.SetDraftWaiting(ctx, sqlc.SetDraftWaitingParams{ID: d.ID, StatusReason: out.Waiting}); err != nil {
			return true, out.Waiting, err
		}
	}
	return true, out.Waiting, nil
}

// applyDraftRequest puts an approved waiting draft live.
func (h *Handler) applyDraftRequest(ctx context.Context, req sqlc.ApprovalRequest) error {
	d, err := h.q.GetDraftByID(ctx, req.DraftID.Int64)
	if err != nil {
		return err
	}
	version, refusal, retry, err := h.publishDraftBG(withGovApplying(ctx), d, req.Tested)
	if err != nil {
		return err
	}
	if retry {
		return errRunGoing
	}
	if refusal != "" {
		return errors.New("governance: draft refused: " + refusal)
	}
	if err := h.q.SetRequestAppVersion(ctx, sqlc.SetRequestAppVersionParams{ID: req.ID, AppVersion: pgInt4(version)}); err != nil {
		return err
	}
	return h.q.UpsertAppGovernance(ctx, sqlc.UpsertAppGovernanceParams{AppID: req.AppID.Int64, ApprovedVersion: pgInt4(version),
		ApprovedRisk: nonEmptyJSON(req.Risk), EverApproved: true})
}

// applyAreaRequest moves the app to the target area, through its folder
// manifest too, so the next folder read keeps it there.
func (h *Handler) applyAreaRequest(ctx context.Context, req sqlc.ApprovalRequest, actorID int64) error {
	a, err := h.q.GetApp(ctx, req.AppID.Int64)
	if err != nil {
		return err
	}
	area, err := h.q.GetArea(ctx, req.TargetAreaID.Int64)
	if err != nil {
		return err
	}
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return err
	}
	from := m.Area
	m.Area = area.Name
	if h.serverPath(a.FolderPath, a.ID) == nil {
		if err := manifest.WriteToFolder(a.FolderPath, m); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(m)
	if err := h.q.SetAppArea(ctx, sqlc.SetAppAreaParams{ID: a.ID, Area: area.Name, AreaID: area.ID, Manifest: raw}); err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]string{"before": from, "after": area.Name})
	return h.q.InsertGovernanceEvent(ctx, eventFor("app", a.ID, "area", actorID, detail))
}

// applyArtifactAreaRequest moves the artifact to the approved area.
func (h *Handler) applyArtifactAreaRequest(ctx context.Context, req sqlc.ApprovalRequest, actorID int64) error {
	a, err := h.q.GetArtifact(ctx, req.ArtifactID.Int64)
	if err != nil {
		return err
	}
	from, err := h.q.GetArea(ctx, a.AreaID)
	if err != nil {
		return err
	}
	to, err := h.q.GetArea(ctx, req.TargetAreaID.Int64)
	if err != nil {
		return err
	}
	if err := h.q.SetArtifactArea(ctx, sqlc.SetArtifactAreaParams{ID: a.ID, AreaID: to.ID}); err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]string{"before": from.Name, "after": to.Name})
	return h.q.InsertGovernanceEvent(ctx, eventFor("artifact", a.ID, "area", actorID, detail))
}

func nonEmptyJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

// holdSources keeps sent code as a waiting draft (with its open request)
// instead of putting it live; the approved version keeps running.
func (h *Handler) holdSources(w http.ResponseWriter, r *http.Request, a sqlc.App, me sqlc.Person,
	files map[string]manifest.InputFile, ficha *fichaUpdate, note string, rs risk.Set, added []string) {
	ctx := r.Context()
	rawFiles, err := json.Marshal(files)
	if err != nil {
		internalError(w, "encode files", err)
		return
	}
	var rawFicha []byte
	if ficha != nil {
		rawFicha, _ = json.Marshal(ficha)
	}
	// The newest version replaces any draft still waiting or open.
	if err := h.q.CancelOpenDrafts(ctx, sqlc.CancelOpenDraftsParams{AppID: a.ID, Reason: msgReplaced}); err != nil {
		internalError(w, "cancel drafts", err)
		return
	}
	d, err := h.q.InsertDraft(ctx, sqlc.InsertDraftParams{AppID: a.ID, AuthorID: pgInt8(me.ID), Channel: originOf(ctx),
		Files: rawFiles, Ficha: rawFicha, Note: note})
	if err != nil {
		internalError(w, "insert draft", err)
		return
	}
	if err := h.q.SetDraftWaiting(ctx, sqlc.SetDraftWaitingParams{ID: d.ID}); err != nil {
		internalError(w, "draft waiting", err)
		return
	}
	out, err := h.openRequest(ctx, requestIn{Kind: "app", AppID: a.ID, AreaID: a.AreaID, Reason: governance.ReasonVersion,
		DraftID: d.ID, Risk: rs, Added: added, Channel: originOf(ctx), RequesterID: me.ID})
	if err != nil {
		internalError(w, "open request", err)
		return
	}
	if out.Waiting != "" {
		if err := h.q.SetDraftWaiting(ctx, sqlc.SetDraftWaitingParams{ID: d.ID, StatusReason: out.Waiting}); err != nil {
			internalError(w, "draft waiting", err)
			return
		}
	}
	latest, _ := h.q.LatestAppVersion(ctx, a.ID)
	saved, _ := h.q.GetApp(ctx, a.ID)
	writeJSON(w, http.StatusOK, map[string]any{"id": a.ID, "version": latest.Number, "changed": out.Applied, "waiting": out.Waiting,
		"draft_id": d.ID, "blocked_reason": "", "updated_at": saved.UpdatedAt.Time})
}

// governFicha: a ficha saved on the site that adds risk (a vault key, 24h on
// the server) asks for approval like a version; text changes ask nothing.
func (h *Handler) governFicha(ctx context.Context, a sqlc.App, requester int64) error {
	if !h.governed(ctx) {
		return nil
	}
	ever, _, err := h.itemRule(ctx, "app", a.ID, 0)
	if err != nil || !ever {
		return err
	}
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return err
	}
	prev, err := h.approvedRisk(ctx, a.ID)
	if err != nil {
		return err
	}
	merged := risk.Merge(risk.Of(m, nil), prev)
	added := merged.Added(prev)
	if len(added) == 0 {
		return nil
	}
	// A version already waiting holds the app; once it is decided the ficha
	// is checked again (approveRequest), so this risk is not lost.
	if open, err := h.q.GetOpenRequestForApp(ctx, pgInt8(a.ID)); err == nil && open.Reason != governance.ReasonArea {
		return nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// The request names the version live now: approving it approves that
	// one, never a newer version that arrives meanwhile.
	var live int32
	if v, err := h.q.LatestAppVersion(ctx, a.ID); err == nil {
		live = v.Number
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = h.openRequest(ctx, requestIn{Kind: "app", AppID: a.ID, AreaID: a.AreaID, Reason: governance.ReasonFicha,
		Risk: merged, Added: added, RequesterID: requester, AppVersion: live})
	return err
}

// governArtifactVersion opens the request for artifact version v (ADR 046).
// The version is held (approved=false) until the request is approved, which
// happens at once when the rule asks nothing. Its only risk is reaching
// people outside: an active external link.
func (h *Handler) governArtifactVersion(ctx context.Context, a sqlc.Artifact, v sqlc.ArtifactVersion, requester int64, preapproved bool) {
	if !h.governed(ctx) {
		return
	}
	if err := h.q.SetArtifactVersionApproved(ctx, sqlc.SetArtifactVersionApprovedParams{ID: v.ID, Approved: false}); err != nil {
		logGov("hold artifact version", a.ID, err)
		return
	}
	added := []string{}
	if open, err := h.q.ArtifactHasActiveLink(ctx, a.ID); err != nil {
		logGov("artifact links", a.ID, err)
		return
	} else if open {
		added = append(added, "tem link externo ativo")
	}
	if _, err := h.openRequest(ctx, requestIn{Kind: "artifact", ArtifactID: a.ID, AreaID: a.AreaID, Reason: governance.ReasonVersion,
		ArtifactVersionID: v.ID, Added: added, Channel: v.Channel, RequesterID: requester, Preapproved: preapproved}); err != nil {
		logGov("artifact request", a.ID, err)
	}
}
