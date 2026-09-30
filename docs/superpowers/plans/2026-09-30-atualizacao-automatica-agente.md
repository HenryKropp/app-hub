# Atualização automática do Agente — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When the Hub has a newer Agente, each connected Agente (0.2.0+) downloads it, swaps itself in when no app is open, and rolls back to the previous copy if the new one does not reach the Hub in 2 minutes.

**Architecture:** The Hub queues a new `update` job (same `agent_jobs` queue) on `hello` and at the start of each `next` when the computer's version differs, it can update, the binary exists, no update is active, and fewer than 3 taken updates for that version failed. The Agente holds its Runner lock (so no app opens), downloads `/api/agent/binary` with its token, checks size + SHA-256, backs up its own exe and starts the download with `--update`. That process (the "swap") quits the installed agent, copies itself over it, starts it and polls its local `/status` for `connected && version == mine`; otherwise it restores the backup. The Hub closes the taken `update` job on the next `hello`: done if the hello version equals the job's target version, failed otherwise.

**Tech Stack:** Go 1 (stdlib `net/http`, `crypto/sha256`), pgx/v5 + sqlc, Postgres (`supabase/postgres` container `app-hub-db`, port 5433), React + Vitest.

**Spec:** `docs/superpowers/specs/2026-09-30-atualizacao-automatica-agente-design.md`

## Global Constraints

- `agentproto.Version` becomes `"0.2.0"`. Every published Agente needs a new number; same number means no update.
- Only computers whose `hello` sent `can_update: true` get `update` jobs; 0.1.1 keeps the "Atualize o Agente" message.
- Never swap while an app opened by the Hub is open (Runner lock held for the whole update).
- Rollback window: 2 minutes. Give up after **3** failed `update` jobs for the same target version, counting only jobs with `taken_at` set.
- `/api/agent/binary` is authenticated with the computer token (not the person login).
- UI copy (pt-BR, exact):
  - updating: `Atualizando o Agente para a versão {v}…`
  - updated (within 24 h): `Agente atualizado sozinho em {formatWhen}.`
  - gave up: `Não conseguimos atualizar o Agente sozinho. Baixe e abra a versão {expected} de novo.`
  - download mismatch: `O arquivo baixado veio diferente do esperado.`
  - run during update: `O Agente está se atualizando. Tente de novo em um minuto.`
- Go commands run from `backend/` via `mise x --`; handler tests need the DB (`podman start app-hub-db`) and `.env` loaded (they read `DATABASE_URL`/test pool as today).
- slog only; no raw SQL in handlers (sqlc).

## Review Focus

1. The Hub is restarted with a new version while the Agente is long-polling (no new `hello`) → the update must still be queued from `next` (test in Task 1).
2. Windows keeps the old exe locked for a moment after `/quit` → the swap must retry the copy, and if it never succeeds, start the old agent again instead of leaving none running (tests in Task 4).
3. The old agent may still answer `/status` during the swap → "connected" only counts when `/status` reports the new version (test in Task 4).
4. Someone clicks Rodar during the update → the run fails with the "está se atualizando" message, not "Já tem um app aberto" (test in Task 3).
5. The download is cut short (Hub restart, network) → size/SHA mismatch, file removed, job failed, nothing started (test in Task 3).

---

### Task 1: Hub queues and closes `update` jobs; binary endpoint

**Files:**
- Modify: `backend/internal/agentproto/proto.go`
- Create: `backend/internal/database/migrations/004_agent_update.sql`
- Modify: `backend/internal/database/queries/agents.sql`
- Regenerate: `backend/internal/database/sqlc/*`
- Create: `backend/internal/handler/agent_update.go`
- Modify: `backend/internal/handler/agents.go` (`agentHello`, `agentNext`)
- Modify: `backend/internal/handler/handler.go` (route)
- Test: `backend/internal/handler/agent_update_test.go`

**Interfaces:**
- Produces (agentproto): `KindUpdate = "update"`; `type UpdatePayload struct { Version string \`json:"version"\`; SHA256 string \`json:"sha256"\`; Size int64 \`json:"size"\` }`; `Hello.CanUpdate bool \`json:"can_update,omitempty"\``.
- Produces (handler): `const maxUpdateTries = 3`; `func (h *Handler) maybeEnqueueUpdate(ctx context.Context, c sqlc.Computer) error`; `func fileDigest(path string) (sum string, size int64, err error)`; route `GET /api/agent/binary`.
- Produces (sqlc, verify names after generate): `CountActiveUpdates(ctx, computerID int64) (int64, error)`, `CountFailedUpdates(ctx, CountFailedUpdatesParams{ComputerID, TargetVersion}) (int64, error)`, `ResolveTakenUpdates(ctx, ResolveTakenUpdatesParams{ComputerID, TargetVersion}) error`, `LatestUpdateJob(ctx, computerID int64) (AgentJob, error)`, `HelloComputerParams` gains `CanUpdate bool`, `Computer` gains `CanUpdate bool`.

- [ ] **Step 1: Protocol additions** (`proto.go`)

```go
// Job kinds.
const (
	KindRun          = "run"
	KindChooseFolder = "choose_folder"
	KindSendFile     = "send_file"
	KindUpdate       = "update"
)

// UpdatePayload asks the agent to replace itself with the Hub's agent binary.
type UpdatePayload struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type Hello struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	CanUpdate bool   `json:"can_update,omitempty"` // 0.2.0+: understands update jobs
}
```

Leave `Version = "0.1.1"` for now (bumped in Task 5).

- [ ] **Step 2: Migration** `004_agent_update.sql`

```sql
ALTER TABLE computers ADD COLUMN IF NOT EXISTS can_update BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE agent_jobs DROP CONSTRAINT IF EXISTS agent_jobs_kind_check;
ALTER TABLE agent_jobs ADD CONSTRAINT agent_jobs_kind_check
    CHECK (kind IN ('run', 'choose_folder', 'send_file', 'update'));

-- At most one update in flight per computer.
CREATE UNIQUE INDEX IF NOT EXISTS agent_jobs_one_update_idx ON agent_jobs(computer_id)
    WHERE kind = 'update' AND status IN ('pending', 'taken');
```

