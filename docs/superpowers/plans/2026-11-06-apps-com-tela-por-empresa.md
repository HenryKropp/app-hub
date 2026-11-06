# Apps com tela em todas as empresas — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Shared apps ("apps com tela", Podman boxes on the server) work in every company, and no company can see, stop, back up, reach or open another company's apps.

**Architecture:** Every company gets its own shared-app stack (deploy service, supervisor, gateway, app-database admin), built by one new wiring package and plugged into the company's handler by the portal. All of these stacks share one Podman engine, one Postgres and one HTTP server. Each company's stack sees the engine through a *scoped* wrapper that only lists and touches boxes carrying the company's key. Every technical name carries a short company key. The main company ("principal") keeps the empty key and today's names exactly, so nothing it runs has to move. A host router in front of the portal picks the company from the Host. One supervisor *group* reconciles and ticks the companies one after another, so a 1 GB VM never runs N loops in parallel. Shared apps are **off per company by default**: the Ziv owner turns them on for a company in principal's Admin tab. Principal is always on, and "teste" starts on. A server-wide slot counter limits how many boxes run at once across all companies.

**Tech Stack:** Go 1.x (pgx, sqlc), Podman (rootless), Caddy on-demand TLS, React/Vitest (one page).

**Spec:** `docs/superpowers/specs/2026-11-06-mcp-registrar-apps-design.md`. This plan covers **only item 5 of section 2** ("Apps com tela em todas as empresas"). Background: ADR 010 (`docs/adr/010-apps-compartilhados.md`), ADR 015 (decision 1: shared apps only in principal), ADR 022 (GCP VM, Caddy `ask`).

## Decisions taken in this plan

**Company key.** Each company other than principal has a key: `"w"` plus the first 11 hex characters of its schema name (`tenant_0123456789abcdef01234567` gives `w0123456789a`). Principal's key is `""`.
- The key starts with a letter, is 12 characters long, and never changes.
- The schema is random and companies are never deleted or renamed, so the key is never reused.
- `Portal.create` regenerates the schema if its key would collide. At boot the portal refuses the shared stack to a company whose key duplicates another's.
- The slug is not used for the key: it can be 40 characters long, which would push Postgres names past their limit.

**Names.** "—" in the principal column means the name stays what it is today.

| Thing | Principal (key `""`) | Other company (key `wK`) |
|---|---|---|
| Container/image/network label | `ziv.app=<id>` (no workspace label) | adds `ziv.workspace=wK` |
| Network | `ziv-apps-<id>` (+ legacy) | `ziv-apps-wK-<id>` |
| Container | `ziv-<app>-<dep>`, `ziv-<app>-teste-<dep>` | `ziv-wK-<app>-<dep>`, `ziv-wK-<app>-teste-<dep>` |
| Image | `localhost/ziv/<app>:<dep>` | `localhost/ziv/wK/<app>:<dep>` |
| App database and role | `app_<id>`, `app_<id>_test_<dep>` | `app_wK_<id>`, `app_wK_<id>_test_<dep>` |
| Data folder (stage, apps, backups, code) | `$SHARED_DATA_DIR` | `$SHARED_DATA_DIR/workspaces/wK` |
| App address | `<app>.<domain>`, `<app>--teste.<domain>` | `<app>.<empresa>.<domain>`, `<app>--teste.<empresa>.<domain>` |
| Usage meter, backup rows, deployments | the company's own schema (already isolated) | same |
| Gateway codes and sessions | principal's gateway | the company's own gateway object |

**Subdomain format: `<app>.<empresa>.zivapp.duckdns.org`.** The alternative was `<app>--<empresa>`. Reasons for the nested format:
1. A DNS label holds at most 63 characters. App slugs can be 40 and company slugs 40, so `<app>--<empresa>` (up to 82) breaks. With the nested format each part stays its own label.
2. The test address keeps its current form, `<app>--teste`, inside each company.
3. Parsing is unambiguous: one label means principal, two labels mean an app in a company. Company slugs may contain `--`, which would make `--` splitting ambiguous.
4. The company's gateway can use `"<empresa>." + SHARED_APPS_DOMAIN` as its own domain, so `Gateway.Match`, `Service.AppURL` and `TestURL` stay unchanged.
5. Checked on 2026-11-06: `foo.teste.zivapp.duckdns.org` and `a--teste.teste.zivapp.duckdns.org` resolve to 34.24.21.104. Browsers resolve `*.*.localhost` locally.

Principal keeps `<app>.zivapp.duckdns.org`, so existing links still work. `<app>.principal.<domain>` is refused (404), so principal has a single address.

**Migrating principal.** Nothing moves. Principal's scope is "boxes with no `ziv.workspace` label", which covers every box running today, including Kova and App Hub era labels. Its database prefix, data folder, domain and legacy-network cleanup are byte-for-byte today's. Rollback hazard: if this binary is replaced by an older one after a company has published, the old principal supervisor would delete company boxes as orphans. Their databases and backups survive. This goes in the ADR.

**Spec conflict ruled on.** Spec §2.3 writes the app link as `https://<app>.zivapp.duckdns.org` for every company. That collides across companies. This plan uses `<app>.<empresa>.zivapp.duckdns.org` outside principal. The ADR in Task 12 records this, and the MCP plan must show `Svc.AppURL(slug)` instead of building the link by hand.

**On/off per company (decided by Henry, 2026-11-06).**
- Shared apps are off by default in every company. Only the Ziv owner can turn them on for a company: a person with `manage_roles` in principal, the same company that alone creates companies (ADR 015).
- The switch lives in principal's existing Admin tab (ADR 017), in a new "Empresas" section.
- Principal is always on (hard-coded, so a fresh server never locks its owner out). The company "teste" is seeded on.
- When a company is off:
  - its API keeps today's 503 (`shared_runtime_unavailable`), reworded: "Os apps com tela não estão ligados para esta empresa. Peça ao administrador do Ziv para ligá-los.";
  - its addresses show the same text;
  - nothing new starts;
  - running boxes are put to sleep on the next supervisor tick.
- Every company still gets its stack attached at boot. Turning a company on takes effect at once, with no restart.

**Containment for strangers' code.** These limits apply to companies other than principal:
- **Code folder.** A shared app may only be built from a folder inside its company's code folder (`<data>/code/<appID>`). Without this, a company could point `folder_path` at `~/ziv/ziv.env` or at another company's code and copy it into an image.
- **Published apps per company:** at most `SHARED_APPS_PER_COMPANY` (default **2**). Sleeping apps cost disk (images of about 200–400 MB each on a 30 GB disk), not RAM.
- **Memory per app:** at most `SHARED_APP_MAX_MEMORY_MB` (default **256**).

**Server-wide running limit (all companies, principal included).**
- At most `SHARED_APPS_MAX_RUNNING` boxes run at once (default **2**), test boxes included.
- Their reserved memory must also fit `SHARED_APPS_MEMORY_BUDGET_MB` (default **512**).
- Why these numbers:
  - The e2-micro has 1024 MB.
  - Measure on the VM before Task 12 and correct the numbers in the ADR if the free memory differs. Estimate: the Ziv server about 80–120 MB, `ziv-db` about 150–200 MB (`shared_buffers=64MB` plus connections), Caddy about 40 MB, Ubuntu Minimal and rootless Podman (conmon, pasta) about 150 MB.
  - That leaves about 550–600 MB, so two boxes of 256 MB fit in RAM. The 2 GB swap absorbs spikes; it is not capacity.
  - The old per-company default (3 × 512 MB) could alone ask for 1.5 GB.
- Principal apps are not capped per app, but they count against the same slots and budget. A principal app reserving 512 MB uses the whole budget while it runs.
- When the server is full:
  - publishing, testing or Ligar says so;
  - a sleeping app's address shows "Servidor no limite" and keeps retrying by itself;
  - a box that replaces its own running version (an update publish or a rollback) is always allowed.

## Global Constraints

- Tools run through `mise x --`. Backend: `cd backend && mise x -- go test ./...` (needs `DATABASE_URL`). Frontend: `cd frontend && mise x -- npx vitest run`.
- sqlc: after editing `backend/internal/database/queries/shared.sql`, run `cd backend && mise x -- sqlc generate`. Keep only `shared.sql.go` and `models.go` changes, and revert other churn with `git checkout` (`people.sql.go` is hand-edited; see docs/TASKS.md).
- User-facing text is Brazilian Portuguese in plain words. Code, comments and identifiers are English.
- Principal's names, addresses and data paths must not change (existing links and boxes keep working).
- Production is a 1 GB e2-micro. Add no new goroutine per company, and make no podman call per company per tick when the company has no shared apps.
- No new dependency.
- Limit defaults are exactly: `SHARED_APPS_MAX_RUNNING=2`, `SHARED_APPS_MEMORY_BUDGET_MB=512`, `SHARED_APPS_PER_COMPANY=2`, `SHARED_APP_MAX_MEMORY_MB=256`.
- Off-message, verbatim: "Os apps com tela não estão ligados para esta empresa. Peça ao administrador do Ziv para ligá-los." 
- Do not commit as part of planning; each task commits its own work.

