# Artifacts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A new "Artifacts" tab where people register Claude dashboards (by link or as an HTML file). Each artifact has tags, search, versions, the same access rules as apps, and no approval. App approval now starts off for new companies.

**Architecture:** New per-company tables (`artifacts`, `artifact_versions`, `artifact_viewers`, `artifact_grants`). HTML is stored in the existing content-addressed `code_blobs`. Access reuses `access.Person` with one generalised rule, so apps and artifacts share it. The HTML is served from an opaque origin (a CSP `sandbox` header plus an iframe `sandbox` attribute). The frontend adds a list page, a "new" page and a detail page, and it generalises `AccessSection` so the same component works for an app or an artifact.

**Tech Stack:** Go net/http, pgx, sqlc, React/Vite/TS, shadcn/ui, Vitest, Playwright (screenshots).

**Spec:** `docs/superpowers/specs/2026-11-03-home-e-artifacts-design.md` (sections 2–4 and 6). The Home tab (section 5) gets its own plan after this one ships.

## Global Constraints

- All UI text is in Brazilian Portuguese, in plain language. The visual style follows ADR 008 (neutral with violet), using the existing components in `components/bits.tsx` and `components/ui`.
- Every table lives inside each company's schema. Add a new migration only; never edit an old one.
- SQL lives only in `backend/internal/database/queries/*.sql`. After `sqlc generate`, keep only the generated files for the queries you touched and `models.go`. Revert any unrelated churn with `git checkout` (this happened before).
- File artifacts accept a single `.html` or `.htm` file of at most **16 MB**. The server checks this; the file picker is not enough.
- Link artifacts accept only `https://`. In development they also accept `http://localhost`.
- The content response always carries `Content-Security-Policy: sandbox allow-scripts allow-popups allow-forms allow-downloads` and never includes `allow-same-origin`. It also carries `X-Content-Type-Options: nosniff`.
- Artifacts have **no approval**. They never show up in Admin's "Esperando aprovação".
- Permissions:
  - Registering an artifact uses the same check as registering an app (`CanRegister`).
  - The role permissions `edit_any_app`, `restore_any_app` and `see_all_apps`, and area-wide grants, also reach the area's artifacts.
  - A per-artifact grant offers only `edit` and `restore`.
- Secrets follow `block_embedded_secrets`, the same way script import does (`script_import.go:98-119`):
  - The block applies only when `h.workspace != nil`.
  - When it does not block, the request needs a confirmation flag.

## Review Focus

1. **HTML that tries to escape.** Inside the iframe, a script calls `fetch("/api/me")`, reads `document.cookie` and touches `parent.document`. All of these must fail. The content URL opened directly in a tab must still be sandboxed. *(Task 3 test `TestArtifactContentIsSandboxed`; Task 8 checks the iframe attribute.)*
2. **Restricted artifact probed by id.** Someone without access calls detail, content, versions, restore or delete. Every one of them returns 404 "Artifact não encontrado.", and the artifact never appears in the list. *(Task 3 `TestRestrictedArtifactHidden`.)*
3. **Upload over the limit or of the wrong type.** A 17 MB `.html`, a `.pdf` renamed to `.html` (not text), or a `.zip`. The server answers 400 with a clear message in Portuguese and saves nothing. *(Task 3 `TestArtifactUploadLimits`.)*
4. **Deleting an area that still has artifacts.** Today this would fail on the foreign key with a 500. It must answer 409 with "Essa área ainda tem apps ou artifacts…". *(Task 4 `TestDeleteAreaWithArtifacts`.)*
5. **Restoring an old version of a link artifact after it was switched to a file, and the reverse.** Restore copies the old version's kind and content, so the artifact's kind follows its current version. *(Task 3 `TestArtifactRestoreSwitchesKind`.)*

---

### Task 1: Migration, queries and the approval default

