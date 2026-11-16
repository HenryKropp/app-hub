package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/testplan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Etapa 2 (ADR 044): a draft under test walks stages kept in the database,
// moved by this clock and by the Agente's result, so a restart picks up
// where it was.

const (
	stageConfirm = "esperando_confirmacao"
	stagePC      = "esperando_pc"
	stageRunning = "rodando"
	stagePublish = "publicando"

	// pcWaitLimit is how long a draft waits for its test PC.
	pcWaitLimit = 24 * time.Hour
	// testGrace is the time past tempo_maximo_min before the test fails.
	testGrace = 5 * time.Minute

	pipelineEvery = 10 * time.Second

	labelConfirm  = "esperando confirmação do plano de teste"
	labelPCOff    = "esperando o PC de teste ligar"
	labelPCOld    = "esperando o Agente do PC de teste se atualizar"
	labelNoPC     = "escolha o PC de teste na confirmação do plano"
	labelPublish  = "publicando"
	msgPCTimeout  = "O PC de teste ficou 24 h sem ligar ao Ziv."
	msgReplaced   = "substituída por outra versão no ar"
	msgRunWaiting = "Espere a execução terminar"
)

// StartTestPipeline runs the test clock of this company until ctx ends.
func (h *Handler) StartTestPipeline(ctx context.Context) {
	go func() {
		t := time.NewTicker(pipelineEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				h.testPipelineTick(ctx, now)
			}
		}
	}()
}

// testPipelineTick moves every draft under test and clears old evidence.
func (h *Handler) testPipelineTick(ctx context.Context, now time.Time) {
	if err := h.q.ExpireStaleJobs(ctx); err != nil {
		slog.Error("pipeline expire jobs", "err", err)
	}
	drafts, err := h.q.ListActiveDrafts(ctx)
	if err != nil {
		slog.Error("pipeline list drafts", "err", err)
		return
	}
	for _, d := range drafts {
		if err := h.advanceDraft(ctx, d, now); err != nil {
			slog.Error("pipeline advance", "draft", d.ID, "err", err)
		}
	}
	if err := h.q.DeleteOldEvidence(ctx); err != nil {
		slog.Error("pipeline old evidence", "err", err)
	}
	if err := h.q.DeleteOldClosedDrafts(ctx); err != nil {
		slog.Error("pipeline old drafts", "err", err)
	}
}

// advanceDraft walks d forward while it can: each step moves from the stage
// it read, so the clock and the Agente's result never take the same step.
func (h *Handler) advanceDraft(ctx context.Context, d sqlc.AppDraft, now time.Time) error {
	for range 4 {
		moved, err := h.stepDraft(ctx, d, now)
		if err != nil || !moved {
			return err
		}
		if d, err = h.q.GetDraftByID(ctx, d.ID); err != nil {
			return err
		}
		if d.Status != "testando" && d.Status != "aprovada" {
			return nil
		}
	}
	return nil
}

func (h *Handler) moveStage(ctx context.Context, d sqlc.AppDraft, to string) (bool, error) {
	n, err := h.q.SetDraftStage(ctx, sqlc.SetDraftStageParams{ID: d.ID, FromStage: d.Stage, ToStage: to})
	return n == 1, err
}

func (h *Handler) failDraft(ctx context.Context, d sqlc.AppDraft, reason string) (bool, error) {
	return false, h.q.FailDraft(ctx, sqlc.FailDraftParams{ID: d.ID, Reason: reason})
}

