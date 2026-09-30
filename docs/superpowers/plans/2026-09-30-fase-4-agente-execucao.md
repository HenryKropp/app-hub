# Fase 4 — Agente do Hub e execução de apps com janela — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A person clicks **Rodar** on a desktop `.exe` app in the Hub; a Windows agent opens it, waits for the window to close, and reports new/changed files, which the Hub groups by the manifest `outputs`, records as a run and lets the person download (streamed on demand from the agent).

**Architecture:** The Hub (Go, WSL) queues *jobs* per computer in Postgres. The Agent (Go, compiled for Windows, same Go module as the backend) long-polls `GET /api/agent/next`, executes `run` / `choose_folder` / `send_file` jobs and posts results back. Downloads use an in-memory rendezvous: the browser request waits while the agent streams the file into `POST /api/agent/jobs/{id}/upload`. Platform-neutral agent logic lives in `backend/internal/agent` (tested on Linux); Windows-only bits (install, registry, folder dialog, browser) live in `backend/cmd/agent/*_windows.go`.

**Tech Stack:** Go 1.27 stdlib `net/http`, pgx/v5, sqlc, `golang.org/x/sys/windows/registry`; React + TypeScript + Vite, shadcn/ui, Vitest; Playwright for screenshots.

**Spec:** `docs/superpowers/specs/2026-09-30-fase-4-agente-execucao-design.md`

**Deviation from spec (deliberate):** the spec says `agent/` as a new module. The plan puts the agent in the **backend module** (`backend/cmd/agent`, `backend/internal/agent`, `backend/internal/agentproto`) so Hub and Agent share the protocol types without a second `go.mod`. Behaviour is unchanged. Record this in ADR 004 (Task 15).

## Global Constraints

- All UI text in Brazilian Portuguese, plain language, no unexplained jargon.
- All SQL in `backend/internal/database/queries/*.sql` via sqlc; never raw SQL in handlers (tests may use raw SQL to backdate rows).
- Logging with `slog` only.
- Go commands run from `backend/` via `mise x --`; frontend commands from `frontend/` via `mise x --`.
- Agent version constant: `agentproto.Version = "0.1.0"`.
- Connected = computer not revoked with `last_seen_at` within **60 s**.
- Pairing code expires in **10 min**, single use.
- Pending jobs older than **2 min** expire; their `waiting` runs become `interrupted`.
- `next` long-poll holds up to **25 s**; download waits up to **60 s** for the agent to start sending; folder choice waits up to **5 min**.
- Runnable = manifest `kind: desktop` and `entrypoint` ending in `.exe` (case-insensitive).
- Runs visible/downloadable by the person who ran them and by admins only.
- Tokens stored only as SHA-256 hex (`token_hash`, `secret_hash`).
- Agent install dir: `%LOCALAPPDATA%\AppHub`; autostart via `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` value `AppHubAgente`; single-instance lock on `127.0.0.1:47631` (serves `POST /quit`).
- Agent binary path served by the Hub: env `AGENT_BINARY_PATH`; built by `scripts/build-agent.sh` into `dist/agent/AppHubAgente.exe` (gitignored).

## Review Focus

1. **Folder paths with accents and spaces** (`8. Programação`, `OneDrive - Alis`): the folder dialog output and the paths sent to the agent must survive UTF-8 round-trips. Pinned by the agent runner test with an accented temp folder (Task 10) and the PowerShell script forcing UTF-8 output (Task 11).
2. **Agent offline when Rodar is clicked, or disconnects before claiming**: the run must not stay "Aguardando" forever. Pinned by the stale-job expiry test (Task 7).
3. **Person clicks Rodar twice / app already open**: second run must fail with a clear message, not open two copies fighting over the folder. Pinned by the runner busy test (Task 10).
4. **Chosen folder already contains a file named like the `.exe`**: never overwrite or delete the person's file. Pinned by the runner test (Task 10).
5. **Download request for a file not in the run, or a path with `..`**: refused on both Hub and Agent. Pinned by Hub download test (Task 8) and agent `PrepareSend` test (Task 10).

---

## File Structure

**Backend (new)**
- `backend/internal/agentproto/proto.go` — job kinds, payloads, results, file entries, version.
- `backend/internal/outputs/group.go` (+ test) — match a relative path to a manifest output group.
- `backend/internal/agent/snapshot.go` (+ test) — folder snapshot and diff.
- `backend/internal/agent/send.go` (+ test) — run record, `PrepareSend`, file/zip writer.
- `backend/internal/agent/runner.go` (+ test) — the `run` job.
- `backend/internal/agent/client.go` — HTTP client to the Hub.
- `backend/internal/agent/agent.go` (+ test) — pairing, main loop, job dispatch.
- `backend/cmd/agent/main.go`, `platform_windows.go`, `platform_other.go` — entrypoint and OS glue.
- `backend/internal/database/migrations/003_agents_runs.sql`
- `backend/internal/database/queries/agents.sql`, `runs.sql`
- `backend/internal/handler/agents.go` (+ `agents_test.go`) — pairing, computers, agent endpoints, jobs.
- `backend/internal/handler/runs.go` (+ `runs_test.go`) — runs, folder choice, downloads.
- `backend/internal/handler/uploads.go` — download rendezvous.
- `scripts/build-agent.sh`

**Backend (modified)**
- `backend/internal/handler/handler.go` — new fields, routes.
- `backend/internal/handler/apps.go` — `can_run` in app detail.
- `backend/internal/handler/handler_test.go` — pool-based client helper and TRUNCATE list.
- `backend/internal/config/config.go` — `AgentBinaryPath`.
- `backend/go.mod` — `golang.org/x/sys`.

**Frontend**
- `frontend/src/lib/api.ts` — types and calls.
- `frontend/src/lib/runs.ts` (+ test) — labels and formatting.
- `frontend/src/pages/ComputerPage.tsx` (+ test), `ConnectPage.tsx` (+ test), `RunPage.tsx` (+ test)
- `frontend/src/components/RunDialog.tsx`, `RunsList.tsx`
- `frontend/src/pages/AppPage.tsx`, `AppPage.test.tsx` (new test)
- `frontend/src/App.tsx`, `components/Layout.tsx`, `pages/LoginPage.tsx`
- `frontend/src/test/fixtures.ts`

**Docs**: `docs/adr/004-...md`, `docs/adr/001-...md`, `docs/PRD.md`, `docs/TASKS.md`, `docs/INFRASTRUCTURE.md`, `.gitignore`, `.env`.

---

### Task 1: Connectivity check and protocol types

Confirms the spec's assumption that a Windows process reaches the Hub in WSL at `http://localhost:8080`, then adds the shared protocol package.

**Files:**
- Create: `backend/internal/agentproto/proto.go`

**Interfaces:**
- Produces: package `agentproto` with everything below (used by Tasks 5–11).

- [ ] **Step 1: Verify Windows → WSL localhost**

Start the database and the Go server (tech-stack steps 1–2), then from WSL run:

```bash
powershell.exe -NoProfile -Command "(Invoke-WebRequest -UseBasicParsing http://localhost:8080/up).StatusCode"
```

Expected: `200`. If it fails, stop and report: the agent would need the WSL IP (`hostname -I`) instead, which changes `hubURL` in Task 11 and `BASE_URL` handling.

- [ ] **Step 2: Write the protocol package**

```go
// Package agentproto holds the messages exchanged between the Hub and the
// Agent do Hub running on people's Windows computers.
package agentproto

import (
	"encoding/json"
	"time"
)

// Version is the agent version this Hub expects.
const Version = "0.1.0"

// Job kinds.
const (
	KindRun          = "run"
	KindChooseFolder = "choose_folder"
	KindSendFile     = "send_file"
)

// Result events.
const (
	EventStarted  = "started"  // run: app is about to open
	EventFinished = "finished" // run: window closed, files listed
	EventFailed   = "failed"   // any job: could not do it
	EventMissing  = "missing"  // send_file: file no longer in the folder
	EventDone     = "done"     // choose_folder: answered (folder or canceled)
)

// Job is what GET /api/agent/next returns.
type Job struct {
	ID      int64           `json:"id"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// RunPayload opens AppFolder\Exe with OutputFolder as the place results go.
// Paths are Windows paths ("C:\...").
type RunPayload struct {
	RunID        int64  `json:"run_id"`
	AppFolder    string `json:"app_folder"`
	Exe          string `json:"exe"`
	OutputFolder string `json:"output_folder"`
}

type ChooseFolderPayload struct {
	Initial string `json:"initial"`
}

// SendFilePayload asks for one file, or several zipped, from a run's results.
// Paths are relative to the run's output folder, with forward slashes.
type SendFilePayload struct {
	RunID int64    `json:"run_id"`
	Paths []string `json:"paths"`
	Zip   bool     `json:"zip"`
}

// FileEntry is a new or changed file found after a run.
type FileEntry struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Change   string    `json:"change"` // "new" or "changed"
}

// Result is posted to POST /api/agent/jobs/{id}/result.
type Result struct {
	Event      string      `json:"event"`
	Error      string      `json:"error,omitempty"`
	ExitCode   int         `json:"exit_code,omitempty"`
	AppVersion string      `json:"app_version,omitempty"`
	Files      []FileEntry `json:"files,omitempty"`
	Folder     string      `json:"folder,omitempty"`
	Canceled   bool        `json:"canceled,omitempty"`
}

type Hello struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type PairStartRequest struct {
	ComputerName string `json:"computer_name"`
}

type PairStartResponse struct {
	Code       string `json:"code"`
	Secret     string `json:"secret"`
	ApproveURL string `json:"approve_url"`
}

type PairPollRequest struct {
	Secret string `json:"secret"`
}

// PairPollResponse.Status is "pending" or "approved" (with Token).
type PairPollResponse struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
}
```

- [ ] **Step 3: Build**

Run: `cd backend && mise x -- go build ./...` — Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/agentproto
git commit -m "Add agent protocol types"
```

---

### Task 2: Output grouping

**Files:**
- Create: `backend/internal/outputs/group.go`, `backend/internal/outputs/group_test.go`

**Interfaces:**
- Consumes: `manifest.Output{Label, Pattern}`.
- Produces: `outputs.OtherLabel = "Outros arquivos"`; `outputs.Match(outs []manifest.Output, rel string) (index int, label string)` — `index` is the manifest position, `-1` for others.

Rule: a pattern with `/` matches the whole relative path (`path.Match`, `*` does not cross `/`); a pattern without `/` matches the base name at any depth. When several match, the pattern with the most literal (non-wildcard) characters wins; ties go to manifest order.

- [ ] **Step 1: Write the failing test**

```go
package outputs

import (
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

var filesPrep = []manifest.Output{
	{Label: "Documentos da CVM", Pattern: "*/*/*.pdf"},
	{Label: "Data Pack", Pattern: "*/MD_Output/*_DataPack_Consolidado.md"},
	{Label: "FRE cortado", Pattern: "*_cortado_secoes.pdf"},
}

func TestMatch(t *testing.T) {
	cases := []struct {
		rel   string
		index int
		label string
	}{
		{"PETROBRAS/FRE/FRE_2025.pdf", 0, "Documentos da CVM"},
		{"PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", 1, "Data Pack"},
		{"PETROBRAS/FRE/FRE_2025_cortado_secoes.pdf", 2, "FRE cortado"},
		{"FRE_cortado_secoes.pdf", 2, "FRE cortado"},
		{"log_download.txt", -1, OtherLabel},
		{"PETROBRAS/x.pdf", -1, OtherLabel},
	}
	for _, c := range cases {
		i, l := Match(filesPrep, c.rel)
		if i != c.index || l != c.label {
			t.Errorf("Match(%q) = %d %q, want %d %q", c.rel, i, l, c.index, c.label)
		}
	}
}

func TestMatchNoOutputs(t *testing.T) {
	if i, l := Match(nil, "a.txt"); i != -1 || l != OtherLabel {
		t.Fatalf("got %d %q", i, l)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && mise x -- go test ./internal/outputs/` — Expected: FAIL, `undefined: Match`.

- [ ] **Step 3: Implement**

```go
// Package outputs sorts a run's files into the groups the manifest describes.
package outputs

import (
	"path"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

const OtherLabel = "Outros arquivos"

// Match returns the manifest output a relative path (forward slashes) belongs
// to, or -1 and OtherLabel. The most specific matching pattern wins.
func Match(outs []manifest.Output, rel string) (int, string) {
	best, bestScore := -1, -1
	for i, o := range outs {
		target := rel
		if !strings.Contains(o.Pattern, "/") {
			target = path.Base(rel)
		}
		ok, err := path.Match(o.Pattern, target)
		if err != nil || !ok {
			continue
		}
		if score := literalChars(o.Pattern); score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		return -1, OtherLabel
	}
	return best, outs[best].Label
}

func literalChars(p string) int {
	n := 0
	for _, r := range p {
		if r != '*' && r != '?' && r != '[' && r != ']' {
			n++
		}
	}
	return n
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && mise x -- go test ./internal/outputs/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/outputs
git commit -m "Group run files by manifest outputs"
```

---

### Task 3: Folder snapshot and diff (agent)

**Files:**
- Create: `backend/internal/agent/snapshot.go`, `backend/internal/agent/snapshot_test.go`

**Interfaces:**
- Produces: `type Snapshot map[string]agent.entry`; `agent.TakeSnapshot(root string, skip ...string) (Snapshot, error)` (skip = relative paths with `/`); `agent.Diff(before, after Snapshot) []agentproto.FileEntry` sorted by path.

- [ ] **Step 1: Write the failing test**

```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestSnapshotDiff(t *testing.T) {
	root := filepath.Join(t.TempDir(), "8. Programação")
	write(t, root, "PETROBRAS/FRE/antigo.pdf", "a")
	write(t, root, "muda.txt", "1")
	write(t, root, "Ferramenta CVM.exe", "exe")
	before, err := TakeSnapshot(root, "Ferramenta CVM.exe")
	if err != nil {
		t.Fatal(err)
	}

	write(t, root, "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", "novo")
	write(t, root, "muda.txt", "22")
	later := time.Now().Add(time.Minute)
	_ = os.Chtimes(filepath.Join(root, "muda.txt"), later, later)
	write(t, root, "Ferramenta CVM.exe", "exe mudou") // skipped
	after, err := TakeSnapshot(root, "Ferramenta CVM.exe")
	if err != nil {
		t.Fatal(err)
	}

	got := Diff(before, after)
	if len(got) != 2 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	if got[0].Path != "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md" || got[0].Change != "new" || got[0].Size != 4 {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Path != "muda.txt" || got[1].Change != "changed" {
		t.Errorf("second = %+v", got[1])
	}
}

func TestSnapshotMissingFolder(t *testing.T) {
	if _, err := TakeSnapshot(filepath.Join(t.TempDir(), "nao-existe")); err == nil {
		t.Fatal("want error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && mise x -- go test ./internal/agent/` — Expected: FAIL, `undefined: TakeSnapshot`.

- [ ] **Step 3: Implement**

```go
// Package agent is the platform-neutral part of the Agente do Hub: it opens
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
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && mise x -- go test ./internal/agent/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/agent
git commit -m "Agent: folder snapshot and diff"
```

---

### Task 4: Database schema and queries

**Files:**
- Create: `backend/internal/database/migrations/003_agents_runs.sql`, `backend/internal/database/queries/agents.sql`, `backend/internal/database/queries/runs.sql`
- Modify: `backend/internal/handler/handler_test.go` (TRUNCATE list)
- Generated: `backend/internal/database/sqlc/*`

**Interfaces:**
- Produces sqlc methods used by Tasks 5–8 (names below). After generating, **read** `sqlc/agents.sql.go` and `sqlc/runs.sql.go` and use the exact param/row struct names.

- [ ] **Step 1: Migration**

```sql
CREATE TABLE IF NOT EXISTS computers (
    id            BIGSERIAL PRIMARY KEY,
    person_id     BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    agent_version TEXT NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS computers_person_idx ON computers(person_id);

CREATE TABLE IF NOT EXISTS pairings (
    code          TEXT PRIMARY KEY,
    secret_hash   TEXT NOT NULL UNIQUE,
    computer_name TEXT NOT NULL,
    person_id     BIGINT REFERENCES people(id) ON DELETE CASCADE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS runs (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    person_id     BIGINT NOT NULL REFERENCES people(id),
    computer_id   BIGINT NOT NULL REFERENCES computers(id),
    output_folder TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'waiting'
                  CHECK (status IN ('waiting', 'running', 'done', 'no_files', 'error', 'interrupted')),
    exit_code     INT,
    error_message TEXT NOT NULL DEFAULT '',
    app_version   TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS runs_app_idx ON runs(app_id, id DESC);

CREATE TABLE IF NOT EXISTS agent_jobs (
    id          BIGSERIAL PRIMARY KEY,
    computer_id BIGINT NOT NULL REFERENCES computers(id) ON DELETE CASCADE,
    run_id      BIGINT REFERENCES runs(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('run', 'choose_folder', 'send_file')),
    payload     JSONB NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'taken', 'done', 'failed')),
    result      JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    taken_at    TIMESTAMPTZ,
    done_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS agent_jobs_pending_idx ON agent_jobs(computer_id, id) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS run_files (
    id          BIGSERIAL PRIMARY KEY,
    run_id      BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    rel_path    TEXT NOT NULL,
    size        BIGINT NOT NULL,
    modified_at TIMESTAMPTZ NOT NULL,
    change      TEXT NOT NULL CHECK (change IN ('new', 'changed')),
    group_index INT NOT NULL,
    group_label TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS run_files_run_idx ON run_files(run_id);
```

- [ ] **Step 2: `queries/agents.sql`**

