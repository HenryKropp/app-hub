# MCP Etapa 2 — Subir apps pelo Claude: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** From Claude Code on the person's PC, "sobe esse programa para o Ziv" registers a script (runs via the Agente) or a screen app (runs on the server as a shared app), or sends a new version of an app whose code the Ziv keeps.

**Architecture:**
- Code never travels inside a tool call. `preparar_envio` creates a one-time upload (`mcp_uploads` row, token hashed) and returns a URL plus the exact `tar`/`curl` commands; Claude Code zips the folder into the temp dir and `curl -X PUT`s it. `registrar_app` opens the zip with a hardened extractor (`internal/bundle`), then calls the same JSON handlers the site uses, in-process through `callAPI` (ADR 021): `POST /api/scripts/import` for a new app, the new `POST /api/apps/{id}/sources` for a new version.
- Every stored-code version is recorded by one helper with `origin='upload'` (or `'original'` for v1) and the new `app_versions.channel='mcp'` when it came through Claude. Restore now rewrites `script_sources` too, so "voltar versão" no longer breaks the next run.
- Screen apps (last tasks): the import accepts `kind: web`; `publicar_app_com_tela` enables sharing with the person's "estou ciente" marks, stops when keys are missing (names + link only), publishes side-by-side via the existing `POST /api/apps/{id}/shared/publish` and waits for the result.

**Tech Stack:** Go stdlib (`archive/zip`, `net/http`), pgx + sqlc, `github.com/modelcontextprotocol/go-sdk/mcp`; React + TS + Vitest.

**Spec:** `docs/superpowers/specs/2026-11-06-mcp-registrar-apps-design.md` (read section 5b — it overrides section 5 where they differ). Also `docs/PROTOCOLO-REGISTRO.md`, `docs/adr/021-mcp-dentro-do-ziv.md`, `docs/adr/010-apps-compartilhados.md`.

**Out of scope (separate plan, written in parallel):** spec §2 item 5, per-company isolation of shared apps. **Dependency:** Tasks 9–10 assume that plan has landed, i.e. every company's `Handler` has a working `h.shared` (`Svc.Test/Publish`, readiness, vault) and `sharedReady` no longer answers `shared_runtime_unavailable` outside "principal". Tasks 1–8 do not depend on it and ship first. Tests for 9–10 use `withShared` (fake engine), so they pass before that plan lands; only the manual test in a non-principal company needs it.

## Global Constraints

