package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Podman struct {
	bin       string
	execAllow map[string]bool
}

// Option configures NewPodman.
type Option func(*Podman)

// WithExecAllowed lets Exec (and only Exec) run in the named containers
// without the Hub label, by exact name. It exists for the Hub's own Postgres
// container (pg_dump/pg_restore). Stop, Remove, RemoveImage and Logs stay
// fully guarded even for these names.
func WithExecAllowed(names ...string) Option {
	return func(p *Podman) {
		for _, n := range names {
			if n != "" {
				p.execAllow[n] = true
			}
		}
	}
}

func NewPodman(bin string, opts ...Option) *Podman {
	if bin == "" {
		bin = "podman"
	}
	p := &Podman{bin: bin, execAllow: map[string]bool{}}
	for _, o := range opts {
		o(p)
	}
	return p
}

const shortTimeout = 30 * time.Second
const buildTimeout = 15 * time.Minute

func (p *Podman) run(ctx context.Context, timeout time.Duration, stdin io.Reader, args ...string) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, p.bin, args...)
	cmd.Stdin = stdin
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err != nil {
		msg := errb.String()
		low := strings.ToLower(msg)
		if errors.Is(err, exec.ErrNotFound) || strings.Contains(low, "cannot connect") ||
			strings.Contains(low, "unable to connect") || strings.Contains(low, "connection refused") {
			slog.Warn("podman", "args", args[0], "err", err, "stderr", tail(msg, 2000))
			return nil, ErrUnavailable
		}
		if strings.Contains(low, "no such container") || strings.Contains(low, "no such object") || strings.Contains(low, "no such image") {
			slog.Debug("podman: not found", "args", args[0], "stderr", tail(msg, 500))
			return nil, ErrNotFound
		}
		slog.Warn("podman", "args", args[0], "err", err, "stderr", tail(msg, 2000))
		return out.Bytes(), fmt.Errorf("%s: %w: %s", args[0], err, tail(msg, 500))
	}
	return out.Bytes(), nil
}

// hubOwned decides whether a container label value marks a Hub container.
func hubOwned(label string) bool { return strings.TrimSpace(label) != "" }

// imageAllowed decides whether an image may be removed: it carries the Hub
// label or one of its tags lives under localhost/apphub/.
func imageAllowed(label string, repoTags []string) bool {
	if hubOwned(label) {
		return true
	}
	for _, t := range repoTags {
		if strings.HasPrefix(t, "localhost/apphub/") {
			return true
		}
	}
	return false
}

var errNotHub = errors.New("recusado: esta caixa não é do Hub")

// guard refuses to act on a container without the Hub label.
func (p *Podman) guard(ctx context.Context, id string) error {
	out, err := p.run(ctx, shortTimeout, nil, "container", "inspect", "--format", `{{index .Config.Labels "`+Label+`"}}`, "--", id)
	if err != nil {
		return err
	}
	if !hubOwned(string(out)) {
		slog.Error("podman: refused to touch a container without the Hub label", "id", id)
		return errNotHub
	}
	return nil
}