// stepDraft takes one step; moved says whether the stage changed.
func (h *Handler) stepDraft(ctx context.Context, d sqlc.AppDraft, now time.Time) (bool, error) {
	a, err := h.q.GetApp(ctx, d.AppID)
	if err != nil {
		return false, err
	}
	m, err := draftManifest(a, d)
	if err != nil {
		return h.failDraft(ctx, d, "Não consegui ler a receita desta versão.")
	}
	switch d.Stage {
	case stageConfirm:
		plan, err := h.q.GetTestPlan(ctx, a.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		if plan.Fingerprint != testplan.Fingerprint(m, testplan.PlacePC) {
			return false, nil
		}
		return h.moveStage(ctx, d, stagePC)
	case stagePC:
		if d.StageSince.Valid && now.Sub(d.StageSince.Time) > pcWaitLimit {
			return h.failDraft(ctx, d, msgPCTimeout)
		}
		pc, ready, err := h.testPC(ctx, a.ID)
		if err != nil || !ready {
			return false, err
		}
		return h.queueTest(ctx, a, d, m, pc, now)
	case stageRunning:
		if d.TestJobID.Valid {
			j, err := h.q.GetJob(ctx, d.TestJobID.Int64)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return false, err
			}
			if errors.Is(err, pgx.ErrNoRows) || (j.Status == "failed" && len(j.Result) == 0) {
				// The PC never took the job: wait for it again.
				return h.moveStage(ctx, d, stagePC)
			}
		}
		timeout := manifest.Teste{}
		if m.Teste != nil {
			timeout = *m.Teste
		}
		if d.TestStartedAt.Valid && now.After(d.TestStartedAt.Time.Add(timeout.Timeout()+testGrace)) {
			return h.failDraft(ctx, d, fmt.Sprintf("O teste passou do tempo máximo de %d min.", int(timeout.Timeout().Minutes())))
		}
		return false, nil
	case stagePublish:
		return h.publishTested(ctx, a, d)
	}
	return false, nil
}

// testPC is the confirmed test computer when it can run the test now.
func (h *Handler) testPC(ctx context.Context, appID int64) (sqlc.GetComputerStatusRow, bool, error) {
	plan, err := h.q.GetTestPlan(ctx, appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.GetComputerStatusRow{}, false, nil
	} else if err != nil || !plan.ComputerID.Valid {
		return sqlc.GetComputerStatusRow{}, false, err
	}
	pc, err := h.q.GetComputerStatus(ctx, plan.ComputerID.Int64)
	if errors.Is(err, pgx.ErrNoRows) {
		return pc, false, nil
	} else if err != nil {
		return pc, false, err
	}
	return pc, pc.Connected && pc.AgentVersion == agentproto.Version, nil
}

// queueTest sends the test job to the PC and moves the draft to rodando.
// Nothing runs when the app is blocked or a key is missing: the draft fails
// with the reason, as a run would be refused.
func (h *Handler) queueTest(ctx context.Context, a sqlc.App, d sqlc.AppDraft, m manifest.Manifest, pc sqlc.GetComputerStatusRow, now time.Time) (bool, error) {
	if reason, err := h.executionBlock(ctx, a.ID); err != nil {
		return false, err
	} else if reason != "" {
		return h.failDraft(ctx, d, "O teste não rodou: "+reason)
	}
	// The draft's ficha decides which keys the test needs.
	withDraft := a
	withDraft.Manifest, _ = json.Marshal(m)
	keys, err := h.appKeys(ctx, withDraft)
	if err != nil {
		return false, err
	}
	if len(keys.Missing) > 0 {
		return h.failDraft(ctx, d, "O teste não rodou: falta preencher nas configurações do app: "+strings.Join(keys.Missing, ", ")+".")
	}
	hasSecrets := slices.ContainsFunc(keys.Keys, func(k keyItem) bool { return k.HasValue })
	if hasSecrets && keys.Locked {
		return false, nil // the vault opens again on its own; keep waiting
	}
	plain, err := h.plainValues(ctx, a)
	if err != nil {
		return false, err
	}
	var files map[string]manifest.InputFile
	if err := json.Unmarshal(d.Files, &files); err != nil {
		return h.failDraft(ctx, d, "Não consegui ler os arquivos desta versão.")
	}
	timeout := 0
	if m.Teste != nil {
		timeout = int(m.Teste.Timeout().Minutes())
	}
	payload, err := json.Marshal(agentproto.TestPayload{
		DraftID: d.ID, AppID: a.ID, AppName: a.Name, Keys: hasSecrets || len(plain) > 0, TimeoutMin: timeout,
		Script: agentproto.ScriptPayload{Manifest: m, Sources: files, Inputs: testplan.ResolveInputs(m, now)},
	})
	if err != nil {
		return false, err
	}
	moved, err := h.moveStage(ctx, d, stageRunning)
	if err != nil || !moved {
		return false, err
	}
	j, err := h.q.CreateTestJob(ctx, sqlc.CreateTestJobParams{ComputerID: pc.ID, DraftID: pgtype.Int8{Int64: d.ID, Valid: true}, Payload: payload})
	if err != nil {
		return true, err
	}
	return true, h.q.SetDraftTestJob(ctx, sqlc.SetDraftTestJobParams{ID: d.ID, JobID: pgtype.Int8{Int64: j.ID, Valid: true}})
}