- Tool names (final): `formato_do_app` (read-only), `preparar_envio`, `registrar_app`, `publicar_app_com_tela` (the last three `ReadOnlyHint:false, DestructiveHint:false`, like `rodar_app`). Descriptions and messages in simple Brazilian Portuguese; the product is "o Ziv".
- Upload: `PUT <BaseURL>/api/mcp/envios/<token>`, token = `ziv_` style random 32 bytes base64url (use `newToken()`), only its SHA-256 (`hashToken`) stored. Valid 30 minutes from creation, body accepted once, max 12 MiB zip. The upload id (`envio`) is usable by `registrar_app` only by the same person, until it succeeds once or expires. At most 5 open (unexpired, unused) uploads per person.
- Limits are the site's: 1–100 kept files, `manifest.MaxInputBytes` (10 MB) uncompressed total, actual bytes counted (never trust zip headers). Zip central directory may list at most 5000 entries (junk folders included).
- Never uploaded / dropped server-side and reported by name only: `.env`, `.env.*`, `*.pem`, `*.key`, `id_rsa`; folders `.git`, `.venv`, `venv`, `node_modules`, `__pycache__`, `.mypy_cache`, `.pytest_cache`; the manifest (`ziv.yaml`/legacy names), `FICHA.md`, `.apphub-*`. No tool output ever contains a file's content, a secret value, a token, or a vault value.
- Kinds accepted through Claude: `script` and `web` only. `desktop`, `executable` (and an entrypoint ending in `.exe`) are refused with: "Apps de janela e .exe ainda não sobem pelo Claude: eles não rodam no servidor. Registre pelo site, em Registrar apps." `scheduled`: "Registre como script e agende pelo Ziv, na ficha do app." `reference`/`system`: "Esse tipo só é registrado pelo site."
- Only apps with a `script_sources` row accept a new version; others get 409 `no_stored_code`: "Este app foi registrado a partir de uma pasta do computador, e o Ziv não guarda o código dele. Para mandar versões pelo Claude, suba o app de novo como app novo (registrar_app sem app_id)."
- Permissions come from the existing handlers, never from the tools: new app = `access.RegisterApps`; new version = `can.Edit` (same as `PUT /api/apps/{id}`); publish = `can.Restore` (what the Publicar button needs today).
- **A new version may also update the ficha (Henry, 2026-11-06):** only `summary`, `inputs`, `outputs`, and for scripts the run settings `entrypoint` + `runtime` (`language`, `dependencies_file`, `dependencies`). Validation is the site's edit path (`writeApp` via the same rules as `updateApp`, including the `updated_at` guard and the `app_changes` record). **Name, area, owner/backup and access are site-only: `registrar_app` ignores them on an update and says so** ("Não mudei nome/área/responsável: isso só muda pelo site."); the API body for an update has no field for them, so they cannot be sent at all. Claude shows the person the before/after of each changed field (from `ver_app`) before sending.
- **Screen-app links** always come from `h.shared.Svc.AppURL(slug)` (exposed as `url` by `GET /api/apps/{id}/shared` and `sharedBrief.URL`); never build `https://<slug>.zivapp…` by hand — outside principal it is `<app>.<empresa>.zivapp.duckdns.org` (isolation plan).
- **Screen apps can be OFF in a company** (until the Ziv owner turns them on; isolation plan). Off = `h.shared.Svc == nil`, the API answers 503 `code: shared_runtime_unavailable`. Registering or publishing a `web` app there returns: "Os apps com tela ainda não estão ligados nesta empresa. Peça ao responsável pelo Ziv para ligar. Scripts sobem normalmente." If the isolation plan exposes the off state with another code, map that code to the same sentence.
- **Stored code of screen apps lives in `h.shared.Svc.CodeDir(appID)`** (mode 0700), never in `os.TempDir()`: outside principal the deploy service refuses any other folder (`deploy.ErrFolderOutside`).
- Approval: a new version needs approval again automatically (`executionBlock` compares `reviewed_version`). Tools say "esperando a aprovação de um administrador" when `blocked_reason` is that text.
- Version history: `app_versions.origin` gains `'upload'`; new column `app_versions.channel TEXT NOT NULL DEFAULT 'manual' CHECK (channel IN ('manual','mcp'))`, set from `originOf(ctx)` only in the stored-code helper. UI: "Código enviado" + the existing `ByClaude` badge ("pelo Claude").
- Migration file: `backend/internal/database/migrations/021_mcp_registrar.sql` (migrations are tracked by file name; if the isolation plan also adds a `021_*`, both apply in name order — no renumbering needed unless they touch the same table).
- sqlc: after `mise x -- sqlc generate` (from `backend/`) always run `git checkout internal/database/sqlc/people.sql.go`. `internal/database/sqlc/script_sources.go` is hand-written; edit it directly.
- Backend tests: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./...`. Frontend: `cd frontend && mise x -- npx vitest run && mise x -- npx tsc -b`.
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Zip from PowerShell `Compress-Archive`** uses `\` separators and may wrap everything in one top folder: the extractor must normalise `\`→`/`, strip `./`, and when the entrypoint is missing say how to zip from inside the folder (test in Task 2: `TestExtractBackslashNames`; Task 7: `TestRegistrarMissingEntrypointHint`).
2. **Secret-confirmation retry:** `registrar_app` returns "possíveis senhas", the person confirms, Claude calls again with the same `envio` — the upload must still be usable (consumed only on success). Test in Task 7: `TestRegistrarSecretsRetrySameEnvio`.
3. **Sending identical code** as a "new version": no empty version, a clear "nada mudou" answer (Task 4: `TestSourcesUnchangedNoVersion`).
4. **Update while a run or a publish is in progress:** refuse instead of rewriting the folder under them (Task 4: `TestSourcesRefusedDuringRun`; Task 9: `TestSourcesRefusedWhileDeployBusy`).
5. **Restart of the server** (temp folder gone) before publishing a screen app or after a version upload: folder rebuilt from `script_sources` before readiness/publish (Task 5: `TestHydrateWebAppFolder`; Task 9: `TestPublishAfterFolderLost`).

---

## File map

| File | Responsibility |
|---|---|
| `backend/internal/database/migrations/021_mcp_registrar.sql` (new) | origin `upload`, `channel` column, `mcp_uploads` table |
| `backend/internal/database/queries/versions.sql`, `queries/mcp.sql` | channel/description on insert, list selects channel; upload queries |
| `backend/internal/database/sqlc/script_sources.go` | `ImportScript` takes kind; new `ReplaceScriptSources` |
| `backend/internal/bundle/bundle.go` (new) | safe zip extraction, pure, no DB |
| `backend/internal/handler/stored_code.go` (new) | `checkSentFiles`, `recordSentVersion`, `syncSourcesFromFolder`, `POST /api/apps/{id}/sources` |
| `backend/internal/handler/script_import.go` | uses the helpers; `kind` script\|web |
| `backend/internal/handler/restore.go`, `versions.go` | sync sources after restore; hydrate any stored-code app; channel param |
| `backend/internal/handler/mcp_envio.go` (new) | upload endpoint + `preparar_envio` |
| `backend/internal/handler/mcp_registrar.go` (new) | `formato_do_app`, `registrar_app`, `publicar_app_com_tela` |
| `backend/internal/handler/mcp_protocolo.md` (new, embedded) | MCP instructions adapted from `docs/PROTOCOLO-REGISTRO.md` |
| `backend/internal/handler/mcp_server.go`, `mcp_tools.go`, `security_middleware.go`, `apps.go`, `shared.go` | routes allow-list, instructions, tool registration, upload exemption, `stored_code`, hydrate/approval in shared |
| `frontend/src/lib/api.ts`, `components/HistoryTimeline.tsx` | `upload` origin, `channel` |
| `docs/adr/025-registrar-apps-pelo-claude.md` (new), `docs/PROTOCOLO-REGISTRO.md` | decision record; protocol v0.3 |

---

### Task 1: Schema — "via Claude" on versions, upload table

**Files:**
- Create: `backend/internal/database/migrations/021_mcp_registrar.sql`
- Modify: `backend/internal/database/queries/versions.sql`, `backend/internal/database/queries/mcp.sql`, `backend/internal/handler/versions.go:117-145`, `backend/internal/handler/restore.go` (insertVersion call), `backend/internal/handler/history.go:19-72,136`, `backend/internal/handler/versions.go:210`, `backend/internal/handler/handler_test.go:54` (add `mcp_uploads` to `resetSQL`)
- Test: `backend/internal/handler/versions_test.go`

**Interfaces:**
- Produces:
  - SQL: `app_versions.origin IN ('original','folder','restore','upload')`; `app_versions.channel` (see Global Constraints); table `mcp_uploads(id BIGSERIAL PK, token_hash TEXT NOT NULL UNIQUE, person_id BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), expires_at TIMESTAMPTZ NOT NULL, received_at TIMESTAMPTZ, used_at TIMESTAMPTZ, size INT NOT NULL DEFAULT 0, content BYTEA)`.
  - `InsertAppVersion` gains `channel` (`sqlc.arg`) and `description` (`sqlc.arg`, default `''` from callers). `ListVersionsBefore` selects `v.channel`.
  - Queries in `mcp.sql`: `CreateMCPUpload(token_hash, person_id, expires_at) :one → id`; `CountOpenMCPUploads(person_id) :one int64` (expires_at > now() AND used_at IS NULL); `ReceiveMCPUpload(token_hash, content, size) :one → id` (`WHERE token_hash=$1 AND received_at IS NULL AND expires_at > now()`); `GetMCPUpload(id, person_id) :one` (all columns); `UseMCPUpload(id) :exec` (`used_at=now(), content=NULL`); `DeleteExpiredMCPUploads :exec` (`expires_at < now() - interval '1 day' OR used_at < now() - interval '1 day'`).
  - `func (h *Handler) insertVersion(ctx, a sqlc.App, prev, snap versions.Snapshot, contents map[string][]byte, origin string, restoredFrom pgtype.Int4, author pgtype.Int8, probable bool, channel, description string) (sqlc.AppVersion, error)` — existing callers pass `"manual", ""`.
  - `versionItem.Channel string \`json:"channel"\``; `newVersionItem(..., channel string, current int32)` (channel inserted before `current`).

