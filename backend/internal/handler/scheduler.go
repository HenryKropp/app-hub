package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/schedule"
)

// Texts a person reads on a schedule (ADR 024).
const (
	schedSkipped       = "Pulada: a anterior ainda estava rodando"
	schedWaitApproval  = "Esperando aprovação do app"
	schedFailedFive    = "Falhou 5 vezes seguidas"
	schedNoAccess      = "Você não tem mais acesso a este app"
	schedInputsChanged = "O app mudou e os campos guardados não servem mais. Edite o agendamento."
	schedAgentOutdated = "Atualize o Agente em Configurações › Meu computador"
	schedComputerGone  = "O computador escolhido não está mais ligado ao Ziv. Edite o agendamento e escolha outro."
	maxFailures        = 5
	schedulerEvery     = 30 * time.Second
)

// StartScheduler runs the schedule clock of this company until ctx ends.
func (h *Handler) StartScheduler(ctx context.Context) {
	go func() {
		t := time.NewTicker(schedulerEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				h.schedulerTick(ctx, now)
			}
		}
	}()
}

// schedulerTick fires every schedule that is due at now. Each due time is
// claimed in the database first, so two clocks never fire it twice.
func (h *Handler) schedulerTick(ctx context.Context, now time.Time) {
	// Runs whose computer never took them settle even when nobody has a
	// page open, so "anterior ainda rodando" does not last forever.
	if err := h.q.ExpireStaleRuns(ctx); err != nil {
		slog.Error("scheduler expire runs", "err", err)
	}
	due, err := h.q.DueSchedules(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		slog.Error("scheduler due", "err", err)
		return
	}
	for _, s := range due {
		if err := h.fireSchedule(ctx, s, now); err != nil {
			slog.Error("scheduler fire", "schedule", s.ID, "err", err)
		}
	}
}

func (h *Handler) fireSchedule(ctx context.Context, s sqlc.Schedule, now time.Time) error {
	var f schedule.Frequency
	if err := json.Unmarshal(s.Frequency, &f); err != nil || f.Validate() != nil {
		return h.pauseBySystem(ctx, s.ID, "O horário guardado é inválido. Edite o agendamento.")
	}
	claimed, err := h.q.ClaimSchedule(ctx, sqlc.ClaimScheduleParams{ID: s.ID, OldNext: s.NextRunAt, NewNext: pgtype.Timestamptz{Time: f.Next(now), Valid: true}})
	if err != nil || claimed == 0 {
		return err
	}
	late := s.NextRunAt.Time
	if now.Sub(late) > schedule.Grace {
		return h.q.SetScheduleSituation(ctx, sqlc.SetScheduleSituationParams{ID: s.ID,
			Situation: "Perdeu o horário de " + late.In(schedule.Location).Format("02/01 15:04") + ": o Ziv estava desligado"})
	}
	if paused, err := h.pauseIfFailing(ctx, s); err != nil || paused {
		return err
	}
	a, err := h.q.GetApp(ctx, s.AppID)
	if err != nil {
		return err
	}
	person, err := h.q.GetPerson(ctx, s.PersonID)
	if err != nil {
		return err
	}
	acc, err := h.loadAccess(ctx, person)
	if err != nil {
		return err
	}
	if !acc.For(appRef(a)).View {
		return h.pauseBySystem(ctx, s.ID, schedNoAccess)
	}
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return err
	}
	if schedulable(m) != "" {
		return h.pauseBySystem(ctx, s.ID, schedInputsChanged)
	}
	// Checked before the computer, so an app that changed says so even
	// while the computer is off, instead of piling up Perdidas.
	var inputs map[string][]string
	var files map[string]manifest.InputFile
	_ = json.Unmarshal(s.Inputs, &inputs)
	_ = json.Unmarshal(s.Files, &files)
	if m.ValidateRunInputs(inputs, files) != nil {
		return h.pauseBySystem(ctx, s.ID, schedInputsChanged)
	}
	if reason, err := h.executionBlock(ctx, a.ID); err != nil {
		return err
	} else if reason != "" {
		return h.blocked(ctx, s.ID, reason)
	}
	var live int
	if err := h.q.SecurityDB().QueryRow(ctx, `SELECT count(*) FROM runs WHERE schedule_id = $1 AND status IN ('waiting', 'running')`, s.ID).Scan(&live); err != nil {
		return err
	}
	if live > 0 {
		return h.q.SetScheduleSituation(ctx, sqlc.SetScheduleSituationParams{ID: s.ID, Situation: schedSkipped})
	}
	comp, err := h.q.GetComputer(ctx, s.ComputerID)
	if err != nil {
		return err
	}
	if comp.RevokedAt.Valid {
		return h.pauseBySystem(ctx, s.ID, schedComputerGone)
	}
	connected, err := h.q.IsComputerConnected(ctx, comp.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		connected = false
	} else if err != nil {
		return err
	}
	if !connected {
		return h.missed(ctx, s, "O computador "+comp.Name+" estava desligado")
	}
	run, refusal, err := h.launchRun(ctx, launchRequest{App: a, Person: person, Computer: comp, Manifest: m, OutputFolder: s.OutputFolder,
		Inputs: inputs, Files: files, Origin: "schedule", ScheduleID: pgtype.Int8{Int64: s.ID, Valid: true}})
	if err != nil {
		return err
	}
	if refusal != nil {
		switch refusal.Code {
		case "blocked":
			return h.blocked(ctx, s.ID, refusal.Msg)
		case "invalid_inputs":
			return h.pauseBySystem(ctx, s.ID, schedInputsChanged)
		case "agent_outdated":
			return h.missed(ctx, s, schedAgentOutdated)
		case "folder_busy":
			return h.q.SetScheduleSituation(ctx, sqlc.SetScheduleSituationParams{ID: s.ID, Situation: "Pulada: " + refusal.Msg})
		default:
			return h.q.SetScheduleSituation(ctx, sqlc.SetScheduleSituationParams{ID: s.ID, Situation: refusal.Msg})
		}
	}
	return h.q.RecordScheduleLaunch(ctx, sqlc.RecordScheduleLaunchParams{ID: s.ID, LastRunID: pgtype.Int8{Int64: run.ID, Valid: true}})
}