## Review Focus

1. **Same numeric IDs in two companies.** App id 1 and deployment id 1 exist in both. A publish in company B must not remove, adopt or reuse A's box, image tag, network, database or data folder. Tested in Task 10 (`TestTwoCompaniesSameAppID`).
2. **A gateway code or session taken from company A to the same app id in company B.** It must be refused and the person sent to B's authorize page. Tested in Task 7 (`TestCodeFromOtherCompanyRefused`).
3. **Principal boxes after the upgrade.** Boxes with no workspace label, and with legacy labels `kova.app` and `app-hub.app`, stay principal's. A company supervisor's reconcile must never treat them as orphans. Tested in Task 2 (`TestScopedListPrincipalKeepsUnlabelled`) and Task 6 (`TestGroupReconcileDoesNotTouchOtherCompany`).
4. **Hostile hosts.** `x.principal.<d>`, `a.b.c.<d>`, a company slug that does not exist, upper case, and a host with a port must give a 404 and no Lookup. A random name must not get a certificate. Tested in Task 7 (`TestRouterRejects`, `TestAsk`).
5. **`folder_path` outside the company's code folder,** including through a symlink. Test, publish and the readiness check must refuse it before reading a single file. Tested in Task 4 (`TestFolderOutsideRootRefused`) and Task 8 (`TestCheckSharedRefusesForeignFolder`).

---

## File Structure

- Create `backend/internal/shared/tenant/tenant.go`: the company key and every derived name and path.
- Modify `backend/internal/shared/engine/engine.go`, `podman.go`, `fake.go`: workspace label, keyed networks.
- Create `backend/internal/shared/engine/scoped.go`: the per-company engine wrapper.
- Modify `backend/internal/shared/appdb/appdb.go`: prefix on the Fake.
- Modify `backend/internal/shared/deploy/deploy.go`, `lifecycle.go`, `deploytest/*.go`: tenant names, folder gate, limits.
- Modify `backend/internal/shared/supervisor/supervisor.go`. Create `supervisor/group.go`.
- Create `backend/internal/shared/gateway/router.go` (host to company, TLS `ask`). Delete `root.go`'s `Root`/`IsAppHost` and move their tests.
- Modify `backend/internal/handler/shared_gateway.go`, `server.go`, `shared.go`. Modify `frontend/src/pages/ServerPage.tsx`.
- Modify `backend/internal/workspace/portal.go`: `WithShared`, `Company`, `Load`, key collision.
- Create `backend/internal/shared/sharedrun/sharedrun.go`: builds one company's stack. Rewrite `backend/cmd/server/main.go` `startShared`.
- Modify `backend/internal/config/config.go`: four limits.
- Create `backend/internal/shared/deploy/slots.go`: the server-wide running limit.
- Modify `backend/internal/handler/admin.go`, `security.go` (`WorkspaceOptions`), `frontend/src/pages/AdminPage.tsx`: the per-company switch.
- Create `backend/internal/handler/server_folder.go`, and modify `versions.go`, `restore.go`, `apps.go`: server-folder gate for companies.
- Create `deploy/Caddyfile`. Modify `scripts/deploy-gcp.sh`.
- Create `docs/adr/025-apps-com-tela-por-empresa.md` (next free number). Mark ADR 015 decision 1 as superseded.

---

### Task 1: Company key and derived names (`tenant`)

**Files:**
- Create: `backend/internal/shared/tenant/tenant.go`, `backend/internal/shared/tenant/tenant_test.go`

**Interfaces:**
- Produces:
  - `type Tenant struct{ Slug, Key string }`
  - `func KeyOf(schema string) (string, error)`: `"public"` gives `""`. `tenant_<24 hex>` gives `"w"+hex[:11]`. Anything else is an error.
  - `func (t Tenant) Principal() bool`: true when `Key == ""`.
  - `func (t Tenant) AppsDomain(base string) string`: `base` for principal, `Slug+"."+base` otherwise.
  - `func (t Tenant) DataDir(root string) string`: `root` for principal, `filepath.Join(root,"workspaces",Key)` otherwise.
  - `func (t Tenant) DBPrefix() string`: `"app_"` for principal, `"app_"+Key+"_"` otherwise.
  - `func (t Tenant) ContainerName(appSlug string, depID int64, test bool) string`
  - `func (t Tenant) ImageTag(appSlug string, depID int64) string`: uses `engine.ImagePrefix`. Pass the prefix in as a constant copy to avoid an import cycle: tenant must not import engine. Define `const imagePrefix = "localhost/ziv/"` and test that it equals `engine.ImagePrefix` in the engine package (Task 2).
  - `var Principal = Tenant{Slug: "principal"}`

- [ ] **Step 1: Write the failing tests**

```go
func TestKeyOf(t *testing.T) {
	k, err := KeyOf("tenant_0123456789abcdef01234567")
	if err != nil || k != "w0123456789a" { t.Fatal(k, err) }
	if k, err := KeyOf("public"); err != nil || k != "" { t.Fatal(k, err) }
	for _, bad := range []string{"", "tenant_xyz", "tenant_0123456789ABCDEF01234567", "other"} {
		if _, err := KeyOf(bad); err == nil { t.Fatal(bad) }
	}
}
func TestPrincipalNamesUnchanged(t *testing.T) {
	p := Principal
	if p.AppsDomain("zivapp.duckdns.org") != "zivapp.duckdns.org" || p.DataDir("/d") != "/d" || p.DBPrefix() != "app_" ||
		p.ContainerName("notas", 7, false) != "ziv-notas-7" || p.ContainerName("notas", 7, true) != "ziv-notas-teste-7" ||
		p.ImageTag("notas", 7) != "localhost/ziv/notas:7" { t.Fatal("principal names changed") }
}
func TestCompanyNames(t *testing.T) {
	c := Tenant{Slug: "teste", Key: "w0123456789a"}
	if c.AppsDomain("zivapp.duckdns.org") != "teste.zivapp.duckdns.org" || c.DataDir("/d") != "/d/workspaces/w0123456789a" ||
		c.DBPrefix() != "app_w0123456789a_" || c.ContainerName("notas", 7, false) != "ziv-w0123456789a-notas-7" ||
		c.ContainerName("notas", 7, true) != "ziv-w0123456789a-notas-teste-7" ||
		c.ImageTag("notas", 7) != "localhost/ziv/w0123456789a/notas:7" { t.Fatal("company names") }
}
```

- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/shared/tenant/`. Expected: FAIL (package does not exist).
- [ ] **Step 3: Implement `tenant.go`** using the formulas in the Interfaces above.
- [ ] **Step 4: Run the same command.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: chave da empresa e nomes derivados"`

### Task 2: Engine knows the company (label, networks, scoped wrapper)

**Files:**
- Modify: `backend/internal/shared/engine/engine.go:52-81`, `podman.go:177-192` (`ensureNetwork`), `podman.go:254-256` (`Start`), `fake.go:136-160,281-297`
- Create: `backend/internal/shared/engine/scoped.go`, `backend/internal/shared/engine/scoped_test.go`

**Interfaces:**
- Consumes: none (the key is a plain string).
- Produces:
  - `const WorkspaceLabel = "ziv.workspace"`
  - `func WorkspaceOf(labels map[string]string) string`: `""` when absent.
  - `func AppNetwork(key string, appID int64) string`: `ziv-apps-<id>` when key is `""`, `ziv-apps-<key>-<id>` otherwise. This changes the signature: update every caller.
  - `LegacyAppNetworks(appID)`: unchanged. Callers use it only when the key is `""`.
  - `HubNetwork(name)` now also accepts `^ziv-apps-w[0-9a-f]{11}-[0-9]+$`.
  - `func Scoped(base Engine, key string) Engine`
  - `var ErrOtherWorkspace = errors.New("caixa de outra empresa")`
- Behavior of `Scoped`:
  - **`List`** returns only containers with `WorkspaceOf(labels) == key`.
  - **`Build` and `Start`** set `Labels[WorkspaceLabel] = key` when key is not empty. `Start` returns an error when `s.Network` is not a network this key owns.
  - **`Stop`, `Remove` and `Logs`** look the id up in `base.List`. Two IDs match when one is a prefix of the other and the shorter has at least 12 characters. If the id belongs to another key they return `ErrOtherWorkspace`. If it is not found they delegate, since a missing id is already a no-op.
  - **`RemoveNetwork`** refuses (`ErrOtherWorkspace`) a name this key does not own. Legacy prefixes count as owned only by key `""`.
  - **`Inspect`, `Stats`, `RemoveImage`, `Exec` and `Available`** pass through. Exec stays limited to `APPS_DB_CONTAINER` by Podman itself.
