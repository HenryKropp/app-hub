package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// maxUpdateTries is how many taken updates to one version may fail before the
// Hub stops asking and shows "Atualize o Agente" again.
const maxUpdateTries = 3

// maybeEnqueueUpdate queues an update job when this computer's agent can
// update itself and runs a different version than the Hub's.
func (h *Handler) maybeEnqueueUpdate(ctx context.Context, c sqlc.Computer) error {
	if !c.CanUpdate || c.AgentVersion == "" || c.AgentVersion == agentproto.Version || h.cfg.AgentBinaryPath == "" {
		return nil
	}
	active, err := h.q.CountActiveUpdates(ctx, c.ID)
	if err != nil || active > 0 {
		return err
	}
	failed, err := h.q.CountFailedUpdates(ctx, sqlc.CountFailedUpdatesParams{ComputerID: c.ID, TargetVersion: agentproto.Version})
	if err != nil || failed >= maxUpdateTries {
		return err
	}
	sum, size, err := fileDigest(h.cfg.AgentBinaryPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = h.enqueue(ctx, c.ID, nil, agentproto.KindUpdate, agentproto.UpdatePayload{Version: agentproto.Version, SHA256: sum, Size: size})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil // hello and next raced; one update is already queued
	}
	return err
}

func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return "", 0, err
	} else if !info.Mode().IsRegular() {
		return "", 0, fs.ErrNotExist
	}
	sum := sha256.New()
	n, err := io.Copy(sum, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(sum.Sum(nil)), n, nil
}

// agentBinary serves the agent binary to a connected agent (update jobs).
func (h *Handler) agentBinary(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentComputer(w, r); !ok {
		return
	}
	if h.cfg.AgentBinaryPath == "" {
		writeError(w, http.StatusNotFound, "O Agente ainda não foi preparado neste Hub.")
		return
	}
	if info, err := os.Stat(h.cfg.AgentBinaryPath); err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "O Agente ainda não foi preparado neste Hub.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.microsoft.portable-executable")
	http.ServeFile(w, r, h.cfg.AgentBinaryPath)
}

// updateState summarises the latest update job of a computer for the page.
func (h *Handler) updateState(ctx context.Context, computerID int64, differs bool) (string, string, *time.Time, error) {
	j, err := h.q.LatestUpdateJob(ctx, computerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, nil
	}
	if err != nil {
		return "", "", nil, err
	}
	var p agentproto.UpdatePayload
	if json.Unmarshal(j.Payload, &p) != nil || p.Version == "" {
		p.Version = agentproto.Version
	}
	switch {
	case j.Status == "pending" || j.Status == "taken":
		return "updating", p.Version, nil, nil
	case j.Status == "done" && j.DoneAt.Valid && time.Since(j.DoneAt.Time) < 24*time.Hour && !differs:
		return "updated", p.Version, &j.DoneAt.Time, nil
	}
	if !differs {
		return "", "", nil, nil
	}
	failed, err := h.q.CountFailedUpdates(ctx, sqlc.CountFailedUpdatesParams{ComputerID: computerID, TargetVersion: agentproto.Version})
	if err != nil {
		return "", "", nil, err
	}
	if failed >= maxUpdateTries {
		return "gave_up", "", nil, nil
	}
	return "", "", nil, nil
}
