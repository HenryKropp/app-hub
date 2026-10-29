// Package agent is the platform-neutral part of the Agente do Kova: it opens
// apps, finds the files they produced and sends them to the Hub.
package agent

import (
	"io/fs"
	"path/filepath"
	"sort"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

type entry struct {
	Size     int64
	Modified time.Time
}

// Snapshot maps relative paths (forward slashes) to size and modification time.
type Snapshot map[string]entry

// TakeSnapshot lists every file under root, except the relative paths in skip.
func TakeSnapshot(root string, skip ...string) (Snapshot, error) {
	skipSet := map[string]bool{}
	for _, s := range skip {
		skipSet[s] = true
	}
	snap := Snapshot{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skipSet[rel] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		snap[rel] = entry{Size: info.Size(), Modified: info.ModTime()}
		return nil
	})
	return snap, err
}

// Diff lists files that are new in after, or whose size or time changed.
func Diff(before, after Snapshot) []agentproto.FileEntry {
	out := []agentproto.FileEntry{}
	for rel, a := range after {
		b, existed := before[rel]
		switch {
		case !existed:
			out = append(out, agentproto.FileEntry{Path: rel, Size: a.Size, Modified: a.Modified, Change: "new"})
		case b.Size != a.Size || !b.Modified.Equal(a.Modified):
			out = append(out, agentproto.FileEntry{Path: rel, Size: a.Size, Modified: a.Modified, Change: "changed"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
