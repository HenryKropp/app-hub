# Fase 5: histórico e voltar atrás — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Each app gets one timeline with code versions (detected automatically from its folder), runs and ficha changes, plus "see changes", "see as it was" and "restore code to this version".

**Architecture:** A new `internal/versions` package holds the pure rules (what enters a version, folder scan, line diff, restore plan/apply) behind a small `Folder` interface (today: local disk via WSL; later: the Agent). Handlers in `internal/handler` call it and store versions in Postgres (migration 005): file list as JSONB on `app_versions`, code contents de-duplicated by SHA-256 in `code_blobs`. The React app gets a "Histórico" section on the ficha and a version page.

**Tech Stack:** Go stdlib `net/http`, pgx/v5, sqlc, React + TypeScript + Vitest, shadcn/ui (base-ui), Tailwind.

**Spec:** `docs/superpowers/specs/2026-10-01-fase-5-historico-design.md`

## Global Constraints

- All UI text in plain Brazilian Portuguese; no unexplained technical terms.
- Only code files are copied: known code/text extension, ≤ 1 MB (`MaxCodeSize = 1 << 20`), no NUL byte in the first 8 KB. Everything else is a binary fingerprint (path, size, mtime, SHA-256).
- Excluded from versions: files matching the manifest `outputs` patterns; any path under `__pycache__`, `.venv`, `venv`, `node_modules`, `.git`, `.app-hub-lixeira`; `app-hub.yaml` and `FICHA.md` at the folder root.
- Files whose size and mtime equal the previous version are trusted without being read (OneDrive "online-only").
- Versions are created automatically, never asked. Folder versions are authored by the app owner with `author_probable = true`.
- Restore rewrites code only; extra code files are moved to `.app-hub-lixeira/AAAA-MM-DD HHMMSS/<path>`, never deleted; binaries are never touched. Only roles `admin` and `criador`. Blocked while the app has a run `waiting` or `running`.
- Secrets in code are masked as `••••` in every screen (changes and "como estava"); stored copies are raw.
- Ficha changes are only recorded, never restored.
- No new Go or npm dependencies.
- sqlc: after editing queries, run `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- sqlc generate'` and **read** the generated files in `backend/internal/database/sqlc/` to confirm names before using them. Never write SQL in Go handler code.
- Go tests need the database: `bash -c 'ROOT="$(git rev-parse --show-toplevel)" && set -a && . "$ROOT/.env" && set +a && cd "$ROOT/backend" && mise x -- go test ./...'`. Frontend: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run && mise x -- npx tsc -b'`.
- Never start a test Hub on port 8080 (the real Agent talks to it).

## Review Focus

1. **Folder unreadable or failing mid-scan** (OneDrive error, permission denied, folder renamed): no version is created and nothing is reported as "removed"; the ficha still opens and Rodar still works. Tests: Task 2 (`TestScanFailsInsteadOfDroppingFiles`), Task 6 (`TestGetAppWithMissingFolder`).
2. **mtime changes but content does not** (OneDrive resync, copy, `touch`): no new version. Test: Task 2 (`TestScanTouchedFileSameContent`) and Task 6 (`TestCheckVersionIgnoresTouch`).
3. **A file cannot be written during restore** (locked/read-only): restore stops, reports what was restored and what was not, and still records a version with what is in the folder. Test: Task 9 (`TestRestoreStopsOnFailure`).
4. **A `usuario` (not admin/criador)** sees only their own runs in the timeline and cannot describe or restore. Tests: Task 7 (`TestHistoryRunVisibility`), Task 9 (`TestRestoreRequiresRole`), Task 8 (`TestDescribeVersion`).
5. **`?path=` with a path not in the version** (`../`, absolute, binary): refused, never reads the disk. Test: Task 8 (`TestVersionFileContent`).

---

## File map

| File | Responsibility |
|---|---|
| `backend/internal/database/migrations/005_versions.sql` | New tables and `runs.version_id` |
| `backend/internal/database/queries/versions.sql` | Version, blob, change and timeline queries |
| `backend/internal/database/queries/apps.sql`, `runs.sql` | `GetAppByFolder`; runs get `version_id` |
| `backend/internal/versions/rules.go` | `Included`, code vs binary, constants |
| `backend/internal/versions/folder.go` | `Folder` interface, `LocalFolder` |
| `backend/internal/versions/snapshot.go` | `File`, `Snapshot`, `Scan`, `Same`, `Changes`, `Summarize` |
| `backend/internal/versions/diff.go` | `DiffLines` |
| `backend/internal/versions/restore.go` | `PlanRestore`, `ApplyRestore` |
| `backend/internal/scanner/secrets.go` | `MaskSecrets` |
| `backend/internal/manifest/diff.go` | `Diff` (ficha changes) |
| `backend/internal/handler/versions.go` | Version check/insert, locks, version detail routes |
| `backend/internal/handler/history.go` | Timeline route |
| `backend/internal/handler/restore.go` | Restore preview and restore routes |
| `frontend/src/lib/api.ts` | Types and calls |
| `frontend/src/components/HistoryTimeline.tsx` | Timeline with filter and "Carregar mais" |
| `frontend/src/components/DescribeDialog.tsx`, `RestoreDialog.tsx` | Dialogs used by timeline and version page |
| `frontend/src/components/DiffView.tsx` | Red/green line view |
| `frontend/src/pages/VersionPage.tsx` | "O que mudou" / "Como estava" |

---

### Task 1: Database (migration 005 and queries)

**Files:**
- Create: `backend/internal/database/migrations/005_versions.sql`
- Create: `backend/internal/database/queries/versions.sql`
- Modify: `backend/internal/database/queries/apps.sql`, `backend/internal/database/queries/runs.sql`
- Modify: `backend/internal/handler/runs.go` (`CreateRun` call gains `VersionID`)
- Modify: `backend/internal/handler/handler_test.go` (TRUNCATE lists)

**Interfaces:**
- Produces (sqlc, confirm exact names after generate): `LatestAppVersion(ctx, appID) (AppVersion, error)`, `GetAppVersion(ctx, GetAppVersionParams{AppID, Number}) (GetAppVersionRow, error)` (has `AuthorName pgtype.Text`), `InsertAppVersion(ctx, InsertAppVersionParams{AppID, Origin, RestoredFrom pgtype.Int4, AuthorID pgtype.Int8, AuthorProbable bool, Files []byte, Summary []byte, FilesChangedAt pgtype.Timestamptz}) (AppVersion, error)`, `PutCodeBlob(ctx, PutCodeBlobParams{Sha256, Content, Size})`, `GetCodeBlob(ctx, sha) ([]byte, error)`, `DescribeAppVersion(ctx, DescribeAppVersionParams{Description, AuthorID, DescribedBy, AppID, Number})`, `ListVersionsBefore`, `ListRunsBefore`, `ListChangesBefore` (params `{AppID, Before pgtype.Timestamptz, Lim int32}`; runs also `SeeAll bool, PersonID int64`), `InsertAppChange(ctx, InsertAppChangeParams{AppID, PersonID, Changes []byte})`, `AppHasActiveRun(ctx, appID) (bool, error)`, `GetAppByFolder(ctx, folder) (App, error)`; `CreateRunParams.VersionID pgtype.Int8`; `GetRunRow.VersionNumber pgtype.Int4`.

- [ ] **Step 1: Write the migration**

```sql
-- 005_versions.sql: code versions, ficha changes, and runs linked to a version.
-- clock_timestamp() instead of now(): several rows written in one transaction
-- (tests) still get distinct, ordered times for the timeline.
CREATE TABLE IF NOT EXISTS code_blobs (
    sha256  TEXT PRIMARY KEY,
    content BYTEA NOT NULL,
    size    BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_versions (
    id               BIGSERIAL PRIMARY KEY,
    app_id           BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    number           INT NOT NULL,
    origin           TEXT NOT NULL CHECK (origin IN ('original', 'folder', 'restore')),
    restored_from    INT,
    author_id        BIGINT REFERENCES people(id),
    author_probable  BOOLEAN NOT NULL DEFAULT false,
    description      TEXT NOT NULL DEFAULT '',
    described_by     BIGINT REFERENCES people(id),
    described_at     TIMESTAMPTZ,
    files            JSONB NOT NULL,
    summary          JSONB NOT NULL,
    files_changed_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (app_id, number)
);
CREATE INDEX IF NOT EXISTS app_versions_app_idx ON app_versions(app_id, created_at DESC);

CREATE TABLE IF NOT EXISTS app_changes (
    id         BIGSERIAL PRIMARY KEY,
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    person_id  BIGINT NOT NULL REFERENCES people(id),
    changes    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS app_changes_app_idx ON app_changes(app_id, created_at DESC);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS version_id BIGINT REFERENCES app_versions(id);
```

- [ ] **Step 2: Write `queries/versions.sql`**

```sql
-- name: LatestAppVersion :one
SELECT * FROM app_versions WHERE app_id = $1 ORDER BY number DESC LIMIT 1;

-- name: GetAppVersion :one
SELECT v.*, p.name AS author_name
FROM app_versions v LEFT JOIN people p ON p.id = v.author_id
WHERE v.app_id = $1 AND v.number = $2;

-- name: InsertAppVersion :one
INSERT INTO app_versions (app_id, number, origin, restored_from, author_id, author_probable, files, summary, files_changed_at)
VALUES (sqlc.arg(app_id),
        (SELECT COALESCE(MAX(number), 0) + 1 FROM app_versions WHERE app_id = sqlc.arg(app_id)),
        sqlc.arg(origin), sqlc.narg(restored_from), sqlc.narg(author_id), sqlc.arg(author_probable),
        sqlc.arg(files), sqlc.arg(summary), sqlc.narg(files_changed_at))
RETURNING *;

-- name: PutCodeBlob :exec
INSERT INTO code_blobs (sha256, content, size) VALUES ($1, $2, $3) ON CONFLICT (sha256) DO NOTHING;

-- name: GetCodeBlob :one
SELECT content FROM code_blobs WHERE sha256 = $1;

-- name: DescribeAppVersion :exec
UPDATE app_versions
SET description = sqlc.arg(description), author_id = sqlc.narg(author_id), author_probable = false,
    described_by = sqlc.arg(described_by), described_at = now()
WHERE app_id = sqlc.arg(app_id) AND number = sqlc.arg(number);

-- name: ListVersionsBefore :many
SELECT v.number, v.origin, v.restored_from, v.author_probable, v.description, v.summary,
       v.files_changed_at, v.created_at, p.name AS author_name
FROM app_versions v LEFT JOIN people p ON p.id = v.author_id
WHERE v.app_id = sqlc.arg(app_id) AND v.created_at < sqlc.arg(before)
ORDER BY v.created_at DESC
LIMIT sqlc.arg(lim);

-- name: ListRunsBefore :many
SELECT r.id, r.status, r.created_at, p.name AS person_name, v.number AS version_number
FROM runs r
JOIN people p ON p.id = r.person_id
LEFT JOIN app_versions v ON v.id = r.version_id
WHERE r.app_id = sqlc.arg(app_id) AND r.created_at < sqlc.arg(before)
  AND (sqlc.arg(see_all)::boolean OR r.person_id = sqlc.arg(person_id))
ORDER BY r.created_at DESC
LIMIT sqlc.arg(lim);

-- name: InsertAppChange :exec
INSERT INTO app_changes (app_id, person_id, changes) VALUES ($1, $2, $3);

-- name: ListChangesBefore :many
SELECT c.changes, c.created_at, p.name AS person_name
FROM app_changes c JOIN people p ON p.id = c.person_id
WHERE c.app_id = sqlc.arg(app_id) AND c.created_at < sqlc.arg(before)
ORDER BY c.created_at DESC
LIMIT sqlc.arg(lim);

-- name: AppHasActiveRun :one
SELECT EXISTS(SELECT 1 FROM runs WHERE app_id = $1 AND status IN ('waiting', 'running'));
```

- [ ] **Step 3: Edit `apps.sql` and `runs.sql`**

Append to `apps.sql`:

```sql
-- name: GetAppByFolder :one
SELECT * FROM apps WHERE folder_path = $1;
```

In `runs.sql`, replace `CreateRun` and `GetRun`:

```sql
-- name: CreateRun :one
INSERT INTO runs (app_id, person_id, computer_id, output_folder, version_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
```

```sql
-- name: GetRun :one
SELECT r.*, a.name AS app_name, a.manifest AS app_manifest, p.name AS person_name, c.name AS computer_name,
       v.number AS version_number
FROM runs r
JOIN apps a ON a.id = r.app_id
JOIN people p ON p.id = r.person_id
JOIN computers c ON c.id = r.computer_id
LEFT JOIN app_versions v ON v.id = r.version_id
WHERE r.id = $1;
```

- [ ] **Step 4: Generate and read**

Run sqlc generate (Global Constraints). Read `backend/internal/database/sqlc/versions.sql.go`, `models.go`, `runs.sql.go`. Write down the exact param/row field names; later tasks use the names listed under **Interfaces** above — if any differs, use the generated one everywhere.

- [ ] **Step 5: Keep the build green**

In `backend/internal/handler/runs.go` `startRun`, the `CreateRun` call compiles unchanged except it now needs `VersionID` (zero value `pgtype.Int8{}` is fine for now; Task 6 fills it). In `handler_test.go`, both TRUNCATE statements become:

```go
"TRUNCATE run_files, agent_jobs, runs, app_changes, app_versions, code_blobs, pairings, computers, apps, sessions, people RESTART IDENTITY CASCADE"
```

- [ ] **Step 6: Run all Go tests**

Run the Go test command. Expected: PASS (migration applies to `apphub_test`; existing tests unchanged).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/database backend/internal/handler/runs.go backend/internal/handler/handler_test.go
git commit -m "Versions: migration 005 and queries"
```

---

### Task 2: `versions` package — rules, folder and scan

**Files:**
- Create: `backend/internal/versions/rules.go`, `folder.go`, `snapshot.go`
- Test: `backend/internal/versions/snapshot_test.go`

**Interfaces:**
- Produces:
  - `const MaxCodeSize = 1 << 20`, `const TrashDir = ".app-hub-lixeira"`, `const KindCode = "code"`, `const KindBinary = "binary"`
  - `func Included(rel string, outs []manifest.Output) bool`
  - `type Folder interface { Walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error; Open(rel string) (io.ReadCloser, error); Write(rel string, data []byte) error; Move(from, to string) error }`
  - `type LocalFolder struct{ Root string }` implementing `Folder`
  - `var ErrFolderMissing`
  - `type File struct { Path string; Size int64; Modified time.Time; SHA256 string; Kind string }` (JSON: `path,size,modified,sha256,kind`)
  - `type Snapshot []File` (sorted by Path)
  - `func Scan(f Folder, outs []manifest.Output, prev Snapshot) (Snapshot, map[string][]byte, error)` — map is SHA-256 → content of code files read in this scan
  - `func Same(a, b Snapshot) bool`
  - `type FileChange struct { Path, Status string; Old, New *File }` (Status `added|removed|modified`)
  - `func Changes(old, new Snapshot) []FileChange`
  - `type Summary struct { Added, Removed, Modified int; Binaries []string }` (JSON `added,removed,modified,binaries`)
  - `func Summarize(ch []FileChange) Summary`
  - `func LastModified(ch []FileChange) (time.Time, bool)`

- [ ] **Step 1: Write the failing tests**

```go
package versions

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

