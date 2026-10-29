package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func safeAppPath(root, rel string) bool {
	if !manifest.SafeRelative(rel) {
		return false
	}
	p, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(rel, `\`, "/"))))
	if err != nil {
		return false
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	r, err := filepath.Rel(base, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}

// Script runs have a private working directory, NOT an OS security sandbox.
// Only apps reviewed by a trusted creator should execute under a user's account.
func (r *Runner) runScript(ctx context.Context, p agentproto.RunPayload, report func(agentproto.Result)) agentproto.Result {
	s := p.Script
	if p.Exe != s.Manifest.Entrypoint || s.Manifest.Kind != manifest.KindScript {
		return failed("A ficha do script não corresponde ao arquivo solicitado.")
	}
	if err := s.Manifest.ValidateRunInputs(s.Inputs, s.Files); err != nil {
		return failed(err.Error())
	}
	if len(s.Sources) == 0 && !safeAppPath(p.AppFolder, p.Exe) {
		return failed("Não encontrei um script seguro dentro da pasta do app. Confira a ficha.")
	}
	if err := checkWritable(p.OutputFolder); err != nil {
		return failed("A pasta de resultados não existe ou não aceita gravação. Escolha outra pasta.")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	root, err := os.MkdirTemp("", "apphub-script-*")
	if err != nil {
		return failed("Não consegui preparar a pasta temporária do script.")
	}
	defer os.RemoveAll(root)
	work := filepath.Join(root, "app")
	if len(s.Sources) > 0 {
		if err := writeScriptSources(work, s.Sources); err != nil {
			return failed(err.Error())
		}
	} else if err := copyScriptTree(p.AppFolder, work); err != nil {
		return failed("Não consegui preparar os arquivos do app: " + err.Error())
	}
	if !safeAppPath(work, p.Exe) {
		return failed("O pacote enviado não contém o arquivo que inicia o script.")
	}
	runtimeName := manifest.ScriptRuntime(p.Exe)
	executable, prefix, err := scriptInterpreter(runtimeName)
	if err != nil {
		return failed(err.Error())
	}
	env := os.Environ()
	env = append(env, "APP_HUB_OUTPUT_DIR="+work, "PYTHONUNBUFFERED=1", "PYTHONDONTWRITEBYTECODE=1")
	report(agentproto.Result{Event: agentproto.EventStarted, AppVersion: "Script · Agente " + agentproto.Version})

	// Dependencies live outside the working folder and never modify the user's
	// Python installation. Do not use a shell or accept install options from UI.
	if runtimeName == "python" && s.Manifest.Runtime.DependenciesFile != "" {
		depsFile := s.Manifest.Runtime.DependenciesFile
		if !safeAppPath(work, depsFile) {
			return failed("Não encontrei o arquivo de dependências dentro da pasta do app.")
		}
		deps := filepath.Join(root, "python-deps")
		args := append(append([]string{}, prefix...), "-m", "pip", "install", "--disable-pip-version-check", "--no-input", "--target", deps, "-r", filepath.Join(work, filepath.FromSlash(depsFile)))
		installCtx, stop := context.WithTimeout(ctx, 5*time.Minute)
		cmd := exec.CommandContext(installCtx, executable, args...)
		cmd.Dir, cmd.Env = work, env
		var log limitedBuffer
		cmd.Stdout, cmd.Stderr = &log, &log
		err := cmd.Run()
		stop()
		if err != nil {
			return failed("Não consegui preparar as bibliotecas Python. Confira o arquivo de dependências, a instalação do pip e a conexão deste computador.")
		}
		env = append(env, "PYTHONPATH="+deps)
	}
	if runtimeName == "node" {
		if data, err := os.ReadFile(filepath.Join(work, "package.json")); err == nil {
			var pkg struct {
				Dependencies map[string]string `json:"dependencies"`
			}
			if json.Unmarshal(data, &pkg) != nil {
				return failed("O package.json do app é inválido.")
			}
			if len(pkg.Dependencies) > 0 {
				if _, err := os.Stat(filepath.Join(work, "node_modules")); err != nil {
					npm, npmErr := exec.LookPath("npm")
					if npmErr != nil {
						return failed("Instale o Node.js com npm neste computador e reinicie o Agente para preparar as bibliotecas.")
					}
					installArgs := []string{"install", "--ignore-scripts", "--no-audit", "--no-fund", "--omit=dev", "--package-lock=false"}
					if _, err := os.Stat(filepath.Join(work, "package-lock.json")); err == nil {
						installArgs[0] = "ci"
					}
					if strings.EqualFold(filepath.Ext(npm), ".cmd") {
						cli := filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js")
						if _, err := os.Stat(cli); err != nil {
							return failed("Não encontrei o npm instalado com o Node.js. Reinstale o Node.js com npm.")
						}
						npm = executable
						installArgs = append([]string{cli}, installArgs...)
					}
					installCtx, stop := context.WithTimeout(ctx, 5*time.Minute)
					install := exec.CommandContext(installCtx, npm, installArgs...)
					install.Dir, install.Env = work, env
					var installLog limitedBuffer
					install.Stdout, install.Stderr = &installLog, &installLog
					err := install.Run()
					stop()
					if err != nil {
						return failed("Não consegui preparar as bibliotecas JavaScript. Confira o package.json, o arquivo de versões e a conexão. Scripts de instalação de pacotes são bloqueados por segurança.")
					}
				}
			}
		}
	}

	args := append(append([]string{}, prefix...), filepath.Join(work, filepath.FromSlash(strings.ReplaceAll(p.Exe, `\`, "/"))))
	var stdin strings.Builder
	for _, in := range s.Manifest.Inputs {
		if in.Source == "window" {
			continue
		}
		values := s.Inputs[in.ID]
		if f, ok := s.Files[in.ID]; ok {
			data, _ := base64.StdEncoding.DecodeString(f.ContentBase64)
			rel := filepath.Join(".apphub-inputs", in.ID, f.Name)
			if in.Source == "fixed_file" {
				rel = filepath.FromSlash(strings.ReplaceAll(in.Path, `\`, "/"))
			}
			// Never overwrite the script, manifest or an existing source file;
			// fixed_file is an explicit exception for the declared data file.
			dest := filepath.Join(work, rel)
			if strings.HasPrefix(strings.ToLower(filepath.ToSlash(rel)), ".apphub-") && in.Source == "fixed_file" {
				return failed("O caminho de entrada usa um nome reservado pelo Kova.")
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
				return failed("Não consegui preparar o arquivo de entrada.")
			}
			if err := os.WriteFile(dest, data, 0o600); err != nil {
				return failed("Não consegui guardar o arquivo de entrada.")
			}
			values = []string{dest}
		}
		if len(values) == 0 {
			continue
		}
		key := strings.ToUpper(strings.ReplaceAll(in.ID, "-", "_"))
		// JSON preserves multiple selections; the plain variable is convenient
		// for scripts with a single value.
		raw, _ := json.Marshal(values)
		env = append(env, "APP_HUB_INPUT_"+key+"="+strings.Join(values, string(os.PathListSeparator)), "APP_HUB_INPUT_"+key+"_JSON="+string(raw))
		switch in.Source {
		case "argument":
			flag := in.Flag
			if flag == "" {
				flag = "--" + in.ID
			}
			for _, value := range values {
				args = append(args, flag+"="+value)
			}
		case "prompt":
			for _, value := range values {
				stdin.WriteString(value)
				stdin.WriteByte('\n')
			}
		}
	}
	before, err := TakeSnapshot(work)
	if err != nil {
		return failed("Não consegui verificar os arquivos antes da execução.")
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = work, env, strings.NewReader(stdin.String())
	var log limitedBuffer
	cmd.Stdout, cmd.Stderr = &log, &log
	err = runTree(ctx, cmd)
	exit := 0
	// A stopped script still hands over what it wrote until then.
	if ctx.Err() != nil && !r.wasStopped(p.RunID) {
		return failed("O script foi interrompido ou ultrapassou o limite de 15 minutos. Confira o script e tente novamente.")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exit = exitErr.ExitCode()
	} else if err != nil {
		return failed("Não consegui iniciar o script. Confira a instalação da linguagem neste computador.")
	}
	after, err := TakeSnapshot(work)
	if err != nil {
		return failed("O script terminou, mas não consegui ler os resultados.")
	}
	generated := Diff(before, after)
	// A console-only script has its printed result delivered as a text file.
	if log.Len() > 0 {
		name := ".apphub-console.txt"
		if err := os.WriteFile(filepath.Join(work, name), log.Bytes(), 0o600); err != nil {
			return failed("Não consegui guardar a saída do script.")
		}
		generated = append(generated, agentproto.FileEntry{Path: name, Size: int64(log.Len()), Modified: time.Now(), Change: "new"})
	}
	subdir := fmt.Sprintf("Execucao-%d", p.RunID)
	out := filepath.Join(p.OutputFolder, subdir)
	if len(generated) > 0 {
		if err := os.Mkdir(out, 0o700); err != nil {
			return failed("Já existe uma pasta desta execução ou não consegui criá-la. Nenhum arquivo anterior foi substituído.")
		}
	}
	files := make([]agentproto.FileEntry, 0, len(generated))
	for _, f := range generated {
		if !safeAppPath(work, f.Path) {
			return failed("O script gerou um atalho fora da pasta de trabalho. Não vou copiar esse arquivo.")
		}
		dest := filepath.Join(out, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return failed("Não consegui criar a pasta de resultados.")
		}
		if err := copyFile(filepath.Join(work, filepath.FromSlash(f.Path)), dest); err != nil {
			return failed("Não consegui guardar todos os arquivos de resultado.")
		}
		f.Path = filepath.ToSlash(filepath.Join(subdir, filepath.FromSlash(f.Path)))
		files = append(files, f)
	}
	if err := saveRecord(r.StateDir, p.RunID, p.OutputFolder, files); err != nil {
		return failed("Não consegui guardar o registro dos resultados.")
	}
	return agentproto.Result{Event: agentproto.EventFinished, ExitCode: exit, Files: files}
}

func scriptInterpreter(language string) (string, []string, error) {
	names := []string{"node"}
	if language == "python" {
		names = []string{"python", "python3", "py"}
	}
	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			if name == "py" {
				return p, []string{"-3"}, nil
			}
			return p, nil, nil
		}
	}
	label := "Node.js"
	if language == "python" {
		label = "Python 3 com pip"
	}
	return "", nil, fmt.Errorf("Instale %s neste computador e reinicie o Agente para rodar este script.", label)
}

type limitedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if remaining := (64 << 10) - b.buffer.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buffer.Write(p)
	}
	return n, nil
}
func (b *limitedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Len()
}
func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.buffer.Bytes())
}

func copyScriptTree(src, dst string) error {
	var total int64
	count := 0
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel != "." && (d.Name() == ".git" || d.Name() == ".venv" || d.Name() == "__pycache__" || strings.HasPrefix(d.Name(), ".apphub-")) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("A pasta contém atalhos simbólicos. Use arquivos reais dentro da pasta do app.")
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("A pasta contém um arquivo especial não suportado.")
		}
		total += info.Size()
		count++
		if total > 200<<20 || count > 10000 {
			return errors.New("A pasta ultrapassa 200 MB ou 10 mil arquivos. Remova dados antigos e dependências que não usa.")
		}
		return copyFile(p, target)
	})
}

func writeScriptSources(root string, sources map[string]manifest.InputFile) error {
	total := 0
	if len(sources) > 100 {
		return errors.New("O pacote tem arquivos demais.")
	}
	seen := map[string]bool{}
	for name, f := range sources {
		key := strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
		if !manifest.SafeRelative(name) || seen[key] {
			return errors.New("O pacote contém um caminho inválido ou repetido.")
		}
		seen[key] = true
		data, err := base64.StdEncoding.DecodeString(f.ContentBase64)
		if err != nil {
			return errors.New("Não consegui ler um arquivo do pacote.")
		}
		total += len(data)
		if total > manifest.MaxInputBytes {
			return errors.New("O pacote ultrapassou 10 MB.")
		}
		p := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(name, `\`, "/")))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(p, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}