- [ ] **Step 3: Queries** (`agents.sql`): replace `HelloComputer` and append the rest.

```sql
-- name: HelloComputer :exec
UPDATE computers SET last_seen_at = now(), name = $2, agent_version = $3, can_update = $4 WHERE id = $1;

-- name: CountActiveUpdates :one
SELECT count(*) FROM agent_jobs
WHERE computer_id = $1 AND kind = 'update' AND status IN ('pending', 'taken');

-- name: CountFailedUpdates :one
-- Only updates the agent took count; one that expired in the queue does not.
SELECT count(*) FROM agent_jobs
WHERE computer_id = $1 AND kind = 'update' AND status = 'failed'
  AND taken_at IS NOT NULL AND payload->>'version' = sqlc.arg(target_version)::text;

-- name: ResolveTakenUpdates :exec
-- Called on hello: the agent restarted, so a taken update either worked
-- (it now reports the target version) or was rolled back.
UPDATE agent_jobs
SET status = CASE WHEN payload->>'version' = sqlc.arg(target_version)::text THEN 'done' ELSE 'failed' END,
    done_at = now()
WHERE computer_id = $1 AND kind = 'update' AND status = 'taken';

-- name: LatestUpdateJob :one
SELECT * FROM agent_jobs WHERE computer_id = $1 AND kind = 'update' ORDER BY id DESC LIMIT 1;
```

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- sqlc generate'`, then **read** `internal/database/sqlc/agents.sql.go` and `models.go` and adjust the names used below if sqlc chose different ones.

- [ ] **Step 4: Write the failing tests** `agent_update_test.go`

```go
package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// withBinary points the Hub at a fake agent binary and returns its digest.
func withBinary(t *testing.T, h *Handler, body string) (string, int64) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "AppHubAgente.exe")
	if err := os.WriteFile(bin, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h.cfg.AgentBinaryPath = bin
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:]), int64(len(body))
}

func nextJob(t *testing.T, c *client, token string) (agentproto.Job, int) {
	t.Helper()
	var j agentproto.Job
	code := c.agent("GET", "/api/agent/next", token, nil, &j)
	return j, code
}

func updateJobs(t *testing.T) (total int) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM agent_jobs WHERE kind = 'update'").Scan(&total); err != nil {
		t.Fatal(err)
	}
	return total
}

func jobStatus(t *testing.T, id int64) (status string) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(), "SELECT status FROM agent_jobs WHERE id = $1", id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestUpdateQueuedOnHello(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	sum, size := withBinary(t, h, "MZ-new")

	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	if n := updateJobs(t); n != 1 {
		t.Fatalf("want 1 update job, got %d", n)
	}
	j, code := nextJob(t, c, token)
	if code != http.StatusOK || j.Kind != agentproto.KindUpdate {
		t.Fatalf("next %d %+v", code, j)
	}
	var p agentproto.UpdatePayload
	_ = json.Unmarshal(j.Payload, &p)
	if p.Version != agentproto.Version || p.SHA256 != sum || p.Size != size {
		t.Fatalf("payload %+v", p)
	}
}

func TestNoUpdateWhenNotPossible(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")

	// No binary on this Hub.
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	// Old agent that cannot update.
	withBinary(t, h, "MZ")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.1"}, nil)
	// Already on the Hub's version.
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true}, nil)
	if n := updateJobs(t); n != 0 {
		t.Fatalf("want no update job, got %d", n)
	}
	if _, code := nextJob(t, c, token); code != http.StatusNoContent {
		t.Fatalf("next %d", code)
	}
}

// Review Focus 1: the Hub restarted with a new version; the agent keeps
// long-polling without a new hello.
func TestUpdateQueuedOnNext(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true}, nil)
	if _, err := testPool.Exec(context.Background(), "UPDATE computers SET agent_version = '0.1.9'"); err != nil {
		t.Fatal(err)
	}
	if j, code := nextJob(t, c, token); code != http.StatusOK || j.Kind != agentproto.KindUpdate {
		t.Fatalf("next %d %+v", code, j)
	}
}

func TestHelloClosesUpdate(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	old := agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}

	c.agent("POST", "/api/agent/hello", token, old, nil)
	j, _ := nextJob(t, c, token)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true}, nil)
	if s := jobStatus(t, j.ID); s != "done" {
		t.Fatalf("worked update: %s", s)
	}
}

func TestUpdateGivesUpAfterThreeFailures(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	old := agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}

	c.agent("POST", "/api/agent/hello", token, old, nil)
	for i := range 3 {
		j, code := nextJob(t, c, token)
		if code != http.StatusOK || j.Kind != agentproto.KindUpdate {
			t.Fatalf("try %d: next %d %+v", i, code, j)
		}
		c.agent("POST", "/api/agent/hello", token, old, nil) // rolled back
		if s := jobStatus(t, j.ID); s != "failed" {
			t.Fatalf("try %d: %s", i, s)
		}
	}
	if _, code := nextJob(t, c, token); code != http.StatusNoContent {
		t.Fatalf("after 3 failures next %d", code)
	}
}

func TestExpiredUpdateDoesNotCount(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}, nil)
	if _, err := testPool.Exec(context.Background(), "UPDATE agent_jobs SET created_at = now() - interval '3 minutes'"); err != nil {
		t.Fatal(err)
	}
	// next expires the old one and queues a fresh one.
	j, code := nextJob(t, c, token)
	if code != http.StatusOK || j.Kind != agentproto.KindUpdate || j.ID != 2 {
		t.Fatalf("next %d %+v", code, j)
	}
}

func TestAgentBinaryNeedsComputerToken(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ-new")

	if code := c.agent("GET", "/api/agent/binary", "", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("no token %d", code)
	}
	req, _ := http.NewRequest("GET", c.srv.URL+"/api/agent/binary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "MZ-new" {
		t.Fatalf("%d %q", resp.StatusCode, b)
	}
}
```

- [ ] **Step 5: Run to see them fail**

Run: `bash -c 'ROOT="$(git rev-parse --show-toplevel)" && set -a && . "$ROOT/.env" && set +a && cd "$ROOT/backend" && mise x -- go test ./internal/handler -run "Update|AgentBinary" -count=1'`
Expected: compile errors / FAIL (`maybeEnqueueUpdate` missing, no route).

