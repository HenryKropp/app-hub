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
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, err
}

// Runner executes run jobs, one at a time.
type Runner struct {
	Launcher Launcher
	StateDir string
	mu       sync.Mutex
	updating atomic.Bool
}

// Hold waits until no app is open and keeps new runs from starting until
// release is called (used while the agent updates itself).
func (r *Runner) Hold() (release func()) {
	r.mu.Lock()
	r.updating.Store(true)
	return func() {
		r.updating.Store(false)
		r.mu.Unlock()
	}
}

func failed(msg string) agentproto.Result {
	return agentproto.Result{Event: agentproto.EventFailed, Error: msg}
}

// Run opens the app, waits for it to close and lists the new or changed files
// in the output folder. report receives the "started" event.
func (r *Runner) Run(ctx context.Context, p agentproto.RunPayload, report func(agentproto.Result)) agentproto.Result {
	if !r.mu.TryLock() {
		if r.updating.Load() {
			return failed("O Agente está se atualizando. Tente de novo em um minuto.")
		}
		return failed("Já tem um app aberto pelo Hub neste computador. Feche-o e tente de novo.")
	}
	defer r.mu.Unlock()

	exe := filepath.Join(p.AppFolder, p.Exe)
	info, err := os.Stat(exe)
	if err != nil {
		return failed("Não encontrei o arquivo " + p.Exe + " na pasta do app.")
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
