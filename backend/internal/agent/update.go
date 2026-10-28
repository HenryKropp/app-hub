package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// ErrBadDownload: the file from the Hub does not match the update job.
var ErrBadDownload = errors.New("downloaded agent does not match the update job")

// Updater replaces this agent with the version the Hub sends.
type Updater struct {
	Client     *Client
	Runner     *Runner
	StateDir   string
	CurrentExe string                                 // the running agent
	Start      func(exe string, args ...string) error // opens a program without waiting
	// HandOver is how long to wait for the new agent to close this one after
	// a successful Start (0 = 5 minutes, above the swap's 2-minute watchdog).
	HandOver time.Duration
	// DownloadTimeout bounds the binary download so a stalled one cannot hold
	// the runner forever (0 = 10 minutes).
	DownloadTimeout time.Duration
}

// Apply waits until no app is open, downloads and checks the new agent, keeps
// a copy of the current one and starts the new one in update mode. On success
// the new agent closes this one (via /quit), so Apply's caller never reports.
// After a successful start the runner stays held: no app may open in the
// seconds before this agent is closed.
func (u *Updater) Apply(ctx context.Context, p agentproto.UpdatePayload) error {
	release, err := u.Runner.Hold(ctx)
	if err != nil {
		return nil // the agent is shutting down: do not report
	}
	if err := u.handOver(ctx, p); err != nil {
		release()
		return err
	}
	wait := u.HandOver
	if wait <= 0 {
		wait = 5 * time.Minute
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		// The agent is shutting down: stay held, do not report.
		return nil
	case <-t.C:
		// The new agent never closed this one: give the runner back.
		release()
		return errors.New("new agent did not take over in time")
	}
}

// capWriter refuses bytes beyond its limit, so an oversized download stops early.
type capWriter struct {
	w    io.Writer
	left int64
}

func (c *capWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > c.left {
		return 0, ErrBadDownload
	}
	c.left -= int64(len(b))
	return c.w.Write(b)
}

func (u *Updater) handOver(ctx context.Context, p agentproto.UpdatePayload) error {
	dir := filepath.Join(u.StateDir, "update")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dl := filepath.Join(dir, "AppHubAgente.exe")
	if err := u.download(ctx, dl, p); err != nil {
		_ = os.Remove(dl)
		return err
	}
	if err := replaceFile(u.CurrentExe, filepath.Join(u.StateDir, "AppHubAgente.anterior.exe")); err != nil {
		return fmt.Errorf("back up current agent: %w", err)
	}
	return u.Start(dl, "--update")
}

func (u *Updater) download(ctx context.Context, dst string, p agentproto.UpdatePayload) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	sum := sha256.New()
	limit := u.DownloadTimeout
	if limit <= 0 {
		limit = 10 * time.Minute
	}
	dctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	n, err := u.Client.DownloadBinary(dctx, io.MultiWriter(&capWriter{w: f, left: p.Size}, sum))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n != p.Size || hex.EncodeToString(sum.Sum(nil)) != p.SHA256 {
		return ErrBadDownload
	}
	return nil
}

// replaceFile copies src over dst through a temporary file, so dst is either
// the old or the new content, never half of each.
func replaceFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