var prepOutputs = []manifest.Output{
	{Label: "Documentos", Pattern: "*/*/**/*.pdf"},
	{Label: "Data Pack", Pattern: "*/MD_Output/*_DataPack_Consolidado.md"},
	{Label: "FRE cortado", Pattern: "*_secoes.pdf"},
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func filesPrepFolder(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "Código-fonte/Files Prep.py", "print('oi')\n")
	write(t, root, "Código-fonte/requirements.txt", "pypdf\n")
	write(t, root, "Ferramenta CVM.exe", "MZ\x00\x01binary")
	write(t, root, "app-hub.yaml", "id: x\n")
	write(t, root, "FICHA.md", "# x\n")
	write(t, root, "PETROBRAS/5. FRE/2025/FRE.pdf", "%PDF")
	write(t, root, "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", "# pack\n")
	write(t, root, "FRE_secoes.pdf", "%PDF")
	write(t, root, "Old/__pycache__/x.pyc", "\x00")
	write(t, root, ".app-hub-lixeira/2026-10-01 120000/velho.py", "x\n")
	return root
}

func paths(s Snapshot) []string {
	out := []string{}
	for _, f := range s {
		out = append(out, f.Path+":"+f.Kind)
	}
	return out
}

func TestScanRules(t *testing.T) {
	root := filesPrepFolder(t)
	snap, contents, err := Scan(LocalFolder{Root: root}, prepOutputs, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := paths(snap)
	want := []string{"Código-fonte/Files Prep.py:code", "Código-fonte/requirements.txt:code", "Ferramenta CVM.exe:binary"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if len(contents) != 2 {
		t.Fatalf("contents %d", len(contents))
	}
	if string(contents[snap[0].SHA256]) != "print('oi')\n" {
		t.Fatalf("content %q", contents[snap[0].SHA256])
	}
	if snap[2].SHA256 == "" || snap[2].Size != int64(len("MZ\x00\x01binary")) {
		t.Fatalf("binary fingerprint %+v", snap[2])
	}
}

func TestScanCodeTooBigOrWithNulIsBinary(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, MaxCodeSize+1)
	for i := range big {
		big[i] = 'a'
	}
	write(t, root, "big.py", string(big))
	write(t, root, "nul.py", "a\x00b")
	snap, contents, err := Scan(LocalFolder{Root: root}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 2 || snap[0].Kind != KindBinary || snap[1].Kind != KindBinary || len(contents) != 0 {
		t.Fatalf("%+v %d", snap, len(contents))
	}
	if snap[0].Size != MaxCodeSize+1 {
		t.Fatalf("size %d", snap[0].Size)
	}
}

func TestScanTrustsUnchangedFiles(t *testing.T) {
	root := filesPrepFolder(t)
	first, _, _ := Scan(LocalFolder{Root: root}, prepOutputs, nil)
	// A previous snapshot with a fake hash but the same size and time is trusted (no read).
	prev := append(Snapshot{}, first...)
	prev[0].SHA256 = "trusted"
	again, contents, err := Scan(LocalFolder{Root: root}, prepOutputs, prev)
	if err != nil {
		t.Fatal(err)
	}
	if again[0].SHA256 != "trusted" || len(contents) != 0 {
		t.Fatalf("%+v %d", again[0], len(contents))
	}
}

func TestScanTouchedFileSameContent(t *testing.T) {
	root := filesPrepFolder(t)
	first, _, _ := Scan(LocalFolder{Root: root}, prepOutputs, nil)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "Código-fonte", "Files Prep.py"), later, later); err != nil {
		t.Fatal(err)
	}
	again, _, err := Scan(LocalFolder{Root: root}, prepOutputs, first)
	if err != nil {
		t.Fatal(err)
	}
	if !Same(first, again) {
		t.Fatal("touch without change must be the same version")
	}
}

func TestScanFailsInsteadOfDroppingFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	root := filesPrepFolder(t)
	locked := filepath.Join(root, "Código-fonte")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	if _, _, err := Scan(LocalFolder{Root: root}, prepOutputs, nil); err == nil {
		t.Fatal("an unreadable folder must fail the scan, not look like removed files")
	}
	if _, _, err := Scan(LocalFolder{Root: filepath.Join(root, "nao-existe")}, nil, nil); err == nil {
		t.Fatal("missing folder must fail")
	}
}