- `Podman.ensureNetwork(ctx, name string, labels map[string]string)` adds `--label ziv.workspace=<key>` when the spec carries it.

- [ ] **Step 1: Write the failing tests** in `scoped_test.go`, using `NewFake()` as the base:
  - `TestScopedListPrincipalKeepsUnlabelled`: start three boxes on the base Fake: one with labels `{ziv.app:1}`, one with `{app-hub.app:2}` and one with `{ziv.app:1, ziv.workspace:w0123456789a}`. Expect `Scoped(f,"").List` to return exactly the first two, and `Scoped(f,"w0123456789a").List` exactly the third.
  - `TestScopedRefusesOtherCompanysBox`: `Scoped(f,"wB").Remove(ctx, idOfA)`, `Stop` and `Logs` return `ErrOtherWorkspace`, and A's box is still in `f.Running()`.
  - `TestScopedStartLabelsAndChecksNetwork`: `Scoped(f,"wB").Start` with `Network: AppNetwork("wB",1)` stores `Labels["ziv.workspace"]=="wB"`. With `Network: AppNetwork("",1)` it returns an error.
  - `TestScopedRemoveNetwork`: `Scoped(f,"wB").RemoveNetwork(ctx,"ziv-apps-1")` and `"app-hub-apps-1"` both return `ErrOtherWorkspace`. `Scoped(f,"").RemoveNetwork(ctx,"ziv-apps-wB0000000000-1")` also returns `ErrOtherWorkspace`.
  - In `podman_guard_test.go` or a new test: `HubNetwork("ziv-apps-w0123456789a-5")` is true, `HubNetwork("ziv-apps-w0123-5")` is false, and `"localhost/ziv/" == ImagePrefix` (guards the copy in tenant).
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/shared/engine/`. Expected: FAIL to compile.
- [ ] **Step 3: Implement** `scoped.go`, the label and network changes, and the Fake support. Fake `Start` already keeps the spec labels; Fake `RemoveNetwork` uses `HubNetwork`. Update the `AppNetwork` callers in `deploy` to `engine.AppNetwork("", id)` so the build stays green. Task 4 replaces them.
- [ ] **Step 4: Run** `cd backend && mise x -- go test ./internal/shared/...`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: motor com rótulo de empresa e caixa restrita"`

### Task 3: App-database prefix on the Fake

**Files:**
- Modify: `backend/internal/shared/appdb/appdb.go:166-205` (Fake)
- Test: `backend/internal/shared/appdb/appdb_test.go`

**Interfaces:**
- Produces: `Fake.Prefix string`. `NewFake()` sets `"app_"`. `Fake.Name`/`TestName` use it, mirroring `Admin.Prefix` (which exists at `appdb.go:47`). The real `Admin` needs no change: sharedrun sets `Admin.Prefix = t.DBPrefix()`.

- [ ] **Step 1: Write the failing test** `TestPrefixedNames`:
  - `a := New(nil,"h",nil,"c","u"); a.Prefix = "app_w0123456789a_"`. Expect `a.Name(5)=="app_w0123456789a_5"` and `a.TestName(5,9)=="app_w0123456789a_5_test_9"`. Expect `safeName.MatchString` to be true for both (package-internal test).
  - `f := NewFake(); f.Prefix = "app_wX_"`. Expect `f.Name(5)=="app_wX_5"`.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/shared/appdb/ -run TestPrefixedNames`. Expected: FAIL (no `Prefix` on Fake).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: prefixo do banco no Fake"`

### Task 4: Deploy service per company (names, folder gate, limits)

**Files:**
- Modify: `backend/internal/shared/deploy/deploy.go` (`Options` at :43, `build` :340-357, `start` :361-379, `runPublish` :465, `Rollback` :666, `prepare` :203, `Configure` :153)
- Modify: `deploy/lifecycle.go` (`runApp` :72, `Unshare` :255-256, `Purge` :285-290)
- Modify: `deploy/deploytest/*.go`: add `func (e *Env) NewTenantService(t tenant.Tenant, folderRoot string, maxApps, maxMemMB int) *deploy.Service`. It uses `engine.Scoped(e.Eng, t.Key)` and `appdb.Fake` with `Prefix = t.DBPrefix()`.
- Modify: `backend/internal/database/queries/shared.sql`. Add:
  ```sql
  -- name: CountPublishedSharedApps :one
  SELECT count(*) FROM shared_apps WHERE unshared_at IS NULL AND current_deployment_id IS NOT NULL;
  ```
  Then run `sqlc generate`.
- Test: `backend/internal/shared/deploy/tenant_test.go`

**Interfaces:**
- Consumes: `tenant.Tenant` (Task 1). `engine.AppNetwork(key,id)` and `engine.Scoped` (Task 2). `appdb` prefix (Task 3).
- Produces:
  - `deploy.Options` gains:
    - `Tenant tenant.Tenant`. The zero value is principal-like (key `""`), so existing tests keep today's names.
    - `FolderRoot string`: `""` means any folder (principal).
    - `MaxApps int`: 0 means no limit.
    - `MaxMemoryMB int`: 0 means no limit.
  - `Options.Domain` keeps its meaning: the zone of this company's addresses. The caller passes `t.AppsDomain(cfg.SharedDomain)`. `AppURL` and `TestURL` stay unchanged.
  - `func (s *Service) CodeDir(appID int64) string`: `filepath.Join(DataDir,"code",<id>)`. This is where the later MCP plan writes stored code.
  - `func (s *Service) CheckFolder(path string) error`:
    - `nil` when `FolderRoot` is `""`.
    - Otherwise `filepath.EvalSymlinks(path)` must sit strictly inside `EvalSymlinks(FolderRoot)` (by prefix plus separator).
    - On failure it returns `ErrFolderOutside`.
  - New errors, verbatim:
    - `ErrFolderOutside = errors.New("Nesta empresa, um app com tela precisa ter o código guardado no Ziv. Suba o app pelo Claude Code (\"sobe esse programa para o Ziv\") e publique de novo.")`
    - `type LimitError struct{ Msg string }` with `Error() string`.
    - Too many apps: `fmt.Sprintf("Esta empresa já tem %d apps com tela publicados, o máximo do servidor. Deixe de compartilhar um deles para publicar outro.", MaxApps)`
    - Too much memory: `fmt.Sprintf("Nesta empresa cada app com tela pode usar até %d MB de memória. Diminua a memória do app e tente de novo.", MaxMemoryMB)`
- Behavior:
  - **Names.** `build` uses `s.opt.Tenant.ImageTag(j.sa.Slug, j.dep.ID)`. `start` uses `Tenant.ContainerName(...)` and `Network: engine.AppNetwork(s.opt.Tenant.Key, j.app.ID)`.
  - **Legacy networks.** `LegacyAppNetworks` loops (deploy.go:465, :666 and lifecycle.go:256, :286) run only when `s.opt.Tenant.Principal()`.
  - **Folder gate.** `CheckFolder(app.FolderPath)` runs first in `prepare`, in `runApp` and in `Rollback`, before any `recipes.Detect` or `stage.Copy`.
  - **Memory limit.** `Configure` returns the memory `LimitError` when `MaxMemoryMB > 0 && sh.MemoryMB > MaxMemoryMB`.
  - **App limit.** `prepare` (test and publish alike) returns the app-count `LimitError` when `MaxApps > 0`, the app has no `CurrentDeploymentID`, and `CountPublishedSharedApps >= MaxApps`. An app that is already published is never blocked.

- [ ] **Step 1: Write the failing tests** in `tenant_test.go`, with `c := tenant.Tenant{Slug: "teste", Key: "w0123456789a"}`:
  - `TestCompanyPublishUsesCompanyNames`:
    - Call `s := e.NewTenantService(c, "", 0, 0)`, then `s.Configure` and `s.Publish`, and wait.
    - `e.Eng.Running()` must equal `["ziv-w0123456789a-notas-<dep>"]`.
    - The container spec's `Network` must be `"ziv-apps-w0123456789a-<appID>"` and `Labels["ziv.workspace"]` must be `"w0123456789a"`.
    - The image tag must start with `"localhost/ziv/w0123456789a/notas:"`.
    - `sa.DbName` must be `"app_w0123456789a_<appID>"`.
    - `s.AppURL("notas")` must be `"http://notas.teste.localhost:8080"` (Options.Domain given as `c.AppsDomain("localhost:8080")`).
  - `TestPrincipalPublishNamesUnchanged`: the existing `TestPublishFirstTime` expectations still hold. They need no change; just run them.
  - `TestFolderOutsideRootRefused`: `root := t.TempDir()`, then `NewTenantService(c, root, 0, 0)`.
    - The app folder is the Env's (outside `root`). `Publish` and `Test` return `deploy.ErrFolderOutside`, and the `shared_deployments` count is 0.
    - A symlink `root/x -> e.App.FolderPath` is also refused.
    - With `FolderPath` set to `s.CodeDir(appID)` (CopyDir testdata there), `Publish` succeeds.
  - `TestMaxAppsBlocksSecondApp`: `MaxApps=1`. App 1 publishes. A second app (inserted like Setup with slug `outra`) gets `LimitError` on `Publish` and on `Test`. App 1 can publish again, which is an update.
  - `TestMaxMemoryOnConfigure`: `MaxMemoryMB=256` and manifest `MemoryMB: 512`. `Configure` returns `LimitError` whose message contains `"256 MB"`.
  - `TestCompanyUnshareKeepsPrincipalNetwork`: on the same Fake, start a principal box on `ziv-apps-<id>` through a principal service. Unshare in the company service. `e.Eng.HasNetwork("ziv-apps-<id>")` is still true.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/shared/deploy/ -run 'Company|Folder|Max|Principal'`. Expected: FAIL.
- [ ] **Step 3: Implement** the changes above, and run `mise x -- sqlc generate` for the new query.
- [ ] **Step 4: Run** `cd backend && mise x -- go test ./internal/shared/... ./internal/handler/`. Expected: PASS. All existing tests must stay green with no expectation changes.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: implantação com nomes, pasta e limites da empresa"`

