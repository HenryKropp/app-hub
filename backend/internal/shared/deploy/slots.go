package deploy

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

// legacyMemoryMB is what a running box without MemoryLabel is counted as
// (boxes started before the label existed; the old default was 512 MB).
const legacyMemoryMB = 512

// Slots limits how many boxes run at once on the whole server, across every
// company: a count and a memory budget (MB). One instance is shared by every
// company's service. A nil *Slots means no limit.
type Slots struct {
	base       engine.Engine // unscoped: it sees every company's boxes
	maxRunning int
	budgetMB   int

	mu         sync.Mutex
	inflight   map[int]int // reservation id → MB, between Acquire and release
	seq        int
	refreshing bool // a rebuild by the Ziv itself is running somewhere
}

// TryRefresh takes the server's single refresh turn: one rebuild by the Ziv
// itself at a time, so the small server never builds two at once for that.
// ok is false while another refresh holds it; done gives it back. A nil
// *Slots always gives the turn.
func (s *Slots) TryRefresh() (done func(), ok bool) {
	if s == nil {
		return func() {}, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshing {
		return nil, false
	}
	s.refreshing = true
	var once sync.Once
	return func() {
		once.Do(func() { s.mu.Lock(); s.refreshing = false; s.mu.Unlock() })
	}, true
}

func NewSlots(base engine.Engine, maxRunning, budgetMB int) *Slots {
	return &Slots{base: base, maxRunning: maxRunning, budgetMB: budgetMB, inflight: map[int]int{}}
}

// Owner names the app whose own running box a new box replaces (update
// publish, rollback, restart): the company key and the app id.
type Owner struct {
	Workspace string
	AppID     int64
}

// Acquire reserves room for a box of memMB. A box that replaces the app's own
// running one (replacing non-nil) never waits for a count slot, and passes
// the memory budget when it does not grow: only the extra memory over the
// app's own box must fit. release drops the reservation: call it right after
// the box started or failed (from then on the box counts itself, through
// List).
func (s *Slots) Acquire(ctx context.Context, memMB int, replacing *Owner) (func(), error) {
	if s == nil {
		return func() {}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maxRunning > 0 || s.budgetMB > 0 {
		all, err := s.base.List(ctx)
		if err != nil {
			return nil, err
		}
		count, sum, own := len(s.inflight), 0, 0
		for _, mb := range s.inflight {
			sum += mb
		}
		for _, c := range all {
			if !c.Running {
				continue
			}
			mb, err := strconv.Atoi(c.Labels[engine.MemoryLabel])
			if err != nil || mb <= 0 {
				mb = legacyMemoryMB
			}
			if replacing != nil && engine.WorkspaceOf(c.Labels) == replacing.Workspace && c.Labels[engine.Label] == strconv.FormatInt(replacing.AppID, 10) {
				own = max(own, mb)
				continue
			}
			count++
			sum += mb
		}
		if replacing == nil && s.maxRunning > 0 && count+1 > s.maxRunning {
			return nil, &LimitError{Msg: fmt.Sprintf("O servidor está no limite de apps com tela ligados ao mesmo tempo (%d). Tente de novo em alguns minutos: apps sem uso dormem sozinhos depois de 30 minutos.", s.maxRunning)}
		}
		if s.budgetMB > 0 && sum+memMB > s.budgetMB && (replacing == nil || memMB > own) {
			return nil, &LimitError{Msg: fmt.Sprintf("O servidor está sem memória livre para ligar este app agora (ele usa %d MB). Tente de novo em alguns minutos: apps sem uso dormem sozinhos depois de 30 minutos.", memMB)}
		}
	}
	s.seq++
	id := s.seq
	s.inflight[id] = memMB
	var once sync.Once
	return func() {
		once.Do(func() { s.mu.Lock(); delete(s.inflight, id); s.mu.Unlock() })
	}, nil
}
