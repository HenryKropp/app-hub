package supervisor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

// Holder is what the group holds while a company is reconciled (the
// company's gateway: it answers "starting" until released).
type Holder interface {
	Hold()
	Release()
}

type member struct {
	s *Supervisor
	h Holder
}

// Group runs every company's supervisor from one goroutine, one company after
// another, so the small server never does two companies' work at once.
type Group struct {
	Interval time.Duration

	eng  engine.Engine // the unscoped engine, only to know whether it is up
	wake chan struct{}
	down bool

	mu      sync.Mutex
	queue   []member
	members []*Supervisor
}

func NewGroup(eng engine.Engine) *Group {
	return &Group{Interval: 30 * time.Second, eng: eng, wake: make(chan struct{}, 1)}
}

// Add holds h at once and queues the company for reconcile; Loop releases h
// once that is done. Safe from any goroutine, also while Loop runs.
func (g *Group) Add(s *Supervisor, h Holder) {
	if h != nil {
		h.Hold()
	}
	g.mu.Lock()
	g.queue = append(g.queue, member{s: s, h: h})
	g.mu.Unlock()
	select {
	case g.wake <- struct{}{}:
	default:
	}
}

// Loop reconciles queued companies and ticks every Interval until ctx ends.
func (g *Group) Loop(ctx context.Context) {
	g.reconcileQueued(ctx)
	t := time.NewTicker(g.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-g.wake:
			g.reconcileQueued(ctx)
		case <-t.C:
			g.reconcileQueued(ctx)
			g.tick(ctx)
		}
	}
}

// reconcileQueued reconciles the companies added since the last call, in
// order, and releases each one's holder afterwards (also on failure).
func (g *Group) reconcileQueued(ctx context.Context) {
	g.mu.Lock()
	queue := g.queue
	g.queue = nil
	g.mu.Unlock()
	for _, m := range queue {
		if m.s.HasApps(ctx) {
			if err := m.s.Reconcile(ctx); err != nil {
				slog.Warn("supervisor reconcile", "err", err)
			}
		}
		if m.h != nil {
			m.h.Release()
		}
		g.mu.Lock()
		g.members = append(g.members, m.s)
		g.mu.Unlock()
	}
}

// tick checks the engine once, then runs one round for each company that has
// shared apps.
func (g *Group) tick(ctx context.Context) {
	if err := g.eng.Available(ctx); err != nil {
		if !g.down {
			slog.Warn("supervisor: container engine unavailable", "err", err)
		}
		g.down = true
		return
	}
	g.down = false
	g.mu.Lock()
	members := append([]*Supervisor(nil), g.members...)
	g.mu.Unlock()
	for _, s := range members {
		if ctx.Err() != nil {
			return
		}
		if s.HasApps(ctx) {
			s.Step(ctx)
		}
	}
}
