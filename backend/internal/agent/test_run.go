package agent

import (
	"archive/zip"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/mask"
)

// TestFolder is where a draft's test writes: inside the Agente's state
// folder, never the app's official output folder.
func TestFolder(stateDir string, appID, draftID int64) string {
	return filepath.Join(stateDir, "testes", "app-"+strconv.FormatInt(appID, 10), "rascunho-"+strconv.FormatInt(draftID, 10))
}

// RunTest runs a draft's script in test mode (ZIV_TESTE=1, ZIV_PASTA_TESTE)
// with folder as its output. Result.Files lists everything in folder and
// Result.Log is the console with the vault values masked.
func (r *Runner) RunTest(ctx context.Context, p agentproto.TestPayload, folder string) agentproto.Result {
	if err := os.RemoveAll(folder); err != nil {
		return failed("Não consegui limpar a pasta do teste anterior.")
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return failed("Não consegui criar a pasta do teste.")
	}
	timeout := time.Duration(p.TimeoutMin) * time.Minute
	if timeout <= 0 || timeout > manifest.MaxTestMinutes*time.Minute {
		timeout = manifest.MaxTestMinutes * time.Minute
	}
	script := p.Script
	run := agentproto.RunPayload{AppFolder: folder, Exe: script.Manifest.Entrypoint, OutputFolder: folder,
		Script: &script, AppName: p.AppName, Keys: p.Keys, KeyValues: p.KeyValues}
	res := r.runScriptIn(ctx, run, func(agentproto.Result) {}, scriptOpts{
		ExtraEnv: []string{"ZIV_TESTE=1", "ZIV_PASTA_TESTE=" + folder},
		Timeout:  timeout,
	})
	values := make([]string, 0, len(p.KeyValues))
	for _, v := range p.KeyValues {
		values = append(values, v)
	}
	res.Error = mask.Values(res.Error, values)
	if res.Event != agentproto.EventFinished {
		return res
	}
	if b, err := os.ReadFile(filepath.Join(folder, consoleName)); err == nil {
		res.Log = mask.Values(string(b), values)
		// The console stays on this PC masked too: the test folder is not the
		// place for a vault value.
		_ = os.WriteFile(filepath.Join(folder, consoleName), []byte(res.Log), 0o600)
	}
	files, err := listFolder(folder)
	if err != nil {
		return failed("O teste terminou, mas não consegui ler a pasta do teste.")
	}
	res.Files = files
	return res
}

const consoleName = ".apphub-console.txt"

// listFolder lists every regular file under folder, with forward slashes.
func listFolder(folder string) ([]agentproto.FileEntry, error) {
	out := []agentproto.FileEntry{}
	err := filepath.WalkDir(folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(folder, p)
		if err != nil {
			return err
		}
		out = append(out, agentproto.FileEntry{Path: filepath.ToSlash(rel), Size: info.Size(), Modified: info.ModTime(), Change: "new"})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// evidenceZip writes the test folder's files (not the console, which goes in
// Result.Log) as a zip, up to limit bytes and MaxEvidenceFiles files. It
// returns the files left out.
func evidenceZip(w io.Writer, folder string, limit int64) ([]string, error) {
	files, err := listFolder(folder)
	if err != nil {
		return nil, err
	}
	zw := zip.NewWriter(w)
	var total int64
	count := 0
	var skipped []string
	for _, f := range files {
		if f.Path == consoleName {
			continue
		}
		if total+f.Size > limit || count >= agentproto.MaxEvidenceFiles {
			skipped = append(skipped, f.Path)
			continue
		}
		src, err := os.Open(filepath.Join(folder, filepath.FromSlash(f.Path)))
		if err != nil {
			skipped = append(skipped, f.Path)
			continue
		}
		dst, err := zw.Create(f.Path)
		if err == nil {
			_, err = io.Copy(dst, io.LimitReader(src, f.Size))
		}
		src.Close()
		if err != nil {
			return skipped, err
		}
		total += f.Size
		count++
	}
	return skipped, zw.Close()
}
