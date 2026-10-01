# Etapa 1 — Rodar em poucos cliques — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Colega roda um app com 1 clique no cartão do catálogo, acompanha por uma faixa fixa embaixo e reencontra tudo em Minhas execuções; a ficha abre na hora.

**Architecture:** Backend Go ganha: conferência da pasta separada (`POST /api/apps/{id}/check`), pasta lembrada por pessoa e app (`run_folders`), execuções dispensáveis (`runs.dismissed_at`) com `GET /api/runs/active`, `POST /api/runs/{id}/dismiss` e a lista `GET /api/runs`. Frontend ganha um `RunsProvider` (contexto React no `Layout`) que é o único lugar que dispara execuções, abre a janela da pasta quando precisa, consulta as ativas e desenha a faixa.

**Tech Stack:** Go stdlib `net/http`, pgx/v5, sqlc, migrações embutidas; React + TypeScript + Tailwind + shadcn/ui; Vitest + Testing Library; Playwright para capturas.

**Spec:** `docs/superpowers/specs/2026-10-01-etapa-1-rodar-poucos-cliques-design.md` (protótipo: `docs/superpowers/specs/2026-10-01-prototipo-ux.html`)

## Global Constraints

- Todo texto de tela em português simples, sem jargão (é o que o Henry e os colegas leem).
- SQL só em `backend/internal/database/queries/*.sql` + `sqlc generate`; nunca SQL no handler. Ler os `.go` gerados antes de usar os nomes.
- Migração nova: `backend/internal/database/migrations/006_run_prefs.sql`, idempotente (`IF NOT EXISTS`).
- Comandos Go/sqlc rodam de `backend/` via `mise x --`; frontend via `mise x -- npm` em `frontend/`.
- Banco local: container `app-hub-db`, porta 5433. **Nunca subir um Hub de teste na porta 8080** (o Agente real do Henry fala com ele e esquece a conexão). Testes Go usam o banco de teste já configurado em `testdb_test.go`.
- Quem vê uma execução não muda: quem rodou e administradores. Só quem rodou dispensa a própria faixa.
- Conferência antes de rodar e ao publicar continua como está (ADR 006).
- Botão Rodar só para apps com `can_run` (hoje: `desktop` com `.exe`).
- Faixa: no máximo 3 visíveis + "+N" para Minhas execuções. Consulta a cada 3 s com execução aberta, 30 s sem.
- Minhas execuções: 50 por página.

## Review Focus

1. **Pasta lembrada que não existe mais / Agente desligado no Rodar de 1 clique** — o erro tem que aparecer na faixa com o próximo passo (trocar pasta / Ver Meu computador), nunca sumir em silêncio. Teste no Task 6.
2. **Recarregar a página com um app aberto** — a faixa volta igual (vem do servidor), e uma execução antiga, de antes da migração, nunca aparece como faixa. Testes no Task 1 (backfill) e Task 4 (`runs/active`).
3. **Duas abas / dois cliques rápidos** — cada clique é uma execução e cada uma vira sua faixa; o botão fica desabilitado enquanto o pedido está saindo. Teste no Task 6.
4. **Colega (perfil usuário) tentando `scope=all` ou dispensar execução de outra pessoa** — 403/404. Testes no Task 4.
5. **Conferência em segundo plano falha ou demora** — a ficha já está na tela; só o aviso de pasta inacessível aparece depois, e o histórico recarrega só se a versão mudou. Teste no Task 7.

---

## File Structure

**Backend**
- Create `backend/internal/database/migrations/006_run_prefs.sql` — tabela `run_folders`, coluna `runs.dismissed_at` e backfill.
- Modify `backend/internal/database/queries/runs.sql` — `ListActiveRuns`, `DismissRun`, `ListRuns`, `GetRunFolder`, `ListRunFolders`, `SetRunFolder`, `ClearRunFolder`.
- Modify `backend/internal/database/queries/apps.sql` — `ListApps` devolve `entrypoint`.
- Modify `backend/internal/handler/apps.go` — `getApp` sem conferência, `checkApp` novo, `listApps` com `can_run` e `remembered_folder`.
- Modify `backend/internal/handler/history.go` — `appHistory` sem conferência.
- Modify `backend/internal/handler/runs.go` — `canRunKind`, `startRun` com `remember`, `activeRuns`, `dismissRun`, `listRuns`.
- Modify `backend/internal/handler/handler.go` — rotas novas.
- Tests: `apps_test.go`, `versions_test.go`, `history_test.go`, `runs_test.go`, novo `runs_list_test.go`.

**Frontend**
- Modify `frontend/src/lib/api.ts` — tipos e chamadas novas.
- Create `frontend/src/lib/runs-context.tsx` — `RunsProvider`, `useRuns()`.
- Create `frontend/src/components/RunStrip.tsx` — a faixa.
- Modify `frontend/src/components/RunDialog.tsx` — caixinha "Lembrar esta pasta"; deixa de navegar, devolve a escolha.
- Modify `frontend/src/components/Layout.tsx` — `RunsProvider`, `RunStrip`, item Minhas execuções com contador.
- Modify `frontend/src/pages/CatalogPage.tsx` — botão Rodar no cartão.
- Modify `frontend/src/pages/AppPage.tsx` — ficha na hora + `check` em segundo plano + "salva em … · trocar".
- Modify `frontend/src/components/HistoryTimeline.tsx` — prop `reloadKey`.
- Create `frontend/src/pages/MyRunsPage.tsx` — Minhas execuções.
- Modify `frontend/src/pages/RunPage.tsx` — Rodar de novo, voltar para Minhas execuções.
- Modify `frontend/src/App.tsx` — rota `/execucoes`.
- Tests: `lib/runs-context.test.tsx`, `components/RunStrip.test.tsx`, `pages/CatalogPage.test.tsx`, `pages/MyRunsPage.test.tsx`, ajustes em `AppPage.test.tsx` e `RunPage.test.tsx`; `test/fixtures.ts` ganha `activeRun()` e `appSummary()`.

---

### Task 1: Migração e consultas

**Files:**
- Create: `backend/internal/database/migrations/006_run_prefs.sql`
- Modify: `backend/internal/database/queries/runs.sql`, `backend/internal/database/queries/apps.sql`
- Generated: `backend/internal/database/sqlc/*`
- Test: `backend/internal/handler/runs_list_test.go`

**Interfaces:**
- Produces (sqlc, confirme os nomes exatos no gerado): `q.ListActiveRuns(ctx, personID int64) ([]ListActiveRunsRow, error)`; `q.DismissRun(ctx, DismissRunParams{ID, PersonID}) (int64, error)`; `q.ListRuns(ctx, ListRunsParams{SeeAll bool, PersonID int64, Before int64, Lim int32}) ([]ListRunsRow, error)`; `q.GetRunFolder(ctx, GetRunFolderParams{PersonID, AppID}) (string, error)`; `q.ListRunFolders(ctx, personID) ([]ListRunFoldersRow, error)`; `q.SetRunFolder(ctx, SetRunFolderParams{PersonID, AppID, Folder})`; `q.ClearRunFolder(ctx, ClearRunFolderParams{PersonID, AppID})`; `ListAppsRow.Entrypoint string`.

- [ ] **Step 1: Escrever a migração**

```sql
-- 006: pasta lembrada por pessoa e app, e faixas de execução dispensáveis.
CREATE TABLE IF NOT EXISTS run_folders (
    person_id  BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    folder     TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (person_id, app_id)
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'runs' AND column_name = 'dismissed_at') THEN
        ALTER TABLE runs ADD COLUMN dismissed_at TIMESTAMPTZ;
        -- Execuções de antes desta etapa não viram faixa.
        UPDATE runs SET dismissed_at = now();
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS runs_person_idx ON runs(person_id, id DESC);
```

- [ ] **Step 2: Escrever as consultas** (acrescentar ao fim de `queries/runs.sql`)

```sql
-- name: ListActiveRuns :many
SELECT r.id, r.app_id, a.name AS app_name, r.status, r.error_message, r.created_at, r.finished_at,
       (SELECT count(*) FROM run_files f WHERE f.run_id = r.id) AS file_count
FROM runs r
JOIN apps a ON a.id = r.app_id
WHERE r.person_id = $1 AND r.dismissed_at IS NULL
ORDER BY r.id DESC
LIMIT 10;

-- name: DismissRun :execrows
UPDATE runs SET dismissed_at = now()
WHERE id = $1 AND person_id = $2 AND dismissed_at IS NULL;

-- name: ListRuns :many
SELECT r.id, r.app_id, a.name AS app_name, r.status, r.output_folder, r.created_at, r.started_at, r.finished_at,
       r.person_id, p.name AS person_name,
       (SELECT count(*) FROM run_files f WHERE f.run_id = r.id) AS file_count
FROM runs r
JOIN apps a ON a.id = r.app_id
JOIN people p ON p.id = r.person_id
WHERE (sqlc.arg(see_all)::boolean OR r.person_id = sqlc.arg(person_id))
  AND (sqlc.arg(before)::bigint = 0 OR r.id < sqlc.arg(before)::bigint)
ORDER BY r.id DESC
LIMIT sqlc.arg(lim);

-- name: GetRunFolder :one
SELECT folder FROM run_folders WHERE person_id = $1 AND app_id = $2;

-- name: ListRunFolders :many
SELECT app_id, folder FROM run_folders WHERE person_id = $1;

-- name: SetRunFolder :exec
INSERT INTO run_folders (person_id, app_id, folder) VALUES ($1, $2, $3)
ON CONFLICT (person_id, app_id) DO UPDATE SET folder = EXCLUDED.folder, updated_at = now();

-- name: ClearRunFolder :exec
DELETE FROM run_folders WHERE person_id = $1 AND app_id = $2;
```

Em `queries/apps.sql`, `ListApps` passa a ser:

```sql
-- name: ListApps :many
SELECT a.id, a.slug, a.name, a.summary, a.kind, a.language, a.area, a.folder_path,
       a.owner_id, o.name AS owner_name, a.backup_id, a.created_at, a.updated_at,
       COALESCE(a.manifest->>'entrypoint', '')::text AS entrypoint
FROM apps a
JOIN people o ON o.id = a.owner_id
ORDER BY a.name;
```

- [ ] **Step 3: Gerar e ler o gerado**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- sqlc generate'`
Depois ler `backend/internal/database/sqlc/runs.sql.go` e `apps.sql.go` e anotar os nomes reais (`Lim` pode sair como `Lim int32`; `FileCount int64`).

- [ ] **Step 4: Teste de que a migração marca as execuções antigas** (`runs_list_test.go`)

```go
package handler

import (
	"context"
	"testing"
)

func TestMigrationColumnsExist(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns WHERE table_name = 'runs' AND column_name = 'dismissed_at'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("dismissed_at: %d %v", n, err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM run_folders`).Scan(&n); err != nil {
		t.Fatalf("run_folders: %v", err)
	}
}
```

(O backfill roda uma vez, no primeiro `ALTER`; não dá para reproduzir num banco de teste já migrado. Ele é conferido de verdade no Task 10, olhando que a faixa não mostra as execuções antigas do Henry.)

- [ ] **Step 5: Rodar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go build ./... && mise x -- go test ./internal/handler -run TestMigrationColumnsExist -v'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/database && git add backend/internal/handler/runs_list_test.go
git commit -m "Runs: remembered folders table, dismissable runs, list queries"
```

---

### Task 2: Ficha sem esperar a conferência

**Files:**
- Modify: `backend/internal/handler/apps.go` (`getApp`, novo `checkApp`), `backend/internal/handler/history.go:121-126`, `backend/internal/handler/handler.go`
- Test: `backend/internal/handler/versions_test.go`, `history_test.go`, `apps_test.go`

**Interfaces:**
- Produces: `POST /api/apps/{id}/check` → `200 {"folder_readable": bool, "version": {"number": int, "created_at": time} | null}`. `GET /api/apps/{id}` devolve `version` = última registrada, `folder_readable: true` e `remembered_folder: string` (pasta lembrada da pessoa logada, ou `""`).

- [ ] **Step 1: Teste que falha** (em `versions_test.go`)

```go
func TestGetAppDoesNotCheckFolderButCheckDoes(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c) // publicar registra a versão 1
	editFile(t, dir, "src/main.py", "print('v2')\n")

	var d appDetail
	c.do("GET", appPath(id, ""), nil, &d)
	if d.Version == nil || d.Version.Number != 1 {
		t.Fatalf("GET não deve conferir a pasta: %+v", d.Version)
	}
	var chk struct {
		FolderReadable bool        `json:"folder_readable"`
		Version        *versionRef `json:"version"`
	}
	if code := c.do("POST", appPath(id, "/check"), nil, &chk); code != http.StatusOK || !chk.FolderReadable || chk.Version == nil || chk.Version.Number != 2 {
		t.Fatalf("check %d %+v", code, chk)
	}
}

func TestCheckWithMissingFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	_ = os.RemoveAll(dir)
	var chk struct {
		FolderReadable bool `json:"folder_readable"`
	}
	if code := c.do("POST", appPath(id, "/check"), nil, &chk); code != http.StatusOK || chk.FolderReadable {
		t.Fatalf("check %d %+v", code, chk)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/handler -run "TestGetAppDoesNotCheck|TestCheckWithMissing" -v'`
Expected: FAIL (GET devolve versão 2; `/check` dá 404/405)

- [ ] **Step 3: Implementar**

Em `apps.go`, no `appDetail` acrescentar `RememberedFolder string \`json:"remembered_folder"\``. Trocar o fim de `getApp` (do `v, readable, err := h.checkVersion(...)` até o `writeJSON`) por:

```go
	d.FolderReadable = true // conhecido só depois do POST /check
	if v, err := h.q.LatestAppVersion(r.Context(), a.ID); err == nil {
		d.Version = &versionRef{Number: v.Number, CreatedAt: v.CreatedAt.Time}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		internalError(w, "latest version", err)
		return
	}
	if f, err := h.q.GetRunFolder(r.Context(), sqlc.GetRunFolderParams{PersonID: me.ID, AppID: a.ID}); err == nil {
		d.RememberedFolder = f
	} else if !errors.Is(err, pgx.ErrNoRows) {
		internalError(w, "run folder", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// checkApp compares the app folder with its latest version (ADR 006). The ficha
// calls it right after opening, so the page never waits for OneDrive.
func (h *Handler) checkApp(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	a, ok := h.appFromPath(w, r)
	if !ok {
		return
	}
	v, readable, err := h.checkVersion(context.WithoutCancel(r.Context()), a)
	if err != nil {
		slog.Error("version check", "app", a.ID, "err", err)
	}
	out := struct {
		FolderReadable bool        `json:"folder_readable"`
		Version        *versionRef `json:"version"`
	}{FolderReadable: readable}
	if v.ID != 0 {
		out.Version = &versionRef{Number: v.Number, CreatedAt: v.CreatedAt.Time}
	}
	writeJSON(w, http.StatusOK, out)
}
```

E no começo de `getApp` trocar `if _, ok := h.currentPerson(w, r); !ok {` por `me, ok := h.currentPerson(w, r)` / `if !ok {`.

Em `history.go`, apagar o bloco:

```go
	if r.URL.Query().Get("before") == "" {
		if _, _, err := h.checkVersion(context.WithoutCancel(ctx), a); err != nil {
			slog.Error("version check", "app", a.ID, "err", err)
		}
	}
```

(remover imports que ficarem sem uso).

Em `handler.go`, depois de `GET /api/apps/{id}`: `mux.HandleFunc("POST /api/apps/{id}/check", h.checkApp)`.

- [ ] **Step 4: Ajustar os testes que contavam com a conferência no GET**

Run: `grep -n 'c.do("GET", appPath(id, "")' backend/internal/handler/*_test.go` e `grep -n '/history' backend/internal/handler/*_test.go`.
Em cada teste que edita a pasta e depois espera versão nova por `GET appPath(id, "")` ou pelo histórico, chamar antes `c.do("POST", appPath(id, "/check"), nil, nil)`. Exemplo em `TestVersionOneOnPublishAndNewVersionOnChange`:

```go
	editFile(t, dir, "src/main.py", "print('v2')\n")
	c.do("POST", appPath(id, "/check"), nil, nil)
	c.do("GET", appPath(id, ""), nil, &d)
```

`TestGetAppWithMissingFolder` passa a conferir `folder_readable` pelo `/check` (o GET agora sempre diz `true`).

- [ ] **Step 5: Rodar o pacote inteiro**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/handler/ -count=1'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler
git commit -m "Ficha: open without waiting for the folder check; POST /check"
```

---

### Task 3: Pasta lembrada e Rodar a partir do catálogo

**Files:**
- Modify: `backend/internal/handler/runs.go` (`canRun`, `startRun`), `backend/internal/handler/apps.go` (`appSummary`, `listApps`)
- Test: `backend/internal/handler/runs_test.go`, `apps_test.go`

**Interfaces:**
- Consumes: `GetRunFolder`, `ListRunFolders`, `SetRunFolder`, `ClearRunFolder`, `ListAppsRow.Entrypoint` (Task 1).
- Produces: `func canRunKind(kind, entrypoint string) bool`; `POST /api/apps/{id}/runs` aceita `{"output_folder": string, "remember"?: bool}`; `GET /api/apps` itens com `can_run: bool` e `remembered_folder: string`.

- [ ] **Step 1: Testes que falham** (em `runs_test.go`)

```go
func TestRememberFolder(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, c, "PC")

	folderOf := func() string {
		var d appDetail
		c.do("GET", appPath(app, ""), nil, &d)
		return d.RememberedFolder
	}
	c.do("POST", appPath(app, "/runs"), map[string]any{"output_folder": `D:\Saídas`, "remember": true}, nil)
	if f := folderOf(); f != `D:\Saídas` {
		t.Fatalf("lembrada %q", f)
	}
	// Rodar de novo / 1 clique: sem "remember", não mexe.
	c.do("POST", appPath(app, "/runs"), map[string]any{"output_folder": `E:\Outra`}, nil)
	if f := folderOf(); f != `D:\Saídas` {
		t.Fatalf("sem remember mudou %q", f)
	}
	var list []appSummary
	c.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || !list[0].CanRun || list[0].RememberedFolder != `D:\Saídas` {
		t.Fatalf("lista %+v", list)
	}
	c.do("POST", appPath(app, "/runs"), map[string]any{"output_folder": `D:\Saídas`, "remember": false}, nil)
	if f := folderOf(); f != "" {
		t.Fatalf("esquecida %q", f)
	}
}

func TestRememberNotSavedWhenRunFails(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	// Sem computador conectado: 409, e a pasta não fica lembrada.
	if code := c.do("POST", appPath(app, "/runs"), map[string]any{"output_folder": `D:\Saídas`, "remember": true}, nil); code != http.StatusConflict {
		t.Fatalf("code %d", code)
	}
	var d appDetail
	c.do("GET", appPath(app, ""), nil, &d)
	if d.RememberedFolder != "" {
		t.Fatalf("lembrou sem rodar: %q", d.RememberedFolder)
	}
}