**Files:**
- Create: `backend/internal/database/migrations/016_artifacts.sql`
- Create: `backend/internal/database/queries/artifacts.sql`
- Modify: `backend/internal/database/queries/access.sql` (`ListAreas` gains `artifact_count`)
- Modify: `backend/internal/workspace/portal.go:227` (after `Migrate` in company creation)
- Test: `backend/internal/workspace/portal_test.go`

**Interfaces:**
- Produces these tables:
  - `artifacts` with the columns `id BIGSERIAL`, `name TEXT NOT NULL`, `summary TEXT NOT NULL DEFAULT ''`, `kind TEXT NOT NULL CHECK (kind IN ('link','file'))`, `tags TEXT[] NOT NULL DEFAULT '{}'`, `area_id BIGINT NOT NULL REFERENCES areas(id)`, `owner_id BIGINT NOT NULL REFERENCES people(id)`, `backup_id BIGINT REFERENCES people(id)`, `restricted BOOLEAN NOT NULL DEFAULT false`, `origin TEXT NOT NULL DEFAULT 'manual' CHECK (origin IN ('manual','mcp'))`, `created_by BIGINT NOT NULL REFERENCES people(id)`, `created_at`, `updated_at`.
  - `artifact_versions` with the columns `id`, `artifact_id … ON DELETE CASCADE`, `number INT`, `kind`, `url TEXT`, `blob_sha256 TEXT REFERENCES code_blobs(sha256)`, `file_name TEXT NOT NULL DEFAULT ''`, `size BIGINT NOT NULL DEFAULT 0`, `note TEXT NOT NULL DEFAULT ''`, `restored_from INT`, `author_id BIGINT REFERENCES people(id)`, `created_at TIMESTAMPTZ DEFAULT clock_timestamp()`. It has `UNIQUE(artifact_id, number)` and `CHECK ((kind='link') = (url IS NOT NULL) AND (kind='file') = (blob_sha256 IS NOT NULL))`.
  - `artifact_viewers` and `artifact_grants`, shaped exactly like `app_viewers` and `app_grants` in `007_access.sql`. In `artifact_grants`, `permission` is CHECKed to `('edit','restore')`.
  - The same migration runs `ALTER TABLE security_settings ALTER COLUMN require_app_approval SET DEFAULT FALSE`.
- Produces these sqlc queries:
  - `CreateArtifact`, `GetArtifact`, `ListArtifacts` (joins the area name, the owner name and the current version number), `UpdateArtifactFicha` (name, summary, tags), `UpdateArtifactCare` (owner, backup, area), `SetArtifactKind`, `TouchArtifact`, `DeleteArtifact`.
  - `CreateArtifactVersion`, `ListArtifactVersions` (joins the author name), `GetArtifactVersion(artifact_id, number)`, `LatestArtifactVersion`.
  - The access set: `ListArtifactViewers`, `ClearArtifactViewers`, `AddArtifactViewerArea`, `AddArtifactViewerPerson`, `SetArtifactRestricted`, `ListArtifactGrants`, `ClearPersonArtifactGrants`, `AddArtifactGrant`, `ListPersonArtifactGrants`, `ListViewerArtifactIDs`. Each mirrors its `app_*` twin in `access.sql`.
  - `ListAreas` gains `(SELECT count(*) FROM artifacts x WHERE x.area_id = a.id) AS artifact_count`.
- Produces `workspace` behaviour: a newly created company has `require_app_approval = false`. Existing schemas keep their current row.

- [ ] **Step 1: Write the failing test.**
  - Add `TestNewCompanyStartsWithoutApprovalRequired` to `portal_test.go`, following the existing company-creation test there.
  - It creates a company, then reads `SELECT require_app_approval FROM security_settings` inside the new schema and asserts `false`.
