package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/schedule"
)

// scheduleJSON is a schedule as people see it. Inputs, files and the output
// folder go only to the person who made it: whoever cares for the app sees
// that a schedule exists, never what it sends.
type scheduleJSON struct {
	ID            int64                         `json:"id"`
	AppID         int64                         `json:"app_id"`
	AppName       string                        `json:"app_name"`
	PersonID      int64                         `json:"person_id"`
	PersonName    string                        `json:"person_name"`
	Mine          bool                          `json:"mine"`
	Name          string                        `json:"name"`
	Frequency     schedule.Frequency            `json:"frequency"`
	Describe      string                        `json:"describe"`
	ComputerID    int64                         `json:"computer_id"`
	ComputerName  string                        `json:"computer_name"`
	Active        bool                          `json:"active"`
	PausedReason  string                        `json:"paused_reason"`
	Situation     string                        `json:"situation"`
	NextRunAt     *time.Time                    `json:"next_run_at"`
	LastRunAt     *time.Time                    `json:"last_run_at"`
	LastRunID     *int64                        `json:"last_run_id"`
	LastRunStatus string                        `json:"last_run_status"`
	Upcoming      []time.Time                   `json:"upcoming"`
	OutputFolder  string                        `json:"output_folder,omitempty"`
	Inputs        map[string][]string           `json:"inputs,omitempty"`
	Files         map[string]manifest.InputFile `json:"files,omitempty"`
}

type scheduleRow = sqlc.ListAppSchedulesRow

func toScheduleJSON(r scheduleRow, me int64, now time.Time) scheduleJSON {
	var f schedule.Frequency
	_ = json.Unmarshal(r.Frequency, &f)
	s := scheduleJSON{
		ID: r.ID, AppID: r.AppID, AppName: r.AppName, PersonID: r.PersonID, PersonName: r.PersonName, Mine: r.PersonID == me,
		Name: r.Name, Frequency: f, Describe: f.Describe(), ComputerID: r.ComputerID, ComputerName: r.ComputerName,
		Active: r.Active, PausedReason: r.PausedReason, Situation: r.Situation,
		NextRunAt: tsPtr(r.NextRunAt), LastRunAt: tsPtr(r.LastRunAt), LastRunStatus: r.LastRunStatus,
	}
	if r.LastRunID.Valid {
		id := r.LastRunID.Int64
		s.LastRunID = &id
	}
	if r.Active {
		s.Upcoming = f.Upcoming(now, 3)
	}
	if s.Mine {
		s.OutputFolder = r.OutputFolder
		_ = json.Unmarshal(r.Inputs, &s.Inputs)
		var files map[string]manifest.InputFile
		_ = json.Unmarshal(r.Files, &files)
		for id, file := range files {
			if s.Files == nil {
				s.Files = map[string]manifest.InputFile{}
			}
			s.Files[id] = manifest.InputFile{Name: file.Name}
		}
	}
	return s
}

// schedulable says why an app cannot be scheduled ("" when it can).
func schedulable(m manifest.Manifest) string {
	if m.Kind != manifest.KindScript || !canRun(m) {
		return "Só scripts podem ser agendados por enquanto."
	}
	for _, in := range m.Inputs {
		if in.Source == "window" {
			return "Este app pergunta coisas depois de abrir, então não dá para agendar."
		}
	}
	return ""
}

type scheduleBody struct {
	Name         string                        `json:"name"`
	Frequency    schedule.Frequency            `json:"frequency"`
	ComputerID   int64                         `json:"computer_id"`
	OutputFolder string                        `json:"output_folder"`
	Inputs       map[string]json.RawMessage    `json:"inputs"`
	Files        map[string]manifest.InputFile `json:"files"`
}