### Task 5: Server-wide running limit (`Slots`)

**Files:**
- Create: `backend/internal/shared/deploy/slots.go`, `slots_test.go`
- Modify: `backend/internal/shared/deploy/deploy.go`:
  - `start`: add the label `ziv.memory_mb=<MemoryMB>`, and acquire a slot before `eng.Start`;
  - `runPublish` and `Rollback`: these replace the app's running box.
- Modify: `deploy/lifecycle.go` `runApp`: on a full server, set status `sleeping` with the message.
- Modify: `supervisor/supervisor.go` `restart`: a `LimitError` is not a restart try.
- Modify: `gateway/gateway.go`:
  - add `Target.Message`;
  - `notRunning`: for `sleeping` with a `Message`, call `Wake` and show "Servidor no limite" with the message and `Refresh: true`.
- Modify: `handler/shared_gateway.go` `Lookup`: copy `sa.StatusMessage` into `Target.Message` when the status is `sleeping`.

**Interfaces:**
- Consumes: the unscoped base engine (Task 2), `LimitError` (Task 4).
- Produces:
  - `const MemoryLabel = "ziv.memory_mb"` (in `engine`).
  - `func NewSlots(base engine.Engine, maxRunning, budgetMB int) *Slots`: zero means no limit on that axis, and a nil `*Slots` means no limit at all.
  - `func (s *Slots) Acquire(ctx context.Context, memMB int, replacing bool) (release func(), err error)`:
    - holds a mutex;
    - counts running containers from `base.List` (all companies), plus in-flight reservations;
    - sums their `MemoryLabel`, counting 512 when the label is missing (old principal boxes);
    - when `!replacing` and (count+1 > maxRunning or sum+memMB > budgetMB), returns `&LimitError{Msg: fmt.Sprintf("O servidor está no limite de apps com tela ligados ao mesmo tempo (%d). Tente de novo em alguns minutos: apps sem uso dormem sozinhos depois de 30 minutos.", maxRunning)}`.
    - `release` drops the reservation. The deploy service calls it right after the box started or failed, and from then on `List` counts the box itself.
  - `deploy.Options.Slots *Slots`. One instance is shared by every company's service (Task 10).
  - `replacing` is true only in `runPublish` when the app has a live box and in `Rollback`.

- [ ] **Step 1: Write the failing tests** in `slots_test.go`, on a Fake with two running boxes started with `MemoryLabel` 256:
  - `TestSlotsFullByCount`: `NewSlots(f,2,0).Acquire(ctx,128,false)` returns a `LimitError` containing `"(2)"`; `Acquire(ctx,128,true)` succeeds.
  - `TestSlotsFullByMemory`: with one box, `NewSlots(f,0,512).Acquire(ctx,300,false)` returns a `LimitError`, and `Acquire(ctx,256,false)` succeeds.
  - `TestSlotsCountsInflight`: with an empty Fake and `NewSlots(f,1,0)`, the first `Acquire` succeeds, the second (before `release`) fails, and after `release` it succeeds.
  - `TestSlotsCountsAllCompanies`: the boxes are labelled `ziv.workspace=wA` and `wB`; a service of company C with `Slots` max 2 gets a `LimitError` on `Publish`.
  - `TestSlotsMissingLabelCountsDefault`: one box without `MemoryLabel` and budget 600; acquiring 128 fails.
  - `TestWakeWhenFullSleepsWithMessage` (deploy): a published app is put to sleep; the slots are filled; `Run` returns a `LimitError`; the app's status is `sleeping` and its `StatusMessage` is the limit message.
  - `TestNotRunningShowsServerFull` (gateway): `Target{Status:"sleeping", Message:"…limite…"}` gives 503, the body contains "Servidor no limite", and `Wake` was called once.
  - `TestRestartWhenFullIsNotATry` (supervisor): when full, `restart` leaves `RestartCount` unchanged.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/shared/... -run 'Slots|WhenFull|ServerFull'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd backend && mise x -- go test ./internal/shared/... ./internal/handler/`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: limite de apps ligados no servidor inteiro"`

### Task 6: One supervisor group for all companies

**Files:**
- Modify: `backend/internal/shared/supervisor/supervisor.go:205-283` (split `Tick`)
- Create: `backend/internal/shared/supervisor/group.go`, `group_test.go`
- Modify: `backend/internal/database/queries/shared.sql`. Add:
  ```sql
  -- name: HasSharedApps :one
  SELECT EXISTS(SELECT 1 FROM shared_apps);
  ```
  Then run `sqlc generate`.

**Interfaces:**
- Consumes: the scoped engine (Task 2). Each company's `Supervisor` is built with its scoped engine, its own `q` and its own `svc`.
- Produces:
  - `func (s *Supervisor) Step(ctx context.Context)`: everything `Tick` did after the engine check. `Tick` becomes `Available` (with the `down` logging) followed by `Step`, so existing tests keep working.
  - `beforeStep func()`: an unexported test hook (like `beforeRestart`), run at the start of `Step`.
  - `func (s *Supervisor) HasApps(ctx context.Context) bool`: `q.HasSharedApps`. It returns true on error, so a failing check never skips the work.
  - `type Holder interface{ Hold(); Release() }`, satisfied by `*gateway.Gateway`.
  - `type Group struct{ Interval time.Duration /* 30s */ }`
  - `func NewGroup(eng engine.Engine) *Group`, where `eng` is the unscoped base and is used only for `Available`.
  - `func (g *Group) Add(s *Supervisor, h Holder)`:
    - calls `h.Hold()` at once and queues the pair for reconcile.
    - Safe to call from any goroutine, including while `Loop` runs (companies created or loaded later).
  - `func (g *Group) Loop(ctx context.Context)`: one goroutine.
    - **Reconcile the queue.** On start, on each `Add` (signalled through a buffered channel of size 1), and before every tick, it reconciles queued members in order:
      - if `HasApps` it runs `Reconcile` (a warning is logged on error);
      - it always calls `h.Release()` afterwards.
    - **Tick.** Every `Interval` it calls `eng.Available` once. If that fails it skips the round (logged once while down). Otherwise it runs `s.Step(ctx)` for each member with `HasApps(ctx)`, one after another.
- Trade-off: a 90 s restart in one company delays the other companies' ticks. Today that already happens between apps of one company. This is accepted to keep the 1 GB VM serial.

- [ ] **Step 1: Write the failing tests** in `group_test.go`. Use two supervisors on the same `engine.Fake`, each through `engine.Scoped(f, key)` and a deploytest-style service. The supervisor tests already have a DB fixture in `supervisor_test.go:25`. For the second company, use a second `sharedtest.Pool(ctx, "apphub_test_supervisor_b")` database.
  - `TestGroupReconcileDoesNotTouchOtherCompany`: company A publishes (box running, labelled `wA`). Group with A (`Scoped "wA"`) and B (`Scoped "wB"`, no apps) plus a principal supervisor (`Scoped ""`) whose DB expects nothing. After the first `Loop` pass, A's box is still in `f.Running()`.
  - `TestGroupHoldsUntilReconciled`: a fake `Holder` records calls. After `Add` the calls are `["hold"]`. After one pass they are `["hold","release"]`, also when `eng` is down. Use `f.SetDown(true)`.
  - `TestGroupSkipsCompaniesWithoutApps`: B has no rows. After one tick, `f.CallsCopy()` contains no `stats`/`inspect` for B. Specifically, `Step` is not called; assert with a counter by setting `s.beforeStep` (a test hook like `beforeRestart`) on B.
  - `TestGroupAddAfterStart`: `Add` while `Loop` runs. The new holder is released within 2 s.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/shared/supervisor/ -run Group`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd backend && mise x -- go test ./internal/shared/supervisor/`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: vigia único que percorre as empresas"`

### Task 7: Host router and TLS `ask`