- [ ] **Step 2: Run the test and confirm it fails.**
  - Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/workspace -run TestNewCompanyStartsWithoutApprovalRequired`
  - Expected: FAIL (`true`).
- [ ] **Step 3: Write the migration, the queries and the update in `portal.go`.**
  - The migration's `ALTER … SET DEFAULT` does not change the row that `010` already inserted, so `portal.go` must also run the update.
  - Run `UPDATE security_settings SET require_app_approval = FALSE WHERE id` through a new sqlc query `DisableAppApprovalByDefault` (in `queries/artifacts.sql` is fine) right after `Migrate`.
  - Run `mise x -- sqlc generate` and restore the unrelated generated files.
- [ ] **Step 4: Run the workspace tests and the full handler tests.**
  - Expected: PASS. The migration must apply cleanly to the test schemas.
- [ ] **Step 5: Commit.**
  - Message: `Artifacts: tabelas, consultas e aprovação de apps desligada em empresa nova`.

### Task 2: Access rules for artifacts

**Files:**
- Modify: `backend/internal/access/access.go`
- Modify: `backend/internal/handler/access.go` (`loadAccess`, a new `artifactRef`)
- Test: `backend/internal/access/access_test.go`

**Interfaces:**
- Consumes the Task 1 queries `ListViewerArtifactIDs` and `ListPersonArtifactGrants`.
- Produces:
  - `access.Person` gains `ViewerArtifacts map[int64]bool` and `ArtifactGrants map[int64]map[string]bool`.
  - `var ArtifactGrants = []string{GrantEdit, GrantRestore}`.
  - `func (p *Person) ForArtifact(a App) Can` follows the same rules as `For`, but takes viewers and per-item grants from the artifact maps. Area grants (`AreaGrants`) and role permissions apply unchanged.
  - For an artifact, `Can.Tech` equals `Can.Edit || Can.Restore` (it gates the access section), and `Can.Runs` is always false.
  - Refactor `For` and `ForArtifact` into one unexported `forItem(a App, viewer bool, itemGrants map[string]bool) Can`, with `For` keeping exactly its current behaviour.
  - `func artifactRef(a sqlc.Artifact) access.App` lives in `handler/access.go`.
- [ ] **Step 1: Write the failing tests in `access_test.go`.**
  - `TestForArtifactRestrictedHiddenWithoutViewer`: a restricted artifact gives an empty `Can{}` to a plain person. The same person gets `View` once `ViewerArtifacts[id]` is set.
  - `TestForArtifactAreaGrantReachesArtifacts`: `AreaGrants[area]["edit"]` gives `Edit` on an artifact of that area.
  - `TestForArtifactRolePermsReach`: `edit_any_app` gives `Edit` and `restore_any_app` gives `Restore`.
  - `TestForArtifactAppGrantDoesNotLeak`: `AppGrants[sameID]["edit"]` does **not** give `Edit` on an artifact with the same numeric id.
  - `TestForArtifactNeverRuns`: even with `see_all_runs`, `Runs` is false.
  - `TestForUnchanged`: keep the existing `For` tests green.
- [ ] **Step 2: Run the tests and confirm they fail.**
  - Run: `cd backend && mise x -- go test ./internal/access`
  - Expected: FAIL (the code does not compile yet).
- [ ] **Step 3: Implement the access changes and load the two new maps in `loadAccess`.**
- [ ] **Step 4: Run the tests and confirm they pass.**
  - Run `go test ./internal/access`, then the full `./internal/handler`.
  - Expected: PASS.
- [ ] **Step 5: Commit.**
  - Message: `Artifacts: mesmas regras de acesso dos apps`.

### Task 3: Artifacts API

**Files:**
- Create: `backend/internal/handler/artifacts.go`
- Create: `backend/internal/handler/artifact_access.go`. It reuses `groupGrants` and `pgInt8` from `app_access.go`.
- Modify: `backend/internal/handler/handler.go` (the routes)
- Test: `backend/internal/handler/artifacts_test.go`. Reuse `team()`, `setRole`, `makeLeader` and `exec` from `permissions_test.go`.

**Interfaces:**
- Consumes the Task 1 queries and Task 2's `ForArtifact` / `artifactRef`.
- Produces these routes. Every error message is in Portuguese. An artifact a person cannot see returns 404 "Artifact não encontrado.".
  - `GET /api/artifacts` returns `[]artifactSummaryJSON`, only the visible ones, newest `updated_at` first. Fields: `id`, `name`, `summary`, `kind`, `tags`, `area_id`, `area`, `owner_name`, `restricted`, `version`, `updated_at`.
  - `POST /api/artifacts` requires `CanRegister`. The body is `{name, summary, tags, area_id, kind, url?, file_name?, content_base64?, note?, confirm_secrets?}`. It returns 201 with `artifactDetailJSON`. The owner is the caller. Version 1 is created in the same transaction.
  - `GET /api/artifacts/{id}` returns `artifactDetailJSON`, which is the summary fields plus `owner_id`, `backup_id`, `backup_name`, `origin`, `can access.Can`, `versions []artifactVersionJSON{number, kind, url, file_name, size, note, author_name, restored_from, created_at}` (newest first) and `current` (the latest version number).
  - `PUT /api/artifacts/{id}` takes `{name, summary, tags}` and needs `can.Edit`. If the body also carries `owner_id`, `backup_id` or `area_id`, it needs `can.Access`.
  - `POST /api/artifacts/{id}/versions` takes `{kind, url?, file_name?, content_base64?, note, confirm_secrets?}`, needs `can.Edit`, and returns `artifactDetailJSON`. It also sets `artifacts.kind` to the new version's kind.
  - `POST /api/artifacts/{id}/versions/{n}/restore` needs `can.Restore`. It creates version N+1, a copy of version `n` with `restored_from = n`.
  - `DELETE /api/artifacts/{id}` needs `can.Access || Has(ManageRoles)` and returns 204.
  - `GET /api/artifacts/{id}/content?v=<n>&download=1` needs `can.View`. With `v` omitted, it serves the latest version. It answers 404 for a link version.
    - It sends `Content-Type: text/html; charset=utf-8` plus the CSP sandbox and nosniff headers from the Global Constraints.
    - With `download=1`, it adds `Content-Disposition: attachment; filename=<file_name>`.
  - `GET` and `PUT /api/artifacts/{id}/access` and `PUT /api/artifacts/{id}/grants` behave like the app versions in `app_access.go`, with these differences:
    - `GET` needs `can.Tech`.
    - Grants are validated against `access.ArtifactGrants`.
    - `whole_area` writes `area_grants` with the same rule as apps.
- Validation, as one `validateArtifactInput(kind, url, fileName string, content []byte, dev bool) (msg string)`. These are the exact messages:
  - A name that is empty after trimming: "Dê um nome ao artifact."
  - A kind other than link or file: "Escolha link ou arquivo."
  - A bad URL: "Cole um link que comece com https://."
  - A bad file extension: "Envie um arquivo .html."
  - A file over 16 MB (`16<<20` bytes): "O arquivo passa de 16 MB."
  - Content that is not valid UTF-8, or that contains a NUL byte: "Esse arquivo não parece uma página HTML."
  - An unknown area: "Escolha uma área existente."
- The body limit is `24<<20`, read through `http.MaxBytesReader`, because base64 adds about a third. Do not use `readJSON`: its limit is too small.
- Secrets run `scanner.FindSecrets(fileName, string(content))` and mirror `script_import.go:98-119`:
  - Codes `artifact_secrets_blocked` (422) and `artifact_secrets_confirmation_required` (409).
  - Each response carries a `secret_count`.
- [ ] **Step 1: Write the failing tests in `artifacts_test.go`.**
  - `TestCreateLinkArtifact`: the criador posts a link with tags `["vendas","semanal"]`. The response is 201, the detail shows version 1 of kind `link`, and the list contains it.
  - `TestCreateFileArtifactAndServeContent`: posts `<html><body>Oi</body></html>` as base64. `GET content` returns 200 and the body. The `Content-Security-Policy` header contains `sandbox allow-scripts` and does **not** contain `allow-same-origin`. `X-Content-Type-Options` is `nosniff`.
  - `TestArtifactContentIsSandboxed`: `GET content?download=1` carries `Content-Disposition` with `attachment` and keeps the CSP. `GET content` on a link artifact returns 404.
  - `TestArtifactUploadLimits`:
    - A 16 MB + 1 byte file returns 400 "O arquivo passa de 16 MB.".
    - `x.pdf` returns 400 "Envie um arquivo .html.".
    - Bytes containing `\x00` return 400 "Esse arquivo não parece uma página HTML.".
    - `http://example.com` returns 400 "Cole um link que comece com https://.".
    - After all of these, the table holds no artifacts.
  - `TestUserCannotCreateArtifact`: the usuário (Bia) gets 403.
  - `TestRestrictedArtifactHidden`: the admin creates an artifact and restricts it with no viewers. For Bia:
    - list, detail, content, the versions POST, restore and DELETE all return 404 "Artifact não encontrado." or the list omits it.
    - After her area is added as a viewer, the detail returns 200.
  - `TestArtifactVersionsAndRestore`: version 2 is posted as a file, then version 1 is restored. That gives 3 versions; version 3 has `restored_from = 1` and kind `link`.
  - `TestArtifactRestoreSwitchesKind`: the artifact's `kind` follows the current version after each step.
  - `TestAreaGrantEditsArtifact`: an `area_grants` row `edit` for Bia lets her `PUT` the ficha, and `restore` is still 403.
  - `TestArtifactSecretsFollowCompanyPolicy`:
    - The HTML holds `const apiKey = "sk-ant-` plus 30 characters `"`.
    - Without confirmation the API returns 409 `artifact_secrets_confirmation_required`; with `confirm_secrets` it returns 201.
    - Do this with the default handler, where workspace is nil.
  - `TestDeleteArtifactNeedsAccess`: a creator who isn't the owner gets 403 and the owner gets 204.
