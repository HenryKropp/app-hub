package versions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

// File is one file of a version. Only code files have their content stored.
type File struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	SHA256   string    `json:"sha256"`
	Kind     string    `json:"kind"`
}

// Snapshot is the list of files of a version, sorted by path.
type Snapshot []File

// Scan reads the folder into a snapshot. Files whose size and modification
// time match prev are trusted without being read, so OneDrive "online-only"
// files stay in the cloud. Files with passwords (SecretFile) are left out. It returns, by SHA-256, the contents of the code
// files it had to read. Any read error fails the whole scan: a file that could
// not be read must never look like a removed file.
func Scan(f Folder, outs []manifest.Output, prev Snapshot) (Snapshot, map[string][]byte, error) {
	known := make(map[string]File, len(prev))
	for _, p := range prev {
		known[p.Path] = p
	}
	snap := Snapshot{}
	contents := map[string][]byte{}
	err := f.Walk(func(rel string, dir bool, size int64, mod time.Time) error {
		if dir {
			if junkDirs[path.Base(rel)] {
				return fs.SkipDir
			}
			return nil
		}
		if !Included(rel, outs) || SecretFile(rel) {
			return nil
		}
		if p, ok := known[rel]; ok && p.Size == size && p.Modified.Equal(mod) {
			snap = append(snap, p)
			return nil
		}
		file, data, err := readFile(f, rel, size, mod)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if data != nil {
			contents[file.SHA256] = data
		}
		snap = append(snap, file)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(snap, func(i, j int) bool { return snap[i].Path < snap[j].Path })
	return snap, contents, nil
}

func readFile(f Folder, rel string, size int64, mod time.Time) (File, []byte, error) {
	r, err := f.Open(rel)
	if err != nil {
		return File{}, nil, err
	}
	defer r.Close()
	file := File{Path: rel, Size: size, Modified: mod, Kind: KindBinary}
	h := sha256.New()
	if isCodeName(rel) && size <= MaxCodeSize {
		data, err := io.ReadAll(io.LimitReader(r, MaxCodeSize+1))
		if err != nil {
			return File{}, nil, err
		}
		h.Write(data)
		file.SHA256 = hex.EncodeToString(h.Sum(nil))
		file.Size = int64(len(data))
		if len(data) <= MaxCodeSize && !bytes.Contains(data[:min(len(data), 8192)], []byte{0}) {
			file.Kind = KindCode
			return file, data, nil
		}
		return file, nil, nil
	}
	if _, err := io.Copy(h, r); err != nil {
		return File{}, nil, err
	}
	file.SHA256 = hex.EncodeToString(h.Sum(nil))
	return file, nil, nil
}

// Same reports whether two snapshots have the same files with the same content.
func Same(a, b Snapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path || a[i].SHA256 != b[i].SHA256 {
			return false
		}
	}
	return true
}

type FileChange struct {
	Path   string
	Status string // "added", "removed" or "modified"
	Old    *File
	New    *File
}

// Changes lists the files that differ between two snapshots, sorted by path.
func Changes(old, cur Snapshot) []FileChange {
	byPath := map[string]*File{}
	for i := range old {
		byPath[old[i].Path] = &old[i]
	}
	var out []FileChange
	seen := map[string]bool{}
	for i := range cur {
		n := &cur[i]
		seen[n.Path] = true
		o, ok := byPath[n.Path]
		switch {
		case !ok:
			out = append(out, FileChange{Path: n.Path, Status: "added", New: n})
		case o.SHA256 != n.SHA256:
			out = append(out, FileChange{Path: n.Path, Status: "modified", Old: o, New: n})
		}
	}
	for i := range old {
		if !seen[old[i].Path] {
			out = append(out, FileChange{Path: old[i].Path, Status: "removed", Old: &old[i]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Summary counts the code changes of a version and names the binaries that changed.
type Summary struct {
	Added    int      `json:"added"`
	Removed  int      `json:"removed"`
	Modified int      `json:"modified"`
	Binaries []string `json:"binaries"`
}

func Summarize(ch []FileChange) Summary {
	s := Summary{Binaries: []string{}}
	for _, c := range ch {
		if (c.New != nil && c.New.Kind == KindBinary) || (c.New == nil && c.Old.Kind == KindBinary) {
			s.Binaries = append(s.Binaries, c.Path)
			continue
		}
		switch c.Status {
		case "added":
			s.Added++
		case "removed":
			s.Removed++
		default:
			s.Modified++
		}
	}
	return s
}

// LastModified is the newest modification time among added or changed files.
func LastModified(ch []FileChange) (time.Time, bool) {
	var last time.Time
	for _, c := range ch {
		if c.New != nil && c.New.Modified.After(last) {
			last = c.New.Modified
		}
	}
	return last, !last.IsZero()
}