**Files:**
- Create: `backend/internal/shared/gateway/router.go`, `router_test.go`
- Modify: `backend/internal/shared/gateway/root.go`. Keep `Hold`, `Release` and `Page`. Remove `IsAppHost` and `Root`, and move `root_test.go`'s cases to the router.
- Modify: `backend/internal/shared/gateway/gateway.go`: add `Known`.

**Interfaces:**
- Produces:
  - `func NewRouter(domain string) *Router`. `domain` is `SHARED_APPS_DOMAIN`, for example `zivapp.duckdns.org` or `localhost:8080`.
  - `func (r *Router) Set(company string, g *Gateway)`: `"principal"` is the one-label zone. A company's gateway must have been built with `New(company+"."+domain, ...)`.
  - `func (r *Router) SetLoader(fn func(ctx context.Context, company string) bool)`: called once for an unknown company. The portal attaches the company, and its hook calls `Set`.
  - `func (r *Router) Root(hub http.Handler) http.Handler`
  - `func (g *Gateway) Known(ctx context.Context, host string) bool`: `Match` succeeds and `Backend.Lookup` returns ok.
  - `const AskPath = "/__ziv/tls-ask"`
- Routing rules for `Root`:
  - **Ask requests.** A request whose Host is not under the domain, whose path is `AskPath`, whose RemoteAddr is loopback and which has no `X-Forwarded-For` gets 200 when `domain` (query param) routes to a gateway with `Known`, and 404 otherwise. Any other `AskPath` request also gets 404.
  - **App hosts.** Lowercase the Host and strip `"."+domain`. The rest splits on `.`:
    - one label: principal's gateway;
    - two labels `[app, company]`: `company` must match `^[a-z][a-z0-9-]{2,39}$` and must not be `"principal"`; use that company's gateway, calling the loader first if it is not set;
    - anything else, an unknown company, or no gateway yet: `Page(w, r, 404, "App não encontrado", "Este endereço não é de nenhum app compartilhado do Ziv.", "")`.
  - **Everything else** goes to `hub`.
- Codes and sessions stay inside each `Gateway`. Per-company isolation of codes comes from having one gateway per company, so no change to `codes.go` is needed.

- [ ] **Step 1: Write the failing tests** in `router_test.go`. Reuse `fakeBackend`/`setup` from `gateway_test.go`, with domain `localhost:8080`.
  - `TestRouterPrincipalHostUnchanged`: `notas.localhost:8080/` reaches the principal backend's `Lookup("notas", false)`.
  - `TestRouterCompanyHost`:
    - `notas.teste.localhost:8080` reaches company `teste`'s backend with `Lookup("notas", false)`;
    - `notas--teste.teste.localhost:8080` reaches it with `Lookup("notas", true)`;
    - the principal backend saw no Lookup.
  - `TestRouterRejects`: for `notas.principal.localhost:8080`, `a.b.c.localhost:8080`, `notas.ab.localhost:8080` (company slug too short) and `notas.nao-existe.localhost:8080` (loader returns false), the answer is 404 and no backend saw a Lookup. The loader was called exactly once, for `nao-existe`. `NOTAS.TESTE.LOCALHOST:8080` routes like the lowercase host.
  - `TestRouterLoadsCompany`: the loader calls `r.Set("nova", gw)` and returns true. The first request to `x.nova.localhost:8080` is served by `gw`.
  - `TestCodeFromOtherCompanyRefused`:
    - Gateways A (company `a`) and B (company `b`) each get a backend with app id 1 at slug `notas`.
    - `codeA := gwA.Codes.Issue(7, 1, false)`.
    - `GET notas.b.localhost:8080/__apphub/entrar?code=<codeA>` must answer 302 to B's hubURL `/api/shared/authorize`, with no `apphub_app` cookie set.
    - A session cookie obtained on A, sent to B's host, must also be redirected to authorize, not proxied.
  - `TestAsk`:
    - `GET http://127.0.0.1:8080/__ziv/tls-ask?domain=notas.teste.localhost:8080` from loopback gets 200 when B knows `notas`;
    - `?domain=zzz.teste.localhost:8080` gets 404;
    - the same request with `X-Forwarded-For: 1.2.3.4` gets 404;
    - a request from RemoteAddr `10.0.0.5:1` gets 404.
  - The moved root tests: hub hosts are served while held, and app hosts are held. `TestRootServesHubWhileHeldAndHoldsAppHosts` is rewritten against `Router.Root`.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/shared/gateway/`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: porteiro escolhe a empresa pelo endereço e responde ao ask do TLS"`

### Task 8: Handler: company-aware messages, server page, folder check

**Files:**
- Modify: `backend/internal/handler/shared_gateway.go:35-55` (`sharedReady`)
- Modify: `backend/internal/handler/server.go:23-130` (`getServer`, `putServerCost`)
- Modify: `backend/internal/handler/shared.go:120-147` (`sharedError`: `deploy.LimitError` and `ErrFolderOutside` answer 409 with their message) and `:275` (`readiness`)
- Modify: `frontend/src/pages/ServerPage.tsx`, `ServerPage.test.tsx`
- Test: `backend/internal/handler/shared_test.go`, `server_test.go`

**Interfaces:**
- Consumes: `deploy.ErrFolderOutside`, `deploy.LimitError`, `Svc.CheckFolder` (Task 4).
- Produces:
  - **`sharedReady`.** When `Svc` is nil, every company gets principal's message ("O servidor de apps não está ligado…"). The off-switch message is added in Task 9. The `code` stays `shared_runtime_unavailable`.
  - **`readiness(a, sa)`** first calls `h.shared.Svc.CheckFolder(a.FolderPath)`. On error, the `checkShared` handler answers 409 `{"error": ErrFolderOutside.Error(), "code": "shared_folder_outside"}` without scanning.
  - **`getServer`** outside principal (`h.workspace != nil && h.workspace.Slug != "principal"`):
    - answers `"company_only": true`;
    - leaves out `memory_total`, `memory_used`, `disk_total`, `disk_used` and `disk_warning`;
    - sends `monthly_cost_cents: 0`, `hub_cents: 0` and every app's `cost_cents: 0`;
    - in principal it sends `"company_only": false`.
  - **`putServerCost`** outside principal answers 403 "O custo do servidor é definido pela empresa principal."
  - **`ServerPage.tsx`**, when `company_only` is true:
    - hides the Memória/Disco meters, the cost editor and the cost column;
    - shows the line "O servidor é dividido com outras empresas. Aqui aparecem só os apps desta empresa."

- [ ] **Step 1: Write the failing tests:**
  - `TestCheckSharedRefusesForeignFolder` (handler): `withShared` with `deploy.Options{FolderRoot: t.TempDir()}`. The helper gets an options parameter or a variant `withSharedOpts`. `GET /api/apps/{id}/shared/check` answers 409 with code `shared_folder_outside`.
  - `TestServerPageCompanyOnly` (handler): a handler with `SetWorkspace(WorkspaceOptions{Slug:"teste"})` plus `withShared`. `GET /api/server` JSON has `company_only==true` and no key `memory_total`. `PUT /api/server/cost` answers 403.
  - `TestServerPageCompanyOnly` (vitest, `ServerPage.test.tsx`): the mocked response has `company_only: true`. Expect no "Memória" text, no cost input, and the line "O servidor é dividido com outras empresas" to be present.
  - Update `TestPortalSharedRuntimeFailsClosed` (`workspace/portal_test.go:311`) only where it asserts the old message text, if it does. It asserts the code, which stays.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/handler/ -run 'CheckSharedRefuses|ServerPageCompanyOnly'` and `cd frontend && mise x -- npx vitest run src/pages/ServerPage.test.tsx`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd backend && mise x -- go test ./internal/handler/` and the vitest command. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: mensagens, página do servidor por empresa e pasta do código"`

### Task 9: On/off per company, in principal's Admin tab

**Files:**
- Modify: `backend/internal/workspace/portal.go`: registry column, `SharedEnabled`, `Companies`, `SetCompanyShared`, and wiring them into `WorkspaceOptions` in `attach`.
- Modify: `backend/internal/handler/security.go:22` (`WorkspaceOptions`), `shared_gateway.go` (`sharedReady`, `Lookup`), `admin.go`, `handler.go` (routes).
- Modify: `backend/internal/shared/deploy/deploy.go` and `lifecycle.go`: `Options.Enabled`.
- Modify: `backend/internal/shared/supervisor/group.go`: skip off companies and put them to sleep.
- Modify: `backend/internal/shared/gateway/gateway.go` `notRunning`: a `disabled` case.
- Modify: `frontend/src/pages/AdminPage.tsx`, `AdminPage.test.tsx`.
- Test: `backend/internal/workspace/portal_test.go`, `handler/admin_test.go` (create it if absent), `deploy/tenant_test.go`, `supervisor/group_test.go`, `gateway/gateway_test.go`.

