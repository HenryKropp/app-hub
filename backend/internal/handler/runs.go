package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/hostpath"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/outputs"
)

var windowsAbs = regexp.MustCompile(`^[A-Za-z]:\\`)

// canRun: in this phase only desktop apps opened by an .exe run through the agent.
func canRun(m manifest.Manifest) bool { return canRunKind(m.Kind, m.Entrypoint) }

func canRunKind(kind, entrypoint string) bool {
	return (kind == "desktop" && manifest.SafeRelative(entrypoint) && strings.HasSuffix(strings.ToLower(entrypoint), ".exe")) ||
		(kind == manifest.KindScript && manifest.ScriptRuntime(entrypoint) != "")
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func (h *Handler) startRun(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.appFor(w, r)
	if !ok {
		return
	}
	var body struct {
		OutputFolder string                        `json:"output_folder"`
		Remember     *bool                         `json:"remember"`
		Inputs       map[string]json.RawMessage    `json:"inputs"`
		Files        map[string]manifest.InputFile `json:"files"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 15<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "O formulário é inválido ou ultrapassou o limite de 10 MB de arquivos.")
		return
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "Envie apenas um formulário por execução.")
		return
	}
	ctx := r.Context()
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	if !canRun(m) {
		writeError(w, http.StatusBadRequest, "Este app ainda não pode rodar pelo Hub.")
		return
	}
	values := map[string][]string{}
	if m.Kind == manifest.KindScript {
		for id, raw := range body.Inputs {
			var value string
			if json.Unmarshal(raw, &value) == nil {
				values[id] = []string{value}
				continue
			}
			var list []string
			if json.Unmarshal(raw, &list) != nil {
				writeError(w, http.StatusBadRequest, "As entradas devem ser texto ou listas de opções.")
				return
			}
			values[id] = list
		}
		if err := m.ValidateRunInputs(values, body.Files); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else if len(body.Inputs) > 0 || len(body.Files) > 0 {
		writeError(w, http.StatusBadRequest, "Este app não recebe um formulário de script.")
		return
	}
	appFolder := hostpath.ToDisplay(a.FolderPath)
	out := strings.TrimSpace(body.OutputFolder)
	if out == "" {
		out = appFolder
	}
	if !windowsAbs.MatchString(out) {
		writeError(w, http.StatusBadRequest, `Informe a pasta completa, começando pela letra do disco (ex.: C:\Users\voce\Documentos).`)
		return
	}
	comp, err := h.q.ConnectedComputerForPerson(ctx, me.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "Seu computador não está conectado.")
		return
	}
	if err != nil {
		internalError(w, "connected computer", err)
		return
	}
	if m.Kind == manifest.KindScript && comp.AgentVersion != agentproto.Version {
		writeError(w, http.StatusConflict, "Atualize o Agente em Configurações › Meu computador para rodar scripts.")
		return
	}
	// Hold the app lock from the version check until the job is queued, so a
	// restore cannot rewrite the folder between recording the version and the run.
	unlock := h.lockApp(a.ID)
	v, _, readable, err := h.checkVersionLocked(ctx, a)
	if err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	// Only a version the Hub just confirmed in the folder is linked to the run.
	versionID := pgtype.Int8{Int64: v.ID, Valid: readable && v.ID != 0}
	run, err := h.q.CreateRun(ctx, sqlc.CreateRunParams{AppID: a.ID, PersonID: me.ID, ComputerID: comp.ID, OutputFolder: out, VersionID: versionID})
	if err != nil {
		unlock()
		internalError(w, "create run", err)
		return
	}
	payload := agentproto.RunPayload{RunID: run.ID, AppFolder: appFolder, Exe: m.Entrypoint, OutputFolder: out}
	if m.Kind == manifest.KindScript {
		sources, err := h.importedSources(r, a.ID)
		if err != nil {
			internalError(w, "script sources", err)
			return
		}
		payload.Script = &agentproto.ScriptPayload{Manifest: m, Inputs: values, Files: body.Files, Sources: sources}
	}
	_, err = h.enqueue(ctx, comp.ID, &run.ID, agentproto.KindRun, payload)
	unlock()
	if err != nil {
		internalError(w, "enqueue run", err)
		return
	}
	if body.Remember != nil {
		var err error
		if *body.Remember {
			err = h.q.SetRunFolder(ctx, sqlc.SetRunFolderParams{PersonID: me.ID, AppID: a.ID, Folder: out})
		} else {
			err = h.q.ClearRunFolder(ctx, sqlc.ClearRunFolderParams{PersonID: me.ID, AppID: a.ID})
		}
		if err != nil {
			slog.Error("remember folder", "app", a.ID, "err", err) // the run is already queued; don't fail it
		}
	}
	row, err := h.q.GetRun(ctx, run.ID)
	if err != nil {
		internalError(w, "get run", err)
		return
	}
	j, err := h.toRunJSON(ctx, row)
	if err != nil {
		internalError(w, "run json", err)
		return
	}
	writeJSON(w, http.StatusCreated, j)
}

// onRunResult applies an agent event to the run.
func (h *Handler) onRunResult(ctx context.Context, j sqlc.AgentJob, res agentproto.Result) error {
	if !j.RunID.Valid {
		return h.finishJob(ctx, j.ID, "failed", res)
	}
	runID := j.RunID.Int64
	switch res.Event {
	case agentproto.EventStarted:
		return h.q.StartRun(ctx, sqlc.StartRunParams{ID: runID, AppVersion: res.AppVersion})
	case agentproto.EventFinished:
		row, err := h.q.GetRun(ctx, runID)
		if err != nil {
			return err
		}
		var m manifest.Manifest
		if err := json.Unmarshal(row.AppManifest, &m); err != nil {
			return err
		}
		for _, f := range res.Files {
			matchPath := f.Path
			if m.Kind == manifest.KindScript {
				matchPath = strings.TrimPrefix(matchPath, fmt.Sprintf("Execucao-%d/", runID))
			}
			idx, label := outputs.Match(m.Outputs, matchPath)
			if err := h.q.AddRunFile(ctx, sqlc.AddRunFileParams{
				RunID: runID, RelPath: f.Path, Size: f.Size,
				ModifiedAt: pgtype.Timestamptz{Time: f.Modified, Valid: true},
				Change:     f.Change, GroupIndex: int32(idx), GroupLabel: label,
			}); err != nil {
				return err
			}
		}
		status, msg := "done", ""
		if len(res.Files) == 0 {
			status = "no_files"
		}
		if res.Stopped {
			status, msg = "stopped", stoppedMessage
		} else if res.ExitCode != 0 {
			status, msg = "error", "O app fechou com erro (código "+strconv.Itoa(res.ExitCode)+"). Confira os arquivos de diagnóstico e peça ajuda ao responsável."
		}
		if err := h.q.FinishRun(ctx, sqlc.FinishRunParams{ID: runID, Status: status,
			ExitCode: pgtype.Int4{Int32: int32(res.ExitCode), Valid: true}, ErrorMessage: msg}); err != nil {
			return err
		}
		return h.finishJob(ctx, j.ID, "done", res)
	default: // failed
		msg := plainRunError(runID, res.Error)
		if err := h.q.FinishRun(ctx, sqlc.FinishRunParams{ID: runID, Status: "error", ErrorMessage: msg}); err != nil {
			return err
		}
		return h.finishJob(ctx, j.ID, "failed", res)
	}
}

// The Agente passes Windows' own error along when it can't start the app; that
// text is English and technical, so the person reads a plain sentence and the
// detail goes to the log.
const openFailPrefix = "Não consegui abrir o app: "

func plainRunError(runID int64, msg string) string {
	switch {
	case msg == "":
		return "O Agente não conseguiu abrir o app."
	case strings.HasPrefix(msg, openFailPrefix):
		slog.Warn("app did not start", "run", runID, "detail", strings.TrimPrefix(msg, openFailPrefix))
		return "O Windows não deixou abrir o app. Confira se o arquivo do app ainda existe e se o antivírus não o bloqueou, e tente de novo."
	}
	return msg
}

func (h *Handler) toRunJSON(ctx context.Context, row sqlc.GetRunRow) (runJSON, error) {
	j := runJSON{
		ID: row.ID, AppID: row.AppID, AppName: row.AppName, PersonName: row.PersonName, ComputerName: row.ComputerName,
		OutputFolder: row.OutputFolder, Status: row.Status, ErrorMessage: row.ErrorMessage, AppVersion: row.AppVersion,
		CreatedAt: row.CreatedAt.Time, StartedAt: tsPtr(row.StartedAt), FinishedAt: tsPtr(row.FinishedAt),
		Groups: []runGroupJSON{},
	}
	if row.ExitCode.Valid {
		j.ExitCode = &row.ExitCode.Int32
	}
	if row.VersionNumber.Valid {
		j.VersionNumber = &row.VersionNumber.Int32
	}
	var m manifest.Manifest
	if err := json.Unmarshal(row.AppManifest, &m); err != nil {
		return j, err
	}
	for i, o := range m.Outputs {
		j.Groups = append(j.Groups, runGroupJSON{Index: i, Label: o.Label, Files: []runFileJSON{}})
	}
	files, err := h.q.ListRunFiles(ctx, row.ID)
	if err != nil {
		return j, err
	}
	for _, f := range files {
		fj := runFileJSON{ID: f.ID, Path: f.RelPath, Size: f.Size, Modified: f.ModifiedAt.Time, Change: f.Change}
		// Group by the current manifest, so fixing its patterns fixes past runs too.
		idx, _ := outputs.Match(m.Outputs, f.RelPath)
		if idx >= 0 {
			j.Groups[idx].Files = append(j.Groups[idx].Files, fj)
			continue
		}
		last := len(j.Groups) - 1
		if last < 0 || j.Groups[last].Index != -1 {
			j.Groups = append(j.Groups, runGroupJSON{Index: -1, Label: outputs.OtherLabel, Files: []runFileJSON{}})
			last++
		}
		j.Groups[last].Files = append(j.Groups[last].Files, fj)
	}
	return j, nil
}

// runForPerson loads the run in the path. It is visible to its own person and
// to whoever can see that app and everyone's runs of it; otherwise 404.
func (h *Handler) runForPerson(w http.ResponseWriter, r *http.Request) (sqlc.GetRunRow, sqlc.Person, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return sqlc.GetRunRow{}, me, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
			internalError(w, "expire runs", err)
			return sqlc.GetRunRow{}, me, false
		}
		row, err := h.q.GetRun(r.Context(), id)
		if err == nil && row.PersonID != me.ID {
			acc, aerr := h.loadAccess(r.Context(), me)
			app, gerr := h.q.GetApp(r.Context(), row.AppID)
			if aerr != nil || gerr != nil {
				internalError(w, "run access", errors.Join(aerr, gerr))
				return sqlc.GetRunRow{}, me, false
			}
			if c := acc.For(appRef(app)); c.View && c.Runs {
				return row, me, true
			}
		} else if err == nil {
			return row, me, true
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get run", err)
			return row, me, false
		}
	}
	writeError(w, http.StatusNotFound, "Execução não encontrada.")
	return sqlc.GetRunRow{}, me, false
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	row, me, ok := h.runForPerson(w, r)
	if !ok {
		return
	}
	j, err := h.toRunJSON(r.Context(), row)
	if err != nil {
		internalError(w, "run json", err)
		return
	}
	// Only the person who ran it stops it: the app is open on their computer.
	j.CanStop = row.PersonID == me.ID && canStop(row.Status, row.ComputerCanStop)
	writeJSON(w, http.StatusOK, j)
}

func (h *Handler) listAppRuns(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	rows, err := h.q.ListAppRuns(r.Context(), sqlc.ListAppRunsParams{AppID: a.ID, SeeAll: can.Runs, PersonID: me.ID})
	if err != nil {
		internalError(w, "list runs", err)
		return
	}
	out := make([]runSummaryJSON, 0, len(rows))
	for _, x := range rows {
		out = append(out, runSummaryJSON{ID: x.ID, Status: x.Status, PersonName: x.PersonName, CreatedAt: x.CreatedAt.Time,
			StartedAt: tsPtr(x.StartedAt), FinishedAt: tsPtr(x.FinishedAt), FileCount: x.FileCount})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) chooseFolder(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	var body struct {
		Initial string `json:"initial"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	ctx := r.Context()
	comp, err := h.q.ConnectedComputerForPerson(ctx, me.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "Seu computador não está conectado.")
		return
	}
	if err != nil {
		internalError(w, "connected computer", err)
		return
	}
	job, err := h.enqueue(ctx, comp.ID, nil, agentproto.KindChooseFolder, agentproto.ChooseFolderPayload{Initial: body.Initial})
	if err != nil {
		internalError(w, "enqueue folder", err)
		return
	}
	done, ok := h.waitJob(ctx, job.ID, h.waitFolder)
	if !ok {
		writeError(w, http.StatusGatewayTimeout, "A janela de pastas não respondeu. Tente de novo.")
		return
	}
	var res agentproto.Result
	_ = json.Unmarshal(done.Result, &res)
	if done.Status == "failed" {
		writeError(w, http.StatusBadGateway, "Não consegui abrir a janela de pastas no seu computador.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folder": res.Folder, "canceled": res.Canceled || res.Folder == ""})
}

type runFileJSON struct {
	ID       int64     `json:"id"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Change   string    `json:"change"`
}
type runGroupJSON struct {
	Index int           `json:"index"`
	Label string        `json:"label"`
	Files []runFileJSON `json:"files"`
}
type runJSON struct {
	ID            int64          `json:"id"`
	AppID         int64          `json:"app_id"`
	AppName       string         `json:"app_name"`
	PersonName    string         `json:"person_name"`
	ComputerName  string         `json:"computer_name"`
	OutputFolder  string         `json:"output_folder"`
	Status        string         `json:"status"`
	ExitCode      *int32         `json:"exit_code"`
	ErrorMessage  string         `json:"error_message"`
	AppVersion    string         `json:"app_version"`
	VersionNumber *int32         `json:"version_number"`
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at"`
	FinishedAt    *time.Time     `json:"finished_at"`
	CanStop       bool           `json:"can_stop"`
	Groups        []runGroupJSON `json:"groups"`
}
type runSummaryJSON struct {
	ID         int64      `json:"id"`
	Status     string     `json:"status"`
	PersonName string     `json:"person_name"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	FileCount  int64      `json:"file_count"`
}

func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request) {
	row, _, ok := h.runForPerson(w, r)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(r.PathValue("fileId"), 10, 64)
	if err != nil {
		downloadError(w, r, http.StatusNotFound, "Arquivo não encontrado.")
		return
	}
	f, err := h.q.GetRunFile(r.Context(), sqlc.GetRunFileParams{ID: fileID, RunID: row.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		downloadError(w, r, http.StatusNotFound, "Arquivo não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get run file", err)
		return
	}
	name := f.RelPath[strings.LastIndex(f.RelPath, "/")+1:]
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	h.relayFromAgent(w, r, row, agentproto.SendFilePayload{RunID: row.ID, Paths: []string{f.RelPath}}, name, ctype)
}

func (h *Handler) downloadGroup(w http.ResponseWriter, r *http.Request) {
	row, _, ok := h.runForPerson(w, r)
	if !ok {
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		downloadError(w, r, http.StatusNotFound, "Grupo não encontrado.")
		return
	}
	j, err := h.toRunJSON(r.Context(), row)
	if err != nil {
		internalError(w, "run json", err)
		return
	}
	for _, g := range j.Groups {
		if g.Index != index || len(g.Files) == 0 {
			continue
		}
		paths := make([]string, 0, len(g.Files))
		for _, f := range g.Files {
			paths = append(paths, f.Path)
		}
		name := strings.Map(func(r rune) rune {
			if strings.ContainsRune(`\/:*?"<>|`, r) {
				return '-'
			}
			return r
		}, row.AppName+" - "+g.Label) + ".zip"
		h.relayFromAgent(w, r, row, agentproto.SendFilePayload{RunID: row.ID, Paths: paths, Zip: true}, name, "application/zip")
		return
	}
	downloadError(w, r, http.StatusNotFound, "Esse grupo não tem arquivos.")
}