// blocked: a suspension pauses the schedule; waiting for approval or the
// company's pause only shows on it, and it runs again by itself afterwards.
func (h *Handler) blocked(ctx context.Context, id int64, reason string) error {
	if strings.Contains(reason, "suspenso") {
		return h.pauseBySystem(ctx, id, reason)
	}
	if strings.Contains(reason, "aprovação") {
		reason = schedWaitApproval
	}
	return h.q.SetScheduleSituation(ctx, sqlc.SetScheduleSituationParams{ID: id, Situation: reason})
}

// missed records a run that could not start, visible in the history.
func (h *Handler) missed(ctx context.Context, s sqlc.Schedule, msg string) error {
	run, err := h.q.CreateMissedRun(ctx, sqlc.CreateMissedRunParams{AppID: s.AppID, PersonID: s.PersonID, ComputerID: s.ComputerID,
		OutputFolder: s.OutputFolder, ScheduleID: pgtype.Int8{Int64: s.ID, Valid: true}, ErrorMessage: msg})
	if err != nil {
		return err
	}
	h.auditSystem(ctx, "schedule.missed")
	if err := h.q.RecordScheduleLaunch(ctx, sqlc.RecordScheduleLaunchParams{ID: s.ID, LastRunID: pgtype.Int8{Int64: run.ID, Valid: true}}); err != nil {
		return err
	}
	_, err = h.pauseIfFailing(ctx, s)
	return err
}

// failureStreak counts the schedule's latest runs, since it was last
// changed or resumed, that failed in a row. Runs someone stopped by hand are
// skipped: they neither count nor break the streak.
func (h *Handler) failureStreak(ctx context.Context, id int64) (int, error) {
	rows, err := h.q.SecurityDB().Query(ctx, `SELECT r.status FROM runs r JOIN schedules s ON s.id = r.schedule_id
		WHERE r.schedule_id = $1 AND r.created_at >= s.updated_at AND r.status <> 'stopped'
		ORDER BY r.id DESC LIMIT $2`, id, maxFailures)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	streak := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return 0, err
		}
		if status != "error" && status != "interrupted" && status != "missed" {
			break
		}
		streak++
	}
	return streak, rows.Err()
}

func (h *Handler) pauseIfFailing(ctx context.Context, s sqlc.Schedule) (bool, error) {
	streak, err := h.failureStreak(ctx, s.ID)
	if err != nil {
		return false, err
	}
	if _, err := h.q.SecurityDB().Exec(ctx, `UPDATE schedules SET failures = $2 WHERE id = $1`, s.ID, streak); err != nil {
		return false, err
	}
	if streak < maxFailures {
		return false, nil
	}
	return true, h.pauseBySystem(ctx, s.ID, schedFailedFive)
}

func (h *Handler) pauseBySystem(ctx context.Context, id int64, reason string) error {
	if err := h.q.SetScheduleActive(ctx, sqlc.SetScheduleActiveParams{ID: id, Active: false, PausedReason: reason}); err != nil {
		return err
	}
	h.auditSystem(ctx, "schedule.paused")
	return nil
}

// auditSystem records something the Ziv did by itself (no person, no request).
func (h *Handler) auditSystem(ctx context.Context, action string) {
	if _, err := h.q.SecurityDB().Exec(ctx, `INSERT INTO security_audit (actor_id, action, route) VALUES (NULL, $1, 'schedule')`, action); err != nil {
		slog.Error("system audit failed", "action", action)
	}
}