**Interfaces:**
- Consumes: Tasks 4–8.
- Produces:
  - **Registry.** In `Portal.New`, after the `CREATE TABLE`:
    - `ALTER TABLE public.workspace_registry ADD COLUMN IF NOT EXISTS shared_apps BOOLEAN`;
    - `UPDATE public.workspace_registry SET shared_apps = (slug = 'teste') WHERE shared_apps IS NULL`;
    - `ALTER TABLE public.workspace_registry ALTER COLUMN shared_apps SET DEFAULT FALSE`.
    - Existing rows are seeded once, and new companies start off.
  - **Portal methods:**
    - `func (p *Portal) SharedEnabled(ctx context.Context, slug string) bool`: always true for `"principal"`; otherwise `shared_apps`, with false on error (fails closed).
    - `func (p *Portal) Companies(ctx context.Context) ([]handler.CompanyShared, error)`: every ready company except principal, sorted by name.
    - `func (p *Portal) SetCompanyShared(ctx context.Context, slug string, on bool) error`: returns `handler.ErrWorkspaceNotFound` (new) for an unknown slug or `"principal"`.
  - `type CompanyShared struct{ Slug, Name string; SharedApps bool }` with JSON tags `slug`, `name`, `shared_apps`, in `handler/security.go`.
  - **`WorkspaceOptions` gains:**
    - `SharedEnabled func(context.Context) bool`, bound to the company's slug in `attach`;
    - `Companies` and `SetCompanyShared` (set only for principal; nil elsewhere).
  - **Handler routes:**
    - `GET /api/admin/companies`: 200 `[]CompanyShared`.
    - `PUT /api/admin/companies/{slug}/shared` with body `{"on": bool}`: 200 `CompanyShared`.
    - Both require `access.ManageRoles` and `h.workspace.Companies != nil`. Otherwise they answer 404 "Página não encontrada." (they do not exist outside principal).
    - The PUT records an audit line like other admin changes.
  - **`sharedReady`:** when `h.workspace.SharedEnabled` is set and returns false, answer the current 503 (JSON or `gateway.Page` for `/api/shared/authorize`) with the off-message from Global Constraints. The code stays `shared_runtime_unavailable`.
  - **Deploy:**
    - `deploy.Options.Enabled func(context.Context) bool` (nil means on).
    - `var ErrSharedOff = errors.New(<off-message>)`.
    - `begin` and `runApp` return `ErrSharedOff` when off. `Stop`, `Unshare`, `Backup` and `Sleep` keep working, so a company that is off can still clean up.
  - **Gateway:** `Lookup` returns `Target{Status: "disabled"}` when off. `notRunning` for `disabled` gives 503 "Apps com tela desligados" with the off-message, no `Wake` and no `Refresh`.
  - **Group:**
    - `Add(s *Supervisor, h Holder, enabled func(context.Context) bool)`. This changes Task 6's signature: update its tests.
    - When a member is off, it runs `s.SleepAll(ctx)` instead of `Step` (only if `HasApps`).
    - `func (s *Supervisor) SleepAll(ctx context.Context)`: calls `svc.Sleep` for every `running` or `crashed` app of the company.
  - **`AdminPage.tsx`:**
    - When `GET /api/admin/companies` answers 200, show a section "Empresas" with one row per company: the name, the slug and a switch "Apps com tela".
    - Turning it on asks for confirmation: "Ligar apps com tela para <nome>? Pessoas dessa empresa poderão rodar programas no servidor do Ziv."
    - On 404 the section is not shown.

- [ ] **Step 1: Write the failing tests:**
  - `TestRegistrySeedsSharedApps` (workspace):
    - Create the companies `teste` and `outra`, then drop the column (`ALTER TABLE public.workspace_registry DROP COLUMN shared_apps`) to simulate the old server.
    - Run `New` again.
    - Expect `SharedEnabled(ctx,"principal")`, `SharedEnabled(ctx,"teste")` and `!SharedEnabled(ctx,"outra")`.
    - Create a new company `nova`; expect `!SharedEnabled(ctx,"nova")`.
  - `TestCompanySharedOffAnswers503` (workspace, extends `TestPortalSharedRuntimeFailsClosed`): with `WithShared` wired to Fake deps and company `outra` off, `GET /w/outra/api/apps/1/shared` answers 503, the code is `shared_runtime_unavailable` and the body contains "Peça ao administrador do Ziv para ligá-los". After `SetCompanyShared(ctx,"outra",true)` the same request no longer answers 503.
  - `TestAdminCompaniesOnlyInPrincipal` (workspace):
    - the principal admin's `GET /api/admin/companies` lists `outra` with `shared_apps:false`;
    - `PUT /api/admin/companies/outra/shared {"on":true}` answers 200;
    - `GET /w/outra/api/admin/companies` answers 404, and so does `PUT /api/admin/companies/principal/shared`;
    - a non-admin in principal gets 403.
  - `TestPublishWhenOff` (deploy): with `Options.Enabled` returning false, `Publish` and `Run` return `ErrSharedOff`, and `Stop` still works.
  - `TestGroupSleepsOffCompany` (supervisor): a running app in a company whose `enabled` returns false is `sleeping` after one tick, and `Step` was not called.
  - `TestNotRunningDisabled` (gateway): `Status:"disabled"` gives 503, the body contains "ligá-los", and no `Wake` happens.
  - `AdminPage.test.tsx`: with the companies mocked, the switch renders, clicking it opens the confirmation, and confirming calls `PUT /api/admin/companies/outra/shared` with `{"on":true}`. A 404 means no "Empresas" heading.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/workspace/ ./internal/handler/ ./internal/shared/...` and `cd frontend && mise x -- npx vitest run src/pages/AdminPage.test.tsx`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run the same commands.** Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: chave liga/desliga por empresa no Admin da principal"`

### Task 10: Wire every company (portal hook, sharedrun, main)

**Files:**
- Modify: `backend/internal/workspace/portal.go` (`Portal` struct :38-49, `WithPrincipal` :54-59, `New` :102-125, `attach` :145-160, `create` :198-201, add `Load`)
- Create: `backend/internal/shared/sharedrun/sharedrun.go`, `sharedrun_test.go`
- Modify: `backend/cmd/server/main.go` (`main` :63-89 and `startShared` :105-182)
- Modify: `backend/internal/config/config.go`: add four fields, and add all four to the config's redacted dump at :113:
  - `SharedAppsPerCompany int` (env `SHARED_APPS_PER_COMPANY`, default 2);
  - `SharedAppMaxMemoryMB int` (env `SHARED_APP_MAX_MEMORY_MB`, default 256);
  - `SharedAppsMaxRunning int` (env `SHARED_APPS_MAX_RUNNING`, default 2);
  - `SharedAppsMemoryBudgetMB int` (env `SHARED_APPS_MEMORY_BUDGET_MB`, default 512).
- Modify: `backend/internal/workspace/portal_test.go:349` (`TestPortalPrincipalHookSkipsOtherCompanies` becomes `TestPortalSharedHookRunsForEveryCompany`)