```sql
-- name: CreatePairing :exec
INSERT INTO pairings (code, secret_hash, computer_name, expires_at)
VALUES ($1, $2, $3, now() + interval '10 minutes');

-- name: GetPairing :one
SELECT code, computer_name, person_id, (expires_at <= now())::boolean AS expired
FROM pairings WHERE code = $1;

-- name: ApprovePairing :execrows
UPDATE pairings SET person_id = $2
WHERE code = $1 AND person_id IS NULL AND expires_at > now();

-- name: GetPairingStateBySecret :one
SELECT person_id, (expires_at <= now())::boolean AS expired
FROM pairings WHERE secret_hash = $1;

-- name: ConsumePairing :one
DELETE FROM pairings
WHERE secret_hash = $1 AND person_id IS NOT NULL AND expires_at > now()
RETURNING computer_name, person_id;

-- name: CreateComputer :one
INSERT INTO computers (person_id, name, token_hash, last_seen_at)
VALUES ($1, $2, $3, now())
RETURNING *;

-- name: GetComputerByToken :one
SELECT * FROM computers WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: GetComputer :one
SELECT * FROM computers WHERE id = $1;

-- name: TouchComputer :exec
UPDATE computers SET last_seen_at = now() WHERE id = $1;

-- name: HelloComputer :exec
UPDATE computers SET last_seen_at = now(), name = $2, agent_version = $3 WHERE id = $1;

-- name: ListPersonComputers :many
SELECT *, (last_seen_at > now() - interval '60 seconds')::boolean AS connected
FROM computers
WHERE person_id = $1 AND revoked_at IS NULL
ORDER BY last_seen_at DESC NULLS LAST;

-- name: ConnectedComputerForPerson :one
SELECT * FROM computers
WHERE person_id = $1 AND revoked_at IS NULL AND last_seen_at > now() - interval '60 seconds'
ORDER BY last_seen_at DESC
LIMIT 1;

-- name: IsComputerConnected :one
SELECT (revoked_at IS NULL AND last_seen_at > now() - interval '60 seconds')::boolean
FROM computers WHERE id = $1;

-- name: RevokeComputer :exec
UPDATE computers SET revoked_at = now() WHERE id = $1;

-- name: CreateJob :one
INSERT INTO agent_jobs (computer_id, run_id, kind, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ClaimJob :one
UPDATE agent_jobs SET status = 'taken', taken_at = now()
WHERE id = (
    SELECT id FROM agent_jobs
    WHERE computer_id = $1 AND status = 'pending' AND created_at > now() - interval '2 minutes'
    ORDER BY id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: GetJob :one
SELECT * FROM agent_jobs WHERE id = $1;

-- name: FinishJob :exec
UPDATE agent_jobs SET status = $2, result = $3, done_at = now() WHERE id = $1;

-- name: ExpireStaleRuns :exec
UPDATE runs SET status = 'interrupted', finished_at = now(),
    error_message = 'O computador não abriu o app a tempo. Confira se ele está ligado e tente de novo.'
WHERE status = 'waiting' AND id IN (
    SELECT run_id FROM agent_jobs
    WHERE status = 'pending' AND run_id IS NOT NULL AND created_at <= now() - interval '2 minutes'
);

-- name: ExpireStaleJobs :exec
UPDATE agent_jobs SET status = 'failed', done_at = now()
WHERE status = 'pending' AND created_at <= now() - interval '2 minutes';

-- name: InterruptTakenRuns :exec
UPDATE runs SET status = 'interrupted', finished_at = now(),
    error_message = 'O Agente reiniciou com o app aberto (o computador pode ter sido desligado).'
WHERE status IN ('waiting', 'running') AND id IN (
    SELECT run_id FROM agent_jobs
    WHERE computer_id = $1 AND status = 'taken' AND run_id IS NOT NULL
);

-- name: FailTakenJobs :exec
UPDATE agent_jobs SET status = 'failed', done_at = now()
WHERE computer_id = $1 AND status = 'taken';
```

- [ ] **Step 3: `queries/runs.sql`**

```sql
-- name: CreateRun :one
INSERT INTO runs (app_id, person_id, computer_id, output_folder)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: StartRun :exec
UPDATE runs SET status = 'running', started_at = now(), app_version = $2
WHERE id = $1 AND status = 'waiting';

-- name: FinishRun :exec
UPDATE runs SET status = $2, exit_code = $3, error_message = $4, finished_at = now(),
    started_at = COALESCE(started_at, now())
WHERE id = $1 AND status IN ('waiting', 'running');

-- name: AddRunFile :exec
INSERT INTO run_files (run_id, rel_path, size, modified_at, change, group_index, group_label)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetRun :one
SELECT r.*, a.name AS app_name, a.manifest AS app_manifest, p.name AS person_name, c.name AS computer_name
FROM runs r
JOIN apps a ON a.id = r.app_id
JOIN people p ON p.id = r.person_id
JOIN computers c ON c.id = r.computer_id
WHERE r.id = $1;

-- name: ListAppRuns :many
SELECT r.id, r.status, r.created_at, r.started_at, r.finished_at, r.person_id, p.name AS person_name,
       (SELECT count(*) FROM run_files f WHERE f.run_id = r.id) AS file_count
FROM runs r
JOIN people p ON p.id = r.person_id
WHERE r.app_id = sqlc.arg(app_id) AND (sqlc.arg(see_all)::boolean OR r.person_id = sqlc.arg(person_id))
ORDER BY r.id DESC
LIMIT 20;

-- name: ListRunFiles :many
SELECT * FROM run_files WHERE run_id = $1 ORDER BY group_index < 0, group_index, rel_path;

-- name: GetRunFile :one
SELECT * FROM run_files WHERE id = $1 AND run_id = $2;
```

- [ ] **Step 4: Generate and read**

Run: `cd backend && mise x -- sqlc generate` — Expected: no output. Then read `internal/database/sqlc/agents.sql.go`, `runs.sql.go` and `models.go` (`Computer`, `Pairing`, `Run`, `AgentJob`, `RunFile`). Note nullable types (`pgtype.Int8`, `pgtype.Int4`, `pgtype.Timestamptz`) — later tasks must use the generated names.

- [ ] **Step 5: Extend the test TRUNCATE**

In `handler_test.go` `newClient`, change the TRUNCATE to:

```go
if _, err := tx.Exec(ctx, "TRUNCATE run_files, agent_jobs, runs, pairings, computers, apps, sessions, people RESTART IDENTITY CASCADE"); err != nil {
```

- [ ] **Step 6: Run the existing tests**

Run: `bash -c 'ROOT=$(git rev-parse --show-toplevel); set -a; . $ROOT/.env; set +a; cd $ROOT/backend && mise x -- go test ./...'` — Expected: all `ok` (migration applies).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/database backend/internal/handler/handler_test.go
git commit -m "Schema and queries for computers, pairings, jobs and runs"
```

---

### Task 5: Pairing and computers (Hub)

**Files:**
- Create: `backend/internal/handler/agents.go`, `backend/internal/handler/agents_test.go`
- Modify: `backend/internal/handler/handler.go` (fields, `New`, routes), `backend/internal/handler/handler_test.go` (pool client helper), `backend/internal/config/config.go`

**Interfaces:**
- Consumes: sqlc queries from Task 4; `agentproto` pair types.
- Produces (used by Tasks 6–9 and frontend):
  - `func hashToken(s string) string` (SHA-256 hex)
  - `func randomToken() (string, error)` (32 bytes base64url)
  - `func (h *Handler) currentComputer(w, r) (sqlc.Computer, bool)` — Bearer auth, touches `last_seen_at`
  - Handler fields: `waitNext, waitFile, waitFolder, tick time.Duration`, `uploads *uploadHub` (type in Task 8; declare the field in Task 8)
  - `config.Config.AgentBinaryPath string` (env `AGENT_BINARY_PATH`)
  - Routes: `POST /api/agent/pair/start`, `POST /api/agent/pair/poll`, `GET /api/pairings/{code}`, `POST /api/pairings/{code}/approve`, `GET /api/computers/me`, `DELETE /api/computers/{id}`
  - JSON `computerJSON{id,name,agent_version,last_seen_at,connected,outdated}`; `GET /api/computers/me` → `{computers, expected_version, agent_available}`
  - Test helpers: `newPoolClient(t) (*client, *Handler)`, `(*client).agent(method, path, token string, body, out any) int`, `pairComputer(t, c *client, name string) string` (returns token)

- [ ] **Step 1: Config**

In `config.go`, add `AgentBinaryPath string` to `Config` and `AgentBinaryPath: os.Getenv("AGENT_BINARY_PATH"),` in `Load`.

- [ ] **Step 2: Handler fields and defaults**

In `handler.go`:

```go
type Handler struct {
	q   *sqlc.Queries
	cfg config.Config

	// Waits used by the agent endpoints; tests shorten them.
	waitNext   time.Duration
	waitFile   time.Duration
	waitFolder time.Duration
	tick       time.Duration
}

func New(q *sqlc.Queries, cfg config.Config) *Handler {
	return &Handler{
		q: q, cfg: cfg,
		waitNext: 25 * time.Second, waitFile: 60 * time.Second, waitFolder: 5 * time.Minute, tick: 300 * time.Millisecond,
	}
}
```

Add routes in `Register` (after apps):

```go
	mux.HandleFunc("POST /api/agent/pair/start", h.pairStart)
	mux.HandleFunc("POST /api/agent/pair/poll", h.pairPoll)
	mux.HandleFunc("GET /api/pairings/{code}", h.getPairing)
	mux.HandleFunc("POST /api/pairings/{code}/approve", h.approvePairing)
	mux.HandleFunc("GET /api/computers/me", h.myComputers)
	mux.HandleFunc("DELETE /api/computers/{id}", h.revokeComputer)
```

- [ ] **Step 3: Pool-based test client**

Append to `handler_test.go`:

```go
// newPoolClient talks to a Handler backed by the shared pool (not a single
// transaction), for tests with concurrent requests (long-poll, downloads).
// Tables are emptied before and after.
func newPoolClient(t *testing.T) (*client, *Handler) {
	t.Helper()
	ctx := context.Background()
	truncate := func() {
		if _, err := testPool.Exec(ctx, "TRUNCATE run_files, agent_jobs, runs, pairings, computers, apps, sessions, people RESTART IDENTITY CASCADE"); err != nil {
			t.Fatal(err)
		}
	}
	truncate()
	h := New(sqlc.New(testPool), config.Config{BaseURL: "http://hub.test"})
	h.waitNext, h.waitFile, h.waitFolder, h.tick = 400*time.Millisecond, 2*time.Second, 2*time.Second, 20*time.Millisecond
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
		truncate()
	})
	jar, _ := cookiejar.New(nil)
	return &client{t: t, srv: srv, c: &http.Client{Jar: jar}}, h
}

// agent sends a request as an agent (Bearer token, no cookies).
func (c *client) agent(method, path, token string, body any, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, c.srv.URL+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}
```

Add `"time"` to the imports of `handler_test.go`.

- [ ] **Step 4: Write the failing tests** (`agents_test.go`)

```go
package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// pairComputer runs the whole pairing flow for the logged-in client c.
func pairComputer(t *testing.T, c *client, name string) string {
	t.Helper()
	var start agentproto.PairStartResponse
	if code := c.agent("POST", "/api/agent/pair/start", "", agentproto.PairStartRequest{ComputerName: name}, &start); code != http.StatusOK {
		t.Fatalf("pair start %d", code)
	}
	if code := c.do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil); code != http.StatusNoContent {
		t.Fatalf("approve %d", code)
	}
	var poll agentproto.PairPollResponse
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, &poll); code != http.StatusOK || poll.Token == "" {
		t.Fatalf("poll %d %+v", code, poll)
	}
	return poll.Token
}

func TestPairingFlow(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)

	var start agentproto.PairStartResponse
	c.agent("POST", "/api/agent/pair/start", "", agentproto.PairStartRequest{ComputerName: "NOTE-HENRY"}, &start)
	if !strings.HasPrefix(start.ApproveURL, "http://hub.test/conectar?codigo="+start.Code) {
		t.Fatalf("approve url %q", start.ApproveURL)
	}

	// Before approval the agent keeps waiting.
	var poll agentproto.PairPollResponse
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, &poll); code != http.StatusOK || poll.Status != "pending" {
		t.Fatalf("pending poll %d %+v", code, poll)
	}

	// The page shows the computer name; approving needs a login.
	var p struct {
		ComputerName string `json:"computer_name"`
		Expired      bool   `json:"expired"`
	}
	c.do("GET", "/api/pairings/"+start.Code, nil, &p)
	if p.ComputerName != "NOTE-HENRY" || p.Expired {
		t.Fatalf("pairing %+v", p)
	}
	if code := c.fresh().do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous approve %d", code)
	}
	c.do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil)

	c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, &poll)
	if poll.Status != "approved" || poll.Token == "" {
		t.Fatalf("approved poll %+v", poll)
	}
	// Single use.
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, nil); code != http.StatusGone {
		t.Fatalf("second poll %d", code)
	}

	var me struct {
		Computers []computerJSON `json:"computers"`
		Expected  string         `json:"expected_version"`
	}
	c.do("GET", "/api/computers/me", nil, &me)
	if len(me.Computers) != 1 || !me.Computers[0].Connected || me.Computers[0].Name != "NOTE-HENRY" || me.Expected != agentproto.Version {
		t.Fatalf("computers %+v", me)
	}
}

func TestPairingExpired(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	var start agentproto.PairStartResponse
	c.agent("POST", "/api/agent/pair/start", "", agentproto.PairStartRequest{ComputerName: "PC"}, &start)
	if _, err := testPool.Exec(context.Background(), "UPDATE pairings SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if code := c.do("POST", "/api/pairings/"+start.Code+"/approve", nil, nil); code != http.StatusGone {
		t.Fatalf("approve expired %d", code)
	}
	if code := c.agent("POST", "/api/agent/pair/poll", "", agentproto.PairPollRequest{Secret: start.Secret}, nil); code != http.StatusGone {
		t.Fatalf("poll expired %d", code)
	}
}

func TestComputerAuthAndRevoke(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	hello := agentproto.Hello{Name: "PC", Version: "0.0.9"}
	if code := c.agent("POST", "/api/agent/hello", "errado", hello, nil); code != http.StatusUnauthorized {
		t.Fatalf("bad token %d", code)
	}
	if code := c.agent("POST", "/api/agent/hello", token, hello, nil); code != http.StatusNoContent {
		t.Fatalf("hello %d", code)
	}
	var me struct {
		Computers []computerJSON `json:"computers"`
	}
	c.do("GET", "/api/computers/me", nil, &me)
	if !me.Computers[0].Outdated {
		t.Fatalf("want outdated: %+v", me.Computers[0])
	}

	// Someone else cannot disconnect it.
	other := c.fresh()
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	loginAs(t, other, "ana@alis.com")
	if code := other.do("DELETE", "/api/computers/1", nil, nil); code != http.StatusNotFound {
		t.Fatalf("other revoke %d", code)
	}
	if code := c.do("DELETE", "/api/computers/1", nil, nil); code != http.StatusNoContent {
		t.Fatalf("revoke %d", code)
	}
	if code := c.agent("POST", "/api/agent/hello", token, hello, nil); code != http.StatusUnauthorized {
		t.Fatalf("revoked token %d", code)
	}
}
```

Add helpers to `agents_test.go` (reuse if `handler_test.go` already has equivalents — check `auth_test.go` first and use existing names instead if present):

```go
func createPersonAs(t *testing.T, admin *client, name, email, role string) {
	t.Helper()
	body := map[string]string{"name": name, "email": email, "password": "senha-forte-1", "role": role}
	if code := admin.do("POST", "/api/people", body, nil); code != http.StatusCreated {
		t.Fatalf("create person %d", code)
	}
}

func loginAs(t *testing.T, c *client, email string) {
	t.Helper()
	if code := c.do("POST", "/api/auth/login", map[string]string{"email": email, "password": "senha-forte-1"}, nil); code != http.StatusOK {
		t.Fatalf("login %d", code)
	}
}
```

The `hello` endpoint is implemented in Task 6; this test's hello assertions are expected to fail until then. To keep the task self-contained, implement `agentHello` here too (Step 6 includes it).

- [ ] **Step 5: Run to verify they fail**

Run: `bash -c 'ROOT=$(git rev-parse --show-toplevel); set -a; . $ROOT/.env; set +a; cd $ROOT/backend && mise x -- go test ./internal/handler/ -run "Pairing|ComputerAuth"'` — Expected: compile FAIL (`undefined: computerJSON`).

- [ ] **Step 6: Implement `agents.go`**

```go
package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

func hashToken(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// pairingCode is short and easy to read: no 0/O, 1/I.
func pairingCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// currentComputer authenticates an agent by its Bearer token and records that
// it was seen.
func (h *Handler) currentComputer(w http.ResponseWriter, r *http.Request) (sqlc.Computer, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if ok && token != "" {
		c, err := h.q.GetComputerByToken(r.Context(), hashToken(token))
		if err == nil {
			if err := h.q.TouchComputer(r.Context(), c.ID); err != nil {
				internalError(w, "touch computer", err)
				return c, false
			}
			return c, true
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get computer", err)
			return sqlc.Computer{}, false
		}
	}
	writeError(w, http.StatusUnauthorized, "Agente não conectado.")
	return sqlc.Computer{}, false
}

func (h *Handler) pairStart(w http.ResponseWriter, r *http.Request) {
	var body agentproto.PairStartRequest
	if !readJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.ComputerName)
	if name == "" || len(name) > 100 {
		writeError(w, http.StatusBadRequest, "Informe o nome do computador.")
		return
	}
	code, err := pairingCode()
	if err != nil {
		internalError(w, "pairing code", err)
		return
	}
	secret, err := randomToken()
	if err != nil {
		internalError(w, "pairing secret", err)
		return
	}
	if err := h.q.CreatePairing(r.Context(), sqlc.CreatePairingParams{Code: code, SecretHash: hashToken(secret), ComputerName: name}); err != nil {
		internalError(w, "create pairing", err)
		return
	}
	writeJSON(w, http.StatusOK, agentproto.PairStartResponse{
		Code: code, Secret: secret, ApproveURL: strings.TrimRight(h.cfg.BaseURL, "/") + "/conectar?codigo=" + code,
	})
}

