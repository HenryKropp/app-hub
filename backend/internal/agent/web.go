package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// Screen apps on the computer (0.3.5+): the Agente writes the code the Ziv
// keeps into a folder of its own per app, prepares the libraries once, starts
// the app on a free local port with the app's vault in its environment, and
// opens the browser. The app runs until it is stopped (Parar) or closes.
// The app's data goes to the folder the person chose (APPHUB_DATA_DIR), as on
// the 24h version (/data).

const (
	webDepsTimeout  = 10 * time.Minute
	webReadyTimeout = 3 * time.Minute
)

// webLogName is the app's console, written in the chosen folder when it
// closes on its own, so the person can see why. It never goes to the Ziv:
// an app may print its own keys.
func webLogName(runID int64) string { return fmt.Sprintf("ziv-app-%d.txt", runID) }

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitPort waits until something answers on the port, or ctx ends.
func waitPort(ctx context.Context, port int) bool {
	addr := "127.0.0.1:" + strconv.Itoa(port)
	for {
		if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
			c.Close()
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// pythonDeps installs the app's libraries once per set of requirements, in
// the Agente's folder, never in the person's Python.
func (r *Runner) pythonDeps(ctx context.Context, base, work, executable string, prefix []string, w *agentproto.WebPayload, env []string) (string, error) {
	if w.DepsFile == "" && len(w.Requirements) == 0 {
		return "", nil // nothing to install
	}
	h := sha256.New()
	var reqFile string
	if w.DepsFile != "" {
		if !safeAppPath(work, w.DepsFile) {
			return "", errors.New("Não encontrei o arquivo de dependências dentro do código do app.")
		}
		reqFile = filepath.Join(work, filepath.FromSlash(w.DepsFile))
		data, err := os.ReadFile(reqFile)
		if err != nil {
			return "", errors.New("Não consegui ler o arquivo de dependências do app.")
		}
		h.Write(data)
	}
	for _, req := range w.Requirements {
		if strings.HasPrefix(req, "-") || strings.ContainsAny(req, " \t\r\n") {
			return "", errors.New("A lista de bibliotecas do app é inválida.")
		}
		h.Write([]byte("\n" + req))
	}
	deps := filepath.Join(base, "deps-"+hex.EncodeToString(h.Sum(nil))[:16])
	if _, err := os.Stat(filepath.Join(deps, ".ok")); err == nil {
		return deps, nil
	}
	_ = os.RemoveAll(deps)
	args := append(append([]string{}, prefix...), "-m", "pip", "install", "--disable-pip-version-check", "--no-input", "--target", deps)
	if reqFile != "" {
		args = append(args, "-r", reqFile)
	}
	args = append(args, w.Requirements...)
	installCtx, stop := context.WithTimeout(ctx, webDepsTimeout)
	defer stop()
	cmd := exec.CommandContext(installCtx, executable, args...)
	cmd.Dir, cmd.Env = work, env
	var log limitedBuffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(deps)
		return "", errors.New("Não consegui preparar as bibliotecas Python do app. Confira a conexão deste computador e o arquivo de dependências.")
	}
	// Older sets of libraries of this app are no longer needed.
	if old, _ := filepath.Glob(filepath.Join(base, "deps-*")); len(old) > 0 {
		for _, o := range old {
			if o != deps {
				_ = os.RemoveAll(o)
			}
		}
	}
	_ = os.WriteFile(filepath.Join(deps, ".ok"), nil, 0o600)
	return deps, nil
}

// npmCommand finds npm. On Windows npm is a .cmd, which Go will not start
// with arguments safely, so npm-cli.js runs under node instead.
func npmCommand(node string) (string, []string, error) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return "", nil, errors.New("Instale o Node.js com npm neste computador e reinicie o Agente para abrir este app.")
	}
	if strings.EqualFold(filepath.Ext(npm), ".cmd") {
		cli := filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js")
		if _, err := os.Stat(cli); err != nil {
			return "", nil, errors.New("Não encontrei o npm instalado com o Node.js. Reinstale o Node.js com npm.")
		}
		return node, []string{cli}, nil
	}
	return npm, nil, nil
}