- [ ] **Step 2: Run the tests and confirm they fail.**
  - Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/handler -run Artifact`
  - Expected: FAIL (the routes return 404 or the code does not compile).
- [ ] **Step 3: Implement `artifacts.go` and `artifact_access.go` and register the routes.**
  - Store file content with the existing `PutCodeBlob` (sha256 hex of the bytes).
  - Write the version and the artifact update in one `h.q.InTx`.
- [ ] **Step 4: Run the tests and confirm they pass.**
  - Run the handler package with `-run Artifact`, then the full `go test ./...`.
  - Expected: PASS.
- [ ] **Step 5: Commit.**
  - Message: `Artifacts: API com link ou HTML isolado, versões e acesso`.

### Task 4: An area with artifacts cannot be deleted

**Files:**
- Modify: `backend/internal/handler/team.go:390-415`, along with the `AppCount` JSON struct near line 282
- Test: `backend/internal/handler/team_test.go`

**Interfaces:**
- Consumes `ListAreas.ArtifactCount` from Task 1.
- Produces `artifact_count` on each area in `GET /api/areas`.
- [ ] **Step 1: Write the failing test.**
  - `TestDeleteAreaWithArtifacts` inserts an artifact in an empty area, then calls `DELETE /api/areas/{id}`.
  - It expects 409 with the body containing "Essa área ainda tem apps ou artifacts".
- [ ] **Step 2: Run the test and confirm it fails.**
  - Expected: FAIL (500 from the foreign key).
- [ ] **Step 3: Implement.**
  - The check becomes `a.AppCount+a.ArtifactCount > 0`.
  - The message becomes "Essa área ainda tem apps ou artifacts. Mude a área deles antes de apagar.".
- [ ] **Step 4: Run the test and confirm it passes.**
  - Run `go test ./internal/handler -run Area`.
  - Expected: PASS.
- [ ] **Step 5: Commit.**
  - Message: `Área com artifacts não pode ser apagada`.

### Task 5: Frontend API client and a generalised AccessSection

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/lib/permissions.ts`
- Modify: `frontend/src/components/ficha/AccessSection.tsx`
- Modify: `frontend/src/pages/AppPage.tsx:335`
- Test: `frontend/src/components/ficha/AccessSection.test.tsx`

