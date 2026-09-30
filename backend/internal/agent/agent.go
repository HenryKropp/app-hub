package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// Agent receives jobs from the Hub and carries them out.
type Agent struct {
	Client       *Client
	Runner       *Runner
	StateDir     string
	Name         string
	ChooseFolder func(ctx context.Context, initial string) (folder string, ok bool, err error)
	// RetryBase is the first wait before re-sending a result (default 1 s).
	RetryBase time.Duration
	// Updater applies update jobs; nil means this agent cannot update itself.
	Updater *Updater
}

// Loop asks the Hub for jobs until ctx ends or the token is revoked.
func (a *Agent) Loop(ctx context.Context) error {
	backoff := time.Second
	for ctx.Err() == nil {
		job, err := a.Client.Next(ctx)
		if errors.Is(err, ErrUnauthorized) {
			return err
		}
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			slog.Warn("hub unreachable", "err", err, "retry_in", backoff)
			time.Sleep(backoff)
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		if job != nil {
			go a.handle(ctx, *job)
		}
	}
	return ctx.Err()
}

// report sends a job result, retrying while the Hub is unreachable (it may be
// restarting) so that a finished run is not lost.
func (a *Agent) report(ctx context.Context, id int64, r agentproto.Result) {
	wait := a.RetryBase
	if wait <= 0 {
		wait = time.Second
	}
	for {
		err := a.Client.Result(ctx, id, r)
		if err == nil {
			return
		}
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFound) {
			slog.Error("report result: hub refused", "job", id, "err", err)
			return
		}
		slog.Warn("report result: retrying", "job", id, "err", err, "in", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, 30*time.Second)
	}
}

func (a *Agent) handle(ctx context.Context, job agentproto.Job) {
	switch job.Kind {
	case agentproto.KindRun:
		var p agentproto.RunPayload
		if err := json.Unmarshal(job.Payload, &p); err != nil {
			a.report(ctx, job.ID, failed("Pedido inválido."))
			return
		}
		res := a.Runner.Run(ctx, p, func(r agentproto.Result) { a.report(ctx, job.ID, r) })
		a.report(ctx, job.ID, res)
	case agentproto.KindChooseFolder:
		var p agentproto.ChooseFolderPayload
		_ = json.Unmarshal(job.Payload, &p)
		if a.ChooseFolder == nil {
			a.report(ctx, job.ID, failed("Este computador não sabe abrir a janela de pastas."))
			return
		}
		folder, ok, err := a.ChooseFolder(ctx, p.Initial)
		if err != nil {
			slog.Error("choose folder", "err", err)
			a.report(ctx, job.ID, failed("Não consegui abrir a janela de pastas."))
			return
		}
		a.report(ctx, job.ID, agentproto.Result{Event: agentproto.EventDone, Folder: folder, Canceled: !ok})
	case agentproto.KindSendFile:
		var p agentproto.SendFilePayload
		_ = json.Unmarshal(job.Payload, &p)
		out, err := PrepareSend(a.StateDir, p)
		if errors.Is(err, ErrMissing) {
			a.report(ctx, job.ID, agentproto.Result{Event: agentproto.EventMissing})
			return
		}
		if err != nil {
			slog.Warn("send refused", "run", p.RunID, "err", err)
			a.report(ctx, job.ID, failed("Arquivo não permitido."))
			return
		}
		pr, pw := io.Pipe()
		go func() { pw.CloseWithError(out.Stream(pw)) }()
		if err := a.Client.Upload(ctx, job.ID, pr); err != nil {
			slog.Error("upload", "job", job.ID, "err", err)
		}
		_ = pr.Close()
	case agentproto.KindUpdate:
		var p agentproto.UpdatePayload
		if err := json.Unmarshal(job.Payload, &p); err != nil || a.Updater == nil {
			a.report(ctx, job.ID, failed("Este Agente não consegue se atualizar sozinho."))
			return
		}
		if err := a.Updater.Apply(ctx, p); err != nil {
			slog.Error("update", "version", p.Version, "err", err)
			msg := "Não consegui atualizar o Agente."
			if errors.Is(err, ErrBadDownload) {
				msg = "O arquivo baixado veio diferente do esperado."
			}
			a.report(ctx, job.ID, failed(msg))
		}
	default:
		a.report(ctx, job.ID, failed("Pedido desconhecido."))
	}
}

// Pair asks the Hub for a connection, opens the approval page and waits.
func Pair(ctx context.Context, c *Client, name string, open func(string) error) (string, error) {
	var start agentproto.PairStartResponse
	for backoff := time.Second; ; backoff = min(backoff*2, 30*time.Second) {
		var err error
		if start, err = c.PairStart(ctx, name); err == nil {
			break
		}
		slog.Warn("pair start", "err", err)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff):
		}
	}
	if err := open(start.ApproveURL); err != nil {
		slog.Error("open browser", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
		res, err := c.PairPoll(ctx, start.Secret)
		if errors.Is(err, ErrGone) {
			return "", err
		}
		if err == nil && res.Status == "approved" {
			return res.Token, nil
		}
	}
}

type state struct {
	Token string `json:"token"`
}

func LoadToken(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "agente.json"))
	if err != nil {
		return ""
	}
	var s state
	_ = json.Unmarshal(b, &s)
	return s.Token
}

func SaveToken(dir, token string) error {
	b, _ := json.Marshal(state{Token: token})
	return os.WriteFile(filepath.Join(dir, "agente.json"), b, 0o600)
}