- [ ] **Step 1: Write the failing test** `TestVersionChannelDefaultsManual` in `versions_test.go`: import a script with `importTestScript`, `GET /api/apps/{id}/history` (whatever route the existing history test uses), assert the version item has `"channel": "manual"` and `"origin": "original"`; then `testPool.Exec("UPDATE app_versions SET origin='upload', channel='mcp' WHERE app_id=$1", id)` must succeed (constraint accepts both).
- [ ] **Step 2: Run** `go test ./internal/handler -run TestVersionChannelDefaultsManual` → FAIL (column `channel` does not exist).
- [ ] **Step 3: Write the migration** (drop `app_versions_origin_check`, re-add with `'upload'`; add `channel`; create `mcp_uploads` with index on `person_id`), the query changes, run `mise x -- sqlc generate` then `git checkout internal/database/sqlc/people.sql.go`, and thread `channel`/`description` through `insertVersion` and `newVersionItem`.
- [ ] **Step 4: Run** the test and the whole handler package → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: versões marcam o canal (pelo Claude) e tabela de envios`.

---

### Task 2: Safe zip extractor (`internal/bundle`)

**Files:**
- Create: `backend/internal/bundle/bundle.go`, `backend/internal/bundle/bundle_test.go`

**Interfaces:**
- Produces:
  ```go
  type Limits struct{ MaxZipBytes, MaxEntries, MaxFiles, MaxTotal int }
  var SiteLimits = Limits{MaxZipBytes: 12 << 20, MaxEntries: 5000, MaxFiles: 100, MaxTotal: manifest.MaxInputBytes}
  type Result struct {
      Files   map[string][]byte // slash paths, relative, kept
      Dropped []string          // paths left out (secret files, junk folders shown once as "node_modules/"), sorted, max 20
  }
  func Extract(data []byte, lim Limits) (Result, error) // errors are Portuguese sentences ending in "."
  func IsSecretFile(base string) bool                  // .env, .env.*, *.pem, *.key, id_rsa (case-insensitive)
  ```
  Move the secret-file test now inline in `script_import.go:73` to `bundle.IsSecretFile` (Task 3 uses it).
  Error texts: not a zip "O envio não é um arquivo .zip válido."; unsafe name/symlink/encrypted "O pacote tem um caminho inseguro: <nome>." (or "…um atalho…", "…um arquivo com senha…"); duplicate "O pacote tem dois arquivos com o mesmo nome: <nome>."; count (0 or >MaxFiles kept, or >MaxEntries listed) "Envie de 1 a 100 arquivos do app. Deixe de fora dados e saídas acumuladas."; size "Os arquivos podem somar no máximo 10 MB."
- Rules (in order, per entry): normalise `\`→`/`, strip leading `./`; skip directories; refuse symlinks (`Mode()&fs.ModeSymlink`), encrypted entries (`Flags&1`), names failing `manifest.SafeRelative` (absolute, `..`, drive letters); drop junk-folder segments and secret files into `Dropped`; skip manifest names/`FICHA.md`/`.apphub-*` silently; refuse case-insensitive duplicates; read each kept file through `io.LimitReader(remaining+1)` against `MaxTotal`. `zip.NewReader` returning `zip.ErrInsecurePath` is a refusal, not a crash.

- [ ] **Step 1: Write the failing tests** (build zips in-memory with `archive/zip`):
  - `TestExtractKeepsCodeDropsJunk`: entries `main.py`, `lib/util.py`, `.env`, `.git/config`, `node_modules/x/i.js`, `ziv.yaml` → `Files` keys `{main.py, lib/util.py}`; `Dropped == [".env", ".git/", "node_modules/"]`.
  - `TestExtractRejectsZipSlip`: entry `../evil.py` → error containing "caminho"; entry `/etc/passwd` → error.
  - `TestExtractRejectsSymlink`: header with `SetMode(fs.ModeSymlink|0777)` → error.
  - `TestExtractZipBombCountsRealBytes`: one entry of 11 MB zeros (deflates to ~10 KB) with `MaxTotal` 10 MB → error "Os arquivos podem somar no máximo 10 MB."; also a header lying `UncompressedSize64=1` is still caught (write with `CreateRaw` + mismatched size or rely on real-bytes reading).
  - `TestExtractTooManyFiles`: 101 kept files → error mentioning "100 arquivos"; 5001 entries → error.
  - `TestExtractBackslashNames`: entry `src\\app.py` → key `src/app.py`; `./main.py` → `main.py`.
  - `TestExtractDuplicateCase`: `Main.py` and `main.py` → error.
  - `TestExtractNotAZip`: `[]byte("hello")` → error "O envio não é um arquivo .zip válido."
- [ ] **Step 2: Run** `go test ./internal/bundle` → FAIL (package missing).
- [ ] **Step 3: Implement** `Extract` and `IsSecretFile`.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: abrir o zip do envio com segurança`.

---

### Task 3: One validation path and one version recorder for stored code

**Files:**
- Create: `backend/internal/handler/stored_code.go`
- Modify: `backend/internal/handler/script_import.go`, `backend/internal/database/sqlc/script_sources.go`
- Test: `backend/internal/handler/stored_code_test.go`

**Interfaces:**
- Consumes: Task 1 `insertVersion(..., channel, description)`; Task 2 `bundle.IsSecretFile`.
- Produces:
  ```go
  // checkSentFiles is lines 53-120 of today's importScript, unchanged in behaviour and messages,
  // for a manifest whose Entrypoint/Runtime.DependenciesFile are final. On refusal it has answered.
  func (h *Handler) checkSentFiles(w http.ResponseWriter, r *http.Request, m manifest.Manifest,
      files map[string]manifest.InputFile, confirm bool) (map[string][]byte, bool)
  // recordSentVersion scans a.FolderPath and records it: origin "original" when the app has no
  // version, else "upload"; channel originOf(ctx); author = by (probable=false); description = note.
  // changed=false (and the latest version) when the snapshot equals the latest. Caller holds lockApp.
  func (h *Handler) recordSentVersion(ctx context.Context, a sqlc.App, by int64, note string) (v sqlc.AppVersion, changed bool, err error)
  func (q *Queries) ImportScript(ctx context.Context, p UpsertAppParams, kind string, files []byte) (int64, error) // kind replaces the literal 'script'
  func (q *Queries) ReplaceScriptSources(ctx context.Context, appID int64, files []byte) error
  ```
  `importScript` keeps forcing `kind=script` in this task (web comes in Task 9), calls `checkSentFiles`, and replaces `h.checkVersion` with `lockApp` + `recordSentVersion(ctx, a, me.ID, "")`. Its response becomes `{"id", "version", "blocked_reason"}` where `blocked_reason` is `executionBlock(ctx,id)` ("" when free). The 409/422 secret bodies stay byte-identical (the site and Task 7 parse `code`, `secret_count`).