- [ ] **Step 6: Implement** `agent_update.go`

```go
package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

// maxUpdateTries is how many taken updates to one version may fail before the
// Hub stops asking and shows "Atualize o Agente" again.
const maxUpdateTries = 3

// maybeEnqueueUpdate queues an update job when this computer's agent can
// update itself and runs a different version than the Hub's.
func (h *Handler) maybeEnqueueUpdate(ctx context.Context, c sqlc.Computer) error {
	if !c.CanUpdate || c.AgentVersion == "" || c.AgentVersion == agentproto.Version || h.cfg.AgentBinaryPath == "" {
		return nil
	}
	active, err := h.q.CountActiveUpdates(ctx, c.ID)
	if err != nil || active > 0 {
		return err
	}
	failed, err := h.q.CountFailedUpdates(ctx, sqlc.CountFailedUpdatesParams{ComputerID: c.ID, TargetVersion: agentproto.Version})
	if err != nil || failed >= maxUpdateTries {
		return err
	}
	sum, size, err := fileDigest(h.cfg.AgentBinaryPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = h.enqueue(ctx, c.ID, nil, agentproto.KindUpdate, agentproto.UpdatePayload{Version: agentproto.Version, SHA256: sum, Size: size})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil // hello and next raced; one update is already queued
	}
	return err
}

func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	sum := sha256.New()
	n, err := io.Copy(sum, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(sum.Sum(nil)), n, nil
}

// agentBinary serves the agent binary to a connected agent (update jobs).
func (h *Handler) agentBinary(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentComputer(w, r); !ok {
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
	http.ServeFile(w, r, h.cfg.AgentBinaryPath)
}
```

In `agents.go` `agentHello`, replace the `HelloComputer` call and add the update steps **before** `InterruptTakenRuns`/`FailTakenJobs`:

```go
	if err := h.q.HelloComputer(r.Context(), sqlc.HelloComputerParams{ID: c.ID, Name: name, AgentVersion: body.Version, CanUpdate: body.CanUpdate}); err != nil {
		internalError(w, "hello", err)
		return
	}
	if err := h.q.ResolveTakenUpdates(r.Context(), sqlc.ResolveTakenUpdatesParams{ComputerID: c.ID, TargetVersion: body.Version}); err != nil {
		internalError(w, "resolve updates", err)
		return
	}
```

and after `FailTakenJobs`, before `w.WriteHeader(http.StatusNoContent)`:

```go
	c.AgentVersion, c.CanUpdate = body.Version, body.CanUpdate
	if err := h.maybeEnqueueUpdate(r.Context(), c); err != nil {
		slog.Error("queue agent update", "computer", c.ID, "err", err)
	}
```

In `agentNext`, after `ExpireStaleJobs` and before the `deadline` loop:

```go
	if err := h.maybeEnqueueUpdate(ctx, c); err != nil {
		slog.Error("queue agent update", "computer", c.ID, "err", err)
	}
```

(add `"log/slog"` to the `agents.go` imports if it is not there.) In `handler.go`, next to `GET /api/agent/download`:

```go
	mux.HandleFunc("GET /api/agent/binary", h.agentBinary)
```

- [ ] **Step 7: Run tests** — the same command as Step 5 without `-run`, i.e. the whole `./internal/handler` package. Expected: PASS, including the older `TestComputerAuthAndRevoke`.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/agentproto backend/internal/database backend/internal/handler
git commit -m "Hub: queue and close agent update jobs; agent binary endpoint"
```

---

### Task 2: Meu computador shows the update state

**Files:**
- Modify: `backend/internal/handler/agents.go` (`computerJSON`, `myComputers`)
- Modify: `backend/internal/handler/agent_update.go` (`updateState`)
- Test: `backend/internal/handler/agent_update_test.go`
- Modify: `frontend/src/lib/api.ts`, `frontend/src/test/fixtures.ts`, `frontend/src/pages/ComputerPage.tsx`
- Test: `frontend/src/pages/ComputerPage.test.tsx`

**Interfaces:**
- Consumes: `LatestUpdateJob`, `CountFailedUpdates`, `maxUpdateTries`, `agentproto.UpdatePayload` (Task 1).
- Produces JSON on each computer: `update_state: "" | "updating" | "updated" | "gave_up"`, `update_version?: string`, `updated_at?: string`. `outdated` now means "show the manual download message": version differs **and** (`!can_update` or gave up).

- [ ] **Step 1: Failing Go test** (append to `agent_update_test.go`)

```go
func TestComputerUpdateState(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	token := pairComputer(t, c, "PC")
	withBinary(t, h, "MZ")
	state := func() computerJSON {
		var me struct {
			Computers []computerJSON `json:"computers"`
		}
		c.do("GET", "/api/computers/me", nil, &me)
		return me.Computers[0]
	}
	old := agentproto.Hello{Name: "PC", Version: "0.1.9", CanUpdate: true}

	c.agent("POST", "/api/agent/hello", token, old, nil)
	if s := state(); s.UpdateState != "updating" || s.UpdateVersion != agentproto.Version || s.Outdated {
		t.Fatalf("updating: %+v", s)
	}
	nextJob(t, c, token)
	c.agent("POST", "/api/agent/hello", token, agentproto.Hello{Name: "PC", Version: agentproto.Version, CanUpdate: true}, nil)
	if s := state(); s.UpdateState != "updated" || s.UpdatedAt == nil {
		t.Fatalf("updated: %+v", s)
	}

	// Three rollbacks: give up and show the manual message.
	if _, err := testPool.Exec(context.Background(), "UPDATE computers SET agent_version = '0.1.9'"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		nextJob(t, c, token)
		c.agent("POST", "/api/agent/hello", token, old, nil)
	}
	if s := state(); s.UpdateState != "gave_up" || !s.Outdated {
		t.Fatalf("gave up: %+v", s)
	}
}
```

- [ ] **Step 2: Run it** (`go test ./internal/handler -run TestComputerUpdateState -count=1`, same env wrapper as Task 1). Expected: compile error (fields missing).

- [ ] **Step 3: Implement.** In `agents.go`:

```go
type computerJSON struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	AgentVersion string     `json:"agent_version"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	Connected    bool       `json:"connected"`
	// Outdated: the person has to download the agent by hand.
	Outdated      bool       `json:"outdated"`
	UpdateState   string     `json:"update_state"` // "", "updating", "updated", "gave_up"
	UpdateVersion string     `json:"update_version,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}