**Interfaces:**
- Produces these types in `api.ts`:
  - `ArtifactSummary`, `ArtifactVersion` and `ArtifactDetail` (`can: Can`), matching the Task 3 JSON.
  - `ArtifactInput = { name: string; summary: string; tags: string[]; area_id: number; kind: "link" | "file"; url?: string; file_name?: string; content_base64?: string; note?: string; confirm_secrets?: boolean }`.
- Produces these calls on the `api` object:
  - `artifacts()`, `artifact(id)`, `createArtifact(b)` and `updateArtifact(id, b)`.
  - `artifactVersion(id, b)`, `restoreArtifact(id, n)` and `deleteArtifact(id)`.
  - `artifactAccess(id)`, `setArtifactAccess(id, b)` and `setArtifactGrants(id, b)`.
- Produces `artifactContentUrl(id: number, v?: number, download = false): string`.
- Produces `ARTIFACT_GRANTS` in `permissions.ts`. It is `GRANTS` filtered to `edit` and `restore`.
- Produces a new signature: `AccessSection({ target, me })`.
  - `target` is `{ kind: "app" | "artifact"; id: number; can: Can; areaName: string }`.
  - The component picks the api calls and the grant list by `kind`.
  - The "whole area" box reads "Em todos os apps e artifacts da área X" for both kinds, because area grants now reach both.
  - `AppPage` passes `{ kind: "app", id: app.id, can: app.can, areaName: app.manifest.area }`.