func (r *Runner) runWeb(ctx context.Context, p agentproto.RunPayload, report func(agentproto.Result)) agentproto.Result {
	w := p.Web
	if w.Runtime != "python" && w.Runtime != "node" || len(w.Args) == 0 || len(w.Sources) == 0 {
		return failed("O Ziv não mandou como ligar este app. Atualize a página e tente de novo.")
	}
	if err := checkWritable(p.OutputFolder); err != nil {
		return failed("Não consegui usar a pasta " + p.OutputFolder + ": ela não existe ou não aceita gravação.")
	}
	base := filepath.Join(r.StateDir, "apps-com-tela", "app-"+strconv.FormatInt(w.AppID, 10))
	work := filepath.Join(base, "codigo")
	// The code is written fresh each time, so a new version replaces the old
	// one; the app's data lives in the chosen folder, not here.
	if err := os.RemoveAll(work); err != nil {
		return failed("Não consegui preparar a pasta do app: ele ainda está aberto em outra janela?")
	}
	if err := writeScriptSources(work, w.Sources); err != nil {
		return failed("Não consegui preparar os arquivos do app: " + err.Error())
	}
	executable, prefix, err := scriptInterpreter(w.Runtime)
	if err != nil {
		return failed(strings.Replace(err.Error(), "rodar este script", "abrir este app", 1))
	}
	port, err := freePort()
	if err != nil {
		return failed("Não achei uma porta livre neste computador para o app.")
	}
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	env := append(os.Environ(), "PYTHONUNBUFFERED=1", "PYTHONDONTWRITEBYTECODE=1", "PYTHONIOENCODING=utf-8")
	report(agentproto.Result{Event: agentproto.EventStarted, AppVersion: "App com tela · " + url})

	args := make([]string, 0, len(w.Args)+len(prefix))
	switch w.Runtime {
	case "python":
		deps, err := r.pythonDeps(ctx, base, work, executable, prefix, w, env)
		if err != nil {
			return failed(err.Error())
		}
		if deps != "" {
			env = append(env, "PYTHONPATH="+deps)
		}
		args = append(args, prefix...)
	case "node":
		if _, err := os.Stat(filepath.Join(work, "package.json")); err != nil {
			return failed("O app não tem package.json.")
		}
		npm, npmPrefix, err := npmCommand(executable)
		if err != nil {
			return failed(err.Error())
		}
		installCtx, stop := context.WithTimeout(ctx, webDepsTimeout)
		install := exec.CommandContext(installCtx, npm, append(append([]string{}, npmPrefix...), "install", "--ignore-scripts", "--no-audit", "--no-fund", "--omit=dev")...)
		install.Dir, install.Env = work, env
		var log limitedBuffer
		install.Stdout, install.Stderr = &log, &log
		err = install.Run()
		stop()
		if err != nil {
			return failed("Não consegui preparar as bibliotecas JavaScript do app. Confira a conexão deste computador.")
		}
		executable = npm
		args = append(args, npmPrefix...)
	}
	for _, a := range w.Args {
		args = append(args, strings.ReplaceAll(a, agentproto.PortPlaceholder, strconv.Itoa(port)))
	}
	// The vault's keys go only to the app, after the libraries are ready.
	env = append(env, "PORT="+strconv.Itoa(port), "BASE_URL="+url, "APPHUB_DATA_DIR="+p.OutputFolder)
	env = append(env, vaultEnv(p.KeyValues)...)

	before, err := TakeSnapshot(p.OutputFolder)
	if err != nil {
		return failed("Não consegui ler a pasta " + p.OutputFolder + ".")
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir, cmd.Env = work, env
	var log limitedBuffer
	cmd.Stdout, cmd.Stderr = &log, &log
	ready := make(chan bool, 1)
	waitCtx, stopWait := context.WithTimeout(ctx, webReadyTimeout)
	defer stopWait()
	go func() { ready <- waitPort(waitCtx, port) }()
	go func() {
		if <-ready && r.OpenURL != nil {
			_ = r.OpenURL(url)
		}
	}()
	err = runTree(ctx, cmd)
	stopWait()
	if !r.wasStopped(p.RunID) {
		// It closed by itself: keep its console for the person, never in the Ziv.
		_ = os.WriteFile(filepath.Join(p.OutputFolder, webLogName(p.RunID)), log.Bytes(), 0o600)
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			return failed("Não consegui ligar o app. Confira a instalação da linguagem neste computador.")
		}
		if err != nil {
			return failed(fmt.Sprintf("O app fechou com erro (código %d). O que ele escreveu está em %s, na pasta escolhida.", exitErr.ExitCode(), webLogName(p.RunID)))
		}
	}
	after, err := TakeSnapshot(p.OutputFolder)
	if err != nil {
		return failed("O app fechou, mas não consegui ler a pasta " + p.OutputFolder + ".")
	}
	files := Diff(before, after)
	if err := saveRecord(r.StateDir, p.RunID, p.OutputFolder, files); err != nil {
		return failed("Não consegui guardar a lista de arquivos desta execução.")
	}
	return agentproto.Result{Event: agentproto.EventFinished, Files: files}
}
