package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const stoppedMessage = "Você parou o app pelo Hub."

// stopWait is how long stopRun waits for the agent to close the app before
// answering that it is still on its way.
const stopWait = 15 * time.Second

// canStop: a run still waiting can be called off in the Hub alone; one that
// is open needs an agent that understands stop jobs.
func canStop(status string, agentCanStop bool) bool {
	return status == "waiting" || (status == "running" && agentCanStop)
}

// stopRun closes an app the Hub opened. Only the person who ran it can.
// 204: stopped; 202: the agent got the request but has not answered yet.
func (h *Handler) stopRun(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var row sqlc.GetRunRow
	if err == nil {
		row, err = h.q.GetRun(ctx, id)
	}
	if err == nil && row.PersonID != me.ID {
		err = pgx.ErrNoRows
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, strconv.ErrSyntax) || errors.Is(err, strconv.ErrRange) {
			writeError(w, http.StatusNotFound, "Execução não encontrada.")
			return
		}
		internalError(w, "get run", err)
		return
	}
	switch row.Status {
	case "waiting":
		n, err := h.q.CancelPendingRunJob(ctx, pgtype.Int8{Int64: id, Valid: true})
		if err != nil {
			internalError(w, "cancel run job", err)
			return
		}
		if n == 0 {
			writeError(w, http.StatusConflict, "O app está abrindo agora. Espere ele aparecer e tente parar de novo.")
			return
		}
		if err := h.q.FinishRun(ctx, sqlc.FinishRunParams{ID: id, Status: "stopped", ErrorMessage: stoppedMessage}); err != nil {
			internalError(w, "stop run", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case "running":
	default:
		writeError(w, http.StatusConflict, "Este app já fechou.")
		return
	}
	if !row.ComputerCanStop {
		writeError(w, http.StatusConflict, "O Agente do "+row.ComputerName+" é antigo e não sabe parar apps. Ele se atualiza sozinho quando o app fechar; até lá, feche a janela do app no computador.")
		return
	}
	connected, err := h.q.IsComputerConnected(ctx, row.ComputerID)
	if err != nil {
		internalError(w, "computer connected", err)
		return
	}
	if !connected {
		writeError(w, http.StatusConflict, "O "+row.ComputerName+" não está respondendo. Confira se ele está ligado com o Agente aberto.")
		return
	}
	job, err := h.enqueue(ctx, row.ComputerID, &id, agentproto.KindStop, agentproto.StopPayload{RunID: id})
	if err != nil {
		internalError(w, "queue stop", err)
		return
	}
	j, done := h.waitJob(ctx, job.ID, stopWait)
	if !done {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if j.Status == "failed" {
		var res agentproto.Result
		_ = json.Unmarshal(j.Result, &res)
		msg := res.Error
		if msg == "" {
			msg = "O Agente não conseguiu parar o app."
		}
		writeError(w, http.StatusConflict, msg)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
