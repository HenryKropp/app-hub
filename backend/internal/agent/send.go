package agent

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

var (
	ErrNotAllowed = errors.New("file is not a result of this run")
	ErrMissing    = errors.New("file is no longer in the folder")
)

// record is what the agent remembers about a finished run, so it only ever
// sends files that the run produced.
type record struct {
	Folder string                 `json:"folder"`
	Files  []agentproto.FileEntry `json:"files"`
}

func recordPath(stateDir string, runID int64) string {
	return filepath.Join(stateDir, "runs", strconv.FormatInt(runID, 10)+".json")
}

func saveRecord(stateDir string, runID int64, folder string, files []agentproto.FileEntry) error {
	p := recordPath(stateDir, runID)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(record{Folder: folder, Files: files})
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// Outgoing is a prepared answer to a send_file job.
type Outgoing struct {
	Root  string
	Paths []string // relative, existing
	Zip   bool
}

func PrepareSend(stateDir string, p agentproto.SendFilePayload) (Outgoing, error) {
	b, err := os.ReadFile(recordPath(stateDir, p.RunID))
	if err != nil {
		return Outgoing{}, ErrNotAllowed
	}
	var rec record
	if err := json.Unmarshal(b, &rec); err != nil {
		return Outgoing{}, ErrNotAllowed
	}
	allowed := map[string]bool{}
	for _, f := range rec.Files {
		allowed[f.Path] = true
	}
	out := Outgoing{Root: rec.Folder, Zip: p.Zip || len(p.Paths) > 1}
	for _, rel := range p.Paths {
		if !allowed[rel] || strings.Contains(rel, "..") || filepath.IsAbs(filepath.FromSlash(rel)) {
			return Outgoing{}, ErrNotAllowed
		}
		if _, err := os.Stat(filepath.Join(rec.Folder, filepath.FromSlash(rel))); err == nil {
			out.Paths = append(out.Paths, rel)
		}
	}
	if len(out.Paths) == 0 {
		return Outgoing{}, ErrMissing
	}
	return out, nil
}

func (o Outgoing) Stream(w io.Writer) error {
	if !o.Zip {
		f, err := os.Open(filepath.Join(o.Root, filepath.FromSlash(o.Paths[0])))
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}
	zw := zip.NewWriter(w)
	for _, rel := range o.Paths {
		f, err := os.Open(filepath.Join(o.Root, filepath.FromSlash(rel)))
		if err != nil {
			continue // disappeared meanwhile
		}
		dst, err := zw.Create(rel)
		if err == nil {
			_, err = io.Copy(dst, f)
		}
		f.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}