func (h *Handler) pairPoll(w http.ResponseWriter, r *http.Request) {
	var body agentproto.PairPollRequest
	if !readJSON(w, r, &body) {
		return
	}
	secretHash := hashToken(body.Secret)
	state, err := h.q.GetPairingStateBySecret(r.Context(), secretHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && state.Expired) {
		writeError(w, http.StatusGone, "Esse pedido de conexão venceu. Abra o Agente de novo.")
		return
	}
	if err != nil {
		internalError(w, "pairing state", err)
		return
	}
	if !state.PersonID.Valid {
		writeJSON(w, http.StatusOK, agentproto.PairPollResponse{Status: "pending"})
		return
	}
	p, err := h.q.ConsumePairing(r.Context(), secretHash)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusGone, "Esse pedido de conexão já foi usado.")
		return
	}
	if err != nil {
		internalError(w, "consume pairing", err)
		return
	}
	token, err := randomToken()
	if err != nil {
		internalError(w, "token", err)
		return
	}
	if _, err := h.q.CreateComputer(r.Context(), sqlc.CreateComputerParams{PersonID: p.PersonID.Int64, Name: p.ComputerName, TokenHash: hashToken(token)}); err != nil {
		internalError(w, "create computer", err)
		return
	}
	writeJSON(w, http.StatusOK, agentproto.PairPollResponse{Status: "approved", Token: token})
}