- [ ] **Step 1: Write the failing tests:**
  - `TestImportViaClaudeMarksChannel`: call `h.callAPI(withMCPPerson(ctx, adminID), "POST", "/api/scripts/import", body)` after adding the route to `mcpRoutes` in this task (`"POST /api/scripts/import"`); assert the v1 row has `origin='original'`, `channel='mcp'`, `author_id=adminID`. Same import through `c.do` → `channel='manual'`.
  - `TestImportAnswersBlockedReason`: with `h.SetWorkspace(...)` and `require_app_approval=true`, the response `blocked_reason == "Este app está esperando a aprovação de um administrador."`.
  - Existing `TestScriptImport*` tests still pass unchanged (regression).
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** (extract, don't rewrite: same order of checks, same messages).
- [ ] **Step 4: Run** the handler package → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: validação e registro de versão únicos para código guardado`.

---

### Task 4: New version of a stored-code app — `POST /api/apps/{id}/sources`

**Files:**
- Modify: `backend/internal/handler/stored_code.go`, `backend/internal/handler/handler.go` (route next to `/api/scripts/import`), `backend/internal/handler/apps.go:182-204` (`StoredCode bool \`json:"stored_code"\`` in `appDetail`, filled in `getApp`/`writeDetailWith` from `h.q.ScriptSources` ≠ `ErrNoRows`), `backend/internal/handler/mcp_server.go` (`"POST /api/apps/{id}/sources"` in `mcpRoutes`)
- Test: `backend/internal/handler/stored_code_test.go`

**Interfaces:**
- Consumes: Task 3 helpers, `ReplaceScriptSources`, `hydrateImportedScript`.
- Produces: `func (h *Handler) sendSources(w, r)`. Body `{files map[string]manifest.InputFile, confirm_secrets bool, note string (≤2000 runes)}` read with `MaxBytesReader(15<<20)` and `DisallowUnknownFields`. Order: `appFor` (404 if not visible) → `!can.Edit` 403 "Você não pode mudar este app." → no sources row 409 `{"code":"no_stored_code","error":<Global Constraints text>}` → `lockApp` → `ExpireStaleRuns` + `AppHasActiveRun` 409 "Espere a execução terminar antes de mandar uma versão nova." → `checkSentFiles` with the app's current manifest → `ReplaceScriptSources` → `os.RemoveAll(a.FolderPath)` → `hydrateImportedScript` (rebuilds folder = sources + manifest) → `recordSentVersion(ctx, a, me.ID, note)`. Answer 200 `{"id", "version", "changed", "blocked_reason"}`.
  **Ficha update (optional):** body also takes `ficha *fichaUpdate` and `updated_at time.Time` (required when `ficha` is set):
  ```go
  type fichaUpdate struct { // the only ficha fields a code upload may change
      Summary    *string            `json:"summary"`
      Inputs     *[]manifest.Input  `json:"inputs"`
      Outputs    *[]manifest.Output `json:"outputs"`
      Entrypoint *string            `json:"entrypoint"` // scripts only; ignored for web
      Runtime    *manifest.Runtime  `json:"runtime"`    // scripts only; ignored for web
  }
  func applyFicha(m manifest.Manifest, f *fichaUpdate) manifest.Manifest // nil fields keep m's value
  ```
  With `ficha`: `updated_at` ≠ `a.UpdatedAt` → 409 with `updateApp`'s exact text; `m := applyFicha(current, ficha)`; trim + 2000-char summary limit as import; `m.Validate()` issues → 400 joined **before** any code is touched; `checkSentFiles` uses the new `m` (entrypoint/deps file must be in the package); after `hydrateImportedScript`, call `h.writeApp(w, r, me, a.FolderPath, appBody{FolderPath: a.FolderPath, OwnerID: a.OwnerID, BackupID: <a.BackupID>, Manifest: m}, &a)` (same validation, manifest file, `app_changes` row as the site), then `recordSentVersion`. Answer adds `"updated_at"` of the saved app.