- [ ] **Step 1: Update the existing `AccessSection` tests to the new props and add two artifact tests.**
  - `shows only edit and restore grants for an artifact`: renders with `kind:"artifact"` and asserts "Ver a parte técnica" is not present.
  - `saves artifact viewers through the artifact api`: mocks `api.setArtifactAccess` and asserts it was called with `(id, { restricted: true, area_ids: [..], person_ids: [] })`.
- [ ] **Step 2: Run the tests and confirm they fail.**
  - Run: `cd frontend && mise x -- npx vitest run src/components/ficha/AccessSection.test.tsx`
  - Expected: FAIL.
- [ ] **Step 3: Implement the api additions and the `AccessSection` refactor, and update `AppPage`.**
- [ ] **Step 4: Run the tests and confirm they pass.**
  - Run the AccessSection tests, `AppPage.test.tsx` and `mise x -- npx tsc -b`.
  - Expected: PASS, with no type errors.
- [ ] **Step 5: Commit.**
  - Message: `Artifacts: cliente da API e seção de acesso reaproveitada`.

### Task 6: The artifacts list and the sidebar

**Files:**
- Create: `frontend/src/pages/ArtifactsPage.tsx`
- Modify: `frontend/src/App.tsx` (routes `artifacts`, `artifacts/novo`, `artifacts/:id`)
- Modify: `frontend/src/components/Layout.tsx:22` (NAV)
- Test: `frontend/src/pages/ArtifactsPage.test.tsx` and `frontend/src/components/Layout.test.tsx`

**Interfaces:**
- Consumes `api.artifacts()` and `canRegister(person)`.
- Produces the route `/artifacts`.
  - The NAV gets the item `{ to: "/artifacts", label: "Artifacts", icon: PanelsTopLeft }` right after "Catálogo". Renaming the catalog to "Apps" belongs to the Home plan.
- Behaviour:
  - Page header "Artifacts", with a search box and a **Novo artifact** button that links to `/artifacts/novo` and shows only when `canRegister`.
  - Search matches name, summary, tags and area, case-insensitive.
  - Clicking a tag chip on a card filters by that tag. The active tag shows as a removable chip ("× vendas") beside the search.
  - There is an area filter (a `NativeSelect`, "Todas as áreas").
  - Card contents: a link or file icon, the name, a one-line summary, tag chips, the area, "versão N · <relative date>", and a lock icon when restricted. The whole card links to `/artifacts/:id`.
  - Empty states:
    - No artifacts: `EmptyState` with "Nenhum artifact ainda", plus the explanation "Guarde aqui os dashboards e páginas que o time cria no Claude." and the button for people who can register.
    - A search with no match: "Nada encontrado para essa busca.".
