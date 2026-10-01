package versions

import (
	"sort"
	"time"
)

// RestorePlan says what restoring a version does to the folder. Lists are
// never nil, so they encode as [] for the screens.
type RestorePlan struct {
	Rewrite  []string `json:"rewrite"`  // code files that go back to how they were
	Recreate []string `json:"recreate"` // code files that come back
	ToTrash  []string `json:"to_trash"` // code files that did not exist in the target version, and files with no copy in the Hub that are replaced
	Binaries []string `json:"binaries"` // binaries that differ from the target; they are not restored
}

func PlanRestore(current, target Snapshot) RestorePlan {
	p := RestorePlan{Rewrite: []string{}, Recreate: []string{}, ToTrash: []string{}, Binaries: []string{}}
	cur := map[string]File{}
	for _, f := range current {
		cur[f.Path] = f
	}
	tgt := map[string]File{}
	for _, f := range target {
		tgt[f.Path] = f
		c, ok := cur[f.Path]
		switch {
		case f.Kind == KindCode && !ok:
			p.Recreate = append(p.Recreate, f.Path)
		case f.Kind == KindCode && c.SHA256 != f.SHA256:
			p.Rewrite = append(p.Rewrite, f.Path)
			if c.Kind != KindCode {
				// Binary now: only its fingerprint is saved, so it goes to the trash first.
				p.ToTrash = append(p.ToTrash, f.Path)
			}
		case f.Kind == KindBinary && (!ok || c.SHA256 != f.SHA256):
			p.Binaries = append(p.Binaries, f.Path)
		}
	}
	for _, c := range current {
		_, ok := tgt[c.Path]
		switch {
		case c.Kind == KindCode && !ok:
			p.ToTrash = append(p.ToTrash, c.Path)
		case c.Kind == KindBinary && !ok:
			p.Binaries = append(p.Binaries, c.Path)
		}
	}
	sort.Strings(p.ToTrash)
	return p
}

// PlanRestoreIn is PlanRestore plus the folder itself: a file that comes back
// over something already on disk but missing from the current version (an
// output, a symlink…) has no copy in the Hub, so it goes to the trash first.
func PlanRestoreIn(f Folder, current, target Snapshot) (RestorePlan, error) {
	p := PlanRestore(current, target)
	added := false
	for _, rel := range p.Recreate {
		ok, err := f.Exists(rel)
		if err != nil {
			return p, err
		}
		if ok {
			p.ToTrash = append(p.ToTrash, rel)
			added = true
		}
	}
	if added {
		sort.Strings(p.ToTrash)
	}
	return p, nil
}

type Failure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// TrashFolder is where extra code files go during a restore.
func TrashFolder(now time.Time) string {
	return TrashDir + "/" + now.Format("2006-01-02 150405")
}

// ApplyRestore moves extra code files to the trash and writes back the code
// files of the target version. A path both in ToTrash and written back is
// moved first and only written if the move worked. It stops at the first
// failure; the files not reached are reported as "não foi tentado".
func ApplyRestore(f Folder, p RestorePlan, target Snapshot, blob func(sha string) ([]byte, error), trash string) (done []string, failed []Failure) {
	sha := map[string]string{}
	for _, t := range target {
		sha[t.Path] = t.SHA256
	}
	type step struct {
		path string
		run  func() error
	}
	writes := append(append([]string{}, p.Rewrite...), p.Recreate...)
	replaced := map[string]bool{}
	for _, rel := range writes {
		replaced[rel] = true
	}
	moveFirst := map[string]bool{}
	var steps []step
	for _, rel := range p.ToTrash {
		rel := rel
		if replaced[rel] {
			moveFirst[rel] = true
			continue
		}
		steps = append(steps, step{rel, func() error { return f.Move(rel, trash+"/"+rel) }})
	}
	for _, rel := range writes {
		rel := rel
		steps = append(steps, step{rel, func() error {
			data, err := blob(sha[rel])
			if err != nil {
				return err
			}
			if moveFirst[rel] {
				if err := f.Move(rel, trash+"/"+rel); err != nil {
					return err
				}
			}
			return f.Write(rel, data)
		}})
	}
	done = []string{}
	for i, s := range steps {
		if err := s.run(); err != nil {
			failed = append(failed, Failure{Path: s.path, Error: err.Error()})
			for _, rest := range steps[i+1:] {
				failed = append(failed, Failure{Path: rest.path, Error: "não foi tentado"})
			}
			return done, failed
		}
		done = append(done, s.path)
	}
	return done, nil
}