func TestRememberedFolderIsPerPerson(t *testing.T) {
	admin, _ := newPoolClient(t)
	setupAdmin(t, admin)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, admin, "PC")
	admin.do("POST", appPath(app, "/runs"), map[string]any{"output_folder": `D:\Henry`, "remember": true}, nil)

	createPersonAs(t, admin, "Ana", "ana@alis.com", "usuario")
	ana := admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	var d appDetail
	ana.do("GET", appPath(app, ""), nil, &d)
	if d.RememberedFolder != "" {
		t.Fatalf("Ana viu a pasta do Henry: %q", d.RememberedFolder)
	}
}
```

Se o `client` de teste não tiver `fresh()` (cliente novo, sem cookie, no mesmo servidor), procurar como outros testes criam o segundo cliente (`grep -n "loginAs(t," backend/internal/handler/*_test.go`) e usar o mesmo jeito.

- [ ] **Step 2: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/handler -run "TestRemember" -v'`
Expected: FAIL (campos não existem)

- [ ] **Step 3: Implementar**

Em `runs.go`:

```go
// canRun: in this phase only desktop apps opened by an .exe run through the agent.
func canRun(m manifest.Manifest) bool { return canRunKind(m.Kind, m.Entrypoint) }

func canRunKind(kind, entrypoint string) bool {
	return kind == "desktop" && strings.HasSuffix(strings.ToLower(entrypoint), ".exe")
}
```

No `startRun`, o corpo vira:

```go
	var body struct {
		OutputFolder string `json:"output_folder"`
		Remember     *bool  `json:"remember"`
	}
```

e logo depois do `writeError` do `enqueue` (isto é, depois de a execução estar na fila, antes do `GetRun`):

```go
	if body.Remember != nil {
		var err error
		if *body.Remember {
			err = h.q.SetRunFolder(ctx, sqlc.SetRunFolderParams{PersonID: me.ID, AppID: a.ID, Folder: out})
		} else {
			err = h.q.ClearRunFolder(ctx, sqlc.ClearRunFolderParams{PersonID: me.ID, AppID: a.ID})
		}
		if err != nil {
			slog.Error("remember folder", "app", a.ID, "err", err) // a execução já saiu; não falhar por isso
		}
	}
```

Em `apps.go`:

```go
type appSummary struct {
	sqlc.ListAppsRow
	FolderDisplay    string `json:"folder_display"`
	CanRun           bool   `json:"can_run"`
	RememberedFolder string `json:"remembered_folder"`
}

func (h *Handler) listApps(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	apps, err := h.q.ListApps(r.Context())
	if err != nil {
		internalError(w, "list apps", err)
		return
	}
	folders, err := h.q.ListRunFolders(r.Context(), me.ID)
	if err != nil {
		internalError(w, "list run folders", err)
		return
	}
	remembered := make(map[int64]string, len(folders))
	for _, f := range folders {
		remembered[f.AppID] = f.Folder
	}
	out := make([]appSummary, 0, len(apps))
	for _, a := range apps {
		out = append(out, appSummary{ListAppsRow: a, FolderDisplay: hostpath.ToDisplay(a.FolderPath),
			CanRun: canRunKind(a.Kind, a.Entrypoint), RememberedFolder: remembered[a.ID]})
	}
	writeJSON(w, http.StatusOK, out)
}
```

- [ ] **Step 4: Rodar o pacote**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/handler/ -count=1'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler
git commit -m "Runs: remember the output folder per person and app; can_run in the catalog"
```

---

### Task 4: Execuções ativas, dispensar e lista

**Files:**
- Modify: `backend/internal/handler/runs.go`, `backend/internal/handler/handler.go`
- Test: `backend/internal/handler/runs_list_test.go`

**Interfaces:**
- Produces:
  - `GET /api/runs/active` → `200 [{"id","app_id","app_name","status","error_message","file_count","created_at","finished_at"}]` (da pessoa logada, sem dispensadas, até 10, mais nova primeiro).
  - `POST /api/runs/{id}/dismiss` → `204`; `404` se a execução não é de quem pediu (inclusive admin dispensando a de outro).
  - `GET /api/runs?scope=mine|all&before=<id>` → `200 {"items":[{"id","app_id","app_name","status","output_folder","person_name","created_at","started_at","finished_at","file_count"}],"more":bool}`; `scope=all` para não-admin → `403 "Só administradores veem as execuções de todos."`.

- [ ] **Step 1: Testes que falham** (acrescentar a `runs_list_test.go`; imports `net/http`, `strconv`, `agentproto`)

```go
func finishRun(t *testing.T, c *client, token string) {
	t.Helper()
	var job agentproto.Job
	c.agent("GET", "/api/agent/next", token, nil, &job)
	p := "/api/agent/jobs/" + strconv.FormatInt(job.ID, 10) + "/result"
	c.agent("POST", p, token, agentproto.Result{Event: agentproto.EventStarted}, nil)
	c.agent("POST", p, token, agentproto.Result{Event: agentproto.EventFinished}, nil)
}

func TestActiveRunsAndDismiss(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	app := insertApp(t, filesPrepManifest())
	token := pairComputer(t, c, "PC")

	var run runJSON
	c.do("POST", appPath(app, "/runs"), map[string]string{}, &run)
	var active []activeRunJSON
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 1 || active[0].ID != run.ID || active[0].Status != "waiting" || active[0].AppName != "Eq. Research Files Prep" {
		t.Fatalf("ativa %+v", active)
	}
	finishRun(t, c, token)
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 1 || active[0].Status != "no_files" {
		t.Fatalf("terminada continua até dispensar %+v", active)
	}
	if code := c.do("POST", "/api/runs/"+strconv.FormatInt(run.ID, 10)+"/dismiss", nil, nil); code != http.StatusNoContent {
		t.Fatalf("dismiss %d", code)
	}
	c.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 0 {
		t.Fatalf("dispensada voltou %+v", active)
	}
}

func TestDismissOnlyOwnRun(t *testing.T) {
	admin, _ := newPoolClient(t)
	setupAdmin(t, admin)
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, admin, "PC")
	var run runJSON
	admin.do("POST", appPath(app, "/runs"), map[string]string{}, &run)

	createPersonAs(t, admin, "Ana", "ana@alis.com", "usuario")
	ana := admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("POST", "/api/runs/"+strconv.FormatInt(run.ID, 10)+"/dismiss", nil, nil); code != http.StatusNotFound {
		t.Fatalf("Ana dispensou a do Henry: %d", code)
	}
	var active []activeRunJSON
	ana.do("GET", "/api/runs/active", nil, &active)
	if len(active) != 0 {
		t.Fatalf("Ana vê faixa do Henry %+v", active)
	}
}

func TestListRunsScopesAndPages(t *testing.T) {
	admin, h := newPoolClient(t)
	setupAdmin(t, admin)
	h.runsPage = 2
	app := insertApp(t, filesPrepManifest())
	pairComputer(t, admin, "PC")
	for range 3 {
		admin.do("POST", appPath(app, "/runs"), map[string]string{}, nil)
	}
	var page runListJSON
	admin.do("GET", "/api/runs?scope=mine", nil, &page)
	if len(page.Items) != 2 || !page.More || page.Items[0].ID < page.Items[1].ID {
		t.Fatalf("página 1 %+v", page)
	}
	admin.do("GET", "/api/runs?scope=mine&before="+strconv.FormatInt(page.Items[1].ID, 10), nil, &page)
	if len(page.Items) != 1 || page.More {
		t.Fatalf("página 2 %+v", page)
	}

	createPersonAs(t, admin, "Ana", "ana@alis.com", "usuario")
	ana := admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	if code := ana.do("GET", "/api/runs?scope=all", nil, nil); code != http.StatusForbidden {
		t.Fatalf("all para usuário %d", code)
	}
	ana.do("GET", "/api/runs?scope=mine", nil, &page)
	if len(page.Items) != 0 {
		t.Fatalf("Ana vê do Henry %+v", page)
	}
	admin.do("GET", "/api/runs?scope=all", nil, &page)
	if len(page.Items) != 2 || page.Items[0].PersonName != "Henry" {
		t.Fatalf("all admin %+v", page)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/handler -run "TestActiveRuns|TestDismissOnly|TestListRuns" -v'`
Expected: FAIL (tipos e rotas não existem)

- [ ] **Step 3: Implementar**

No `Handler` (em `handler.go`, junto de `historyPage`) acrescentar o campo `runsPage int`, iniciado com `50` no construtor (mesmo lugar onde `historyPage` recebe o valor).

Em `runs.go`:

```go
type activeRunJSON struct {
	ID           int64      `json:"id"`
	AppID        int64      `json:"app_id"`
	AppName      string     `json:"app_name"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message"`
	FileCount    int64      `json:"file_count"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

type runListItemJSON struct {
	ID           int64      `json:"id"`
	AppID        int64      `json:"app_id"`
	AppName      string     `json:"app_name"`
	Status       string     `json:"status"`
	OutputFolder string     `json:"output_folder"`
	PersonName   string     `json:"person_name"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	FileCount    int64      `json:"file_count"`
}

type runListJSON struct {
	Items []runListItemJSON `json:"items"`
	More  bool              `json:"more"`
}

func (h *Handler) activeRuns(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	rows, err := h.q.ListActiveRuns(r.Context(), me.ID)
	if err != nil {
		internalError(w, "active runs", err)
		return
	}
	out := make([]activeRunJSON, 0, len(rows))
	for _, x := range rows {
		out = append(out, activeRunJSON{ID: x.ID, AppID: x.AppID, AppName: x.AppName, Status: x.Status,
			ErrorMessage: x.ErrorMessage, FileCount: x.FileCount, CreatedAt: x.CreatedAt.Time, FinishedAt: tsPtr(x.FinishedAt)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) dismissRun(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Execução não encontrada.")
		return
	}
	n, err := h.q.DismissRun(r.Context(), sqlc.DismissRunParams{ID: id, PersonID: me.ID})
	if err != nil {
		internalError(w, "dismiss run", err)
		return
	}
	if n == 0 {
		// Já dispensada é sucesso; de outra pessoa ou inexistente é 404.
		row, err := h.q.GetRun(r.Context(), id)
		if err != nil || row.PersonID != me.ID {
			writeError(w, http.StatusNotFound, "Execução não encontrada.")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	all := r.URL.Query().Get("scope") == "all"
	if all && me.Role != "admin" {
		writeError(w, http.StatusForbidden, "Só administradores veem as execuções de todos.")
		return
	}
	var before int64
	if s := r.URL.Query().Get("before"); s != "" {
		var err error
		if before, err = strconv.ParseInt(s, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "Não entendi de onde carregar mais.")
			return
		}
	}
	if err := h.q.ExpireStaleRuns(r.Context()); err != nil {
		internalError(w, "expire runs", err)
		return
	}
	rows, err := h.q.ListRuns(r.Context(), sqlc.ListRunsParams{SeeAll: all, PersonID: me.ID, Before: before, Lim: int32(h.runsPage + 1)})
	if err != nil {
		internalError(w, "list runs", err)
		return
	}
	out := runListJSON{Items: []runListItemJSON{}}
	for i, x := range rows {
		if i == h.runsPage {
			out.More = true
			break
		}
		out.Items = append(out.Items, runListItemJSON{ID: x.ID, AppID: x.AppID, AppName: x.AppName, Status: x.Status,
			OutputFolder: x.OutputFolder, PersonName: x.PersonName, CreatedAt: x.CreatedAt.Time,
			StartedAt: tsPtr(x.StartedAt), FinishedAt: tsPtr(x.FinishedAt), FileCount: x.FileCount})
	}
	writeJSON(w, http.StatusOK, out)
}
```

Rotas em `handler.go`, antes de `GET /api/runs/{id}`:

```go
	mux.HandleFunc("GET /api/runs", h.listRuns)
	mux.HandleFunc("GET /api/runs/active", h.activeRuns)
	mux.HandleFunc("POST /api/runs/{id}/dismiss", h.dismissRun)
```

(`/api/runs/active` é literal, então ganha de `/api/runs/{id}` no roteador do Go.)

- [ ] **Step 4: Rodar tudo do backend**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go vet ./... && mise x -- go test ./... -count=1'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler
git commit -m "Runs: active runs for the strip, dismiss, and paged list (mine/all)"
```

---

### Task 5: API do frontend e fixtures

**Files:**
- Modify: `frontend/src/lib/api.ts`, `frontend/src/test/fixtures.ts`

**Interfaces:**
- Produces (TypeScript):

```ts
export type ActiveRun = {
  id: number; app_id: number; app_name: string; status: RunStatus; error_message: string
  file_count: number; created_at: string; finished_at: string | null
}
export type RunListItem = {
  id: number; app_id: number; app_name: string; status: RunStatus; output_folder: string; person_name: string
  created_at: string; started_at: string | null; finished_at: string | null; file_count: number
}
export type RunList = { items: RunListItem[]; more: boolean }
export type AppCheck = { folder_readable: boolean; version: { number: number; created_at: string } | null }
// AppSummary e AppDetail ganham: can_run: boolean (AppSummary) e remembered_folder: string (os dois)

api.startRun(appId: number, output_folder: string, remember?: boolean): Promise<Run>
api.checkApp(id: number | string): Promise<AppCheck>
api.activeRuns(): Promise<ActiveRun[]>
api.dismissRun(id: number): Promise<void>
api.runs(scope: "mine" | "all", before?: number): Promise<RunList>
```

- [ ] **Step 1: Implementar** em `api.ts`

```ts
  startRun: (appId: number, output_folder: string, remember?: boolean) =>
    request<Run>("POST", `/api/apps/${appId}/runs`, remember === undefined ? { output_folder } : { output_folder, remember }),
  checkApp: (id: number | string) => request<AppCheck>("POST", `/api/apps/${id}/check`),
  activeRuns: () => request<ActiveRun[]>("GET", "/api/runs/active"),
  dismissRun: (id: number) => request<void>("POST", `/api/runs/${id}/dismiss`),
  runs: (scope: "mine" | "all", before?: number) =>
    request<RunList>("GET", `/api/runs?scope=${scope}${before ? `&before=${before}` : ""}`),
```

e os tipos acima; `AppSummary` ganha `can_run: boolean; remembered_folder: string`, `AppDetail` ganha `remembered_folder: string`.

Em `fixtures.ts`:

```ts
export function activeRun(over: Partial<ActiveRun> = {}): ActiveRun {
  return { id: 7, app_id: 3, app_name: "Eq. Research Files Prep", status: "running", error_message: "",
    file_count: 0, created_at: "2026-10-01T14:00:00Z", finished_at: null, ...over }
}

export function appSummary(over: Partial<AppSummary> = {}): AppSummary {
  return { id: 3, slug: "files-prep", name: "Eq. Research Files Prep", summary: "Baixa da CVM", kind: "desktop",
    language: "python", area: "Research", folder_path: "/mnt/c/apps/fp", folder_display: "C:\\apps\\fp",
    owner_id: 1, owner_name: "Henry", backup_id: null, updated_at: "2026-09-30T10:00:00Z",
    can_run: true, remembered_folder: "", ...over }
}
```

Corrigir `detail()` em `AppPage.test.tsx` acrescentando `remembered_folder: ""`.

- [ ] **Step 2: Conferir tipos**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx tsc -b'`
Expected: sem erros (se algum teste ou página reclamar de `remembered_folder`/`can_run`, acrescentar o campo nos objetos de teste).

- [ ] **Step 3: Commit**

```bash
git add frontend/src
git commit -m "Frontend API: active runs, dismiss, run list, folder check, remembered folder"
```

---

### Task 6: RunsProvider, janela com "Lembrar" e faixa

**Files:**
- Create: `frontend/src/lib/runs-context.tsx`, `frontend/src/components/RunStrip.tsx`
- Modify: `frontend/src/components/RunDialog.tsx`, `frontend/src/components/Layout.tsx`
- Test: `frontend/src/lib/runs-context.test.tsx`, `frontend/src/components/RunStrip.test.tsx`

**Interfaces:**
- Consumes: `api.startRun`, `api.activeRuns`, `api.dismissRun` (Task 5).
- Produces:

```ts
export type RunTarget = { id: number; name: string; folder_display: string; remembered_folder: string }
export type RunOptions = { folder?: string; ask?: boolean } // folder: Rodar de novo; ask: "trocar"
export type LocalNotice = { key: string; app_name: string; kind: "starting" | "error"; message?: string; offline?: boolean; target?: RunTarget }
export function RunsProvider({ children }: { children: ReactNode }): JSX.Element
export function useRuns(): {
  active: ActiveRun[]                 // do servidor
  notices: LocalNotice[]              // "Abrindo…" e erros ao pedir
  busy: Set<number>                   // ids de app com pedido saindo (desabilita o botão)
  run: (t: RunTarget, o?: RunOptions) => void
  dismiss: (id: number) => void       // servidor
  dropNotice: (key: string) => void   // local
  refresh: () => void
  rememberedFor: (t: RunTarget) => string // pasta lembrada já considerando o que mudou nesta aba
}
// RunDialog passa a ser controlado pelo provider:
export function RunDialog(props: { target: RunTarget | null; onCancel: () => void; onConfirm: (folder: string, remember: boolean) => void }): JSX.Element
```

Regras de `run(t, o)`:
- `o.folder` presente → roda direto com essa pasta, sem `remember`.
- `o.ask` ou `t.remembered_folder === ""` → abre a janela (pasta inicial: `t.remembered_folder || t.folder_display`, "Lembrar esta pasta" marcada). Confirmar → roda com `remember` = caixinha.
- Senão → roda direto com `t.remembered_folder`, sem `remember`.
- Rodar: põe `{kind:"starting"}` em `notices` e o id em `busy`; chama `api.startRun`; no sucesso tira o aviso e chama `refresh()`; no erro troca por `{kind:"error", message, offline: status === 409, target}`.
- Consulta `api.activeRuns()` ao montar, depois a cada 3 s se alguma está `waiting`/`running` ou há `starting`, senão a cada 30 s.

- [ ] **Step 1: Teste do provider que falha** (`runs-context.test.tsx`)

```tsx
import { act, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { run as runFx } from "@/test/fixtures"
import { RunsProvider, useRuns, type RunTarget } from "./runs-context"

afterEach(() => vi.restoreAllMocks())

const target = (remembered = ""): RunTarget => ({ id: 3, name: "Files Prep", folder_display: "C:\\apps\\fp", remembered_folder: remembered })

function Probe({ t, folder, ask }: { t: RunTarget; folder?: string; ask?: boolean }) {
  const { run, notices } = useRuns()
  return (
    <>
      <button onClick={() => run(t, { folder, ask })}>go</button>
      {notices.map((n) => <p key={n.key}>{n.kind}:{n.message}</p>)}
    </>
  )
}

const mount = (ui: React.ReactNode) => render(<RunsProvider>{ui}</RunsProvider>)

test("first time asks with Lembrar checked and sends remember", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx({ status: "waiting" }))
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("C:\\apps\\fp")
  expect(screen.getByLabelText("Lembrar esta pasta")).toBeChecked()
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(start).toHaveBeenCalledWith(3, "C:\\apps\\fp", true)
})

test("unchecking Lembrar sends remember false", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  await user.click(screen.getByLabelText("Lembrar esta pasta"))
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(start).toHaveBeenCalledWith(3, "C:\\apps\\fp", false)
})

test("remembered folder runs in one click without remember", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target("D:\\Saídas")} />)
  await user.click(screen.getByText("go"))
  expect(screen.queryByLabelText("Onde salvar os resultados?")).not.toBeInTheDocument()
  expect(start).toHaveBeenCalledWith(3, "D:\\Saídas", undefined)
})

test("Rodar de novo uses the given folder", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target("D:\\Saídas")} folder="E:\\Velha" />)
  await user.click(screen.getByText("go"))
  expect(start).toHaveBeenCalledWith(3, "E:\\Velha", undefined)
})

test("error while asking to run becomes an error notice", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "startRun").mockRejectedValue(new ApiError(409, "Seu computador não está conectado.", {}))
  mount(<Probe t={target("D:\\Saídas")} />)
  await user.click(screen.getByText("go"))
  expect(await screen.findByText("error:Seu computador não está conectado.")).toBeInTheDocument()
})
```

(Se o construtor de `ApiError` tiver outra assinatura, ver `api.ts` e ajustar.)

- [ ] **Step 2: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/lib/runs-context.test.tsx'`
Expected: FAIL (módulo não existe)

- [ ] **Step 3: Implementar `RunDialog` controlado**

```tsx
import { useEffect, useState } from "react"
import { FolderOpen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { api } from "@/lib/api"
import type { RunTarget } from "@/lib/runs-context"

export function RunDialog({ target, onCancel, onConfirm }: { target: RunTarget | null; onCancel: () => void; onConfirm: (folder: string, remember: boolean) => void }) {
  const [folder, setFolder] = useState("")
  const [remember, setRemember] = useState(true)
  const [error, setError] = useState("")
  const [choosing, setChoosing] = useState(false)

  useEffect(() => {
    if (target) {
      setFolder(target.remembered_folder || target.folder_display)
      setRemember(true)
      setError("")
    }
  }, [target])

  const choose = async () => {
    setError("")
    setChoosing(true)
    try {
      const r = await api.chooseFolder(folder)
      if (!r.canceled) setFolder(r.folder)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível abrir a janela de pastas.")
    } finally {
      setChoosing(false)
    }
  }

  return (
    <Dialog open={target !== null} onOpenChange={(v) => !v && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rodar {target?.name}</DialogTitle>
          <DialogDescription>O app vai abrir no seu computador. Use a janela normalmente e feche quando terminar. O Hub mostra os arquivos que ele gerou.</DialogDescription>
        </DialogHeader>
        <Field label="Onde salvar os resultados?" htmlFor="run-folder" hint="Por padrão, a própria pasta do app.">
          <div className="flex gap-2">
            <Input id="run-folder" value={folder} onChange={(e) => setFolder(e.target.value)} className="font-mono text-xs" />
            <Button type="button" variant="outline" onClick={choose} disabled={choosing} className="shrink-0 gap-2">
              <FolderOpen className="size-4" /> Escolher outra…
            </Button>
          </div>
        </Field>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} className="size-4 accent-[var(--stamp)]" />
          Lembrar esta pasta
        </label>
        {choosing && <p className="text-sm text-muted-foreground">A janela de pastas abriu no seu computador.</p>}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button onClick={() => onConfirm(folder.trim(), remember)} disabled={choosing || folder.trim() === ""}>Abrir app</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 4: Implementar `runs-context.tsx`**

```tsx
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { RunDialog } from "@/components/RunDialog"
import { api, ApiError, type ActiveRun } from "@/lib/api"
import { isLive } from "@/lib/runs"

export type RunTarget = { id: number; name: string; folder_display: string; remembered_folder: string }
export type RunOptions = { folder?: string; ask?: boolean }
export type LocalNotice = { key: string; app_name: string; kind: "starting" | "error"; message?: string; offline?: boolean; target?: RunTarget }

type Ctx = {
  active: ActiveRun[]
  notices: LocalNotice[]
  busy: Set<number>
  run: (t: RunTarget, o?: RunOptions) => void
  dismiss: (id: number) => void
  dropNotice: (key: string) => void
  refresh: () => void
}

const RunsCtx = createContext<Ctx | null>(null)

export function useRuns(): Ctx {
  const c = useContext(RunsCtx)
  if (!c) throw new Error("useRuns fora do RunsProvider")
  return c
}

let seq = 0

export function RunsProvider({ children }: { children: ReactNode }) {
  const [active, setActive] = useState<ActiveRun[]>([])
  const [notices, setNotices] = useState<LocalNotice[]>([])
  const [busy, setBusy] = useState<Set<number>>(new Set())
  const [asking, setAsking] = useState<RunTarget | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout>>()

  const refresh = useCallback(() => {
    api.activeRuns().then(setActive, () => {})
  }, [])

  const live = active.some((r) => isLive(r.status)) || notices.some((n) => n.kind === "starting")
  useEffect(() => {
    refresh()
    const tick = () => {
      refresh()
      timer.current = setTimeout(tick, live ? 3000 : 30000)
    }
    timer.current = setTimeout(tick, live ? 3000 : 30000)
    return () => clearTimeout(timer.current)
  }, [refresh, live])

  const send = useCallback(
    (t: RunTarget, folder: string, remember?: boolean) => {
      const key = `n${++seq}`
      setNotices((ns) => [...ns, { key, app_name: t.name, kind: "starting" }])
      setBusy((b) => new Set(b).add(t.id))
      api.startRun(t.id, folder, remember).then(
        () => {
          setNotices((ns) => ns.filter((n) => n.key !== key))
          refresh()
        },
        (e: unknown) => {
          const message = e instanceof Error ? e.message : "Não foi possível abrir o app."
          const offline = e instanceof ApiError && e.status === 409
          setNotices((ns) => ns.map((n) => (n.key === key ? { ...n, kind: "error", message, offline, target: t } : n)))
        },
      ).finally(() =>
        setBusy((b) => {
          const nb = new Set(b)
          nb.delete(t.id)
          return nb
        }),
      )
    },
    [refresh],
  )

  const run = useCallback(
    (t: RunTarget, o: RunOptions = {}) => {
      if (o.folder) return send(t, o.folder)
      if (o.ask || t.remembered_folder === "") return setAsking(t)
      send(t, t.remembered_folder)
    },
    [send],
  )

  const dismiss = useCallback((id: number) => {
    setActive((a) => a.filter((r) => r.id !== id))
    api.dismissRun(id).catch(() => {})
  }, [])

  const dropNotice = useCallback((key: string) => setNotices((ns) => ns.filter((n) => n.key !== key)), [])

  const value = useMemo(() => ({ active, notices, busy, run, dismiss, dropNotice, refresh }), [active, notices, busy, run, dismiss, dropNotice, refresh])

  return (
    <RunsCtx.Provider value={value}>
      {children}
      <RunDialog
        target={asking}
        onCancel={() => setAsking(null)}
        onConfirm={(folder, remember) => {
          const t = asking
          setAsking(null)
          if (t) send(t, folder, remember)
        }}
      />
    </RunsCtx.Provider>
  )
}
```

Observação: depois que a pessoa marca "Lembrar" e roda, o `remembered_folder` do cartão/ficha na tela ainda é o antigo até recarregar. Para o próximo clique já ser direto, a página que chamou recarrega os dados (Task 7 e 8 fazem isso ouvindo `active` mudar) — **ou** o provider guarda um mapa local `remembered: Record<number, string>` atualizado no `send` com `remember === true` (e apagado com `false`) e o `run` consulta esse mapa antes de `t.remembered_folder`. Usar o mapa local (mais simples e não depende das páginas): acrescentar

```tsx
  const [remembered, setRemembered] = useState<Record<number, string>>({})
```

no `send`, dentro do sucesso: `if (remember !== undefined) setRemembered((m) => ({ ...m, [t.id]: remember ? folder : "" }))`; no `run`: `const r = t.id in remembered ? remembered[t.id] : t.remembered_folder` e usar `r` no lugar de `t.remembered_folder` (também como pasta inicial ao abrir a janela: `setAsking({ ...t, remembered_folder: r })`). Expor `rememberedFor: (t: RunTarget) => string` no contexto para a ficha mostrar "salva em …" atualizado. Acrescentar ao teste:

```tsx
test("after remembering, the next click runs directly", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  await user.click(screen.getByText("go"))
  expect(start).toHaveBeenLastCalledWith(3, "C:\\apps\\fp", undefined)
})
```

- [ ] **Step 5: Rodar os testes do provider**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/lib/runs-context.test.tsx'`
Expected: PASS

- [ ] **Step 6: Teste da faixa que falha** (`RunStrip.test.tsx`)

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { RunsProvider } from "@/lib/runs-context"
import { activeRun } from "@/test/fixtures"
import { RunStrip } from "./RunStrip"

afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/"]}>
      <RunsProvider>
        <Routes>
          <Route path="/" element={<RunStrip />} />
          <Route path="/execucoes/:id" element={<p>página da execução</p>} />
          <Route path="/execucoes" element={<p>minhas execuções</p>} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )

test("open app shows the hint and can be hidden", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "running" })])
  mount()
  expect(await screen.findByText(/aberto no seu computador/)).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "ocultar" }))
  expect(screen.queryByText(/aberto no seu computador/)).not.toBeInTheDocument()
})

test("done run offers Ver e baixar, which dismisses and opens the run", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "done", file_count: 4 })])
  const dismiss = vi.spyOn(api, "dismissRun").mockResolvedValue()
  mount()
  expect(await screen.findByText(/terminou: 4 arquivos novos/)).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Ver e baixar" }))
  expect(dismiss).toHaveBeenCalledWith(7)
  expect(await screen.findByText("página da execução")).toBeInTheDocument()
})

test("each ending has its own text", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([
    activeRun({ id: 1, status: "no_files" }),
    activeRun({ id: 2, status: "interrupted" }),
    activeRun({ id: 3, status: "error", error_message: "O Agente não conseguiu abrir o app." }),
  ])
  mount()
  expect(await screen.findByText(/terminou sem arquivos novos/)).toBeInTheDocument()
  expect(screen.getByText(/foi interrompido/)).toBeInTheDocument()
  expect(screen.getByText(/O Agente não conseguiu abrir o app./)).toBeInTheDocument()
})

test("✕ dismisses a finished run", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "no_files" })])
  const dismiss = vi.spyOn(api, "dismissRun").mockResolvedValue()
  mount()
  await user.click(await screen.findByRole("button", { name: "Dispensar" }))
  expect(dismiss).toHaveBeenCalledWith(7)
  expect(screen.queryByText(/terminou sem arquivos novos/)).not.toBeInTheDocument()
})

test("more than 3 shows +N linking to Minhas execuções", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([1, 2, 3, 4, 5].map((id) => activeRun({ id, status: "running" })))
  mount()
  await user.click(await screen.findByRole("link", { name: "+2 em Minhas execuções" }))
  expect(await screen.findByText("minhas execuções")).toBeInTheDocument()
})
```

- [ ] **Step 7: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/components/RunStrip.test.tsx'`
Expected: FAIL

- [ ] **Step 8: Implementar `RunStrip.tsx`**

Seguir o `frontend-design` do projeto (tokens `bg-ink`, `text-ink-foreground`, `font-heading`, `text-stamp`); visual como no protótipo: cartões escuros empilhados, fixos embaixo, à direita da barra lateral no computador e em largura cheia no celular.

```tsx
import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { Check, LoaderCircle, TriangleAlert, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import type { ActiveRun } from "@/lib/api"
import { useRuns, type LocalNotice } from "@/lib/runs-context"
import { isLive } from "@/lib/runs"
import { cn } from "@/lib/utils"

const MAX = 3

function text(r: ActiveRun) {
  const n = r.file_count
  switch (r.status) {
    case "waiting": return <>Abrindo <b>{r.app_name}</b> no seu computador…</>
    case "running": return <><b>{r.app_name}</b> aberto no seu computador. Use a janela e feche quando terminar.</>
    case "done": return <><b>{r.app_name}</b> terminou: {n} {n === 1 ? "arquivo novo" : "arquivos novos"}</>
    case "no_files": return <><b>{r.app_name}</b> terminou sem arquivos novos</>
    case "interrupted": return <><b>{r.app_name}</b> foi interrompido: o computador parou de responder</>
    default: return <>Não deu para abrir o <b>{r.app_name}</b>: {r.error_message}</>
  }
}

function Card({ tone, children, actions }: { tone: "live" | "ok" | "bad"; children: React.ReactNode; actions: React.ReactNode }) {
  return (
    <div role="status" className={cn("pointer-events-auto flex flex-wrap items-center justify-between gap-3 rounded-lg px-4 py-3 text-sm shadow-lg",
      tone === "ok" ? "bg-[oklch(0.35_0.07_150)] text-white" : "bg-ink text-ink-foreground")}>
      <span className="flex min-w-0 items-center gap-2">
        {tone === "live" && <LoaderCircle className="size-4 shrink-0 animate-spin" />}
        {tone === "ok" && <Check className="size-4 shrink-0" />}
        {tone === "bad" && <TriangleAlert className="size-4 shrink-0 text-stamp" />}
        <span>{children}</span>
      </span>
      <span className="flex shrink-0 items-center gap-2">{actions}</span>
    </div>
  )
}

export function RunStrip() {
  const { active, notices, dismiss, dropNotice, run } = useRuns()
  const [hidden, setHidden] = useState<Set<number>>(new Set())
  const navigate = useNavigate()
  const open = (id: number) => { dismiss(id); navigate(`/execucoes/${id}`) }

  const runs = active.filter((r) => !(isLive(r.status) && hidden.has(r.id)))
  const items: React.ReactNode[] = [
    ...notices.map((n: LocalNotice) =>
      n.kind === "starting" ? (
        <Card key={n.key} tone="live" actions={null}>Abrindo <b>{n.app_name}</b> no seu computador…</Card>
      ) : (
        <Card key={n.key} tone="bad" actions={<>
          {n.offline
            ? <Button size="sm" variant="secondary" onClick={() => { dropNotice(n.key); navigate("/meu-computador") }}>Ver Meu computador</Button>
            : n.target && <Button size="sm" variant="secondary" onClick={() => { dropNotice(n.key); run(n.target!, { ask: true }) }}>trocar pasta</Button>}
          <button type="button" aria-label="Dispensar" onClick={() => dropNotice(n.key)} className="rounded p-1 hover:bg-white/10"><X className="size-4" /></button>
        </>}>Não deu para abrir o <b>{n.app_name}</b>: {n.message}</Card>
      ),
    ),
    ...runs.map((r) => {
      const live = isLive(r.status)
      const tone = live ? "live" : r.status === "done" ? "ok" : "bad"
      return (
        <Card key={r.id} tone={tone} actions={live ? (
          <Button size="sm" variant="secondary" onClick={() => setHidden((h) => new Set(h).add(r.id))}>ocultar</Button>
        ) : (<>
          <Button size="sm" variant="secondary" onClick={() => open(r.id)}>{r.status === "done" ? "Ver e baixar" : "Ver detalhes"}</Button>
          <button type="button" aria-label="Dispensar" onClick={() => dismiss(r.id)} className="rounded p-1 hover:bg-white/10"><X className="size-4" /></button>
        </>)}>{text(r)}</Card>
      )
    }),
  ]
  if (items.length === 0) return null
  const extra = items.length - MAX
  return (
    <div className="pointer-events-none fixed inset-x-4 bottom-4 z-40 flex flex-col gap-2 md:left-[calc(16rem+1rem)]">
      {items.slice(0, MAX)}
      {extra > 0 && (
        <Link to="/execucoes" className="pointer-events-auto self-end rounded-md bg-ink px-3 py-1.5 text-xs text-ink-foreground underline-offset-4 hover:underline">
          +{extra} em Minhas execuções
        </Link>
      )}
    </div>
  )
}
```

- [ ] **Step 9: Ligar no `Layout`**

Envolver o conteúdo do `Layout` em `<RunsProvider>` e pôr `<RunStrip />` logo depois de `</main>`. Dar ao `<main>` um `pb-28` para a faixa não cobrir o fim da página.

- [ ] **Step 10: Rodar a camada de frontend**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run && mise x -- npx tsc -b'`
Expected: os testes novos passam. `AppPage.test.tsx` ainda usa o `RunDialog` antigo e vai falhar — é corrigido no Task 7 (pode marcar esses 2 testes com `.skip` aqui e tirar no Task 7, ou fazer os dois Tasks no mesmo ciclo).

- [ ] **Step 11: Commit**

```bash
git add frontend/src
git commit -m "Runs strip: one place to start runs, remember-folder dialog, bottom strip"
```

---

### Task 7: Ficha na hora e "salva em … · trocar"

**Files:**
- Modify: `frontend/src/pages/AppPage.tsx`, `frontend/src/components/HistoryTimeline.tsx`
- Test: `frontend/src/pages/AppPage.test.tsx`

**Interfaces:**
- Consumes: `api.checkApp` (Task 5), `useRuns().run`, `useRuns().rememberedFor` (Task 6).
- Produces: `HistoryTimeline` aceita `reloadKey?: number` (recarrega quando muda).

- [ ] **Step 1: Reescrever os testes** (`AppPage.test.tsx`; o `renderAt` passa a envolver em `<RunsProvider>`, e todo teste mocka `api.activeRuns` com `[]` e `api.checkApp`)

```tsx
const renderAt = () =>
  render(
    <MemoryRouter initialEntries={["/apps/3"]}>
      <RunsProvider>
        <Routes>
          <Route path="/apps/:id" element={<AppPage />} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )

const base = (d: AppDetail) => {
  vi.spyOn(api, "app").mockResolvedValue(d)
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
}

test("ficha shows before the folder check answers", async () => {
  base(detail(true))
  vi.spyOn(api, "checkApp").mockReturnValue(new Promise(() => {})) // nunca responde
  renderAt()
  expect(await screen.findByRole("heading", { name: "Eq. Research Files Prep" })).toBeInTheDocument()
  expect(screen.getByText(/versão 2/)).toBeInTheDocument()
})

test("check finding a new version updates the field and reloads history", async () => {
  base(detail(true))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: { number: 3, created_at: "2026-10-01T15:00:00Z" } })
  renderAt()
  expect(await screen.findByText(/versão 3/)).toBeInTheDocument()
  expect(api.history).toHaveBeenCalledTimes(2)
})

test("check with same version does not reload history", async () => {
  base(detail(true))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: { number: 2, created_at: "2026-09-30T10:00:00Z" } })
  renderAt()
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  await new Promise((r) => setTimeout(r, 50))
  expect(api.history).toHaveBeenCalledTimes(1)
})

test("unreadable folder warning appears after the check", async () => {
  base(detail(true))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: false, version: null })
  renderAt()
  expect(await screen.findByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora")
})

test("Rodar with no remembered folder asks; with one, shows 'salva em' and runs directly", async () => {
  const user = userEvent.setup()
  base({ ...detail(true), remembered_folder: "D:\\Saídas" })
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run({ status: "waiting" }))
  renderAt()
  expect(await screen.findByText("D:\\Saídas")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Rodar" }))
  expect(start).toHaveBeenCalledWith(3, "D:\\Saídas", undefined)
  await user.click(screen.getByRole("button", { name: "trocar" }))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("D:\\Saídas")
})
```

(Remover os dois testes antigos de "Rodar opens the dialog…" e "choose another folder…": o primeiro está coberto em `runs-context.test.tsx`; o "Escolher outra…" ganha um teste equivalente lá:)

```tsx
test("Escolher outra fills the field", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "chooseFolder").mockResolvedValue({ folder: "D:\\Saídas", canceled: false })
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  await user.click(screen.getByRole("button", { name: "Escolher outra…" }))
  expect(await screen.findByDisplayValue("D:\\Saídas")).toBeInTheDocument()
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/pages/AppPage.test.tsx'`
Expected: FAIL

- [ ] **Step 3: Implementar**

`HistoryTimeline`: assinatura `({ appId, canEdit, onChanged, reloadKey = 0 }: { …; reloadKey?: number })` e `useEffect(() => load(filter), [load, filter, reloadKey])`.

`AppPage`:

```tsx
  const [readable, setReadable] = useState(true)
  const [historyKey, setHistoryKey] = useState(0)
  const { run, busy, rememberedFor } = useRuns()

  useEffect(() => {
    if (!id) return
    let gone = false
    api.app(id).then(
      (d) => {
        if (gone) return
        setApp(d)
        api.checkApp(id).then((c) => {
          if (gone) return
          setReadable(c.folder_readable)
          if (c.version && c.version.number !== d.version?.number) {
            setApp((a) => (a ? { ...a, version: c.version } : a))
            setHistoryKey((k) => k + 1)
          }
        }, () => {})
      },
      (e: Error) => setError(e.message),
    )
    return () => { gone = true }
  }, [id])
```

O aviso usa `!readable` no lugar de `!app.folder_readable`. O bloco do Rodar vira:

```tsx
        {app.can_run && (() => {
          const t = { id: app.id, name: m.name, folder_display: app.folder_display, remembered_folder: app.remembered_folder }
          const saved = rememberedFor(t)
          return (
            <div className="mt-5 flex flex-wrap items-center gap-x-3 gap-y-2">
              <Button onClick={() => run(t)} disabled={busy.has(app.id)} className="gap-2">
                <Play className="size-4" /> Rodar
              </Button>
              {saved && (
                <span className="text-sm text-muted-foreground">
                  salva em <code className="font-mono text-xs break-all">{saved}</code> ·{" "}
                  <button type="button" onClick={() => run(t, { ask: true })} className="underline underline-offset-4 hover:text-foreground">trocar</button>
                </span>
              )}
            </div>
          )
        })()}
```

E `<HistoryTimeline appId={app.id} canEdit={canEdit} reloadKey={historyKey} onChanged={…} />`. Remover o import e o estado do `RunDialog`.

- [ ] **Step 4: Rodar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run && mise x -- npx tsc -b'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "Ficha: show at once, check the folder in the background; 'salva em · trocar'"
```

---

### Task 8: Rodar no cartão do catálogo

**Files:**
- Modify: `frontend/src/pages/CatalogPage.tsx`
- Test: `frontend/src/pages/CatalogPage.test.tsx` (novo)

**Interfaces:**
- Consumes: `AppSummary.can_run`, `AppSummary.remembered_folder` (Task 5); `useRuns().run`, `busy` (Task 6).

- [ ] **Step 1: Teste que falha**

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { RunsProvider } from "@/lib/runs-context"
import { appSummary, run } from "@/test/fixtures"
import { CatalogPage } from "./CatalogPage"

afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/"]}>
      <RunsProvider>
        <Routes>
          <Route path="/" element={<CatalogPage />} />
          <Route path="/apps/:id" element={<p>ficha</p>} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )

test("card with can_run has Rodar; doc has none", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "apps").mockResolvedValue([appSummary(), appSummary({ id: 4, name: "Expandir Projeção", kind: "reference", can_run: false })])
  mount()
  await screen.findByText("Expandir Projeção")
  expect(screen.getAllByRole("button", { name: "Rodar Eq. Research Files Prep" })).toHaveLength(1)
  expect(screen.queryByRole("button", { name: "Rodar Expandir Projeção" })).not.toBeInTheDocument()
})

test("Rodar on the card runs directly with the remembered folder and stays on the catalog", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "apps").mockResolvedValue([appSummary({ remembered_folder: "D:\\Saídas" })])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run())
  mount()
  await user.click(await screen.findByRole("button", { name: "Rodar Eq. Research Files Prep" }))
  expect(start).toHaveBeenCalledWith(3, "D:\\Saídas", undefined)
  expect(screen.queryByText("ficha")).not.toBeInTheDocument()
})

test("clicking the card name still opens the ficha", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "apps").mockResolvedValue([appSummary()])
  mount()
  await user.click(await screen.findByRole("heading", { name: "Eq. Research Files Prep" }))
  expect(await screen.findByText("ficha")).toBeInTheDocument()
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/pages/CatalogPage.test.tsx'`
Expected: FAIL

- [ ] **Step 3: Implementar**

O cartão deixa de ser um `<Link>` inteiro (botão dentro de link não é válido). Estrutura: `<li>` com o `KindTab`, e o cartão `div` com `relative`; o título é um `<Link>` com a classe `after:absolute after:inset-0` (o cartão todo continua clicável para a ficha) e o rodapé tem o botão com `relative z-10` (fica por cima do link):

```tsx
          <li key={a.id} className="rise" style={{ animationDelay: `${Math.min(i, 12) * 40}ms` }}>
            <div className="group">
              <KindTab kind={a.kind} className="ml-4" />
              <div className="relative flex min-h-40 flex-col rounded-lg border border-border bg-card p-5 shadow-card transition-transform group-hover:-translate-y-0.5">
                <h2 className="font-heading text-xl leading-snug group-hover:text-stamp">
                  <Link to={`/apps/${a.id}`} className="after:absolute after:inset-0">{a.name}</Link>
                </h2>
                <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">{a.summary || "Sem resumo ainda."}</p>
                <div className="mt-auto flex items-center justify-between gap-2 border-t border-dashed border-border pt-3 text-xs text-muted-foreground">
                  <span className="truncate">{a.area || "Sem área"} · <span className="font-medium text-foreground/80">{a.owner_name}</span></span>
                  {a.can_run && (
                    <Button size="sm" aria-label={`Rodar ${a.name}`} disabled={busy.has(a.id)} className="relative z-10 h-7 gap-1.5"
                      onClick={() => run({ id: a.id, name: a.name, folder_display: a.folder_display, remembered_folder: a.remembered_folder })}>
                      <Play className="size-3.5" /> Rodar
                    </Button>
                  )}
                </div>
              </div>
            </div>
          </li>
```

(Imports: `Button`, `Play`, `useRuns`.) O `h2` passa a ter um link dentro; o teste "clicking the card name" clica no heading, que contém o link.

- [ ] **Step 4: Rodar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run && mise x -- npx tsc -b'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "Catalog: Rodar button on each runnable card"
```

---

### Task 9: Minhas execuções e Rodar de novo

**Files:**
- Create: `frontend/src/pages/MyRunsPage.tsx`
- Modify: `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`, `frontend/src/pages/RunPage.tsx`
- Test: `frontend/src/pages/MyRunsPage.test.tsx` (novo), `frontend/src/pages/RunPage.test.tsx`

**Interfaces:**
- Consumes: `api.runs`, `RunListItem` (Task 5); `useRuns().run`, `active` (Task 6); `STATUS`, `formatWhen`, `formatDuration` de `lib/runs.ts`.

- [ ] **Step 1: Testes que falham** (`MyRunsPage.test.tsx`)

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type RunListItem } from "@/lib/api"
import { AuthContext } from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { ana, henry, run } from "@/test/fixtures"
import { MyRunsPage } from "./MyRunsPage"

afterEach(() => vi.restoreAllMocks())

const item = (over: Partial<RunListItem> = {}): RunListItem => ({
  id: 9, app_id: 3, app_name: "Eq. Research Files Prep", status: "done", output_folder: "E:\\Velha", person_name: "Henry",
  created_at: "2026-09-30T13:42:00Z", started_at: "2026-09-30T13:42:00Z", finished_at: "2026-09-30T13:43:00Z", file_count: 4, ...over,
})

const mount = (person = henry) =>
  render(
    <MemoryRouter>
      <AuthContext.Provider value={{ person, setPerson: () => {}, loading: false }}>
        <RunsProvider><MyRunsPage /></RunsProvider>
      </AuthContext.Provider>
    </MemoryRouter>,
  )

test("lists my runs with Ver e baixar and Rodar de novo in the same folder", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "runs").mockResolvedValue({ items: [item()], more: false })
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run())
  mount()
  expect(await screen.findByText("Eq. Research Files Prep")).toBeInTheDocument()
  expect(screen.getByText(/4 arquivos/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Ver e baixar" })).toHaveAttribute("href", "/execucoes/9")
  await user.click(screen.getByRole("button", { name: "Rodar de novo" }))
  expect(start).toHaveBeenCalledWith(3, "E:\\Velha", undefined)
})

test("admin can switch to De todos; others don't see the switch", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const runs = vi.spyOn(api, "runs").mockResolvedValue({ items: [item()], more: false })
  mount(henry)
  await user.click(await screen.findByRole("button", { name: "De todos" }))
  expect(runs).toHaveBeenLastCalledWith("all", undefined)
  runs.mockClear()
})

test("non-admin has no De todos", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "runs").mockResolvedValue({ items: [], more: false })
  mount(ana)
  expect(await screen.findByText(/Você ainda não rodou nenhum app/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "De todos" })).not.toBeInTheDocument()
})

test("Carregar mais asks for the next page", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const runs = vi.spyOn(api, "runs").mockResolvedValueOnce({ items: [item({ id: 9 })], more: true }).mockResolvedValueOnce({ items: [item({ id: 5 })], more: false })
  mount()
  await user.click(await screen.findByRole("button", { name: "Carregar mais" }))
  expect(runs).toHaveBeenLastCalledWith("mine", 9)
  expect(await screen.findAllByText("Eq. Research Files Prep")).toHaveLength(2)
})
```

Ver como `lib/auth.tsx` expõe o contexto (`AuthContext` ou outro nome) e como outros testes de página fornecem `person` (`grep -rn "AuthProvider\|useAuth" frontend/src/pages/*.test.tsx`); usar o mesmo jeito.

- [ ] **Step 2: Rodar e ver falhar**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run src/pages/MyRunsPage.test.tsx'`
Expected: FAIL

- [ ] **Step 3: Implementar `MyRunsPage.tsx`**

```tsx
import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { Play } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { EmptyState, PageHeader } from "@/components/bits"
import { api, type RunListItem } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useRuns } from "@/lib/runs-context"
import { formatDuration, formatWhen, isLive, STATUS } from "@/lib/runs"
import { cn } from "@/lib/utils"

export function MyRunsPage() {
  const { person } = useAuth()
  const { run, active } = useRuns()
  const [scope, setScope] = useState<"mine" | "all">("mine")
  const [items, setItems] = useState<RunListItem[] | null>(null)
  const [more, setMore] = useState(false)
  const [error, setError] = useState("")

  // Recarrega ao trocar o filtro e quando uma execução ativa muda de situação.
  const signal = active.map((r) => `${r.id}:${r.status}`).join(",")
  useEffect(() => {
    api.runs(scope).then((l) => { setItems(l.items); setMore(l.more) }, (e: Error) => setError(e.message))
  }, [scope, signal])

  const loadMore = () => {
    const last = items?.[items.length - 1]
    if (!last) return
    api.runs(scope, last.id).then((l) => { setItems((xs) => [...(xs ?? []), ...l.items]); setMore(l.more) }, (e: Error) => setError(e.message))
  }

  return (
    <>
      <PageHeader eyebrow="Minhas execuções" title="O que você rodou">
        {person?.role === "admin" && (
          <div className="flex gap-1 rounded-md border border-border bg-card p-1 text-sm">
            {(["mine", "all"] as const).map((s) => (
              <button key={s} type="button" onClick={() => setScope(s)}
                className={cn("rounded px-3 py-1", scope === s ? "bg-ink text-ink-foreground" : "text-muted-foreground hover:text-foreground")}>
                {s === "mine" ? "Minhas" : "De todos"}
              </button>
            ))}
          </div>
        )}
      </PageHeader>
      {error && <p className="text-destructive">{error}</p>}
      {items === null && !error && <p className="text-muted-foreground">Carregando…</p>}
      {items?.length === 0 && (
        <EmptyState title="Você ainda não rodou nenhum app">No catálogo, clique em Rodar no cartão do app. O resultado aparece aqui.</EmptyState>
      )}
      {items && items.length > 0 && (
        <ul className="divide-y divide-border rounded-lg border border-border bg-card shadow-card">
          {items.map((r) => (
            <li key={r.id} className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 px-5 py-4">
              <div className="min-w-0">
                <Link to={`/apps/${r.app_id}`} className="font-heading text-lg hover:text-stamp">{r.app_name}</Link>
                <p className="text-sm text-muted-foreground">
                  {formatWhen(r.created_at)} · {formatDuration(r.started_at, r.finished_at)} · {STATUS[r.status].label}
                  {!isLive(r.status) && ` · ${r.file_count} ${r.file_count === 1 ? "arquivo" : "arquivos"}`}
                  {scope === "all" && ` · ${r.person_name}`}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <Link to={`/execucoes/${r.id}`} className={buttonVariants({ variant: "outline", size: "sm" })}>Ver e baixar</Link>
                <Button size="sm" className="gap-1.5" onClick={() => run({ id: r.app_id, name: r.app_name, folder_display: r.output_folder, remembered_folder: "" }, { folder: r.output_folder })}>
                  <Play className="size-3.5" /> Rodar de novo
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {more && <Button variant="outline" className="mt-4" onClick={loadMore}>Carregar mais</Button>}
    </>
  )
}
```

- [ ] **Step 4: Rota, barra lateral e página da execução**

`App.tsx`: `<Route path="execucoes" element={<MyRunsPage />} />` antes de `execucoes/:id`.

`Layout.tsx`: em `NAV`, depois de Catálogo:
```ts
  { to: "/execucoes", label: "Minhas execuções", icon: History, roles: ["admin", "criador", "usuario"] },
```
(`History` de `lucide-react`). No `NavLink`, depois do `{label}`, mostrar o contador quando `to === "/execucoes"`:
```tsx
{to === "/execucoes" && openCount > 0 && (
  <span className="ml-auto rounded-full bg-stamp px-1.5 text-[0.65rem] leading-4 text-white">{openCount}</span>
)}
```
com `const openCount = useRuns().active.filter((r) => isLive(r.status)).length`. Como o `Layout` é quem monta o `RunsProvider`, separar: `Layout` renderiza `<RunsProvider><Shell /></RunsProvider>`, e o conteúdo atual vira `Shell` (que pode usar `useRuns`). `NavLink` do `/execucoes` precisa de `end` para não ficar marcado também em `/execucoes/:id`? Não: ficar marcado na página de uma execução é o certo (ela faz parte de Minhas execuções). Deixar sem `end`.

`RunPage.tsx`: o link de voltar passa a ser `<Link to="/execucoes">… Minhas execuções</Link>`; o nome do app no título vira `<Link to={`/apps/${run.app_id}`}>`; acrescentar perto do topo, quando a execução não está aberta:
```tsx
{!isLive(run.status) && (
  <Button size="sm" className="gap-1.5" onClick={() => runApp({ id: run.app_id, name: run.app_name, folder_display: run.output_folder, remembered_folder: "" }, { folder: run.output_folder })}>
    <Play className="size-3.5" /> Rodar de novo
  </Button>
)}
```
com `const { run: runApp } = useRuns()`. Em `RunPage.test.tsx`, envolver o render em `<RunsProvider>`, mockar `api.activeRuns` com `[]`, e acrescentar:

```tsx
test("Rodar de novo uses the same folder", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "done", output_folder: "E:\\Velha", app_id: 3 }))
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run())
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar de novo" }))
  expect(start).toHaveBeenCalledWith(3, "E:\\Velha", undefined)
})
```
(usar o nome real da chamada que a `RunPage` usa para buscar a execução — ver o arquivo.)

- [ ] **Step 5: Rodar a camada inteira**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run && mise x -- npx tsc -b'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add frontend/src
git commit -m "Minhas execuções: list, De todos for admin, Rodar de novo; sidebar counter"
```

---

### Task 10: Barra lateral, conferência visual, documentos e teste de verdade

**Files:**
- Possibly modify: `frontend/src/components/Layout.tsx`
- Create: `e2e/etapa1.mjs`
- Modify: `docs/TASKS.md`, `docs/PRD.md`, `docs/adr/007-faixa-e-pasta-lembrada.md` (novo)

- [ ] **Step 1: Barra lateral rolando de verdade**

Com o Vite rodando (o Hub do Henry já está na 8080; não subir outro ali), escrever `e2e/etapa1.mjs` que faz login de dev (`POST /api/dev/login` com `dev@apphub.local`), abre `/apps/1` numa janela de 1440×900, espera a ficha, rola até o fim (`page.mouse.wheel(0, 5000)`) e tira uma captura **da tela** (não `fullPage`), e confere com `page.evaluate` que o `aside` tem `getBoundingClientRect().bottom >= innerHeight - 1`. Se não tiver, corrigir no `Layout` (provável: `md:h-svh` → `md:h-dvh` e/ou `md:self-start`) e repetir.

- [ ] **Step 2: Capturas da Etapa 1** (no mesmo script, sem rodar nada de verdade: interceptar `**/api/runs/active` com `page.route` para devolver uma execução `running` e depois uma `done`)

Capturas: catálogo com Rodar; faixa "aberto" e "terminou"; Minhas execuções; ficha com "salva em … · trocar"; nos temas claro e escuro e no celular (390×844). Ler cada imagem e corrigir o que estiver desalinhado, apertado ou com pouco contraste.

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/e2e" && OUT=/tmp/claude-1000/etapa1 mise x -- node etapa1.mjs'`