// publishTested puts an approved draft live as "testada" and writes the notice.
func (h *Handler) publishTested(ctx context.Context, a sqlc.App, d sqlc.AppDraft) (bool, error) {
	version, refusal, retry, err := h.publishDraftBG(ctx, d)
	if err != nil {
		return false, err
	}
	if retry {
		return false, nil
	}
	if refusal != "" {
		return h.failDraft(ctx, d, "Não consegui publicar: "+refusal)
	}
	if err := h.q.ClearInTest(ctx, a.ID); err != nil {
		return true, err
	}
	var checks []testplan.Check
	_ = json.Unmarshal(d.TestChecks, &checks)
	text := fmt.Sprintf("%s versão %d no ar — testada: %s", a.Name, version, testplan.Summary(checks))
	if reason, err := h.executionBlock(ctx, a.ID); err == nil && reason != "" {
		text += ". Ela precisa de aprovação antes de rodar."
	}
	return true, h.q.InsertNotice(ctx, sqlc.InsertNoticeParams{AppID: a.ID, Version: version, Text: text})
}

// publishDraftBG publishes d outside any request, as its author, through the
// same publishDraft the site uses (ADR 044: a recorder stands in for the
// response). retry is true when a run is going on.
func (h *Handler) publishDraftBG(ctx context.Context, d sqlc.AppDraft) (version int32, refusal string, retry bool, err error) {
	if !d.AuthorID.Valid {
		return 0, "a versão não tem autor.", false, nil
	}
	me, err := h.q.GetPerson(ctx, d.AuthorID.Int64)
	if err != nil {
		return 0, "", false, err
	}
	a, err := h.q.GetApp(ctx, d.AppID)
	if err != nil {
		return 0, "", false, err
	}
	if d.Channel == "mcp" {
		ctx = withMCPPerson(ctx, me.ID)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/apps/publish-tested", nil)
	_, v, _, ok := h.publishDraft(rec, req, a, me, d, true)
	if ok {
		return v.Number, "", false, nil
	}
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code == http.StatusConflict && strings.HasPrefix(body.Error, msgRunWaiting) {
		return 0, "", true, nil
	}
	if rec.Code >= 500 {
		return 0, "", false, fmt.Errorf("publish draft %d: status %d", d.ID, rec.Code)
	}
	if body.Error == "" {
		body.Error = fmt.Sprintf("erro %d", rec.Code)
	}
	return 0, body.Error, false, nil
}

// draftStageLabel says in words where a draft under test is.
func draftStageLabel(d sqlc.AppDraft, plan *sqlc.AppTestPlan, pc *sqlc.GetComputerStatusRow, now time.Time) string {
	switch d.Stage {
	case stageConfirm:
		return labelConfirm
	case stagePC:
		switch {
		case plan == nil || !plan.ComputerID.Valid || pc == nil:
			return labelNoPC
		case !pc.Connected:
			return labelPCOff
		case pc.AgentVersion != agentproto.Version:
			return labelPCOld
		}
		return labelPCOff
	case stageRunning:
		min := 0
		if d.TestStartedAt.Valid {
			min = int(now.Sub(d.TestStartedAt.Time).Minutes())
		}
		return fmt.Sprintf("testando no PC — %d min", max(min, 0))
	case stagePublish:
		return labelPublish
	}
	return ""
}

// draftLabel loads what draftStageLabel needs.
func (h *Handler) draftLabel(ctx context.Context, d sqlc.AppDraft) string {
	var plan *sqlc.AppTestPlan
	var pc *sqlc.GetComputerStatusRow
	if p, err := h.q.GetTestPlan(ctx, d.AppID); err == nil {
		plan = &p
		if p.ComputerID.Valid {
			if c, err := h.q.GetComputerStatus(ctx, p.ComputerID.Int64); err == nil {
				pc = &c
			}
		}
	}
	return draftStageLabel(d, plan, pc, time.Now())
}