**Interfaces:**
- Consumes: Tasks 1–9.
- Produces:
  - **workspace package:**
    - `type Company struct { Tenant tenant.Tenant; Name string; Pool *pgxpool.Pool; BaseURL string }`. `BaseURL` is `cfg.BaseURL` for principal and `cfg.BaseURL + "/w/<slug>"` otherwise; it is the same value `attach` already computes.
    - `func WithShared(fn func(Company, *handler.Handler)) Option`. It replaces `WithPrincipal`: delete `WithPrincipal`.
    - `attach` calls `fn` for every company whose key is unique among attached entries. On a duplicate it calls `slog.Error("shared apps off: company key collides", ...)` and skips the hook.
    - `entry` gains `key string`, set from `tenant.KeyOf(e.schema)`.
    - `create` regenerates the random schema, at most 5 tries, while `SELECT EXISTS(SELECT 1 FROM public.workspace_registry WHERE left(schema_name,18)=left($1,18))` is true.
    - `func (p *Portal) Load(ctx context.Context, slug string) bool`: an exported wrapper over `load` for the router's loader. Unknown or invalid slugs give false.
  - **sharedrun package:**
    - `type Deps struct { Engine engine.Engine; AdminPool *pgxpool.Pool; DBHost, DBContainer, DBAdminUser string; NewDBs func(prefix string) appdb.DBs /* tests; nil = appdb.New with Prefix */; Vault *vault.Vault; Router *gateway.Router; Group *supervisor.Group; DataRoot, Domain, Scheme string; Secure bool; MaxApps, MaxMemoryMB int; Slots *deploy.Slots; Enabled func(ctx context.Context, slug string) bool; HealthTimeout time.Duration }`
    - `func (d Deps) Attach(c workspace.Company, h *handler.Handler)`. For company `c` it:
      1. builds `eng := engine.Scoped(d.Engine, c.Tenant.Key)` and `dbs` with prefix `c.Tenant.DBPrefix()`;
      2. sets `dataDir := c.Tenant.DataDir(d.DataRoot)`;
      3. builds `svc := deploy.New(sqlc.New(c.Pool), eng, dbs, d.Vault, deploy.Options{DataDir: dataDir, Domain: c.Tenant.AppsDomain(d.Domain), Scheme: d.Scheme, Tenant: c.Tenant, FolderRoot: <"" for principal, filepath.Join(dataDir,"code") otherwise>, MaxApps/MaxMemoryMB: <0 for principal, d.* otherwise>, Slots: d.Slots, Enabled: func(ctx) bool { return d.Enabled(ctx, c.Tenant.Slug) }})`;
      4. calls `svc.SealOldBackups(ctx)`;
      5. builds `sup := supervisor.New(q, eng, svc)` and `gw := gateway.New(c.Tenant.AppsDomain(d.Domain), c.BaseURL, d.Secure, h.GatewayBackend())`;
      6. calls `h.SetShared(handler.Shared{Svc: svc, Sup: sup, Vault: d.Vault, DBs: dbs, Eng: eng, Codes: gw.Codes, DataDir: dataDir})`;
      7. calls `d.Router.Set(c.Tenant.Slug, gw)` and `d.Group.Add(sup, gw, <same enabled func>)`.
  - **`main.go`:**
    - `startShared` keeps the global, once-only steps: vault, container/data-dir adoption, `MoveBackupPaths`, `engine.NewPodman`, admin pool, `LockDown`. It returns `sharedrun.Deps`.
    - `main` then runs:
      - `router := gateway.NewRouter(cfg.SharedDomain)`;
      - `group := supervisor.NewGroup(eng)`;
      - `deps.Slots = deploy.NewSlots(eng, cfg.SharedAppsMaxRunning, cfg.SharedAppsMemoryBudgetMB)`;
      - `workspace.New(ctx, pool, cfg, workspace.WithShared(deps.Attach))`;
      - `deps.Enabled` is set to `portal.SharedEnabled` before any request is served. Use a small indirection, `var portal *workspace.Portal` captured by a closure, because `Attach` runs inside `New`; until `New` returns, the closure answers false except for principal;
      - `router.SetLoader(portal.Load)`, `go group.Loop(ctx)`, and `Handler: router.Root(portal)`.
    - The old `gw.Hold()`/Reconcile goroutine is deleted; the Group does that work.
  - **Guarantee for later plans:** after this task, every company handler built by `cmd/server` has `h.shared.Svc`, `Sup`, `Eng`, `DBs`, `Codes` and `DataDir` set (with `Vault` possibly nil without `HUB_SECRET_KEY`). `Svc.Configure`, `Test`, `Publish`, `Rollback`, `Start`, `Stop`, `Unshare` and `Backup` work for that company's apps, isolated from every other company.

- [ ] **Step 1: Write the failing tests:**
  - `TestPortalSharedHookRunsForEveryCompany` (workspace):
    - Set up principal and create `other-company`.
    - `New(..., WithShared(func(c Company, _ *handler.Handler){ got = append(got, c) }))` gives `len(got)==2`.
    - Principal has `Key ""` and `BaseURL "http://localhost"`.
    - The other company has a `Key` matching `^w[0-9a-f]{11}$` and `BaseURL "http://localhost/w/other-company"`.
  - `TestPortalLoadUnknown` (workspace): `p.Load(ctx,"nao-existe")==false` and `p.Load(ctx,"BAD!")==false`.
  - `TestTwoCompaniesSameAppID` (sharedrun):
    - Use `testPortal`-like setup. Copy the bootstrap from `workspace/portal_test.go:36` into a small helper in sharedrun tests, with its own database name `apphub_test_sharedrun`.
    - Use `Deps{Engine: engine.NewFake(), NewDBs: func(p string) appdb.DBs { f := appdb.NewFake(); f.Prefix = p; return f }, Router: gateway.NewRouter("localhost:8080"), Group: supervisor.NewGroup(fake), Vault: <vault.New(vault.NewKey())>, DataRoot: t.TempDir(), Domain: "localhost:8080", Scheme: "http", MaxApps: 2, MaxMemoryMB: 256, Enabled: func(context.Context, string) bool { return true }, HealthTimeout: time.Second}`.
    - Create company `teste`. In principal and in `teste`, insert app id 1 slug `notas`, with the folder copied from `testdata/shared/streamlit`. For `teste`, copy it into `<DataRoot>/workspaces/<key>/code/1`.
    - Publish both through each company's API (`POST /api/apps/1/shared/publish` and `/w/teste/api/apps/1/shared/publish`).
    - Assert:
      - the Fake runs exactly `ziv-notas-<d1>` and `ziv-<key>-notas-<d2>`;
      - the networks are `ziv-apps-1` and `ziv-apps-<key>-1`;
      - the databases are `app_1` and `app_<key>_1`;
      - stopping in `teste` (`POST /w/teste/api/apps/1/shared/stop`) leaves principal's box running;
      - `GET` with Host `notas.localhost:8080` and with Host `notas.teste.localhost:8080` through `router.Root(portal)` each reach their own company's gateway: without a session, each redirects to its own `.../api/shared/authorize` (`http://localhost/api/...` vs `http://localhost/w/teste/api/...`);
      - after a backup in each company (`POST .../shared/backups`), the files live under `<DataRoot>/backups/1/` and `<DataRoot>/workspaces/<key>/backups/1/` respectively.
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/workspace/ ./internal/shared/sharedrun/`. Expected: FAIL.
- [ ] **Step 3: Implement** the portal, sharedrun, config and main changes.
- [ ] **Step 4: Run** `cd backend && mise x -- go vet ./... && mise x -- go test ./...`. Expected: PASS. Also run `mise x -- go build ./cmd/server`. Then run the app locally (`cofounder:tech-stack` "run the app locally"; not on 8080 if `alis-research-hub-v2` holds it). Publish the streamlit testdata app in principal (expect `notas.localhost:<port>` to open) and in a test company (expect `notas.<empresa>.localhost:<port>` to open), and run `podman ps --format '{{.Names}} {{.Labels}}'` to see the two name patterns.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: liga os apps com tela em todas as empresas"`

### Task 11: Companies cannot read or write server folders (versions, restore, app files)

The security gap: `handler/versions.go:36` `folderOf` reads `app.FolderPath` on the server for any app. The version scan (`checkVersionLocked` → `versions.Scan`, triggered by `POST /api/apps/{id}/check` and `writeApp`) stores the files' contents. `GET /api/apps/{id}/versions/{n}/content` then serves them. `restore.go:61,127` writes into the folder. A person in any company who registers an app with `folder_path` `/home/ziv/ziv` could read `ziv.env` (with `HUB_SECRET_KEY`), or overwrite server files through a restore.

**Files:**
- Create: `backend/internal/handler/server_folder.go`, `server_folder_test.go`
- Modify: `backend/internal/handler/versions.go:36` (`folderOf` becomes a method that can fail), and its callers: `versions.go:100`, `restore.go:61`, `restore.go:127`.
- Modify: `apps.go:435` (`os.Stat` of the folder on update) and every other server-disk read or write of `a.FolderPath` or `body.FolderPath` found in Step 0.

**Interfaces:**
- Consumes: `Svc.CodeDir` (Task 4).
- Produces:
  - `var errFolderNotOnServer = errors.New("O código deste app não está guardado no Ziv, então o Ziv não abre a pasta dele no servidor.")`
  - `func (h *Handler) serverFolder(a sqlc.App) (versions.Folder, error)`. Principal (`h.workspace == nil || h.workspace.Slug == "principal"`) gets `versions.LocalFolder{Root: a.FolderPath}` as today. Any other company is allowed only when `filepath.EvalSymlinks(a.FolderPath)` is either:
    - a direct child of `EvalSymlinks(filepath.Join(os.TempDir(), "apphub-imports"))` whose base name matches `^[A-Za-z0-9_-]{43}$` (the folder `script_import.go:130` creates; the 256-bit token is never shown to another company); or
    - inside `h.shared.Svc.CodeDir(a.ID)` when `h.shared.Svc != nil`.
  - Otherwise it returns `errFolderNotOnServer`, and endpoints answer 409 `{"error": ..., "code": "folder_not_on_server"}`.
  - The version check in `writeApp` and `checkApp` treats `errFolderNotOnServer` like an unreadable folder: no version is recorded and the app is still saved. That is today's behavior for a PC path the server cannot see.