- [ ] **Step 3: Documentos**

- `docs/TASKS.md`: em "Prioridade atual", item 3 vira "UX em 3 etapas (desenho em `docs/superpowers/specs/2026-10-01-etapa-1-…`)": Etapa 1 feita, Etapas 2 e 3 pendentes; na Fase 3/4, marcar o que esta etapa entregou (Rodar no cartão, Minhas execuções, faixa, pasta lembrada; "barra lateral" feita); tirar da lista de sobras da Fase 5 "a ficha e o histórico conferem a pasta duas vezes seguidas".
- `docs/PRD.md`: 3.3 ganha "Rodar no cartão, pasta lembrada por pessoa, faixa de andamento, Minhas execuções"; 3.4 cita o botão Rodar no cartão.
- `docs/adr/007-faixa-e-pasta-lembrada.md` (modelo da tech-stack): decisão de a faixa e a pasta lembrada ficarem no servidor (e não no navegador), com a conferência da pasta separada da ficha; alternativas: `localStorage` (não vale em outro navegador/aba), SSE (mais peças; consulta a cada 3 s basta para um piloto).

- [ ] **Step 4: Rodar tudo e commit**

Run: `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./... -count=1' && bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run && mise x -- npx tsc -b && mise x -- npm run build'`
Expected: PASS

```bash
git add -A && git commit -m "Etapa 1: visual check, docs, ADR 007" && git push
```

- [ ] **Step 5: Reiniciar o Hub do Henry e teste de verdade, um passo por vez**

Reiniciar o Hub real (o mesmo processo que roda na 8080, para a migração 006 entrar). Depois, com o Henry, uma ação por pergunta, conferindo pelos dados do Hub:
1. Faixa vazia ao abrir (as execuções antigas não aparecem).
2. Ficha do Files Prep abre na hora.
3. Rodar no cartão → janela com "Lembrar esta pasta" marcada → Abrir app.
4. Faixa "aberto…" enquanto usa; fecha o app → "terminou: N arquivos — Ver e baixar".
5. Rodar no cartão de novo → abre direto, sem janela.
6. Minhas execuções → Rodar de novo.
7. Barra lateral acompanha a rolagem na ficha.
