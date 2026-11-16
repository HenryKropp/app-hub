package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/mask"
	"github.com/HenryKropp/app-hub/backend/internal/testplan"
	"github.com/jackc/pgx/v5/pgtype"
)

// Etapa 2 (ADR 044): the Agente runs a draft in test mode and sends back
// its files (evidence) and its result; the Ziv decides whether it passed.

// draftManifest is the manifest the draft would put live: the app's ficha as
// it is now with the draft's ficha changes.
func draftManifest(a sqlc.App, d sqlc.AppDraft) (manifest.Manifest, error) {
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return m, err
	}
	if len(d.Ficha) == 0 {
		return m, nil
	}
	var f fichaUpdate
	if err := json.Unmarshal(d.Ficha, &f); err != nil {
		return m, err
	}
	return applyFicha(m, &f), nil
}

// testFile is one line of app_drafts.test_files.
type testFile struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Saved bool   `json:"saved"`
}

// agentEvidence stores a test's files (POST /api/agent/jobs/{id}/evidence):
// a zip from the computer that took the test job.
func (h *Handler) agentEvidence(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	j, ok := h.jobForComputer(w, r, c)
	if !ok {
		return
	}
	if j.Kind != agentproto.KindTest || !j.DraftID.Valid {
		writeError(w, http.StatusBadRequest, "Esse pedido não é de teste.")
		return
	}
	if j.Status != "taken" {
		writeError(w, http.StatusConflict, "Este teste já terminou.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, agentproto.MaxEvidenceBytes+1<<20))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "A evidência passou de 20 MB.")
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Não consegui ler a evidência.")
		return
	}
	ctx := r.Context()
	var total int64
	count := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !manifest.SafeRelative(f.Name) || strings.Contains(f.Name, `\`) {
			continue
		}
		if count >= agentproto.MaxEvidenceFiles {
			break
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		// The zip's declared sizes are not trusted: read at most what is left.
		data, err := io.ReadAll(io.LimitReader(rc, agentproto.MaxEvidenceBytes-total+1))
		rc.Close()
		if err != nil {
			continue
		}
		if total+int64(len(data)) > agentproto.MaxEvidenceBytes {
			break
		}
		if err := h.q.InsertEvidence(ctx, sqlc.InsertEvidenceParams{DraftID: j.DraftID.Int64, Path: f.Name, Size: int64(len(data)), Content: data}); err != nil {
			internalError(w, "insert evidence", err)
			return
		}
		total += int64(len(data))
		count++
	}
	w.WriteHeader(http.StatusNoContent)
}

// onTestResult records what the Agente reported for a test job and decides.
func (h *Handler) onTestResult(ctx context.Context, j sqlc.AgentJob, res agentproto.Result) error {
	if res.Event == agentproto.EventStarted {
		return nil
	}
	status := "done"
	if res.Event == agentproto.EventFailed {
		status = "failed"
	}
	// The job keeps no log: the masked log lives on the draft only.
	stored := res
	stored.Log = ""
	if err := h.finishJob(ctx, j.ID, status, stored); err != nil {
		return err
	}
	if !j.DraftID.Valid {
		return nil
	}
	d, err := h.q.GetDraftByID(ctx, j.DraftID.Int64)
	if err != nil {
		return err
	}
	if d.Stage != "rodando" || d.TestJobID.Int64 != j.ID {
		return nil // an old job of a draft that moved on
	}
	a, err := h.q.GetApp(ctx, d.AppID)
	if err != nil {
		return err
	}
	if res.Event != agentproto.EventFinished {
		return h.q.FailDraft(ctx, sqlc.FailDraftParams{ID: d.ID, Reason: "O teste não rodou: " + h.maskFor(ctx, a, res.Error)})
	}
	m, err := draftManifest(a, d)
	if err != nil {
		return err
	}
	log := h.maskFor(ctx, a, res.Log)
	var plan manifest.Teste
	if m.Teste != nil {
		plan = *m.Teste
	}
	checks := testplan.Evaluate(plan, res.ExitCode, res.Files, log)
	saved, err := h.q.ListEvidencePaths(ctx, d.ID)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, p := range saved {
		have[p] = true
	}
	files := make([]testFile, 0, len(res.Files))
	for _, f := range res.Files {
		files = append(files, testFile{Path: f.Path, Size: f.Size, Saved: have[f.Path]})
	}
	rawChecks, _ := json.Marshal(checks)
	rawFiles, _ := json.Marshal(files)
	if err := h.q.SaveDraftResult(ctx, sqlc.SaveDraftResultParams{
		ID: d.ID, ExitCode: pgtype.Int4{Int32: int32(res.ExitCode), Valid: true}, Log: log, Checks: rawChecks, Files: rawFiles,
	}); err != nil {
		return err
	}
	if !testplan.Passed(checks) {
		reason := "O teste não passou."
		for _, c := range checks {
			if !c.OK {
				reason = "O teste não passou: " + c.Detail
				break
			}
		}
		return h.q.FailDraft(ctx, sqlc.FailDraftParams{ID: d.ID, Reason: reason})
	}
	n, err := h.q.ApproveDraft(ctx, d.ID)
	if err != nil || n == 0 {
		return err
	}
	d, err = h.q.GetDraftByID(ctx, d.ID)
	if err != nil {
		return err
	}
	if err := h.advanceDraft(ctx, d, time.Now()); err != nil {
		slog.Error("advance approved draft", "draft", d.ID, "err", err)
	}
	return nil
}

// maskFor hides the app's vault values in text, again on the server: the
// Agente already masked it, but an old or broken Agente must not leak them.
func (h *Handler) maskFor(ctx context.Context, a sqlc.App, text string) string {
	if text == "" {
		return text
	}
	values, err := h.keyValues(ctx, a)
	if err != nil {
		slog.Warn("mask test log: keys unavailable", "app", a.ID, "err", err)
		return text
	}
	list := make([]string, 0, len(values))
	for _, v := range values {
		list = append(list, v)
	}
	return mask.Values(text, list)
}
