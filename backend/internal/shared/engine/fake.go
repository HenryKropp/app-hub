package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Fake is an in-memory Engine for tests. Each started container is a real
// httptest server. It never touches real containers.
type Fake struct {
	mu         sync.Mutex
	Images     map[string]BuildSpec
	Containers map[string]*FakeContainer
	BuildErr   error // próxima montagem falha com isto (e volta a nil)
	BuildLog   string
	Handler    http.Handler // o que as caixas respondem; padrão: 200 "ok <nome>" + eco dos X-AppHub-*
	Down       bool         // Available devolve ErrUnavailable
	Calls      []string     // "build <tag>", "start <name>", "stop <id>", "remove <id>", "exec <container> <cmd>"
	ExecFn     func(container string, stdin io.Reader, stdout io.Writer, cmd []string) error
}

type FakeContainer struct {
	Spec    StartSpec
	Running bool
	Exit    int
	srv     *httptest.Server
}

var _ Engine = (*Fake)(nil)

func NewFake() *Fake {
	return &Fake{Images: map[string]BuildSpec{}, Containers: map[string]*FakeContainer{}}
}

func (f *Fake) record(s string) { f.Calls = append(f.Calls, s) }

func (f *Fake) Available(context.Context) error {
	if f.Down {
		return ErrUnavailable
	}
	return nil
}

func (f *Fake) Build(_ context.Context, b BuildSpec) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("build " + b.Tag)
	if err := f.BuildErr; err != nil {
		f.BuildErr = nil
		return "", f.BuildLog + "\n" + err.Error(), err
	}
	id := "img-" + strconv.Itoa(len(f.Images)+1)
	f.Images[id] = b
	return id, f.BuildLog, nil
}

func (f *Fake) defaultHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Apphub-") {
				w.Header().Set("Echo-"+k, strings.Join(v, ","))
			}
		}
		fmt.Fprint(w, "ok "+name)
	})
}

func (f *Fake) Start(_ context.Context, s StartSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("start " + s.Name)
	h := f.Handler
	if h == nil {
		h = f.defaultHandler(s.Name)
	}
	id := "ctr-" + strconv.Itoa(len(f.Containers)+1) + "-" + s.Name
	f.Containers[id] = &FakeContainer{Spec: s, Running: true, srv: httptest.NewServer(h)}
	return id, nil
}

func (f *Fake) stopLocked(id string, exit int) {
	if c := f.Containers[id]; c != nil && c.Running {
		c.Running, c.Exit = false, exit
		c.srv.Close()
	}
}

func (f *Fake) Stop(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("stop " + id)
	if f.Containers[id] == nil {
		return ErrNotFound
	}
	f.stopLocked(id, 0)
	return nil
}

func (f *Fake) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("remove " + id)
	f.stopLocked(id, 0)
	delete(f.Containers, id)
	return nil
}

func (f *Fake) RemoveImage(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Images, id)
	return nil
}

func (f *Fake) Crash(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopLocked(id, 1)
}

func (f *Fake) SetHandler(id string, h http.Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c := f.Containers[id]; c != nil && c.Running {
		c.srv.Config.Handler = h
	}
}

func (f *Fake) Inspect(_ context.Context, id string) (State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.Containers[id]
	if c == nil {
		return State{}, nil
	}
	st := State{Exists: true, Running: c.Running, ExitCode: c.Exit}
	if c.Running {
		u, _ := url.Parse(c.srv.URL)
		st.HostPort, _ = strconv.Atoi(u.Port())
	}
	return st, nil
}

func (f *Fake) Stats(_ context.Context, ids []string) ([]Usage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Usage
	for _, id := range ids {
		if c := f.Containers[id]; c != nil && c.Running {
			out = append(out, Usage{ContainerID: id, MemBytes: int64(c.Spec.MemoryMB) << 19, CPUPct: 2.5}) // metade do limite
		}
	}
	return out, nil
}

func (f *Fake) Logs(_ context.Context, id string, lines int) (string, error) {
	return "linha 1\nlinha 2\n", nil
}

func (f *Fake) Exec(_ context.Context, container string, stdin io.Reader, stdout io.Writer, cmd ...string) error {
	f.mu.Lock()
	f.record("exec " + container + " " + strings.Join(cmd, " "))
	fn := f.ExecFn
	f.mu.Unlock()
	if fn != nil {
		return fn(container, stdin, stdout, cmd)
	}
	return nil
}

func (f *Fake) List(context.Context) ([]Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Container
	for id, c := range f.Containers {
		if _, ok := c.Spec.Labels[Label]; ok {
			out = append(out, Container{ID: id, Name: c.Spec.Name, Running: c.Running, Labels: c.Spec.Labels})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *Fake) Running() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.Containers {
		if c.Running {
			out = append(out, c.Spec.Name)
		}
	}
	sort.Strings(out)
	return out
}