func TestChangesAndSummary(t *testing.T) {
	old := Snapshot{
		{Path: "a.py", SHA256: "1", Kind: KindCode},
		{Path: "app.exe", SHA256: "x", Kind: KindBinary, Size: 10},
		{Path: "b.py", SHA256: "2", Kind: KindCode},
	}
	now := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	cur := Snapshot{
		{Path: "a.py", SHA256: "1b", Kind: KindCode, Modified: now},
		{Path: "app.exe", SHA256: "y", Kind: KindBinary, Size: 12, Modified: now.Add(-time.Hour)},
		{Path: "c.py", SHA256: "3", Kind: KindCode, Modified: now.Add(-2 * time.Hour)},
	}
	ch := Changes(old, cur)
	if len(ch) != 4 || ch[0].Path != "a.py" || ch[0].Status != "modified" || ch[2].Path != "b.py" || ch[2].Status != "removed" || ch[3].Status != "added" {
		t.Fatalf("%+v", ch)
	}
	s := Summarize(ch)
	if s.Added != 1 || s.Removed != 1 || s.Modified != 1 || len(s.Binaries) != 1 || s.Binaries[0] != "app.exe" {
		t.Fatalf("%+v", s)
	}
	if at, ok := LastModified(ch); !ok || !at.Equal(now) {
		t.Fatalf("%v %v", at, ok)
	}
	if Same(old, cur) || !Same(cur, append(Snapshot{}, cur...)) {
		t.Fatal("Same")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/versions/'`
Expected: FAIL (package has no Go files / undefined names).

- [ ] **Step 3: Implement `rules.go`**

```go
// Package versions keeps the history of an app's code: what enters a version,
// how a folder is compared with the last version, line diffs and restores.
// It knows nothing about HTTP or the database.
package versions

import (
	"path"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/outputs"
)

const (
	MaxCodeSize = 1 << 20
	TrashDir    = ".app-hub-lixeira"
	KindCode    = "code"
	KindBinary  = "binary"
)

var junkDirs = map[string]bool{"__pycache__": true, ".venv": true, "venv": true, "node_modules": true, ".git": true, TrashDir: true}

// hubFiles are written by the Hub; their changes enter the history through the ficha.
var hubFiles = map[string]bool{"app-hub.yaml": true, "FICHA.md": true}

var codeExts = map[string]bool{
	".py": true, ".pyw": true, ".ipynb": true, ".r": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true,
	".tsx": true, ".jsx": true, ".go": true, ".java": true, ".cs": true, ".rb": true, ".php": true, ".sql": true,
	".bat": true, ".cmd": true, ".ps1": true, ".sh": true, ".vbs": true, ".spec": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true, ".env": true,
	".txt": true, ".md": true, ".html": true, ".htm": true, ".css": true, ".xml": true,
}

// Included reports whether a file (path relative to the app folder, forward
// slashes) belongs in a version.
func Included(rel string, outs []manifest.Output) bool {
	segs := strings.Split(rel, "/")
	for _, s := range segs[:len(segs)-1] {
		if junkDirs[s] {
			return false
		}
	}
	if len(segs) == 1 && hubFiles[rel] {
		return false
	}
	idx, _ := outputs.Match(outs, rel)
	return idx < 0
}

func isCodeName(rel string) bool {
	return codeExts[strings.ToLower(path.Ext(rel))]
}
```

- [ ] **Step 4: Implement `folder.go`**

```go
package versions

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var ErrFolderMissing = errors.New("a pasta do app não foi encontrada ou não pode ser lida")

// Folder is how versions reach an app folder. Today the Hub reads the disk
// directly (WSL); when the Hub moves to a central server, the Agent will
// implement the same interface. Paths are relative, with forward slashes.
type Folder interface {
	// Walk calls fn for every directory (dir=true) and regular file. fn may
	// return fs.SkipDir for a directory. Any read error aborts the walk.
	Walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error
	Open(rel string) (io.ReadCloser, error)
	// Write replaces a file atomically, creating parent folders.
	Write(rel string, data []byte) error
	// Move renames a file, creating the destination's parent folders.
	Move(from, to string) error
}

type LocalFolder struct{ Root string }

func (f LocalFolder) abs(rel string) string { return filepath.Join(f.Root, filepath.FromSlash(rel)) }

func (f LocalFolder) Walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error {
	if info, err := os.Stat(f.Root); err != nil || !info.IsDir() {
		return ErrFolderMissing
	}
	return filepath.WalkDir(f.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == f.Root {
			return nil
		}
		rel, err := filepath.Rel(f.Root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			return fn(rel, true, 0, time.Time{})
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return fn(rel, false, info.Size(), info.ModTime())
	})
}

func (f LocalFolder) Open(rel string) (io.ReadCloser, error) { return os.Open(f.abs(rel)) }

func (f LocalFolder) Write(rel string, data []byte) error {
	p := f.abs(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".app-hub-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (f LocalFolder) Move(from, to string) error {
	dst := f.abs(to)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(f.abs(from), dst)
}
```

- [ ] **Step 5: Implement `snapshot.go`**

```go
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
// files stay in the cloud. It returns, by SHA-256, the contents of the code
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
		if !Included(rel, outs) {
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
```

- [ ] **Step 6: Run tests**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/versions/ -v'`
Expected: PASS. (`TestScanFailsInsteadOfDroppingFiles` may skip as root.)

- [ ] **Step 7: Commit**

```bash
git add backend/internal/versions
git commit -m "Versions: rules, local folder and scan"
```

---

### Task 3: Line diff

**Files:**
- Create: `backend/internal/versions/diff.go`
- Test: `backend/internal/versions/diff_test.go`

**Interfaces:**
- Produces: `type Line struct { Op, Text string; Old, New int }` (JSON `op,text,old,new`; Op is `" "`, `"-"` or `"+"`; line numbers start at 1, 0 when absent), `type Hunk struct { Lines []Line }` (JSON `lines`), `func DiffLines(a, b string, context int) []Hunk`.

- [ ] **Step 1: Write the failing tests**

```go
package versions

import (
	"strings"
	"testing"
)

func render(h []Hunk) string {
	var b strings.Builder
	for i, hk := range h {
		if i > 0 {
			b.WriteString("@@\n")
		}
		for _, l := range hk.Lines {
			b.WriteString(l.Op + l.Text + "\n")
		}
	}
	return b.String()
}

func TestDiffLinesChangeWithContext(t *testing.T) {
	a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	b := "1\n2\n3\n4\nCINCO\n6\n7\n8\n9\n10\n"
	got := render(DiffLines(a, b, 2))
	want := " 3\n 4\n-5\n+CINCO\n 6\n 7\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	h := DiffLines(a, b, 2)
	if h[0].Lines[2].Old != 5 || h[0].Lines[3].New != 5 || h[0].Lines[4].Old != 6 || h[0].Lines[4].New != 6 {
		t.Fatalf("numbers %+v", h[0].Lines)
	}
}

func TestDiffLinesSeparateHunksAndCRLF(t *testing.T) {
	a := "a\r\nb\r\nc\r\nd\r\ne\r\nf\r\ng\r\nh\r\n"
	b := "A\nb\nc\nd\ne\nf\ng\nH\n"
	got := render(DiffLines(a, b, 1))
	want := "-a\n+A\n b\n@@\n g\n-h\n+H\n"
	if got != want {
		t.Fatalf("got\n%s", got)
	}
}

func TestDiffLinesAddedRemovedAndEqual(t *testing.T) {
	if got := render(DiffLines("", "x\ny\n", 3)); got != "+x\n+y\n" {
		t.Fatalf("added %q", got)
	}
	if got := render(DiffLines("x\n", "", 3)); got != "-x\n" {
		t.Fatalf("removed %q", got)
	}
	if h := DiffLines("same\n", "same\n", 3); len(h) != 0 {
		t.Fatalf("equal %v", h)
	}
}

func TestDiffLinesHugeFallsBack(t *testing.T) {
	var a, b strings.Builder
	for i := 0; i < 3000; i++ {
		a.WriteString("a" + strings.Repeat("x", i%7) + "\n")
		b.WriteString("b" + strings.Repeat("y", i%5) + "\n")
	}
	h := DiffLines(a.String(), b.String(), 3)
	minus, plus := 0, 0
	for _, hk := range h {
		for _, l := range hk.Lines {
			if l.Op == "-" {
				minus++
			} else if l.Op == "+" {
				plus++
			}
		}
	}
	if minus != 3000 || plus != 3000 {
		t.Fatalf("%d %d", minus, plus)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/versions/ -run DiffLines'`
Expected: FAIL (`DiffLines` undefined).

- [ ] **Step 3: Implement `diff.go`**

```go
package versions

import "strings"

// Line is one line of a diff: " " kept, "-" removed, "+" added. Old and New
// are 1-based line numbers, 0 when the line does not exist on that side.
type Line struct {
	Op   string `json:"op"`
	Text string `json:"text"`
	Old  int    `json:"old"`
	New  int    `json:"new"`
}

type Hunk struct {
	Lines []Line `json:"lines"`
}

// maxCells bounds the LCS table (int32 cells, ~16 MB). Beyond it the changed
// block is shown as fully removed and fully added.
const maxCells = 4_000_000

// DiffLines compares two texts line by line and returns the changes with
// `context` unchanged lines around them, grouped into hunks.
func DiffLines(a, b string, context int) []Hunk {
	all := diffOps(splitLines(a), splitLines(b))
	keep := make([]bool, len(all))
	for i, l := range all {
		if l.Op == " " {
			continue
		}
		for k := max(0, i-context); k <= min(len(all)-1, i+context); k++ {
			keep[k] = true
		}
	}
	var hunks []Hunk
	open := false
	for i, l := range all {
		if !keep[i] {
			open = false
			continue
		}
		if !open {
			hunks = append(hunks, Hunk{})
			open = true
		}
		last := len(hunks) - 1
		hunks[last].Lines = append(hunks[last].Lines, l)
	}
	return hunks
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func diffOps(a, b []string) []Line {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	var out []Line
	for i := 0; i < pre; i++ {
		out = append(out, Line{Op: " ", Text: a[i], Old: i + 1, New: i + 1})
	}
	out = append(out, middleOps(a[pre:len(a)-suf], b[pre:len(b)-suf], pre)...)
	for i := 0; i < suf; i++ {
		ai, bi := len(a)-suf+i, len(b)-suf+i
		out = append(out, Line{Op: " ", Text: a[ai], Old: ai + 1, New: bi + 1})
	}
	return out
}

func middleOps(a, b []string, off int) []Line {
	n, m := len(a), len(b)
	var out []Line
	if n*m > maxCells {
		for i, s := range a {
			out = append(out, Line{Op: "-", Text: s, Old: off + i + 1})
		}
		for j, s := range b {
			out = append(out, Line{Op: "+", Text: s, New: off + j + 1})
		}
		return out
	}
	// lcs[i][j] is the longest common subsequence of a[i:] and b[j:].
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			out = append(out, Line{Op: " ", Text: a[i], Old: off + i + 1, New: off + j + 1})
			i++
			j++
		case i < n && (j == m || lcs[i+1][j] >= lcs[i][j+1]):
			out = append(out, Line{Op: "-", Text: a[i], Old: off + i + 1})
			i++
		default:
			out = append(out, Line{Op: "+", Text: b[j], New: off + j + 1})
			j++
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/versions/ -v'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/versions/diff.go backend/internal/versions/diff_test.go
git commit -m "Versions: line diff"
```

---

### Task 4: Restore plan and apply

**Files:**
- Create: `backend/internal/versions/restore.go`
- Test: `backend/internal/versions/restore_test.go`

**Interfaces:**
- Consumes: `Snapshot`, `File`, `Folder`, `LocalFolder`, `Scan` (Task 2).
- Produces: `type RestorePlan struct { Rewrite, Recreate, ToTrash, Binaries []string }` (JSON `rewrite,recreate,to_trash,binaries`, never null), `func PlanRestore(current, target Snapshot) RestorePlan`, `type Failure struct { Path, Error string }` (JSON `path,error`), `func ApplyRestore(f Folder, p RestorePlan, target Snapshot, blob func(sha string) ([]byte, error), trash string) (done []string, failed []Failure)`, `func TrashFolder(now time.Time) string`.

- [ ] **Step 1: Write the failing tests**

```go
package versions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanRestore(t *testing.T) {
	target := Snapshot{
		{Path: "a.py", SHA256: "1", Kind: KindCode},
		{Path: "app.exe", SHA256: "old", Kind: KindBinary},
		{Path: "gone.py", SHA256: "2", Kind: KindCode},
		{Path: "same.py", SHA256: "3", Kind: KindCode},
	}
	current := Snapshot{
		{Path: "a.py", SHA256: "1b", Kind: KindCode},
		{Path: "app.exe", SHA256: "new", Kind: KindBinary},
		{Path: "extra.py", SHA256: "4", Kind: KindCode},
		{Path: "same.py", SHA256: "3", Kind: KindCode},
	}
	p := PlanRestore(current, target)
	if len(p.Rewrite) != 1 || p.Rewrite[0] != "a.py" || len(p.Recreate) != 1 || p.Recreate[0] != "gone.py" ||
		len(p.ToTrash) != 1 || p.ToTrash[0] != "extra.py" || len(p.Binaries) != 1 || p.Binaries[0] != "app.exe" {
		t.Fatalf("%+v", p)
	}
	if e := PlanRestore(target, target); len(e.Rewrite)+len(e.Recreate)+len(e.ToTrash)+len(e.Binaries) != 0 || e.Rewrite == nil {
		t.Fatalf("same snapshot plan %+v", e)
	}
}

func TestApplyRestore(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/a.py", "v1\n")
	write(t, root, "src/b.py", "b\n")
	f := LocalFolder{Root: root}
	v1, contents, err := Scan(f, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Change a.py, remove b.py, add c.py.
	write(t, root, "src/a.py", "v2 quebrado\n")
	os.Remove(filepath.Join(root, "src", "b.py"))
	write(t, root, "src/c.py", "novo\n")
	cur, _, _ := Scan(f, nil, v1)

	plan := PlanRestore(cur, v1)
	trash := TrashFolder(time.Date(2026, 10, 1, 15, 30, 0, 0, time.Local))
	if trash != ".app-hub-lixeira/2026-10-01 153000" {
		t.Fatalf("trash %q", trash)
	}
	blob := func(sha string) ([]byte, error) { return contents[sha], nil }
	done, failed := ApplyRestore(f, plan, v1, blob, trash)
	if len(failed) != 0 || len(done) != 3 {
		t.Fatalf("done %v failed %v", done, failed)
	}
	read := func(rel string) string {
		b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return string(b)
	}
	if read("src/a.py") != "v1\n" || read("src/b.py") != "b\n" || read(trash+"/src/c.py") != "novo\n" {
		t.Fatal("files not restored")
	}
	if _, err := os.Stat(filepath.Join(root, "src", "c.py")); !os.IsNotExist(err) {
		t.Fatal("extra file must leave the code folder")
	}
	after, _, _ := Scan(f, nil, nil)
	if !Same(after, v1) {
		t.Fatal("folder must equal the target version (trash is excluded)")
	}
}

func TestApplyRestoreStopsOnFirstFailure(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.py", "1\n")
	write(t, root, "b.py", "2\n")
	f := LocalFolder{Root: root}
	target, _, _ := Scan(f, nil, nil)
	plan := RestorePlan{Rewrite: []string{"a.py", "b.py"}, Recreate: []string{}, ToTrash: []string{}, Binaries: []string{}}
	blob := func(sha string) ([]byte, error) { return nil, errors.New("sem cópia") }
	done, failed := ApplyRestore(f, plan, target, blob, TrashFolder(time.Now()))
	if len(done) != 0 || len(failed) != 2 || failed[0].Path != "a.py" || failed[1].Error != "não foi tentado" {
		t.Fatalf("done %v failed %+v", done, failed)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/versions/ -run Restore'`
Expected: FAIL (undefined `PlanRestore`).

- [ ] **Step 3: Implement `restore.go`**

```go
package versions

import (
	"time"
)

// RestorePlan says what restoring a version does to the folder. Lists are
// never nil, so they encode as [] for the screens.
type RestorePlan struct {
	Rewrite  []string `json:"rewrite"`  // code files that go back to how they were
	Recreate []string `json:"recreate"` // code files that come back
	ToTrash  []string `json:"to_trash"` // code files that did not exist in the target version
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
		case f.Kind == KindBinary && (!ok || c.SHA256 != f.SHA256):
			p.Binaries = append(p.Binaries, f.Path)
		}
	}
	for _, c := range current {
		t, ok := tgt[c.Path]
		switch {
		case c.Kind == KindCode && !ok:
			p.ToTrash = append(p.ToTrash, c.Path)
		case c.Kind == KindBinary && !ok:
			p.Binaries = append(p.Binaries, c.Path)
		case c.Kind == KindBinary && ok && t.Kind == KindCode:
			// Already in Rewrite: the target has it as code.
		}
	}
	return p
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
// files of the target version. It stops at the first failure; the files not
// reached are reported as "não foi tentado".
func ApplyRestore(f Folder, p RestorePlan, target Snapshot, blob func(sha string) ([]byte, error), trash string) (done []string, failed []Failure) {
	sha := map[string]string{}
	for _, t := range target {
		sha[t.Path] = t.SHA256
	}
	type step struct {
		path string
		run  func() error
	}
	var steps []step
	for _, rel := range p.ToTrash {
		steps = append(steps, step{rel, func() error { return f.Move(rel, trash+"/"+rel) }})
	}
	for _, rel := range append(append([]string{}, p.Rewrite...), p.Recreate...) {
		steps = append(steps, step{rel, func() error {
			data, err := blob(sha[rel])
			if err != nil {
				return err
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
```

(Remove the empty third `case` in `PlanRestore` if the linter complains; it only documents intent.)

- [ ] **Step 4: Run tests**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/versions/ -v'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/versions/restore.go backend/internal/versions/restore_test.go
git commit -m "Versions: restore plan and apply with trash"
```

---

### Task 5: Mask secrets and ficha diff

**Files:**
- Modify: `backend/internal/scanner/secrets.go`
- Test: `backend/internal/scanner/secrets_test.go` (create)
- Create: `backend/internal/manifest/diff.go`
- Test: `backend/internal/manifest/diff_test.go`

**Interfaces:**
- Produces: `func scanner.MaskSecrets(text string) string`; `type manifest.Change struct { Field, Label, Before, After string; Long bool }` (JSON `field,label,before,after,long`, `omitempty` on before/after/long), `func manifest.Diff(a, b Manifest) []Change`, `func manifest.Created() []Change`.

- [ ] **Step 1: Write the failing tests**

`backend/internal/scanner/secrets_test.go`:

```go
package scanner

import "testing"

func TestMaskSecrets(t *testing.T) {
	in := "API_KEY = \"abc123def456ghi\"\nchave = os.getenv('X')\nclient = Anthropic(api_key=\"sk-ant-abcdefghijklmnopqrstuvwxyz\")\nprint('oi')\n"
	got := MaskSecrets(in)
	want := "API_KEY = \"••••\"\nchave = os.getenv('X')\nclient = Anthropic(api_key=\"••••\")\nprint('oi')\n"
	if got != want {
		t.Fatalf("got\n%s", got)
	}
	if MaskSecrets("token: \"${TOKEN}\"") != "token: \"${TOKEN}\"" {
		t.Fatal("placeholders stay")
	}
}
```

`backend/internal/manifest/diff_test.go`:

```go
package manifest

import "testing"

func TestDiff(t *testing.T) {
	a := Manifest{Name: "Files Prep", Summary: "Baixa", Area: "Research", Kind: KindDesktop,
		Owner: &PersonRef{Name: "Ana"}, Inputs: []Input{{ID: "x", Label: "X", Type: "text", Source: "window"}}}
	b := a
	b.Area = "Equity"
	b.Owner = &PersonRef{Name: "Henry"}
	b.Summary = "Baixa da CVM"
	b.Inputs = []Input{{ID: "x", Label: "Empresa", Type: "text", Source: "window"}}
	got := Diff(a, b)
	if len(got) != 4 {
		t.Fatalf("%+v", got)
	}
	if got[0].Field != "summary" || !got[0].Long || got[1].Field != "area" || got[1].Before != "Research" || got[1].After != "Equity" ||
		got[2].Label != "Responsável" || got[2].After != "Henry" || got[3].Field != "inputs" || !got[3].Long {
		t.Fatalf("%+v", got)
	}
	// nil and empty lists are the same.
	c := a
	c.Tags = []string{}
	if d := Diff(a, c); len(d) != 0 {
		t.Fatalf("empty vs nil %+v", d)
	}
	if cr := Created(); len(cr) != 1 || cr[0].Label != "Ficha criada" {
		t.Fatalf("%+v", cr)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/scanner/ ./internal/manifest/'`
Expected: FAIL (undefined `MaskSecrets`, `Diff`).

- [ ] **Step 3: Implement `MaskSecrets`** (append to `secrets.go`)

```go
const secretMask = "••••"

// MaskSecrets hides the values findSecrets would report, line by line, so code
// can be shown on screen. Placeholders (os.getenv, ${X}…) stay as they are.
func MaskSecrets(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		ms := secretAssignment.FindAllStringSubmatchIndex(line, -1)
		for k := len(ms) - 1; k >= 0; k-- {
			s, e := ms[k][4], ms[k][5]
			if !looksLikePlaceholder(line[s:e]) {
				line = line[:s] + secretMask + line[e:]
			}
		}
		for _, p := range secretTokenPatterns {
			line = p.re.ReplaceAllString(line, secretMask)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Implement `manifest/diff.go`**

```go
package manifest

import "encoding/json"

// Change is one field of the ficha that changed. Long fields (lists, blocks,
// the summary) appear as "X alterado" with before/after on demand.
type Change struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Long   bool   `json:"long,omitempty"`
}

// Created is the first entry of an app's ficha history.
func Created() []Change { return []Change{{Field: "created", Label: "Ficha criada"}} }

// Diff lists what changed from ficha a to ficha b, in display order.
func Diff(a, b Manifest) []Change {
	var out []Change
	add := func(field, label, x, y string, long bool) {
		if x != y {
			out = append(out, Change{Field: field, Label: label, Before: x, After: y, Long: long})
		}
	}
	add("id", "Identificador", a.ID, b.ID, false)
	add("name", "Nome", a.Name, b.Name, false)
	add("summary", "Resumo", a.Summary, b.Summary, true)
	add("kind", "Tipo", KindLabels[a.Kind], KindLabels[b.Kind], false)
	add("language", "Linguagem", a.Language, b.Language, false)
	add("area", "Área", a.Area, b.Area, false)
	add("owner", "Responsável", personName(a.Owner), personName(b.Owner), false)
	add("backup", "Reserva", personName(a.Backup), personName(b.Backup), false)
	add("entrypoint", "Arquivo que abre o app", a.Entrypoint, b.Entrypoint, false)
	add("schedule", "Agendamento", a.Schedule, b.Schedule, false)
	add("run", "Como roda", jsonText(a.Run), jsonText(b.Run), true)
	add("runtime", "Ambiente", jsonText(a.Runtime), jsonText(b.Runtime), true)
	add("inputs", "Entradas", jsonText(a.Inputs), jsonText(b.Inputs), true)
	add("outputs", "Saídas", jsonText(a.Outputs), jsonText(b.Outputs), true)
	add("secrets", "Chaves e senhas", jsonText(a.Secrets), jsonText(b.Secrets), true)
	add("tags", "Etiquetas", jsonText(a.Tags), jsonText(b.Tags), true)
	add("depends_on", "Depende de", jsonText(a.DependsOn), jsonText(b.DependsOn), true)
	add("build", "Como recompilar", jsonText(a.Build), jsonText(b.Build), true)
	add("notes", "Observações", jsonText(a.Notes), jsonText(b.Notes), true)
	return out
}

func personName(p *PersonRef) string {
	if p == nil {
		return ""
	}
	return p.Name
}

// jsonText renders a block for before/after; nil and empty are both "".
func jsonText(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	switch s := string(b); s {
	case "null", "[]", "{}":
		return ""
	default:
		return s
	}
}
```

Check: `KindLabels` is a `map[string]string` in `manifest.go` (it is: `var KindLabels = map[string]string{`).

- [ ] **Step 5: Run tests**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/scanner/ ./internal/manifest/ -v'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/scanner backend/internal/manifest
git commit -m "Mask secrets for display; ficha field diff"
```

---

### Task 6: Version check wired into ficha, runs and publishing

**Files:**
- Create: `backend/internal/handler/versions.go`
- Modify: `backend/internal/handler/handler.go` (field `locks`, init in `New`)
- Modify: `backend/internal/handler/apps.go` (`appDetail`, `getApp`, `saveApp`)
- Modify: `backend/internal/handler/runs.go` (`startRun`, `runJSON`, `toRunJSON`)
- Test: `backend/internal/handler/versions_test.go`

**Interfaces:**
- Consumes: Task 1 queries; `versions.Scan/Same/Changes/Summarize/LastModified/LocalFolder` (Task 2); `manifest.Diff/Created` (Task 5).
- Produces:
  - `func (h *Handler) checkVersion(ctx context.Context, a sqlc.App) (v sqlc.AppVersion, readable bool, err error)` — takes the per-app lock.
  - `func (h *Handler) checkVersionLocked(ctx context.Context, a sqlc.App) (sqlc.AppVersion, versions.Snapshot, bool, error)` — caller holds the lock; also returns the current snapshot.
  - `func (h *Handler) insertVersion(ctx context.Context, a sqlc.App, prev, snap versions.Snapshot, contents map[string][]byte, origin string, restoredFrom pgtype.Int4, author pgtype.Int8, probable bool) (sqlc.AppVersion, error)`
  - `func (h *Handler) lockApp(id int64) (unlock func())`
  - `func folderOf(a sqlc.App) versions.Folder`
  - `appDetail` gains `Version *versionRef json:"version"` and `FolderReadable bool json:"folder_readable"`; `type versionRef struct { Number int32 json:"number"; CreatedAt time.Time json:"created_at" }`.
  - `runJSON` gains `VersionNumber *int32 json:"version_number"`.

- [ ] **Step 1: Write the failing tests** (`versions_test.go`)

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// codeApp creates a real folder with code and registers it through the API,
// so versão 1 and "Ficha criada" are recorded like in production.
func codeApp(t *testing.T, c *client) (int64, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.py"), []byte("print('v1')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := manifest.Manifest{Name: "App Teste", Summary: "Teste", Kind: manifest.KindScript, Language: "python", Entrypoint: "src/main.py", Area: "Research"}
	var d appDetail
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &d); code != http.StatusCreated {
		t.Fatalf("publish = %d", code)
	}
	return d.ID, dir
}

func editFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute) // make sure size or mtime differ
	_ = os.Chtimes(p, later, later)
}

func TestVersionOneOnPublishAndNewVersionOnChange(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)

	var d appDetail
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version == nil || d.Version.Number != 1 || !d.FolderReadable {
		t.Fatalf("detail %+v", d)
	}
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version.Number != 2 {
		t.Fatalf("after change %+v", d.Version)
	}
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version.Number != 2 {
		t.Fatal("no change must not create a version")
	}
	var origin string
	var probable bool
	var author int64
	testPool.QueryRow(context.Background(), `SELECT origin, author_probable, author_id FROM app_versions WHERE app_id=$1 AND number=2`, id).Scan(&origin, &probable, &author)
	if origin != "folder" || !probable || author != 1 {
		t.Fatalf("%s %v %d", origin, probable, author)
	}
	var raw []byte
	testPool.QueryRow(context.Background(), `SELECT summary FROM app_versions WHERE app_id=$1 AND number=2`, id).Scan(&raw)
	var summary versions.Summary
	if err := json.Unmarshal(raw, &summary); err != nil || summary.Modified != 1 || summary.Added != 0 || len(summary.Binaries) != 0 {
		t.Fatalf("summary %s", raw)
	}
}