```

In `myComputers`, replace the loop body:

```go
	for _, c := range rows {
		j := computerJSON{ID: c.ID, Name: c.Name, AgentVersion: c.AgentVersion, Connected: c.Connected}
		if c.LastSeenAt.Valid {
			j.LastSeenAt = &c.LastSeenAt.Time
		}
		differs := c.AgentVersion != "" && c.AgentVersion != agentproto.Version
		if j.UpdateState, j.UpdateVersion, j.UpdatedAt, err = h.updateState(r.Context(), c.ID, differs); err != nil {
			internalError(w, "update state", err)
			return
		}
		j.Outdated = differs && (!c.CanUpdate || j.UpdateState == "gave_up")
		out = append(out, j)
	}
```

(`ListPersonComputers` uses `SELECT *, ...` so the row gains `CanUpdate` after regenerate; confirm the field name in `agents.sql.go`.) In `agent_update.go`:

```go
// updateState summarises the latest update job of a computer for the page.
func (h *Handler) updateState(ctx context.Context, computerID int64, differs bool) (string, string, *time.Time, error) {
	j, err := h.q.LatestUpdateJob(ctx, computerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, nil
	}
	if err != nil {
		return "", "", nil, err
	}
	var p agentproto.UpdatePayload
	_ = json.Unmarshal(j.Payload, &p)
	switch {
	case j.Status == "pending" || j.Status == "taken":
		return "updating", p.Version, nil, nil
	case j.Status == "done" && j.DoneAt.Valid && time.Since(j.DoneAt.Time) < 24*time.Hour && !differs:
		return "updated", p.Version, &j.DoneAt.Time, nil
	}
	if !differs {
		return "", "", nil, nil
	}
	failed, err := h.q.CountFailedUpdates(ctx, sqlc.CountFailedUpdatesParams{ComputerID: computerID, TargetVersion: agentproto.Version})
	if err != nil {
		return "", "", nil, err
	}
	if failed >= maxUpdateTries {
		return "gave_up", "", nil, nil
	}
	return "", "", nil, nil
}
```

(imports: `encoding/json`, `time`, `github.com/jackc/pgx/v5`.)

- [ ] **Step 4: Go tests pass** — run the whole handler package. Expected: PASS.

- [ ] **Step 5: Failing Vitest tests** (append to `ComputerPage.test.tsx`)

```tsx
test("update states", async () => {
  const info = (over: Partial<Computer>) => ({ computers: [computer(over)], expected_version: "0.2.0", agent_available: true })
  const spy = vi.spyOn(api, "computers").mockResolvedValue(info({ update_state: "updating", update_version: "0.2.0" }))
  const { unmount } = renderPage()
  expect(await screen.findByText("Atualizando o Agente para a versão 0.2.0…")).toBeInTheDocument()
  unmount()

  spy.mockResolvedValue(info({ update_state: "updated", update_version: "0.2.0", updated_at: "2026-09-30T14:00:00Z", agent_version: "0.2.0" }))
  renderPage()
  expect(await screen.findByText(/Agente atualizado sozinho em/)).toBeInTheDocument()
  cleanup()

  spy.mockResolvedValue(info({ update_state: "gave_up", outdated: true, agent_version: "0.1.9" }))
  renderPage()
  expect(await screen.findByText("Não conseguimos atualizar o Agente sozinho. Baixe e abra a versão 0.2.0 de novo.")).toBeInTheDocument()
  expect(screen.queryByText(/Atualize o Agente/)).not.toBeInTheDocument()
})
```

(add `cleanup` to the `@testing-library/react` import and `type Computer` from `@/lib/api`.)

- [ ] **Step 6: Run** `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/pages/ComputerPage.test.tsx'`. Expected: FAIL.

- [ ] **Step 7: Implement.** `api.ts`:

```ts
export type UpdateState = "" | "updating" | "updated" | "gave_up"
export type Computer = {
  id: number; name: string; agent_version: string; last_seen_at: string | null; connected: boolean; outdated: boolean
  update_state: UpdateState; update_version?: string; updated_at?: string
}
```

`fixtures.ts` `computer()`: add `update_state: ""` to the defaults. `ComputerPage.tsx`: import `RefreshCw` from `lucide-react` and replace the `{c.outdated && (...)}` block with:

```tsx
                {c.update_state === "updating" && (
                  <p className="mt-2 flex items-center gap-2 text-sm text-muted-foreground">
                    <RefreshCw className="size-4 animate-spin" /> Atualizando o Agente para a versão {c.update_version}…
                  </p>
                )}
                {c.update_state === "updated" && c.updated_at && (
                  <p className="mt-2 text-sm text-ok">Agente atualizado sozinho em {formatWhen(c.updated_at)}.</p>
                )}
                {c.update_state === "gave_up" ? (
                  <p className="mt-2 flex items-center gap-2 text-sm text-warn">
                    <TriangleAlert className="size-4" /> Não conseguimos atualizar o Agente sozinho. Baixe e abra a versão {info.expected_version} de novo.
                  </p>
                ) : c.outdated && (
                  <p className="mt-2 flex items-center gap-2 text-sm text-warn">
                    <TriangleAlert className="size-4" /> Atualize o Agente: baixe e abra a versão {info.expected_version}.
                  </p>
                )}
```