- [ ] **Step 1: Write the failing tests:**
  - `TestSourcesNewVersion`: import (v1), send `main.py` = `print('v2')` with note "Corrige a soma" → 200, `version==2`, `changed==true`; row v2 `origin='upload'`, `description='Corrige a soma'`; `ScriptSources` holds v2; a run (`POST /api/apps/{id}/runs` with a paired computer, as in `TestScriptImportQueueValidationAndCleanup`) is queued, not 409 `code_changed`.
  - `TestSourcesRemovedFileDisappears`: v1 has `main.py`,`old.py`; v2 sends only `main.py` → `old.py` absent from the folder and from sources.
  - `TestSourcesUnchangedNoVersion`: send v1's files again → `changed==false`, `version==1`, no new row.
  - `TestSourcesFolderAppRefused`: an app from `insertApp` (no sources) → 409 `no_stored_code`.
  - `TestSourcesNeedsEdit`: a `usuario` who can see but not edit → 403.
  - `TestSourcesRefusedDuringRun`: with a `waiting` run → 409, sources unchanged.
  - `TestSourcesSecretsNeedConfirm`: file with `AKIA...` style key → 409 `script_secrets_confirmation_required`; same with `confirm_secrets:true` → 200.
  - `TestSourcesWaitsApproval`: workspace with approval; app approved at v1 (`PUT /api/security/apps/{id}` as the existing security tests do); after v2 → `blocked_reason` is the approval text.
  - `TestAppDetailStoredCode`: imported app `stored_code==true`; folder app `false`.
  - `TestSourcesWithFicha`: v2 plus `ficha{summary:"Novo resumo", inputs:[+ text input "data"]}` and the current `updated_at` → 200; `GET /api/apps/{id}` shows both; one `app_changes` row for the app lists `summary` and `inputs`; v2 row exists.
  - `TestSourcesFichaInvalidTouchesNothing`: `ficha.inputs` with type `"xyz"` → 400 "Tipo de entrada inválido", sources and folder still v1, no new version.
  - `TestSourcesFichaStale`: `updated_at` older than the app's → 409 "Alguém mudou esta ficha…", nothing changed.
  - `TestSourcesFichaNewEntrypoint`: `ficha.entrypoint:"run.py"` with `run.py` in files → 200 and manifest entrypoint `run.py`; without `run.py` in files → 400 "Inclua o arquivo que inicia o script."
  - `TestSourcesBodyHasNoNameOrArea`: body with `"ficha":{"name":"X"}` or top-level `"area"` → 400 (unknown field); name and area unchanged.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: versão nova de app com código guardado`.

---

### Task 5: Restore keeps stored code in step; hydrate any stored-code app

**Files:**
- Modify: `backend/internal/handler/restore.go` (after `insertVersion` in `restoreVersion`), `backend/internal/handler/versions.go:79-83` (hydrate regardless of kind), `backend/internal/handler/stored_code.go`
- Test: `backend/internal/handler/restore_test.go`

**Interfaces:**
- Produces: `func (h *Handler) syncSourcesFromFolder(ctx context.Context, a sqlc.App, snap versions.Snapshot) error` — no-op when the app has no sources row; otherwise reads every `snap` path from `a.FolderPath` (skipping manifest names, `FICHA.md`, `.apphub-*`), base64-encodes into `map[string]manifest.InputFile{Name: base}` and calls `ReplaceScriptSources`. `checkVersionLocked` calls `hydrateImportedScript` for every kind (it already returns nil without a sources row).

- [ ] **Step 1: Write the failing tests:**
  - `TestRestoreImportedScriptThenRun`: import (v1 `print('a')`), sources v2 (`print('b')`), `POST /api/apps/{id}/versions/1/restore` → 200; `ScriptSources` `main.py` decodes to `print('a')`; queuing a run is accepted (today: 409 `code_changed`).
  - `TestHydrateWebAppFolder`: `testPool.Exec` an app with `kind web` manifest plus a `script_sources` row and a non-existent `folder_path`; `h.checkVersion` creates the folder with the files.
- [ ] **Step 2: Run** → `TestRestoreImportedScriptThenRun` FAIL with `code_changed`.
- [ ] **Step 3: Implement** (on sync error after a successful restore, log and still answer 200 — the restore happened; the next run will say `code_changed` as today).
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `Voltar versão de script importado atualiza o código guardado`.

---

### Task 6: Upload endpoint and `preparar_envio`

**Files:**
- Create: `backend/internal/handler/mcp_envio.go`
- Modify: `backend/internal/handler/handler.go` (route `PUT /api/mcp/envios/{token}`), `backend/internal/handler/security_middleware.go:136` (treat `strings.HasPrefix(r.URL.Path, "/api/mcp/envios/")` like `isClaude`: no Origin/JSON-content-type checks), `backend/internal/handler/mcp_tools.go` (register tool)
- Test: `backend/internal/handler/mcp_envio_test.go`

**Interfaces:**
- Consumes: Task 1 queries; Task 2 `bundle.Extract`, `bundle.SiteLimits`.
- Produces:
  - `func (h *Handler) receiveUpload(w, r)`: `http.MaxBytesReader(SiteLimits.MaxZipBytes)`; empty body 400; over limit 413 "O pacote passa de 12 MB. Deixe de fora dados e saídas acumuladas."; `bundle.Extract` error → 400 with its text (nothing stored); `ReceiveMCPUpload(hashToken(token), body, len)` no row → 404 "Este endereço de envio não vale mais. Peça um novo com preparar_envio." Success 200 `{"recebido": true, "arquivos": N}`. Never logs the token.
  - Tool `preparar_envio`, input `{}` (emptyIn): `DeleteExpiredMCPUploads`; `CountOpenMCPUploads ≥ 5` → toolFail "Você já tem 5 envios abertos. Termine ou espere 30 minutos."; create row (`expires_at = now+30min`). Structured out: `{"envio": id, "endereco": url, "vale_ate": RFC3339, "comando_windows": ..., "comando_mac_linux": ...}`. Exact Windows command text (the person's folder is `<PASTA>`, the zip goes to the temp folder, never the app folder):
    ```
    tar.exe -a -c -f "%TEMP%\ziv-envio-<id>.zip" --exclude=.git --exclude=.venv --exclude=venv --exclude=node_modules --exclude=__pycache__ --exclude=.env --exclude=".env.*" --exclude="*.pem" --exclude="*.key" -C "<PASTA>" .
    curl.exe -sS --fail-with-body -X PUT -H "Content-Type: application/zip" --data-binary "@%TEMP%\ziv-envio-<id>.zip" "<endereco>"
    ```
    Mac/Linux: same with `zip -r -X "$TMPDIR/ziv-envio-<id>.zip" . -x '.git/*' ...` run with `cd "<PASTA>"` in a subshell, then `curl`. Text tells Claude: after curl prints `"recebido": true`, call `registrar_app` with `envio`; delete the temp zip afterwards.
  - Tool description: "Prepara o envio do código de um app para o Ziv. Devolve um endereço de uso único (vale 30 minutos) e os comandos para zipar a pasta (sem .env, .git, venv, node_modules) numa pasta temporária e mandar com curl. Use só depois que a pessoa disse \"pode registrar\"."

- [ ] **Step 1: Write the failing tests:**
  - `TestEnvioRoundTrip`: `preparar_envio` via `toolSession`; PUT a small zip (`main.py`) to `endereco` with `Content-Type: application/zip` through `c.srv` (Secure in the chain) → 200 `arquivos:1`; second PUT → 404; row has `received_at` set, `person_id` = the MCP person.
  - `TestEnvioExpired`: set `expires_at=now()-1s` → PUT 404.
  - `TestEnvioRejectsBadZip`: PUT `"hello"` → 400, row `received_at IS NULL` (the same URL still accepts a good zip after).
  - `TestEnvioTooBig`: 12 MiB + 1 byte → 413.
  - `TestEnvioOpenLimit`: sixth `preparar_envio` → error text contains "5 envios abertos".
  - `TestEnvioCommandsUseTemp`: `comando_windows` contains `%TEMP%` and `-C "` and never writes into `<PASTA>`.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: endereço de envio de uso único`.

---

### Task 7: `formato_do_app`, `registrar_app` (scripts) and the MCP protocol text

**Files:**
- Create: `backend/internal/handler/mcp_registrar.go`, `backend/internal/handler/mcp_protocolo.md`
- Modify: `backend/internal/handler/mcp_tools.go` (register; `ver_app` shows `codigo_guardado` from `appDetail.StoredCode` and the line "Pode receber versão nova pelo Claude (registrar_app com app_id)." or the refusal reason; when `codigo_guardado` and `can.edit`, structured adds `ficha_editavel: {summary, inputs, outputs, entrypoint, runtime, updated_at}` — the full inputs incl. `window` ones — so Claude can show before/after), `backend/internal/handler/mcp_server.go` (`mcpInstructions` + one sentence: "Para registrar ou atualizar um app a partir de uma pasta do computador, chame formato_do_app primeiro e siga o protocolo que ele devolve.")
- Test: `backend/internal/handler/mcp_registrar_test.go`