func TestCheckVersionIgnoresTouch(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	later := time.Now().Add(time.Hour)
	_ = os.Chtimes(filepath.Join(dir, "src", "main.py"), later, later)
	var d appDetail
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version.Number != 1 {
		t.Fatalf("touch created version %d", d.Version.Number)
	}
}

func TestConcurrentChecksCreateOneVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.do("GET", appPath(id, ""), nil, nil) }()
	}
	wg.Wait()
	var n int
	testPool.QueryRow(context.Background(), `SELECT count(*) FROM app_versions WHERE app_id=$1`, id).Scan(&n)
	if n != 2 {
		t.Fatalf("versions = %d", n)
	}
}

func TestGetAppWithMissingFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest()) // folder /mnt/c/apps/files-prep does not exist
	var d appDetail
	if code := c.do("GET", appPath(app, ""), nil, &d); code != http.StatusOK {
		t.Fatalf("ficha must open: %d", code)
	}
	if d.FolderReadable || d.Version != nil {
		t.Fatalf("%+v", d)
	}
}

func TestRunLinksToVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Ferramenta CVM.exe"), []byte("MZ"), 0o644)
	m := filesPrepManifest()
	m.Area = "Research"
	var d appDetail
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &d); code != http.StatusCreated {
		t.Fatalf("publish %d", code)
	}
	pairComputer(t, c, "PC")
	os.WriteFile(filepath.Join(dir, "Ferramenta CVM.exe"), []byte("MZ2"), 0o644)
	var run runJSON
	if code := c.do("POST", appPath(d.ID, "/runs"), map[string]string{}, &run); code != http.StatusCreated {
		t.Fatalf("run %d", code)
	}
	if run.VersionNumber == nil || *run.VersionNumber != 2 {
		t.Fatalf("version %v", run.VersionNumber)
	}
}

func TestFichaChangesRecorded(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	m := manifest.Manifest{Name: "App Teste", Summary: "Teste", Kind: manifest.KindScript, Language: "python", Entrypoint: "src/main.py", Area: "Equity"}
	c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, nil)
	c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, nil) // no change
	rows, _ := testPool.Query(context.Background(), `SELECT changes FROM app_changes WHERE app_id=$1 ORDER BY id`, id)
	var all []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		all = append(all, s)
	}
	if len(all) != 2 {
		t.Fatalf("changes %v", all)
	}
	var first, second []manifest.Change
	json.Unmarshal([]byte(all[0]), &first)
	json.Unmarshal([]byte(all[1]), &second)
	if first[0].Field != "created" || len(second) != 1 || second[0].Field != "area" || second[0].After != "Equity" {
		t.Fatalf("%+v %+v", first, second)
	}
}
```

Note: `pairComputer` already exists in the handler tests (used by `TestRunLifecycle`); `filesPrepManifest()` has no owner/area, which is why `Area` is set before publishing. If publishing `filesPrepManifest()` fails validation for another field, set that field in the test (read `manifest.Validate`).

- [ ] **Step 2: Run to verify failure**

Run the Go test command with `-run 'Version|Ficha|Concurrent|MissingFolder|RunLinks'`.
Expected: FAIL to compile (`d.Version`, `run.VersionNumber` undefined).

- [ ] **Step 3: Add the lock field** in `handler.go`

In `type Handler struct`, add after `uploads *uploadHub`:

```go
	// One version check per app at a time.
	locksMu sync.Mutex
	locks   map[int64]*sync.Mutex
```

In `New`, add `locks: map[int64]*sync.Mutex{},` to the struct literal and `"sync"` to the imports.

- [ ] **Step 4: Implement `handler/versions.go` (check part)**

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

type versionRef struct {
	Number    int32     `json:"number"`
	CreatedAt time.Time `json:"created_at"`
}

func folderOf(a sqlc.App) versions.Folder { return versions.LocalFolder{Root: a.FolderPath} }

func (h *Handler) lockApp(id int64) func() {
	h.locksMu.Lock()
	m, ok := h.locks[id]
	if !ok {
		m = &sync.Mutex{}
		h.locks[id] = m
	}
	h.locksMu.Unlock()
	m.Lock()
	return m.Unlock
}

// checkVersion compares the app folder with its latest version and records a
// new version when the code changed. readable is false when the folder could
// not be read; then nothing is recorded and v is the latest version (or zero).
func (h *Handler) checkVersion(ctx context.Context, a sqlc.App) (sqlc.AppVersion, bool, error) {
	defer h.lockApp(a.ID)()
	v, _, readable, err := h.checkVersionLocked(ctx, a)
	return v, readable, err
}

func (h *Handler) checkVersionLocked(ctx context.Context, a sqlc.App) (sqlc.AppVersion, versions.Snapshot, bool, error) {
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return sqlc.AppVersion{}, nil, false, err
	}
	latest, err := h.q.LatestAppVersion(ctx, a.ID)
	hasLatest := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AppVersion{}, nil, false, err
	}
	var prev versions.Snapshot
	if hasLatest {
		if err := json.Unmarshal(latest.Files, &prev); err != nil {
			return latest, nil, false, err
		}
	}
	snap, contents, err := versions.Scan(folderOf(a), m.Outputs, prev)
	if err != nil {
		slog.Warn("version check: folder not readable", "app", a.ID, "err", err)
		return latest, prev, false, nil
	}
	if hasLatest && versions.Same(prev, snap) {
		return latest, snap, true, nil
	}
	origin := "folder"
	if !hasLatest {
		origin = "original"
	}
	v, err := h.insertVersion(ctx, a, prev, snap, contents, origin, pgtype.Int4{}, pgtype.Int8{Int64: a.OwnerID, Valid: true}, true)
	return v, snap, true, err
}

func (h *Handler) insertVersion(ctx context.Context, a sqlc.App, prev, snap versions.Snapshot, contents map[string][]byte,
	origin string, restoredFrom pgtype.Int4, author pgtype.Int8, probable bool) (sqlc.AppVersion, error) {
	for sha, data := range contents {
		if err := h.q.PutCodeBlob(ctx, sqlc.PutCodeBlobParams{Sha256: sha, Content: data, Size: int64(len(data))}); err != nil {
			return sqlc.AppVersion{}, err
		}
	}
	changes := versions.Changes(prev, snap)
	files, err := json.Marshal(snap)
	if err != nil {
		return sqlc.AppVersion{}, err
	}
	summary, err := json.Marshal(versions.Summarize(changes))
	if err != nil {
		return sqlc.AppVersion{}, err
	}
	changedAt := pgtype.Timestamptz{}
	if t, ok := versions.LastModified(changes); ok {
		changedAt = pgtype.Timestamptz{Time: t, Valid: true}
	}
	return h.q.InsertAppVersion(ctx, sqlc.InsertAppVersionParams{
		AppID: a.ID, Origin: origin, RestoredFrom: restoredFrom, AuthorID: author, AuthorProbable: probable,
		Files: files, Summary: summary, FilesChangedAt: changedAt,
	})
}
```

- [ ] **Step 5: Wire `getApp`** (`apps.go`)

Add to `appDetail`:

```go
	Version        *versionRef        `json:"version"`
	FolderReadable bool               `json:"folder_readable"`
```

In `getApp`, after `d, err := toAppDetail(a)` succeeds and before `writeJSON`:

```go
	v, readable, err := h.checkVersion(r.Context(), a)
	if err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	d.FolderReadable = readable
	if v.ID != 0 {
		d.Version = &versionRef{Number: v.Number, CreatedAt: v.CreatedAt.Time}
	}
```

(add `"log/slog"` to imports).

- [ ] **Step 6: Wire `saveApp`** (`apps.go`)

Before `a, err := h.q.UpsertApp(...)`:

```go
	old, oldErr := h.q.GetAppByFolder(r.Context(), folder)
	if oldErr != nil && !errors.Is(oldErr, pgx.ErrNoRows) {
		internalError(w, "get app by folder", oldErr)
		return
	}
```

After the upsert succeeds:

```go
	changes := manifest.Created()
	if oldErr == nil {
		var oldM manifest.Manifest
		if err := json.Unmarshal(old.Manifest, &oldM); err != nil {
			internalError(w, "decode old manifest", err)
			return
		}
		changes = manifest.Diff(oldM, m)
	}
	if len(changes) > 0 {
		raw, _ := json.Marshal(changes)
		if err := h.q.InsertAppChange(r.Context(), sqlc.InsertAppChangeParams{AppID: a.ID, PersonID: me.ID, Changes: raw}); err != nil {
			internalError(w, "insert app change", err)
			return
		}
	}
	if _, _, err := h.checkVersion(r.Context(), a); err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
```

- [ ] **Step 7: Wire `startRun` and the run JSON** (`runs.go`)

After the `canRun` check and before `CreateRun`:

```go
	v, _, err := h.checkVersion(ctx, a)
	if err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	versionID := pgtype.Int8{Int64: v.ID, Valid: v.ID != 0}
```

and pass `VersionID: versionID` in `sqlc.CreateRunParams`. Add `VersionNumber *int32 \`json:"version_number"\`` to `runJSON`, and in `toRunJSON`:

```go
	if row.VersionNumber.Valid {
		j.VersionNumber = &row.VersionNumber.Int32
	}
```

(add `"log/slog"` to imports if missing).

- [ ] **Step 8: Run all Go tests**

Expected: PASS, including the old run and publish tests.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/handler
git commit -m "Versions: automatic check on ficha, run and publish; ficha changes recorded"
```

---

### Task 7: Timeline route

**Files:**
- Create: `backend/internal/handler/history.go`
- Modify: `backend/internal/handler/handler.go` (route)
- Test: `backend/internal/handler/history_test.go`

**Interfaces:**
- Consumes: `ListVersionsBefore`, `ListRunsBefore`, `ListChangesBefore`, `LatestAppVersion`, `checkVersion`.
- Produces: `GET /api/apps/{id}/history?type=all|code|runs|ficha&before=<RFC3339Nano>` → `historyJSON{Items []historyItem; More bool; NextBefore string}` (JSON `items,more,next_before`). `historyItem{Type "version"|"run"|"ficha"; At time.Time; Version *versionItem; Run *runItem; Ficha *fichaItem}`. `versionItem{Number int32; Origin string; RestoredFrom *int32; AuthorName string; AuthorProbable bool; Description string; Summary versions.Summary; FilesChangedAt *time.Time; CreatedAt time.Time; Current bool}` (JSON snake_case). `runItem{ID int64; Status string; PersonName string; VersionNumber *int32}`. `fichaItem{PersonName string; Changes []manifest.Change}`. Also `func (h *Handler) appFromPath(w, r) (sqlc.App, bool)` helper (404 handling) used by Tasks 8–9.

- [ ] **Step 1: Write the failing tests**

```go
package handler

import (
	"net/http"
	"testing"
)

func TestHistoryMixesAndPages(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c) // versão 1 + "Ficha criada"
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil) // versão 2

	var hist historyJSON
	if code := c.do("GET", appPath(id, "/history"), nil, &hist); code != http.StatusOK {
		t.Fatalf("history %d", code)
	}
	if len(hist.Items) != 3 || hist.Items[0].Type != "version" || hist.Items[0].Version.Number != 2 || !hist.Items[0].Version.Current ||
		hist.Items[0].Version.AuthorName != "Henry" || !hist.Items[0].Version.AuthorProbable || hist.More {
		t.Fatalf("%+v", hist.Items)
	}
	var code_ historyJSON
	c.do("GET", appPath(id, "/history?type=code"), nil, &code_)
	if len(code_.Items) != 2 {
		t.Fatalf("code filter %+v", code_.Items)
	}
	var ficha historyJSON
	c.do("GET", appPath(id, "/history?type=ficha"), nil, &ficha)
	if len(ficha.Items) != 1 || ficha.Items[0].Ficha.Changes[0].Label != "Ficha criada" {
		t.Fatalf("ficha filter %+v", ficha.Items)
	}

	h.historyPage = 2
	var p1, p2 historyJSON
	c.do("GET", appPath(id, "/history"), nil, &p1)
	if len(p1.Items) != 2 || !p1.More || p1.NextBefore == "" {
		t.Fatalf("page1 %+v", p1)
	}
	c.do("GET", appPath(id, "/history?before="+p1.NextBefore), nil, &p2)
	if len(p2.Items) != 1 || p2.More || p2.Items[0].Type != "version" || p2.Items[0].Version.Number != 1 {
		t.Fatalf("page2 %+v", p2)
	}
}

func TestHistoryRunVisibility(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	c.do("POST", appPath(app, "/runs"), map[string]string{}, nil)
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	var hist historyJSON
	bia.do("GET", appPath(app, "/history?type=runs"), nil, &hist)
	if len(hist.Items) != 0 {
		t.Fatalf("usuario sees others' runs: %+v", hist.Items)
	}
	var mine historyJSON
	c.do("GET", appPath(app, "/history?type=runs"), nil, &mine)
	if len(mine.Items) != 1 || mine.Items[0].Run.PersonName != "Henry" {
		t.Fatalf("%+v", mine.Items)
	}
}