// readScheduleBody decodes and checks a schedule form for app a. stored are
// the files already saved with the schedule being edited: a file sent with
// only its name keeps the stored content.
func (h *Handler) readScheduleBody(w http.ResponseWriter, r *http.Request, a sqlc.App, me sqlc.Person, stored map[string]manifest.InputFile) (scheduleBody, map[string][]string, bool) {
	var body scheduleBody
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 15<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "O formulário é inválido ou ultrapassou o limite de 10 MB de arquivos.")
		return body, nil, false
	}
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		internalError(w, "decode manifest", err)
		return body, nil, false
	}
	if msg := schedulable(m); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return body, nil, false
	}
	if err := body.Frequency.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return body, nil, false
	}
	body.Name = strings.TrimSpace(body.Name)
	if len([]rune(body.Name)) > 80 {
		writeError(w, http.StatusBadRequest, "Use um nome com até 80 letras.")
		return body, nil, false
	}
	values := map[string][]string{}
	for id, raw := range body.Inputs {
		var value string
		if json.Unmarshal(raw, &value) == nil {
			values[id] = []string{value}
			continue
		}
		var list []string
		if json.Unmarshal(raw, &list) != nil {
			writeError(w, http.StatusBadRequest, "As entradas devem ser texto ou listas de opções.")
			return body, nil, false
		}
		values[id] = list
	}
	for id, f := range body.Files {
		if f.ContentBase64 == "" {
			if old, ok := stored[id]; ok && old.Name == f.Name {
				body.Files[id] = old
			}
		}
	}
	if err := m.ValidateRunInputs(values, body.Files); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return body, nil, false
	}
	body.OutputFolder = strings.TrimSpace(body.OutputFolder)
	if !windowsAbs.MatchString(body.OutputFolder) {
		writeError(w, http.StatusBadRequest, `Informe a pasta completa, começando pela letra do disco (ex.: C:\Users\voce\Documentos).`)
		return body, nil, false
	}
	comp, err := h.q.GetComputer(r.Context(), body.ComputerID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (comp.PersonID != me.ID || comp.RevokedAt.Valid)) {
		writeError(w, http.StatusBadRequest, "Escolha um computador seu.")
		return body, nil, false
	}
	if err != nil {
		internalError(w, "schedule computer", err)
		return body, nil, false
	}
	return body, values, true
}

func (h *Handler) createSchedule(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.appFor(w, r)
	if !ok {
		return
	}
	body, values, ok := h.readScheduleBody(w, r, a, me, nil)
	if !ok {
		return
	}
	freq, _ := json.Marshal(body.Frequency)
	inputs, _ := json.Marshal(values)
	files, _ := json.Marshal(nonNilFiles(body.Files))
	now := time.Now()
	s, err := h.q.CreateSchedule(r.Context(), sqlc.CreateScheduleParams{
		AppID: a.ID, PersonID: me.ID, ComputerID: body.ComputerID, Name: body.Name, Frequency: freq,
		Inputs: inputs, Files: files, OutputFolder: body.OutputFolder,
		NextRunAt: pgtype.Timestamptz{Time: body.Frequency.Next(now), Valid: true},
	})
	if err != nil {
		internalError(w, "create schedule", err)
		return
	}
	h.writeSchedule(w, r, s.ID, me.ID, http.StatusCreated)
}

func nonNilFiles(f map[string]manifest.InputFile) map[string]manifest.InputFile {
	if f == nil {
		return map[string]manifest.InputFile{}
	}
	return f
}

func (h *Handler) writeSchedule(w http.ResponseWriter, r *http.Request, id, me int64, status int) {
	row, err := h.q.GetScheduleRow(r.Context(), id)
	if err != nil {
		internalError(w, "get schedule", err)
		return
	}
	writeJSON(w, status, toScheduleJSON(scheduleRow(row), me, time.Now()))
}

// scheduleFor loads the schedule in the path and says whether the person
// made it (mine) or cares for its app (carer). Anyone else gets 404, the
// same answer as for a schedule that does not exist.
func (h *Handler) scheduleFor(w http.ResponseWriter, r *http.Request) (sqlc.Schedule, sqlc.Person, bool, bool, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return sqlc.Schedule{}, me, false, false, false
	}
	id, err := strconv.ParseInt(r.PathValue("sid"), 10, 64)
	notFound := func() (sqlc.Schedule, sqlc.Person, bool, bool, bool) {
		writeError(w, http.StatusNotFound, "Agendamento não encontrado.")
		return sqlc.Schedule{}, me, false, false, false
	}
	if err != nil {
		return notFound()
	}
	s, err := h.q.GetSchedule(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound()
	}
	if err != nil {
		internalError(w, "get schedule", err)
		return sqlc.Schedule{}, me, false, false, false
	}
	mine := s.PersonID == me.ID
	carer := false
	if a, err := h.q.GetApp(r.Context(), s.AppID); err == nil {
		acc, err := h.loadAccess(r.Context(), me)
		if err != nil {
			internalError(w, "load access", err)
			return sqlc.Schedule{}, me, false, false, false
		}
		can := acc.For(appRef(a))
		carer = can.View && can.Access
	}
	if !mine && !carer {
		return notFound()
	}
	return s, me, mine, carer, true
}

