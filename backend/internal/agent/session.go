package agent

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State is what the Agente's icon shows (ADR 049).
type State int

const (
	StateConnecting   State = iota // pairing or saying hello for the first time
	StateConnected                 // talking to the Ziv
	StateOffline                   // no internet or Ziv down: retrying by itself
	StateDisconnected              // removed in the Ziv: waits for "Conectar de novo"
)

// Status holds the current State and tells subscribers when it changes.
type Status struct {
	mu    sync.Mutex
	state State
	subs  []func(State)
}

func (s *Status) Get() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Set changes the state; subscribers run only when it really changes.
func (s *Status) Set(st State) {
	s.mu.Lock()
	if s.state == st && st != StateConnecting {
		s.mu.Unlock()
		return
	}
	s.state = st
	subs := append([]func(State){}, s.subs...)
	s.mu.Unlock()
	for _, fn := range subs {
		fn(st)
	}
}

func (s *Status) Subscribe(fn func(State)) {
	s.mu.Lock()
	s.subs = append(s.subs, fn)
	s.mu.Unlock()
}

const disconnectedMark = "desconectado"

// MarkDisconnected records that this computer was removed in the Ziv, so a
// restart waits for the person instead of opening the browser on its own.
func MarkDisconnected(stateDir string) error {
	return os.WriteFile(filepath.Join(stateDir, disconnectedMark), nil, 0o600)
}

// ClearDisconnected drops the mark: running a fresh download is an explicit
// request to connect again.
func ClearDisconnected(stateDir string) {
	_ = os.Remove(filepath.Join(stateDir, disconnectedMark))
}

func IsMarkedDisconnected(stateDir string) bool {
	_, err := os.Stat(filepath.Join(stateDir, disconnectedMark))
	return err == nil
}

// Session keeps the Agente connected: pair when there is no key, say hello,
// run the job loop, and after a removal in the Ziv wait for Reconnect.
type Session struct {
	Client    *Client
	Name      string
	StateDir  string
	Open      func(url string) error
	Status    *Status
	Reconnect <-chan struct{}
	Run       func(ctx context.Context) error // the job loop; ErrUnauthorized when removed
	Retry     time.Duration
}

// Start returns only when ctx ends.
func (s *Session) Start(ctx context.Context) error {
	for ctx.Err() == nil {
		if IsMarkedDisconnected(s.StateDir) {
			s.Status.Set(StateDisconnected)
			if !s.waitReconnect(ctx) {
				break
			}
		}
		s.Status.Set(StateConnecting)
		s.Client.Token = LoadToken(s.StateDir)
		if s.Client.Token == "" {
			token, err := Pair(ctx, s.Client, s.Name, s.Open)
			if err != nil {
				if ctx.Err() != nil {
					break
				}
				// The pairing expired: wait for the person, do not reopen the browser.
				slog.Warn("pairing", "err", err)
				s.forget()
				continue
			}
			if err := SaveToken(s.StateDir, token); err != nil {
				slog.Error("save token", "err", err)
			}
			ClearDisconnected(s.StateDir)
			s.Client.Token = token
		}
		if !s.hello(ctx) {
			continue
		}
		s.Status.Set(StateConnected)
		ProtectToken(s.StateDir) // this version works here: no rollback needs the plain token
		err := s.Run(ctx)
		if errors.Is(err, ErrUnauthorized) {
			s.forget()
			continue
		}
		if ctx.Err() == nil {
			s.sleep(ctx)
		}
	}
	return ctx.Err()
}

// hello retries until the Ziv answers; false when the key was refused (or ctx ended).
func (s *Session) hello(ctx context.Context) bool {
	for {
		err := s.Client.Hello(ctx, s.Name)
		if err == nil {
			return true
		}
		if errors.Is(err, ErrUnauthorized) {
			s.forget()
			return false
		}
		if ctx.Err() != nil {
			return false
		}
		slog.Warn("hello", "err", err)
		s.Status.Set(StateOffline)
		if !s.sleep(ctx) {
			return false
		}
	}
}

func (s *Session) forget() {
	slog.Info("computer disconnected in the Ziv; waiting for Conectar de novo")
	_ = SaveToken(s.StateDir, "")
	if err := MarkDisconnected(s.StateDir); err != nil {
		slog.Warn("mark disconnected", "err", err)
	}
}

// waitReconnect drops clicks made before this point (while pairing or
// connected), then waits for a new one.
func (s *Session) waitReconnect(ctx context.Context) bool {
	for {
		select {
		case <-s.Reconnect:
			continue
		default:
		}
		break
	}
	select {
	case <-ctx.Done():
		return false
	case <-s.Reconnect:
		return true
	}
}

func (s *Session) sleep(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(s.Retry):
		return true
	}
}