func TestHistoryBadParams(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	if code := c.do("GET", appPath(app, "/history?before=ontem"), nil, nil); code != http.StatusBadRequest {
		t.Fatalf("bad before %d", code)
	}
	if code := c.do("GET", appPath(app, "/history?type=x"), nil, nil); code != http.StatusBadRequest {
		t.Fatalf("bad type %d", code)
	}
	if code := c.do("GET", "/api/apps/999/history", nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing app %d", code)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Expected: FAIL to compile (`historyJSON`, `h.historyPage` undefined).

- [ ] **Step 3: Add `historyPage int` to `Handler`** (default 50 in `New`: `historyPage: 50,`) and the route in `Register`:

```go
	mux.HandleFunc("GET /api/apps/{id}/history", h.appHistory)
```

- [ ] **Step 4: Implement `history.go`**

```go
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

type versionItem struct {
	Number         int32            `json:"number"`
	Origin         string           `json:"origin"`
	RestoredFrom   *int32           `json:"restored_from"`
	AuthorName     string           `json:"author_name"`
	AuthorProbable bool             `json:"author_probable"`
	Description    string           `json:"description"`
	Summary        versions.Summary `json:"summary"`
	FilesChangedAt *time.Time       `json:"files_changed_at"`
	CreatedAt      time.Time        `json:"created_at"`
	Current        bool             `json:"current"`
}

type runItem struct {
	ID            int64  `json:"id"`
	Status        string `json:"status"`
	PersonName    string `json:"person_name"`
	VersionNumber *int32 `json:"version_number"`
}

type fichaItem struct {
	PersonName string            `json:"person_name"`
	Changes    []manifest.Change `json:"changes"`
}

type historyItem struct {
	Type    string       `json:"type"`
	At      time.Time    `json:"at"`
	Version *versionItem `json:"version,omitempty"`
	Run     *runItem     `json:"run,omitempty"`
	Ficha   *fichaItem   `json:"ficha,omitempty"`
}

type historyJSON struct {
	Items      []historyItem `json:"items"`
	More       bool          `json:"more"`
	NextBefore string        `json:"next_before"`
}

func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

func newVersionItem(number int32, origin string, restoredFrom pgtype.Int4, author pgtype.Text, probable bool,
	description string, summary []byte, changedAt, createdAt pgtype.Timestamptz, current int32) versionItem {
	it := versionItem{Number: number, Origin: origin, RestoredFrom: int4Ptr(restoredFrom), AuthorName: author.String,
		AuthorProbable: probable, Description: description, FilesChangedAt: tsPtr(changedAt), CreatedAt: createdAt.Time,
		Current: number == current, Summary: versions.Summary{Binaries: []string{}}}
	_ = json.Unmarshal(summary, &it.Summary)
	return it
}

// appFromPath loads the app named in the URL, answering 404 itself.
func (h *Handler) appFromPath(w http.ResponseWriter, r *http.Request) (sqlc.App, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return sqlc.App{}, false
	}
	a, err := h.q.GetApp(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return sqlc.App{}, false
	}
	if err != nil {
		internalError(w, "get app", err)
		return sqlc.App{}, false
	}
	return a, true
}

func (h *Handler) appHistory(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "all"
	}
	if typ != "all" && typ != "code" && typ != "runs" && typ != "ficha" {
		writeError(w, http.StatusBadRequest, "Filtro desconhecido.")
		return
	}
	before := time.Now().Add(time.Hour)
	if s := r.URL.Query().Get("before"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Não entendi a data para carregar mais.")
			return
		}
		before = t
	}
	a, ok := h.appFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if r.URL.Query().Get("before") == "" {
		if _, _, err := h.checkVersion(ctx, a); err != nil {
			slog.Error("version check", "app", a.ID, "err", err)
		}
	}
	var current int32
	if v, err := h.q.LatestAppVersion(ctx, a.ID); err == nil {
		current = v.Number
	}
	page := h.historyPage
	ts := pgtype.Timestamptz{Time: before, Valid: true}
	lim := int32(page + 1)
	var items []historyItem

	if typ == "all" || typ == "code" {
		rows, err := h.q.ListVersionsBefore(ctx, sqlc.ListVersionsBeforeParams{AppID: a.ID, Before: ts, Lim: lim})
		if err != nil {
			internalError(w, "list versions", err)
			return
		}
		for _, v := range rows {
			it := newVersionItem(v.Number, v.Origin, v.RestoredFrom, v.AuthorName, v.AuthorProbable, v.Description, v.Summary, v.FilesChangedAt, v.CreatedAt, current)
			items = append(items, historyItem{Type: "version", At: v.CreatedAt.Time, Version: &it})
		}
	}
	if typ == "all" || typ == "runs" {
		if err := h.q.ExpireStaleRuns(ctx); err != nil {
			internalError(w, "expire runs", err)
			return
		}
		rows, err := h.q.ListRunsBefore(ctx, sqlc.ListRunsBeforeParams{AppID: a.ID, Before: ts, SeeAll: me.Role == "admin", PersonID: me.ID, Lim: lim})
		if err != nil {
			internalError(w, "list runs", err)
			return
		}
		for _, x := range rows {
			items = append(items, historyItem{Type: "run", At: x.CreatedAt.Time,
				Run: &runItem{ID: x.ID, Status: x.Status, PersonName: x.PersonName, VersionNumber: int4Ptr(x.VersionNumber)}})
		}
	}
	if typ == "all" || typ == "ficha" {
		rows, err := h.q.ListChangesBefore(ctx, sqlc.ListChangesBeforeParams{AppID: a.ID, Before: ts, Lim: lim})
		if err != nil {
			internalError(w, "list changes", err)
			return
		}
		for _, x := range rows {
			f := fichaItem{PersonName: x.PersonName, Changes: []manifest.Change{}}
			_ = json.Unmarshal(x.Changes, &f.Changes)
			items = append(items, historyItem{Type: "ficha", At: x.CreatedAt.Time, Ficha: &f})
		}
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
	out := historyJSON{Items: items}
	if len(items) > page {
		out.Items, out.More = items[:page], true
	}
	if out.Items == nil {
		out.Items = []historyItem{}
	}
	if out.More {
		out.NextBefore = out.Items[len(out.Items)-1].At.Format(time.RFC3339Nano)
	}
	writeJSON(w, http.StatusOK, out)
}
```

Check the generated row types: `ListVersionsBeforeRow.AuthorName` is `pgtype.Text` (LEFT JOIN), `ListRunsBeforeRow.VersionNumber` is `pgtype.Int4`, `ListRunsBeforeRow.PersonName` is `string`. Adjust if sqlc chose differently.

- [ ] **Step 5: Run all Go tests** — Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler
git commit -m "History: timeline of versions, runs and ficha changes"
```

---

### Task 8: Version detail routes (changes, files, content, describe)

**Files:**
- Modify: `backend/internal/handler/versions.go` (append), `handler.go` (routes)
- Test: `backend/internal/handler/versions_detail_test.go`

**Interfaces:**
- Consumes: `appFromPath`, `newVersionItem` (Task 7); `GetAppVersion`, `GetCodeBlob`, `DescribeAppVersion`, `LatestAppVersion`; `versions.Changes/DiffLines`; `scanner.MaskSecrets`.
- Produces routes:
  - `GET /api/apps/{id}/versions/{n}` → `versionItem`
  - `GET /api/apps/{id}/versions/{n}/changes` → `{version: versionItem, files: []fileChangeJSON}`; `fileChangeJSON{Path, Status, Kind string; OldSize, NewSize *int64; Hunks []versions.Hunk}` (JSON `path,status,kind,old_size,new_size,hunks`)
  - `GET /api/apps/{id}/versions/{n}/files` → `[]versionFileJSON{Path string; Size int64; Kind string; Modified time.Time}`
  - `GET /api/apps/{id}/versions/{n}/content?path=` → `{path, text}`
  - `PATCH /api/apps/{id}/versions/{n}` body `{description string, author_id *int64}` → `versionItem`
  - helper `func (h *Handler) versionFromPath(w, r) (sqlc.App, sqlc.GetAppVersionRow, versions.Snapshot, bool)`

- [ ] **Step 1: Write the failing tests**

```go
package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestVersionChanges(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "API_KEY = \"abc123def456ghi\"\nprint('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)

	var res struct {
		Version versionItem      `json:"version"`
		Files   []fileChangeJSON `json:"files"`
	}
	if code := c.do("GET", appPath(id, "/versions/2/changes"), nil, &res); code != http.StatusOK {
		t.Fatalf("%d", code)
	}
	if res.Version.Number != 2 || len(res.Files) != 1 || res.Files[0].Status != "modified" || res.Files[0].Kind != "code" {
		t.Fatalf("%+v", res)
	}
	var text strings.Builder
	for _, hk := range res.Files[0].Hunks {
		for _, l := range hk.Lines {
			text.WriteString(l.Op + l.Text + "\n")
		}
	}
	if text.String() != "-print('v1')\n+API_KEY = \"••••\"\n+print('v2')\n" {
		t.Fatalf("diff\n%s", text.String())
	}
	var v1 struct {
		Files []fileChangeJSON `json:"files"`
	}
	c.do("GET", appPath(id, "/versions/1/changes"), nil, &v1)
	if len(v1.Files) != 1 || v1.Files[0].Status != "added" {
		t.Fatalf("v1 %+v", v1.Files)
	}
	if code := c.do("GET", appPath(id, "/versions/9/changes"), nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing version %d", code)
	}
}

func TestVersionFileContent(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "token = \"abcdefghijklmnop\"\n")
	c.do("GET", appPath(id, ""), nil, nil)

	var files []versionFileJSON
	c.do("GET", appPath(id, "/versions/1/files"), nil, &files)
	if len(files) != 1 || files[0].Path != "src/main.py" || files[0].Kind != "code" {
		t.Fatalf("%+v", files)
	}
	var got struct{ Path, Text string }
	c.do("GET", appPath(id, "/versions/1/content?path="+url.QueryEscape("src/main.py")), nil, &got)
	if got.Text != "print('v1')\n" {
		t.Fatalf("v1 content %q", got.Text)
	}
	c.do("GET", appPath(id, "/versions/2/content?path="+url.QueryEscape("src/main.py")), nil, &got)
	if got.Text != "token = \"••••\"\n" {
		t.Fatalf("masked %q", got.Text)
	}
	for _, bad := range []string{"../etc/passwd", "/etc/passwd", "nao-existe.py"} {
		if code := c.do("GET", appPath(id, "/versions/1/content?path="+url.QueryEscape(bad)), nil, nil); code != http.StatusNotFound {
			t.Fatalf("%s = %d", bad, code)
		}
	}
}

func TestDescribeVersion(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, _ := codeApp(t, c)
	createPersonAs(t, c, "Ana", "ana@alis.com", "criador")
	const anaID = 2 // setupAdmin is person 1 (RESTART IDENTITY)
	var v versionItem
	body := map[string]any{"description": "  Primeira versão  ", "author_id": anaID}
	if code := c.do("PATCH", appPath(id, "/versions/1"), body, &v); code != http.StatusOK {
		t.Fatalf("%d", code)
	}
	if v.Description != "Primeira versão" || v.AuthorName != "Ana" || v.AuthorProbable {
		t.Fatalf("%+v", v)
	}
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	if code := bia.do("PATCH", appPath(id, "/versions/1"), body, nil); code != http.StatusForbidden {
		t.Fatalf("usuario describe %d", code)
	}
	if code := c.do("PATCH", appPath(id, "/versions/1"), map[string]any{"description": strings.Repeat("x", 2001)}, nil); code != http.StatusBadRequest {
		t.Fatalf("too long %d", code)
	}
}
```

`requireRole` answers 403 "Você não tem permissão para isso." for a wrong role. Helpers `createPersonAs` and `loginAs` live in `agents_test.go` (password `senha-forte-1`).

- [ ] **Step 2: Run to verify failure** — Expected: FAIL to compile.

- [ ] **Step 3: Routes** in `Register`:

```go
	mux.HandleFunc("GET /api/apps/{id}/versions/{n}", h.getVersion)
	mux.HandleFunc("PATCH /api/apps/{id}/versions/{n}", h.describeVersion)
	mux.HandleFunc("GET /api/apps/{id}/versions/{n}/changes", h.versionChanges)
	mux.HandleFunc("GET /api/apps/{id}/versions/{n}/files", h.versionFiles)
	mux.HandleFunc("GET /api/apps/{id}/versions/{n}/content", h.versionContent)
```

- [ ] **Step 4: Implement** (append to `handler/versions.go`; add imports `net/http`, `strconv`, `strings`, `github.com/HenryKropp/app-hub/backend/internal/scanner`)

```go
type fileChangeJSON struct {
	Path    string           `json:"path"`
	Status  string           `json:"status"`
	Kind    string           `json:"kind"`
	OldSize *int64           `json:"old_size"`
	NewSize *int64           `json:"new_size"`
	Hunks   []versions.Hunk  `json:"hunks"`
}

type versionFileJSON struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Kind     string    `json:"kind"`
	Modified time.Time `json:"modified"`
}

func (h *Handler) versionFromPath(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.GetAppVersionRow, versions.Snapshot, bool) {
	a, ok := h.appFromPath(w, r)
	if !ok {
		return a, sqlc.GetAppVersionRow{}, nil, false
	}
	n, err := strconv.ParseInt(r.PathValue("n"), 10, 32)
	if err != nil {
		writeError(w, http.StatusNotFound, "Versão não encontrada.")
		return a, sqlc.GetAppVersionRow{}, nil, false
	}
	v, err := h.q.GetAppVersion(r.Context(), sqlc.GetAppVersionParams{AppID: a.ID, Number: int32(n)})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Versão não encontrada.")
		return a, v, nil, false
	}
	if err != nil {
		internalError(w, "get version", err)
		return a, v, nil, false
	}
	var snap versions.Snapshot
	if err := json.Unmarshal(v.Files, &snap); err != nil {
		internalError(w, "decode version files", err)
		return a, v, nil, false
	}
	return a, v, snap, true
}

func (h *Handler) versionItemOf(r *http.Request, a sqlc.App, v sqlc.GetAppVersionRow) versionItem {
	var current int32
	if latest, err := h.q.LatestAppVersion(r.Context(), a.ID); err == nil {
		current = latest.Number
	}
	return newVersionItem(v.Number, v.Origin, v.RestoredFrom, v.AuthorName, v.AuthorProbable, v.Description, v.Summary, v.FilesChangedAt, v.CreatedAt, current)
}

func (h *Handler) getVersion(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	a, v, _, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.versionItemOf(r, a, v))
}

// codeText returns the masked text of a code file, or "" for binaries/absent files.
func (h *Handler) codeText(r *http.Request, f *versions.File) (string, error) {
	if f == nil || f.Kind != versions.KindCode {
		return "", nil
	}
	data, err := h.q.GetCodeBlob(r.Context(), f.SHA256)
	if err != nil {
		return "", err
	}
	return scanner.MaskSecrets(string(data)), nil
}

func (h *Handler) versionChanges(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	a, v, snap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	var prev versions.Snapshot
	if v.Number > 1 {
		pv, err := h.q.GetAppVersion(r.Context(), sqlc.GetAppVersionParams{AppID: a.ID, Number: v.Number - 1})
		if err != nil {
			internalError(w, "get previous version", err)
			return
		}
		if err := json.Unmarshal(pv.Files, &prev); err != nil {
			internalError(w, "decode previous files", err)
			return
		}
	}
	files := []fileChangeJSON{}
	for _, c := range versions.Changes(prev, snap) {
		fc := fileChangeJSON{Path: c.Path, Status: c.Status, Hunks: []versions.Hunk{}}
		if c.Old != nil {
			fc.OldSize, fc.Kind = &c.Old.Size, c.Old.Kind
		}
		if c.New != nil {
			fc.NewSize, fc.Kind = &c.New.Size, c.New.Kind
		}
		before, err := h.codeText(r, c.Old)
		if err != nil {
			internalError(w, "get blob", err)
			return
		}
		after, err := h.codeText(r, c.New)
		if err != nil {
			internalError(w, "get blob", err)
			return
		}
		if fc.Kind == versions.KindCode {
			fc.Hunks = versions.DiffLines(before, after, 3)
		}
		files = append(files, fc)
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": h.versionItemOf(r, a, v), "files": files})
}

func (h *Handler) versionFiles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	_, _, snap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	out := make([]versionFileJSON, 0, len(snap))
	for _, f := range snap {
		out = append(out, versionFileJSON{Path: f.Path, Size: f.Size, Kind: f.Kind, Modified: f.Modified})
	}
	writeJSON(w, http.StatusOK, out)
}

// versionContent only serves paths listed in the version, from the stored
// copy; it never reads the disk.
func (h *Handler) versionContent(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	_, _, snap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	rel := r.URL.Query().Get("path")
	for i := range snap {
		if snap[i].Path != rel {
			continue
		}
		if snap[i].Kind != versions.KindCode {
			writeError(w, http.StatusBadRequest, "Este arquivo não é texto. O Hub guarda só o tamanho e a data dele.")
			return
		}
		text, err := h.codeText(r, &snap[i])
		if err != nil {
			internalError(w, "get blob", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"path": rel, "text": text})
		return
	}
	writeError(w, http.StatusNotFound, "Arquivo não encontrado nesta versão.")
}

