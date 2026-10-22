package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

func hashToken(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// pairingCode is short and easy to read: no 0/O, 1/I.
func pairingCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// currentComputer authenticates an agent by its Bearer token and records that
// it was seen.
func (h *Handler) currentComputer(w http.ResponseWriter, r *http.Request) (sqlc.Computer, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if ok && token != "" {
		c, err := h.q.GetComputerByToken(r.Context(), hashToken(token))
		if err == nil {
			if err := h.q.TouchComputer(r.Context(), c.ID); err != nil {
				internalError(w, "touch computer", err)
				return c, false
			}
			return c, true
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get computer", err)
			return sqlc.Computer{}, false
		}
	}
	writeError(w, http.StatusUnauthorized, "Agente não conectado.")
	return sqlc.Computer{}, false
}

func (h *Handler) pairStart(w http.ResponseWriter, r *http.Request) {
	var body agentproto.PairStartRequest
	if !readJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.ComputerName)
	if name == "" || len(name) > 100 {
		writeError(w, http.StatusBadRequest, "Informe o nome do computador.")
		return
	}
	code, err := pairingCode()
	if err != nil {
		internalError(w, "pairing code", err)
		return
	}
	secret, err := randomToken()
	if err != nil {
		internalError(w, "pairing secret", err)
		return
	}
	if err := h.q.CreatePairing(r.Context(), sqlc.CreatePairingParams{Code: code, SecretHash: hashToken(secret), ComputerName: name}); err != nil {
		internalError(w, "create pairing", err)
		return
	}
	writeJSON(w, http.StatusOK, agentproto.PairStartResponse{
		Code: code, Secret: secret, ApproveURL: strings.TrimRight(h.cfg.BaseURL, "/") + "/conectar?codigo=" + code,
	})
}

func (h *Handler) pairPoll(w http.ResponseWriter, r *http.Request) {
	var body agentproto.PairPollRequest
	if !readJSON(w, r, &body) {
		return
	}
	secretHash := hashToken(body.Secret)
	state, err := h.q.GetPairingStateBySecret(r.Context(), secretHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && state.Expired) {
		writeError(w, http.StatusGone, "Esse pedido de conexão venceu. Abra o Agente de novo.")
		return
	}
	if err != nil {
		internalError(w, "pairing state", err)
		return
	}
	if !state.PersonID.Valid {
		writeJSON(w, http.StatusOK, agentproto.PairPollResponse{Status: "pending"})
		return
	}
	p, err := h.q.ConsumePairing(r.Context(), secretHash)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusGone, "Esse pedido de conexão já foi usado.")
		return
	}
	if err != nil {
		internalError(w, "consume pairing", err)
		return
	}
	token, err := randomToken()
	if err != nil {
		internalError(w, "token", err)
		return
	}
	if _, err := h.q.CreateComputer(r.Context(), sqlc.CreateComputerParams{PersonID: p.PersonID.Int64, Name: p.ComputerName, TokenHash: hashToken(token)}); err != nil {
		internalError(w, "create computer", err)
		return
	}
	writeJSON(w, http.StatusOK, agentproto.PairPollResponse{Status: "approved", Token: token})
}

