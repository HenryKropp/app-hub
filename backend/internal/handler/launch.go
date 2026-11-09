package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/hostpath"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// launchRequest is everything needed to start one run on a chosen computer.
// The Rodar button and the schedule clock both go through launchRun, so a
// schedule never skips a check the button makes.
type launchRequest struct {
	App          sqlc.App
	Person       sqlc.Person
	Computer     sqlc.Computer
	Manifest     manifest.Manifest
	OutputFolder string
	Inputs       map[string][]string
	Files        map[string]manifest.InputFile
	Origin       string // "manual" | "mcp" | "schedule"
	ScheduleID   pgtype.Int8
}

// launchError is a refusal the person can act on. Code lets the scheduler
// decide what to do: blocked, invalid_inputs, agent_outdated, folder_busy,
// code_changed.
type launchError struct {
	Status int
	Msg    string
	Code   string
}

func refuse(status int, code, msg string) *launchError {
	return &launchError{Status: status, Code: code, Msg: msg}
}

// launchRun checks that the run may happen, records it and queues it for the
// computer's agent. err is an internal failure; *launchError a refusal.
func (h *Handler) launchRun(ctx context.Context, req launchRequest) (sqlc.Run, *launchError, error) {
	a, m := req.App, req.Manifest
	if reason, err := h.executionBlock(ctx, a.ID); err != nil {
		return sqlc.Run{}, nil, err
	} else if reason != "" {
		return sqlc.Run{}, refuse(403, "blocked", reason), nil
	}
	script := m.Kind == manifest.KindScript
	if script {
		if err := m.ValidateRunInputs(req.Inputs, req.Files); err != nil {
			return sqlc.Run{}, refuse(400, "invalid_inputs", err.Error()), nil
		}
		if req.Computer.AgentVersion != agentproto.Version {
			return sqlc.Run{}, refuse(409, "agent_outdated", "Atualize o Agente em Configurações › Meu computador para rodar scripts."), nil
		}
		if len(m.Keys) > 0 {
			keys, err := h.appKeys(ctx, a)
			if err != nil {
				return sqlc.Run{}, nil, err
			}
			if keys.Locked {
				return sqlc.Run{}, refuse(503, "keys_locked", msgKeysLocked), nil
			}
			if len(keys.Missing) > 0 {
				return sqlc.Run{}, refuse(409, "keys_missing", "Falta preencher no cofre do app: "+strings.Join(keys.Missing, ", ")+". O dono do app ou um administrador preenche na página do app, parte Cofre."), nil
			}
		}
	}
	if msg, err := h.folderInUse(ctx, req.Computer.ID, req.OutputFolder, script); err != nil {
		return sqlc.Run{}, nil, err
	} else if msg != "" {
		return sqlc.Run{}, refuse(409, "folder_busy", msg), nil
	}
	// Hold the app lock from the version check until the job is queued, so a
	// restore cannot rewrite the folder between recording the version and the run.
	unlock := h.lockApp(a.ID)
	defer unlock()
	v, _, readable, err := h.checkVersionLocked(ctx, a)
	if err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	// A scan may have discovered changed code. Recheck the reviewed version while
	// holding the same lock used by approval and suspension.
	if reason, err := h.executionBlock(ctx, a.ID); err != nil {
		return sqlc.Run{}, nil, err
	} else if reason != "" {
		return sqlc.Run{}, refuse(403, "blocked", reason), nil
	}
	// Only a version the Hub just confirmed in the folder is linked to the run.
	versionID := pgtype.Int8{Int64: v.ID, Valid: readable && v.ID != 0}
	var sources map[string]manifest.InputFile
	if script {
		sources, err = h.importedSources(ctx, a.ID)
		if err != nil {
			return sqlc.Run{}, nil, err
		}
		if h.serverPath(a.FolderPath, a.ID) != nil {
			return sqlc.Run{}, refuse(409, "folder_not_on_server", errFolderNotOnServer.Error()), nil
		}
		for name, f := range sources {
			if !manifest.SafeRelative(name) {
				return sqlc.Run{}, refuse(409, "code_changed", "O pacote salvo não é seguro. Reimporte o script."), nil
			}
			original, readErr := base64.StdEncoding.DecodeString(f.ContentBase64)
			if readErr != nil {
				return sqlc.Run{}, refuse(409, "code_changed", "O pacote salvo está inválido. Reimporte o script."), nil
			}
			current, readErr := os.ReadFile(filepath.Join(a.FolderPath, filepath.FromSlash(name)))
			if readErr != nil || !bytes.Equal(current, original) {
				return sqlc.Run{}, refuse(409, "code_changed", "O código da pasta está diferente do pacote salvo. Reimporte o script antes de executar."), nil
			}
		}
	}
	run, err := h.q.CreateRun(ctx, sqlc.CreateRunParams{AppID: a.ID, PersonID: req.Person.ID, ComputerID: req.Computer.ID, OutputFolder: req.OutputFolder, VersionID: versionID, Origin: req.Origin, ScheduleID: req.ScheduleID})
	if err != nil {
		return sqlc.Run{}, nil, err
	}
	payload := agentproto.RunPayload{RunID: run.ID, AppFolder: hostpath.ToDisplay(a.FolderPath), Exe: m.Entrypoint, OutputFolder: req.OutputFolder, AppName: a.Name}
	if script {
		payload.Script = &agentproto.ScriptPayload{Manifest: m, Inputs: req.Inputs, Files: req.Files, Sources: sources}
		payload.Keys = len(m.Keys) > 0
	}
	if _, err = h.enqueue(ctx, req.Computer.ID, &run.ID, agentproto.KindRun, payload); err != nil {
		return sqlc.Run{}, nil, err
	}
	return run, nil, nil
}
