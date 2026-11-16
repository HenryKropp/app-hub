package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/testplan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Etapa 2 (ADR 044), the site side: start a draft's test, the plan
// confirmation screen and the test files (evidence).

const (
	msgTestPlanMissing = "Falta a seção teste da receita. Escreva a seção teste; veja receita_app_ziv."
	msgTestKind        = "Por enquanto o teste automático é só para scripts. Apps com tela ainda publicam pelo caminho de hoje."
	msgDraftClosed     = "Este rascunho não está mais aberto."
	msgPlanChanged     = "Este plano mudou. Recarregue a página."
	msgPlanOwnerOnly   = "Só o dono do app ou um administrador confirma este plano."
	msgPlanSiteOnly    = "O plano de teste só é confirmado pelo site do Ziv."
	msgNotYourPC       = "Escolha um computador seu, ligado ao Ziv."
	msgConfirmFirst    = "Confirme o plano de teste primeiro."
	msgEvidenceOwner   = "Só o dono do app ou um administrador vê os arquivos do teste."
)

func writeCoded(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "error": msg})
}

// ownerOrAdmin is who confirms test plans and sees test files: the app's
// owner or a company admin (spec decision 7).
func ownerOrAdmin(acc *access.Person, me sqlc.Person, a sqlc.App) bool {
	return a.OwnerID == me.ID || acc.Has(access.ManageRoles)
}

// draftView is what GET /api/apps/{id}/drafts/{d} answers. The log is
// already masked; it goes only to who can edit the app.
func (h *Handler) draftView(ctx context.Context, a sqlc.App, d sqlc.AppDraft, withLog bool) map[string]any {
	var number *int32
	if d.VersionNumber.Valid {
		number = &d.VersionNumber.Int32
	}
	var exit *int32
	if d.TestExitCode.Valid {
		exit = &d.TestExitCode.Int32
	}
	checks := json.RawMessage("[]")
	if len(d.TestChecks) > 0 {
		checks = d.TestChecks
	}
	files := json.RawMessage("[]")
	if len(d.TestFiles) > 0 {
		files = d.TestFiles
	}
	out := map[string]any{
		"id": d.ID, "status": d.Status, "status_reason": d.StatusReason,
		"version_number": number, "note": d.Note, "created_at": d.CreatedAt.Time,
		"stage": d.Stage, "stage_label": h.draftLabel(ctx, d), "checks": checks, "exit_code": exit, "files": files,
	}
	if withLog {
		out["log"] = d.TestLog
	}
	if d.Stage == stageConfirm {
		out["confirm_url"] = "/apps/" + strconv.FormatInt(a.ID, 10) + "/plano-de-teste"
	}
	return out
}

// startDraftTest is POST /api/apps/{id}/drafts/{d}/test: the draft goes to
// test instead of straight live.
func (h *Handler) startDraftTest(w http.ResponseWriter, r *http.Request) {
	a, _, d, ok := h.draftFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if d.Status != "rascunho" {
		writeCoded(w, http.StatusConflict, "draft_closed", msgDraftClosed)
		return
	}
	m, err := draftManifest(a, d)
	if err != nil {
		internalError(w, "draft manifest", err)
		return
	}
	if m.Kind != manifest.KindScript {
		writeCoded(w, http.StatusConflict, "test_kind", msgTestKind)
		return
	}
	if m.Teste == nil {
		writeCoded(w, http.StatusConflict, "test_plan_missing", msgTestPlanMissing)
		return
	}
	if issues := m.Validate(); len(issues) > 0 {
		writeCoded(w, http.StatusBadRequest, "invalid_plan", strings.Join(issues, " "))
		return
	}
	if err := m.ValidateRunInputs(testplan.ResolveInputs(m, time.Now()), nil); err != nil {
		writeCoded(w, http.StatusBadRequest, "invalid_inputs", "As entradas do teste não servem: "+err.Error())
		return
	}
	withDraft := a
	withDraft.Manifest, _ = json.Marshal(m)
	keys, err := h.appKeys(ctx, withDraft)
	if err != nil {
		internalError(w, "app keys", err)
		return
	}
	if len(keys.Missing) > 0 {
		writeCoded(w, http.StatusConflict, "keys_missing", "Falta preencher nas configurações do app: "+strings.Join(keys.Missing, ", ")+". O dono do app ou um administrador preenche na página do app, parte Configurações.")
		return
	}
	if n, err := h.q.StartDraftTest(ctx, d.ID); err != nil {
		internalError(w, "start draft test", err)
		return
	} else if n == 0 {
		writeCoded(w, http.StatusConflict, "draft_closed", msgDraftClosed)
		return
	}
	if d, err = h.q.GetDraftByID(ctx, d.ID); err != nil {
		internalError(w, "get draft", err)
		return
	}
	if err := h.advanceDraft(ctx, d, time.Now()); err != nil {
		internalError(w, "advance draft", err)
		return
	}
	if d, err = h.q.GetDraftByID(ctx, d.ID); err != nil {
		internalError(w, "get draft", err)
		return
	}
	writeJSON(w, http.StatusOK, h.draftView(ctx, a, d, true))
}