func (h *Handler) describeVersion(w http.ResponseWriter, r *http.Request) {
	me, ok := h.requireRole(w, r, "admin", "criador")
	if !ok {
		return
	}
	var body struct {
		Description string `json:"description"`
		AuthorID    *int64 `json:"author_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	desc := strings.TrimSpace(body.Description)
	if len([]rune(desc)) > 2000 {
		writeError(w, http.StatusBadRequest, "A descrição pode ter até 2000 caracteres.")
		return
	}
	a, v, _, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	author := pgtype.Int8{}
	if body.AuthorID != nil && *body.AuthorID != 0 {
		if _, err := h.q.GetPerson(r.Context(), *body.AuthorID); err != nil {
			writeError(w, http.StatusBadRequest, "Pessoa não encontrada.")
			return
		}
		author = pgtype.Int8{Int64: *body.AuthorID, Valid: true}
	}
	if err := h.q.DescribeAppVersion(r.Context(), sqlc.DescribeAppVersionParams{
		Description: desc, AuthorID: author, DescribedBy: pgtype.Int8{Int64: me.ID, Valid: true}, AppID: a.ID, Number: v.Number,
	}); err != nil {
		internalError(w, "describe version", err)
		return
	}
	v, err := h.q.GetAppVersion(r.Context(), sqlc.GetAppVersionParams{AppID: a.ID, Number: v.Number})
	if err != nil {
		internalError(w, "get version", err)
		return
	}
	writeJSON(w, http.StatusOK, h.versionItemOf(r, a, v))
}
```

`DescribedBy` type depends on sqlc (`described_by` is nullable BIGINT; `sqlc.arg` on a nullable column usually yields `pgtype.Int8`). Use the generated type.

- [ ] **Step 5: Run all Go tests** — Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler
git commit -m "Versions: see changes, see as it was, describe"
```

---

### Task 9: Restore routes

**Files:**
- Create: `backend/internal/handler/restore.go`
- Modify: `backend/internal/handler/handler.go` (routes)
- Test: `backend/internal/handler/restore_test.go`

**Interfaces:**
- Consumes: `versionFromPath` (Task 8), `checkVersionLocked`, `insertVersion`, `lockApp`, `folderOf` (Task 6), `versions.PlanRestore/ApplyRestore/TrashFolder/Scan` (Tasks 2, 4), `AppHasActiveRun`, `ExpireStaleRuns`, `GetCodeBlob`.
- Produces: `GET /api/apps/{id}/versions/{n}/restore-preview` → `{plan: versions.RestorePlan, current: int32}`; `POST /api/apps/{id}/versions/{n}/restore` → `restoreJSON{Number int32; Done []string; Failed []versions.Failure}` (JSON `number,done,failed`).

- [ ] **Step 1: Write the failing tests**

```go
package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func TestRestoreFlow(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('quebrado')\n")
	os.WriteFile(filepath.Join(dir, "src", "extra.py"), []byte("x\n"), 0o644)
	// Not checked yet: the preview must record the current state first (versão 2).

	var prev struct {
		Plan    versions.RestorePlan `json:"plan"`
		Current int32                `json:"current"`
	}
	if code := c.do("GET", appPath(id, "/versions/1/restore-preview"), nil, &prev); code != http.StatusOK {
		t.Fatalf("preview %d", code)
	}
	if prev.Current != 2 || len(prev.Plan.Rewrite) != 1 || len(prev.Plan.ToTrash) != 1 || prev.Plan.ToTrash[0] != "src/extra.py" {
		t.Fatalf("%+v", prev)
	}

	var res restoreJSON
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &res); code != http.StatusOK {
		t.Fatalf("restore %d", code)
	}
	if res.Number != 3 || len(res.Failed) != 0 || len(res.Done) != 2 {
		t.Fatalf("%+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "src", "main.py"))
	if string(b) != "print('v1')\n" {
		t.Fatalf("main.py = %q", b)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".app-hub-lixeira", "*", "src", "extra.py"))
	if len(matches) != 1 {
		t.Fatal("extra.py must be in the trash")
	}
	var origin string
	var from int32
	var probable bool
	testPool.QueryRow(context.Background(), `SELECT origin, restored_from, author_probable FROM app_versions WHERE app_id=$1 AND number=3`, id).Scan(&origin, &from, &probable)
	if origin != "restore" || from != 1 || probable {
		t.Fatalf("%s %d %v", origin, from, probable)
	}
	// Opening the ficha afterwards does not create another version (trash is excluded).
	var d appDetail
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version.Number != 3 {
		t.Fatalf("after restore %d", d.Version.Number)
	}
	if code := c.do("POST", appPath(id, "/versions/3/restore"), nil, nil); code != http.StatusBadRequest {
		t.Fatalf("restore current %d", code)
	}
}

func TestRestoreRequiresRole(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	if code := bia.do("POST", appPath(id, "/versions/1/restore"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("usuario restore %d", code)
	}
	if code := bia.do("GET", appPath(id, "/versions/1/restore-preview"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("usuario preview %d", code)
	}
}

func TestRestoreBlockedWhileRunning(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)
	testPool.Exec(context.Background(), `INSERT INTO computers (person_id, name, token_hash) VALUES (1, 'PC', 'h')`)
	testPool.Exec(context.Background(), `INSERT INTO runs (app_id, person_id, computer_id, output_folder, status, started_at) VALUES ($1, 1, 1, 'C:\x', 'running', now())`, id)
	var e map[string]string
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &e); code != http.StatusConflict || e["error"] != "Espere a execução terminar antes de voltar o código." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestRestoreStopsOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)
	src := filepath.Join(dir, "src")
	os.Chmod(src, 0o555) // cannot create the temp file next to main.py
	defer os.Chmod(src, 0o755)
	var res restoreJSON
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &res); code != http.StatusOK {
		t.Fatalf("restore %d", code)
	}
	if len(res.Failed) != 1 || res.Failed[0].Path != "src/main.py" || res.Number != 3 {
		t.Fatalf("%+v", res)
	}
}

func TestRestoreFolderMissing(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("GET", appPath(id, ""), nil, nil)
	os.RemoveAll(dir)
	var e map[string]string
	if code := c.do("POST", appPath(id, "/versions/1/restore"), nil, &e); code != http.StatusConflict {
		t.Fatalf("%d %v", code, e)
	}
}
```

Check `ExpireStaleRuns` (in `agents.sql`/`runs.sql`) does not expire a run started `now()`; if it does, use the status `waiting` with a fresh `created_at` instead.

- [ ] **Step 2: Run to verify failure** — Expected: FAIL to compile.

- [ ] **Step 3: Routes**

```go
	mux.HandleFunc("GET /api/apps/{id}/versions/{n}/restore-preview", h.restorePreview)
	mux.HandleFunc("POST /api/apps/{id}/versions/{n}/restore", h.restoreVersion)
```

- [ ] **Step 4: Implement `restore.go`**

```go
package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

type restoreJSON struct {
	Number int32              `json:"number"`
	Done   []string           `json:"done"`
	Failed []versions.Failure `json:"failed"`
}

// prepareRestore runs the checks shared by preview and restore, with the app
// lock held by the caller: role, no active run, folder readable, not the
// current version. It records the current state as a version (safety copy).
func (h *Handler) prepareRestore(w http.ResponseWriter, r *http.Request, a sqlc.App, target sqlc.GetAppVersionRow) (sqlc.AppVersion, versions.Snapshot, bool) {
	ctx := r.Context()
	if err := h.q.ExpireStaleRuns(ctx); err != nil {
		internalError(w, "expire runs", err)
		return sqlc.AppVersion{}, nil, false
	}
	active, err := h.q.AppHasActiveRun(ctx, a.ID)
	if err != nil {
		internalError(w, "active run", err)
		return sqlc.AppVersion{}, nil, false
	}
	if active {
		writeError(w, http.StatusConflict, "Espere a execução terminar antes de voltar o código.")
		return sqlc.AppVersion{}, nil, false
	}
	cur, snap, readable, err := h.checkVersionLocked(ctx, a)
	if err != nil {
		internalError(w, "version check", err)
		return cur, nil, false
	}
	if !readable {
		writeError(w, http.StatusConflict, "Não consegui ler a pasta do app. Confira se ela existe e se o OneDrive está sincronizado.")
		return cur, nil, false
	}
	if cur.Number == target.Number {
		writeError(w, http.StatusBadRequest, "Esta já é a versão atual.")
		return cur, nil, false
	}
	return cur, snap, true
}

func (h *Handler) restorePreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireRole(w, r, "admin", "criador"); !ok {
		return
	}
	a, target, targetSnap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	defer h.lockApp(a.ID)()
	cur, snap, ok := h.prepareRestore(w, r, a, target)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": versions.PlanRestore(snap, targetSnap), "current": cur.Number})
}

func (h *Handler) restoreVersion(w http.ResponseWriter, r *http.Request) {
	me, ok := h.requireRole(w, r, "admin", "criador")
	if !ok {
		return
	}
	a, target, targetSnap, ok := h.versionFromPath(w, r)
	if !ok {
		return
	}
	defer h.lockApp(a.ID)()
	_, snap, ok := h.prepareRestore(w, r, a, target)
	if !ok {
		return
	}
	ctx := r.Context()
	plan := versions.PlanRestore(snap, targetSnap)
	blob := func(sha string) ([]byte, error) { return h.q.GetCodeBlob(ctx, sha) }
	folder := folderOf(a)
	done, failed := versions.ApplyRestore(folder, plan, targetSnap, blob, versions.TrashFolder(time.Now()))

	var m manifestOutputs
	if err := m.load(a); err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	after, contents, err := versions.Scan(folder, m.Outputs, snap)
	if err != nil {
		internalError(w, "scan after restore", err)
		return
	}
	v, err := h.insertVersion(ctx, a, snap, after, contents, "restore",
		pgtype.Int4{Int32: target.Number, Valid: true}, pgtype.Int8{Int64: me.ID, Valid: true}, false)
	if err != nil {
		internalError(w, "insert restore version", err)
		return
	}
	if failed == nil {
		failed = []versions.Failure{}
	}
	writeJSON(w, http.StatusOK, restoreJSON{Number: v.Number, Done: done, Failed: failed})
}
```

Add to `handler/versions.go` the tiny helper used above (keeps manifest decoding in one place):

```go
type manifestOutputs struct{ Outputs []manifest.Output }

func (m *manifestOutputs) load(a sqlc.App) error {
	var full manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &full); err != nil {
		return err
	}
	m.Outputs = full.Outputs
	return nil
}
```

- [ ] **Step 5: Run all Go tests** — Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler
git commit -m "Versions: restore code with preview, safety version and trash"
```

---

### Task 10: Frontend — API, timeline on the ficha, version on run page

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Create: `frontend/src/components/HistoryTimeline.tsx`, `frontend/src/components/DescribeDialog.tsx`, `frontend/src/components/RestoreDialog.tsx`
- Modify: `frontend/src/pages/AppPage.tsx`, `frontend/src/pages/RunPage.tsx`
- Delete: `frontend/src/components/RunsList.tsx` (replaced by the timeline; `api.appRuns` stays)
- Modify: `frontend/src/test/fixtures.ts` (`run()` gets `version_number: null`)
- Test: `frontend/src/components/HistoryTimeline.test.tsx`; update `frontend/src/pages/AppPage.test.tsx`

**Interfaces:**
- Produces (TS):

```ts
export type VersionSummary = { added: number; removed: number; modified: number; binaries: string[] | null }
export type VersionInfo = {
  number: number; origin: "original" | "folder" | "restore"; restored_from: number | null
  author_name: string; author_probable: boolean; description: string; summary: VersionSummary
  files_changed_at: string | null; created_at: string; current: boolean
}
export type FichaChange = { field: string; label: string; before?: string; after?: string; long?: boolean }
export type HistoryItem = {
  type: "version" | "run" | "ficha"; at: string
  version?: VersionInfo
  run?: { id: number; status: RunStatus; person_name: string; version_number: number | null }
  ficha?: { person_name: string; changes: FichaChange[] }
}
export type HistoryFilter = "all" | "code" | "runs" | "ficha"
export type History = { items: HistoryItem[]; more: boolean; next_before: string }
export type DiffLine = { op: " " | "-" | "+"; text: string; old: number; new: number }
export type FileChange = { path: string; status: "added" | "removed" | "modified"; kind: "code" | "binary"; old_size: number | null; new_size: number | null; hunks: { lines: DiffLine[] }[] }
export type VersionFile = { path: string; size: number; kind: "code" | "binary"; modified: string }
export type RestorePlan = { rewrite: string[]; recreate: string[]; to_trash: string[]; binaries: string[] }
export type RestoreResult = { number: number; done: string[]; failed: { path: string; error: string }[] }
```

  `AppDetail` gains `version: { number: number; created_at: string } | null; folder_readable: boolean`. `Run` gains `version_number: number | null`.
  `api.history(appId, filter, before?)`, `api.version(appId, n)`, `api.versionChanges(appId, n)`, `api.versionFiles(appId, n)`, `api.versionContent(appId, n, path)`, `api.describeVersion(appId, n, { description, author_id })`, `api.restorePreview(appId, n)`, `api.restore(appId, n)`.
  Components: `<HistoryTimeline appId canEdit />`, `<DescribeDialog appId version open onOpenChange onSaved />`, `<RestoreDialog appId number open onOpenChange onDone />`; helper `versionLine(v: VersionInfo): string`, exported from `HistoryTimeline.tsx`.

Before writing UI code, invoke the `cofounder:frontend-design` skill and follow the existing look (paper card, `font-mono` uppercase section titles, `text-stamp`, `text-warn`, lucide icons).

- [ ] **Step 1: API additions** (`api.ts`): add the types above; add the two fields to `AppDetail` and `version_number` to `Run`; add to `api`:

```ts
  history: (appId: number | string, filter: HistoryFilter, before?: string) =>
    request<History>("GET", `/api/apps/${appId}/history?type=${filter}${before ? `&before=${encodeURIComponent(before)}` : ""}`),
  version: (appId: number | string, n: number | string) => request<VersionInfo>("GET", `/api/apps/${appId}/versions/${n}`),
  versionChanges: (appId: number | string, n: number | string) =>
    request<{ version: VersionInfo; files: FileChange[] }>("GET", `/api/apps/${appId}/versions/${n}/changes`),
  versionFiles: (appId: number | string, n: number | string) => request<VersionFile[]>("GET", `/api/apps/${appId}/versions/${n}/files`),
  versionContent: (appId: number | string, n: number | string, path: string) =>
    request<{ path: string; text: string }>("GET", `/api/apps/${appId}/versions/${n}/content?path=${encodeURIComponent(path)}`),
  describeVersion: (appId: number, n: number, b: { description: string; author_id: number | null }) =>
    request<VersionInfo>("PATCH", `/api/apps/${appId}/versions/${n}`, b),
  restorePreview: (appId: number, n: number) => request<{ plan: RestorePlan; current: number }>("GET", `/api/apps/${appId}/versions/${n}/restore-preview`),
  restore: (appId: number, n: number) => request<RestoreResult>("POST", `/api/apps/${appId}/versions/${n}/restore`),
```

- [ ] **Step 2: Write the failing timeline test** (`HistoryTimeline.test.tsx`)