func (h *Handler) getPairing(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	p, err := h.q.GetPairing(r.Context(), r.PathValue("code"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Não encontrei esse pedido de conexão. Abra o Agente de novo.")
		return
	}
	if err != nil {
		internalError(w, "get pairing", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": p.Code, "computer_name": p.ComputerName, "expired": p.Expired, "approved": p.PersonID.Valid,
	})
}

func (h *Handler) approvePairing(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	n, err := h.q.ApprovePairing(r.Context(), sqlc.ApprovePairingParams{Code: r.PathValue("code"), PersonID: pgtype.Int8{Int64: me.ID, Valid: true}})
	if err != nil {
		internalError(w, "approve pairing", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusGone, "Esse pedido de conexão venceu ou já foi usado. Abra o Agente de novo.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type computerJSON struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	AgentVersion string     `json:"agent_version"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	Connected    bool       `json:"connected"`
	Outdated     bool       `json:"outdated"`
}

func (h *Handler) myComputers(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListPersonComputers(r.Context(), me.ID)
	if err != nil {
		internalError(w, "list computers", err)
		return
	}
	out := make([]computerJSON, 0, len(rows))
	for _, c := range rows {
		j := computerJSON{ID: c.ID, Name: c.Name, AgentVersion: c.AgentVersion, Connected: c.Connected,
			Outdated: c.AgentVersion != "" && c.AgentVersion != agentproto.Version}
		if c.LastSeenAt.Valid {
			j.LastSeenAt = &c.LastSeenAt.Time
		}
		out = append(out, j)
	}
	_, statErr := os.Stat(h.cfg.AgentBinaryPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"computers": out, "expected_version": agentproto.Version,
		"agent_available": h.cfg.AgentBinaryPath != "" && statErr == nil,
	})
}

func (h *Handler) revokeComputer(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Computador não encontrado.")
		return
	}
	c, err := h.q.GetComputer(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.PersonID != me.ID && me.Role != "admin") {
		writeError(w, http.StatusNotFound, "Computador não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get computer", err)
		return
	}
	if err := h.q.RevokeComputer(r.Context(), id); err != nil {
		internalError(w, "revoke computer", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// agentHello is called when the agent starts. Runs that were open on this
// computer cannot be finished any more.
func (h *Handler) agentHello(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	var body agentproto.Hello
	if !readJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = c.Name
	}
	if err := h.q.HelloComputer(r.Context(), sqlc.HelloComputerParams{ID: c.ID, Name: name, AgentVersion: body.Version}); err != nil {
		internalError(w, "hello", err)
		return
	}
	if err := h.q.InterruptTakenRuns(r.Context(), c.ID); err != nil {
		internalError(w, "interrupt runs", err)
		return
	}
	if err := h.q.FailTakenJobs(r.Context(), c.ID); err != nil {
		internalError(w, "fail jobs", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Register `mux.HandleFunc("POST /api/agent/hello", h.agentHello)`. Match the generated field names from Task 4 Step 4 (e.g. `ListPersonComputersRow.Connected`, `GetPairingRow.Expired`); adjust if sqlc named them differently.

- [ ] **Step 7: Run to verify they pass**

Run the Step 5 command — Expected: `ok`.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/handler backend/internal/config
git commit -m "Hub: pair computers with people and authenticate agents"
```

---

### Task 6: Job queue — `next` long-poll and results (Hub)

**Files:**
- Modify: `backend/internal/handler/agents.go`, `backend/internal/handler/handler.go`, `backend/internal/handler/agents_test.go`

**Interfaces:**
- Consumes: `currentComputer`, sqlc `ClaimJob`, `ExpireStaleRuns`, `ExpireStaleJobs`, `GetJob`, `FinishJob`.
- Produces:
  - `GET /api/agent/next` → 200 `agentproto.Job` or 204 after `h.waitNext`
  - `POST /api/agent/jobs/{id}/result` (body `agentproto.Result`) → 204; dispatches to `h.onRunResult(ctx, job, res) error` (Task 7) and `h.uploads.missing(jobID)` (Task 8); for `choose_folder` it finishes the job with the result JSON.
  - `func (h *Handler) waitJob(ctx context.Context, id int64, max time.Duration) (sqlc.AgentJob, bool)` — polls until status is `done`/`failed`.
  - `func (h *Handler) enqueue(ctx context.Context, computerID int64, runID *int64, kind string, payload any) (sqlc.AgentJob, error)`

- [ ] **Step 1: Write the failing tests** (append to `agents_test.go`)

```go
func TestNextLongPollAndChooseFolderResult(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	// Nothing queued: 204 after the wait.
	if code := c.agent("GET", "/api/agent/next", token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("empty next %d", code)
	}

	ctx := context.Background()
	job, err := h.enqueue(ctx, 1, nil, agentproto.KindChooseFolder, agentproto.ChooseFolderPayload{Initial: `C:\apps`})
	if err != nil {
		t.Fatal(err)
	}
	var got agentproto.Job
	if code := c.agent("GET", "/api/agent/next", token, nil, &got); code != http.StatusOK || got.ID != job.ID || got.Kind != agentproto.KindChooseFolder {
		t.Fatalf("next %d %+v", code, got)
	}
	// Claimed once only.
	if code := c.agent("GET", "/api/agent/next", token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("second next %d", code)
	}

	res := agentproto.Result{Event: agentproto.EventDone, Folder: `D:\Saídas`}
	if code := c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, res, nil); code != http.StatusNoContent {
		t.Fatalf("result %d", code)
	}
	done, ok := h.waitJob(ctx, job.ID, time.Second)
	if !ok || done.Status != "done" || !strings.Contains(string(done.Result), `D:\\Saídas`) {
		t.Fatalf("job %+v", done)
	}
}

func TestResultForOtherComputersJobIsRejected(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	pairComputer(t, c, "PC1")
	token2 := pairComputer(t, c, "PC2")
	job, err := h.enqueue(context.Background(), 1, nil, agentproto.KindChooseFolder, agentproto.ChooseFolderPayload{})
	if err != nil {
		t.Fatal(err)
	}
	res := agentproto.Result{Event: agentproto.EventDone, Canceled: true}
	if code := c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token2, res, nil); code != http.StatusNotFound {
		t.Fatalf("foreign result %d", code)
	}
}
```

Add `"strconv"` and `"time"` imports.

- [ ] **Step 2: Run to verify they fail**

Run: `... go test ./internal/handler/ -run "NextLongPoll|OtherComputers"` — Expected: compile FAIL (`h.enqueue undefined`).

- [ ] **Step 3: Implement** (append to `agents.go`)

```go
func (h *Handler) enqueue(ctx context.Context, computerID int64, runID *int64, kind string, payload any) (sqlc.AgentJob, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return sqlc.AgentJob{}, err
	}
	run := pgtype.Int8{}
	if runID != nil {
		run = pgtype.Int8{Int64: *runID, Valid: true}
	}
	return h.q.CreateJob(ctx, sqlc.CreateJobParams{ComputerID: computerID, RunID: run, Kind: kind, Payload: raw})
}

// waitJob polls until the job is done or failed, or max passes.
func (h *Handler) waitJob(ctx context.Context, id int64, max time.Duration) (sqlc.AgentJob, bool) {
	deadline := time.Now().Add(max)
	for {
		j, err := h.q.GetJob(ctx, id)
		if err == nil && (j.Status == "done" || j.Status == "failed") {
			return j, true
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return j, false
		}
		time.Sleep(h.tick)
	}
}

func (h *Handler) agentNext(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := h.q.ExpireStaleRuns(ctx); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	if err := h.q.ExpireStaleJobs(ctx); err != nil {
		internalError(w, "expire jobs", err)
		return
	}
	deadline := time.Now().Add(h.waitNext)
	for {
		j, err := h.q.ClaimJob(ctx, c.ID)
		if err == nil {
			writeJSON(w, http.StatusOK, agentproto.Job{ID: j.ID, Kind: j.Kind, Payload: j.Payload})
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "claim job", err)
			return
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		time.Sleep(h.tick)
	}
}

// jobForComputer loads a job by path id and checks it belongs to c.
func (h *Handler) jobForComputer(w http.ResponseWriter, r *http.Request, c sqlc.Computer) (sqlc.AgentJob, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		j, err := h.q.GetJob(r.Context(), id)
		if err == nil && j.ComputerID == c.ID {
			return j, true
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get job", err)
			return j, false
		}
	}
	writeError(w, http.StatusNotFound, "Pedido não encontrado.")
	return sqlc.AgentJob{}, false
}

func (h *Handler) agentResult(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	j, ok := h.jobForComputer(w, r, c)
	if !ok {
		return
	}
	var res agentproto.Result
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20) // big run file lists
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		writeError(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	ctx := r.Context()
	var err error
	switch j.Kind {
	case agentproto.KindRun:
		err = h.onRunResult(ctx, j, res)
	case agentproto.KindSendFile:
		if res.Event == agentproto.EventMissing || res.Event == agentproto.EventFailed {
			h.uploads.missing(j.ID)
		}
		err = h.finishJob(ctx, j.ID, "failed", res)
	default:
		status := "done"
		if res.Event == agentproto.EventFailed {
			status = "failed"
		}
		err = h.finishJob(ctx, j.ID, status, res)
	}
	if err != nil {
		internalError(w, "job result", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) finishJob(ctx context.Context, id int64, status string, res agentproto.Result) error {
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return h.q.FinishJob(ctx, sqlc.FinishJobParams{ID: id, Status: status, Result: raw})
}
```

Add imports `context`, `encoding/json`. Register:

```go
	mux.HandleFunc("GET /api/agent/next", h.agentNext)
	mux.HandleFunc("POST /api/agent/jobs/{id}/result", h.agentResult)
```

`onRunResult` and `h.uploads` don't exist yet. To compile in this task, add temporary minimal versions at the bottom of `agents.go`, replaced in Tasks 7 and 8:

```go
func (h *Handler) onRunResult(ctx context.Context, j sqlc.AgentJob, res agentproto.Result) error {
	return h.finishJob(ctx, j.ID, "done", res)
}
```

and in `handler.go` add the field `uploads *uploadHub` plus, in a new file `uploads.go`:

```go
package handler

type uploadHub struct{}

func newUploadHub() *uploadHub         { return &uploadHub{} }
func (u *uploadHub) missing(id int64) {}
```

and `uploads: newUploadHub(),` in `New`.

- [ ] **Step 4: Run to verify they pass**

Run: `... go test ./internal/handler/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler
git commit -m "Hub: agent job queue with long-poll and results"
```

---

### Task 7: Runs (Hub)

**Files:**
- Create: `backend/internal/handler/runs.go`, `backend/internal/handler/runs_test.go`
- Modify: `backend/internal/handler/agents.go` (remove the temporary `onRunResult`), `backend/internal/handler/apps.go` (`can_run`), `backend/internal/handler/handler.go` (routes)

**Interfaces:**
- Consumes: `enqueue`, `waitJob`, `finishJob`, `outputs.Match`, `hostpath.ToDisplay`, `manifest.Manifest`.
- Produces:
  - `func canRun(m manifest.Manifest) bool`
  - `appDetail.CanRun bool` (`json:"can_run"`)
  - `POST /api/apps/{id}/runs` body `{output_folder}` → 201 `runJSON`; 409 `"Seu computador não está conectado."` when none
  - `GET /api/apps/{id}/runs` → `[]runSummaryJSON`
  - `GET /api/runs/{id}` → `runJSON`
  - `POST /api/choose-folder` body `{initial}` → `{folder, canceled}`
  - `func (h *Handler) onRunResult(ctx, j sqlc.AgentJob, res agentproto.Result) error`
  - `func (h *Handler) runForPerson(w, r) (sqlc.GetRunRow, sqlc.Person, bool)` (owner or admin, else 404)
  - JSON types:

```go
type runFileJSON struct {
	ID       int64     `json:"id"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Change   string    `json:"change"`
}
type runGroupJSON struct {
	Index int           `json:"index"`
	Label string        `json:"label"`
	Files []runFileJSON `json:"files"`
}
type runJSON struct {
	ID           int64          `json:"id"`
	AppID        int64          `json:"app_id"`
	AppName      string         `json:"app_name"`
	PersonName   string         `json:"person_name"`
	ComputerName string         `json:"computer_name"`
	OutputFolder string         `json:"output_folder"`
	Status       string         `json:"status"`
	ExitCode     *int32         `json:"exit_code"`
	ErrorMessage string         `json:"error_message"`
	AppVersion   string         `json:"app_version"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    *time.Time     `json:"started_at"`
	FinishedAt   *time.Time     `json:"finished_at"`
	Groups       []runGroupJSON `json:"groups"`
}
type runSummaryJSON struct {
	ID         int64      `json:"id"`
	Status     string     `json:"status"`
	PersonName string     `json:"person_name"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	FileCount  int64      `json:"file_count"`
}
```

- [ ] **Step 1: Write the failing tests** (`runs_test.go`)

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func filesPrepManifest() manifest.Manifest {
	return manifest.Manifest{
		ManifestVersion: 1, ID: "eq-research-files-prep", Name: "Eq. Research Files Prep", Kind: "desktop",
		Entrypoint: "Ferramenta CVM.exe",
		Outputs: []manifest.Output{
			{Label: "Documentos da CVM", Pattern: "*/*/*.pdf"},
			{Label: "Data Pack", Pattern: "*/MD_Output/*_DataPack_Consolidado.md"},
			{Label: "FRE cortado", Pattern: "*_cortado_secoes.pdf"},
		},
	}
}

// insertApp stores an app directly (the registration flow is tested elsewhere).
func insertApp(t *testing.T, m manifest.Manifest) int64 {
	t.Helper()
	raw, _ := json.Marshal(m)
	var id int64
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO apps (slug, name, kind, folder_path, owner_id, manifest, created_by)
		 VALUES ($1, $2, $3, '/mnt/c/apps/files-prep', 1, $4, 1) RETURNING id`,
		m.ID, m.Name, m.Kind, raw).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func appPath(id int64, suffix string) string { return "/api/apps/" + strconv.FormatInt(id, 10) + suffix }

func TestCanRun(t *testing.T) {
	m := filesPrepManifest()
	if !canRun(m) {
		t.Fatal("exe desktop should run")
	}
	m.Entrypoint = "app.py"
	if canRun(m) {
		t.Fatal("py should not run in this phase")
	}
	m.Entrypoint, m.Kind = "x.EXE", "desktop"
	if !canRun(m) {
		t.Fatal("case-insensitive exe")
	}
}

func TestRunWithoutComputer(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	var e map[string]string
	if code := c.do("POST", appPath(app, "/runs"), map[string]string{}, &e); code != http.StatusConflict || e["error"] != "Seu computador não está conectado." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestRunLifecycle(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")

	var run runJSON
	if code := c.do("POST", appPath(app, "/runs"), map[string]string{}, &run); code != http.StatusCreated {
		t.Fatalf("create run %d", code)
	}
	if run.Status != "waiting" || run.OutputFolder != `C:\apps\files-prep` {
		t.Fatalf("run %+v", run)
	}

	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	var p agentproto.RunPayload
	_ = json.Unmarshal(job.Payload, &p)
	if job.Kind != agentproto.KindRun || p.RunID != run.ID || p.Exe != "Ferramenta CVM.exe" || p.AppFolder != `C:\apps\files-prep` {
		t.Fatalf("job %+v payload %+v", job, p)
	}
	jobPath := "/api/agent/jobs/" + strconv.FormatInt(job.ID, 10) + "/result"

	c.agent("POST", jobPath, token, agentproto.Result{Event: agentproto.EventStarted, AppVersion: "2026-08-13 17:54 · 75,6 MB"}, nil)
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "running" || run.StartedAt == nil {
		t.Fatalf("running %+v", run)
	}

	now := time.Now()
	files := []agentproto.FileEntry{
		{Path: "PETROBRAS/FRE/FRE_2025.pdf", Size: 10, Modified: now, Change: "new"},
		{Path: "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", Size: 20, Modified: now, Change: "new"},
		{Path: "log.txt", Size: 1, Modified: now, Change: "changed"},
	}
	c.agent("POST", jobPath, token, agentproto.Result{Event: agentproto.EventFinished, Files: files}, nil)
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "done" || run.FinishedAt == nil || run.AppVersion == "" {
		t.Fatalf("done %+v", run)
	}
	labels := []string{}
	counts := []int{}
	for _, g := range run.Groups {
		labels = append(labels, g.Label)
		counts = append(counts, len(g.Files))
	}
	want := []string{"Documentos da CVM", "Data Pack", "FRE cortado", "Outros arquivos"}
	if len(labels) != 4 || labels[0] != want[0] || labels[1] != want[1] || labels[2] != want[2] || labels[3] != want[3] {
		t.Fatalf("labels %v", labels)
	}
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 0 || counts[3] != 1 {
		t.Fatalf("counts %v", counts)
	}

	var list []runSummaryJSON
	c.do("GET", appPath(app, "/runs"), nil, &list)
	if len(list) != 1 || list[0].FileCount != 3 {
		t.Fatalf("list %+v", list)
	}
}

func TestRunNoFilesAndFailure(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")

	for _, tc := range []struct {
		res    agentproto.Result
		status string
	}{
		{agentproto.Result{Event: agentproto.EventFinished}, "no_files"},
		{agentproto.Result{Event: agentproto.EventFailed, Error: "Já existe um Ferramenta CVM.exe nessa pasta."}, "error"},
	} {
		var run runJSON
		c.do("POST", appPath(app, "/runs"), map[string]string{"output_folder": `D:\Saídas`}, &run)
		var job agentproto.Job
		c.agent("GET", "/api/agent/next", token, nil, &job)
		c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, tc.res, nil)
		c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
		if run.Status != tc.status || run.OutputFolder != `D:\Saídas` || run.ErrorMessage != tc.res.Error {
			t.Fatalf("%s: %+v", tc.status, run)
		}
	}
}

func TestRunRejectsBadFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	if code := c.do("POST", appPath(app, "/runs"), map[string]string{"output_folder": "pasta relativa"}, nil); code != http.StatusBadRequest {
		t.Fatalf("relative folder %d", code)
	}
}

func TestStaleRunIsInterrupted(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	if _, err := testPool.Exec(context.Background(), "UPDATE agent_jobs SET created_at = now() - interval '3 minutes'"); err != nil {
		t.Fatal(err)
	}
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "interrupted" || run.ErrorMessage == "" {
		t.Fatalf("stale %+v", run)
	}
}

func TestHelloInterruptsOpenRun(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version}, nil)
	c.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, &run)
	if run.Status != "interrupted" {
		t.Fatalf("after hello %+v", run)
	}
}

func TestRunVisibility(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)

	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("GET", "/api/runs/"+strconv.FormatInt(run.ID, 10), nil, nil); code != http.StatusNotFound {
		t.Fatalf("other person sees run: %d", code)
	}
	var list []runSummaryJSON
	ana.do("GET", appPath(app, "/runs"), nil, &list)
	if len(list) != 0 {
		t.Fatalf("other person lists %d runs", len(list))
	}
}

func TestChooseFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	go func() {
		var job agentproto.Job
		c.agent("GET", "/api/agent/next", token, nil, &job)
		c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token,
			agentproto.Result{Event: agentproto.EventDone, Folder: `C:\Users\Henry\Relatórios`}, nil)
	}()
	var out struct {
		Folder   string `json:"folder"`
		Canceled bool   `json:"canceled"`
	}
	if code := c.do("POST", "/api/choose-folder", map[string]string{"initial": `C:\`}, &out); code != http.StatusOK || out.Folder != `C:\Users\Henry\Relatórios` {
		t.Fatalf("%d %+v", code, out)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `... go test ./internal/handler/ -run "Run|ChooseFolder|CanRun|Stale|Hello"` — Expected: compile FAIL.

- [ ] **Step 3: Implement `runs.go`**

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/hostpath"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/outputs"
)

var windowsAbs = regexp.MustCompile(`^[A-Za-z]:\\`)

// canRun: in this phase only desktop apps opened by an .exe run through the agent.
func canRun(m manifest.Manifest) bool {
	return m.Kind == "desktop" && strings.HasSuffix(strings.ToLower(m.Entrypoint), ".exe")
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func (h *Handler) startRun(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return
	}
	var body struct {
		OutputFolder string `json:"output_folder"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	ctx := r.Context()
	a, err := h.q.GetApp(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get app", err)
		return
	}
	var m manifest.Manifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		internalError(w, "decode manifest", err)
		return
	}
	if !canRun(m) {
		writeError(w, http.StatusBadRequest, "Este app ainda não pode rodar pelo Hub.")
		return
	}
	appFolder := hostpath.ToDisplay(a.FolderPath)
	out := strings.TrimSpace(body.OutputFolder)
	if out == "" {
		out = appFolder
	}
	if !windowsAbs.MatchString(out) {
		writeError(w, http.StatusBadRequest, `Informe a pasta completa, começando pela letra do disco (ex.: C:\Users\voce\Documentos).`)
		return
	}
	comp, err := h.q.ConnectedComputerForPerson(ctx, me.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "Seu computador não está conectado.")
		return
	}
	if err != nil {
		internalError(w, "connected computer", err)
		return
	}
	run, err := h.q.CreateRun(ctx, sqlc.CreateRunParams{AppID: a.ID, PersonID: me.ID, ComputerID: comp.ID, OutputFolder: out})
	if err != nil {
		internalError(w, "create run", err)
		return
	}
	payload := agentproto.RunPayload{RunID: run.ID, AppFolder: appFolder, Exe: m.Entrypoint, OutputFolder: out}
	if _, err := h.enqueue(ctx, comp.ID, &run.ID, agentproto.KindRun, payload); err != nil {
		internalError(w, "enqueue run", err)
		return
	}
	row, err := h.q.GetRun(ctx, run.ID)
	if err != nil {
		internalError(w, "get run", err)
		return
	}
	j, err := h.toRunJSON(ctx, row)
	if err != nil {
		internalError(w, "run json", err)
		return
	}
	writeJSON(w, http.StatusCreated, j)
}

// onRunResult applies an agent event to the run.
func (h *Handler) onRunResult(ctx context.Context, j sqlc.AgentJob, res agentproto.Result) error {
	if !j.RunID.Valid {
		return h.finishJob(ctx, j.ID, "failed", res)
	}
	runID := j.RunID.Int64
	switch res.Event {
	case agentproto.EventStarted:
		return h.q.StartRun(ctx, sqlc.StartRunParams{ID: runID, AppVersion: res.AppVersion})
	case agentproto.EventFinished:
		row, err := h.q.GetRun(ctx, runID)
		if err != nil {
			return err
		}
		var m manifest.Manifest
		if err := json.Unmarshal(row.AppManifest, &m); err != nil {
			return err
		}
		for _, f := range res.Files {
			idx, label := outputs.Match(m.Outputs, f.Path)
			if err := h.q.AddRunFile(ctx, sqlc.AddRunFileParams{
				RunID: runID, RelPath: f.Path, Size: f.Size,
				ModifiedAt: pgtype.Timestamptz{Time: f.Modified, Valid: true},
				Change:     f.Change, GroupIndex: int32(idx), GroupLabel: label,
			}); err != nil {
				return err
			}
		}
		status, msg := "done", ""
		if len(res.Files) == 0 {
			status = "no_files"
			if res.ExitCode != 0 {
				status, msg = "error", "O app fechou com erro (código "+strconv.Itoa(res.ExitCode)+") e não gerou arquivos."
			}
		}
		if err := h.q.FinishRun(ctx, sqlc.FinishRunParams{ID: runID, Status: status,
			ExitCode: pgtype.Int4{Int32: int32(res.ExitCode), Valid: true}, ErrorMessage: msg}); err != nil {
			return err
		}
		return h.finishJob(ctx, j.ID, "done", res)
	default: // failed
		msg := res.Error
		if msg == "" {
			msg = "O Agente não conseguiu abrir o app."
		}
		if err := h.q.FinishRun(ctx, sqlc.FinishRunParams{ID: runID, Status: "error", ErrorMessage: msg}); err != nil {
			return err
		}
		return h.finishJob(ctx, j.ID, "failed", res)
	}
}

func (h *Handler) toRunJSON(ctx context.Context, row sqlc.GetRunRow) (runJSON, error) {
	j := runJSON{
		ID: row.ID, AppID: row.AppID, AppName: row.AppName, PersonName: row.PersonName, ComputerName: row.ComputerName,
		OutputFolder: row.OutputFolder, Status: row.Status, ErrorMessage: row.ErrorMessage, AppVersion: row.AppVersion,
		CreatedAt: row.CreatedAt.Time, StartedAt: tsPtr(row.StartedAt), FinishedAt: tsPtr(row.FinishedAt),
		Groups: []runGroupJSON{},
	}
	if row.ExitCode.Valid {
		j.ExitCode = &row.ExitCode.Int32
	}
	var m manifest.Manifest
	if err := json.Unmarshal(row.AppManifest, &m); err != nil {
		return j, err
	}
	for i, o := range m.Outputs {
		j.Groups = append(j.Groups, runGroupJSON{Index: i, Label: o.Label, Files: []runFileJSON{}})
	}
	files, err := h.q.ListRunFiles(ctx, row.ID)
	if err != nil {
		return j, err
	}
	for _, f := range files {
		fj := runFileJSON{ID: f.ID, Path: f.RelPath, Size: f.Size, Modified: f.ModifiedAt.Time, Change: f.Change}
		idx := int(f.GroupIndex)
		if idx < 0 || idx >= len(m.Outputs) || m.Outputs[idx].Label != f.GroupLabel {
			idx = -1 // others, or a group the manifest no longer has
		}
		if idx >= 0 {
			j.Groups[idx].Files = append(j.Groups[idx].Files, fj)
			continue
		}
		last := len(j.Groups) - 1
		if last < 0 || j.Groups[last].Index != -1 {
			j.Groups = append(j.Groups, runGroupJSON{Index: -1, Label: outputs.OtherLabel, Files: []runFileJSON{}})
			last++
		}
		j.Groups[last].Files = append(j.Groups[last].Files, fj)
	}
	return j, nil
}

// runForPerson loads the run in the path; only its person and admins see it.
func (h *Handler) runForPerson(w http.ResponseWriter, r *http.Request) (sqlc.GetRunRow, sqlc.Person, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return sqlc.GetRunRow{}, me, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
			internalError(w, "expire runs", err)
			return sqlc.GetRunRow{}, me, false
		}
		row, err := h.q.GetRun(r.Context(), id)
		if err == nil && (row.PersonID == me.ID || me.Role == "admin") {
			return row, me, true
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			internalError(w, "get run", err)
			return row, me, false
		}
	}
	writeError(w, http.StatusNotFound, "Execução não encontrada.")
	return sqlc.GetRunRow{}, me, false
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	row, _, ok := h.runForPerson(w, r)
	if !ok {
		return
	}
	j, err := h.toRunJSON(r.Context(), row)
	if err != nil {
		internalError(w, "run json", err)
		return
	}
	writeJSON(w, http.StatusOK, j)
}

func (h *Handler) listAppRuns(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return
	}
	if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	rows, err := h.q.ListAppRuns(r.Context(), sqlc.ListAppRunsParams{AppID: id, SeeAll: me.Role == "admin", PersonID: me.ID})
	if err != nil {
		internalError(w, "list runs", err)
		return
	}
	out := make([]runSummaryJSON, 0, len(rows))
	for _, x := range rows {
		out = append(out, runSummaryJSON{ID: x.ID, Status: x.Status, PersonName: x.PersonName, CreatedAt: x.CreatedAt.Time,
			StartedAt: tsPtr(x.StartedAt), FinishedAt: tsPtr(x.FinishedAt), FileCount: x.FileCount})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) chooseFolder(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	var body struct {
		Initial string `json:"initial"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	ctx := r.Context()
	comp, err := h.q.ConnectedComputerForPerson(ctx, me.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "Seu computador não está conectado.")
		return
	}
	if err != nil {
		internalError(w, "connected computer", err)
		return
	}
	job, err := h.enqueue(ctx, comp.ID, nil, agentproto.KindChooseFolder, agentproto.ChooseFolderPayload{Initial: body.Initial})
	if err != nil {
		internalError(w, "enqueue folder", err)
		return
	}
	done, ok := h.waitJob(ctx, job.ID, h.waitFolder)
	if !ok {
		writeError(w, http.StatusGatewayTimeout, "A janela de pastas não respondeu. Tente de novo.")
		return
	}
	var res agentproto.Result
	_ = json.Unmarshal(done.Result, &res)
	if done.Status == "failed" {
		writeError(w, http.StatusBadGateway, "Não consegui abrir a janela de pastas no seu computador.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folder": res.Folder, "canceled": res.Canceled || res.Folder == ""})
}
```

Delete the temporary `onRunResult` from `agents.go`. Register routes:

```go
	mux.HandleFunc("POST /api/apps/{id}/runs", h.startRun)
	mux.HandleFunc("GET /api/apps/{id}/runs", h.listAppRuns)
	mux.HandleFunc("GET /api/runs/{id}", h.getRun)
	mux.HandleFunc("POST /api/choose-folder", h.chooseFolder)
```

In `apps.go`, add `CanRun bool \`json:"can_run"\`` to `appDetail` and set `d.CanRun = canRun(d.Manifest)` in `toAppDetail` after unmarshalling (keep returning the unmarshal error).

- [ ] **Step 4: Run to verify they pass**

Run: `... go test ./internal/handler/` — Expected: `ok` (all tests, old and new).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler
git commit -m "Hub: runs, folder choice and run results grouped by manifest"
```

---

### Task 8: Downloads through the agent (Hub)

**Files:**
- Modify: `backend/internal/handler/uploads.go` (replace the stub), `backend/internal/handler/runs.go`, `backend/internal/handler/handler.go` (routes)
- Test: `backend/internal/handler/runs_test.go`

**Interfaces:**
- Consumes: `runForPerson`, `enqueue`, `IsComputerConnected`, `GetRunFile`, `ListRunFiles`.
- Produces:
  - `GET /api/runs/{id}/files/{fileId}` and `GET /api/runs/{id}/groups/{index}/zip` (index `-1` = others)
  - `POST /api/agent/jobs/{id}/upload` (raw body)
  - `uploadHub` with `expect(id) <-chan upload`, `cancel(id)`, `deliver(id, body io.Reader, wait time.Duration) (<-chan error, bool)`, `missing(id)`

- [ ] **Step 1: Write the failing tests** (append to `runs_test.go`)

```go
// finishedRun creates a run with the given files and returns run id, token.
func finishedRun(t *testing.T, c *client, files []agentproto.FileEntry) (int64, string) {
	t.Helper()
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")
	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	c.agent("POST", "/api/agent/jobs/"+strconv.FormatInt(job.ID, 10)+"/result", token, agentproto.Result{Event: agentproto.EventFinished, Files: files}, nil)
	return run.ID, token
}

// fakeAgentSend answers one send_file job with body, or reports missing.
func fakeAgentSend(t *testing.T, c *client, token string, body string, missing bool) <-chan agentproto.SendFilePayload {
	got := make(chan agentproto.SendFilePayload, 1)
	go func() {
		var job agentproto.Job
		for job.ID == 0 {
			c.agent("GET", "/api/agent/next", token, nil, &job)
		}
		var p agentproto.SendFilePayload
		_ = json.Unmarshal(job.Payload, &p)
		got <- p
		id := strconv.FormatInt(job.ID, 10)
		if missing {
			c.agent("POST", "/api/agent/jobs/"+id+"/result", token, agentproto.Result{Event: agentproto.EventMissing}, nil)
			return
		}
		req, _ := http.NewRequest("POST", c.srv.URL+"/api/agent/jobs/"+id+"/upload", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
	return got
}

func TestDownloadFile(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, token := finishedRun(t, c, []agentproto.FileEntry{
		{Path: "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", Size: 5, Modified: time.Now(), Change: "new"},
	})
	sent := fakeAgentSend(t, c, token, "# Data", false)

	resp, err := c.c.Get(c.srv.URL + "/api/runs/" + strconv.FormatInt(runID, 10) + "/files/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "# Data" {
		t.Fatalf("%d %q", resp.StatusCode, b)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "PETROBRAS_DataPack_Consolidado.md") {
		t.Fatalf("disposition %q", cd)
	}
	p := <-sent
	if p.Zip || len(p.Paths) != 1 || p.Paths[0] != "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md" {
		t.Fatalf("payload %+v", p)
	}
}

func TestDownloadMissingFile(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, token := finishedRun(t, c, []agentproto.FileEntry{{Path: "a.txt", Size: 1, Modified: time.Now(), Change: "new"}})
	fakeAgentSend(t, c, token, "", true)
	var e map[string]string
	if code := c.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/1", nil, &e); code != http.StatusNotFound || e["error"] != "Esse arquivo não está mais na pasta." {
		t.Fatalf("%d %v", code, e)
	}
}

func TestDownloadGroupZipPayload(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	now := time.Now()
	runID, token := finishedRun(t, c, []agentproto.FileEntry{
		{Path: "P/FRE/a.pdf", Size: 1, Modified: now, Change: "new"},
		{Path: "P/DFP/b.pdf", Size: 1, Modified: now, Change: "new"},
		{Path: "log.txt", Size: 1, Modified: now, Change: "new"},
	})
	sent := fakeAgentSend(t, c, token, "PK", false)
	resp, err := c.c.Get(c.srv.URL + "/api/runs/" + strconv.FormatInt(runID, 10) + "/groups/0/zip")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	p := <-sent
	if !p.Zip || len(p.Paths) != 2 || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("payload %+v type %q", p, resp.Header.Get("Content-Type"))
	}
}

func TestDownloadRefusesUnknownFileAndOtherPerson(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, _ := finishedRun(t, c, []agentproto.FileEntry{{Path: "a.txt", Size: 1, Modified: time.Now(), Change: "new"}})
	if code := c.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/999", nil, nil); code != http.StatusNotFound {
		t.Fatalf("unknown file %d", code)
	}
	createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	ana := c.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/1", nil, nil); code != http.StatusNotFound {
		t.Fatalf("other person %d", code)
	}
	var jobs int
	_ = testPool.QueryRow(context.Background(), "SELECT count(*) FROM agent_jobs WHERE kind = 'send_file'").Scan(&jobs)
	if jobs != 0 {
		t.Fatalf("refused downloads must not reach the agent, got %d jobs", jobs)
	}
}

func TestDownloadComputerOffline(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	runID, _ := finishedRun(t, c, []agentproto.FileEntry{{Path: "a.txt", Size: 1, Modified: time.Now(), Change: "new"}})
	_, _ = testPool.Exec(context.Background(), "UPDATE computers SET last_seen_at = now() - interval '5 minutes'")
	if code := c.do("GET", "/api/runs/"+strconv.FormatInt(runID, 10)+"/files/1", nil, nil); code != http.StatusConflict {
		t.Fatalf("offline %d", code)
	}
}
```

Add `"io"` and `"strings"` imports.

- [ ] **Step 2: Run to verify they fail**

Run: `... go test ./internal/handler/ -run Download` — Expected: FAIL (404s: routes missing).

- [ ] **Step 3: Implement `uploads.go`**

```go
package handler

import (
	"io"
	"sync"
	"time"
)

// upload is one agent upload handed to the waiting browser request.
type upload struct {
	body    io.Reader
	missing bool
	done    chan error // the browser side reports when it finished copying
}

// uploadHub pairs a browser download (waiting on a send_file job) with the
// agent's upload for that job. The Hub is a single process in the pilot.
type uploadHub struct {
	mu      sync.Mutex
	waiting map[int64]chan upload
}

func newUploadHub() *uploadHub { return &uploadHub{waiting: map[int64]chan upload{}} }

func (u *uploadHub) expect(id int64) <-chan upload {
	u.mu.Lock()
	defer u.mu.Unlock()
	ch := make(chan upload, 1)
	u.waiting[id] = ch
	return ch
}

func (u *uploadHub) cancel(id int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.waiting, id)
}