- [ ] **Step 0: Audit.** Run `grep -n "FolderPath\|folderOf\|scanner.Scan\|os.Stat\|ReadFromFolder\|WriteToFolder" backend/internal/handler/*.go | grep -v _test`. Every hit that touches the server disk with a client-controlled path in a non-principal company goes through `serverFolder`, or `filepath`-checks the same way. List the hits and the decision for each in the commit message. `hydrateImportedScript` and `script_import.go` write only to the folder they created, so they stay.
- [ ] **Step 1: Write the RED tests** in `server_folder_test.go`. Use a handler with `SetWorkspace(WorkspaceOptions{Slug:"teste"})` and a temp dir holding `segredo.txt` with content `"ZIV_SEGREDO_123"`:
  - `TestCompanyCannotReadServerFolderThroughVersions`:
    - register an app with `folder_path` set to that dir;
    - call `POST /api/apps/{id}/check`;
    - expect no `app_versions` row for the app;
    - `GET /api/apps/{id}/versions/1/content?path=segredo.txt` answers 404 or 409, and the body does not contain `ZIV_SEGREDO_123`.
  - `TestCompanyCannotRestoreIntoServerFolder`: insert an `app_versions` row for an app whose `folder_path` is the temp dir. `POST /api/apps/{id}/versions/1/restore` answers 409 `folder_not_on_server`, and the dir is unchanged (same files, same bytes).
  - `TestCompanySymlinkIntoImportsRefused`: `<imports>/<43-char token>` is a symlink to the secret dir, and it is refused.
  - `TestCompanyImportedScriptStillVersions`: a script imported through `POST /api/scripts/import` in the company still gets version 1 and its content.
  - `TestPrincipalFolderUnchanged`: in principal, the same secret-dir app gets version 1 as today (no behavior change for the owner's team).
- [ ] **Step 2: Run** `cd backend && mise x -- go test ./internal/handler/ -run 'CannotReadServerFolder|CannotRestore|SymlinkIntoImports|ImportedScriptStill|PrincipalFolderUnchanged'`. Expected: the first three FAIL (RED: the secret is read and the restore writes), and the last two PASS.
- [ ] **Step 3: Implement** `serverFolder` and switch every caller found in Step 0.
- [ ] **Step 4: Run** `cd backend && mise x -- go test ./internal/handler/ ./internal/workspace/`. Expected: PASS.
- [ ] **Step 5: Commit:** `git commit -m "Segurança: empresas não leem nem escrevem pastas do servidor pelas versões"`

### Task 12: Production (Caddy `ask`, deploy, ADR) and the isolation check on the VM

**Files:**
- Create: `deploy/Caddyfile`, the source of truth from now on. First copy the VM's current file (`ssh -i ~/.ssh/ziv-gcp ziv@34.24.21.104 cat /etc/caddy/Caddyfile`) and change only what is below.
- Modify: `scripts/deploy-gcp.sh`: a step 5 that copies `deploy/Caddyfile` and runs `sudo caddy validate` and `sudo systemctl reload caddy`, only when the file changed (compare with `ssh ... sudo cat`).
- Create: `docs/adr/025-apps-com-tela-por-empresa.md` (use the next free number if 025 is taken by then).
- Modify: `docs/adr/015-*.md`: add a line under decision 1, "Substituída pela ADR 025". Modify `docs/adr/022-*.md`: replace the "O `ask` … aceita qualquer subdomínio" consequence with a pointer to ADR 025.

**Interfaces:**
- Consumes: `gateway.AskPath` (Task 7) and the four limits (Task 10), which keep their defaults unless Step 2's measurement says otherwise.
- Produces:
  - **Caddy.** `on_demand_tls { ask http://localhost:8080/__ziv/tls-ask }`. The on-demand site block matches any host under `zivapp.duckdns.org`. A `*.zivapp.duckdns.org` wildcard matches one label only, so use a catch-all `https://` block with `tls { on_demand }`, or add `*.*.zivapp.duckdns.org`. Every site block has `respond /__ziv/* 404`.
  - **ADR 025** records:
    - the key and name table;
    - the subdomain format and its reasons;
    - the spec §2.3 ruling;
    - the principal migration and rollback hazard;
    - the on/off switch (who, where, default, what "off" does);
    - the company limits and the server-wide running limit, with the memory measurement from Step 2 and the arithmetic behind 2 × 256 MB;
    - the serial supervisor group trade-off;
    - the VM isolation result from Step 3.

- [ ] **Step 1: Write the ADR and the Caddyfile.** Run `caddy validate --config deploy/Caddyfile` locally if caddy is installed, otherwise on the VM in Step 2. Expected: "Valid configuration".
- [ ] **Step 2: Deploy** with `bash scripts/deploy-gcp.sh`. Expected output: `site: 200`. Then check:
  - `curl -s -o /dev/null -w '%{http_code}' 'http://localhost:8080/__ziv/tls-ask?domain=zzz.teste.zivapp.duckdns.org'` (over ssh on the VM) answers `404`;
  - `curl -sI https://zivapp.duckdns.org/__ziv/tls-ask` answers `404`;
  - an existing principal app link still opens;
  - with no shared app running, `free -m` and `ps -eo rss,comm --sort=-rss | head` on the VM. Write the numbers into ADR 025. If less than 512 MB is available, lower `SHARED_APPS_MEMORY_BUDGET_MB` in `~/ziv/ziv.env` to what is free minus 64 MB, and note it.
- [ ] **Step 3: VM isolation check, on the VM.** Publish a tiny app in company Teste (the streamlit testdata app, uploaded into its code folder by hand for this check). From inside its box, run `podman exec <teste box> python -c "import urllib.request;print(urllib.request.urlopen('http://host.containers.internal:<principal box host port>/').status)"`, taking the port from `podman port <principal box>`.
  - **Expected: the connection is refused or times out.** Record the result in ADR 025.
  - **If it connects,** a company's box can reach another company's app without the gateway and send forged `X-AppHub-*` headers. Stop and report it to the product owner (see "Needs the product owner" below) before announcing the feature. The fix (gateway reaching boxes over their own network instead of a host port, or an `nft` rule on the rootless network) is a follow-up plan.
  - Also check `nc -zv host.containers.internal 8080` and `5433` from the box, and record what answers.
- [ ] **Step 4: Henry's hands-on test in company Teste** (one action per question; check the result through Hub data):
  1. Open an existing principal app by its usual link: it still opens.
  2. In principal's Admin tab, "Empresas" shows Teste switched on and every other company off.
  3. In company Teste, publish the test app: the card shows `https://<app>.teste.zivapp.duckdns.org` and it opens with a valid certificate.
  4. Stop it in Teste: principal's app stays up.
  5. On "Servidor e gastos" in Teste, only Teste's apps show, with no server cost.
  6. Switch Teste off in Admin: the app's address shows "Apps com tela desligados". Switch it back on.
- [ ] **Step 5: Commit:** `git commit -m "Apps por empresa: Caddy com ask, ADR 025 e implantação"`

---

## Interfaces for the later plan (MCP registration of screen apps)

After this plan:
- **Every company handler has the shared stack.** Every company handler created by `cmd/server` (principal and every other company, including ones created or loaded after boot) has `h.shared.Svc`, `h.shared.Sup`, `h.shared.Eng`, `h.shared.DBs`, `h.shared.Codes` and `h.shared.DataDir` set. `h.shared.Vault` is nil only without `HUB_SECRET_KEY`.
- **Every operation works in every company.** `Svc.Configure`, `Svc.Test`, `Svc.Publish`, `Svc.Rollback`, `Svc.Start`, `Svc.Stop`, `Svc.Unshare` and `Svc.Backup` work for apps of that company, and only that company. A background publish keeps running after the request returns: poll `GET /api/apps/{id}/shared/deployments/{dep}`.
- **Code location.** Outside principal, the app's `folder_path` must be `h.shared.Svc.CodeDir(appID)` or below. The MCP plan writes the stored code there (mode 0700) before calling `Configure`/`Publish`. Otherwise `Test`, `Publish` and `/shared/check` return `deploy.ErrFolderOutside` (HTTP 409 `shared_folder_outside`). In principal any folder works, as today.
- **App address.** `h.shared.Svc.AppURL(slug)` and `TestURL(slug)` give the correct address for the company. Never build the link by hand.
- **On/off.** In a company that is off, `Svc.Test`, `Publish` and `Start` return `deploy.ErrSharedOff`, and the shared API answers 503 `shared_runtime_unavailable` with the off-message. The MCP tool must relay that message ("peça ao administrador do Ziv") and not retry. `h.workspace.SharedEnabled(ctx)` tells it up front.
- **Limits.** These errors carry a Portuguese message ready to show in the conversation:
  - a first publish or test can fail with `deploy.LimitError` (too many apps per company, or the server is full);
  - `Configure` can fail with the memory `LimitError`.
- **Server folders.** Outside principal, the Ziv reads or writes an app folder on the server only through `h.serverFolder(a)`. The MCP plan's stored code must live in `Svc.CodeDir(appID)` to be versioned and published.

## Needs the product owner

Decided on 2026-11-06: on/off per company, owner-only (Task 9); limits re-sized to the VM (Task 5); server-folder gap fixed (Task 11); nested subdomain accepted.

Still open: the result of Task 12 Step 3. If a box reaches another app's host port, the fix comes before turning any company other than Teste on.