func (h *Handler) listAppSchedules(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListAppSchedules(r.Context(), a.ID)
	if err != nil {
		internalError(w, "list schedules", err)
		return
	}
	now := time.Now()
	out := []scheduleJSON{}
	for _, row := range rows {
		if row.PersonID == me.ID || can.Access {
			out = append(out, toScheduleJSON(row, me.ID, now))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listMySchedules(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListPersonSchedules(r.Context(), me.ID)
	if err != nil {
		internalError(w, "list my schedules", err)
		return
	}
	now := time.Now()
	out := []scheduleJSON{}
	for _, row := range rows {
		out = append(out, toScheduleJSON(scheduleRow(row), me.ID, now))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	s, me, mine, _, ok := h.scheduleFor(w, r)
	if !ok {
		return
	}
	if !mine {
		writeError(w, http.StatusNotFound, "Agendamento não encontrado.")
		return
	}
	h.writeSchedule(w, r, s.ID, me.ID, http.StatusOK)
}

func (h *Handler) updateSchedule(w http.ResponseWriter, r *http.Request) {
	s, me, mine, _, ok := h.scheduleFor(w, r)
	if !ok {
		return
	}
	if !mine {
		writeError(w, http.StatusNotFound, "Agendamento não encontrado.")
		return
	}
	a, err := h.q.GetApp(r.Context(), s.AppID)
	if err != nil {
		internalError(w, "schedule app", err)
		return
	}
	var stored map[string]manifest.InputFile
	_ = json.Unmarshal(s.Files, &stored)
	body, values, ok := h.readScheduleBody(w, r, a, me, stored)
	if !ok {
		return
	}
	freq, _ := json.Marshal(body.Frequency)
	inputs, _ := json.Marshal(values)
	files, _ := json.Marshal(nonNilFiles(body.Files))
	if _, err := h.q.UpdateSchedule(r.Context(), sqlc.UpdateScheduleParams{
		ID: s.ID, ComputerID: body.ComputerID, Name: body.Name, Frequency: freq, Inputs: inputs, Files: files,
		OutputFolder: body.OutputFolder, NextRunAt: pgtype.Timestamptz{Time: body.Frequency.Next(time.Now()), Valid: true},
	}); err != nil {
		internalError(w, "update schedule", err)
		return
	}
	h.writeSchedule(w, r, s.ID, me.ID, http.StatusOK)
}

func (h *Handler) pauseSchedule(w http.ResponseWriter, r *http.Request) {
	s, me, _, _, ok := h.scheduleFor(w, r)
	if !ok {
		return
	}
	if err := h.q.SetScheduleActive(r.Context(), sqlc.SetScheduleActiveParams{ID: s.ID, Active: false, PausedReason: ""}); err != nil {
		internalError(w, "pause schedule", err)
		return
	}
	h.writeSchedule(w, r, s.ID, me.ID, http.StatusOK)
}

func (h *Handler) resumeSchedule(w http.ResponseWriter, r *http.Request) {
	s, me, mine, _, ok := h.scheduleFor(w, r)
	if !ok {
		return
	}
	if !mine {
		writeError(w, http.StatusNotFound, "Agendamento não encontrado.")
		return
	}
	var f schedule.Frequency
	if err := json.Unmarshal(s.Frequency, &f); err != nil {
		internalError(w, "schedule frequency", err)
		return
	}
	if err := h.q.ResumeSchedule(r.Context(), sqlc.ResumeScheduleParams{ID: s.ID, NextRunAt: pgtype.Timestamptz{Time: f.Next(time.Now()), Valid: true}}); err != nil {
		internalError(w, "resume schedule", err)
		return
	}
	h.writeSchedule(w, r, s.ID, me.ID, http.StatusOK)
}

func (h *Handler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	s, _, _, _, ok := h.scheduleFor(w, r)
	if !ok {
		return
	}
	if err := h.q.DeleteSchedule(r.Context(), s.ID); err != nil {
		internalError(w, "delete schedule", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