// unavailable reports whether a failed command was really the engine being down.
func (p *Podman) unavailable(ctx context.Context, err error) bool {
	// fresh context: the caller's may already be expired (e.g. build timeout)
	pctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Is(err, exec.ErrNotFound) || p.Available(pctx) != nil
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func (p *Podman) Available(ctx context.Context) error {
	_, err := p.run(ctx, 5*time.Second, nil, "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// exists runs a `podman ... exists` command: exit 0 = yes, exit 1 = no,
// anything else is an error. Nothing is logged above Debug.
func (p *Podman) exists(ctx context.Context, args ...string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, shortTimeout)
	defer cancel()
	err := exec.CommandContext(ctx, p.bin, args...).Run()
	if err == nil {
		return true, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		slog.Debug("podman: does not exist", "args", args)
		return false, nil
	}
	if errors.Is(err, exec.ErrNotFound) || p.unavailable(ctx, err) {
		return false, ErrUnavailable
	}
	return false, err
}

func (p *Podman) ensureNetwork(ctx context.Context) error {
	ok, err := p.exists(ctx, "network", "exists", Network)
	if err != nil || ok {
		return err
	}
	_, err = p.run(ctx, shortTimeout, nil, "network", "create", Network)
	return err
}

func labelArgs(l map[string]string) []string {
	var out []string
	for k, v := range l {
		out = append(out, "--label", k+"="+v)
	}
	return out
}

func (p *Podman) Build(ctx context.Context, b BuildSpec) (string, string, error) {
	df := b.DockerfilePath
	if df == "" {
		df = "Dockerfile"
	}
	iid := filepath.Join(os.TempDir(), "apphub-iid-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	defer os.Remove(iid)
	args := append([]string{"build", "--iidfile", iid, "-t", b.Tag, "-f", filepath.Join(b.ContextDir, df)}, labelArgs(b.Labels)...)
	args = append(args, b.ContextDir)
	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, args...)
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	err := cmd.Run()
	text := tail(log.String(), 64<<10)
	if err != nil {
		if p.unavailable(ctx, err) {
			return "", text, ErrUnavailable
		}
		slog.Warn("podman build", "err", err, "log", tail(text, 2000))
		return "", text, err
	}
	id, err := os.ReadFile(iid)
	return strings.TrimSpace(string(id)), text, err
}

func (p *Podman) Start(ctx context.Context, s StartSpec) (string, error) {
	if err := p.ensureNetwork(ctx); err != nil {
		return "", err
	}
	if taken, err := p.exists(ctx, "container", "exists", "--", s.Name); err != nil {
		return "", err
	} else if taken {
		return "", fmt.Errorf("já existe uma caixa com o nome %q", s.Name)
	}
	for k, v := range s.Env {
		if !envKeyRe.MatchString(k) || strings.ContainsAny(v, "\r\n\x00") {
			return "", fmt.Errorf("variável %q tem quebra de linha ou nome inválido", k)
		}
	}
	envFile, err := os.CreateTemp("", "apphub-env-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(envFile.Name())
	for k, v := range s.Env {
		fmt.Fprintf(envFile, "%s=%s\n", k, v)
	}
	if err := envFile.Close(); err != nil {
		return "", err
	}
	args := []string{"run", "-d", "--name", s.Name, "--network", Network,
		"--add-host", "host.containers.internal:host-gateway",
		"-p", fmt.Sprintf("127.0.0.1::%d", s.Port),
		"--memory", fmt.Sprintf("%dm", s.MemoryMB), "--cpus", strconv.FormatFloat(s.CPUs, 'f', 2, 64),
		"--env-file", envFile.Name()}
	if s.DataDir != "" {
		if err := os.MkdirAll(s.DataDir, 0o700); err != nil {
			return "", err
		}
		args = append(args, "-v", s.DataDir+":/data:Z")
	}
	args = append(args, labelArgs(s.Labels)...)
	args = append(args, s.Image)
	out, err := p.run(ctx, shortTimeout, nil, args...)
	if err != nil {
		if !errors.Is(err, ErrUnavailable) {
			_ = p.Remove(ctx, s.Name) // best effort; the name was free before run, so it is ours
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *Podman) Stop(ctx context.Context, id string) error {
	if err := p.guard(ctx, id); err != nil {
		return err
	}
	_, err := p.run(ctx, shortTimeout, nil, "stop", "-t", "10", "--", id)
	return err
}

func (p *Podman) Remove(ctx context.Context, id string) error {
	if err := p.guard(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	_, err := p.run(ctx, shortTimeout, nil, "rm", "-f", "--ignore", "--", id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (p *Podman) RemoveImage(ctx context.Context, id string) error {
	out, err := p.run(ctx, shortTimeout, nil, "image", "inspect", "--format",
		`{{index .Labels "`+Label+`"}}|{{join .RepoTags ","}}`, "--", id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	label, tags, _ := strings.Cut(strings.TrimSpace(string(out)), "|")
	var repoTags []string
	if tags != "" {
		repoTags = strings.Split(tags, ",")
	}
	if !imageAllowed(label, repoTags) {
		slog.Error("podman: refused to remove an image that is not the Hub's", "id", id)
		return errors.New("recusado: esta imagem não é do Hub")
	}
	_, err = p.run(ctx, shortTimeout, nil, "rmi", "--ignore", "--", id)
	return err
}

func (p *Podman) Inspect(ctx context.Context, id string) (State, error) {
	out, err := p.run(ctx, shortTimeout, nil, "inspect", "--type", "container", "--", id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return State{}, nil
		}
		return State{}, err
	}
	return parseInspect(out, 0)
}

// parseInspect reads `podman inspect`. port 0 = the first published port.
func parseInspect(raw []byte, port int) (State, error) {
	var v []struct {
		State struct {
			Running  bool
			ExitCode int
		}
		NetworkSettings struct {
			Ports map[string][]struct{ HostPort string }
		}
	}
	if err := json.Unmarshal(raw, &v); err != nil || len(v) == 0 {
		return State{}, err
	}
	st := State{Exists: true, Running: v[0].State.Running, ExitCode: v[0].State.ExitCode}
	for k, binds := range v[0].NetworkSettings.Ports {
		if (port == 0 || k == fmt.Sprintf("%d/tcp", port)) && len(binds) > 0 {
			st.HostPort, _ = strconv.Atoi(binds[0].HostPort)
			break
		}
	}
	return st, nil
}

func (p *Podman) Stats(ctx context.Context, ids []string) ([]Usage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// All running containers; podman fails the whole call on a stopped id.
	out, err := p.run(ctx, shortTimeout, nil, "stats", "--no-stream", "--format", "json")
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	all, err := parseStats(out)
	if err != nil {
		return nil, err
	}
	return normalizeStats(all, ids), nil
}

// normalizeStats keeps only the requested ids. Podman reports short ids; the
// result carries the full requested id.
func normalizeStats(all []Usage, ids []string) []Usage {
	var res []Usage
	for _, id := range ids {
		if id == "" {
			continue
		}
		for _, u := range all {
			if u.ContainerID != "" && (strings.HasPrefix(id, u.ContainerID) || strings.HasPrefix(u.ContainerID, id)) {
				u.ContainerID = id
				res = append(res, u)
				break
			}
		}
	}
	return res
}

func parseStats(raw []byte) ([]Usage, error) {
	var v []struct {
		ID       string `json:"id"`
		CPU      string `json:"cpu_percent"`
		MemUsage string `json:"mem_usage"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	out := make([]Usage, 0, len(v))
	for _, s := range v {
		mem, _ := ParseSize(strings.TrimSpace(strings.SplitN(s.MemUsage, "/", 2)[0]))
		cpu, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s.CPU), "%"), 64)
		out = append(out, Usage{ContainerID: s.ID, MemBytes: mem, CPUPct: cpu})
	}
	return out, nil
}

var sizeRe = regexp.MustCompile(`^([0-9.]+)\s*([kKMGT]?i?B)$`)

// ParseSize reads sizes as Podman prints them: decimal (kB, MB) or binary (KiB, MiB).
func ParseSize(s string) (int64, error) {
	m := sizeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("tamanho inválido %q", s)
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}
	mult := map[string]float64{"B": 1, "kB": 1e3, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12,
		"KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "TiB": 1 << 40}[m[2]]
	if mult == 0 {
		return 0, fmt.Errorf("unidade inválida %q", s)
	}
	return int64(n*mult + 0.5), nil
}

func (p *Podman) Logs(ctx context.Context, id string, lines int) (string, error) {
	if err := p.guard(ctx, id); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, shortTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, "logs", "--tail", strconv.Itoa(lines), "--", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("podman logs", "err", err, "out", tail(string(out), 500))
		if p.unavailable(ctx, err) {
			return "", ErrUnavailable
		}
	}
	return string(out), err
}

func (p *Podman) Exec(ctx context.Context, container string, stdin io.Reader, stdout io.Writer, cmd ...string) error {
	if len(cmd) == 0 {
		return errors.New("exec: comando vazio")
	}
	if !p.execAllow[container] {
		if err := p.guard(ctx, container); err != nil {
			return err
		}
	}
	args := append([]string{"exec", "-i", container}, cmd...)
	c := exec.CommandContext(ctx, p.bin, args...)
	c.Stdin, c.Stdout = stdin, stdout
	var errb bytes.Buffer
	c.Stderr = &errb
	if err := c.Run(); err != nil {
		slog.Warn("podman exec", "err", err, "stderr", tail(errb.String(), 2000))
		if p.unavailable(ctx, err) {
			return ErrUnavailable
		}
		return fmt.Errorf("exec %s: %w: %s", cmd[0], err, tail(errb.String(), 500))
	}
	return nil
}

func (p *Podman) List(ctx context.Context) ([]Container, error) {
	out, err := p.run(ctx, shortTimeout, nil, "ps", "-a", "--filter", "label="+Label, "--format", "json")
	if err != nil {
		return nil, err
	}
	return parsePS(out)
}

func parsePS(raw []byte) ([]Container, error) {
	var v []struct {
		ID     string `json:"Id"`
		Names  []string
		State  string
		Labels map[string]string
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(v))
	for _, c := range v {
		if _, ok := c.Labels[Label]; !ok {
			continue // nunca mexer no que não é do Hub
		}
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		out = append(out, Container{ID: c.ID, Name: name, Running: c.State == "running", Labels: c.Labels})
	}
	return out, nil
}

var _ Engine = (*Podman)(nil)
var _ Engine = (*Fake)(nil)