// downloadSelected sends the files the person ticked: one file as itself,
// several as a zip. Every id must belong to the run.
func (h *Handler) downloadSelected(w http.ResponseWriter, r *http.Request) {
	row, _, ok := h.runForPerson(w, r)
	if !ok {
		return
	}
	var ids []int64
	for _, part := range strings.Split(r.URL.Query().Get("files"), ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			downloadError(w, r, http.StatusBadRequest, "Escolha pelo menos um arquivo.")
			return
		}
		ids = append(ids, id)
	}
	var files []sqlc.RunFile
	for _, id := range ids {
		f, err := h.q.GetRunFile(r.Context(), sqlc.GetRunFileParams{ID: id, RunID: row.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			downloadError(w, r, http.StatusNotFound, "Arquivo não encontrado.")
			return
		}
		if err != nil {
			internalError(w, "get run file", err)
			return
		}
		files = append(files, f)
	}
	if len(files) == 1 {
		name := files[0].RelPath[strings.LastIndex(files[0].RelPath, "/")+1:]
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		h.relayFromAgent(w, r, row, agentproto.SendFilePayload{RunID: row.ID, Paths: []string{files[0].RelPath}}, name, ctype)
		return
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.RelPath)
	}
	h.relayFromAgent(w, r, row, agentproto.SendFilePayload{RunID: row.ID, Paths: paths, Zip: true}, zipName(row.AppName+" - arquivos escolhidos"), "application/zip")
}