// draftFile is GET /api/apps/{id}/drafts/{d}/files?path=: one test file.
// The content is the work's data, not masked; only owner and admins see it.
func (h *Handler) draftFile(w http.ResponseWriter, r *http.Request) {
	a, me, d, ok := h.draftFromPath(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	if !ownerOrAdmin(acc, me, a) {
		writeError(w, http.StatusForbidden, msgEvidenceOwner)
		return
	}
	e, err := h.q.GetEvidence(r.Context(), sqlc.GetEvidenceParams{DraftID: d.ID, Path: r.URL.Query().Get("path")})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Arquivo não encontrado. Os arquivos do teste ficam 7 dias.")
		return
	} else if err != nil {
		internalError(w, "get evidence", err)
		return
	}
	writeEvidence(w, e)
}

// writeEvidence sends a test file as a download. Types a browser would run
// (html, svg, scripts) go as plain text.
func writeEvidence(w http.ResponseWriter, e sqlc.DraftEvidence) {
	ctype := mime.TypeByExtension(strings.ToLower(path.Ext(e.Path)))
	switch {
	case ctype == "":
		ctype = "application/octet-stream"
	case strings.Contains(ctype, "html"), strings.Contains(ctype, "xml"), strings.Contains(ctype, "javascript"), strings.Contains(ctype, "svg"):
		ctype = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(e.Path)}))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(e.Content)
}

// planSource is the test plan shown for confirmation: the one of the draft
// under test or, without one, the app's.
func (h *Handler) planSource(ctx context.Context, a sqlc.App) (manifest.Manifest, *sqlc.AppDraft, error) {
	d, err := h.q.OpenDraftForApp(ctx, a.ID)
	if err == nil {
		m, err := draftManifest(a, d)
		return m, &d, err
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return manifest.Manifest{}, nil, err
	}
	var m manifest.Manifest
	err = json.Unmarshal(a.Manifest, &m)
	return m, nil, err
}

type planComputer struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Outdated  bool   `json:"outdated"`
}

type planConfirmed struct {
	By         string    `json:"by"`
	At         time.Time `json:"at"`
	ComputerID *int64    `json:"computer_id"`
}

type planShown struct {
	Acessa []string `json:"acessa"`
	Pula   string   `json:"pula"`
	Keys   []string `json:"keys"`
	Place  string   `json:"place"`
}

type testPlanView struct {
	Plan        *planShown     `json:"plan"`
	Fingerprint string         `json:"fingerprint"`
	Confirmed   *planConfirmed `json:"confirmed"`
	Matches     bool           `json:"matches"`
	Computers   []planComputer `json:"computers"`
	CanConfirm  bool           `json:"can_confirm"`
	ConfirmText string         `json:"confirm_text"`
}

const confirmText = "Os próximos testes deste app serão automáticos. Só pergunto de novo se o plano de teste mudar."