- [ ] **Step 1: Write the failing tests.**
  - `ArtifactsPage.test.tsx`:
    - `lists artifacts and filters by search`.
    - `clicking a tag filters and the chip clears it`.
    - `filters by area`.
    - `hides Novo artifact for a usuário`.
    - `shows empty state`.
  - `Layout.test.tsx`: `shows Artifacts in the sidebar`.
- [ ] **Step 2: Run the tests and confirm they fail.**
  - Run: `mise x -- npx vitest run src/pages/ArtifactsPage.test.tsx src/components/Layout.test.tsx`
  - Expected: FAIL.
- [ ] **Step 3: Implement the page, the routes and the NAV item.**
  - Load the cofounder `frontend-design` skill first.
  - Follow `CatalogPage.tsx` for the header and search pattern.
- [ ] **Step 4: Run the tests and confirm they pass.**
  - Expected: PASS.
- [ ] **Step 5: Commit.**
  - Message: `Artifacts: lista com busca, etiquetas e área`.

### Task 7: Creating an artifact

**Files:**
- Create: `frontend/src/pages/NewArtifactPage.tsx`
- Create: `frontend/src/components/ArtifactSourceField.tsx`. It is shared with Task 8's "Enviar versão nova".
- Create: `frontend/src/lib/tags.ts`, containing `parseTags(text: string): string[]`, which trims, lowercases, drops empties and dedupes.
- Test: `frontend/src/pages/NewArtifactPage.test.tsx` and `frontend/src/lib/tags.test.ts`

**Interfaces:**
- Produces `ArtifactSourceField({ value, onChange })`.
  - `value` is `{ kind: "link" | "file"; url: string; file?: File }`.
  - It shows two tabs, "Colar link" and "Subir arquivo".
  - The file input has `accept=".html,.htm"`.
  - Before sending, it shows the client-side errors with the same texts as Task 3: wrong extension, over 16 MB, and a link not starting with `https://`.
- Produces `readAsBase64(file: File): Promise<string>`, exported from `ArtifactSourceField.tsx`.
- Form fields: name, description, tags (one text field with commas, plus suggestion chips from the tags of existing artifacts, where a click adds the tag), area (`NativeSelect` from `api.areas()`), and "Quem vê" (Todos / Só quem eu escolher).
  - When "só quem eu escolher" is chosen, the artifact is created and then the page lands on the artifact page with the access section open to pick viewers.
- The 409 secrets response opens a confirmation in the page:
  - The text is "Encontrei N possíveis senhas ou chaves nesse arquivo. Quem abrir o artifact poderá vê-las."
  - The buttons are "Enviar mesmo assim" and "Cancelar".
- The 422 response shows its error message.
- On success the page navigates to `/artifacts/:id`.
- [ ] **Step 1: Write the failing tests.**
  - `tags.test.ts`: `parseTags(" Vendas, semanal ,,vendas")` gives `["vendas","semanal"]`.
  - `NewArtifactPage.test.tsx`:
    - `creates a link artifact` (asserts the `createArtifact` payload).
    - `uploads a file as base64`.
    - `rejects a non-html file before sending`.
    - `asks to confirm secrets on 409 and resends with confirm_secrets`.
- [ ] **Step 2: Run the tests and confirm they fail.**
  - Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run the tests and confirm they pass.**
  - Expected: PASS.
- [ ] **Step 5: Commit.**
  - Message: `Artifacts: criar por link ou arquivo`.

### Task 8: The artifact page

**Files:**
- Create: `frontend/src/pages/ArtifactPage.tsx`
- Test: `frontend/src/pages/ArtifactPage.test.tsx`

**Interfaces:**
- Consumes:
  - `api.artifact`, `updateArtifact`, `artifactVersion`, `restoreArtifact`, `deleteArtifact` and `artifactContentUrl`.
  - `AccessSection({ target: { kind: "artifact", … } })` and `ArtifactSourceField`.