func (h *Handler) getPairing(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	p, err := h.q.GetPairing(r.Context(), r.PathValue("code"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Não encontrei esse pedido de conexão. Abra o Agente de novo.")
		return
	}
	if err != nil {
		internalError(w, "get pairing", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": p.Code, "computer_name": p.ComputerName, "expired": p.Expired, "approved": p.PersonID.Valid,
	})
}

func (h *Handler) approvePairing(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	n, err := h.q.ApprovePairing(r.Context(), sqlc.ApprovePairingParams{Code: r.PathValue("code"), PersonID: pgtype.Int8{Int64: me.ID, Valid: true}})
	if err != nil {
		internalError(w, "approve pairing", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusGone, "Esse pedido de conexão venceu ou já foi usado. Abra o Agente de novo.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type computerJSON struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	AgentVersion string     `json:"agent_version"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	Connected    bool       `json:"connected"`
	// Outdated: the person has to download the agent by hand.
	Outdated      bool       `json:"outdated"`
	UpdateState   string     `json:"update_state"` // "", "updating", "updated", "gave_up"
	UpdateVersion string     `json:"update_version,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

func (h *Handler) myComputers(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListPersonComputers(r.Context(), me.ID)
	if err != nil {
		internalError(w, "list computers", err)
		return
	}
	out := make([]computerJSON, 0, len(rows))
	for _, c := range rows {
		j := computerJSON{ID: c.ID, Name: c.Name, AgentVersion: c.AgentVersion, Connected: c.Connected}
		if c.LastSeenAt.Valid {
			j.LastSeenAt = &c.LastSeenAt.Time
		}
		differs := c.AgentVersion != "" && c.AgentVersion != agentproto.Version
		if j.UpdateState, j.UpdateVersion, j.UpdatedAt, err = h.updateState(r.Context(), c.ID, differs); err != nil {
			internalError(w, "update state", err)
			return
		}
		j.Outdated = differs && (!c.CanUpdate || j.UpdateState == "gave_up")
		out = append(out, j)
	}
	_, statErr := os.Stat(h.cfg.AgentBinaryPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"computers": out, "expected_version": agentproto.Version,
		"agent_available": h.cfg.AgentBinaryPath != "" && statErr == nil,
	})
}

func (h *Handler) revokeComputer(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Computador não encontrado.")
		return
	}
	c, err := h.q.GetComputer(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Computador não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get computer", err)
		return
	}
	if c.PersonID != me.ID {
		acc, err := h.loadAccess(r.Context(), me)
		if err != nil {
			internalError(w, "load access", err)
			return
		}
		if !acc.Has(access.ManagePeople) {
			writeError(w, http.StatusNotFound, "Computador não encontrado.")
			return
		}
	}
	if err := h.q.RevokeComputer(r.Context(), id); err != nil {
		internalError(w, "revoke computer", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// agentHello is called when the agent starts. Runs that were open on this
// computer cannot be finished any more.
func (h *Handler) agentHello(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	var body agentproto.Hello
	if !readJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = c.Name
	}
	if err := h.q.HelloComputer(r.Context(), sqlc.HelloComputerParams{ID: c.ID, Name: name, AgentVersion: body.Version, CanUpdate: body.CanUpdate}); err != nil {
		internalError(w, "hello", err)
		return
	}
	if err := h.q.ResolveTakenUpdates(r.Context(), sqlc.ResolveTakenUpdatesParams{ComputerID: c.ID, HelloVersion: body.Version}); err != nil {
		internalError(w, "resolve updates", err)
		return
	}
	if err := h.q.InterruptTakenRuns(r.Context(), c.ID); err != nil {
		internalError(w, "interrupt runs", err)
		return
	}
	if err := h.q.FailTakenJobs(r.Context(), c.ID); err != nil {
		internalError(w, "fail jobs", err)
		return
	}
	c.AgentVersion, c.CanUpdate = body.Version, body.CanUpdate
	if err := h.maybeEnqueueUpdate(r.Context(), c); err != nil {
		slog.Error("queue agent update", "computer", c.ID, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) enqueue(ctx context.Context, computerID int64, runID *int64, kind string, payload any) (sqlc.AgentJob, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return sqlc.AgentJob{}, err
	}
	run := pgtype.Int8{}
	if runID != nil {
		run = pgtype.Int8{Int64: *runID, Valid: true}
	}
	return h.q.CreateJob(ctx, sqlc.CreateJobParams{ComputerID: computerID, RunID: run, Kind: kind, Payload: raw})
}

// waitJob polls until the job is done or failed, or max passes.
func (h *Handler) waitJob(ctx context.Context, id int64, max time.Duration) (sqlc.AgentJob, bool) {
	deadline := time.Now().Add(max)
	for {
		j, err := h.q.GetJob(ctx, id)
		if err == nil && (j.Status == "done" || j.Status == "failed") {
			return j, true
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return j, false
		}
		time.Sleep(h.tick)
	}
}

func (h *Handler) agentNext(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := h.q.ExpireStaleRuns(ctx); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	if err := h.q.ExpireStaleJobs(ctx); err != nil {
		internalError(w, "expire jobs", err)
		return
	}
	if err := h.maybeEnqueueUpdate(ctx, c); err != nil {
		slog.Error("queue agent update", "computer", c.ID, "err", err)
	}
	deadline := time.Now().Add(h.waitNext)
	for {
		j, err := h.q.ClaimJob(ctx, c.ID)
		if err == nil {
			writeJSON(w, http.StatusOK, agentproto.Job{ID: j.ID, Kind: j.Kind, Payload: j.Payload})
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "claim job", err)
			return
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		time.Sleep(h.tick)
	}
}

// jobForComputer loads a job by path id and checks it belongs to c.
func (h *Handler) jobForComputer(w http.ResponseWriter, r *http.Request, c sqlc.Computer) (sqlc.AgentJob, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		j, err := h.q.GetJob(r.Context(), id)
		if err == nil && j.ComputerID == c.ID {
			return j, true
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get job", err)
			return j, false
		}
	}
	writeError(w, http.StatusNotFound, "Pedido não encontrado.")
	return sqlc.AgentJob{}, false
}

func (h *Handler) agentResult(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	j, ok := h.jobForComputer(w, r, c)
	if !ok {
		return
	}
	var res agentproto.Result
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20) // big run file lists
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		writeError(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	ctx := r.Context()
	var err error
	switch j.Kind {
	case agentproto.KindRun:
		err = h.onRunResult(ctx, j, res)
	case agentproto.KindSendFile:
		if res.Event == agentproto.EventMissing || res.Event == agentproto.EventFailed {
			h.uploads.missing(j.ID)
		}
		err = h.finishJob(ctx, j.ID, "failed", res)
	default:
		status := "done"
		if res.Event == agentproto.EventFailed {
			status = "failed"
		}
		err = h.finishJob(ctx, j.ID, status, res)
	}
	if err != nil {
		internalError(w, "job result", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) finishJob(ctx context.Context, id int64, status string, res agentproto.Result) error {
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if err := h.q.FinishJob(ctx, sqlc.FinishJobParams{ID: id, Status: status, Result: raw}); err != nil {
		return err
	}
	return h.q.ClearRunInputs(ctx, id)
}
func (h *Handler) agentDownload(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	if h.cfg.AgentBinaryPath == "" {
		writeError(w, http.StatusNotFound, "O Agente ainda não foi preparado neste Hub.")
		return
	}
	if _, err := os.Stat(h.cfg.AgentBinaryPath); err != nil {
		writeError(w, http.StatusNotFound, "O Agente ainda não foi preparado neste Hub.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.microsoft.portable-executable")
	w.Header().Set("Content-Disposition", `attachment; filename="AppHubAgente.exe"`)
	http.ServeFile(w, r, h.cfg.AgentBinaryPath)
}