func (h *Handler) testPlanView(ctx context.Context, a sqlc.App, me sqlc.Person, acc *access.Person) (testPlanView, error) {
	v := testPlanView{Computers: []planComputer{}, CanConfirm: ownerOrAdmin(acc, me, a), ConfirmText: confirmText}
	m, _, err := h.planSource(ctx, a)
	if err != nil {
		return v, err
	}
	if m.Teste != nil {
		withPlan := a
		withPlan.Manifest, _ = json.Marshal(m)
		keys, err := h.appKeys(ctx, withPlan)
		if err != nil {
			return v, err
		}
		names := []string{}
		for _, k := range keys.Keys {
			names = append(names, k.Name)
		}
		v.Plan = &planShown{Acessa: m.Teste.Acessa, Pula: m.Teste.Pula, Keys: names, Place: testplan.PlacePC}
		v.Fingerprint = testplan.Fingerprint(m, testplan.PlacePC)
	}
	stored, err := h.q.GetTestPlan(ctx, a.ID)
	if err == nil {
		c := &planConfirmed{At: stored.ConfirmedAt.Time}
		if stored.ConfirmedBy.Valid {
			if p, err := h.q.GetPerson(ctx, stored.ConfirmedBy.Int64); err == nil {
				c.By = p.Name
			}
		}
		if stored.ComputerID.Valid {
			c.ComputerID = &stored.ComputerID.Int64
		}
		v.Confirmed = c
		v.Matches = v.Fingerprint != "" && stored.Fingerprint == v.Fingerprint
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	pcs, err := h.q.ListPersonComputers(ctx, me.ID)
	if err != nil {
		return v, err
	}
	for _, c := range pcs {
		v.Computers = append(v.Computers, planComputer{ID: c.ID, Name: c.Name, Connected: c.Connected, Outdated: c.AgentVersion != "" && c.AgentVersion != agentproto.Version})
	}
	return v, nil
}

// getTestPlan is GET /api/apps/{id}/test-plan.
func (h *Handler) getTestPlan(w http.ResponseWriter, r *http.Request) {
	a, me, acc, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode mudar este app.")
		return
	}
	v, err := h.testPlanView(r.Context(), a, me, acc)
	if err != nil {
		internalError(w, "test plan", err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// planChanger loads the app for confirming or changing its test plan: only
// on the site (never by Claude), only owner or admin.
func (h *Handler) planChanger(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, *access.Person, int64, string, bool) {
	if viaMCP(r.Context()) {
		writeError(w, http.StatusForbidden, msgPlanSiteOnly)
		return sqlc.App{}, sqlc.Person{}, nil, 0, "", false
	}
	a, me, acc, _, ok := h.appFor(w, r)
	if !ok {
		return a, me, acc, 0, "", false
	}
	if !ownerOrAdmin(acc, me, a) {
		writeError(w, http.StatusForbidden, msgPlanOwnerOnly)
		return a, me, acc, 0, "", false
	}
	var b struct {
		Fingerprint string `json:"fingerprint"`
		ComputerID  int64  `json:"computer_id"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&b) != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "Não consegui ler o pedido.")
		return a, me, acc, 0, "", false
	}
	c, err := h.q.GetComputer(r.Context(), b.ComputerID)
	if err != nil || c.PersonID != me.ID || c.RevokedAt.Valid {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get computer", err)
			return a, me, acc, 0, "", false
		}
		writeError(w, http.StatusBadRequest, msgNotYourPC)
		return a, me, acc, 0, "", false
	}
	return a, me, acc, c.ID, b.Fingerprint, true
}

// advanceWaiting moves the app's draft under test, if any, right away.
func (h *Handler) advanceWaiting(ctx context.Context, appID int64) error {
	d, err := h.q.OpenDraftForApp(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	return h.advanceDraft(ctx, d, time.Now())
}

// confirmTestPlan is POST /api/apps/{id}/test-plan/confirm.
func (h *Handler) confirmTestPlan(w http.ResponseWriter, r *http.Request) {
	a, me, acc, pc, fingerprint, ok := h.planChanger(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	m, _, err := h.planSource(ctx, a)
	if err != nil {
		internalError(w, "plan source", err)
		return
	}
	if m.Teste == nil {
		writeCoded(w, http.StatusConflict, "test_plan_missing", msgTestPlanMissing)
		return
	}
	if fingerprint != testplan.Fingerprint(m, testplan.PlacePC) {
		writeCoded(w, http.StatusConflict, "plan_changed", msgPlanChanged)
		return
	}
	if _, err := h.q.UpsertTestPlan(ctx, sqlc.UpsertTestPlanParams{
		AppID: a.ID, Fingerprint: fingerprint, ComputerID: pgtype.Int8{Int64: pc, Valid: true}, ConfirmedBy: pgtype.Int8{Int64: me.ID, Valid: true},
	}); err != nil {
		internalError(w, "upsert test plan", err)
		return
	}
	if err := h.advanceWaiting(ctx, a.ID); err != nil {
		internalError(w, "advance draft", err)
		return
	}
	h.writeTestPlan(w, r, a, me, acc)
}

// setTestComputer is PUT /api/apps/{id}/test-plan/computer: another PC for
// the confirmed plan, without confirming it again.
func (h *Handler) setTestComputer(w http.ResponseWriter, r *http.Request) {
	a, me, acc, pc, _, ok := h.planChanger(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	n, err := h.q.SetTestPlanComputer(ctx, sqlc.SetTestPlanComputerParams{AppID: a.ID, ComputerID: pgtype.Int8{Int64: pc, Valid: true}})
	if err != nil {
		internalError(w, "set test computer", err)
		return
	}
	if n == 0 {
		writeCoded(w, http.StatusConflict, "plan_not_confirmed", msgConfirmFirst)
		return
	}
	if err := h.advanceWaiting(ctx, a.ID); err != nil {
		internalError(w, "advance draft", err)
		return
	}
	h.writeTestPlan(w, r, a, me, acc)
}

func (h *Handler) writeTestPlan(w http.ResponseWriter, r *http.Request, a sqlc.App, me sqlc.Person, acc *access.Person) {
	v, err := h.testPlanView(r.Context(), a, me, acc)
	if err != nil {
		internalError(w, "test plan", err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