**Interfaces:**
- Consumes: Task 3 import response `{id, version, blocked_reason}` and secret codes; Task 4 `/sources` response and `no_stored_code`; Task 6 `GetMCPUpload`, `UseMCPUpload`; Task 2 `Extract`.
- Produces:
  - `formato_do_app` (read-only, `emptyIn`): text = `mcpProtocolo` (embedded) + the company's area names (`GET /api/areas`) + limits line. Structured `{"protocolo", "areas": []string, "tipos": ["script","web"], "limites": {"arquivos":100,"mb":10}}`.
  - `mcp_protocolo.md`: `docs/PROTOCOLO-REGISTRO.md` §1, §3 (Passos 0–11, 13), §4, §6, §7, rewritten for this flow: Passo 0 uses `buscar`/`ver_app` (`codigo_guardado`); §2 table replaced by "você lê a pasta no disco, só leitura"; Passo 12 kept (check paths, never run); new section "Conferir e enviar": show the summary (nome, descrição, tipo, o que pede e entrega, área, pontos de atenção, chaves que faltam) → wait for "pode registrar" → `preparar_envio` → run its commands → `registrar_app`; "Vários de uma vez": one summary and one confirmation per app, one `preparar_envio` per app; "Possível senha": ask, then call again with `confirmar_segredos: true`; window apps/.exe: explain and point to the site; keys: never read values, list names. Owner = the person registering (the server forces it), so `owner`/`backup` are not asked. **Versão nova:** read `ver_app` (`ficha_editavel`), show the person, field by field, what changes in resumo, entradas, saídas, arquivo de início and dependências (antes → depois) together with the code summary, and send only after "pode mandar"; nome, área, responsável, reserva e acesso só mudam pelo site — say so if the person asks.
  - `registrar_app` input:
    ```go
    type registrarIn struct {
        Envio             int64             `json:"envio" jsonschema:"o número que preparar_envio devolveu, depois do curl"`
        AppID             int64             `json:"app_id,omitempty" jsonschema:"para mandar versão nova de um app que já existe"`
        Manifesto         manifest.Manifest `json:"manifesto,omitempty" jsonschema:"o manifesto do app, no formato de formato_do_app; obrigatório para app novo; numa versão nova, só resumo, entradas, saídas e (scripts) arquivo de início e dependências mudam"`
        Nota              string            `json:"nota,omitempty" jsonschema:"o que mudou nesta versão"`
        ConfirmarSegredos bool              `json:"confirmar_segredos,omitempty" jsonschema:"true só depois que a pessoa confirmar"`
    }
    ```
    Flow: `GetMCPUpload(envio, me)` no row (incl. another person's) → "Não achei esse envio. Chame preparar_envio de novo."; expired or used → "Esse envio já foi usado ou venceu. Chame preparar_envio de novo."; not received → "O Ziv ainda não recebeu o zip desse envio. Rode o comando curl que preparar_envio mostrou e confira se ele respondeu \"recebido\"."; kind check (Global Constraints texts; `web` → in this task toolFail "Apps com tela ainda não sobem pelo Claude." — replaced in Task 9); `bundle.Extract(content)`; build `files` map; new app → `auditedAPI POST /api/scripts/import {manifest, files, confirm_secrets}`; with `app_id` → read `GET /api/apps/{id}` first; when `manifesto` is non-empty build `ficha` from it (only the `fichaUpdate` fields that differ from the current ficha; `entrypoint`/`runtime` only for scripts) plus that `updated_at`, and list in the answer any of `name`, `area`, `owner`, `backup` that differ as ignored; then `auditedAPI POST /api/apps/{id}/sources {files, confirm_secrets, note, ficha?, updated_at?}`. 400 validation text is returned as-is ("Nada foi mandado."). On success `UseMCPUpload`. Answer text (and structured `app_id, versao, mudou, link, esperando_aprovacao, deixados_de_fora`):
    - new: "Registrei o app \"<nome>\" (versão 1). Link: …"; update: "Mandei a versão <n> do \"<nome>\"." / unchanged: "O código é igual ao da versão <n>; nada mudou."
    - `Dropped` non-empty: "Deixei de fora: .env, … (senhas e pastas que não sobem)." plus, when a secret file was dropped: "Se o app precisa dessas chaves, a pessoa preenche no Ziv; nunca mande os valores pela conversa."
    - `blocked_reason` approval → "Está esperando a aprovação de um administrador; só depois dá para rodar."
    - 409 `script_secrets_confirmation_required` → same wording pattern as `publicar_artifact` ("…só se ela confirmar, chame registrar_app de novo com o mesmo envio e confirmar_segredos: true."); 422 `script_secrets_blocked` → its text + "Tire as senhas do código e prepare um envio novo."
    - Entrypoint missing in the zip → "O arquivo de início <x> não está no pacote. Confira se o zip foi feito de dentro da pasta do app (tar com -C \"<pasta>\" .)."
    - 409 `no_stored_code` → its text.

- [ ] **Step 1: Write the failing tests** (MCP session + real upload via Task 6):
  - `TestRegistrarScriptNovo`: upload `main.py`; `registrar_app` with a valid script manifest → app exists, kind `script`, v1 `channel='mcp'`; upload row `used_at` set; text has the link.
  - `TestRegistrarVersaoNova`: then a second envio with `app_id` and changed `main.py` → `versao==2`, catalog has one app with that name.
  - `TestRegistrarSecretsRetrySameEnvio`: file with a fake key → tool error mentions "confirmar_segredos"; same `envio` with `confirmar_segredos:true` → success.
  - `TestRegistrarDropsEnv`: zip with `.env` → success, text contains "Deixei de fora: .env", sources have no `.env`.
  - `TestRegistrarRefusesDesktopAndExe`: kind `desktop` → error contains "Apps de janela"; entrypoint `app.exe` → same.
  - `TestRegistrarOtherPersonsEnvio`: envio created by person A, used by B → "Não achei esse envio".
  - `TestRegistrarMissingEntrypointHint`: zip with `pasta/main.py` and manifest entrypoint `main.py` → text contains "de dentro da pasta".
  - `TestRegistrarFolderAppRefused`: `app_id` of a folder app → text contains "pasta do computador".
  - `TestRegistrarAudited`: `claudeAudits` grows by 1 per successful call.
  - `TestFormatoDoApp`: text contains "pode registrar" and the area "Research"; `tipos == ["script","web"]`.
  - `TestVerAppCodigoGuardado` (+ `ficha_editavel` present for an editor, absent for a viewer).
  - `TestRegistrarVersaoMudaFicha`: update with `manifesto.summary` changed and a new input → app shows both; text lists "resumo" and "entradas" as changed.
  - `TestRegistrarVersaoIgnoraNomeArea`: update with `manifesto.name:"Outro"`, `area:"Compliance"`, other owner → success, app name/area/owner unchanged, text contains "isso só muda pelo site".
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** the handler package → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: registrar_app e formato_do_app para scripts`.

---

### Task 8: History shows "Código enviado · pelo Claude"

**Files:**
- Modify: `frontend/src/lib/api.ts:254` (`origin: "original" | "folder" | "restore" | "upload"; channel: "manual" | "mcp"`), `frontend/src/components/HistoryTimeline.tsx:38-41,54`, `frontend/src/pages/VersionPage.tsx:107`
- Test: `frontend/src/components/HistoryTimeline.test.tsx`

**Interfaces:**
- Consumes: Task 1 `channel` in version JSON.
- Produces: `versionOrigin` returns `"Código enviado"` for `upload`; rows and VersionPage render `<ByClaude />` after the origin text when `channel === "mcp"` (also on an `original` v1 that came by Claude).

- [ ] **Step 1: Write the failing tests:** `versionOrigin({origin:"upload",...})` → "Código enviado"; rendering a timeline with a `channel:"mcp"` version shows "pelo Claude"; `channel:"manual"` does not.
- [ ] **Step 2: Run** `npx vitest run src/components/HistoryTimeline.test.tsx` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** vitest + `tsc -b` → PASS.
- [ ] **Step 5: Commit** `Histórico mostra versões enviadas pelo Claude`.

---

### Task 9: Screen apps — import as `web`, versions, server-side guards

*Depends on the isolation plan for non-principal companies (see header).*

**Files:**
- Modify: `backend/internal/handler/script_import.go`, `backend/internal/handler/stored_code.go`, `backend/internal/handler/shared.go` (`checkShared`, `startShared`, `putShared`), `backend/internal/handler/mcp_registrar.go` (remove the Task 7 `web` refusal)
- Test: `backend/internal/handler/stored_code_test.go`, `backend/internal/handler/shared_test.go`

**Interfaces:**
- Consumes: Tasks 3–5, 7.
- Produces:
  - `importScript`: `m.Kind` ∈ {`""`→script, `script`, `web`}, else 400 "Escolha script ou app com tela.". For `web`: keep `m.Shared` as `&manifest.Shared{Enabled: true, Env: <sent Env with Default cleared when Secret>}` (all other shared fields zero → defaults; `Command` always cleared — confirmed later through `PUT /shared`, which needs `can.Restore`); `m.Run.Command=""` for both; `m.Language = manifest.ScriptRuntime(entrypoint)` for both. Slug for `web`: `Slugify(name)` cut to 40 chars (trailing `-` trimmed), and on clash with an existing `apps.slug` append `-2`, `-3`… (cut to fit 40); scripts keep `slug-<token8>`.
  - `web` when `h.shared.Svc == nil` → 503 `{"code":"shared_runtime_unavailable","error":<off sentence from Global Constraints>}` before any write.
  - `web` folder: after `ImportScript` returns the id, `SetAppFolder(id, h.shared.Svc.CodeDir(id))` (new hand-written query in `script_sources.go`: `UPDATE apps SET folder_path=$2 WHERE id=$1`), `os.RemoveAll(<temp folder>)`, `hydrateImportedScript` into the new path (create dirs 0700), then `recordSentVersion`. Done inside the same request; on failure delete the app row (`DELETE FROM apps WHERE id=$1`) so no half app is left.
  - `func (h *Handler) hydrateForShared(ctx context.Context, a sqlc.App) error` = `hydrateImportedScript(ctx, a, manifestOf(a))`, called at the top of `checkShared`, `startShared` and `putShared` (before `os.Stat`).
  - `sendSources`: when `h.shared.Svc != nil && h.shared.Svc.Busy(a.ID)` → 409 `busyMsg`.
  - `startShared` with `test == false`: `if reason, _ := h.executionBlock(ctx, a.ID); reason != ""` → 403 reason (publishing an unapproved version is refused, site included — decided by the product owner).

- [ ] **Step 1: Write the failing tests:**
  - `TestImportWebApp`: import kind `web`, files from `testdata/shared/streamlit`, `shared.env` `[{name:"API_KEY",secret:true,default:"x"}]` → app kind `web`, `manifest.shared.enabled`, env default cleared, slug `notas` (no token suffix); second import named "Notas" → slug `notas-2`.
  - `TestImportWebCodeDir`: `folder_path == h.shared.Svc.CodeDir(id)`, files present there; with `withShared` options `FolderRoot = <DataDir>/code` (isolation plan's helper), `GET /shared/check` is 200, not `shared_folder_outside`.
  - `TestImportWebSharedOff`: no `withShared` (Svc nil) → 503 `shared_runtime_unavailable` with the off sentence; no app row.
  - `TestImportWebCommandCleared`: `shared.command:"rm -rf /"` → stored manifest command empty.
  - `TestPublishAfterFolderLost` (with `withShared`): import web, `PUT /shared` enabled, `os.RemoveAll(folder)`, `POST /shared/publish` → 202 (folder rebuilt).
  - `TestSourcesRefusedWhileDeployBusy`: hold the deploy lock (start a publish with the fake engine blocked, as existing shared tests do) → `/sources` 409 `busyMsg`.
  - `TestPublishWaitsApproval`: workspace with approval, unapproved web app → publish 403 approval text; Testar still 202.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** handler + `./internal/shared/...` → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: apps com tela pelo mesmo caminho do import`.

---

### Task 10: `publicar_app_com_tela`, ADR and protocol v0.3

**Files:**
- Modify: `backend/internal/handler/mcp_registrar.go`, `backend/internal/handler/mcp_server.go` (`mcpRoutes` += `"GET /api/apps/{id}/shared"`, `"PUT /api/apps/{id}/shared"`, `"GET /api/apps/{id}/shared/check"`, `"GET /api/apps/{id}/shared/config"`, `"POST /api/apps/{id}/shared/publish"`, `"GET /api/apps/{id}/shared/deployments/{id}"`), `backend/internal/handler/mcp_protocolo.md` (screen-app section), `docs/PROTOCOLO-REGISTRO.md` (v0.3: pointer to the MCP flow, Passo 0 via `ver_app`), `backend/internal/handler/mcp_server_test.go` (route list test, if one counts routes)
- Create: `docs/adr/025-registrar-apps-pelo-claude.md`
- Test: `backend/internal/handler/mcp_registrar_test.go`

**Interfaces:**
- Consumes: Task 9; existing `SharedSettings`, `readiness.Report`, `configItem`, `deploymentOut`; `url` from `GET /api/apps/{id}/shared` (= `Svc.AppURL`).
- Produces: tool `publicar_app_com_tela`:
  ```go
  type publicarTelaIn struct {
      AppID   int64    `json:"app_id"`
      Ciente  []string `json:"ciente,omitempty" jsonschema:"os avisos que a pessoa disse estar ciente: file_data, windows_paths, secrets, container_limits"`
      Comando string   `json:"comando,omitempty" jsonschema:"o comando de início, só se a checagem pedir e a pessoa confirmar"`
      Esperar int      `json:"esperar,omitempty" jsonschema:"segundos para esperar o resultado, até 30"`
  }
  ```
  Flow: `GET /api/apps/{id}` (need `can.restore`, `updated_at`, `manifest.shared`); `PUT /api/apps/{id}/shared` with the current shared values, `Env` = manifest env ∪ `check.suggest_env` (from a first `GET …/shared/check`), `Command` = `in.Comando` or current, `Acks` = current ∪ `in.Ciente` filtered to `knownAcks`; `GET …/shared/check`:
  - any `missing` item → toolFail listing `title: detail` (and `claude_text` when present) — "Nada foi publicado.";
  - any `warn` not acked → toolFail listing each warning with its key and: "Pergunte à pessoa se está ciente de cada um; só então chame de novo com ciente."
  - `GET …/shared/config`: `required && !has_value` names → toolOK (not an error) "Registrei, mas falta preencher no Ziv: API_KEY, …. A pessoa preenche em <link /apps/{id}> (parte Compartilhar › Configurações) — nunca pela conversa. Quando ela avisar, chame publicar_app_com_tela de novo."
  - `POST …/shared/publish` (via `auditedAPI`); then poll `GET …/shared/deployments/{dep}` once per second up to `min(max(Esperar,0),30)` (default 30) like `ver_execucao`. Done ok → "No ar: <url> (versão <n>)." where `<url>` is `shared.url` from `GET /api/apps/{id}/shared`, never built by the tool. Failed → "A versão nova não abriu; a anterior continua no ar. Erro: <error_message>." Still running → "Ainda publicando; chame de novo com esperar 30." (a repeat call while busy must not start a second publish: on 409 `busyMsg`, find the latest deployment via `GET …/shared` and keep waiting on it).
  - 403 approval → "Esperando a aprovação de um administrador; publico depois que aprovarem."
  - 503 `shared_runtime_unavailable` (any step) → the off sentence; 409 `shared_folder_outside` → its text.
  Structured: `{app_id, situacao: "no_ar"|"erro"|"publicando"|"faltam_chaves"|"pendencias"|"esperando_aprovacao", endereco, faltam, avisos, erro}`.
  `registrar_app` text for a `web` app ends with "Agora chame publicar_app_com_tela com app_id <id>." (new app and new version alike — spec §2.4: updating publishes by itself, side-by-side).
  ADR 025 records: upload-by-URL instead of code in the call, `mcp_uploads`, server-side drop list, `origin='upload'` + `channel`, restore syncs sources, only stored-code apps update, approval gates publish, tool list; links ADR 010/021 and the spec.

- [ ] **Step 1: Write the failing tests** (`withShared`, fake engine):
  - `TestPublicarTelaNoAr`: register a Streamlit app by Claude (Tasks 6–7 path), `publicar_app_com_tela` → `situacao=="no_ar"`, `endereco` set, a deployment row `kind=publish`, `status` ok.
  - `TestPublicarTelaAvisoPedeCiente`: app writing `sqlite3.connect` (warn `file_data`) → `situacao=="pendencias"` text lists `file_data`; again with `ciente:["file_data"]` → `no_ar`; `shared_apps.acks` contains `file_data`.
  - `TestPublicarTelaFaltamChaves`: code reads `os.getenv("API_KEY")` → `faltam_chaves`, text has `API_KEY` and the `/apps/<id>` link and no value; after `PUT /shared/config {name:"API_KEY", value:"s3cr3t"}` → `no_ar`; no tool text in the test ever contains `s3cr3t`.
  - `TestPublicarTelaErroMantemAnterior`: publish v1, send v2 that fails health (fake engine failing start), publish → `situacao=="erro"`, text contains "a anterior continua no ar", `current_deployment_id` unchanged.
  - `TestPublicarTelaPrecisaRestore`: person without `can.Restore` → permission text, nothing published.
  - `TestPublicarTelaLinkFromService`: with `withShared` domain `teste.localhost:8080`, `endereco == h.shared.Svc.AppURL(slug)`.
  - `TestPublicarTelaSharedOff`: Svc nil → text is the off sentence, nothing written.
  - `TestMCPRoutesListed`: every new route is reachable by `callAPI` and `PUT /api/apps/{id}` is still refused.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** the tool, protocol section, ADR, protocol v0.3 note.
- [ ] **Step 4: Run** the full backend suite and frontend suite → PASS.
- [ ] **Step 5: Commit** `MCP Etapa 2: publicar app com tela pelo Claude, ADR 025`.

---

## Manual acceptance (Henry, Ziv no ar, empresa Teste — spec §6)

Run after deploy (`scripts/deploy-gcp.sh`), with the Claude Code "ziv" MCP pointing to `/w/teste/mcp`: (1) script novo e rodar; (2) mudar uma linha → versão 2 "Código enviado · pelo Claude", sem app duplicado; (3) pasta com dois programas, confirmando cada um; (4) Streamlit simples no link próprio, atualizar e ver a nova (needs the isolation plan for Teste); (5) pasta com `.env`: não sobe e o Claude diz qual chave preencher.

## Decided by the product owner (2026-11-06)

- Site's Publicar also waits for approval when the company requires it (Task 9) — accepted.
- A version via Claude may update code + resumo, entradas, saídas and (scripts) run settings; name, area, owner/backup, access stay site-only and are ignored with a notice (Tasks 4, 7).
- Readable addresses for screen apps — accepted; links always from `Svc.AppURL()`.
- Screen apps may be off per company; clear message when off (Tasks 9, 10).