func zipName(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return '-'
		}
		return r
	}, s) + ".zip"
}

// relayFromAgent asks the run's computer for the files and streams its upload
// to the browser.
func (h *Handler) relayFromAgent(w http.ResponseWriter, r *http.Request, row sqlc.GetRunRow, p agentproto.SendFilePayload, name, ctype string) {
	ctx := r.Context()
	connected, err := h.q.IsComputerConnected(ctx, row.ComputerID)
	if err != nil {
		internalError(w, "computer connected", err)
		return
	}
	if !connected {
		downloadError(w, r, http.StatusConflict, "O computador onde o app rodou ("+row.ComputerName+") não está conectado. Ligue-o e tente de novo.")
		return
	}
	job, err := h.enqueue(ctx, row.ComputerID, &row.ID, agentproto.KindSendFile, p)
	if err != nil {
		internalError(w, "enqueue send", err)
		return
	}
	ch := h.uploads.expect(job.ID)
	defer h.uploads.cancel(job.ID)
	select {
	case up := <-ch:
		if up.missing {
			downloadError(w, r, http.StatusNotFound, "Esse arquivo não está mais na pasta.")
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
		_, err := io.Copy(w, up.body)
		up.done <- err
	case <-time.After(h.waitFile):
		downloadError(w, r, http.StatusGatewayTimeout, "O computador não respondeu. Confira se ele está ligado e tente de novo.")
	case <-ctx.Done():
	}
}

func (h *Handler) agentUpload(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	j, ok := h.jobForComputer(w, r, c)
	if !ok {
		return
	}
	if j.Kind != agentproto.KindSendFile {
		writeError(w, http.StatusBadRequest, "Esse pedido não é de envio de arquivo.")
		return
	}
	done, ok := h.uploads.deliver(j.ID, r.Body, 5*time.Second)
	if !ok {
		writeError(w, http.StatusGone, "Ninguém está esperando esse arquivo.")
		return
	}
	var copyErr error
	select {
	case copyErr = <-done:
	case <-r.Context().Done():
		copyErr = r.Context().Err()
	}
	status := "done"
	if copyErr != nil {
		status = "failed"
	}
	if err := h.finishJob(r.Context(), j.ID, status, agentproto.Result{Event: agentproto.EventDone}); err != nil {
		internalError(w, "finish send", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// downloadError answers a download link. Browsers navigate to these links, so
// they get a small readable page instead of JSON.
func downloadError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		writeError(w, status, msg)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html lang="pt-BR"><meta charset="utf-8"><title>Não foi possível baixar</title>
<body style="font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;line-height:1.5">
<h1 style="font-size:1.25rem">Não foi possível baixar</h1><p>%s</p>
<p><a href="javascript:history.back()">Voltar</a></p></body></html>`, html.EscapeString(msg))
}

type activeRunJSON struct {
	ID           int64      `json:"id"`
	AppID        int64      `json:"app_id"`
	AppName      string     `json:"app_name"`
	AppFolder    string     `json:"app_folder_display"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message"`
	OutputFolder string     `json:"output_folder"`
	FileCount    int64      `json:"file_count"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CanStop      bool       `json:"can_stop"`
}

type runListItemJSON struct {
	ID           int64      `json:"id"`
	AppID        int64      `json:"app_id"`
	AppName      string     `json:"app_name"`
	Status       string     `json:"status"`
	OutputFolder string     `json:"output_folder"`
	PersonID     int64      `json:"person_id"`
	PersonName   string     `json:"person_name"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	FileCount    int64      `json:"file_count"`
}

type runListJSON struct {
	Items []runListItemJSON `json:"items"`
	More  bool              `json:"more"`
}

// activeRuns feeds the run strip: the person's runs not dismissed yet.
func (h *Handler) activeRuns(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	rows, err := h.q.ListActiveRuns(r.Context(), me.ID)
	if err != nil {
		internalError(w, "active runs", err)
		return
	}
	out := make([]activeRunJSON, 0, len(rows))
	for _, x := range rows {
		out = append(out, activeRunJSON{ID: x.ID, AppID: x.AppID, AppName: x.AppName, AppFolder: hostpath.ToDisplay(x.AppFolderPath), Status: x.Status,
			ErrorMessage: x.ErrorMessage, OutputFolder: x.OutputFolder, FileCount: x.FileCount, CreatedAt: x.CreatedAt.Time, FinishedAt: tsPtr(x.FinishedAt),
			CanStop: canStop(x.Status, x.ComputerCanStop)})
	}
	writeJSON(w, http.StatusOK, out)
}

// dismissRun hides a run from the strip. Only the person who ran it can.
func (h *Handler) dismissRun(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Execução não encontrada.")
		return
	}
	n, err := h.q.DismissRun(r.Context(), sqlc.DismissRunParams{ID: id, PersonID: me.ID})
	if err != nil {
		internalError(w, "dismiss run", err)
		return
	}
	if n == 0 {
		// Already dismissed is fine; someone else's or missing is 404.
		row, err := h.q.GetRun(r.Context(), id)
		if err != nil || row.PersonID != me.ID {
			writeError(w, http.StatusNotFound, "Execução não encontrada.")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// listRuns is Minhas execuções: the person's runs; with scope=all also the
// runs of others in the apps whose runs the person may see.
func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	// scope=all adds the others' runs of the apps the person sees and may see
	// everyone's runs of (see_all_runs reaches only visible apps). With none
	// in reach it is just the person's own runs, never an error.
	appIDs := []int64{}
	if r.URL.Query().Get("scope") == "all" {
		acc, err := h.loadAccess(r.Context(), me)
		if err != nil {
			internalError(w, "load access", err)
			return
		}
		links, err := h.q.ListAppLinks(r.Context())
		if err != nil {
			internalError(w, "list app links", err)
			return
		}
		for _, l := range links {
			c := acc.For(access.App{ID: l.ID, AreaID: l.AreaID, OwnerID: l.OwnerID, BackupID: l.BackupID.Int64, Restricted: l.Restricted})
			if c.View && c.Runs {
				appIDs = append(appIDs, l.ID)
			}
		}
	}
	var before int64
	if s := r.URL.Query().Get("before"); s != "" {
		var err error
		if before, err = strconv.ParseInt(s, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "Não entendi de onde carregar mais.")
			return
		}
	}
	if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	rows, err := h.q.ListRuns(r.Context(), sqlc.ListRunsParams{PersonID: me.ID, AppIds: appIDs, Before: before, Lim: int32(h.runsPage + 1)})
	if err != nil {
		internalError(w, "list runs", err)
		return
	}
	out := runListJSON{Items: []runListItemJSON{}}
	for i, x := range rows {
		if i == h.runsPage {
			out.More = true
			break
		}
		out.Items = append(out.Items, runListItemJSON{ID: x.ID, AppID: x.AppID, AppName: x.AppName, Status: x.Status,
			OutputFolder: x.OutputFolder, PersonID: x.PersonID, PersonName: x.PersonName, CreatedAt: x.CreatedAt.Time,
			StartedAt: tsPtr(x.StartedAt), FinishedAt: tsPtr(x.FinishedAt), FileCount: x.FileCount})
	}
	writeJSON(w, http.StatusOK, out)
}
