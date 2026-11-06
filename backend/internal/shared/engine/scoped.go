package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrOtherWorkspace refuses an action on a box or network of another company.
var ErrOtherWorkspace = errors.New("caixa de outra empresa")

// Scoped wraps an engine so one company only sees and touches its own boxes:
// those whose WorkspaceLabel equals key (principal, key "", owns the boxes with
// no such label, including the legacy Kova and App Hub ones).
func Scoped(base Engine, key string) Engine { return &scoped{base: base, key: key} }

type scoped struct {
	base Engine
	key  string
}

func (s *scoped) withKey(labels map[string]string) map[string]string {
	if s.key == "" {
		return labels
	}
	out := make(map[string]string, len(labels)+1)
	for k, v := range labels {
		out[k] = v
	}
	out[WorkspaceLabel] = s.key
	return out
}

// ownsNetwork: principal owns ziv-apps-<id> and the legacy names; a company
// owns only ziv-apps-<key>-<id>.
func (s *scoped) ownsNetwork(name string) bool {
	if !HubNetwork(name) {
		return false
	}
	rest, ok := strings.CutPrefix(name, NetworkPrefix)
	company := ok && strings.HasPrefix(rest, "w")
	if s.key == "" {
		return !company
	}
	return company && strings.HasPrefix(rest, s.key+"-")
}

func sameID(a, b string) bool {
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}
	return len(short) >= 12 && strings.HasPrefix(long, short) || a == b
}

// check refuses an id that belongs to another company's box.
func (s *scoped) check(ctx context.Context, id string) error {
	all, err := s.base.List(ctx)
	if err != nil {
		return err
	}
	for _, c := range all {
		if sameID(c.ID, id) && WorkspaceOf(c.Labels) != s.key {
			return ErrOtherWorkspace
		}
	}
	return nil
}

func (s *scoped) Available(ctx context.Context) error { return s.base.Available(ctx) }

func (s *scoped) Build(ctx context.Context, b BuildSpec) (string, string, error) {
	b.Labels = s.withKey(b.Labels)
	return s.base.Build(ctx, b)
}

func (s *scoped) Start(ctx context.Context, sp StartSpec) (string, error) {
	if !s.ownsNetwork(sp.Network) {
		return "", fmt.Errorf("%w: rede %q", ErrOtherWorkspace, sp.Network)
	}
	sp.Labels = s.withKey(sp.Labels)
	return s.base.Start(ctx, sp)
}

func (s *scoped) Stop(ctx context.Context, id string) error {
	if err := s.check(ctx, id); err != nil {
		return err
	}
	return s.base.Stop(ctx, id)
}

func (s *scoped) Remove(ctx context.Context, id string) error {
	if err := s.check(ctx, id); err != nil {
		return err
	}
	return s.base.Remove(ctx, id)
}

func (s *scoped) Logs(ctx context.Context, id string, lines int) (string, error) {
	if err := s.check(ctx, id); err != nil {
		return "", err
	}
	return s.base.Logs(ctx, id, lines)
}

func (s *scoped) RemoveImage(ctx context.Context, id string) error { return s.base.RemoveImage(ctx, id) }
func (s *scoped) Inspect(ctx context.Context, id string) (State, error) {
	return s.base.Inspect(ctx, id)
}
func (s *scoped) Stats(ctx context.Context, ids []string) ([]Usage, error) {
	return s.base.Stats(ctx, ids)
}
func (s *scoped) Exec(ctx context.Context, c string, in io.Reader, out io.Writer, cmd ...string) error {
	return s.base.Exec(ctx, c, in, out, cmd...)
}

func (s *scoped) List(ctx context.Context) ([]Container, error) {
	all, err := s.base.List(ctx)
	if err != nil {
		return nil, err
	}
	out := all[:0:0]
	for _, c := range all {
		if WorkspaceOf(c.Labels) == s.key {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *scoped) RemoveNetwork(ctx context.Context, name string) error {
	if !s.ownsNetwork(name) {
		return ErrOtherWorkspace
	}
	return s.base.RemoveNetwork(ctx, name)
}