- Layout, top to bottom:
  - The header has the name, the tags and the area.
  - File kind: the actions **Abrir em tela cheia** (`artifactContentUrl(id)` in a new tab) and **Baixar** (`download=1`).
  - Link kind: the action **Abrir**, which opens the URL in a new tab with `rel="noopener noreferrer"`.
  - File kind: an `<iframe title={name} src={artifactContentUrl(id)} sandbox="allow-scripts allow-popups allow-forms allow-downloads">`.
    - The iframe takes the full content width, with height `calc(100vh - 12rem)` and a minimum of 480px. It sits in a bordered rounded box.
    - The `sandbox` attribute value must match the constant `ARTIFACT_SANDBOX`, exported from this file.
  - Link kind: a card that shows the link's domain and the Abrir button.
  - "Sobre": the description, the owner and the backup, with an "Editar" link (`can.edit`) for name, description and tags.
  - "Versões": a list, newest first, showing "Versão N", the note, the author, the date, "(cópia da versão M)" when restored, "Abrir", and "Voltar para esta" when `can.restore` and the version is not current.
    - **Enviar versão nova** (`can.edit`) opens an inline form with `ArtifactSourceField` and a "O que mudou?" note.
  - `AccessSection` shows when `can.tech`.
  - **Apagar artifact** shows when `can.access` or the person is an admin. It asks for confirmation with the text "Apagar \"<nome>\"? Isso não pode ser desfeito.".
  - A 404 from the API shows "Artifact não encontrado." with a link back to `/artifacts`.
- [ ] **Step 1: Write the failing tests.**
  - `renders file artifact in a sandboxed iframe`: asserts the attribute equals `ARTIFACT_SANDBOX` and does not contain `allow-same-origin`.
  - `renders link artifact with Abrir in new tab`.
  - `hides edit, new version and restore without permission`.
  - `restores an older version`.
  - `sends a new version with a note`.
  - `deletes after confirmation and goes back to the list`.
  - `shows not found`.
- [ ] **Step 2: Run the tests and confirm they fail.**
  - Expected: FAIL.
- [ ] **Step 3: Implement the page.**
  - Load the `frontend-design` skill first.
- [ ] **Step 4: Run the full Vitest suite and tsc.**
  - Run: `cd frontend && mise x -- npx vitest run && mise x -- npx tsc -b`
  - Expected: PASS.
- [ ] **Step 5: Commit.**
  - Message: `Artifacts: página com HTML isolado, versões e acesso`.

### Task 9: Visual check, docs, then commit and push

**Files:**
- Create: `e2e/artifacts.mjs`. Copy the pattern of `e2e/baixar-resultado.mjs`.
  - It logs into the running dev Hub on 5173 and seeds data through the API: one link artifact, plus one file artifact with a small Chart.js-from-CDN HTML.
  - It screenshots the list, the file artifact page and the new-artifact page, in light, dark and at 390px mobile.
- Modify: `docs/PRD.md`. Add "3.14 Artifacts" with what it does, who sees it and no approval. Change 3.7/Admin to say approval starts off in a new company.
- Modify: `docs/TASKS.md`. Add an Artifacts row (Done), a Home row (Pending, its own plan) and an MCP publishing row (Pending).
- Create: `docs/adr/018-artifacts.md`. It covers the sandboxed opaque origin versus a subdomain, HTML in `code_blobs`, permissions shared with apps, and no approval.
- Modify: `docs/INFRASTRUCTURE.md`, only if it is missing (no new services).
- [ ] **Step 1: Run the screenshot script with the dev servers up.**
  - Never start a test Hub on 8080. Use the dev Hub that is already running.
  - Review every image for alignment, contrast, mobile overflow and whether the chart renders inside the iframe. Fix anything off and re-run the affected tests.
- [ ] **Step 2: Run the full Go and Vitest suites one last time.**
  - Expected: all PASS.
- [ ] **Step 3: Update the docs.**
- [ ] **Step 4: Commit and push.**
  - Message: `Artifacts: conferência visual e documentação`. Then `git push`.