- [ ] **Step 8: Run** Vitest for the file, then `mise x -- npx tsc -b` in `frontend/`. Expected: PASS, no type errors.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/handler frontend/src
git commit -m "Meu computador: show agent updating, updated and gave-up states"
```

---

### Task 3: Agent downloads, checks and hands over to the new version

**Files:**
- Modify: `backend/internal/agent/client.go` (`DownloadBinary`)
- Modify: `backend/internal/agent/runner.go` (`Hold`, busy message)
- Create: `backend/internal/agent/update.go`
- Modify: `backend/internal/agent/agent.go` (`Updater` field, `KindUpdate` case)
- Test: `backend/internal/agent/update_test.go`

**Interfaces:**
- Consumes: `agentproto.KindUpdate`, `agentproto.UpdatePayload` (Task 1).
- Produces: `func (c *Client) DownloadBinary(ctx context.Context, w io.Writer) (int64, error)`; `func (r *Runner) Hold() (release func())`; `type Updater struct { Client *Client; Runner *Runner; StateDir, CurrentExe string; Start func(exe string, args ...string) error }`; `func (u *Updater) Apply(ctx context.Context, p agentproto.UpdatePayload) error`; `var ErrBadDownload`; `func replaceFile(src, dst string) error`; `Agent.Updater *Updater`.

- [ ] **Step 1: Failing tests** `update_test.go`

```go
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

func digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

// binaryHub serves body at /api/agent/binary.
func binaryHub(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/binary" || r.Header.Get("Authorization") != "Bearer t" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}
}

type startRec struct {
	mu    sync.Mutex
	calls [][]string
}

func (s *startRec) start(exe string, args ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, append([]string{exe}, args...))
	return nil
}

func (s *startRec) n() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.calls) }

func newUpdater(t *testing.T, c *Client) (*Updater, *startRec) {
	t.Helper()
	state := t.TempDir()
	cur := filepath.Join(state, "AppHubAgente.exe")
	_ = os.WriteFile(cur, []byte("MZ-old"), 0o755)
	rec := &startRec{}
	return &Updater{Client: c, Runner: &Runner{}, StateDir: state, CurrentExe: cur, Start: rec.start}, rec
}

func TestUpdaterAppliesVerifiedDownload(t *testing.T) {
	u, rec := newUpdater(t, binaryHub(t, "MZ-new"))
	if err := u.Apply(context.Background(), agentproto.UpdatePayload{Version: "0.2.1", SHA256: digest("MZ-new"), Size: 6}); err != nil {
		t.Fatal(err)
	}
	dl := filepath.Join(u.StateDir, "update", "AppHubAgente.exe")
	if b, _ := os.ReadFile(dl); string(b) != "MZ-new" {
		t.Fatalf("download %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(u.StateDir, "AppHubAgente.anterior.exe")); string(b) != "MZ-old" {
		t.Fatalf("backup %q", b)
	}
	if rec.n() != 1 || rec.calls[0][0] != dl || rec.calls[0][1] != "--update" {
		t.Fatalf("start %v", rec.calls)
	}
	// Handed over: no app may open until this agent is closed.
	if u.Runner.mu.TryLock() {
		t.Fatal("runner released after a successful hand-over")
	}
}

// Review Focus 5: a cut-short or different download never starts.
func TestUpdaterRejectsBadDownload(t *testing.T) {
	for name, p := range map[string]agentproto.UpdatePayload{
		"short":    {SHA256: digest("MZ-new-and-more"), Size: 15},
		"tampered": {SHA256: digest("MZ-evil"), Size: 6},
	} {
		t.Run(name, func(t *testing.T) {
			u, rec := newUpdater(t, binaryHub(t, "MZ-new"))
			if err := u.Apply(context.Background(), p); !errors.Is(err, ErrBadDownload) {
				t.Fatalf("got %v", err)
			}
			if _, err := os.Stat(filepath.Join(u.StateDir, "update", "AppHubAgente.exe")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("download left behind: %v", err)
			}
			if rec.n() != 0 {
				t.Fatal("started a bad download")
			}
		})
	}
}

func TestUpdaterWaitsForOpenApp(t *testing.T) {
	u, rec := newUpdater(t, binaryHub(t, "MZ-new"))
	u.Runner.mu.Lock() // an app is open
	done := make(chan error, 1)
	go func() { done <- u.Apply(context.Background(), agentproto.UpdatePayload{SHA256: digest("MZ-new"), Size: 6}) }()
	time.Sleep(100 * time.Millisecond)
	if rec.n() != 0 {
		t.Fatal("updated while an app was open")
	}
	u.Runner.mu.Unlock()
	if err := <-done; err != nil || rec.n() != 1 {
		t.Fatalf("err %v starts %d", err, rec.n())
	}
}

// Review Focus 4: Rodar during an update says so.
func TestRunDuringUpdate(t *testing.T) {
	r := &Runner{StateDir: t.TempDir()}
	release := r.Hold()
	defer release()
	res := r.Run(context.Background(), agentproto.RunPayload{}, func(agentproto.Result) {})
	if res.Event != agentproto.EventFailed || !strings.Contains(res.Error, "se atualizando") {
		t.Fatalf("%+v", res)
	}
}

func TestAgentReportsFailedUpdate(t *testing.T) {
	hub := &fakeHub{results: map[int64][]agentproto.Result{}, uploads: map[int64]string{}}
	hub.jobs = []agentproto.Job{{ID: 9, Kind: agentproto.KindUpdate, Payload: payload(agentproto.UpdatePayload{SHA256: "x", Size: 1})}}
	srv := httptest.NewServer(hub.handler())
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}
	u, _ := newUpdater(t, client)
	a := &Agent{Client: client, Runner: u.Runner, StateDir: u.StateDir, Updater: u}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Loop(ctx)
	waitFor(t, func() bool { hub.mu.Lock(); defer hub.mu.Unlock(); return len(hub.results[9]) == 1 })
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if r := hub.results[9][0]; r.Event != agentproto.EventFailed || r.Error != "O arquivo baixado veio diferente do esperado." {
		t.Fatalf("%+v", r)
	}
}
```

The `fakeHub` has no `/api/agent/binary` route, so the download in `TestAgentReportsFailedUpdate` gets a 404 (not ErrBadDownload). Add this route to `fakeHub.handler()` in `agent_test.go` so the test exercises the mismatch path:

```go
	mux.HandleFunc("GET /api/agent/binary", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("MZ-new"))
	})
```

- [ ] **Step 2: Run** `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/agent -count=1'`. Expected: compile errors.

- [ ] **Step 3: Implement.** `client.go`:

```go
// DownloadBinary streams the Hub's agent binary into w. No timeout: it is a
// multi-MB file.
func (c *Client) DownloadBinary(ctx context.Context, w io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/api/agent/binary", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return 0, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("hub GET /api/agent/binary: %d", resp.StatusCode)
	}
	return io.Copy(w, resp.Body)
}
```

`runner.go`: add `updating atomic.Bool` (import `sync/atomic`) to `Runner`, change the TryLock branch and add `Hold`:

```go
type Runner struct {
	Launcher Launcher
	StateDir string
	mu       sync.Mutex
	updating atomic.Bool
}
```

```go
	if !r.mu.TryLock() {
		if r.updating.Load() {
			return failed("O Agente está se atualizando. Tente de novo em um minuto.")
		}
		return failed("Já tem um app aberto pelo Hub neste computador. Feche-o e tente de novo.")
	}
```

```go
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
```

`update.go`:

```go
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

// ErrBadDownload: the file from the Hub does not match the update job.
var ErrBadDownload = errors.New("downloaded agent does not match the update job")

// Updater replaces this agent with the version the Hub sends.
type Updater struct {
	Client     *Client
	Runner     *Runner
	StateDir   string
	CurrentExe string                                // the running agent
	Start      func(exe string, args ...string) error // opens a program without waiting
}

// Apply waits until no app is open, downloads and checks the new agent, keeps
// a copy of the current one and starts the new one in update mode. On success
// the new agent closes this one (via /quit), so Apply's caller never reports.
// After a successful start the runner stays held: no app may open in the
// seconds before this agent is closed.
func (u *Updater) Apply(ctx context.Context, p agentproto.UpdatePayload) error {
	release := u.Runner.Hold()
	err := u.handOver(ctx, p)
	if err != nil {
		release()
	}
	return err
}

func (u *Updater) handOver(ctx context.Context, p agentproto.UpdatePayload) error {
	dir := filepath.Join(u.StateDir, "update")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dl := filepath.Join(dir, "AppHubAgente.exe")
	if err := u.download(ctx, dl, p); err != nil {
		_ = os.Remove(dl)
		return err
	}
	if err := replaceFile(u.CurrentExe, filepath.Join(u.StateDir, "AppHubAgente.anterior.exe")); err != nil {
		return fmt.Errorf("back up current agent: %w", err)
	}
	return u.Start(dl, "--update")
}

func (u *Updater) download(ctx context.Context, dst string, p agentproto.UpdatePayload) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	sum := sha256.New()
	n, err := u.Client.DownloadBinary(ctx, io.MultiWriter(f, sum))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n != p.Size || hex.EncodeToString(sum.Sum(nil)) != p.SHA256 {
		return ErrBadDownload
	}
	return nil
}

// replaceFile copies src over dst through a temporary file, so dst is either
// the old or the new content, never half of each.
func replaceFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
```

`agent.go`: add the field `Updater *Updater` to `Agent`, and before `default:` in `handle`:

```go
	case agentproto.KindUpdate:
		var p agentproto.UpdatePayload
		if err := json.Unmarshal(job.Payload, &p); err != nil || a.Updater == nil {
			a.report(ctx, job.ID, failed("Este Agente não consegue se atualizar sozinho."))
			return
		}
		if err := a.Updater.Apply(ctx, p); err != nil {
			slog.Error("update", "version", p.Version, "err", err)
			msg := "Não consegui atualizar o Agente."
			if errors.Is(err, ErrBadDownload) {
				msg = "O arquivo baixado veio diferente do esperado."
			}
			a.report(ctx, job.ID, failed(msg))
		}
```

- [ ] **Step 4: Run** the agent package tests. Expected: PASS (including the older tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/agent
git commit -m "Agent: download, verify and hand over to a new version"
```

---

### Task 4: Swap with rollback, and the local status check

**Files:**
- Create: `backend/internal/agent/swap.go`
- Test: `backend/internal/agent/swap_test.go`

**Interfaces:**
- Consumes: `replaceFile` (Task 3).
- Produces: `type Swap struct { Self, Target, Backup string; QuitRunning func(); Start func(exe string) (stop func(), err error); Connected func(ctx context.Context) bool; Copy func(src, dst string) error; Wait, Poll, RetryWait time.Duration }`; `func (s *Swap) Run(ctx context.Context) error`; `var ErrRolledBack`; `func StatusConnected(ctx context.Context, url, want string) bool`.

- [ ] **Step 1: Failing tests** `swap_test.go`

```go
package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type swapEnv struct {
	s       *Swap
	starts  int
	stopped int
}

func newSwap(t *testing.T, connectAfter int) *swapEnv {
	t.Helper()
	dir := t.TempDir()
	e := &swapEnv{}
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte(body), 0o755)
		return p
	}
	polls := 0
	e.s = &Swap{
		Self: write("novo.exe", "MZ-new"), Target: write("AppHubAgente.exe", "MZ-old"), Backup: write("AppHubAgente.anterior.exe", "MZ-old"),
		QuitRunning: func() {},
		Start: func(string) (func(), error) {
			e.starts++
			return func() { e.stopped++ }, nil
		},
		Connected: func(context.Context) bool {
			polls++
			return connectAfter >= 0 && polls > connectAfter
		},
		Wait: 200 * time.Millisecond, Poll: 10 * time.Millisecond, RetryWait: time.Millisecond,
	}
	return e
}

func content(t *testing.T, p string) string { b, _ := os.ReadFile(p); return string(b) }

func TestSwapInstallsNewVersion(t *testing.T) {
	e := newSwap(t, 2)
	if err := e.s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if content(t, e.s.Target) != "MZ-new" || e.starts != 1 || e.stopped != 0 {
		t.Fatalf("target %q starts %d stopped %d", content(t, e.s.Target), e.starts, e.stopped)
	}
}

func TestSwapRollsBack(t *testing.T) {
	e := newSwap(t, -1) // never connects
	if err := e.s.Run(context.Background()); !errors.Is(err, ErrRolledBack) {
		t.Fatalf("got %v", err)
	}
	if content(t, e.s.Target) != "MZ-old" || e.starts != 2 || e.stopped != 1 {
		t.Fatalf("target %q starts %d stopped %d", content(t, e.s.Target), e.starts, e.stopped)
	}
}