func (u *uploadHub) take(id int64, wait time.Duration) (chan upload, bool) {
	deadline := time.Now().Add(wait)
	for {
		u.mu.Lock()
		ch, ok := u.waiting[id]
		if ok {
			delete(u.waiting, id)
		}
		u.mu.Unlock()
		if ok || time.Now().After(deadline) {
			return ch, ok
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// deliver hands body to the waiting browser; the returned channel reports
// when the copy ended.
func (u *uploadHub) deliver(id int64, body io.Reader, wait time.Duration) (<-chan error, bool) {
	ch, ok := u.take(id, wait)
	if !ok {
		return nil, false
	}
	done := make(chan error, 1)
	ch <- upload{body: body, done: done}
	return done, true
}

// missing tells the waiting browser that the file is gone.
func (u *uploadHub) missing(id int64) {
	if ch, ok := u.take(id, 0); ok {
		ch <- upload{missing: true}
	}
}
```

- [ ] **Step 4: Implement the download handlers** (append to `runs.go`)

```go
func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request) {
	row, _, ok := h.runForPerson(w, r)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(r.PathValue("fileId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Arquivo não encontrado.")
		return
	}
	f, err := h.q.GetRunFile(r.Context(), sqlc.GetRunFileParams{ID: fileID, RunID: row.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Arquivo não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get run file", err)
		return
	}
	name := f.RelPath[strings.LastIndex(f.RelPath, "/")+1:]
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	h.relayFromAgent(w, r, row, agentproto.SendFilePayload{RunID: row.ID, Paths: []string{f.RelPath}}, name, ctype)
}

func (h *Handler) downloadGroup(w http.ResponseWriter, r *http.Request) {
	row, _, ok := h.runForPerson(w, r)
	if !ok {
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		writeError(w, http.StatusNotFound, "Grupo não encontrado.")
		return
	}
	j, err := h.toRunJSON(r.Context(), row)
	if err != nil {
		internalError(w, "run json", err)
		return
	}
	for _, g := range j.Groups {
		if g.Index != index || len(g.Files) == 0 {
			continue
		}
		paths := make([]string, 0, len(g.Files))
		for _, f := range g.Files {
			paths = append(paths, f.Path)
		}
		name := strings.Map(func(r rune) rune {
			if strings.ContainsRune(`\/:*?"<>|`, r) {
				return '-'
			}
			return r
		}, row.AppName+" - "+g.Label) + ".zip"
		h.relayFromAgent(w, r, row, agentproto.SendFilePayload{RunID: row.ID, Paths: paths, Zip: true}, name, "application/zip")
		return
	}
	writeError(w, http.StatusNotFound, "Esse grupo não tem arquivos.")
}

// relayFromAgent asks the run's computer for the files and streams its upload
// to the browser.
func (h *Handler) relayFromAgent(w http.ResponseWriter, r *http.Request, row sqlc.GetRunRow, p agentproto.SendFilePayload, name, ctype string) {
	ctx := r.Context()
	connected, err := h.q.IsComputerConnected(ctx, row.ComputerID)
	if err != nil {
		internalError(w, "computer connected", err)
		return
	}
	if !connected {
		writeError(w, http.StatusConflict, "O computador onde o app rodou ("+row.ComputerName+") não está conectado. Ligue-o e tente de novo.")
		return
	}
	job, err := h.enqueue(ctx, row.ComputerID, &row.ID, agentproto.KindSendFile, p)
	if err != nil {
		internalError(w, "enqueue send", err)
		return
	}
	ch := h.uploads.expect(job.ID)
	defer h.uploads.cancel(job.ID)
	select {
	case up := <-ch:
		if up.missing {
			writeError(w, http.StatusNotFound, "Esse arquivo não está mais na pasta.")
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
		_, err := io.Copy(w, up.body)
		up.done <- err
	case <-time.After(h.waitFile):
		writeError(w, http.StatusGatewayTimeout, "O computador não respondeu. Confira se ele está ligado e tente de novo.")
	case <-ctx.Done():
	}
}

func (h *Handler) agentUpload(w http.ResponseWriter, r *http.Request) {
	c, ok := h.currentComputer(w, r)
	if !ok {
		return
	}
	j, ok := h.jobForComputer(w, r, c)
	if !ok {
		return
	}
	if j.Kind != agentproto.KindSendFile {
		writeError(w, http.StatusBadRequest, "Esse pedido não é de envio de arquivo.")
		return
	}
	done, ok := h.uploads.deliver(j.ID, r.Body, 5*time.Second)
	if !ok {
		writeError(w, http.StatusGone, "Ninguém está esperando esse arquivo.")
		return
	}
	var copyErr error
	select {
	case copyErr = <-done:
	case <-r.Context().Done():
		copyErr = r.Context().Err()
	}
	status := "done"
	if copyErr != nil {
		status = "failed"
	}
	if err := h.finishJob(r.Context(), j.ID, status, agentproto.Result{Event: agentproto.EventDone}); err != nil {
		internalError(w, "finish send", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Add imports `io`, `mime`, `net/url`, `path/filepath`. Register:

```go
	mux.HandleFunc("GET /api/runs/{id}/files/{fileId}", h.downloadFile)
	mux.HandleFunc("GET /api/runs/{id}/groups/{index}/zip", h.downloadGroup)
	mux.HandleFunc("POST /api/agent/jobs/{id}/upload", h.agentUpload)
```

- [ ] **Step 5: Run to verify they pass**

Run: `... go test ./internal/handler/` — Expected: `ok`. Also run with `-race` once: `... go test -race ./internal/handler/ -run Download`.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler
git commit -m "Hub: stream run files from the agent on download"
```

---

### Task 9: Agent download endpoint and build script

**Files:**
- Create: `scripts/build-agent.sh`
- Modify: `backend/internal/handler/agents.go`, `backend/internal/handler/handler.go`, `backend/internal/handler/agents_test.go`, `.gitignore`, `.env`

**Interfaces:**
- Produces: `GET /api/agent/download` (logged in) → the exe as `AppHubAgente.exe`; 404 `"O Agente ainda não foi preparado neste Hub."` when missing.

- [ ] **Step 1: Failing test** (append to `agents_test.go`)

```go
func TestAgentDownload(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	if code := c.do("GET", "/api/agent/download", nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing binary %d", code)
	}
	bin := filepath.Join(t.TempDir(), "AppHubAgente.exe")
	_ = os.WriteFile(bin, []byte("MZ"), 0o644)
	h.cfg.AgentBinaryPath = bin
	resp, err := c.c.Get(c.srv.URL + "/api/agent/download")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Disposition"), "AppHubAgente.exe") {
		t.Fatalf("%d %q", resp.StatusCode, resp.Header.Get("Content-Disposition"))
	}
	if code := c.fresh().do("GET", "/api/agent/download", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous %d", code)
	}
}
```

Add imports `os`, `path/filepath`.

- [ ] **Step 2: Run to verify it fails** — `... go test ./internal/handler/ -run AgentDownload` → FAIL (404 route missing gives 404 first, then fails at 200 check).

- [ ] **Step 3: Implement**

```go
func (h *Handler) agentDownload(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	if h.cfg.AgentBinaryPath == "" {
		writeError(w, http.StatusNotFound, "O Agente ainda não foi preparado neste Hub.")
		return
	}
	if _, err := os.Stat(h.cfg.AgentBinaryPath); err != nil {
		writeError(w, http.StatusNotFound, "O Agente ainda não foi preparado neste Hub.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.microsoft.portable-executable")
	w.Header().Set("Content-Disposition", `attachment; filename="AppHubAgente.exe"`)
	http.ServeFile(w, r, h.cfg.AgentBinaryPath)
}
```

Register `mux.HandleFunc("GET /api/agent/download", h.agentDownload)`.

`scripts/build-agent.sh`:

```bash
#!/usr/bin/env bash
# Compiles the Agente do Hub for Windows into dist/agent/AppHubAgente.exe.
# HUB_URL is the address the agent uses to reach the Hub's Go server.
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
HUB_URL="${HUB_URL:-http://localhost:8080}"
mkdir -p "$ROOT/dist/agent"
cd "$ROOT/backend"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 mise x -- go build -trimpath \
  -ldflags "-H=windowsgui -X main.hubURL=$HUB_URL" \
  -o "$ROOT/dist/agent/AppHubAgente.exe" ./cmd/agent
echo "$ROOT/dist/agent/AppHubAgente.exe"
```

`chmod +x scripts/build-agent.sh`. Add `dist/` to `.gitignore`. Add to `.env` (quoted, absolute): `AGENT_BINARY_PATH="<repo root>/dist/agent/AppHubAgente.exe"`.

- [ ] **Step 4: Run to verify it passes** — `... go test ./internal/handler/` → `ok`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler scripts/build-agent.sh .gitignore
git commit -m "Hub: serve the agent installer; build script"
```

---

### Task 10: Agent core — runner, send, client, loop

**Files:**
- Create: `backend/internal/agent/runner.go`, `runner_test.go`, `send.go`, `send_test.go`, `client.go`, `agent.go`, `agent_test.go`

**Interfaces:**
- Consumes: `agentproto`, `TakeSnapshot`, `Diff`.
- Produces (used by Task 11):
  - `type Launcher interface { Launch(ctx context.Context, exe, dir string) (int, error) }`; `ExecLauncher{}` implements it with `os/exec`.
  - `type Runner struct { Launcher Launcher; StateDir string }`; `(*Runner).Run(ctx, p agentproto.RunPayload, report func(agentproto.Result)) agentproto.Result`
  - `PrepareSend(stateDir string, p agentproto.SendFilePayload) (Outgoing, error)`; `ErrNotAllowed`, `ErrMissing`; `(Outgoing).WriteTo(w io.Writer) error`
  - `type Client struct { BaseURL, Token string; HTTP *http.Client }` with `PairStart`, `PairPoll`, `Hello`, `Next`, `Result`, `Upload`; `ErrUnauthorized`, `ErrGone`
  - `type Agent struct { Client *Client; Runner *Runner; StateDir, Name string; ChooseFolder func(ctx context.Context, initial string) (string, bool, error) }`; `(*Agent).Loop(ctx) error` (returns `ErrUnauthorized` on revoke)
  - `Pair(ctx, c *Client, name string, open func(string) error) (string, error)`
  - State: `LoadToken(dir) string`, `SaveToken(dir, token string) error` (file `agente.json`)

- [ ] **Step 1: Failing runner tests** (`runner_test.go`)

```go
package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// fakeLauncher simulates the app: records where it ran and writes files.
type fakeLauncher struct {
	exe, dir string
	write    map[string]string
	block    chan struct{}
}

func (f *fakeLauncher) Launch(ctx context.Context, exe, dir string) (int, error) {
	f.exe, f.dir = exe, dir
	if f.block != nil {
		<-f.block
	}
	for rel, c := range f.write {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(c), 0o644)
	}
	return 0, nil
}

func appFolder(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), "2026 06 23 - Henry Files Prep")
	write(t, dir, "Ferramenta CVM.exe", "MZ")
	write(t, dir, "PETROBRAS/FRE/velho.pdf", "x")
	return dir
}

func TestRunInAppFolder(t *testing.T) {
	app := appFolder(t)
	l := &fakeLauncher{write: map[string]string{"PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md": "# pack"}}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	var events []string
	res := r.Run(context.Background(), agentproto.RunPayload{RunID: 7, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app},
		func(x agentproto.Result) { events = append(events, x.Event+":"+x.AppVersion) })
	if res.Event != agentproto.EventFinished || len(res.Files) != 1 || res.Files[0].Path != "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md" {
		t.Fatalf("result %+v", res)
	}
	if l.exe != filepath.Join(app, "Ferramenta CVM.exe") || l.dir != app {
		t.Fatalf("launched %q in %q", l.exe, l.dir)
	}
	if len(events) != 1 || !strings.HasPrefix(events[0], "started:") || events[0] == "started:" {
		t.Fatalf("events %v", events)
	}
}

func TestRunInOtherFolderCopiesAndRemovesExe(t *testing.T) {
	app := appFolder(t)
	out := filepath.Join(t.TempDir(), "Saídas Relatórios")
	_ = os.MkdirAll(out, 0o755)
	l := &fakeLauncher{write: map[string]string{"X/FRE/novo.pdf": "p"}}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	res := r.Run(context.Background(), agentproto.RunPayload{RunID: 8, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: out}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFinished || len(res.Files) != 1 || res.Files[0].Path != "X/FRE/novo.pdf" {
		t.Fatalf("result %+v", res)
	}
	if l.exe != filepath.Join(out, "Ferramenta CVM.exe") {
		t.Fatalf("launched %q", l.exe)
	}
	if _, err := os.Stat(filepath.Join(out, "Ferramenta CVM.exe")); !os.IsNotExist(err) {
		t.Fatal("temporary copy must be removed")
	}
}

func TestRunRefusesExistingExeInOtherFolder(t *testing.T) {
	app := appFolder(t)
	out := t.TempDir()
	write(t, out, "Ferramenta CVM.exe", "da pessoa")
	r := &Runner{Launcher: &fakeLauncher{}, StateDir: t.TempDir()}
	res := r.Run(context.Background(), agentproto.RunPayload{RunID: 9, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: out}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Já existe") {
		t.Fatalf("result %+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(out, "Ferramenta CVM.exe"))
	if string(b) != "da pessoa" {
		t.Fatal("person's file was touched")
	}
}

func TestRunMissingFolderOrExe(t *testing.T) {
	app := appFolder(t)
	r := &Runner{Launcher: &fakeLauncher{}, StateDir: t.TempDir()}
	res := r.Run(context.Background(), agentproto.RunPayload{AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: filepath.Join(app, "nao-existe")}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Não consegui usar a pasta") {
		t.Fatalf("missing folder %+v", res)
	}
	res = r.Run(context.Background(), agentproto.RunPayload{AppFolder: app, Exe: "Outro.exe", OutputFolder: app}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Outro.exe") {
		t.Fatalf("missing exe %+v", res)
	}
}

func TestRunBusy(t *testing.T) {
	app := appFolder(t)
	l := &fakeLauncher{block: make(chan struct{})}
	r := &Runner{Launcher: l, StateDir: t.TempDir()}
	p := agentproto.RunPayload{RunID: 1, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app}
	started := make(chan struct{})
	go r.Run(context.Background(), p, func(agentproto.Result) { close(started) })
	<-started
	res := r.Run(context.Background(), p, func(agentproto.Result) {})
	close(l.block)
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "Já tem um app aberto") {
		t.Fatalf("busy %+v", res)
	}
}
```

- [ ] **Step 2: Failing send tests** (`send_test.go`)

```go
package agent

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

func recordRun(t *testing.T, state, folder string, paths ...string) {
	t.Helper()
	files := []agentproto.FileEntry{}
	for _, p := range paths {
		files = append(files, agentproto.FileEntry{Path: p, Change: "new"})
	}
	if err := saveRecord(state, 5, folder, files); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareSendSingle(t *testing.T) {
	state, folder := t.TempDir(), t.TempDir()
	write(t, folder, "P/MD_Output/pack.md", "# pack")
	recordRun(t, state, folder, "P/MD_Output/pack.md")
	o, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: []string{"P/MD_Output/pack.md"}})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := o.WriteTo(&buf); err != nil || buf.String() != "# pack" {
		t.Fatalf("%v %q", err, buf.String())
	}
}

func TestPrepareSendZipSkipsMissing(t *testing.T) {
	state, folder := t.TempDir(), t.TempDir()
	write(t, folder, "a.pdf", "A")
	recordRun(t, state, folder, "a.pdf", "b.pdf")
	o, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: []string{"a.pdf", "b.pdf"}, Zip: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := o.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil || len(zr.File) != 1 || zr.File[0].Name != "a.pdf" {
		t.Fatalf("zip %v %+v", err, zr)
	}
}

func TestPrepareSendRefuses(t *testing.T) {
	state, folder := t.TempDir(), t.TempDir()
	write(t, folder, "a.pdf", "A")
	write(t, folder, "segredo.txt", "S")
	recordRun(t, state, folder, "a.pdf", "../fora.txt")
	for _, p := range [][]string{{"segredo.txt"}, {"../fora.txt"}} {
		if _, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: p}); !errors.Is(err, ErrNotAllowed) {
			t.Fatalf("%v: %v", p, err)
		}
	}
	if _, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 99, Paths: []string{"a.pdf"}}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("unknown run: %v", err)
	}
	_ = os.Remove(filepath.Join(folder, "a.pdf"))
	if _, err := PrepareSend(state, agentproto.SendFilePayload{RunID: 5, Paths: []string{"a.pdf"}}); !errors.Is(err, ErrMissing) {
		t.Fatalf("missing: %v", err)
	}
}
```

- [ ] **Step 3: Run to verify they fail** — `cd backend && mise x -- go test ./internal/agent/` → compile FAIL.

- [ ] **Step 4: Implement `runner.go`**

```go
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
}

func failed(msg string) agentproto.Result {
	return agentproto.Result{Event: agentproto.EventFailed, Error: msg}
}

// Run opens the app, waits for it to close and lists the new or changed files
// in the output folder. report receives the "started" event.
func (r *Runner) Run(ctx context.Context, p agentproto.RunPayload, report func(agentproto.Result)) agentproto.Result {
	if !r.mu.TryLock() {
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
			_ = os.Remove(target)
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
		return err
	}
	return out.Close()
}

func humanSize(n int64) string {
	mb := float64(n) / (1 << 20)
	if mb >= 1 {
		return strings.Replace(fmt.Sprintf("%.1f MB", mb), ".", ",", 1)
	}
	return fmt.Sprintf("%d KB", n>>10)
}
```

- [ ] **Step 5: Implement `send.go`**

```go
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

func (o Outgoing) WriteTo(w io.Writer) error {
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
```

- [ ] **Step 6: Run runner and send tests** — `go test ./internal/agent/` → `ok`.

- [ ] **Step 7: Failing agent-loop test** (`agent_test.go`) — a fake Hub with `httptest`

```go
package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// fakeHub serves queued jobs and records results and uploads.
type fakeHub struct {
	mu      sync.Mutex
	jobs    []agentproto.Job
	results map[int64][]agentproto.Result
	uploads map[int64]string
	revoked bool
}

func (f *fakeHub) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/agent/next", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.revoked {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if len(f.jobs) == 0 {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		j := f.jobs[0]
		f.jobs = f.jobs[1:]
		_ = json.NewEncoder(w).Encode(j)
	})
	mux.HandleFunc("POST /api/agent/jobs/{id}/result", func(w http.ResponseWriter, r *http.Request) {
		var res agentproto.Result
		_ = json.NewDecoder(r.Body).Decode(&res)
		var id int64
		_, _ = fmtSscan(r.PathValue("id"), &id)
		f.mu.Lock()
		f.results[id] = append(f.results[id], res)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/agent/jobs/{id}/upload", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var id int64
		_, _ = fmtSscan(r.PathValue("id"), &id)
		f.mu.Lock()
		f.uploads[id] = string(b)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func payload(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestAgentLoop(t *testing.T) {
	app := appFolder(t)
	state := t.TempDir()
	hub := &fakeHub{results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	hub.jobs = []agentproto.Job{
		{ID: 1, Kind: agentproto.KindChooseFolder, Payload: payload(agentproto.ChooseFolderPayload{Initial: app})},
		{ID: 2, Kind: agentproto.KindRun, Payload: payload(agentproto.RunPayload{RunID: 3, AppFolder: app, Exe: "Ferramenta CVM.exe", OutputFolder: app})},
	}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()

	a := &Agent{
		Client: &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()},
		Runner: &Runner{Launcher: &fakeLauncher{write: map[string]string{"novo.md": "# oi"}}, StateDir: state},
		StateDir: state,
		ChooseFolder: func(ctx context.Context, initial string) (string, bool, error) {
			return `C:\Relatórios`, true, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go a.Loop(ctx)

	waitFor(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return len(hub.results[2]) == 2
	})
	hub.mu.Lock()
	if r := hub.results[1]; len(r) != 1 || r[0].Folder != `C:\Relatórios` {
		t.Fatalf("folder result %+v", r)
	}
	if r := hub.results[2]; r[0].Event != agentproto.EventStarted || r[1].Event != agentproto.EventFinished || len(r[1].Files) != 1 {
		t.Fatalf("run results %+v", r)
	}
	hub.jobs = append(hub.jobs, agentproto.Job{ID: 4, Kind: agentproto.KindSendFile, Payload: payload(agentproto.SendFilePayload{RunID: 3, Paths: []string{"novo.md"}})},
		agentproto.Job{ID: 5, Kind: agentproto.KindSendFile, Payload: payload(agentproto.SendFilePayload{RunID: 3, Paths: []string{"Ferramenta CVM.exe"}})})
	hub.mu.Unlock()

	waitFor(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return hub.uploads[4] != "" && len(hub.results[5]) == 1
	})
	hub.mu.Lock()
	if hub.uploads[4] != "# oi" || hub.results[5][0].Event != agentproto.EventFailed {
		t.Fatalf("uploads %v results5 %+v", hub.uploads, hub.results[5])
	}
	hub.revoked = true
	hub.mu.Unlock()
	cancel()
}

func TestAgentLoopStopsWhenRevoked(t *testing.T) {
	hub := &fakeHub{revoked: true, results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()
	a := &Agent{Client: &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}, Runner: &Runner{}, StateDir: t.TempDir()}
	if err := a.Loop(context.Background()); err != ErrUnauthorized {
		t.Fatalf("got %v", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
```

and in the same file:

```go
func fmtSscan(s string, v *int64) (int, error) { return fmt.Sscan(s, v) }
```

(add `"fmt"` import).

- [ ] **Step 8: Implement `client.go`**

```go
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

var (
	ErrUnauthorized = errors.New("agent token rejected")
	ErrGone         = errors.New("pairing expired or used")
)

// Client talks to the Hub. Every call is started by the agent.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return resp.StatusCode, ErrUnauthorized
	case resp.StatusCode == http.StatusGone:
		return resp.StatusCode, ErrGone
	case resp.StatusCode >= 300:
		return resp.StatusCode, fmt.Errorf("hub %s %s: %d", method, path, resp.StatusCode)
	}
	if out != nil && resp.StatusCode == http.StatusOK {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

func (c *Client) postJSON(ctx context.Context, path string, in, out any) (int, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return 0, err
	}
	return c.do(ctx, "POST", path, bytes.NewReader(b), "application/json", out)
}

func (c *Client) PairStart(ctx context.Context, name string) (agentproto.PairStartResponse, error) {
	var out agentproto.PairStartResponse
	_, err := c.postJSON(ctx, "/api/agent/pair/start", agentproto.PairStartRequest{ComputerName: name}, &out)
	return out, err
}

func (c *Client) PairPoll(ctx context.Context, secret string) (agentproto.PairPollResponse, error) {
	var out agentproto.PairPollResponse
	_, err := c.postJSON(ctx, "/api/agent/pair/poll", agentproto.PairPollRequest{Secret: secret}, &out)
	return out, err
}

func (c *Client) Hello(ctx context.Context, name string) error {
	_, err := c.postJSON(ctx, "/api/agent/hello", agentproto.Hello{Name: name, Version: agentproto.Version}, nil)
	return err
}

// Next waits for the next job; nil means none arrived in the Hub's wait.
func (c *Client) Next(ctx context.Context) (*agentproto.Job, error) {
	var j agentproto.Job
	code, err := c.do(ctx, "GET", "/api/agent/next", nil, "", &j)
	if err != nil || code == http.StatusNoContent {
		return nil, err
	}
	return &j, nil
}

func (c *Client) Result(ctx context.Context, jobID int64, r agentproto.Result) error {
	_, err := c.postJSON(ctx, "/api/agent/jobs/"+strconv.FormatInt(jobID, 10)+"/result", r, nil)
	return err
}

func (c *Client) Upload(ctx context.Context, jobID int64, body io.Reader) error {
	_, err := c.do(ctx, "POST", "/api/agent/jobs/"+strconv.FormatInt(jobID, 10)+"/upload", body, "application/octet-stream", nil)
	return err
}
```

- [ ] **Step 9: Implement `agent.go`**

```go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// Agent receives jobs from the Hub and carries them out.
type Agent struct {
	Client       *Client
	Runner       *Runner
	StateDir     string
	Name         string
	ChooseFolder func(ctx context.Context, initial string) (folder string, ok bool, err error)
}

// Loop asks the Hub for jobs until ctx ends or the token is revoked.
func (a *Agent) Loop(ctx context.Context) error {
	backoff := time.Second
	for ctx.Err() == nil {
		job, err := a.Client.Next(ctx)
		if errors.Is(err, ErrUnauthorized) {
			return err
		}
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			slog.Warn("hub unreachable", "err", err, "retry_in", backoff)
			time.Sleep(backoff)
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		if job != nil {
			go a.handle(ctx, *job)
		}
	}
	return ctx.Err()
}

func (a *Agent) report(ctx context.Context, id int64, r agentproto.Result) {
	if err := a.Client.Result(ctx, id, r); err != nil {
		slog.Error("report result", "job", id, "err", err)
	}
}

func (a *Agent) handle(ctx context.Context, job agentproto.Job) {
	switch job.Kind {
	case agentproto.KindRun:
		var p agentproto.RunPayload
		if err := json.Unmarshal(job.Payload, &p); err != nil {
			a.report(ctx, job.ID, failed("Pedido inválido."))
			return
		}
		res := a.Runner.Run(ctx, p, func(r agentproto.Result) { a.report(ctx, job.ID, r) })
		a.report(ctx, job.ID, res)
	case agentproto.KindChooseFolder:
		var p agentproto.ChooseFolderPayload
		_ = json.Unmarshal(job.Payload, &p)
		if a.ChooseFolder == nil {
			a.report(ctx, job.ID, failed("Este computador não sabe abrir a janela de pastas."))
			return
		}
		folder, ok, err := a.ChooseFolder(ctx, p.Initial)
		if err != nil {
			slog.Error("choose folder", "err", err)
			a.report(ctx, job.ID, failed("Não consegui abrir a janela de pastas."))
			return
		}
		a.report(ctx, job.ID, agentproto.Result{Event: agentproto.EventDone, Folder: folder, Canceled: !ok})
	case agentproto.KindSendFile:
		var p agentproto.SendFilePayload
		_ = json.Unmarshal(job.Payload, &p)
		out, err := PrepareSend(a.StateDir, p)
		if errors.Is(err, ErrMissing) {
			a.report(ctx, job.ID, agentproto.Result{Event: agentproto.EventMissing})
			return
		}
		if err != nil {
			slog.Warn("send refused", "run", p.RunID, "err", err)
			a.report(ctx, job.ID, failed("Arquivo não permitido."))
			return
		}
		pr, pw := io.Pipe()
		go func() { pw.CloseWithError(out.WriteTo(pw)) }()
		if err := a.Client.Upload(ctx, job.ID, pr); err != nil {
			slog.Error("upload", "job", job.ID, "err", err)
		}
		_ = pr.Close()
	default:
		a.report(ctx, job.ID, failed("Pedido desconhecido."))
	}
}

// Pair asks the Hub for a connection, opens the approval page and waits.
func Pair(ctx context.Context, c *Client, name string, open func(string) error) (string, error) {
	var start agentproto.PairStartResponse
	for backoff := time.Second; ; backoff = min(backoff*2, 30*time.Second) {
		var err error
		if start, err = c.PairStart(ctx, name); err == nil {
			break
		}
		slog.Warn("pair start", "err", err)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff):
		}
	}
	if err := open(start.ApproveURL); err != nil {
		slog.Error("open browser", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
		res, err := c.PairPoll(ctx, start.Secret)
		if errors.Is(err, ErrGone) {
			return "", err
		}
		if err == nil && res.Status == "approved" {
			return res.Token, nil
		}
	}
}

type state struct {
	Token string `json:"token"`
}

func LoadToken(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "agente.json"))
	if err != nil {
		return ""
	}
	var s state
	_ = json.Unmarshal(b, &s)
	return s.Token
}

func SaveToken(dir, token string) error {
	b, _ := json.Marshal(state{Token: token})
	return os.WriteFile(filepath.Join(dir, "agente.json"), b, 0o600)
}
```

- [ ] **Step 10: Run all agent tests** — `cd backend && mise x -- go test -race ./internal/agent/` → `ok`.

- [ ] **Step 11: Commit**

```bash
git add backend/internal/agent
git commit -m "Agent core: run apps, list results, send files, talk to the Hub"
```

---

### Task 11: Agent entrypoint and Windows glue

**Files:**
- Create: `backend/cmd/agent/main.go`, `backend/cmd/agent/platform_windows.go`, `backend/cmd/agent/platform_other.go`
- Modify: `backend/go.mod`, `backend/go.sum`

**Interfaces:**
- Consumes: everything in `internal/agent`.
- Produces: platform functions `stateDir() string`, `install(dir string) (handedOff bool, err error)`, `chooseFolder(ctx, initial string) (string, bool, error)`, `openBrowser(url string) error`.

- [ ] **Step 1: `main.go`**

```go
// Command agent is the Agente do Hub: it runs invisibly on a person's Windows
// computer and opens apps when the Hub asks.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agent"
)

// hubURL is set at build time (scripts/build-agent.sh); APPHUB_URL overrides it.
var hubURL = "http://localhost:8080"

const lockAddr = "127.0.0.1:47631"

func main() {
	if v := os.Getenv("APPHUB_URL"); v != "" {
		hubURL = v
	}
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		os.Exit(1)
	}
	if f, err := os.OpenFile(dir+string(os.PathSeparator)+"agente.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(f, nil)))
	}

	handedOff, err := install(dir)
	if err != nil {
		slog.Error("install", "err", err)
		os.Exit(1)
	}
	if handedOff {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ln, err := net.Listen("tcp", lockAddr)
	if err != nil {
		slog.Info("another agent is already running")
		return
	}
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/quit" {
			w.WriteHeader(http.StatusNoContent)
			go func() { time.Sleep(100 * time.Millisecond); os.Exit(0) }()
			return
		}
		http.NotFound(w, r)
	}))

	name, _ := os.Hostname()
	client := &agent.Client{BaseURL: hubURL, Token: agent.LoadToken(dir), HTTP: &http.Client{Timeout: 0}}
	if client.Token == "" {
		token, err := agent.Pair(ctx, client, name, openBrowser)
		if err != nil {
			slog.Error("pairing", "err", err)
			return
		}
		if err := agent.SaveToken(dir, token); err != nil {
			slog.Error("save token", "err", err)
			return
		}
		client.Token = token
	}
	for {
		err := client.Hello(ctx, name)
		if err == nil {
			break
		}
		if errors.Is(err, agent.ErrUnauthorized) {
			_ = agent.SaveToken(dir, "")
			return
		}
		slog.Warn("hello", "err", err)
		time.Sleep(5 * time.Second)
	}
	a := &agent.Agent{
		Client: client, StateDir: dir, Name: name, ChooseFolder: chooseFolder,
		Runner: &agent.Runner{Launcher: agent.ExecLauncher{}, StateDir: dir},
	}
	if err := a.Loop(ctx); errors.Is(err, agent.ErrUnauthorized) {
		slog.Info("computer disconnected in the Hub; forgetting token")
		_ = agent.SaveToken(dir, "")
	}
}
```

- [ ] **Step 2: `platform_other.go`** (dev use on Linux/macOS)

```go
//go:build !windows

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".apphub-agente")
}

func install(string) (bool, error) { return false, nil }

func chooseFolder(context.Context, string) (string, bool, error) {
	return "", false, errors.New("folder dialog only on Windows")
}

func openBrowser(url string) error {
	slog.Info("open this page to connect", "url", url)
	return nil
}
```

- [ ] **Step 3: `platform_windows.go`**

```go
//go:build windows

package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

const createNoWindow = 0x08000000

func stateDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "AppHub")
}

// install copies this program to %LOCALAPPDATA%\AppHub, registers it to start
// with Windows and starts the installed copy. It returns true when the
// installed copy took over.
func install(dir string) (bool, error) {
	self, err := os.Executable()
	if err != nil {
		return false, err
	}
	target := filepath.Join(dir, "AppHubAgente.exe")
	if strings.EqualFold(filepath.Clean(self), filepath.Clean(target)) {
		return false, nil
	}
	// An older agent may be running from the target; ask it to quit.
	if resp, err := http.Post("http://"+lockAddr+"/quit", "", nil); err == nil {
		resp.Body.Close()
		time.Sleep(2 * time.Second)
	}
	var copyErr error
	for range 5 {
		if copyErr = copySelf(self, target); copyErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if copyErr != nil {
		return false, copyErr
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	if err := k.SetStringValue("AppHubAgente", `"`+target+`"`); err != nil {
		return false, err
	}
	return true, exec.Command(target).Start()
}

func copySelf(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// folderScript shows the Windows folder picker on top of other windows and
// prints the chosen path as UTF-8 (paths like "8. Programação").
const folderScript = `
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form -Property @{TopMost = $true}
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = 'Onde salvar os resultados'
$d.ShowNewFolderButton = $true
if ($env:APPHUB_INITIAL -and (Test-Path -LiteralPath $env:APPHUB_INITIAL)) { $d.SelectedPath = $env:APPHUB_INITIAL }
if ($d.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($d.SelectedPath) }
`

func chooseFolder(ctx context.Context, initial string) (string, bool, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", folderScript)
	cmd.Env = append(os.Environ(), "APPHUB_INITIAL="+initial)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return "", false, err
	}
	folder := strings.TrimSpace(strings.TrimPrefix(string(out), "\ufeff"))
	return folder, folder != "", nil
}

func openBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
```

- [ ] **Step 4: Dependencies and builds**

```bash
cd backend && mise x -- go get golang.org/x/sys@latest && mise x -- go mod tidy
mise x -- go vet ./...
GOOS=windows GOARCH=amd64 mise x -- go vet ./cmd/agent/ ./internal/agent/
cd .. && scripts/build-agent.sh
```

Expected: vet clean on both platforms; the script prints the path of `dist/agent/AppHubAgente.exe`.

- [ ] **Step 5: Smoke-test the Linux build against the dev Hub**

With the Hub running: `cd backend && APPHUB_URL=http://localhost:8080 timeout 5 mise x -- go run ./cmd/agent; tail -3 ~/.apphub-agente/agente.log` — Expected: a log line `open this page to connect url=http://localhost:5173/conectar?codigo=...`. Then `rm -rf ~/.apphub-agente`.

- [ ] **Step 6: Commit**

```bash
git add backend/cmd/agent backend/go.mod backend/go.sum
git commit -m "Agent entrypoint: install, autostart, folder dialog, browser"
```

---

### Task 12: Frontend — API, Meu computador, Conectar

**Files:**
- Modify: `frontend/src/lib/api.ts`, `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`, `frontend/src/pages/LoginPage.tsx`, `frontend/src/test/fixtures.ts`
- Create: `frontend/src/lib/runs.ts`, `frontend/src/lib/runs.test.ts`, `frontend/src/pages/ComputerPage.tsx`, `ComputerPage.test.tsx`, `frontend/src/pages/ConnectPage.tsx`, `ConnectPage.test.tsx`

Load the **frontend-design** skill before writing the pages; keep the fichário look (paper card, mono eyebrow labels, `stamp` accent).

**Interfaces:**
- Produces (used by Task 13):

```ts
export type Computer = { id: number; name: string; agent_version: string; last_seen_at: string | null; connected: boolean; outdated: boolean }
export type ComputersInfo = { computers: Computer[]; expected_version: string; agent_available: boolean }
export type Pairing = { code: string; computer_name: string; expired: boolean; approved: boolean }
export type RunStatus = "waiting" | "running" | "done" | "no_files" | "error" | "interrupted"
export type RunFile = { id: number; path: string; size: number; modified: string; change: "new" | "changed" }
export type RunGroup = { index: number; label: string; files: RunFile[] }
export type Run = {
  id: number; app_id: number; app_name: string; person_name: string; computer_name: string; output_folder: string
  status: RunStatus; exit_code: number | null; error_message: string; app_version: string
  created_at: string; started_at: string | null; finished_at: string | null; groups: RunGroup[]
}
export type RunSummary = { id: number; status: RunStatus; person_name: string; created_at: string; started_at: string | null; finished_at: string | null; file_count: number }
// AppDetail gains: can_run: boolean
api.computers(): Promise<ComputersInfo>
api.disconnect(id: number): Promise<void>
api.pairing(code: string): Promise<Pairing>
api.approvePairing(code: string): Promise<void>
api.chooseFolder(initial: string): Promise<{ folder: string; canceled: boolean }>
api.startRun(appId: number, output_folder: string): Promise<Run>
api.appRuns(appId: number | string): Promise<RunSummary[]>
api.run(id: number | string): Promise<Run>
export const AGENT_DOWNLOAD_URL = "/api/agent/download"
export const fileUrl = (runId: number, fileId: number) => `/api/runs/${runId}/files/${fileId}`
export const groupZipUrl = (runId: number, index: number) => `/api/runs/${runId}/groups/${index}/zip`
// lib/runs.ts
export const STATUS: Record<RunStatus, { label: string; tone: "live" | "ok" | "muted" | "bad" }>
export function isLive(s: RunStatus): boolean
export function formatDuration(startISO: string | null, endISO: string | null, nowMs?: number): string // "12 min", "45 s", "1 h 05 min", "—"
export function formatSize(bytes: number): string // "820 B", "12 KB", "75,6 MB"
export function formatWhen(iso: string): string // "30/09 às 14:10"
```

- [ ] **Step 1: Failing test for `runs.ts`**

```ts
import { expect, test } from "vitest"
import { formatDuration, formatSize, formatWhen, isLive } from "./runs"

test("durations", () => {
  const t0 = "2026-09-30T14:10:00Z"
  expect(formatDuration(t0, "2026-09-30T14:10:45Z")).toBe("45 s")
  expect(formatDuration(t0, "2026-09-30T14:22:10Z")).toBe("12 min")
  expect(formatDuration(t0, "2026-09-30T15:15:00Z")).toBe("1 h 05 min")
  expect(formatDuration(null, null)).toBe("—")
  expect(formatDuration(t0, null, Date.parse("2026-09-30T14:11:00Z"))).toBe("1 min")
})

test("sizes", () => {
  expect(formatSize(820)).toBe("820 B")
  expect(formatSize(12 * 1024)).toBe("12 KB")
  expect(formatSize(79291437)).toBe("75,6 MB")
})

test("when and live", () => {
  expect(formatWhen("2026-09-30T14:10:00-03:00")).toMatch(/30\/09 às \d\d:10/)
  expect(isLive("running")).toBe(true)
  expect(isLive("done")).toBe(false)
})
```

- [ ] **Step 2: Run** — `cd frontend && mise x -- npx vitest run src/lib/runs.test.ts` → FAIL (module missing).

- [ ] **Step 3: Implement `runs.ts`**

```ts
import type { RunStatus } from "./api"

export const STATUS: Record<RunStatus, { label: string; tone: "live" | "ok" | "muted" | "bad" }> = {
  waiting: { label: "Aguardando o computador", tone: "live" },
  running: { label: "Rodando", tone: "live" },
  done: { label: "Concluída", tone: "ok" },
  no_files: { label: "Sem arquivos novos", tone: "muted" },
  error: { label: "Erro", tone: "bad" },
  interrupted: { label: "Interrompida", tone: "bad" },
}

export const isLive = (s: RunStatus) => s === "waiting" || s === "running"

export function formatDuration(startISO: string | null, endISO: string | null, nowMs = Date.now()): string {
  if (!startISO) return "—"
  const secs = Math.max(0, Math.round(((endISO ? Date.parse(endISO) : nowMs) - Date.parse(startISO)) / 1000))
  if (secs < 60) return `${secs} s`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins} min`
  return `${Math.floor(mins / 60)} h ${String(mins % 60).padStart(2, "0")} min`
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1).replace(".", ",")} MB`
}

export function formatWhen(iso: string): string {
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, "0")
  return `${p(d.getDate())}/${p(d.getMonth() + 1)} às ${p(d.getHours())}:${p(d.getMinutes())}`
}
```

- [ ] **Step 4: Add API types and calls** to `api.ts` (types from the Interfaces block; add `can_run: boolean` to `AppDetail`), and in the `api` object:

```ts
  computers: () => request<ComputersInfo>("GET", "/api/computers/me"),
  disconnect: (id: number) => request<void>("DELETE", `/api/computers/${id}`),
  pairing: (code: string) => request<Pairing>("GET", `/api/pairings/${encodeURIComponent(code)}`),
  approvePairing: (code: string) => request<void>("POST", `/api/pairings/${encodeURIComponent(code)}/approve`),
  chooseFolder: (initial: string) => request<{ folder: string; canceled: boolean }>("POST", "/api/choose-folder", { initial }),
  startRun: (appId: number, output_folder: string) => request<Run>("POST", `/api/apps/${appId}/runs`, { output_folder }),
  appRuns: (appId: number | string) => request<RunSummary[]>("GET", `/api/apps/${appId}/runs`),
  run: (id: number | string) => request<Run>("GET", `/api/runs/${id}`),
```

plus the three URL helpers. In `fixtures.ts`, add:

```ts
export function computer(over: Partial<Computer> = {}): Computer {
  return { id: 1, name: "NOTE-HENRY", agent_version: "0.1.0", last_seen_at: "2026-09-30T14:00:00Z", connected: true, outdated: false, ...over }
}
```

- [ ] **Step 5: Failing page tests**

`ComputerPage.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { computer } from "@/test/fixtures"
import { ComputerPage } from "./ComputerPage"

afterEach(() => vi.restoreAllMocks())
const renderPage = () => render(<MemoryRouter><ComputerPage /></MemoryRouter>)

test("no computer: shows how to install", async () => {
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "0.1.0", agent_available: true })
  renderPage()
  expect(await screen.findByText("Nenhum computador conectado")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /Instalar o Agente/ })).toHaveAttribute("href", "/api/agent/download")
})

test("connected computer, outdated agent, disconnect", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [computer({ outdated: true, agent_version: "0.0.9" })], expected_version: "0.1.0", agent_available: true })
  const disconnect = vi.spyOn(api, "disconnect").mockResolvedValue(undefined)
  vi.spyOn(window, "confirm").mockReturnValue(true)
  renderPage()
  expect(await screen.findByText("NOTE-HENRY")).toBeInTheDocument()
  expect(screen.getByText("Conectado")).toBeInTheDocument()
  expect(screen.getByText(/Atualize o Agente/)).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Desconectar" }))
  expect(disconnect).toHaveBeenCalledWith(1)
})

test("agent not built on this Hub", async () => {
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "0.1.0", agent_available: false })
  renderPage()
  expect(await screen.findByText(/O Agente ainda não foi preparado neste Hub/)).toBeInTheDocument()
})
```

`ConnectPage.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { ConnectPage } from "./ConnectPage"

afterEach(() => vi.restoreAllMocks())
const renderAt = (url: string) =>
  render(
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path="/conectar" element={<ConnectPage />} />
        <Route path="/meu-computador" element={<p>página do computador</p>} />
      </Routes>
    </MemoryRouter>,
  )

test("approves the computer", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "pairing").mockResolvedValue({ code: "ABCD2345", computer_name: "NOTE-HENRY", expired: false, approved: false })
  const approve = vi.spyOn(api, "approvePairing").mockResolvedValue(undefined)
  renderAt("/conectar?codigo=ABCD2345")
  expect(await screen.findByText("NOTE-HENRY")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Conectar" }))
  expect(approve).toHaveBeenCalledWith("ABCD2345")
  expect(await screen.findByText("página do computador")).toBeInTheDocument()
})

test("expired code", async () => {
  vi.spyOn(api, "pairing").mockResolvedValue({ code: "X", computer_name: "PC", expired: true, approved: false })
  renderAt("/conectar?codigo=X")
  expect(await screen.findByText(/venceu/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Conectar" })).not.toBeInTheDocument()
})

test("unknown code", async () => {
  vi.spyOn(api, "pairing").mockRejectedValue(new ApiError(404, "Não encontrei esse pedido de conexão. Abra o Agente de novo."))
  renderAt("/conectar?codigo=Z")
  expect(await screen.findByRole("alert")).toHaveTextContent("Não encontrei esse pedido")
})
```

- [ ] **Step 6: Run** — `mise x -- npx vitest run src/pages/ComputerPage.test.tsx src/pages/ConnectPage.test.tsx` → FAIL.

- [ ] **Step 7: Implement `ComputerPage.tsx`**

```tsx
import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Download, Laptop, TriangleAlert } from "lucide-react"
import { PageHeader } from "@/components/bits"
import { Button, buttonVariants } from "@/components/ui/button"
import { AGENT_DOWNLOAD_URL, api, type ComputersInfo } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { cn } from "@/lib/utils"

export function ComputerPage() {
  const [info, setInfo] = useState<ComputersInfo | null>(null)
  const [error, setError] = useState("")

  const load = () => api.computers().then(setInfo, (e: Error) => setError(e.message))
  useEffect(() => {
    load()
    const t = setInterval(load, 5000)
    return () => clearInterval(t)
  }, [])

  const disconnect = async (id: number, name: string) => {
    if (!window.confirm(`Desconectar ${name}? O Hub não vai mais abrir apps nele até você instalar o Agente de novo.`)) return
    await api.disconnect(id).then(() => toast.success(`${name} foi desconectado.`), (e: Error) => toast.error(e.message))
    load()
  }

  if (error) return <p role="alert" className="text-destructive">{error}</p>
  if (!info) return <p className="text-muted-foreground">Carregando…</p>

  const install = info.agent_available ? (
    <a href={AGENT_DOWNLOAD_URL} className={cn(buttonVariants(), "gap-2")}>
      <Download className="size-4" /> Instalar o Agente
    </a>
  ) : (
    <p className="text-sm text-warn">O Agente ainda não foi preparado neste Hub. Peça a quem administra o Hub.</p>
  )

  return (
    <>
      <PageHeader eyebrow="Agente do Hub" title="Meu computador">
        O Agente é um programa pequeno que fica no seu Windows, sem ícone. É ele que abre os apps quando você clica em Rodar.
      </PageHeader>

      {info.computers.length === 0 ? (
        <section className="paper rounded-lg border border-border bg-card p-8 shadow-card">
          <h2 className="font-heading text-2xl">Nenhum computador conectado</h2>
          <ol className="mt-4 flex list-decimal flex-col gap-2 pl-5 text-sm">
            <li>Baixe o Agente pelo botão abaixo.</li>
            <li>Abra o arquivo baixado (AppHubAgente.exe). Se o Windows perguntar, confirme que quer abrir.</li>
            <li>O navegador vai abrir uma página do Hub. Clique em <strong>Conectar</strong>.</li>
          </ol>
          <div className="mt-6">{install}</div>
        </section>
      ) : (
        <ul className="flex flex-col gap-4">
          {info.computers.map((c) => (
            <li key={c.id} className="paper flex flex-wrap items-center gap-4 rounded-lg border border-border bg-card p-6 shadow-card">
              <Laptop className="size-8 text-stamp" />
              <div className="min-w-0 flex-1">
                <p className="font-heading text-xl">{c.name}</p>
                <p className="text-sm text-muted-foreground">
                  <span className={c.connected ? "font-medium text-ok" : "font-medium text-warn"}>{c.connected ? "Conectado" : "Desligado ou sem internet"}</span>
                  {c.last_seen_at && ` · visto pela última vez em ${formatWhen(c.last_seen_at)}`}
                  {c.agent_version && ` · Agente ${c.agent_version}`}
                </p>
                {c.outdated && (
                  <p className="mt-2 flex items-center gap-2 text-sm text-warn">
                    <TriangleAlert className="size-4" /> Atualize o Agente: baixe e abra a versão {info.expected_version}.
                  </p>
                )}
              </div>
              <Button variant="outline" onClick={() => disconnect(c.id, c.name)}>Desconectar</Button>
            </li>
          ))}
          <li className="pt-2">{install}</li>
        </ul>
      )}
    </>
  )
}
```

If the `text-ok` / `text-warn` tokens don't exist in `index.css`, check the existing tokens (`text-warn` is used in `AppPage`) and add `--ok` in the theme the same way `--warn` is defined, for light and dark.

- [ ] **Step 8: Implement `ConnectPage.tsx`**

```tsx
import { useEffect, useState } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { Laptop } from "lucide-react"
import { Button } from "@/components/ui/button"
import { api, type Pairing } from "@/lib/api"

export function ConnectPage() {
  const [params] = useSearchParams()
  const code = params.get("codigo") ?? ""
  const navigate = useNavigate()
  const [pairing, setPairing] = useState<Pairing | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api.pairing(code).then(setPairing, (e: Error) => setError(e.message))
  }, [code])

  const approve = async () => {
    setBusy(true)
    try {
      await api.approvePairing(code)
      navigate("/meu-computador")
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível conectar.")
      setBusy(false)
    }
  }

  return (
    <section className="paper mx-auto mt-10 max-w-lg rounded-lg border border-border bg-card p-8 text-center shadow-card">
      <Laptop className="mx-auto size-10 text-stamp" />
      {error ? (
        <p role="alert" className="mt-4 text-destructive">{error}</p>
      ) : !pairing ? (
        <p className="mt-4 text-muted-foreground">Carregando…</p>
      ) : pairing.expired || pairing.approved ? (
        <p className="mt-4">Esse pedido de conexão venceu ou já foi usado. Abra o Agente de novo no seu computador.</p>
      ) : (
        <>
          <h1 className="mt-4 font-heading text-3xl">Conectar este computador à sua conta?</h1>
          <p className="mt-4 font-mono text-lg">{pairing.computer_name}</p>
          <p className="mt-2 text-sm text-muted-foreground">Depois disso, o Hub poderá abrir apps neste computador quando você clicar em Rodar.</p>
          <div className="mt-6 flex justify-center gap-3">
            <Button variant="outline" onClick={() => navigate("/")}>Cancelar</Button>
            <Button onClick={approve} disabled={busy}>Conectar</Button>
          </div>
        </>
      )}
    </section>
  )
}
```

- [ ] **Step 9: Routes, nav, login return**

`App.tsx`: inside `Protected` routes add `<Route path="meu-computador" element={<ComputerPage />} />` and `<Route path="conectar" element={<ConnectPage />} />`; in `Protected`, keep the destination:

```tsx
function Protected() {
  const { loading, person } = useAuth()
  const location = useLocation()
  if (loading) return null
  if (!person) return <Navigate to="/entrar" replace state={{ from: location.pathname + location.search }} />
  return <Layout />
}
```

`LoginPage.tsx`: read `const from = (useLocation().state as { from?: string } | null)?.from ?? "/"` and replace `navigate("/")` with `navigate(from, { replace: true })`.

`Layout.tsx`: add to `NAV` `{ to: "/meu-computador", label: "Meu computador", icon: Laptop, roles: ["admin", "criador", "usuario"] }` (import `Laptop`).

- [ ] **Step 10: Run the whole frontend suite** — `mise x -- npx vitest run && mise x -- npx tsc -b` → all pass.

- [ ] **Step 11: Commit**

```bash
git add frontend/src
git commit -m "Frontend: Meu computador and Conectar pages"
```

---

### Task 13: Frontend — Rodar, execuções e página da execução

**Files:**
- Create: `frontend/src/components/RunDialog.tsx`, `frontend/src/components/RunsList.tsx`, `frontend/src/pages/RunPage.tsx`, `frontend/src/pages/RunPage.test.tsx`, `frontend/src/pages/AppPage.test.tsx`
- Modify: `frontend/src/pages/AppPage.tsx`, `frontend/src/App.tsx`, `frontend/src/test/fixtures.ts`

**Interfaces:**
- Consumes: API and `runs.ts` from Task 12.
- Produces: route `/execucoes/:id`; `RunDialog({ app, open, onOpenChange })`; `RunsList({ appId })`.

- [ ] **Step 1: Fixtures**

```ts
export function run(over: Partial<Run> = {}): Run {
  return {
    id: 7, app_id: 3, app_name: "Eq. Research Files Prep", person_name: "Henry", computer_name: "NOTE-HENRY",
    output_folder: "C:\\apps\\Files Prep", status: "done", exit_code: 0, error_message: "", app_version: "2026-08-13 17:54 · 75,6 MB",
    created_at: "2026-09-30T14:10:00Z", started_at: "2026-09-30T14:10:05Z", finished_at: "2026-09-30T14:22:05Z",
    groups: [
      { index: 0, label: "Documentos da CVM", files: [{ id: 1, path: "PETROBRAS/FRE/FRE_2025.pdf", size: 2048, modified: "2026-09-30T14:20:00Z", change: "new" }] },
      { index: 1, label: "Data Pack", files: [{ id: 2, path: "PETROBRAS/MD_Output/PETROBRAS_DataPack_Consolidado.md", size: 900, modified: "2026-09-30T14:21:00Z", change: "new" }] },
      { index: 2, label: "FRE cortado", files: [] },
      { index: -1, label: "Outros arquivos", files: [{ id: 3, path: "log.txt", size: 10, modified: "2026-09-30T14:21:00Z", change: "changed" }] },
    ],
    ...over,
  }
}
```

- [ ] **Step 2: Failing tests**

`RunPage.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { run } from "@/test/fixtures"
import { RunPage } from "./RunPage"

afterEach(() => vi.restoreAllMocks())
const renderAt = () =>
  render(
    <MemoryRouter initialEntries={["/execucoes/7"]}>
      <Routes><Route path="/execucoes/:id" element={<RunPage />} /></Routes>
    </MemoryRouter>,
  )

test("finished run shows groups with downloads", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run())
  renderAt()
  expect(await screen.findByText("Concluída")).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: /Data Pack/ })).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: /FRE cortado/ })).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /PETROBRAS_DataPack_Consolidado\.md/ })).toHaveAttribute("href", "/api/runs/7/files/2")
  expect(screen.getAllByRole("link", { name: /Baixar tudo/ })[0]).toHaveAttribute("href", "/api/runs/7/groups/0/zip")
})

test("running run tells the window is open", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "running", finished_at: null, groups: [] }))
  renderAt()
  expect(await screen.findByText(/a janela está aberta no seu computador/)).toBeInTheDocument()
})

test("error shows the message", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "error", error_message: "Já existe um Ferramenta CVM.exe nessa pasta.", groups: [] }))
  renderAt()
  expect(await screen.findByRole("alert")).toHaveTextContent("Já existe um Ferramenta CVM.exe nessa pasta.")
})
```

`AppPage.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError, type AppDetail } from "@/lib/api"
import { manifest, run } from "@/test/fixtures"
import { AppPage } from "./AppPage"

afterEach(() => vi.restoreAllMocks())

const detail = (can_run: boolean): AppDetail => ({
  id: 3, folder_path: "/mnt/c/apps/fp", folder_display: "C:\\apps\\fp", owner_id: 1, backup_id: null, updated_at: "2026-09-30T10:00:00Z",
  manifest: manifest({ kind: "desktop", entrypoint: "Ferramenta CVM.exe", name: "Eq. Research Files Prep" }), can_run,
})

const renderAt = () =>
  render(
    <MemoryRouter initialEntries={["/apps/3"]}>
      <Routes>
        <Route path="/apps/:id" element={<AppPage />} />
        <Route path="/execucoes/:id" element={<p>página da execução</p>} />
      </Routes>
    </MemoryRouter>,
  )

test("Rodar opens the dialog with the app folder and starts the run", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "appRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run({ status: "waiting" }))
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar" }))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("C:\\apps\\fp")
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(start).toHaveBeenCalledWith(3, "C:\\apps\\fp")
  expect(await screen.findByText("página da execução")).toBeInTheDocument()
})

test("choose another folder fills the field", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "appRuns").mockResolvedValue([])
  vi.spyOn(api, "chooseFolder").mockResolvedValue({ folder: "D:\\Saídas", canceled: false })
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar" }))
  await user.click(screen.getByRole("button", { name: "Escolher outra…" }))
  expect(await screen.findByDisplayValue("D:\\Saídas")).toBeInTheDocument()
})

test("computer not connected points to Meu computador", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "appRuns").mockResolvedValue([])
  vi.spyOn(api, "startRun").mockRejectedValue(new ApiError(409, "Seu computador não está conectado."))
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar" }))
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Seu computador não está conectado.")
  expect(screen.getByRole("link", { name: /Meu computador/ })).toHaveAttribute("href", "/meu-computador")
})

test("apps that cannot run have no Rodar button", async () => {
  vi.spyOn(api, "app").mockResolvedValue(detail(false))
  vi.spyOn(api, "appRuns").mockResolvedValue([])
  renderAt()
  expect(await screen.findByText("Eq. Research Files Prep")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Rodar" })).not.toBeInTheDocument()
})

test("lists past runs", async () => {
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "appRuns").mockResolvedValue([
    { id: 7, status: "done", person_name: "Henry", created_at: "2026-09-30T14:10:00Z", started_at: "2026-09-30T14:10:05Z", finished_at: "2026-09-30T14:22:05Z", file_count: 41 },
  ])
  renderAt()
  expect(await screen.findByRole("link", { name: /Concluída/ })).toHaveAttribute("href", "/execucoes/7")
  expect(screen.getByText(/41 arquivos/)).toBeInTheDocument()
})
```

- [ ] **Step 3: Run** — `mise x -- npx vitest run src/pages/RunPage.test.tsx src/pages/AppPage.test.tsx` → FAIL.

- [ ] **Step 4: Implement `RunDialog.tsx`**

```tsx
import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { FolderOpen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { api, ApiError, type AppDetail } from "@/lib/api"

export function RunDialog({ app, open, onOpenChange }: { app: AppDetail; open: boolean; onOpenChange: (v: boolean) => void }) {
  const navigate = useNavigate()
  const [folder, setFolder] = useState(app.folder_display)
  const [error, setError] = useState<{ msg: string; offline: boolean } | null>(null)
  const [busy, setBusy] = useState<"" | "folder" | "run">("")

  const choose = async () => {
    setError(null)
    setBusy("folder")
    try {
      const r = await api.chooseFolder(folder)
      if (!r.canceled) setFolder(r.folder)
    } catch (e) {
      setError({ msg: e instanceof Error ? e.message : "Não foi possível abrir a janela de pastas.", offline: e instanceof ApiError && e.status === 409 })
    } finally {
      setBusy("")
    }
  }

  const start = async () => {
    setError(null)
    setBusy("run")
    try {
      const r = await api.startRun(app.id, folder)
      navigate(`/execucoes/${r.id}`)
    } catch (e) {
      setError({ msg: e instanceof Error ? e.message : "Não foi possível abrir o app.", offline: e instanceof ApiError && e.status === 409 })
      setBusy("")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rodar {app.manifest.name}</DialogTitle>
          <DialogDescription>O app vai abrir no seu computador. Use a janela normalmente e feche quando terminar. O Hub mostra os arquivos que ele gerou.</DialogDescription>
        </DialogHeader>
        <Field label="Onde salvar os resultados?" htmlFor="run-folder" hint="Por padrão, a própria pasta do app.">
          <div className="flex gap-2">
            <Input id="run-folder" value={folder} onChange={(e) => setFolder(e.target.value)} className="font-mono text-xs" />
            <Button type="button" variant="outline" onClick={choose} disabled={busy !== ""} className="shrink-0 gap-2">
              <FolderOpen className="size-4" /> Escolher outra…
            </Button>
          </div>
        </Field>
        {busy === "folder" && <p className="text-sm text-muted-foreground">A janela de pastas abriu no seu computador.</p>}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error.msg}{" "}
            {error.offline && <Link to="/meu-computador" className="underline">Ver Meu computador</Link>}
          </p>
        )}
        <DialogFooter>
          <Button onClick={start} disabled={busy !== "" || folder.trim() === ""}>Abrir app</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

Check `Field` renders a `<label htmlFor>` so `getByLabelText` works; if not, add `aria-label="Onde salvar os resultados?"` to the input.

- [ ] **Step 5: Implement `RunsList.tsx`**

```tsx
import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { api, type RunSummary } from "@/lib/api"
import { formatDuration, formatWhen, STATUS } from "@/lib/runs"

export function RunsList({ appId }: { appId: number }) {
  const [runs, setRuns] = useState<RunSummary[] | null>(null)
  useEffect(() => {
    api.appRuns(appId).then(setRuns, () => setRuns([]))
  }, [appId])
  if (!runs) return null
  if (runs.length === 0) return <p className="text-sm text-muted-foreground">Nenhuma execução ainda.</p>
  return (
    <ul className="flex flex-col divide-y divide-border text-sm">
      {runs.map((r) => (
        <li key={r.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 py-2">
          <Link to={`/execucoes/${r.id}`} className="font-medium underline-offset-4 hover:underline">{STATUS[r.status].label}</Link>
          <span className="text-muted-foreground">{formatWhen(r.created_at)} · {r.person_name} · {formatDuration(r.started_at, r.finished_at)}</span>
          <span className="ml-auto font-mono text-xs text-muted-foreground">{r.file_count} arquivos</span>
        </li>
      ))}
    </ul>
  )
}
```

- [ ] **Step 6: Wire into `AppPage.tsx`**

Import `Play` from lucide, `Button`, `RunDialog`, `RunsList`. Add `const [runOpen, setRunOpen] = useState(false)`. Below the summary paragraph (after the `<p>` with `m.summary`):

```tsx
        {app.can_run && (
          <div className="mt-5">
            <Button onClick={() => setRunOpen(true)} className="gap-2">
              <Play className="size-4" /> Rodar
            </Button>
            <RunDialog app={app} open={runOpen} onOpenChange={setRunOpen} />
          </div>
        )}
```

and a new section before "Onde está":

```tsx
          <Section title="Execuções">
            <RunsList appId={app.id} />
          </Section>
```

- [ ] **Step 7: Implement `RunPage.tsx`**

```tsx
import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, Download, LoaderCircle } from "lucide-react"
import { buttonVariants } from "@/components/ui/button"
import { api, fileUrl, groupZipUrl, type Run, type RunGroup } from "@/lib/api"
import { formatDuration, formatSize, formatWhen, isLive, STATUS } from "@/lib/runs"
import { cn } from "@/lib/utils"

const SHOWN = 10

function Group({ run, g }: { run: Run; g: RunGroup }) {
  const [all, setAll] = useState(false)
  const files = all ? g.files : g.files.slice(0, SHOWN)
  return (
    <section className="border-t border-border py-5">
      <div className="mb-3 flex items-center justify-between gap-4">
        <h2 className="font-mono text-xs tracking-[0.16em] text-muted-foreground uppercase">{g.label} ({g.files.length})</h2>
        {g.files.length > 1 && (
          <a href={groupZipUrl(run.id, g.index)} className={cn(buttonVariants({ variant: "outline", size: "sm" }), "gap-2")}>
            <Download className="size-4" /> Baixar tudo
          </a>
        )}
      </div>
      {g.files.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nada desta vez.</p>
      ) : (
        <ul className="flex flex-col gap-1.5 text-sm">
          {files.map((f) => (
            <li key={f.id} className="flex items-baseline gap-3">
              <a href={fileUrl(run.id, f.id)} className="min-w-0 truncate font-mono text-xs underline-offset-4 hover:underline">{f.path}</a>
              <span className="ml-auto shrink-0 text-xs text-muted-foreground">{f.change === "changed" ? "alterado · " : ""}{formatSize(f.size)}</span>
            </li>
          ))}
          {!all && g.files.length > SHOWN && (
            <li>
              <button type="button" onClick={() => setAll(true)} className="text-sm text-stamp underline-offset-4 hover:underline">
                … mais {g.files.length - SHOWN}
              </button>
            </li>
          )}
        </ul>
      )}
    </section>
  )
}

export function RunPage() {
  const { id } = useParams()
  const [run, setRun] = useState<Run | null>(null)
  const [error, setError] = useState("")
  const [, setNow] = useState(Date.now())

  useEffect(() => {
    if (!id) return
    let stop = false
    const load = () => api.run(id).then((r) => !stop && setRun(r), (e: Error) => !stop && setError(e.message))
    load()
    const t = setInterval(() => {
      setNow(Date.now())
      setRun((r) => {
        if (r && isLive(r.status)) load()
        return r
      })
    }, 2000)
    return () => {
      stop = true
      clearInterval(t)
    }
  }, [id])

  if (error) return <p role="alert" className="text-destructive">{error}</p>
  if (!run) return <p className="text-muted-foreground">Carregando…</p>
  const st = STATUS[run.status]

  return (
    <article className="rise">
      <Link to={`/apps/${run.app_id}`} className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> {run.app_name}
      </Link>
      <div className="paper rounded-lg border border-border bg-card py-8 pr-8 pl-20 shadow-card">
        <p className="font-mono text-xs tracking-[0.16em] text-stamp uppercase">Execução</p>
        <h1 className="mt-2 font-heading text-3xl">{formatWhen(run.created_at)} · {formatDuration(run.started_at, run.finished_at)} · {run.person_name}</h1>
        <p className={cn("mt-3 inline-flex items-center gap-2 font-medium", st.tone === "bad" && "text-destructive", st.tone === "ok" && "text-ok", st.tone === "live" && "text-stamp")}>
          {isLive(run.status) && <LoaderCircle className="size-4 animate-spin" />}
          {st.label}
        </p>
        {run.status === "running" && <p className="mt-2 text-muted-foreground">Rodando: a janela está aberta no seu computador. Quando você fechar, os arquivos aparecem aqui.</p>}
        {run.status === "waiting" && <p className="mt-2 text-muted-foreground">Esperando o computador {run.computer_name} abrir o app…</p>}
        {(run.status === "error" || run.status === "interrupted") && run.error_message && (
          <p role="alert" className="mt-2 text-destructive">{run.error_message}</p>
        )}
        <dl className="mt-6 grid gap-x-8 gap-y-3 text-sm sm:grid-cols-3">
          <div><dt className="text-muted-foreground">Pasta</dt><dd className="font-mono text-xs break-all">{run.output_folder}</dd></div>
          <div><dt className="text-muted-foreground">Computador</dt><dd className="font-medium">{run.computer_name}</dd></div>
          <div><dt className="text-muted-foreground">Versão do app</dt><dd className="font-medium">{run.app_version || "—"}</dd></div>
        </dl>
        {!isLive(run.status) && run.groups.length > 0 && (
          <div className="mt-6">{run.groups.map((g) => <Group key={`${g.index}-${g.label}`} run={run} g={g} />)}</div>
        )}
      </div>
    </article>
  )
}
```

Route in `App.tsx`: `<Route path="execucoes/:id" element={<RunPage />} />`.

- [ ] **Step 8: Run the whole suite** — `mise x -- npx vitest run && mise x -- npx tsc -b && mise x -- npm run lint` → all pass.

- [ ] **Step 9: Commit**

```bash
git add frontend/src
git commit -m "Frontend: Rodar dialog, run history and run page"
```

---

### Task 14: Visual check

**Files:**
- Modify: `e2e/screenshots.mjs`

- [ ] **Step 1:** Add shots (after dev login) for `/meu-computador`, `/apps/<Files Prep id>` with the dialog open (click "Rodar"), and `/execucoes/<id>` for a finished run. Seed a finished run for the screenshot through the real agent in Task 16, or, before that, by pairing a fake agent with `curl` against the API (`pair/start` → approve in the page → `pair/poll` → `POST /api/apps/{id}/runs` → `next` → `result` finished with a few files).
- [ ] **Step 2:** Run the screenshots (light and dark, 1440 and 375 wide), read each PNG, fix spacing/contrast issues, re-run tests.
- [ ] **Step 3: Commit** `git add e2e frontend && git commit -m "Visual check for computer, run dialog and run page"`.

---

### Task 15: Docs

**Files:**
- Create: `docs/adr/004-agente-do-hub-no-windows.md`
- Modify: `docs/adr/001-hub-roda-no-wsl-e-le-pastas-do-windows.md`, `docs/PRD.md`, `docs/TASKS.md`, `docs/INFRASTRUCTURE.md`

- [ ] **Step 1: ADR 004** (template from tech-stack): context (Hub in WSL, apps need Windows, Module 2 cloud cannot open windows remotely); decision (Agent on Windows, agent-initiated long-poll, per-computer token, jobs in Postgres, downloads streamed on demand, invisible, same Go module as backend); rationale; trade-offs (one more program to install/update; downloads need the computer on; `.exe` copy only for single-file exes); alternatives (Hub calling `powershell.exe` via WSL interop — rejected by Henry in favour of building the agent now; tray icon — deferred; uploading every result to the Hub — deferred to Module 2).
- [ ] **Step 2: ADR 001**: replace the sentence about interop with "A execução é feita pelo Agente do Hub no Windows (ADR 004)."
- [ ] **Step 3: PRD 3.3**: add the Agent, the "Onde salvar" choice with default app folder, downloads via the agent, and "only who ran and admins see a run until Phase 6".
- [ ] **Step 4: TASKS Fase 4**: replace the first two rows with: "Agente do Hub no Windows (instalar, conectar, ligar com o Windows)", "Rodar app com janela/.exe pelo Agente, com pasta escolhida", "Resultados agrupados pelo manifesto", "Baixar resultados pelo Agente (arquivo e .zip)", "Registro das execuções (quem, quando, duração, pasta, computador, versão)" — status per what is done; mark "Registro e repetição" as partially done (repetir pending). Update "Prioridade atual" item 1.
- [ ] **Step 5: INFRASTRUCTURE**: note "Agente do Hub (AppHubAgente.exe) em cada computador Windows; fala com o Hub pela porta 8080; compilado por `scripts/build-agent.sh`; `AGENT_BINARY_PATH` no `.env`." (not a container).
- [ ] **Step 6: Commit** `git add docs && git commit -m "Docs: ADR 004 agent, PRD, TASKS and infrastructure for Phase 4"`.

---

### Task 16: Real test on Henry's computer

Needs Henry at the keyboard. Keep the Hub (Go + Vite) running.

- [ ] **Step 1:** `scripts/build-agent.sh`; confirm `.env` has `AGENT_BINARY_PATH`; restart Go.
- [ ] **Step 2:** Henry opens `http://localhost:5173/meu-computador`, clicks **Instalar o Agente**, opens the downloaded file, clicks **Conectar** in the page that opens. Check: page shows "Conectado"; `%LOCALAPPDATA%\AppHub\AppHubAgente.exe` exists (`ls /mnt/c/Users/HenryKroppAlisInvest/AppData/Local/AppHub`); Run key exists (`reg.exe query 'HKCU\Software\Microsoft\Windows\CurrentVersion\Run' /v AppHubAgente`).
- [ ] **Step 3:** Rodar the Eq. Research Files Prep in the original folder; Henry downloads a small company (1 year, FRE only) and generates a Data Pack, closes the window. Check groups, download the Data Pack, download a group zip.
- [ ] **Step 4:** Rodar again choosing another folder with **Escolher outra…** (e.g. `C:\Users\HenryKroppAlisInvest\Documents\Teste Hub`). Check the window's "Pasta raiz" shows it and the `.exe` copy is gone after closing.
- [ ] **Step 5:** Disconnect in Meu computador; Rodar must say "Seu computador não está conectado." Reconnect by opening the Agent again (it pairs again because the token was forgotten).
- [ ] **Step 6:** Record any fixes as new tests + commits; update TASKS with what was verified.
