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
	Self, Target, Backup string
	QuitRunning          func()                                  // asks the installed agent to close
	Start                func(exe string) (stop func(), err error) // opens a program without waiting
	Connected            func(ctx context.Context) bool          // installed agent reached the Hub with this version
	Copy                 func(src, dst string) error             // default replaceFile
	Wait, Poll, RetryWait time.Duration                          // defaults 2 min, 2 s, 1 s
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
	s.QuitRunning()
	if err := s.copyRetrying(s.Self, s.Target); err != nil {
		// The old agent already quit: bring it back rather than leave none.
		if _, serr := s.Start(s.Target); serr != nil {
			slog.Error("restart previous agent", "err", serr)
		}
		return fmt.Errorf("install new agent: %w", err)
	}
	stop, err := s.Start(s.Target)
	if err == nil && s.waitConnected(ctx) {
		return nil
	}
	slog.Warn("new agent did not connect; rolling back", "start_err", err)
	if stop != nil {
		stop()
	}
	if err := s.copyRetrying(s.Backup, s.Target); err != nil {
		return fmt.Errorf("restore previous agent: %w", err)
	}
	if _, err := s.Start(s.Target); err != nil {
		return fmt.Errorf("start previous agent: %w", err)
	}
	return ErrRolledBack
}

func (s *Swap) copyRetrying(src, dst string) error {
	var err error
	for range 5 {
		if err = s.Copy(src, dst); err == nil {
			return nil
		}
		time.Sleep(s.RetryWait)
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
	var st struct {
		Version   string `json:"version"`
		Connected bool   `json:"connected"`
	}
	if json.NewDecoder(resp.Body).Decode(&st) != nil {
		return false
	}
	return st.Connected && st.Version == want
}