```tsx
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type HistoryItem, type VersionInfo } from "@/lib/api"
import { HistoryTimeline } from "./HistoryTimeline"

afterEach(() => vi.restoreAllMocks())

const v = (over: Partial<VersionInfo> = {}): VersionInfo => ({
  number: 2, origin: "folder", restored_from: null, author_name: "Henry Kropp", author_probable: true, description: "",
  summary: { added: 1, removed: 0, modified: 2, binaries: ["Ferramenta CVM.exe"] }, files_changed_at: "2026-10-01T12:00:00Z",
  created_at: "2026-10-01T12:05:00Z", current: true, ...over,
})

const items: HistoryItem[] = [
  { type: "version", at: "2026-10-01T12:05:00Z", version: v() },
  { type: "run", at: "2026-10-01T11:00:00Z", run: { id: 7, status: "done", person_name: "Ana", version_number: 1 } },
  { type: "ficha", at: "2026-10-01T10:00:00Z", ficha: { person_name: "Henry", changes: [{ field: "owner", label: "Responsável", before: "Ana", after: "Henry" }, { field: "summary", label: "Resumo", before: "a", after: "b", long: true }] } },
  { type: "version", at: "2026-10-01T09:00:00Z", version: v({ number: 1, origin: "original", current: false, summary: { added: 3, removed: 0, modified: 0, binaries: [] } }) },
]

const renderIt = (canEdit = true) =>
  render(
    <MemoryRouter>
      <HistoryTimeline appId={3} canEdit={canEdit} />
    </MemoryRouter>,
  )

test("shows the three kinds of item in one list", async () => {
  vi.spyOn(api, "history").mockResolvedValue({ items, more: false, next_before: "" })
  renderIt()
  const list = await screen.findByRole("list", { name: "Histórico" })
  const rows = within(list).getAllByRole("listitem")
  expect(rows).toHaveLength(4)
  expect(rows[0]).toHaveTextContent("Versão 2")
  expect(rows[0]).toHaveTextContent("Mudança na pasta")
  expect(rows[0]).toHaveTextContent("provável: Henry Kropp")
  expect(rows[0]).toHaveTextContent("2 arquivos mudaram, 1 novo · Ferramenta CVM.exe mudou")
  expect(rows[1]).toHaveTextContent("Concluída")
  expect(rows[1]).toHaveTextContent("versão 1")
  expect(within(rows[1]).getByRole("link")).toHaveAttribute("href", "/execucoes/7")
  expect(rows[2]).toHaveTextContent("Responsável: Ana → Henry")
  expect(rows[2]).toHaveTextContent("Resumo alterado")
  expect(rows[3]).toHaveTextContent("Estado original")
  // Current version has no restore button; older one has.
  expect(within(rows[0]).queryByRole("button", { name: /Voltar o código/ })).toBeNull()
  expect(within(rows[3]).getByRole("button", { name: /Voltar o código/ })).toBeInTheDocument()
  expect(within(rows[0]).getByRole("link", { name: "Ver mudanças" })).toHaveAttribute("href", "/apps/3/versoes/2")
  expect(within(rows[0]).getByRole("link", { name: "Ver como estava" })).toHaveAttribute("href", "/apps/3/versoes/2?ver=arquivos")
})

test("filter and load more", async () => {
  const user = userEvent.setup()
  const spy = vi.spyOn(api, "history")
    .mockResolvedValueOnce({ items: items.slice(0, 2), more: true, next_before: "2026-10-01T11:00:00Z" })
    .mockResolvedValueOnce({ items: items.slice(2), more: false, next_before: "" })
    .mockResolvedValueOnce({ items: [items[0], items[3]], more: false, next_before: "" })
  renderIt()
  await user.click(await screen.findByRole("button", { name: "Carregar mais" }))
  expect(spy).toHaveBeenLastCalledWith(3, "all", "2026-10-01T11:00:00Z")
  expect(await screen.findAllByRole("listitem")).toHaveLength(4)
  await user.click(screen.getByRole("button", { name: "Código" }))
  expect(spy).toHaveBeenLastCalledWith(3, "code", undefined)
})

test("people without edit rights see no Descrever or Voltar", async () => {
  vi.spyOn(api, "history").mockResolvedValue({ items, more: false, next_before: "" })
  renderIt(false)
  await screen.findByRole("list", { name: "Histórico" })
  expect(screen.queryByRole("button", { name: "Descrever" })).toBeNull()
  expect(screen.queryByRole("button", { name: /Voltar o código/ })).toBeNull()
})
```

- [ ] **Step 3: Run to verify failure**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/components/HistoryTimeline.test.tsx'`
Expected: FAIL (module not found).

- [ ] **Step 4: Implement `HistoryTimeline.tsx`**

```tsx
import { useCallback, useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { FileCode2, PencilLine, Play, RotateCcw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DescribeDialog } from "@/components/DescribeDialog"
import { RestoreDialog } from "@/components/RestoreDialog"
import { api, type HistoryFilter, type HistoryItem, type VersionInfo } from "@/lib/api"
import { formatWhen, STATUS } from "@/lib/runs"

const FILTERS: { value: HistoryFilter; label: string }[] = [
  { value: "all", label: "Tudo" },
  { value: "code", label: "Código" },
  { value: "runs", label: "Execuções" },
  { value: "ficha", label: "Ficha" },
]

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

// eslint-disable-next-line react-refresh/only-export-components
export function versionLine(v: VersionInfo): string {
  const parts: string[] = []
  if (v.summary.modified) parts.push(`${plural(v.summary.modified, "arquivo mudou", "arquivos mudaram")}`)
  if (v.summary.added) parts.push(plural(v.summary.added, "novo", "novos"))
  if (v.summary.removed) parts.push(plural(v.summary.removed, "removido", "removidos"))
  const code = parts.join(", ")
  const bins = (v.summary.binaries ?? []).map((b) => `${b} mudou`)
  return [code, ...bins].filter(Boolean).join(" · ") || "Sem mudança no código"
}

// eslint-disable-next-line react-refresh/only-export-components
export function versionOrigin(v: VersionInfo): string {
  if (v.origin === "original") return "Estado original"
  if (v.origin === "restore") return `Restaurada da versão ${v.restored_from}`
  return "Mudança na pasta"
}

function VersionRow({ appId, v, canEdit, onChanged }: { appId: number; v: VersionInfo; canEdit: boolean; onChanged: () => void }) {
  const [describe, setDescribe] = useState(false)
  const [restore, setRestore] = useState(false)
  return (
    <div className="flex gap-3">
      <FileCode2 className="mt-0.5 size-4 shrink-0 text-stamp" />
      <div className="min-w-0 flex-1">
        <p className="font-medium">
          Versão {v.number} <span className="font-normal text-muted-foreground">· {versionOrigin(v)}{v.current && " · atual"}</span>
        </p>
        <p className="text-muted-foreground">
          {formatWhen(v.created_at)} · {v.author_probable ? `provável: ${v.author_name}` : v.author_name || "autor não informado"}
        </p>
        {v.description && <p className="mt-1">{v.description}</p>}
        <p className="mt-1 text-muted-foreground">{versionLine(v)}</p>
        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm">
          <Link to={`/apps/${appId}/versoes/${v.number}`} className="underline underline-offset-4">Ver mudanças</Link>
          <Link to={`/apps/${appId}/versoes/${v.number}?ver=arquivos`} className="underline underline-offset-4">Ver como estava</Link>
          {canEdit && (
            <button type="button" onClick={() => setDescribe(true)} className="inline-flex items-center gap-1 underline underline-offset-4">
              <PencilLine className="size-3.5" /> Descrever
            </button>
          )}
          {canEdit && !v.current && (
            <button type="button" onClick={() => setRestore(true)} className="inline-flex items-center gap-1 text-stamp underline underline-offset-4">
              <RotateCcw className="size-3.5" /> Voltar o código para esta versão
            </button>
          )}
        </div>
        {canEdit && <DescribeDialog appId={appId} version={v} open={describe} onOpenChange={setDescribe} onSaved={onChanged} />}
        {canEdit && <RestoreDialog appId={appId} number={v.number} open={restore} onOpenChange={setRestore} onDone={onChanged} />}
      </div>
    </div>
  )
}

function Row({ appId, item, canEdit, onChanged }: { appId: number; item: HistoryItem; canEdit: boolean; onChanged: () => void }) {
  if (item.version) return <VersionRow appId={appId} v={item.version} canEdit={canEdit} onChanged={onChanged} />
  if (item.run) {
    const r = item.run
    return (
      <div className="flex gap-3">
        <Play className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
        <p>
          <Link to={`/execucoes/${r.id}`} className="font-medium underline-offset-4 hover:underline">{STATUS[r.status].label}</Link>{" "}
          <span className="text-muted-foreground">
            · {formatWhen(item.at)} · {r.person_name}{r.version_number ? ` · versão ${r.version_number}` : ""}
          </span>
        </p>
      </div>
    )
  }
  const f = item.ficha!
  return (
    <div className="flex gap-3">
      <PencilLine className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div>
        <p className="text-muted-foreground">Ficha · {formatWhen(item.at)} · {f.person_name}</p>
        <ul className="mt-1">
          {f.changes.map((c) => (
            <li key={c.field}>
              {c.field === "created" ? c.label : c.long ? `${c.label} alterado` : `${c.label}: ${c.before || "—"} → ${c.after || "—"}`}
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}

export function HistoryTimeline({ appId, canEdit }: { appId: number; canEdit: boolean }) {
  const [filter, setFilter] = useState<HistoryFilter>("all")
  const [items, setItems] = useState<HistoryItem[] | null>(null)
  const [more, setMore] = useState(false)
  const [next, setNext] = useState("")
  const [error, setError] = useState("")

  const load = useCallback(
    (f: HistoryFilter) => {
      setError("")
      api.history(appId, f).then(
        (h) => {
          setItems(h.items)
          setMore(h.more)
          setNext(h.next_before)
        },
        (e: Error) => setError(e.message),
      )
    },
    [appId],
  )

  useEffect(() => load(filter), [load, filter])

  const loadMore = () =>
    api.history(appId, filter, next).then(
      (h) => {
        setItems((cur) => [...(cur ?? []), ...h.items])
        setMore(h.more)
        setNext(h.next_before)
      },
      (e: Error) => setError(e.message),
    )

  return (
    <div>
      <div className="mb-4 flex flex-wrap gap-2" role="group" aria-label="Filtrar histórico">
        {FILTERS.map((f) => (
          <Button key={f.value} size="sm" variant={filter === f.value ? "default" : "outline"} aria-pressed={filter === f.value} onClick={() => setFilter(f.value)}>
            {f.label}
          </Button>
        ))}
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {items && items.length === 0 && <p className="text-sm text-muted-foreground">Nada por aqui ainda.</p>}
      {items && items.length > 0 && (
        <ul aria-label="Histórico" className="flex flex-col divide-y divide-border text-sm">
          {items.map((it, i) => (
            <li key={`${it.type}-${it.at}-${i}`} className="py-3">
              <Row appId={appId} item={it} canEdit={canEdit} onChanged={() => load(filter)} />
            </li>
          ))}
        </ul>
      )}
      {more && (
        <Button variant="outline" size="sm" className="mt-3" onClick={loadMore}>Carregar mais</Button>
      )}
    </div>
  )
}
```

(Check that `Button` supports `size="sm"`; read `components/ui/button.tsx`.)

- [ ] **Step 5: Implement `DescribeDialog.tsx`**

```tsx
import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import { Field } from "@/components/bits"
import { api, type Person, type VersionInfo } from "@/lib/api"

export function DescribeDialog({ appId, version, open, onOpenChange, onSaved }: {
  appId: number; version: VersionInfo; open: boolean; onOpenChange: (v: boolean) => void; onSaved: () => void
}) {
  const [text, setText] = useState(version.description)
  const [people, setPeople] = useState<Person[]>([])
  const [author, setAuthor] = useState<number | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setText(version.description)
    api.people().then((ps) => {
      setPeople(ps)
      setAuthor(ps.find((p) => p.name === version.author_name)?.id ?? null)
    }, () => setPeople([]))
  }, [open, version])

  const save = async () => {
    setBusy(true)
    setError("")
    try {
      await api.describeVersion(appId, version.number, { description: text, author_id: author })
      onOpenChange(false)
      onSaved()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Descrever a versão {version.number}</DialogTitle>
          <DialogDescription>Conte o que mudou e quem fez. É opcional e pode ser mudado depois.</DialogDescription>
        </DialogHeader>
        <Field label="O que mudou?" htmlFor="version-desc">
          <Textarea id="version-desc" value={text} onChange={(e) => setText(e.target.value)} rows={4} maxLength={2000} />
        </Field>
        <Field label="Quem fez a mudança?" htmlFor="version-author">
          <select id="version-author" value={author ?? ""} onChange={(e) => setAuthor(e.target.value ? Number(e.target.value) : null)}
            className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
            <option value="">Não sei</option>
            {people.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </Field>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
          <Button onClick={save} disabled={busy}>Salvar</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

(Read `components/bits.tsx` for `Field`'s props; if the project already has a person picker in `ReviewForm.tsx`, reuse its pattern instead of a raw `select`.)

- [ ] **Step 6: Implement `RestoreDialog.tsx`**

```tsx
import { useEffect, useState } from "react"
import { TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, type RestorePlan, type RestoreResult } from "@/lib/api"

function Files({ title, list }: { title: string; list: string[] }) {
  if (list.length === 0) return null
  return (
    <div>
      <p className="font-medium">{title}</p>
      <ul className="mt-1 font-mono text-xs text-muted-foreground">{list.map((p) => <li key={p}>{p}</li>)}</ul>
    </div>
  )
}

