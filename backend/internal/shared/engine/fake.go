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
	"sync/atomic"
)

// Fake is an in-memory Engine for tests. Each started container is a real
// httptest server. It never touches real containers.
//
// The exported fields may be written directly only before the Fake is used
// concurrently. Afterwards use SetDown, SetHandler and CallsCopy.
type Fake struct {
	mu  sync.Mutex
	seq int
	sync.Mutex
	Images     map[string]BuildSpec
	Containers map[string]*FakeContainer
	Networks   map[string]bool // redes criadas por Start (e não removidas)
	BuildErr   error           // próxima montagem falha com isto (e volta a nil)
	BuildLog   string
	Handler    http.Handler                   // o que as caixas respondem; padrão: 200 "ok <nome>" + eco dos X-AppHub-*
	HandlerFor func(name string) http.Handler // se devolve não-nil, vale para a caixa com esse nome (antes de Handler)
	Down       bool                           // Available devolve ErrUnavailable
	Calls      []string                       // "build <tag>", "start <name>", "stop <id>", "remove <id>", "exec <container> <cmd>"
	ExecFn     func(container string, stdin io.Reader, stdout io.Writer, cmd []string) error
	logs       string // SetLogs; "" = two plain lines
}

// SetLogs sets what Logs answers.
func (f *Fake) SetLogs(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = text
}

type FakeContainer struct {
	Spec    StartSpec
	Running bool
	Exit    int
	srv     *httptest.Server
	swap    *swapHandler
}

// swapHandler lets a running server change its handler safely.
type swapHandler struct{ v atomic.Value }

type handlerBox struct{ h http.Handler }

func (s *swapHandler) set(h http.Handler) { s.v.Store(handlerBox{h}) }

func (s *swapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.v.Load().(handlerBox).h.ServeHTTP(w, r)
}

var _ Engine = (*Fake)(nil)

func NewFake() *Fake {
	return &Fake{Images: map[string]BuildSpec{}, Containers: map[string]*FakeContainer{}, Networks: map[string]bool{}}
}

// SetDown sets Down under the lock.
func (f *Fake) SetDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Down = down
}

// CallsCopy returns a copy of Calls.
func (f *Fake) CallsCopy() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.Calls...)
}

// Close shuts down every running container server.
func (f *Fake) Close() {
	f.mu.Lock()
	var srvs []*httptest.Server
	for id := range f.Containers {
		if srv := f.stopLocked(id, 0); srv != nil {
			srvs = append(srvs, srv)
		}
	}
	f.mu.Unlock()
	for _, srv := range srvs {
		srv.Close()
	}
}

func (f *Fake) record(s string) { f.Calls = append(f.Calls, s) }

func (f *Fake) Available(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
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
	f.seq++
	id := "img-" + strconv.Itoa(f.seq)
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
	if s.Network != "" {
		f.Networks[s.Network] = true
	}
	h := f.Handler
	if f.HandlerFor != nil {
		if hn := f.HandlerFor(s.Name); hn != nil {
			h = hn
		}
	}
	if h == nil {
		h = f.defaultHandler(s.Name)
	}
	f.seq++
	id := "ctr-" + strconv.Itoa(f.seq) + "-" + s.Name
	sw := &swapHandler{}
	sw.set(h)
	f.Containers[id] = &FakeContainer{Spec: s, Running: true, srv: httptest.NewServer(sw), swap: sw}
	return id, nil
}

// stopLocked marks the container stopped and returns its server, which the
// caller must Close after releasing the lock (nil if nothing to close).
func (f *Fake) stopLocked(id string, exit int) *httptest.Server {
	if c := f.Containers[id]; c != nil && c.Running {
		c.Running, c.Exit = false, exit
		return c.srv
	}
	return nil
}

func closeSrv(srv *httptest.Server) {
	if srv != nil {
		srv.Close()
	}
}

func (f *Fake) Stop(_ context.Context, id string) error {
	f.mu.Lock()
	f.record("stop " + id)
	if f.Containers[id] == nil {
		f.mu.Unlock()
		return ErrNotFound
	}
	srv := f.stopLocked(id, 0)
	f.mu.Unlock()
	closeSrv(srv)
	return nil
}

func (f *Fake) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	f.record("remove " + id)
	srv := f.stopLocked(id, 0)
	delete(f.Containers, id)
	f.mu.Unlock()
	closeSrv(srv)
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
	srv := f.stopLocked(id, 1)
	f.mu.Unlock()
	closeSrv(srv)
}

func (f *Fake) SetHandler(id string, h http.Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c := f.Containers[id]; c != nil {
		c.swap.set(h)
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
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logs != "" {
		return f.logs, nil
	}
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
		if _, ok := AppLabel(c.Spec.Labels); ok {
			out = append(out, Container{ID: id, Name: c.Spec.Name, Running: c.Running, Labels: c.Spec.Labels})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *Fake) RemoveNetwork(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("remove-network " + name)
	if !HubNetwork(name) {
		return fmt.Errorf("recusado: a rede %q não é do Kova", name)
	}
	delete(f.Networks, name)
	return nil
}

// HasNetwork reports whether the network exists (under the lock).
func (f *Fake) HasNetwork(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Networks[name]
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
