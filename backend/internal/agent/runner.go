package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// Launcher opens an app and waits for it to close.
type Launcher interface {
	Launch(ctx context.Context, exe, dir string) (exitCode int, err error)
}

// ExecLauncher opens the program with dir as its working folder.
type ExecLauncher struct{}

func (ExecLauncher) Launch(ctx context.Context, exe, dir string) (int, error) {
	cmd := exec.Command(exe)
	cmd.Dir = dir
	err := runTree(ctx, cmd)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, err
}

// Runner executes run jobs; several apps may be open at once as long as they
// use separate folders.
type Runner struct {
	Launcher Launcher
	StateDir string
	mu       sync.Mutex
	open     map[int64]*openRun
	held     bool          // updating: no new run may start
	idle     chan struct{} // closed when the last open run ends
}

// openRun is a run in progress, so a stop job can reach it.
type openRun struct {
	id      int64
	name    string
	folder  string
	script  bool
	cancel  context.CancelFunc
	stopped atomic.Bool
	done    chan struct{}
}

// Stop closes the app opened for runID and returns a channel closed once the
// run has wrapped up. ok is false when that app is not open.
func (r *Runner) Stop(runID int64) (done <-chan struct{}, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o := r.open[runID]
	if o == nil {
		return nil, false
	}
	o.stopped.Store(true)
	o.cancel()
	return o.done, true
}

// wasStopped tells whether runID was closed by Stop.
func (r *Runner) wasStopped(runID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	o := r.open[runID]
	return o != nil && o.stopped.Load()
}

// Hold waits until no app is open and keeps new runs from starting until
// release is called (used while the agent updates itself). Apps may still
// open while it waits, so an update never blocks someone's work.
func (r *Runner) Hold(ctx context.Context) (release func(), err error) {
	for {
		r.mu.Lock()
		if len(r.open) == 0 {
			r.held = true
			r.mu.Unlock()
			return func() {
				r.mu.Lock()
				r.held = false
				r.mu.Unlock()
			}, nil
		}
		if r.idle == nil {
			r.idle = make(chan struct{})
		}
		idle := r.idle
		r.mu.Unlock()
		select {
		case <-idle:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// claim records o as open, unless an update is starting or another open app
// uses an overlapping folder (its new files would mix with this one's).
// Scripts write only into their own Execucao-N subfolder, so two scripts
// never clash with each other.
func (r *Runner) claim(o *openRun) agentproto.Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.held {
		return failed("O Agente está se atualizando. Tente de novo em um minuto.")
	}
	if r.open[o.id] != nil {
		return failed("Esta execução já está aberta neste computador.")
	}
	for _, other := range r.open {
		if (o.script && other.script) || !agentproto.FoldersOverlap(o.folder, other.folder) {
			continue
		}
		return failed("Essa pasta já está sendo usada pelo " + other.name + ", que está aberto agora. Escolha outra pasta ou espere ele fechar.")
	}
	if r.open == nil {
		r.open = map[int64]*openRun{}
	}
	r.open[o.id] = o
	return agentproto.Result{}
}

func (r *Runner) release(o *openRun) {
	r.mu.Lock()
	delete(r.open, o.id)
	if len(r.open) == 0 && r.idle != nil {
		close(r.idle)
		r.idle = nil
	}
	r.mu.Unlock()
	close(o.done)
}

func failed(msg string) agentproto.Result {
	return agentproto.Result{Event: agentproto.EventFailed, Error: msg}
}

// Run opens the app, waits for it to close and lists the new or changed files
// in the output folder. report receives the "started" event.
func (r *Runner) Run(ctx context.Context, p agentproto.RunPayload, report func(agentproto.Result)) agentproto.Result {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	name := p.AppName
	if name == "" {
		name = p.Exe
	}
	o := &openRun{id: p.RunID, name: name, folder: p.OutputFolder, script: p.Script != nil, cancel: cancel, done: make(chan struct{})}
	if res := r.claim(o); res.Event != "" {
		return res
	}
	defer r.release(o)
	res := r.run(ctx, p, report)
	if o.stopped.Load() && res.Event != agentproto.EventFinished {
		// Stopped before the app opened, or while it was getting ready.
		res = agentproto.Result{Event: agentproto.EventFinished}
	}
	res.Stopped = o.stopped.Load()
	return res
}

func (r *Runner) run(ctx context.Context, p agentproto.RunPayload, report func(agentproto.Result)) agentproto.Result {
	if p.Script != nil {
		return r.runScript(ctx, p, report)
	}
	exe := filepath.Join(p.AppFolder, p.Exe)
	info, err := os.Stat(exe)
	if err != nil {
		return failed("Não encontrei o arquivo " + p.Exe + " na pasta do app.")
	}
	if !safeAppPath(p.AppFolder, p.Exe) {
		return failed("O arquivo que inicia o app precisa ficar dentro da pasta do app.")
	}
	if err := checkWritable(p.OutputFolder); err != nil {
		return failed("Não consegui usar a pasta " + p.OutputFolder + ": ela não existe ou não aceita gravação.")
	}
	target, skip := exe, []string{}
	if !samePath(p.AppFolder, p.OutputFolder) {
		target = filepath.Join(p.OutputFolder, p.Exe)
		if _, err := os.Stat(target); err == nil {
			return failed("Já existe um " + p.Exe + " nessa pasta. Escolha outra pasta ou apague esse arquivo.")
		}
		skip = append(skip, filepath.ToSlash(p.Exe))
	}
	before, err := TakeSnapshot(p.OutputFolder, skip...)
	if err != nil {
		return failed("Não consegui ler a pasta " + p.OutputFolder + ".")
	}
	report(agentproto.Result{Event: agentproto.EventStarted,
		AppVersion: info.ModTime().Format("2006-01-02 15:04") + " · " + humanSize(info.Size())})

	if target != exe {
		if err := copyFile(exe, target); err != nil {
			// copyFile cleans up only what it created; never touch a file that was there.
			return failed("Não consegui copiar o app para a pasta escolhida.")
		}
		defer os.Remove(target)
	}
	code, err := r.Launcher.Launch(ctx, target, p.OutputFolder)
	if err != nil {
		return failed("Não consegui abrir o app: " + err.Error())
	}
	after, err := TakeSnapshot(p.OutputFolder, skip...)
	if err != nil {
		return failed("O app fechou, mas não consegui ler a pasta " + p.OutputFolder + ".")
	}
	files := Diff(before, after)
	if err := saveRecord(r.StateDir, p.RunID, p.OutputFolder, files); err != nil {
		return failed("Não consegui guardar a lista de arquivos desta execução.")
	}
	return agentproto.Result{Event: agentproto.EventFinished, ExitCode: code, Files: files}
}

func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".apphub-teste-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst) // we created it (O_EXCL), so it is ours to remove
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

func humanSize(n int64) string {
	mb := float64(n) / (1 << 20)
	if mb >= 1 {
		return strings.Replace(fmt.Sprintf("%.1f MB", mb), ".", ",", 1)
	}
	return fmt.Sprintf("%d KB", n>>10)
}
