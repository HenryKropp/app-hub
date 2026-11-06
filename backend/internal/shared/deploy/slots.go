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

	mu       sync.Mutex
	inflight map[int]int // reservation id → MB, between Acquire and release
	seq      int
}

func NewSlots(base engine.Engine, maxRunning, budgetMB int) *Slots {
	return &Slots{base: base, maxRunning: maxRunning, budgetMB: budgetMB, inflight: map[int]int{}}
}

// Acquire reserves room for a box of memMB. replacing (a box that replaces the
// app's own running one) is always allowed. release drops the reservation:
// call it right after the box started or failed (from then on the box counts
// itself, through List).
func (s *Slots) Acquire(ctx context.Context, memMB int, replacing bool) (func(), error) {
	if s == nil {
		return func() {}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !replacing && (s.maxRunning > 0 || s.budgetMB > 0) {
		all, err := s.base.List(ctx)
		if err != nil {
			return nil, err
		}
		count, sum := len(s.inflight), 0
		for _, mb := range s.inflight {
			sum += mb
		}
		for _, c := range all {
			if !c.Running {
				continue
			}
			count++
			mb, err := strconv.Atoi(c.Labels[engine.MemoryLabel])
			if err != nil || mb <= 0 {
				mb = legacyMemoryMB
			}
			sum += mb
		}
		if (s.maxRunning > 0 && count+1 > s.maxRunning) || (s.budgetMB > 0 && sum+memMB > s.budgetMB) {
			return nil, &LimitError{Msg: fmt.Sprintf("O servidor está no limite de apps com tela ligados ao mesmo tempo (%d). Tente de novo em alguns minutos: apps sem uso dormem sozinhos depois de 30 minutos.", s.maxRunning)}
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