// Review Focus 2: Windows holds the old exe for a moment after /quit.
func TestSwapRetriesLockedFile(t *testing.T) {
	e := newSwap(t, 0)
	fails := 2
	e.s.Copy = func(src, dst string) error {
		if fails > 0 {
			fails--
			return fmt.Errorf("file in use")
		}
		return replaceFile(src, dst)
	}
	if err := e.s.Run(context.Background()); err != nil || content(t, e.s.Target) != "MZ-new" {
		t.Fatalf("err %v target %q", err, content(t, e.s.Target))
	}
}

func TestSwapCopyNeverWorksRestartsOld(t *testing.T) {
	e := newSwap(t, 0)
	e.s.Copy = func(string, string) error { return fmt.Errorf("file in use") }
	if err := e.s.Run(context.Background()); err == nil {
		t.Fatal("want error")
	}
	if e.starts != 1 || content(t, e.s.Target) != "MZ-old" {
		t.Fatalf("old agent not restarted: starts %d target %q", e.starts, content(t, e.s.Target))
	}
}

// Review Focus 3: the old agent may still answer /status.
func TestStatusConnectedChecksVersion(t *testing.T) {
	version := "0.2.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":%q,"connected":true}`, version)
	}))
	defer srv.Close()
	if StatusConnected(context.Background(), srv.URL, "0.2.1") {
		t.Fatal("old version counted as connected")
	}
	version = "0.2.1"
	if !StatusConnected(context.Background(), srv.URL, "0.2.1") {
		t.Fatal("new version not seen")
	}
	if StatusConnected(context.Background(), "http://127.0.0.1:1", "0.2.1") {
		t.Fatal("nothing listening counted as connected")
	}
}
```

- [ ] **Step 2: Run** the agent package tests. Expected: compile errors.

- [ ] **Step 3: Implement** `swap.go`

```go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// ErrRolledBack: the new agent did not reach the Hub in time; the previous
// one was restored.
var ErrRolledBack = errors.New("new agent did not connect; previous one restored")

// Swap runs in the downloaded agent (update mode). It replaces the installed
// agent with itself and restores the previous copy if the new one does not
// reach the Hub within Wait.
type Swap struct {
	Self, Target, Backup string
	QuitRunning          func()                                  // asks the installed agent to close
	Start                func(exe string) (stop func(), err error) // opens a program without waiting
	Connected            func(ctx context.Context) bool          // installed agent reached the Hub with this version
	Copy                 func(src, dst string) error             // default replaceFile
	Wait, Poll, RetryWait time.Duration                          // defaults 2 min, 2 s, 1 s
}

func (s *Swap) Run(ctx context.Context) error {
	if s.Copy == nil {
		s.Copy = replaceFile
	}
	if s.Wait == 0 {
		s.Wait = 2 * time.Minute
	}
	if s.Poll == 0 {
		s.Poll = 2 * time.Second
	}
	if s.RetryWait == 0 {
		s.RetryWait = time.Second
	}
	s.QuitRunning()
	if err := s.copyRetrying(s.Self, s.Target); err != nil {
		// The old agent already quit: bring it back rather than leave none.
		if _, serr := s.Start(s.Target); serr != nil {
			slog.Error("restart previous agent", "err", serr)
		}
		return fmt.Errorf("install new agent: %w", err)
	}
	stop, err := s.Start(s.Target)
	if err == nil && s.waitConnected(ctx) {
		return nil
	}
	slog.Warn("new agent did not connect; rolling back", "start_err", err)
	if stop != nil {
		stop()
	}
	if err := s.copyRetrying(s.Backup, s.Target); err != nil {
		return fmt.Errorf("restore previous agent: %w", err)
	}
	if _, err := s.Start(s.Target); err != nil {
		return fmt.Errorf("start previous agent: %w", err)
	}
	return ErrRolledBack
}

func (s *Swap) copyRetrying(src, dst string) error {
	var err error
	for range 5 {
		if err = s.Copy(src, dst); err == nil {
			return nil
		}
		time.Sleep(s.RetryWait)
	}
	return err
}

func (s *Swap) waitConnected(ctx context.Context) bool {
	deadline := time.Now().Add(s.Wait)
	for time.Now().Before(deadline) {
		if s.Connected(ctx) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(s.Poll):
		}
	}
	return false
}

// StatusConnected asks an agent's local /status whether it reached the Hub
// and runs version want.
func StatusConnected(ctx context.Context, url, want string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var st struct {
		Version   string `json:"version"`
		Connected bool   `json:"connected"`
	}
	if json.NewDecoder(resp.Body).Decode(&st) != nil {
		return false
	}
	return st.Connected && st.Version == want
}
```

- [ ] **Step 4: Run** the agent package tests. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/agent
git commit -m "Agent: swap in the new version and roll back if it does not connect"
```

---

### Task 5: Wire the Agente program (0.2.0)

**Files:**
- Modify: `backend/cmd/agent/main.go`
- Modify: `backend/internal/agent/client.go` (`Hello` sends `CanUpdate: true`)
- Modify: `backend/internal/agentproto/proto.go` (`Version = "0.2.0"`)
- Modify: `scripts/build-agent.sh` (release note comment)

**Interfaces:**
- Consumes: `Updater`, `Swap`, `StatusConnected`, `ErrRolledBack` (Tasks 3–4).
- Produces: `--update` mode; local `GET http://127.0.0.1:47631/status` → `{"version": "...", "connected": bool}`.

- [ ] **Step 1: `client.go` Hello**

```go
func (c *Client) Hello(ctx context.Context, name string) error {
	_, err := c.postJSON(ctx, "/api/agent/hello", agentproto.Hello{Name: name, Version: agentproto.Version, CanUpdate: true}, nil)
	return err
}
```

- [ ] **Step 2: `proto.go`** → `const Version = "0.2.0"`.

- [ ] **Step 3: `main.go`.** Add imports `context` (already), `encoding/json`, `os/exec`, `path/filepath`, `sync/atomic`, `agentproto`. At package level:

```go
// connected turns true once this agent reached the Hub; /status reports it so
// an updating agent knows the new version works.
var connected atomic.Bool
```

At the top of `main()`, right after the log file setup and **before** `install(dir)`:

```go
	if len(os.Args) > 1 && os.Args[1] == "--update" {
		runUpdate(dir)
		return
	}
```

In the lock-server handler, before `http.NotFound`:

```go
		if r.Method == http.MethodGet && r.URL.Path == "/status" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"version": agentproto.Version, "connected": connected.Load()})
			return
		}
```

After the `Hello` retry loop succeeds (right after it, before building `a`): `connected.Store(true)`. Build the agent with an updater:

```go
	exe, _ := os.Executable()
	runner := &agent.Runner{Launcher: agent.ExecLauncher{}, StateDir: dir}
	a := &agent.Agent{
		Client: client, StateDir: dir, Name: name, ChooseFolder: chooseFolder, Runner: runner,
		Updater: &agent.Updater{Client: client, Runner: runner, StateDir: dir, CurrentExe: exe,
			Start: func(p string, args ...string) error { _, err := startProcess(p, args...); return err }},
	}
```

And at the end of the file:

```go
// runUpdate is update mode: this is the downloaded agent, which installs
// itself over the current one and rolls back if it does not connect.
func runUpdate(dir string) {
	self, err := os.Executable()
	if err != nil {
		slog.Error("update: executable", "err", err)
		return
	}
	s := &agent.Swap{
		Self: self, Target: filepath.Join(dir, "AppHubAgente.exe"), Backup: filepath.Join(dir, "AppHubAgente.anterior.exe"),
		QuitRunning: quitRunning,
		Start:       func(exe string) (func(), error) { return startProcess(exe) },
		Connected: func(ctx context.Context) bool {
			return agent.StatusConnected(ctx, "http://"+lockAddr+"/status", agentproto.Version)
		},
	}
	switch err := s.Run(context.Background()); {
	case err == nil:
		slog.Info("update done", "version", agentproto.Version)
	case errors.Is(err, agent.ErrRolledBack):
		slog.Warn("update rolled back", "version", agentproto.Version)
	default:
		slog.Error("update", "err", err)
	}
}

// quitRunning asks the installed agent to close and gives Windows a moment to
// release its file.
func quitRunning() {
	if resp, err := http.Post("http://"+lockAddr+"/quit", "", nil); err == nil {
		resp.Body.Close()
		time.Sleep(2 * time.Second)
	}
}

// startProcess opens a program without waiting; stop closes it (politely,
// then by force).
func startProcess(exe string, args ...string) (func(), error) {
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _ = cmd.Wait() }()
	return func() {
		quitRunning()
		_ = cmd.Process.Kill()
	}, nil
}
```

- [ ] **Step 4: `scripts/build-agent.sh`** — add under the header comment:

```bash
# Releasing a new agent: bump agentproto.Version, run this script, then restart
# the Hub. Connected agents (0.2.0+) update themselves; same version = no update.
```

- [ ] **Step 5: Verify builds and all tests**

Run:
```bash
bash -c 'ROOT="$(git rev-parse --show-toplevel)" && set -a && . "$ROOT/.env" && set +a && cd "$ROOT/backend" && mise x -- go vet ./... && mise x -- go test ./... -count=1 && GOOS=windows mise x -- go vet ./cmd/agent && bash "$ROOT/scripts/build-agent.sh"'
bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run && mise x -- npx tsc -b'
```
Expected: vet clean, all Go and Vitest tests PASS, `dist/agent/AppHubAgente.exe` built.

- [ ] **Step 6: Commit**

```bash
git add backend scripts/build-agent.sh
git commit -m "Agent 0.2.0: updates itself from the Hub, with rollback"
```

---

### Task 6: Visual check, docs and the real test on Henry's PC

**Files:**
- Modify: `docs/TASKS.md`, `docs/adr/004-agente-do-hub-no-windows.md`, `docs/PRD.md` (3.3 "Próximo"), `Documentação/Especificação do produto v2.md` (the "Próximo passo, atualizar sozinho" bullet and the pilot status table)

- [ ] **Step 1: Visual check.** Start the DB, the Go server (`DEV_MODE=1`) and Vite as in the tech-stack skill. Put a computer in each state with SQL against `app-hub-db`: `UPDATE computers SET ...` and `INSERT INTO agent_jobs ...` with kind `update`. Screenshot `/meu-computador` through the dev login with Playwright (`e2e/`). Check that the spinner line, the green "atualizado" line and the warning line read well in light and dark themes.

- [ ] **Step 2: Real test.** Follow the hands-on rule: one action per question, and check the Hub data after each step.
  1. Restart the Hub with 0.2.0. Henry downloads and opens the Agente from Meu computador (the last manual install). Check: `computers.agent_version = '0.2.0'`, `can_update = true`.
  2. Bump to `0.2.1`, build and restart the Hub. Check: an `update` job reaches `done`, `agent_version = '0.2.1'`, and the page shows "Agente atualizado sozinho em…".
  3. Bump to `0.2.2`, then build with `HUB_URL=http://localhost:9 scripts/build-agent.sh` (this version cannot reach the Hub). Restart the Hub. Check: after about 2 minutes 0.2.1 is back, the job is `failed`, and after 3 tries the page shows "Não conseguimos atualizar…".
  4. Rebuild 0.2.2 with the right URL. It still counts as gave up for 0.2.2, so bump to `0.2.3` to finish the test cleanly. Check: the update goes through.

- [ ] **Step 3: Docs.**
  - TASKS: "Atualização automática do Agente" becomes Done, with the date and a note on how to release.
  - ADR 004 Cons: replace "A atualização automática fica para depois" with "Atualiza sozinho desde a 0.2.0 (spec 2026-09-30); a 0.1.x precisa de uma última instalação manual".
  - PRD 3.3: turn the "Próximo" bullet into present tense.
  - Spec v2: the Agente bullet becomes "Atualiza sozinho: …" and the pilot table gets a row for it.

- [ ] **Step 4: Commit and push**

```bash
git add docs "Documentação"
git commit -m "Docs: agent auto-update done and tested"
git push
```
