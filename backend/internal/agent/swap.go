package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// ErrRolledBack: the new agent did not reach the Hub in time; the previous
// one was restored.
var ErrRolledBack = errors.New("new agent did not connect; previous one restored")

// Swap runs in the downloaded agent (update mode). It replaces the installed
// agent with itself and restores the previous copy if the new one does not
// reach the Hub within Wait.
type Swap struct {
	Self, Target, Backup  string
	QuitRunning           func()                                    // asks the installed agent to close
	Start                 func(exe string) (stop func(), err error) // opens a program without waiting
	Connected             func(ctx context.Context) bool            // installed agent reached the Hub with this version
	Copy                  func(src, dst string) error               // default replaceFile
	Wait, Poll, RetryWait time.Duration                             // defaults 2 min, 2 s, 1 s
	Tries                 int                                       // copy attempts; 0 = 30
}

func (s *Swap) Run(ctx context.Context) error {
	if s.Copy == nil {
		s.Copy = replaceFile
	}
	if s.Wait == 0 {
		s.Wait = 2 * time.Minute
	}
	if s.Poll == 0 {
		s.Poll = 2 * time.Second
	}
	if s.RetryWait == 0 {
		s.RetryWait = time.Second
	}
	if s.Tries == 0 {
		s.Tries = 30
	}
	s.QuitRunning()
	if err := s.copyRetrying(ctx, s.Self, s.Target); err != nil {
		// The old agent already quit: bring it back rather than leave none.
		// The target was never touched, so it is still the previous agent.
		berr := s.bringBack(true)
		return fmt.Errorf("install new agent: %w (restart: %w)", errors.Join(ErrRolledBack, err), berr)
	}
	stop, err := s.Start(s.Target)
	if err == nil && s.waitConnected(ctx) {
		return nil
	}
	slog.Warn("new agent did not connect; rolling back", "start_err", err)
	if stop != nil {
		stop()
	}
	cerr := s.copyRetrying(ctx, s.Backup, s.Target)
	if cerr != nil {
		slog.Error("restore previous agent", "err", cerr)
	}
	if berr := s.bringBack(cerr == nil); berr != nil {
		return fmt.Errorf("%w: no agent could be started: %w", ErrRolledBack, berr)
	}
	return ErrRolledBack
}

// bringBack starts some agent so the PC is never left with none: Target (when
// it holds the previous agent), then Backup, then Target as a last resort.
func (s *Swap) bringBack(targetIsOld bool) error {
	var errs []error
	if targetIsOld {
		_, err := s.Start(s.Target)
		if err == nil {
			return nil
		}
		slog.Warn("start installed agent failed; trying backup", "err", err)
		errs = append(errs, err)
	}
	_, err := s.Start(s.Backup)
	if err == nil {
		return nil
	}
	slog.Warn("start backup agent failed", "err", err)
	errs = append(errs, err)
	if !targetIsOld {
		slog.Warn("trying installed agent as last resort")
		_, err := s.Start(s.Target)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (s *Swap) copyRetrying(ctx context.Context, src, dst string) error {
	var err error
	for i := 0; i < s.Tries; i++ {
		if err = s.Copy(src, dst); err == nil {
			return nil
		}
		if i == s.Tries-1 {
			break
		}
		select {
		case <-ctx.Done():
			return errors.Join(err, ctx.Err())
		case <-time.After(s.RetryWait):
		}
	}
	return err
}

func (s *Swap) waitConnected(ctx context.Context) bool {
	deadline := time.Now().Add(s.Wait)
	for time.Now().Before(deadline) {
		if s.Connected(ctx) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(s.Poll):
		}
	}
	return false
}

// StatusConnected asks an agent's local /status whether it reached the Hub
// and runs version want.
func StatusConnected(ctx context.Context, url, want string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var st struct {
		Version   string `json:"version"`
		Connected bool   `json:"connected"`
	}
	if json.NewDecoder(resp.Body).Decode(&st) != nil {
		return false
	}
	return st.Connected && st.Version == want
}