export function RestoreDialog({ appId, number, open, onOpenChange, onDone }: {
  appId: number; number: number; open: boolean; onOpenChange: (v: boolean) => void; onDone: () => void
}) {
  const [plan, setPlan] = useState<RestorePlan | null>(null)
  const [result, setResult] = useState<RestoreResult | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setPlan(null)
    setResult(null)
    setError("")
    api.restorePreview(appId, number).then((p) => setPlan(p.plan), (e: Error) => setError(e.message))
  }, [open, appId, number])

  const confirm = async () => {
    setBusy(true)
    setError("")
    try {
      setResult(await api.restore(appId, number))
      onDone()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível voltar o código.")
    } finally {
      setBusy(false)
    }
  }

  const nothing = plan && plan.rewrite.length + plan.recreate.length + plan.to_trash.length === 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Voltar o código para a versão {number}</DialogTitle>
          <DialogDescription>
            Antes de mexer, o Hub guarda o estado atual como uma versão. Nada é apagado: arquivos a mais vão para a pasta .app-hub-lixeira dentro do app.
          </DialogDescription>
        </DialogHeader>
        {!plan && !error && <p className="text-sm text-muted-foreground">Conferindo a pasta…</p>}
        {plan && !result && (
          <div className="flex flex-col gap-3 text-sm">
            {nothing && <p>O código já está igual ao da versão {number}.</p>}
            <Files title="Voltam ao que eram" list={plan.rewrite} />
            <Files title="Voltam a existir" list={plan.recreate} />
            <Files title="Vão para a lixeira" list={plan.to_trash} />
            {plan.binaries.length > 0 && (
              <p className="flex gap-2 text-warn">
                <TriangleAlert className="mt-0.5 size-4 shrink-0" />
                {plan.binaries.join(", ")} não volta. O Hub guarda só a impressão digital de programas prontos. Recompile depois de voltar o código.
              </p>
            )}
          </div>
        )}
        {result && (
          <div className="flex flex-col gap-2 text-sm" role="status">
            {result.failed.length === 0 ? (
              <p>Pronto. O código voltou para a versão {number} e foi registrado como versão {result.number}.</p>
            ) : (
              <>
                <p className="text-destructive">Parei no meio. A versão {result.number} registra como a pasta ficou.</p>
                <Files title="Voltaram" list={result.done} />
                <Files title="Não voltaram" list={result.failed.map((f) => `${f.path} (${f.error})`)} />
              </>
            )}
          </div>
        )}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          {result ? (
            <Button onClick={() => onOpenChange(false)}>Fechar</Button>
          ) : (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
              <Button onClick={confirm} disabled={!plan || busy || !!nothing}>Voltar o código</Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 7: Run the timeline tests** — Expected: PASS.

- [ ] **Step 8: Wire `AppPage.tsx`**

- Replace the `RunsList` import with `import { HistoryTimeline } from "@/components/HistoryTimeline"` and `import { useAuth } from "@/lib/auth"`; add `const { person } = useAuth()` and `const canEdit = person?.role === "admin" || person?.role === "criador"`.
- Make the `<dl>` `sm:grid-cols-4` and add:

```tsx
          <div>
            <dt className="text-muted-foreground">Versão</dt>
            <dd className="font-medium">{app.version ? `versão ${app.version.number} · ${formatWhen(app.version.created_at)}` : "—"}</dd>
          </div>
```

  (`import { formatWhen } from "@/lib/runs"`).
- Under the `<dl>`, when `!app.folder_readable`:

```tsx
        {!app.folder_readable && (
          <p role="alert" className="mt-4 flex gap-2 text-sm text-warn">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" /> Não consegui ler a pasta do app agora. O histórico continua aqui, mas mudanças novas no código não são registradas e não dá para voltar versões.
          </p>
        )}
```

- Replace the `Execuções` section with:

```tsx
          <Section title="Histórico">
            <HistoryTimeline appId={app.id} canEdit={canEdit} />
          </Section>
```

- Delete `frontend/src/components/RunsList.tsx`.
- In `AppPage.test.tsx`: `detail()` gets `version: { number: 2, created_at: "2026-09-30T10:00:00Z" }, folder_readable: true`; replace `vi.spyOn(api, "appRuns").mockResolvedValue([])` with `vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })` in every test; add:

```tsx
test("shows the version and warns when the folder cannot be read", async () => {
  vi.spyOn(api, "app").mockResolvedValue({ ...detail(true), folder_readable: false })
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  renderAt()
  expect(await screen.findByText(/versão 2 ·/)).toBeInTheDocument()
  expect(screen.getByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora.")
})
```

- [ ] **Step 9: `RunPage.tsx`** — the "Versão do app" `dd` becomes:

```tsx
<dd className="font-medium">{run.version_number ? `versão ${run.version_number}` : run.app_version || "—"}</dd>
```

  and `fixtures.ts` `run()` default gets `version_number: null`.

- [ ] **Step 10: Run all frontend tests and type check** — Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add frontend
git commit -m "History on the ficha: timeline, describe and restore dialogs"
```

---

### Task 11: Frontend — version page (what changed / as it was)

**Files:**
- Create: `frontend/src/components/DiffView.tsx`, `frontend/src/pages/VersionPage.tsx`
- Modify: `frontend/src/App.tsx` (route `apps/:id/versoes/:n`)
- Test: `frontend/src/pages/VersionPage.test.tsx`

**Interfaces:**
- Consumes: `api.version`, `api.versionChanges`, `api.versionFiles`, `api.versionContent`, `api.app`; `DescribeDialog`, `RestoreDialog`, `versionLine`, `versionOrigin` (Task 10); `useAuth`.
- Produces: `<DiffView hunks />`; page at `/apps/:id/versoes/:n`, `?ver=arquivos` selects "Como estava".

- [ ] **Step 1: Write the failing tests**

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type VersionInfo } from "@/lib/api"
import { VersionPage } from "./VersionPage"

afterEach(() => vi.restoreAllMocks())

const info: VersionInfo = {
  number: 2, origin: "folder", restored_from: null, author_name: "Henry Kropp", author_probable: true, description: "Troca do título",
  summary: { added: 0, removed: 0, modified: 1, binaries: ["Ferramenta CVM.exe"] }, files_changed_at: null, created_at: "2026-10-01T12:00:00Z", current: false,
}

const renderAt = (url: string) =>
  render(
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path="/apps/:id/versoes/:n" element={<VersionPage />} />
      </Routes>
    </MemoryRouter>,
  )

test("O que mudou shows red and green lines and binary sizes", async () => {
  vi.spyOn(api, "versionChanges").mockResolvedValue({
    version: info,
    files: [
      { path: "Código-fonte/Files Prep.py", status: "modified", kind: "code", old_size: 10, new_size: 12,
        hunks: [{ lines: [{ op: " ", text: "import os", old: 1, new: 1 }, { op: "-", text: "TITLE = 'Pipeline'", old: 2, new: 0 }, { op: "+", text: "TITLE = 'Files Prep'", old: 0, new: 2 }] }] },
      { path: "Ferramenta CVM.exe", status: "modified", kind: "binary", old_size: 79291437, new_size: 79500000, hunks: [] },
    ],
  })
  renderAt("/apps/3/versoes/2")
  expect(await screen.findByRole("heading", { name: /Versão 2/ })).toBeInTheDocument()
  expect(screen.getByText("Troca do título")).toBeInTheDocument()
  expect(screen.getByText("TITLE = 'Pipeline'").closest("[data-op]")).toHaveAttribute("data-op", "-")
  expect(screen.getByText("TITLE = 'Files Prep'").closest("[data-op]")).toHaveAttribute("data-op", "+")
  expect(screen.getByText("mudou (75,6 MB → 75,8 MB)")).toBeInTheDocument()
})

test("Como estava lists files and opens one", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "version").mockResolvedValue(info)
  vi.spyOn(api, "versionFiles").mockResolvedValue([
    { path: "Código-fonte/Files Prep.py", size: 12, kind: "code", modified: "2026-10-01T12:00:00Z" },
    { path: "Ferramenta CVM.exe", size: 79291437, kind: "binary", modified: "2026-08-13T17:54:00Z" },
  ])
  const content = vi.spyOn(api, "versionContent").mockResolvedValue({ path: "Código-fonte/Files Prep.py", text: "API = \"••••\"\nprint(1)\n" })
  renderAt("/apps/3/versoes/2?ver=arquivos")
  await user.click(await screen.findByRole("button", { name: "Código-fonte/Files Prep.py" }))
  expect(content).toHaveBeenCalledWith("3", "2", "Código-fonte/Files Prep.py")
  expect(await screen.findByText(/print\(1\)/)).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Ferramenta CVM.exe" })).toBeDisabled()
})
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL (module not found).

- [ ] **Step 3: Implement `DiffView.tsx`**

```tsx
import type { DiffLine } from "@/lib/api"

const tone: Record<DiffLine["op"], string> = {
  " ": "",
  "-": "bg-destructive/10 text-destructive",
  "+": "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
}

export function DiffView({ hunks }: { hunks: { lines: DiffLine[] }[] }) {
  return (
    <div className="overflow-x-auto rounded-md border border-border font-mono text-xs">
      {hunks.map((h, i) => (
        <div key={i} className={i > 0 ? "border-t border-dashed border-border" : ""}>
          {h.lines.map((l, j) => (
            <div key={j} data-op={l.op} className={`flex whitespace-pre ${tone[l.op]}`}>
              <span className="w-10 shrink-0 px-1 text-right text-muted-foreground select-none">{l.old || ""}</span>
              <span className="w-10 shrink-0 px-1 text-right text-muted-foreground select-none">{l.new || ""}</span>
              <span className="w-4 shrink-0 select-none">{l.op}</span>
              <span>{l.text}</span>
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
```

(If the project defines a success/ok color token, e.g. used by `STATUS` tone "ok", use it instead of `emerald`.)

- [ ] **Step 4: Implement `VersionPage.tsx`**

```tsx
import { useEffect, useState } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"
import { ArrowLeft, PencilLine, RotateCcw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DescribeDialog } from "@/components/DescribeDialog"
import { DiffView } from "@/components/DiffView"
import { RestoreDialog } from "@/components/RestoreDialog"
import { versionLine, versionOrigin } from "@/components/HistoryTimeline"
import { api, type FileChange, type VersionFile, type VersionInfo } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { formatSize, formatWhen } from "@/lib/runs"

const STATUS_LABEL: Record<FileChange["status"], string> = { added: "novo", removed: "removido", modified: "alterado" }

function Changes({ files }: { files: FileChange[] }) {
  if (files.length === 0) return <p className="text-sm text-muted-foreground">Nenhum arquivo mudou.</p>
  return (
    <div className="flex flex-col gap-5">
      {files.map((f) => (
        <section key={f.path}>
          <p className="mb-2 text-sm">
            <code className="font-mono text-xs">{f.path}</code> <span className="text-muted-foreground">· {STATUS_LABEL[f.status]}</span>
          </p>
          {f.kind === "binary" ? (
            <p className="text-sm text-muted-foreground">
              {f.status === "modified" ? `mudou (${formatSize(f.old_size ?? 0)} → ${formatSize(f.new_size ?? 0)})` : formatSize((f.new_size ?? f.old_size) ?? 0)}
            </p>
          ) : (
            <DiffView hunks={f.hunks} />
          )}
        </section>
      ))}
    </div>
  )
}

function AsItWas({ appId, n, files }: { appId: string; n: string; files: VersionFile[] }) {
  const [open, setOpen] = useState<{ path: string; text: string } | null>(null)
  const [error, setError] = useState("")
  const show = (path: string) => {
    setError("")
    api.versionContent(appId, n, path).then(setOpen, (e: Error) => setError(e.message))
  }
  return (
    <div className="grid gap-4 md:grid-cols-[minmax(0,16rem)_1fr]">
      <ul className="flex flex-col gap-1 text-sm">
        {files.map((f) => (
          <li key={f.path}>
            <button type="button" disabled={f.kind === "binary"} onClick={() => show(f.path)}
              className={`w-full truncate text-left font-mono text-xs ${open?.path === f.path ? "text-stamp" : ""} disabled:text-muted-foreground`}
              title={f.kind === "binary" ? "Programa pronto: o Hub guarda só o tamanho e a data" : f.path}>
              {f.path}
            </button>
          </li>
        ))}
      </ul>
      <div className="min-w-0">
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        {open ? (
          <pre className="overflow-x-auto rounded-md border border-border p-3 font-mono text-xs whitespace-pre">{open.text}</pre>
        ) : (
          <p className="text-sm text-muted-foreground">Escolha um arquivo para ver como ele estava.</p>
        )}
      </div>
    </div>
  )
}

export function VersionPage() {
  const { id = "", n = "" } = useParams()
  const [params, setParams] = useSearchParams()
  const view = params.get("ver") === "arquivos" ? "arquivos" : "mudancas"
  const { person } = useAuth()
  const canEdit = person?.role === "admin" || person?.role === "criador"
  const [info, setInfo] = useState<VersionInfo | null>(null)
  const [changes, setChanges] = useState<FileChange[] | null>(null)
  const [files, setFiles] = useState<VersionFile[] | null>(null)
  const [error, setError] = useState("")
  const [describe, setDescribe] = useState(false)
  const [restore, setRestore] = useState(false)
  const [reload, setReload] = useState(0)

  useEffect(() => {
    setError("")
    if (view === "mudancas") {
      api.versionChanges(id, n).then((r) => { setInfo(r.version); setChanges(r.files) }, (e: Error) => setError(e.message))
    } else {
      api.version(id, n).then(setInfo, (e: Error) => setError(e.message))
      api.versionFiles(id, n).then(setFiles, (e: Error) => setError(e.message))
    }
  }, [id, n, view, reload])

  if (error) return <p className="text-destructive">{error}</p>
  if (!info) return <p className="text-muted-foreground">Carregando…</p>

  return (
    <article className="rise">
      <Link to={`/apps/${id}`} className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Ficha do app
      </Link>
      <div className="paper rounded-lg border border-border bg-card p-8 shadow-card">
        <h1 className="font-heading text-3xl">Versão {info.number}{info.current && <span className="ml-2 text-base text-muted-foreground">(atual)</span>}</h1>
        <p className="mt-2 text-muted-foreground">
          {versionOrigin(info)} · {formatWhen(info.created_at)} · {info.author_probable ? `provável: ${info.author_name}` : info.author_name || "autor não informado"}
        </p>
        {info.description && <p className="mt-2">{info.description}</p>}
        <p className="mt-1 text-sm text-muted-foreground">{versionLine(info)}</p>
        {canEdit && (
          <div className="mt-4 flex flex-wrap gap-2">
            <Button variant="outline" className="gap-2" onClick={() => setDescribe(true)}><PencilLine className="size-4" /> Descrever</Button>
            {!info.current && (
              <Button className="gap-2" onClick={() => setRestore(true)}><RotateCcw className="size-4" /> Voltar o código para esta versão</Button>
            )}
            <DescribeDialog appId={Number(id)} version={info} open={describe} onOpenChange={setDescribe} onSaved={() => setReload((x) => x + 1)} />
            <RestoreDialog appId={Number(id)} number={info.number} open={restore} onOpenChange={setRestore} onDone={() => setReload((x) => x + 1)} />
          </div>
        )}
        <div className="mt-6 mb-4 flex gap-2 border-t border-border pt-5" role="tablist">
          <Button role="tab" aria-selected={view === "mudancas"} size="sm" variant={view === "mudancas" ? "default" : "outline"} onClick={() => setParams({})}>O que mudou</Button>
          <Button role="tab" aria-selected={view === "arquivos"} size="sm" variant={view === "arquivos" ? "default" : "outline"} onClick={() => setParams({ ver: "arquivos" })}>Como estava</Button>
        </div>
        {view === "mudancas" ? changes && <Changes files={changes} /> : files && <AsItWas appId={id} n={n} files={files} />}
      </div>
    </article>
  )
}
```

- [ ] **Step 5: Route** in `App.tsx`, next to `apps/:id`:

```tsx
        <Route path="apps/:id/versoes/:n" element={<VersionPage />} />
```

(`import { VersionPage } from "@/pages/VersionPage"` — match how the other pages are imported.)

- [ ] **Step 6: Run all frontend tests and type check** — Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend
git commit -m "Version page: what changed and as it was"
```

---

### Task 12: Visual check, docs and real test with Henry

**Files:**
- Modify: `docs/TASKS.md`, `docs/PRD.md` (3.6 status), `docs/superpowers/specs/2026-10-01-fase-5-historico-design.md` (status line)
- Create: `docs/adr/006-historico-de-codigo-no-hub.md`

- [ ] **Step 1: Full test run** — Go and frontend commands from Global Constraints. Expected: all PASS.

- [ ] **Step 2: Run the app for the visual check**

The real Hub (port 8080, real database) is what Henry's Agent talks to. Restart **that** Hub with the new code (same `.env`, same port), never a second Hub on 8080. Start Vite and read its `Local:` URL. Log in with `POST /api/dev/login` only if the running Hub has `DEV_MODE=1`; otherwise ask Henry before using his session.

- [ ] **Step 3: Screenshots** with Playwright (`e2e/`): the Files Prep ficha (Versão field + Histórico with filters), the version page "O que mudou" and "Como estava", the restore dialog (open it, do not confirm). Desktop 1280×720 and mobile 375×812. Check alignment, spacing, contrast in light and dark. Fix and re-shoot.

- [ ] **Step 4: ADR 006** (template from the tech-stack skill): history stored in the Hub (JSONB file list + `code_blobs` by SHA-256), not git in the app folder. Alternatives: git repository inside the folder (touches the user's folder and OneDrive, rejected by "não destrutivo"); copying the whole folder (2,3 GB of outputs); copying the `.exe` (Henry chose source only). Consequence: the `Folder` interface is the seam for the Agent when the Hub moves to the central server (ADR 005).

- [ ] **Step 5: Docs** — `docs/TASKS.md`: Fase 5 items "Versão 1 no registro + detecção de mudança na pasta" and "Histórico, comparação e voltar atrás" → Done (after the real test); update "Prioridade atual" (next: acabamento); add leftovers found during the work. `docs/PRD.md` 3.6: note what was built (source-only copies, ficha changes recorded only). Spec status → "implementado".

- [ ] **Step 6: Commit and push**

```bash
git add docs
git commit -m "Docs: Fase 5 history done; ADR 006"
git push
```

- [ ] **Step 7: Real test with Henry (one action per question, verify through Hub data)**

1. Open the Files Prep ficha → Histórico shows "Versão 1 · Estado original" (check `app_versions` in the dev database).
2. Henry changes one line in `Código-fonte\Files Prep.py` (e.g. a comment) and saves.
3. Reopen the ficha → "Versão 2 · Mudança na pasta · provável: Henry Kropp", "1 arquivo mudou"; "Ver mudanças" shows the line in red/green.
4. "Voltar o código para esta versão" on Versão 1 → the dialog lists `Código-fonte/Files Prep.py` under "Voltam ao que eram" → confirm.
5. Henry opens the `.py` in the folder and confirms the line is back; the timeline shows "Versão 3 · Restaurada da versão 1".
6. Run the app once from the Hub → the run shows "versão 3".
