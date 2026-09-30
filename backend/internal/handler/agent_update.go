package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"

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
	if _, err := os.Stat(h.cfg.AgentBinaryPath); err != nil {
		writeError(w, http.StatusNotFound, "O Agente ainda não foi preparado neste Hub.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.microsoft.portable-executable")
	http.ServeFile(w, r, h.cfg.AgentBinaryPath)
}
