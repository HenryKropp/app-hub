# Etapa 2 — Cargos, permissões e ficha editável — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Trocar os 3 papéis fixos por cargos personalizáveis, áreas com líderes, acessos extras por app e apps restritos. A ficha (desenho B) passa a ter o técnico recolhido só para quem pode e um "Editar" por parte, incluindo os campos v0.2.

**Architecture:** Um pacote Go puro, `internal/access`, decide o que uma pessoa pode fazer com um app (`Person.For(App) Can`). O handler monta o `access.Person` uma vez por pedido (`loadAccess`), e todos os handlers usam esse resultado. O frontend recebe `can` pronto em cada app e `permissions` na pessoa logada, e nunca deduz regra sozinho. As edições da ficha vão por `PUT /api/apps/{id}` (manifesto inteiro, com conferência de `updated_at`), que reaproveita o caminho de publicação (`writeApp`). A tela Equipe (Pessoas, Áreas, Cargos) substitui Pessoas. Os editores de cada parte ficam em `components/ficha/editors.tsx` e servem à ficha e à tela de registro.

**Tech Stack:** Go stdlib `net/http`, pgx/v5, sqlc, migrações embutidas; React + TypeScript + Tailwind + shadcn/ui; Vitest + Testing Library; Playwright para fotos.

**Spec:** `docs/superpowers/specs/2026-10-05-etapa-2-permissoes-e-ficha-design.md` (protótipo da ficha: `docs/superpowers/specs/2026-10-05-prototipo-ficha.html`, desenho B)

## Global Constraints

- Todo texto de tela em português simples, sem jargão.
- SQL só em `backend/internal/database/queries/*.sql` + `cd backend && mise x -- sqlc generate`; nunca SQL no handler. Ler os `.go` gerados antes de usar os nomes (sqlc pode nomear campos diferente do que este plano supõe; ajuste o uso, não a intenção).
- Migração nova: `backend/internal/database/migrations/007_access.sql`. Roda dentro de uma transação (ver `database/migrate.go`).
- Comandos Go/sqlc rodam de `backend/` com `mise x --` e `set -a && . ../.env && set +a` antes dos testes. O frontend roda em `frontend/` com `mise x -- npm`.
- Banco local: container `app-hub-db`, porta 5433. **Nunca subir um Hub de teste na porta 8080.** O Hub real do Henry está lá (DEV_MODE=1, `backend/hub.log`); reiniciá-lo no fim, com o mesmo comando que já usa, para aplicar a migração 007.
- Permissões de cargo, nomes exatos: `register_apps`, `edit_any_app`, `restore_any_app`, `see_tech_any_app`, `see_all_runs`, `see_all_apps`, `manage_people`, `manage_areas`, `manage_roles`.
- Acessos extras, nomes exatos: `edit`, `restore`, `tech`, `runs`.
- Cargos prontos: `admin` "Administrador" (todas), `criador` "Criador" (`register_apps`), `usuario` "Usuário" (nenhuma). Os prontos podem ser renomeados e mudados, mas não apagados.
- Sempre existe ao menos uma pessoa cujo cargo tem `manage_people` **e** `manage_roles`.
- Responsável e reserva **não** veem execuções dos outros sem o acesso extra `runs`.
- App que a pessoa não pode ver responde **404 "App não encontrado."**, igual a um app que não existe. Execução de app invisível também é 404.
- `app-hub.yaml` guarda a área pelo **nome**; restrição e acessos ficam só no banco.
- Estilo visual: seguir `DESIGN.md` (violeta só no que se pode iniciar; "Editar" discreto em texto cinza; bloco recolhido com `<details>`).

## Review Focus

1. **Pessoa perde o acesso com a página aberta** (o líder restringe o app enquanto o colega está na ficha) → o próximo pedido dá 404 e a tela mostra "App não encontrado." com "Tentar de novo", sem quebrar. Teste no Task 3 (`TestRestrictedAppHidden`, mudança de acesso no meio).
2. **Dois "Salvar" na mesma ficha** (duas abas, ou duas pessoas) → o segundo recebe 409 com a frase de recarregar, e nada é escrito na pasta. Teste no Task 4 (`TestUpdateAppConflict`).
3. **Trocar o próprio cargo e perder a tela Equipe**, ou o último administrador se rebaixar → 409 "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos." Teste no Task 5 (`TestLastManagerProtected`, as três formas: trocar cargo, mudar caixas do cargo, apagar cargo).
4. **App antigo com área vazia ou área digitada fora da lista** → a migração cria "Sem área" e liga o app; publicar com área fora da lista dá 400 "Escolha a área do app na lista." Testes no Task 1 (migração) e Task 4.
5. **`depends_on` apontando para app restrito ou inexistente** → quem não vê o app restrito não descobre o nome dele (o bloco "Outros apps" só vai para quem vê o técnico); um id desconhecido aparece como "fora do catálogo" sem quebrar a ficha. Teste no Task 4 (`TestDependsLinks`).

---

## File Structure

**Backend**
- Create `backend/internal/database/migrations/007_access.sql` — cargos, áreas, líderes, membros, quem vê, acessos extras; passagem dos dados.
- Create `backend/internal/database/queries/access.sql` — consultas de cargos, áreas, acessos.
- Modify `backend/internal/database/queries/people.sql`, `apps.sql`, `runs.sql` — `role_id`, `area_id`, `restricted`, filtro de execuções por apps.
- Create `backend/internal/access/access.go` + `access_test.go` — regras puras (sem banco).
- Create `backend/internal/handler/access.go` — `loadAccess`, `appFor`, `requirePerm`, `personJSON` com permissões.
- Create `backend/internal/handler/team.go` + `team_test.go` — cargos, áreas, pessoas.
- Create `backend/internal/handler/app_access.go` + `app_access_test.go` — aberto/restrito, quem vê, acessos extras.
- Create `backend/internal/handler/permissions_test.go` — quem pode o quê nos handlers existentes.
- Modify `handler.go` (rotas, pessoas, `requireRole` sai), `apps.go` (`writeApp`, `updateApp`, catálogo filtrado, detalhe com `can`), `runs.go`, `history.go`, `versions.go`, `restore.go`, `agents.go`.
- Modify `backend/internal/manifest/manifest.go` + test — lista de opções sem opções é recusada.
- Modify `handler_test.go` (`resetSQL`), `runs_test.go` (`insertApp` com área).

**Frontend**
- Modify `src/lib/api.ts` — tipos `Person`, `Can`, `Role`, `Area`, `AppAccess`, chamadas novas.
- Create `src/lib/permissions.ts` + test — nomes em português e ajudantes (`has`, `canRegister`, `canSeeTeam`).
- Create `src/components/ficha/editors.tsx` + test — editores de cada parte.
- Create `src/components/ficha/Part.tsx` — bloco da ficha com "Editar", Salvar e Cancelar.
- Create `src/components/ficha/AccessSection.tsx` + test — bloco Acesso.
- Modify `src/pages/AppPage.tsx` + test — desenho B.
- Modify `src/components/ReviewForm.tsx` + test — editores compartilhados, área da lista, campos v0.2.
- Modify `src/pages/RegisterPage.tsx` — carrega áreas e apps para os editores.
- Create `src/pages/TeamPage.tsx` + test — Equipe com 3 abas. Delete `src/pages/PeoplePage.tsx`.
- Modify `App.tsx`, `Layout.tsx`, `CatalogPage.tsx`, `MyRunsPage.tsx`, `VersionPage.tsx`, `HistoryTimeline.tsx`, `lib/kinds.ts`, `test/fixtures.ts` e os testes que usam `role`.

**Docs**
- `docs/adr/009-cargos-areas-e-acessos.md`, `docs/PRD.md` (3.7), `docs/TASKS.md`, `DESIGN.md` (ficha B, Equipe), `e2e/etapa2.mjs`.

---

### Task 1: Migração 007, consultas e a pessoa logada com permissões

**Files:**
- Create: `backend/internal/database/migrations/007_access.sql`
- Create: `backend/internal/database/queries/access.sql`
- Modify: `backend/internal/database/queries/people.sql`, `backend/internal/database/queries/apps.sql`
- Create: `backend/internal/handler/access.go`
- Modify: `backend/internal/handler/handler.go` (personJSON, setup, login, devLogin, authStatus, listPeople, createPerson)
- Modify: `backend/internal/handler/handler_test.go`, `backend/internal/handler/runs_test.go`
- Test: `backend/internal/handler/access_migration_test.go`

**Interfaces:**
- Produces: tabelas `roles(id, key, name, permissions)`, `areas`, `area_leaders`, `area_members`, `app_viewers`, `app_grants`, `area_grants`; `people.role_id`; `apps.area_id`, `apps.restricted`.
- Produces: `personJSON{ID, Name, Email, RoleID int64, RoleName string, Permissions []string, Leads []int64, Areas []int64, SeesOthersRuns bool}` (JSON: `id,name,email,role_id,role_name,permissions,leads,areas,sees_others_runs`).
- Produces: `func (h *Handler) personOut(ctx context.Context, p sqlc.Person) (personJSON, error)`.
- Produces (test helper): `const resetSQL`, áreas de teste `Research` (id 1) e `Investimentos` (id 2) depois de cada reset.

- [ ] **Step 1: Escrever a migração**

`backend/internal/database/migrations/007_access.sql`:

```sql
-- Cargos personalizáveis (um por pessoa), áreas com líderes e membros,
-- apps restritos e acessos extras (Etapa 2).

CREATE TABLE IF NOT EXISTS roles (
    id          BIGSERIAL PRIMARY KEY,
    key         TEXT UNIQUE,              -- 'admin', 'criador', 'usuario' nos prontos; NULL nos criados
    name        TEXT NOT NULL UNIQUE,
    permissions TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO roles (key, name, permissions) VALUES
    ('admin', 'Administrador', ARRAY['register_apps','edit_any_app','restore_any_app','see_tech_any_app','see_all_runs','see_all_apps','manage_people','manage_areas','manage_roles']),
    ('criador', 'Criador', ARRAY['register_apps']),
    ('usuario', 'Usuário', ARRAY[]::TEXT[])
ON CONFLICT (key) DO NOTHING;

ALTER TABLE people ADD COLUMN IF NOT EXISTS role_id BIGINT REFERENCES roles(id);
UPDATE people p SET role_id = r.id FROM roles r WHERE r.key = p.role AND p.role_id IS NULL;
ALTER TABLE people ALTER COLUMN role_id SET NOT NULL;
ALTER TABLE people DROP COLUMN IF EXISTS role;

CREATE TABLE IF NOT EXISTS areas (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO areas (name) SELECT DISTINCT area FROM apps WHERE area <> '' ON CONFLICT (name) DO NOTHING;
INSERT INTO areas (name) VALUES ('Investimentos'), ('Operações'), ('Relações com Investidores'), ('Research'), ('Compliance'), ('Administrativo')
ON CONFLICT (name) DO NOTHING;
INSERT INTO areas (name) SELECT 'Sem área' WHERE EXISTS (SELECT 1 FROM apps WHERE area = '') ON CONFLICT (name) DO NOTHING;

ALTER TABLE apps ADD COLUMN IF NOT EXISTS area_id BIGINT REFERENCES areas(id);
UPDATE apps a SET area_id = ar.id,
                  area = ar.name,
                  manifest = jsonb_set(a.manifest, '{area}', to_jsonb(ar.name))
FROM areas ar
WHERE a.area_id IS NULL AND ar.name = CASE WHEN a.area = '' THEN 'Sem área' ELSE a.area END;
ALTER TABLE apps ALTER COLUMN area_id SET NOT NULL;
ALTER TABLE apps ADD COLUMN IF NOT EXISTS restricted BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS area_leaders (
    area_id   BIGINT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    person_id BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, person_id)
);

CREATE TABLE IF NOT EXISTS area_members (
    area_id   BIGINT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    person_id BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, person_id)
);

-- Who sees a restricted app: whole areas and/or single people.
CREATE TABLE IF NOT EXISTS app_viewers (
    id        BIGSERIAL PRIMARY KEY,
    app_id    BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    area_id   BIGINT REFERENCES areas(id) ON DELETE CASCADE,
    person_id BIGINT REFERENCES people(id) ON DELETE CASCADE,
    CHECK ((area_id IS NULL) <> (person_id IS NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS app_viewers_area_idx ON app_viewers(app_id, area_id) WHERE area_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS app_viewers_person_idx ON app_viewers(app_id, person_id) WHERE person_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS app_grants (
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    person_id  BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('edit', 'restore', 'tech', 'runs')),
    PRIMARY KEY (app_id, person_id, permission)
);

-- "Todos os apps da área": vale também para apps que entrarem depois.
CREATE TABLE IF NOT EXISTS area_grants (
    area_id    BIGINT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    person_id  BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('edit', 'restore', 'tech', 'runs')),
    PRIMARY KEY (area_id, person_id, permission)
);
```

- [ ] **Step 2: Atualizar `people.sql`**

Trocar as consultas de pessoas (o resto do arquivo fica):

```sql
-- name: CreatePerson :one
INSERT INTO people (name, email, password_hash, role_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpsertDevPerson :one
INSERT INTO people (name, email, password_hash, role_id)
VALUES ($1, $2, '', (SELECT id FROM roles WHERE key = 'admin'))
ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
RETURNING *;

-- name: ListPeople :many
SELECT p.id, p.name, p.email, p.role_id, r.name AS role_name, p.created_at,
       COALESCE((SELECT array_agg(m.area_id ORDER BY m.area_id) FROM area_members m WHERE m.person_id = p.id), '{}')::bigint[] AS area_ids
FROM people p
JOIN roles r ON r.id = p.role_id
ORDER BY p.name;

-- name: SetPersonRole :exec
UPDATE people SET role_id = $2 WHERE id = $1;
```

- [ ] **Step 3: Escrever `access.sql`**

`backend/internal/database/queries/access.sql`:

```sql
-- name: GetRole :one
SELECT * FROM roles WHERE id = $1;

-- name: GetRoleByKey :one
SELECT * FROM roles WHERE key = $1;

-- name: ListRoles :many
SELECT r.id, r.key, r.name, r.permissions, (SELECT count(*) FROM people p WHERE p.role_id = r.id) AS people_count
FROM roles r
ORDER BY r.id;

-- name: CreateRole :one
INSERT INTO roles (name, permissions) VALUES ($1, $2) RETURNING *;

-- name: UpdateRole :one
UPDATE roles SET name = $2, permissions = $3 WHERE id = $1 RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1;

-- name: CountManagersExceptPerson :one
SELECT count(*) FROM people p JOIN roles r ON r.id = p.role_id
WHERE r.permissions @> ARRAY['manage_people','manage_roles']::text[] AND p.id <> $1;

-- name: CountManagersExceptRole :one
SELECT count(*) FROM people p JOIN roles r ON r.id = p.role_id
WHERE r.permissions @> ARRAY['manage_people','manage_roles']::text[] AND r.id <> $1;

-- name: ListLedAreaIDs :many
SELECT area_id FROM area_leaders WHERE person_id = $1;

-- name: ListMemberAreaIDs :many
SELECT area_id FROM area_members WHERE person_id = $1;

-- name: ListPersonAppGrants :many
SELECT app_id, permission FROM app_grants WHERE person_id = $1;

-- name: ListPersonAreaGrants :many
SELECT area_id, permission FROM area_grants WHERE person_id = $1;

-- name: ListViewerAppIDs :many
SELECT DISTINCT v.app_id FROM app_viewers v
WHERE v.person_id = sqlc.arg(person_id)
   OR v.area_id IN (SELECT m.area_id FROM area_members m WHERE m.person_id = sqlc.arg(person_id));

-- name: ListAreas :many
SELECT a.id, a.name, (SELECT count(*) FROM apps x WHERE x.area_id = a.id) AS app_count
FROM areas a ORDER BY a.name;

-- name: ListAreaLeaders :many
SELECT l.area_id, p.id, p.name FROM area_leaders l JOIN people p ON p.id = l.person_id ORDER BY p.name;

-- name: ListAreaMembers :many
SELECT m.area_id, p.id, p.name FROM area_members m JOIN people p ON p.id = m.person_id ORDER BY p.name;

-- name: GetArea :one
SELECT * FROM areas WHERE id = $1;

-- name: GetAreaByName :one
SELECT * FROM areas WHERE name = $1;

-- name: CreateArea :one
INSERT INTO areas (name) VALUES ($1) RETURNING *;

-- name: RenameArea :one
UPDATE areas SET name = $2 WHERE id = $1 RETURNING *;

-- name: RenameAppsArea :exec
UPDATE apps SET area = sqlc.arg(name), manifest = jsonb_set(manifest, '{area}', to_jsonb(sqlc.arg(name)::text))
WHERE area_id = sqlc.arg(area_id);

-- name: DeleteArea :exec
DELETE FROM areas WHERE id = $1;

-- name: ClearAreaLeaders :exec
DELETE FROM area_leaders WHERE area_id = $1;

-- name: AddAreaLeader :exec
INSERT INTO area_leaders (area_id, person_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ClearAreaMembers :exec
DELETE FROM area_members WHERE area_id = $1;

-- name: AddAreaMember :exec
INSERT INTO area_members (area_id, person_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: SetAppRestricted :exec
UPDATE apps SET restricted = $2 WHERE id = $1;

-- name: ClearAppViewers :exec
DELETE FROM app_viewers WHERE app_id = $1;

-- name: AddAppViewerArea :exec
INSERT INTO app_viewers (app_id, area_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: AddAppViewerPerson :exec
INSERT INTO app_viewers (app_id, person_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListAppViewers :many
SELECT v.area_id, ar.name AS area_name, v.person_id, p.name AS person_name
FROM app_viewers v
LEFT JOIN areas ar ON ar.id = v.area_id
LEFT JOIN people p ON p.id = v.person_id
WHERE v.app_id = $1
ORDER BY ar.name, p.name;

-- name: ListAppGrants :many
SELECT g.person_id, p.name AS person_name, g.permission
FROM app_grants g JOIN people p ON p.id = g.person_id
WHERE g.app_id = $1 ORDER BY p.name, g.permission;

-- name: ClearPersonAppGrants :exec
DELETE FROM app_grants WHERE app_id = $1 AND person_id = $2;

-- name: AddAppGrant :exec
INSERT INTO app_grants (app_id, person_id, permission) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: ListAreaGrants :many
SELECT g.person_id, p.name AS person_name, g.permission
FROM area_grants g JOIN people p ON p.id = g.person_id
WHERE g.area_id = $1 ORDER BY p.name, g.permission;

-- name: ClearPersonAreaGrants :exec
DELETE FROM area_grants WHERE area_id = $1 AND person_id = $2;

-- name: AddAreaGrant :exec
INSERT INTO area_grants (area_id, person_id, permission) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;
```

- [ ] **Step 4: Atualizar `apps.sql`**

```sql
-- name: ListApps :many
SELECT a.id, a.slug, a.name, a.summary, a.kind, a.language, a.area, a.area_id, a.restricted, a.folder_path,
       a.owner_id, o.name AS owner_name, a.backup_id, a.created_at, a.updated_at,
       COALESCE(a.manifest->>'entrypoint', '')::text AS entrypoint
FROM apps a
JOIN people o ON o.id = a.owner_id
ORDER BY a.name;

-- name: UpsertApp :one
INSERT INTO apps (slug, name, summary, kind, language, area, area_id, folder_path, owner_id, backup_id, manifest, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (folder_path) DO UPDATE SET
    name = EXCLUDED.name,
    summary = EXCLUDED.summary,
    kind = EXCLUDED.kind,
    language = EXCLUDED.language,
    area = EXCLUDED.area,
    area_id = EXCLUDED.area_id,
    owner_id = EXCLUDED.owner_id,
    backup_id = EXCLUDED.backup_id,
    manifest = EXCLUDED.manifest,
    updated_at = now()
RETURNING *;

-- name: ListAppLinks :many
SELECT id, slug, name, area_id, owner_id, backup_id, restricted,
       COALESCE(manifest->'depends_on', '[]'::jsonb) AS depends_on
FROM apps ORDER BY name;
```

(`GetApp`, `ListAppIdentities`, `SlugExists`, `GetAppByFolder` ficam como estão.)

- [ ] **Step 5: Gerar o código**

Run: `cd backend && mise x -- sqlc generate`
Expected: sem erro. Abrir `internal/database/sqlc/models.go` e conferir `Person.RoleID int64`, `App.AreaID int64`, `App.Restricted bool`, `Role.Key pgtype.Text`, `Role.Permissions []string`.

- [ ] **Step 6: Reset dos testes**

Em `handler_test.go`, trocar os dois `TRUNCATE` (em `newClient` e `newPoolClient`) por `resetSQL`:

```go
// resetSQL empties the Hub and puts back the built-in roles and two areas
// (Research = 1, Investimentos = 2), so every test starts the same way.
const resetSQL = `
TRUNCATE run_files, agent_jobs, runs, app_changes, app_versions, code_blobs, pairings, computers,
         app_grants, app_viewers, area_grants, area_leaders, area_members, apps, sessions, people, areas
         RESTART IDENTITY CASCADE;
DELETE FROM roles WHERE key IS NULL;
UPDATE roles SET name = 'Administrador', permissions = ARRAY['register_apps','edit_any_app','restore_any_app','see_tech_any_app','see_all_runs','see_all_apps','manage_people','manage_areas','manage_roles'] WHERE key = 'admin';
UPDATE roles SET name = 'Criador', permissions = ARRAY['register_apps'] WHERE key = 'criador';
UPDATE roles SET name = 'Usuário', permissions = ARRAY[]::text[] WHERE key = 'usuario';
INSERT INTO areas (name) VALUES ('Research'), ('Investimentos');
`
```

Em `runs_test.go`, `insertApp` passa a gravar a área:

```go
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO apps (slug, name, kind, area, area_id, folder_path, owner_id, manifest, created_by)
		 VALUES ($1, $2, $3, 'Research', 1, '/mnt/c/apps/' || $1, 1, $4, 1) RETURNING id`, // one folder per slug, so a test can insert several apps
		m.ID, m.Name, m.Kind, raw).Scan(&id)
```

- [ ] **Step 7: Pessoa logada com permissões**

Criar `backend/internal/handler/access.go` (o `loadAccess` completo entra no Task 2; aqui só a pessoa):

```go
package handler

import (
	"context"
	"slices"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

type personJSON struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	RoleID         int64    `json:"role_id"`
	RoleName       string   `json:"role_name"`
	Permissions    []string `json:"permissions"`
	Leads          []int64  `json:"leads"`
	Areas          []int64  `json:"areas"`
	SeesOthersRuns bool     `json:"sees_others_runs"`
}

// personOut is the logged-in person as the frontend needs it: role, what it
// allows, and the areas the person leads or belongs to.
func (h *Handler) personOut(ctx context.Context, p sqlc.Person) (personJSON, error) {
	role, err := h.q.GetRole(ctx, p.RoleID)
	if err != nil {
		return personJSON{}, err
	}
	leads, err := h.q.ListLedAreaIDs(ctx, p.ID)
	if err != nil {
		return personJSON{}, err
	}
	areas, err := h.q.ListMemberAreaIDs(ctx, p.ID)
	if err != nil {
		return personJSON{}, err
	}
	out := personJSON{ID: p.ID, Name: p.Name, Email: p.Email, RoleID: role.ID, RoleName: role.Name,
		Permissions: role.Permissions, Leads: leads, Areas: areas}
	if out.Permissions == nil {
		out.Permissions = []string{}
	}
	if out.Leads == nil {
		out.Leads = []int64{}
	}
	if out.Areas == nil {
		out.Areas = []int64{}
	}
	out.SeesOthersRuns = slices.Contains(out.Permissions, "see_all_runs") || len(out.Leads) > 0
	if !out.SeesOthersRuns {
		grants, err := h.q.ListPersonAppGrants(ctx, p.ID)
		if err != nil {
			return personJSON{}, err
		}
		areaGrants, err := h.q.ListPersonAreaGrants(ctx, p.ID)
		if err != nil {
			return personJSON{}, err
		}
		for _, g := range grants {
			out.SeesOthersRuns = out.SeesOthersRuns || g.Permission == "runs"
		}
		for _, g := range areaGrants {
			out.SeesOthersRuns = out.SeesOthersRuns || g.Permission == "runs"
		}
	}
	return out, nil
}
```

Em `handler.go`: apagar o `personJSON` antigo e `toPersonJSON`. Em `authStatus`, `setup`, `login` e `devLogin`, trocar `toPersonJSON(p)` por:

```go
	out, err := h.personOut(r.Context(), p)
	if err != nil {
		internalError(w, "person", err)
		return
	}
	writeJSON(w, http.StatusOK, out) // (StatusCreated em setup)
```

(Em `authStatus`, `resp["person"] = out` só quando `personOut` não falha.)

`setup` cria a pessoa com o cargo pronto `admin`:

```go
	admin, err := h.q.GetRoleByKey(r.Context(), pgtype.Text{String: "admin", Valid: true})
	if err != nil {
		internalError(w, "admin role", err)
		return
	}
	p, err := h.q.CreatePerson(r.Context(), sqlc.CreatePersonParams{Name: c.Name, Email: c.Email, PasswordHash: string(hash), RoleID: admin.ID})
```

`listPeople` devolve a lista nova:

```go
type personRow struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	RoleID   int64   `json:"role_id"`
	RoleName string  `json:"role_name"`
	AreaIDs  []int64 `json:"area_ids"`
}
```

preenchido de `ListPeopleRow` (sem `Permissions`: só quem está logado recebe as suas).

`credentials` ganha `RoleID int64 \`json:"role_id"\``. Em `createPerson`: a permissão vira `h.requirePerm(w, r, access.ManagePeople)` (criada no Task 2; até lá, deixe `currentPerson` e o teste de permissão para o Task 3). Resolver o cargo assim — `role` (chave do cargo pronto) continua aceito, para os testes antigos:

```go
	roleID := c.RoleID
	if roleID == 0 && c.Role != "" {
		role, err := h.q.GetRoleByKey(r.Context(), pgtype.Text{String: c.Role, Valid: true})
		if err == nil {
			roleID = role.ID
		}
	}
	if _, err := h.q.GetRole(r.Context(), roleID); err != nil {
		writeError(w, http.StatusBadRequest, "Escolha o cargo da pessoa.")
		return
	}
```

e a resposta é um `personRow`.

`requireRole` e todos os usos de `p.Role`/`me.Role` deixam de compilar. **Nesta tarefa**, troque cada uso para manter o comportamento de hoje enquanto o Task 3 não chega, por meio de um ajudante temporário em `access.go`:

```go
// roleKey is temporary (removed in Task 3): the built-in key of the person's role.
func (h *Handler) roleKey(ctx context.Context, p sqlc.Person) string {
	role, err := h.q.GetRole(ctx, p.RoleID)
	if err != nil || !role.Key.Valid {
		return ""
	}
	return role.Key.String
}
```

`requireRole` passa a comparar `h.roleKey(r.Context(), p)`; `me.Role == "admin"` vira `h.roleKey(ctx, me) == "admin"`.

`UpsertApp` ganhou `AreaID`: em `saveApp`, resolver a área pelo nome antes do `UpsertApp`:

```go
	area, err := h.q.GetAreaByName(r.Context(), m.Area)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "Escolha a área do app na lista.")
		return
	}
	if err != nil {
		internalError(w, "get area", err)
		return
	}
```

e passar `AreaID: area.ID` no `UpsertAppParams`.

- [ ] **Step 8: Teste da migração e da pessoa**

`backend/internal/handler/access_migration_test.go`:

```go
package handler

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestMigrationAccessTables(t *testing.T) {
	ctx := context.Background()
	for _, table := range []string{"roles", "areas", "area_leaders", "area_members", "app_viewers", "app_grants", "area_grants"} {
		var ok bool
		if err := testPool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil || !ok {
			t.Fatalf("table %s missing (%v)", table, err)
		}
	}
	var n int
	if err := testPool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE key IN ('admin','criador','usuario')").Scan(&n); err != nil || n != 3 {
		t.Fatalf("built-in roles = %d (%v)", n, err)
	}
}

func TestLoggedPersonCarriesPermissions(t *testing.T) {
	c := newClient(t, false)
	admin := setupAdmin(t, c)
	if admin.RoleName != "Administrador" || !slices.Contains(admin.Permissions, "manage_roles") || !admin.SeesOthersRuns {
		t.Fatalf("admin = %+v", admin)
	}
	createPersonAs(t, c, "Bia", "bia@alis.com", "usuario")
	bia := c.fresh()
	loginAs(t, bia, "bia@alis.com")
	var st struct {
		Person personJSON `json:"person"`
	}
	if code := bia.do("GET", "/api/auth/status", nil, &st); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if st.Person.RoleName != "Usuário" || len(st.Person.Permissions) != 0 || st.Person.SeesOthersRuns {
		t.Fatalf("bia = %+v", st.Person)
	}
}

func TestPublishNeedsAreaFromList(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	dir := t.TempDir()
	m := map[string]any{"name": "X", "kind": "reference", "area": "Inventada"}
	var out map[string]any
	if code := c.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 1, "manifest": m}, &out); code != http.StatusBadRequest || out["error"] != "Escolha a área do app na lista." {
		t.Fatalf("code %d %v", code, out)
	}
}
```

Adicionar um teste que roda a parte de dados da migração sobre dados como os de hoje. Ele recria a situação antiga dentro de uma transação desfeita no fim:

```go
func TestMigrationMovesOldData(t *testing.T) {
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Old shape: a role text column and free-text areas.
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE old_people (id int, role text);
		INSERT INTO old_people VALUES (1, 'criador');
		CREATE TEMP TABLE old_apps (id int, area text);
		INSERT INTO old_apps VALUES (1, ''), (2, 'Mesa de Operações');`); err != nil {
		t.Fatal(err)
	}
	var roleName string
	if err := tx.QueryRow(ctx, `SELECT r.name FROM old_people p JOIN roles r ON r.key = p.role`).Scan(&roleName); err != nil || roleName != "Criador" {
		t.Fatalf("role mapping %q %v", roleName, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO areas (name) SELECT DISTINCT area FROM old_apps WHERE area <> '' ON CONFLICT (name) DO NOTHING;
		INSERT INTO areas (name) SELECT 'Sem área' WHERE EXISTS (SELECT 1 FROM old_apps WHERE area = '') ON CONFLICT (name) DO NOTHING;`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM areas WHERE name IN ('Sem área', 'Mesa de Operações')`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("areas from old data = %d %v", n, err)
	}
}
```

- [ ] **Step 9: Rodar os testes Go**

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go build ./... && mise x -- go test ./...`
Expected: tudo PASS. Os testes antigos (`createPersonAs` com `"usuario"`/`"criador"`, `codeApp` com área `Research`) continuam passando. Se algum teste antigo comparar `personJSON.Role`, troque por `RoleName`.

- [ ] **Step 10: Commit**

```bash
git add backend
git commit -m "Access: roles, areas, grants tables; logged person carries permissions"
```

---

### Task 2: Pacote `access` (regras puras) e o carregador

**Files:**
- Create: `backend/internal/access/access.go`
- Test: `backend/internal/access/access_test.go`
- Modify: `backend/internal/handler/access.go`

**Interfaces:**
- Consumes: consultas do Task 1 (`GetRole`, `ListLedAreaIDs`, `ListMemberAreaIDs`, `ListPersonAppGrants`, `ListPersonAreaGrants`, `ListViewerAppIDs`, `ListAppLinks`).
- Produces (pacote `access`):
  - constantes `RegisterApps … ManageRoles` e `RolePerms []string`; `GrantEdit, GrantRestore, GrantTech, GrantRuns` e `Grants []string`.
  - `type Person struct{ ID int64; Perms, Leads, Member, ViewerApps (sets); AppGrants map[int64]map[string]bool; AreaGrants map[int64]map[string]bool }`
  - `func (p *Person) Has(perm string) bool`, `func (p *Person) CanRegister() bool`, `func (p *Person) SeesTeam() bool`
  - `type App struct{ ID, AreaID, OwnerID, BackupID int64; Restricted bool }`
  - `type Can struct{ View, Edit, Restore, Tech, Runs, Access, Grants bool }` (JSON `view, edit, restore, tech, runs, access, grants`)
  - `func (p *Person) For(a App) Can`
- Produces (handler):
  - `func (h *Handler) loadAccess(ctx context.Context, me sqlc.Person) (*access.Person, error)`
  - `func appRef(a sqlc.App) access.App`
  - `func (h *Handler) requirePerm(w http.ResponseWriter, r *http.Request, perm string) (sqlc.Person, *access.Person, bool)`
  - `func (h *Handler) appFor(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, *access.Person, access.Can, bool)` — 404 quando o app não existe **ou** a pessoa não pode vê-lo.

- [ ] **Step 1: Escrever o teste das regras**

`backend/internal/access/access_test.go`:

```go
package access

import "testing"

func person(id int64) *Person {
	return &Person{ID: id, Perms: map[string]bool{}, Leads: map[int64]bool{}, Member: map[int64]bool{},
		ViewerApps: map[int64]bool{}, AppGrants: map[int64]map[string]bool{}, AreaGrants: map[int64]map[string]bool{}}
}

var app = App{ID: 10, AreaID: 1, OwnerID: 1, BackupID: 2}

func TestColleagueOnOpenApp(t *testing.T) {
	c := person(9).For(app)
	if !c.View || c.Edit || c.Restore || c.Tech || c.Runs || c.Access || c.Grants {
		t.Fatalf("%+v", c)
	}
}

func TestOwnerAndBackupCareButDoNotSeeOthersRuns(t *testing.T) {
	for _, id := range []int64{1, 2} {
		c := person(id).For(app)
		if !c.View || !c.Edit || !c.Restore || !c.Tech || !c.Access || c.Runs || c.Grants {
			t.Fatalf("person %d: %+v", id, c)
		}
	}
}

func TestLeaderDoesEverythingInArea(t *testing.T) {
	p := person(5)
	p.Leads[1] = true
	c := p.For(App{ID: 10, AreaID: 1, OwnerID: 1, Restricted: true})
	if !(c.View && c.Edit && c.Restore && c.Tech && c.Runs && c.Access && c.Grants) {
		t.Fatalf("%+v", c)
	}
	if other := p.For(App{ID: 11, AreaID: 2, OwnerID: 1}); other.Edit || other.Grants {
		t.Fatalf("leader of area 1 in area 2: %+v", other)
	}
}

func TestRolePermissions(t *testing.T) {
	cases := []struct {
		perm string
		want func(Can) bool
	}{
		{EditAnyApp, func(c Can) bool { return c.Edit && c.Tech && !c.Restore && !c.Runs }},
		{RestoreAnyApp, func(c Can) bool { return c.Restore && c.Tech && !c.Edit }},
		{SeeTechAnyApp, func(c Can) bool { return c.Tech && !c.Edit }},
		{SeeAllRuns, func(c Can) bool { return c.Runs && !c.Tech }},
		{ManageAreas, func(c Can) bool { return c.Grants && c.Access && !c.Edit }},
	}
	for _, tc := range cases {
		p := person(9)
		p.Perms[tc.perm] = true
		if c := p.For(app); !tc.want(c) {
			t.Fatalf("%s: %+v", tc.perm, c)
		}
	}
}

func TestGrantsPerAppAndPerArea(t *testing.T) {
	p := person(9)
	p.AppGrants[10] = map[string]bool{GrantEdit: true}
	if c := p.For(app); !c.Edit || !c.Tech || c.Restore {
		t.Fatalf("app grant: %+v", c)
	}
	if c := p.For(App{ID: 11, AreaID: 1, OwnerID: 1}); c.Edit {
		t.Fatal("app grant leaked to another app")
	}
	q := person(9)
	q.AreaGrants[1] = map[string]bool{GrantRuns: true}
	if c := q.For(App{ID: 99, AreaID: 1, OwnerID: 1}); !c.Runs || c.Edit {
		t.Fatalf("area grant on a later app: %+v", c)
	}
}

func TestRestrictedVisibility(t *testing.T) {
	r := App{ID: 10, AreaID: 1, OwnerID: 1, Restricted: true}
	if person(9).For(r).View {
		t.Fatal("stranger sees restricted app")
	}
	v := person(9)
	v.ViewerApps[10] = true
	if c := v.For(r); !c.View || c.Edit {
		t.Fatalf("listed viewer: %+v", c)
	}
	all := person(9)
	all.Perms[SeeAllApps] = true
	if !all.For(r).View {
		t.Fatal("see_all_apps")
	}
	g := person(9)
	g.AppGrants[10] = map[string]bool{GrantRuns: true}
	if !g.For(r).View {
		t.Fatal("someone with an extra access must see the app")
	}
}

func TestRegisterAndTeam(t *testing.T) {
	if person(9).CanRegister() || person(9).SeesTeam() {
		t.Fatal("colleague")
	}
	l := person(9)
	l.Leads[1] = true
	if !l.CanRegister() || !l.SeesTeam() {
		t.Fatal("leader registers and sees Equipe")
	}
	c := person(9)
	c.Perms[RegisterApps] = true
	if !c.CanRegister() || c.SeesTeam() {
		t.Fatal("creator")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && mise x -- go test ./internal/access/`
Expected: FAIL (pacote não existe).

- [ ] **Step 3: Escrever o pacote**

`backend/internal/access/access.go`:

```go
// Package access decides what a person may do in the Hub. It is pure (no
// database): the handler loads the facts once per request and asks here.
package access

// Role permissions: company-wide, ticked per role in Equipe → Cargos.
const (
	RegisterApps  = "register_apps"
	EditAnyApp    = "edit_any_app"
	RestoreAnyApp = "restore_any_app"
	SeeTechAnyApp = "see_tech_any_app"
	SeeAllRuns    = "see_all_runs"
	SeeAllApps    = "see_all_apps"
	ManagePeople  = "manage_people"
	ManageAreas   = "manage_areas"
	ManageRoles   = "manage_roles"
)

var RolePerms = []string{RegisterApps, EditAnyApp, RestoreAnyApp, SeeTechAnyApp, SeeAllRuns, SeeAllApps, ManagePeople, ManageAreas, ManageRoles}

// Extra accesses an area leader gives to one person, per app or for the whole area.
const (
	GrantEdit    = "edit"
	GrantRestore = "restore"
	GrantTech    = "tech"
	GrantRuns    = "runs"
)

var Grants = []string{GrantEdit, GrantRestore, GrantTech, GrantRuns}

type Person struct {
	ID         int64
	Perms      map[string]bool
	Leads      map[int64]bool // area ids
	Member     map[int64]bool // area ids
	ViewerApps map[int64]bool // restricted apps that list the person or one of their areas
	AppGrants  map[int64]map[string]bool
	AreaGrants map[int64]map[string]bool
}

func (p *Person) Has(perm string) bool { return p.Perms[perm] }

// CanRegister: the role allows it, or the person leads an area.
func (p *Person) CanRegister() bool { return p.Has(RegisterApps) || len(p.Leads) > 0 }

// SeesTeam: Equipe shows for whoever manages people, areas or roles, or leads an area.
func (p *Person) SeesTeam() bool {
	return p.Has(ManagePeople) || p.Has(ManageAreas) || p.Has(ManageRoles) || len(p.Leads) > 0
}

type App struct {
	ID, AreaID, OwnerID, BackupID int64 // BackupID 0 = no backup
	Restricted                    bool
}

type Can struct {
	View    bool `json:"view"`    // see it in the catalog and run it
	Edit    bool `json:"edit"`    // change the ficha
	Restore bool `json:"restore"` // go back to an older version
	Tech    bool `json:"tech"`    // see the technical part of the ficha
	Runs    bool `json:"runs"`    // see everyone's runs of this app
	Access  bool `json:"access"`  // open/restrict the app and choose who sees it
	Grants  bool `json:"grants"`  // give and take extra accesses
}

func (p *Person) For(a App) Can {
	leader := p.Leads[a.AreaID]
	cares := leader || a.OwnerID == p.ID || (a.BackupID != 0 && a.BackupID == p.ID)
	granted := func(g string) bool { return p.AppGrants[a.ID][g] || p.AreaGrants[a.AreaID][g] }

	var c Can
	c.Edit = cares || p.Has(EditAnyApp) || granted(GrantEdit)
	c.Restore = cares || p.Has(RestoreAnyApp) || granted(GrantRestore)
	c.Tech = c.Edit || c.Restore || p.Has(SeeTechAnyApp) || granted(GrantTech)
	c.Runs = leader || p.Has(SeeAllRuns) || granted(GrantRuns)
	c.Grants = leader || p.Has(ManageAreas)
	c.Access = cares || c.Grants
	c.View = !a.Restricted || p.Has(SeeAllApps) || p.ViewerApps[a.ID] || c.Access || c.Tech || c.Runs
	return c
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && mise x -- go test ./internal/access/`
Expected: PASS.

- [ ] **Step 5: Carregador no handler**

Acrescentar a `backend/internal/handler/access.go` (imports: `errors`, `net/http`, `strconv`, `pgx`, `access`):

```go
// loadAccess gathers everything access.Person needs about me, in a few queries.
func (h *Handler) loadAccess(ctx context.Context, me sqlc.Person) (*access.Person, error) {
	role, err := h.q.GetRole(ctx, me.RoleID)
	if err != nil {
		return nil, err
	}
	p := &access.Person{ID: me.ID, Perms: map[string]bool{}, Leads: map[int64]bool{}, Member: map[int64]bool{},
		ViewerApps: map[int64]bool{}, AppGrants: map[int64]map[string]bool{}, AreaGrants: map[int64]map[string]bool{}}
	for _, perm := range role.Permissions {
		p.Perms[perm] = true
	}
	leads, err := h.q.ListLedAreaIDs(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range leads {
		p.Leads[id] = true
	}
	member, err := h.q.ListMemberAreaIDs(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range member {
		p.Member[id] = true
	}
	viewer, err := h.q.ListViewerAppIDs(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range viewer {
		p.ViewerApps[id] = true
	}
	grants, err := h.q.ListPersonAppGrants(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, g := range grants {
		if p.AppGrants[g.AppID] == nil {
			p.AppGrants[g.AppID] = map[string]bool{}
		}
		p.AppGrants[g.AppID][g.Permission] = true
	}
	areaGrants, err := h.q.ListPersonAreaGrants(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	for _, g := range areaGrants {
		if p.AreaGrants[g.AreaID] == nil {
			p.AreaGrants[g.AreaID] = map[string]bool{}
		}
		p.AreaGrants[g.AreaID][g.Permission] = true
	}
	return p, nil
}

func appRef(a sqlc.App) access.App {
	return access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted}
}

// requirePerm checks a role permission; 403 in plain Portuguese otherwise.
func (h *Handler) requirePerm(w http.ResponseWriter, r *http.Request, perm string) (sqlc.Person, *access.Person, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return me, nil, false
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return me, nil, false
	}
	if !acc.Has(perm) {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return me, acc, false
	}
	return me, acc, true
}

// appFor loads the app in the URL and what me can do with it. An app me
// cannot see answers 404, the same as one that does not exist.
func (h *Handler) appFor(w http.ResponseWriter, r *http.Request) (sqlc.App, sqlc.Person, *access.Person, access.Can, bool) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return sqlc.App{}, me, nil, access.Can{}, false
	}
	a, ok := h.appFromPath(w, r)
	if !ok {
		return a, me, nil, access.Can{}, false
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return a, me, nil, access.Can{}, false
	}
	can := acc.For(appRef(a))
	if !can.View {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return a, me, acc, can, false
	}
	return a, me, acc, can, true
}
```

(Conferir em `sqlc/models.go` que `App.BackupID` é `pgtype.Int8`; se não for, ajustar `appRef`.)

- [ ] **Step 6: Build e testes**

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go vet ./... && mise x -- go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend
git commit -m "Access: pure rules package and per-request loader"
```

---

### Task 3: Os handlers de hoje passam a obedecer

**Files:**
- Modify: `backend/internal/handler/apps.go`, `history.go`, `versions.go`, `restore.go`, `runs.go`, `agents.go`, `handler.go`
- Modify: `backend/internal/database/queries/runs.sql`
- Delete from `access.go`: `roleKey`; from `handler.go`: `requireRole`
- Test: `backend/internal/handler/permissions_test.go`

**Interfaces:**
- Consumes: `loadAccess`, `appFor`, `requirePerm`, `access.Can`.
- Produces: `appSummary` e `appDetail` com `AreaID int64 \`json:"area_id"\``, `Restricted bool \`json:"restricted"\``, `Can access.Can \`json:"can"\``.
- Produces: `ListRuns` com o parâmetro `AppIds []int64` (execuções dos outros nesses apps).
- Produces (test helpers em `permissions_test.go`): `func setRole(t, key string, perms ...string)`, `func makeLeader(t, areaID, personID int64)`, `func grant(t, appID, personID int64, perm string)`, `func restrict(t, appID int64)`, `func ownerOf(t, appID, personID int64)`.

- [ ] **Step 1: Escrever os testes de permissão**

`backend/internal/handler/permissions_test.go`:

```go
package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
)

func exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func makeLeader(t *testing.T, areaID, personID int64) {
	exec(t, "INSERT INTO area_leaders (area_id, person_id) VALUES ($1, $2)", areaID, personID)
}

func grant(t *testing.T, appID, personID int64, perm string) {
	exec(t, "INSERT INTO app_grants (app_id, person_id, permission) VALUES ($1, $2, $3)", appID, personID, perm)
}

func restrict(t *testing.T, appID int64) { exec(t, "UPDATE apps SET restricted = true WHERE id = $1", appID) }

func ownerOf(t *testing.T, appID, personID int64) {
	exec(t, "UPDATE apps SET owner_id = $2 WHERE id = $1", appID, personID)
}

// team: admin Henry (1), criador Ana (2), usuário Bia (3); one app owned by Henry.
func team(t *testing.T) (admin, ana, bia *client, appID int64) {
	t.Helper()
	admin, _ = newPoolClient(t)
	setupAdmin(t, admin)
	createPersonAs(t, admin, "Ana", "ana@alis.com", "criador")
	createPersonAs(t, admin, "Bia", "bia@alis.com", "usuario")
	ana, bia = admin.fresh(), admin.fresh()
	loginAs(t, ana, "ana@alis.com")
	loginAs(t, bia, "bia@alis.com")
	m := filesPrepManifest()
	appID = insertApp(t, m)
	return
}

func TestRestrictedAppHidden(t *testing.T) {
	admin, _, bia, id := team(t)
	var list []appSummary
	bia.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || !list[0].Can.View || list[0].Can.Edit {
		t.Fatalf("open app for bia: %+v", list)
	}
	restrict(t, id)
	bia.do("GET", "/api/apps", nil, &list)
	if len(list) != 0 {
		t.Fatalf("restricted app listed: %+v", list)
	}
	for _, path := range []string{appPath(id, ""), appPath(id, "/runs")} {
		if code := bia.do("GET", path, nil, nil); code != http.StatusNotFound {
			t.Fatalf("GET %s = %d", path, code)
		}
	}
	if code := bia.do("POST", appPath(id, "/runs"), map[string]any{"output_folder": `C:\x`}, nil); code != http.StatusNotFound {
		t.Fatalf("run restricted = %d", code)
	}
	exec(t, "INSERT INTO app_viewers (app_id, person_id) VALUES ($1, 3)", id)
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusOK {
		t.Fatalf("listed viewer = %d", code)
	}
	admin.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || !list[0].Restricted {
		t.Fatalf("admin list %+v", list)
	}
}

func TestCreatorEditsOnlyOwnApps(t *testing.T) {
	_, ana, _, id := team(t)
	var d appDetail
	ana.do("GET", appPath(id, ""), nil, &d)
	if d.Can.Edit || d.Can.Tech {
		t.Fatalf("criador on someone else's app: %+v", d.Can)
	}
	if code := ana.do("GET", appPath(id, "/history"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("history without tech = %d", code)
	}
	ownerOf(t, id, 2)
	ana.do("GET", appPath(id, ""), nil, &d)
	if !d.Can.Edit || !d.Can.Tech || d.Can.Runs {
		t.Fatalf("owner: %+v", d.Can)
	}
}

func TestGrantGivesOneThing(t *testing.T) {
	_, _, bia, id := team(t)
	grant(t, id, 3, "tech")
	var d appDetail
	bia.do("GET", appPath(id, ""), nil, &d)
	if !d.Can.Tech || d.Can.Edit || d.Can.Restore {
		t.Fatalf("tech grant: %+v", d.Can)
	}
	if code := bia.do("GET", appPath(id, "/history"), nil, nil); code != http.StatusOK {
		t.Fatalf("history with tech = %d", code)
	}
}

func TestLeaderRunsVisibility(t *testing.T) {
	admin, ana, bia, id := team(t)
	_ = admin
	exec(t, `INSERT INTO runs (app_id, person_id, status, output_folder) VALUES ($1, 3, 'done', 'C:\x')`, id)
	var runs []runSummaryJSON
	ana.do("GET", appPath(id, "/runs"), nil, &runs)
	if len(runs) != 0 {
		t.Fatal("criador sees bia's run")
	}
	makeLeader(t, 1, 2)
	ana.do("GET", appPath(id, "/runs"), nil, &runs)
	if len(runs) != 1 {
		t.Fatal("leader must see the area's runs")
	}
	var all runListJSON
	if code := ana.do("GET", "/api/runs?scope=all", nil, &all); code != http.StatusOK || len(all.Items) != 1 {
		t.Fatalf("leader scope=all %d %+v", code, all)
	}
	if code := bia.do("GET", "/api/runs?scope=all", nil, nil); code != http.StatusForbidden {
		t.Fatalf("bia scope=all = %d", code)
	}
}

func TestOwnerDoesNotSeeOthersRuns(t *testing.T) {
	_, ana, _, id := team(t)
	ownerOf(t, id, 2)
	exec(t, `INSERT INTO runs (app_id, person_id, status, output_folder) VALUES ($1, 3, 'done', 'C:\x')`, id)
	var runs []runSummaryJSON
	ana.do("GET", appPath(id, "/runs"), nil, &runs)
	if len(runs) != 0 {
		t.Fatal("owner without the runs access sees others' runs")
	}
	var run map[string]any
	if code := ana.do("GET", "/api/runs/1", nil, &run); code != http.StatusNotFound {
		t.Fatalf("someone else's run = %d", code)
	}
}

func TestRegisterNeedsPermissionOrLeadership(t *testing.T) {
	_, _, bia, _ := team(t)
	if code := bia.do("POST", "/api/scan", map[string]string{"path": t.TempDir(), "mode": "single"}, nil); code != http.StatusForbidden {
		t.Fatalf("usuario scan = %d", code)
	}
	makeLeader(t, 2, 3)
	if code := bia.do("POST", "/api/scan", map[string]string{"path": t.TempDir(), "mode": "single"}, nil); code != http.StatusOK {
		t.Fatalf("leader scan = %d", code)
	}
}

func TestRepublishNeedsEdit(t *testing.T) {
	admin, ana, _, _ := team(t)
	id, dir := codeApp(t, admin) // owned by Henry (1)
	m := manifest.Manifest{Name: "App Teste", Kind: manifest.KindScript, Language: "python", Entrypoint: "src/main.py", Area: "Research"}
	if code := ana.do("POST", "/api/apps", map[string]any{"folder_path": dir, "owner_id": 2, "manifest": m}, nil); code != http.StatusForbidden {
		t.Fatalf("criador republishing someone else's app = %d", code)
	}
	_ = id
}

func TestRestoreAndDescribeNeedPermission(t *testing.T) {
	admin, ana, _, _ := team(t)
	id, _ := codeApp(t, admin)
	if code := ana.do("GET", appPath(id, "/versions/1/restore-preview"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("restore preview = %d", code)
	}
	if code := ana.do("PATCH", appPath(id, "/versions/1"), map[string]any{"description": "x"}, nil); code != http.StatusForbidden {
		t.Fatalf("describe = %d", code)
	}
	grant(t, id, 2, "restore")
	if code := ana.do("GET", appPath(id, "/versions/1/restore-preview"), nil, nil); code == http.StatusForbidden || code == http.StatusNotFound {
		t.Fatalf("restore with grant = %d", code)
	}
}
```

(Se `runs` exigir outras colunas `NOT NULL` no `INSERT`, conferir `003_agents_runs.sql` e preencher. Se `filesPrepManifest` não existir com esse nome em `runs_test.go`, usar a função de manifesto de exemplo que estiver lá.)

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/handler/ -run 'Restricted|CreatorEdits|GrantGives|LeaderRuns|OwnerDoesNot|RegisterNeeds|RepublishNeeds|RestoreAndDescribe'`
Expected: FAIL (falta `Can` nos tipos; regras ainda antigas).

- [ ] **Step 3: Filtro das execuções por apps**

Em `runs.sql`, `ListRuns`:

```sql
WHERE (sqlc.arg(see_all)::boolean OR r.person_id = sqlc.arg(person_id) OR r.app_id = ANY(sqlc.arg(app_ids)::bigint[]))
```

Run: `cd backend && mise x -- sqlc generate`

- [ ] **Step 4: Trocar as regras, handler por handler**

`apps.go`:
- `appSummary` ganha `AreaID`, `Restricted` (já vêm de `ListAppsRow`) e `Can access.Can \`json:"can"\``. `listApps` carrega `acc := h.loadAccess(...)` e só põe na lista os apps com `can.View`, usando `access.App{ID: a.ID, AreaID: a.AreaID, OwnerID: a.OwnerID, BackupID: a.BackupID.Int64, Restricted: a.Restricted}`.
- `appDetail` ganha `AreaID int64 \`json:"area_id"\``, `Restricted bool \`json:"restricted"\``, `Can access.Can \`json:"can"\``; `toAppDetail` preenche `AreaID` e `Restricted`.
- `getApp`: no começo, `a, me, _, can, ok := h.appFor(w, r)`; o resto usa `a` (apagar a leitura própria do id e o `GetApp`). `d.Can = can`.
- `checkApp`: `a, _, _, _, ok := h.appFor(w, r)`.
- `scan`: `me, ok := h.currentPerson(...)`, `acc := h.loadAccess(...)`, `if !acc.CanRegister() { writeError(w, 403, "Você não tem permissão para registrar apps.") }`.
- `saveApp`: igual ao `scan` para app novo; quando `GetAppByFolder` acha o app, exigir `acc.For(appRef(old)).Edit`, senão 403 "Você não pode mudar este app." Mover a busca `GetAppByFolder` para antes de escrever na pasta (hoje ela vem depois de `WriteToFolder`).

`history.go`, `appHistory`: `a, _, _, can, ok := h.appFor(w, r)` no começo, antes de ler os parâmetros. `if !can.Tech { writeError(w, http.StatusForbidden, "Você não pode ver o histórico deste app."); return }`. `SeeAll: can.Runs`.

`versions.go`: `versionFromPath` passa a usar `appFor` e, para `getVersion`, `versionChanges`, `versionFiles`, `versionContent`, exige `can.Tech` (403 "Você não pode ver o código deste app."). Mudar a assinatura para `(sqlc.App, access.Can, sqlc.GetAppVersionRow, versions.Snapshot, bool)` e ajustar os quatro chamadores e `describeVersion`. `describeVersion` exige `can.Edit` (403 "Você não pode mudar este app.").

`restore.go`: `restorePreview` e `restoreVersion` usam `appFor` e exigem `can.Restore` (403 "Você não pode voltar versões deste app."). `me` vem do `appFor`.

`runs.go`:
- `startRun`: trocar a busca do app por `a, me, _, _, ok := h.appFor(w, r)`.
- `runForPerson`: a execução é visível se `row.PersonID == me.ID`, ou se a pessoa pode ver as execuções daquele app:

```go
		if err == nil && row.PersonID != me.ID {
			acc, aerr := h.loadAccess(r.Context(), me)
			app, gerr := h.q.GetApp(r.Context(), row.AppID)
			if aerr == nil && gerr == nil {
				if c := acc.For(appRef(app)); c.View && c.Runs {
					return row, me, true
				}
			}
		} else if err == nil {
			return row, me, true
		}
```

- `listAppRuns`: `a, me, _, can, ok := h.appFor(w, r)`; `SeeAll: can.Runs`, `AppID: a.ID`.
- `listRuns`: com `scope=all`, carregar `acc`; se `acc.Has(access.SeeAllRuns)` → `SeeAll: true`; senão juntar os ids dos apps de `ListAppLinks` em que `acc.For(...)` dá `View && Runs`; lista vazia → 403 "Você não pode ver as execuções dos outros." Com `scope=mine`, `AppIds: []int64{}`.

`agents.go`, `revokeComputer`: `me.Role != "admin"` vira `!acc.Has(access.ManagePeople)` (carregar `acc` só quando o computador é de outra pessoa).

`handler.go`, `createPerson`: `if _, _, ok := h.requirePerm(w, r, access.ManagePeople); !ok { return }`. Apagar `requireRole`. Em `access.go`, apagar `roleKey`.

Run: `cd backend && mise x -- go build ./... && grep -rn "roleKey\|requireRole\|\.Role\b" internal/handler/*.go`
Expected: build ok; grep não acha nada em código que não seja de teste.

- [ ] **Step 5: Rodar todos os testes**

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./...`
Expected: PASS. Testes antigos que testavam "criador mexe em qualquer app" (por exemplo `TestRestoreRequiresRole`) mudam de sentido: um criador que não é responsável recebe 403 e um usuário também. Ajustar a expectativa e dizer no commit.

- [ ] **Step 6: Commit**

```bash
git add backend
git commit -m "Access: catalog, ficha, versions, restore and runs obey roles, leaders and grants"
```

---
### Task 4: Editar a ficha por partes (`PUT /api/apps/{id}`), outros apps e a regra das opções

**Files:**
- Modify: `backend/internal/manifest/manifest.go`, `backend/internal/manifest/manifest_test.go`
- Modify: `backend/internal/handler/apps.go`, `backend/internal/handler/handler.go` (rota)
- Test: `backend/internal/handler/update_app_test.go`

**Interfaces:**
- Consumes: `appFor`, `loadAccess`, `appRef`, `ListAppLinks`.
- Produces: `func (h *Handler) writeApp(w http.ResponseWriter, r *http.Request, me sqlc.Person, folder string, b appBody, old *sqlc.App) (sqlc.App, bool)` com `type appBody struct{ FolderPath string; OwnerID int64; BackupID *int64; Manifest manifest.Manifest }` (JSON `folder_path, owner_id, backup_id, manifest`).
- Produces: `PUT /api/apps/{id}` com corpo `{owner_id, backup_id, manifest, updated_at}`; respostas 200 `appDetail`, 403, 404, 409 (conflito ou pasta), 400 (validação, com `errors`).
- Produces: `appDetail.DependsOn []appLink` e `appDetail.UsedBy []appLink` (JSON `depends_on_apps`, `used_by`), com `type appLink struct{ ID int64 \`json:"id"\`; Slug string \`json:"slug"\`; Name string \`json:"name"\`; Visible bool \`json:"visible"\` }`. Só preenchidos quando `can.Tech`; `ID` 0 = fora do catálogo.

- [ ] **Step 1: Teste da regra das opções**

Em `manifest_test.go`:

```go
func TestChoiceNeedsOptions(t *testing.T) {
	m := validManifest()
	m.Inputs = []Input{{ID: "docs", Label: "Quais documentos", Type: "choice", Source: "window", Required: true}}
	errs := m.Validate()
	if len(errs) == 0 || errs[0] != `Escreva ao menos uma opção para "Quais documentos".` {
		t.Fatalf("errs = %v", errs)
	}
	m.Inputs[0].Options = []string{"FRE"}
	m.Inputs[0].Multiple = true
	if errs := m.Validate(); len(errs) != 0 {
		t.Fatalf("valid choice: %v", errs)
	}
}
```

(Usar o construtor de manifesto válido que já existe em `manifest_test.go`; se o nome for outro, ajuste.)

Run: `cd backend && mise x -- go test ./internal/manifest/ -run Choice` → FAIL.

- [ ] **Step 2: Implementar a regra**

Em `Validate`, dentro do laço das entradas, depois da checagem de `Multiple`:

```go
		if in.Type == "choice" && len(in.Options) == 0 {
			errs = append(errs, fmt.Sprintf("Escreva ao menos uma opção para %q.", in.Label))
		}
```

Run: `cd backend && mise x -- go test ./internal/manifest/` → PASS. Se o scanner gera `choice` sem opções em algum teste de `scanner`, rodar `go test ./...` e ajustar só a expectativa daquele teste: o formulário de revisão vai pedir as opções.

- [ ] **Step 3: Testes do editar**

`backend/internal/handler/update_app_test.go`:

```go
package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func detailOf(t *testing.T, c *client, id int64) appDetail {
	t.Helper()
	var d appDetail
	if code := c.do("GET", appPath(id, ""), nil, &d); code != http.StatusOK {
		t.Fatalf("get app %d", code)
	}
	return d
}

func putApp(c *client, id int64, d appDetail) (int, map[string]any) {
	var out map[string]any
	code := c.do("PUT", appPath(id, ""), map[string]any{
		"owner_id": d.OwnerID, "backup_id": d.BackupID, "manifest": d.Manifest, "updated_at": d.UpdatedAt,
	}, &out)
	return code, out
}

func TestUpdateAppOnePart(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	d := detailOf(t, c, id)
	d.Manifest.Summary = "Resumo novo"
	d.Manifest.Notes = []string{"Precisa da VPN."}
	if code, out := putApp(c, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "app-hub.yaml"))
	if !strings.Contains(string(yaml), "Resumo novo") || !strings.Contains(string(yaml), "Precisa da VPN.") {
		t.Fatalf("yaml not rewritten:\n%s", yaml)
	}
	var h struct {
		Items []historyItem `json:"items"`
	}
	c.do("GET", appPath(id, "/history?type=ficha"), nil, &h)
	if len(h.Items) < 2 {
		t.Fatalf("history should record the change: %+v", h.Items)
	}
}

func TestUpdateAppConflict(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, dir := codeApp(t, c)
	stale := detailOf(t, c, id)
	fresh := stale
	fresh.Manifest.Summary = "Primeiro"
	if code, _ := putApp(c, id, fresh); code != http.StatusOK {
		t.Fatal("first save")
	}
	stale.Manifest.Summary = "Segundo"
	code, out := putApp(c, id, stale)
	if code != http.StatusConflict || out["error"] != "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova." {
		t.Fatalf("conflict %d %v", code, out)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "app-hub.yaml"))
	if strings.Contains(string(yaml), "Segundo") {
		t.Fatal("stale save reached the folder")
	}
}

func TestUpdateAppPermissionAndFolder(t *testing.T) {
	admin, ana, bia, _ := team(t)
	id, dir := codeApp(t, admin)
	d := detailOf(t, admin, id)
	if code, _ := putApp(ana, id, d); code != http.StatusForbidden {
		t.Fatalf("criador on someone else's app = %d", code)
	}
	restrict(t, id)
	if code, _ := putApp(bia, id, d); code != http.StatusNotFound {
		t.Fatalf("restricted = %d", code)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	code, out := putApp(admin, id, d)
	if code != http.StatusConflict || !strings.HasPrefix(out["error"].(string), "Não consegui ler a pasta do app agora.") {
		t.Fatalf("missing folder %d %v", code, out)
	}
}

func TestUpdateAppValidation(t *testing.T) {
	c, _ := newPoolClient(t)
	setupAdmin(t, c)
	id, _ := codeApp(t, c)
	d := detailOf(t, c, id)
	d.Manifest.DependsOn = []string{d.Manifest.ID}
	code, out := putApp(c, id, d)
	if code != http.StatusBadRequest || out["error"] != "O app não pode depender dele mesmo." {
		t.Fatalf("self dependency %d %v", code, out)
	}
}

func TestDependsLinks(t *testing.T) {
	admin, ana, _, fp := team(t) // fp: slug eq-research-files-prep
	id, _ := codeApp(t, admin)
	d := detailOf(t, admin, id)
	d.Manifest.DependsOn = []string{"eq-research-files-prep", "nao-existe"}
	if code, out := putApp(admin, id, d); code != http.StatusOK {
		t.Fatalf("put %d %v", code, out)
	}
	d = detailOf(t, admin, id)
	if len(d.DependsOn) != 2 || d.DependsOn[0].Name == "" || d.DependsOn[1].ID != 0 {
		t.Fatalf("depends = %+v", d.DependsOn)
	}
	var other appDetail
	admin.do("GET", appPath(fp, ""), nil, &other)
	if len(other.UsedBy) != 1 || other.UsedBy[0].ID != id {
		t.Fatalf("used_by = %+v", other.UsedBy)
	}
	if d := detailOf(t, ana, id); d.DependsOn != nil {
		t.Fatal("someone without tech must not get the links")
	}
}
```

(`ana` é Criador e não é responsável pelo app: não vê a parte técnica, então não recebe os links.)

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/handler/ -run 'UpdateApp|DependsLinks'` → FAIL (rota não existe).

- [ ] **Step 4: Extrair `writeApp` de `saveApp`**

Em `apps.go`, mover o miolo de `saveApp` (do `m := body.Manifest` até o `writeJSON` final) para:

```go
type appBody struct {
	FolderPath string            `json:"folder_path"`
	OwnerID    int64             `json:"owner_id"`
	BackupID   *int64            `json:"backup_id"`
	Manifest   manifest.Manifest `json:"manifest"`
}

// writeApp is the one way an app reaches the catalog: it validates, writes
// app-hub.yaml and FICHA.md into the folder (never touching code), records the
// app and what changed in the ficha. old is the app being updated, or nil.
func (h *Handler) writeApp(w http.ResponseWriter, r *http.Request, me sqlc.Person, folder string, b appBody, old *sqlc.App) (sqlc.App, bool) {
	// …the current body of saveApp, unchanged, except:
	//  - the area is resolved with GetAreaByName (Task 1) and passed as AreaID;
	//  - when old != nil, m.ID = old.Slug (an edit never renames the slug) and the
	//    old manifest for Diff comes from old.Manifest instead of GetAppByFolder;
	//  - errors are written here and the function returns (sqlc.App{}, false).
}
```

`saveApp` fica: autenticação e regras do Task 3, `os.Stat` da pasta, `GetAppByFolder` (para saber se é novo), `a, ok := h.writeApp(w, r, me, folder, body, oldPtr)`, e depois `toAppDetail` + `writeJSON(201)` como hoje.

- [ ] **Step 5: `updateApp` e a rota**

```go
// updateApp saves one part of the ficha (the frontend sends the whole manifest
// with only that part changed). updated_at guards against a second, stale save.
func (h *Handler) updateApp(w http.ResponseWriter, r *http.Request) {
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Edit {
		writeError(w, http.StatusForbidden, "Você não pode mudar este app.")
		return
	}
	var body struct {
		appBody
		UpdatedAt time.Time `json:"updated_at"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !body.UpdatedAt.Equal(a.UpdatedAt.Time) {
		writeError(w, http.StatusConflict, "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova.")
		return
	}
	if info, err := os.Stat(a.FolderPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusConflict, "Não consegui ler a pasta do app agora. Nada foi salvo. Confira se o OneDrive está sincronizado e tente de novo.")
		return
	}
	saved, ok := h.writeApp(w, r, me, a.FolderPath, body.appBody, &a)
	if !ok {
		return
	}
	h.writeDetail(w, r, me, saved, http.StatusOK)
}
```

`handler.go`: `mux.HandleFunc("PUT /api/apps/{id}", h.updateApp)`.

`writeDetail` junta o que `getApp` faz hoje depois de achar o app (versão, pasta lembrada, `can`, links), para `getApp` e `updateApp` usarem igual:

```go
func (h *Handler) writeDetail(w http.ResponseWriter, r *http.Request, me sqlc.Person, a sqlc.App, status int) {
	d, err := toAppDetail(a)
	// …FolderReadable, Version, RememberedFolder as getApp does today…
	acc, err := h.loadAccess(r.Context(), me)
	// …
	d.Can = acc.For(appRef(a))
	if d.Can.Tech {
		if d.DependsOn, d.UsedBy, err = h.appLinks(r.Context(), acc, a, d.Manifest); err != nil {
			internalError(w, "app links", err)
			return
		}
	}
	writeJSON(w, status, d)
}

// appLinks resolves depends_on ids to catalog apps and finds who depends on a.
// Apps me cannot see keep their name but lose the link (Visible false).
func (h *Handler) appLinks(ctx context.Context, acc *access.Person, a sqlc.App, m manifest.Manifest) ([]appLink, []appLink, error) {
	rows, err := h.q.ListAppLinks(ctx)
	if err != nil {
		return nil, nil, err
	}
	bySlug := map[string]sqlc.ListAppLinksRow{}
	for _, x := range rows {
		bySlug[x.Slug] = x
	}
	visible := func(x sqlc.ListAppLinksRow) bool {
		return acc.For(access.App{ID: x.ID, AreaID: x.AreaID, OwnerID: x.OwnerID, BackupID: x.BackupID.Int64, Restricted: x.Restricted}).View
	}
	deps := []appLink{}
	for _, slug := range m.DependsOn {
		if x, ok := bySlug[slug]; ok {
			deps = append(deps, appLink{ID: x.ID, Slug: x.Slug, Name: x.Name, Visible: visible(x)})
		} else {
			deps = append(deps, appLink{Slug: slug, Name: slug})
		}
	}
	usedBy := []appLink{}
	for _, x := range rows {
		var ids []string
		_ = json.Unmarshal(x.DependsOn, &ids)
		if x.ID != a.ID && slices.Contains(ids, a.Slug) {
			usedBy = append(usedBy, appLink{ID: x.ID, Slug: x.Slug, Name: x.Name, Visible: visible(x)})
		}
	}
	return deps, usedBy, nil
}
```

`appDetail` ganha:

```go
	DependsOn []appLink `json:"depends_on_apps,omitempty"`
	UsedBy    []appLink `json:"used_by,omitempty"`
```

e `getApp` vira `a, me, _, _, ok := h.appFor(w, r)` + `h.writeDetail(w, r, me, a, http.StatusOK)`.

- [ ] **Step 6: Rodar os testes**

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend
git commit -m "Ficha: PUT /api/apps/{id} saves one part, guards stale saves, links other apps"
```

---

### Task 5: Equipe no backend: cargos, áreas e pessoas

**Files:**
- Create: `backend/internal/handler/team.go`
- Modify: `backend/internal/handler/handler.go` (rotas)
- Test: `backend/internal/handler/team_test.go`

**Interfaces:**
- Consumes: `requirePerm`, `loadAccess`, `access.RolePerms`, consultas do Task 1.
- Produces (rotas e formatos):
  - `GET /api/roles` (quem tem `manage_roles` ou `manage_people`) → `[{id, key, name, permissions, people_count}]` (`key` é `""` nos criados).
  - `POST /api/roles` `{name, permissions}` → 201; `PUT /api/roles/{id}` `{name, permissions}` → 200; `DELETE /api/roles/{id}` → 204 (`manage_roles`).
  - `PUT /api/people/{id}` `{role_id}` → 200 `personRow` (`manage_people`).
  - `GET /api/areas` (qualquer pessoa logada) → `[{id, name, app_count, leaders:[{id,name}], members:[{id,name}]}]`.
  - `POST /api/areas` `{name}`; `PUT /api/areas/{id}` `{name}`; `DELETE /api/areas/{id}`; `PUT /api/areas/{id}/leaders` `{person_ids}` (`manage_areas`).
  - `PUT /api/areas/{id}/members` `{person_ids}` (`manage_areas` ou líder daquela área).
- Mensagens fixas: "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos." (409) · "Esse cargo ainda tem pessoas. Troque o cargo delas antes de apagar." (409) · "Os cargos prontos não podem ser apagados." (409) · "Essa área ainda tem apps. Mude a área deles antes de apagar." (409) · "Já existe um cargo com esse nome." / "Já existe uma área com esse nome." (409) · "Permissão desconhecida." (400) · "Dê um nome." (400).

- [ ] **Step 1: Escrever os testes**

`backend/internal/handler/team_test.go`:

```go
package handler

import (
	"net/http"
	"testing"
)

type roleOut struct {
	ID          int64    `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	PeopleCount int64    `json:"people_count"`
}

type areaOut struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	AppCount int64  `json:"app_count"`
	Leaders  []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"leaders"`
	Members []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"members"`
}

func TestRolesCRUD(t *testing.T) {
	admin, ana, _, _ := team(t)
	var r roleOut
	if code := admin.do("POST", "/api/roles", map[string]any{"name": "Sub-admin", "permissions": []string{"see_all_runs", "edit_any_app"}}, &r); code != http.StatusCreated {
		t.Fatalf("create %d", code)
	}
	if code := admin.do("POST", "/api/roles", map[string]any{"name": "X", "permissions": []string{"voar"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown permission = %d", code)
	}
	if code := ana.do("POST", "/api/roles", map[string]any{"name": "Y"}, nil); code != http.StatusForbidden {
		t.Fatalf("criador creating role = %d", code)
	}
	if code := admin.do("PUT", "/api/people/2", map[string]any{"role_id": r.ID}, nil); code != http.StatusOK {
		t.Fatalf("give role %d", code)
	}
	if code := admin.do("DELETE", "/api/roles/"+itoa(r.ID), nil, nil); code != http.StatusConflict {
		t.Fatalf("delete role in use = %d", code)
	}
	if code := admin.do("DELETE", "/api/roles/2", nil, nil); code != http.StatusConflict {
		t.Fatalf("delete built-in = %d", code)
	}
	var st struct {
		Person personJSON `json:"person"`
	}
	ana.do("GET", "/api/auth/status", nil, &st)
	if st.Person.RoleName != "Sub-admin" || !st.Person.SeesOthersRuns {
		t.Fatalf("ana after new role: %+v", st.Person)
	}
}

func TestLastManagerProtected(t *testing.T) {
	admin, _, _, _ := team(t)
	const msg = "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos."
	var out map[string]any
	if code := admin.do("PUT", "/api/people/1", map[string]any{"role_id": 3}, &out); code != http.StatusConflict || out["error"] != msg {
		t.Fatalf("demote last admin %d %v", code, out)
	}
	if code := admin.do("PUT", "/api/roles/1", map[string]any{"name": "Administrador", "permissions": []string{"register_apps"}}, &out); code != http.StatusConflict || out["error"] != msg {
		t.Fatalf("strip admin role %d %v", code, out)
	}
	createPersonAs(t, admin, "Caio", "caio@alis.com", "admin")
	if code := admin.do("PUT", "/api/people/1", map[string]any{"role_id": 3}, nil); code != http.StatusOK {
		t.Fatalf("demote with another admin = %d", code)
	}
}

func TestAreasAndLeaders(t *testing.T) {
	admin, ana, bia, _ := team(t)
	var a areaOut
	if code := admin.do("POST", "/api/areas", map[string]string{"name": "Operações"}, &a); code != http.StatusCreated {
		t.Fatalf("create area %d", code)
	}
	if code := admin.do("POST", "/api/areas", map[string]string{"name": "Operações"}, nil); code != http.StatusConflict {
		t.Fatalf("duplicate = %d", code)
	}
	if code := admin.do("PUT", "/api/areas/"+itoa(a.ID)+"/leaders", map[string]any{"person_ids": []int64{2}}, nil); code != http.StatusOK {
		t.Fatalf("leaders %d", code)
	}
	if code := ana.do("PUT", "/api/areas/"+itoa(a.ID)+"/members", map[string]any{"person_ids": []int64{3}}, nil); code != http.StatusOK {
		t.Fatalf("leader sets members %d", code)
	}
	if code := ana.do("PUT", "/api/areas/1/members", map[string]any{"person_ids": []int64{3}}, nil); code != http.StatusForbidden {
		t.Fatalf("leader on another area = %d", code)
	}
	if code := ana.do("PUT", "/api/areas/"+itoa(a.ID)+"/leaders", map[string]any{"person_ids": []int64{3}}, nil); code != http.StatusForbidden {
		t.Fatalf("leader choosing leaders = %d", code)
	}
	var areas []areaOut
	bia.do("GET", "/api/areas", nil, &areas)
	var ops areaOut
	for _, x := range areas {
		if x.ID == a.ID {
			ops = x
		}
	}
	if len(ops.Leaders) != 1 || len(ops.Members) != 1 {
		t.Fatalf("area = %+v", ops)
	}
	if code := admin.do("DELETE", "/api/areas/1", nil, nil); code != http.StatusConflict {
		t.Fatalf("delete area with apps = %d", code)
	}
	if code := admin.do("PUT", "/api/areas/1", map[string]string{"name": "Pesquisa"}, nil); code != http.StatusOK {
		t.Fatalf("rename %d", code)
	}
	var list []appSummary
	admin.do("GET", "/api/apps", nil, &list)
	if list[0].Area != "Pesquisa" {
		t.Fatalf("rename must reach the apps: %+v", list[0])
	}
}
```

Acrescentar ao arquivo:

```go
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
```

(import `strconv`).

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/handler/ -run 'Roles|LastManager|AreasAnd'` → FAIL.

- [ ] **Step 2: Escrever `team.go`**

```go
package handler

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

const lastManager = "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos."

func isManager(perms []string) bool {
	return slices.Contains(perms, access.ManagePeople) && slices.Contains(perms, access.ManageRoles)
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

type roleJSON struct {
	ID          int64    `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	PeopleCount int64    `json:"people_count"`
}

type roleBody struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

func (b *roleBody) check() string {
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		return "Dê um nome."
	}
	if b.Permissions == nil {
		b.Permissions = []string{}
	}
	for _, p := range b.Permissions {
		if !slices.Contains(access.RolePerms, p) {
			return "Permissão desconhecida."
		}
	}
	return ""
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	if !acc.Has(access.ManageRoles) && !acc.Has(access.ManagePeople) {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return
	}
	rows, err := h.q.ListRoles(r.Context())
	if err != nil {
		internalError(w, "list roles", err)
		return
	}
	out := make([]roleJSON, 0, len(rows))
	for _, x := range rows {
		out = append(out, roleJSON{ID: x.ID, Key: x.Key.String, Name: x.Name, Permissions: orEmpty(x.Permissions), PeopleCount: x.PeopleCount})
	}
	writeJSON(w, http.StatusOK, out)
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	var b roleBody
	if !readJSON(w, r, &b) {
		return
	}
	if msg := b.check(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	x, err := h.q.CreateRole(r.Context(), sqlc.CreateRoleParams{Name: b.Name, Permissions: b.Permissions})
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe um cargo com esse nome.")
		return
	}
	if err != nil {
		internalError(w, "create role", err)
		return
	}
	writeJSON(w, http.StatusCreated, roleJSON{ID: x.ID, Key: x.Key.String, Name: x.Name, Permissions: orEmpty(x.Permissions)})
}

func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "Cargo não encontrado.")
		return
	}
	var b roleBody
	if !readJSON(w, r, &b) {
		return
	}
	if msg := b.check(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	cur, err := h.q.GetRole(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cargo não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "get role", err)
		return
	}
	if isManager(cur.Permissions) && !isManager(b.Permissions) {
		n, err := h.q.CountManagersExceptRole(r.Context(), id)
		if err != nil {
			internalError(w, "count managers", err)
			return
		}
		if n == 0 {
			writeError(w, http.StatusConflict, lastManager)
			return
		}
	}
	x, err := h.q.UpdateRole(r.Context(), sqlc.UpdateRoleParams{ID: id, Name: b.Name, Permissions: b.Permissions})
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe um cargo com esse nome.")
		return
	}
	if err != nil {
		internalError(w, "update role", err)
		return
	}
	writeJSON(w, http.StatusOK, roleJSON{ID: x.ID, Key: x.Key.String, Name: x.Name, Permissions: orEmpty(x.Permissions)})
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageRoles); !ok {
		return
	}
	id, _ := pathID(r)
	rows, err := h.q.ListRoles(r.Context())
	if err != nil {
		internalError(w, "list roles", err)
		return
	}
	for _, x := range rows {
		if x.ID != id {
			continue
		}
		switch {
		case x.Key.Valid:
			writeError(w, http.StatusConflict, "Os cargos prontos não podem ser apagados.")
		case x.PeopleCount > 0:
			writeError(w, http.StatusConflict, "Esse cargo ainda tem pessoas. Troque o cargo delas antes de apagar.")
		default:
			if err := h.q.DeleteRole(r.Context(), id); err != nil {
				internalError(w, "delete role", err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}
	writeError(w, http.StatusNotFound, "Cargo não encontrado.")
}

func (h *Handler) setPersonRole(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManagePeople); !ok {
		return
	}
	id, _ := pathID(r)
	var b struct {
		RoleID int64 `json:"role_id"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	p, err := h.q.GetPerson(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Pessoa não encontrada.")
		return
	}
	if err != nil {
		internalError(w, "get person", err)
		return
	}
	next, err := h.q.GetRole(r.Context(), b.RoleID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Escolha o cargo da pessoa.")
		return
	}
	cur, err := h.q.GetRole(r.Context(), p.RoleID)
	if err != nil {
		internalError(w, "get role", err)
		return
	}
	if isManager(cur.Permissions) && !isManager(next.Permissions) {
		n, err := h.q.CountManagersExceptPerson(r.Context(), id)
		if err != nil {
			internalError(w, "count managers", err)
			return
		}
		if n == 0 {
			writeError(w, http.StatusConflict, lastManager)
			return
		}
	}
	if err := h.q.SetPersonRole(r.Context(), sqlc.SetPersonRoleParams{ID: id, RoleID: next.ID}); err != nil {
		internalError(w, "set role", err)
		return
	}
	writeJSON(w, http.StatusOK, personRow{ID: p.ID, Name: p.Name, Email: p.Email, RoleID: next.ID, RoleName: next.Name, AreaIDs: []int64{}})
}

type nameRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type areaJSON struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	AppCount int64     `json:"app_count"`
	Leaders  []nameRef `json:"leaders"`
	Members  []nameRef `json:"members"`
}

func (h *Handler) listAreas(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentPerson(w, r); !ok {
		return
	}
	areas, err := h.q.ListAreas(r.Context())
	if err != nil {
		internalError(w, "list areas", err)
		return
	}
	leaders, err := h.q.ListAreaLeaders(r.Context())
	if err != nil {
		internalError(w, "area leaders", err)
		return
	}
	members, err := h.q.ListAreaMembers(r.Context())
	if err != nil {
		internalError(w, "area members", err)
		return
	}
	out := make([]areaJSON, 0, len(areas))
	for _, a := range areas {
		x := areaJSON{ID: a.ID, Name: a.Name, AppCount: a.AppCount, Leaders: []nameRef{}, Members: []nameRef{}}
		for _, l := range leaders {
			if l.AreaID == a.ID {
				x.Leaders = append(x.Leaders, nameRef{ID: l.ID, Name: l.Name})
			}
		}
		for _, m := range members {
			if m.AreaID == a.ID {
				x.Members = append(x.Members, nameRef{ID: m.ID, Name: m.Name})
			}
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createArea(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	var b struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if b.Name = strings.TrimSpace(b.Name); b.Name == "" {
		writeError(w, http.StatusBadRequest, "Dê um nome.")
		return
	}
	a, err := h.q.CreateArea(r.Context(), b.Name)
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe uma área com esse nome.")
		return
	}
	if err != nil {
		internalError(w, "create area", err)
		return
	}
	writeJSON(w, http.StatusCreated, areaJSON{ID: a.ID, Name: a.Name, Leaders: []nameRef{}, Members: []nameRef{}})
}

// renameArea also renames it in every app of the area; each folder's
// app-hub.yaml catches up the next time that ficha is saved.
func (h *Handler) renameArea(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	id, _ := pathID(r)
	var b struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if b.Name = strings.TrimSpace(b.Name); b.Name == "" {
		writeError(w, http.StatusBadRequest, "Dê um nome.")
		return
	}
	a, err := h.q.RenameArea(r.Context(), sqlc.RenameAreaParams{ID: id, Name: b.Name})
	if isUnique(err) {
		writeError(w, http.StatusConflict, "Já existe uma área com esse nome.")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Área não encontrada.")
		return
	}
	if err != nil {
		internalError(w, "rename area", err)
		return
	}
	if err := h.q.RenameAppsArea(r.Context(), sqlc.RenameAppsAreaParams{AreaID: a.ID, Name: a.Name}); err != nil {
		internalError(w, "rename apps area", err)
		return
	}
	writeJSON(w, http.StatusOK, areaJSON{ID: a.ID, Name: a.Name, Leaders: []nameRef{}, Members: []nameRef{}})
}

func (h *Handler) deleteArea(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	id, _ := pathID(r)
	areas, err := h.q.ListAreas(r.Context())
	if err != nil {
		internalError(w, "list areas", err)
		return
	}
	for _, a := range areas {
		if a.ID != id {
			continue
		}
		if a.AppCount > 0 {
			writeError(w, http.StatusConflict, "Essa área ainda tem apps. Mude a área deles antes de apagar.")
			return
		}
		if err := h.q.DeleteArea(r.Context(), id); err != nil {
			internalError(w, "delete area", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "Área não encontrada.")
}

func (h *Handler) setAreaLeaders(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageAreas); !ok {
		return
	}
	h.replaceAreaPeople(w, r, h.q.ClearAreaLeaders, func(area, person int64) error {
		return h.q.AddAreaLeader(r.Context(), sqlc.AddAreaLeaderParams{AreaID: area, PersonID: person})
	})
}

func (h *Handler) setAreaMembers(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentPerson(w, r)
	if !ok {
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "load access", err)
		return
	}
	id, _ := pathID(r)
	if !acc.Has(access.ManageAreas) && !acc.Leads[id] {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return
	}
	h.replaceAreaPeople(w, r, h.q.ClearAreaMembers, func(area, person int64) error {
		return h.q.AddAreaMember(r.Context(), sqlc.AddAreaMemberParams{AreaID: area, PersonID: person})
	})
}

func (h *Handler) replaceAreaPeople(w http.ResponseWriter, r *http.Request, clear func(context.Context, int64) error, add func(area, person int64) error) {
	id, _ := pathID(r)
	if _, err := h.q.GetArea(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "Área não encontrada.")
		return
	}
	var b struct {
		PersonIDs []int64 `json:"person_ids"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	if err := clear(r.Context(), id); err != nil {
		internalError(w, "clear area people", err)
		return
	}
	for _, p := range b.PersonIDs {
		if err := add(id, p); err != nil {
			internalError(w, "add area person", err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
```

(import `context`. Os nomes dos parâmetros gerados pelo sqlc — `CreateRoleParams`, `RenameAppsAreaParams` etc. — devem ser conferidos no código gerado.)

Rotas em `handler.go`:

```go
	mux.HandleFunc("PUT /api/people/{id}", h.setPersonRole)
	mux.HandleFunc("GET /api/roles", h.listRoles)
	mux.HandleFunc("POST /api/roles", h.createRole)
	mux.HandleFunc("PUT /api/roles/{id}", h.updateRole)
	mux.HandleFunc("DELETE /api/roles/{id}", h.deleteRole)
	mux.HandleFunc("GET /api/areas", h.listAreas)
	mux.HandleFunc("POST /api/areas", h.createArea)
	mux.HandleFunc("PUT /api/areas/{id}", h.renameArea)
	mux.HandleFunc("DELETE /api/areas/{id}", h.deleteArea)
	mux.HandleFunc("PUT /api/areas/{id}/leaders", h.setAreaLeaders)
	mux.HandleFunc("PUT /api/areas/{id}/members", h.setAreaMembers)
```

- [ ] **Step 3: Rodar os testes**

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend
git commit -m "Equipe API: roles with permissions, areas with leaders and members, last-manager guard"
```

---

### Task 6: Acesso de cada app no backend (aberto/restrito, quem vê, acessos extras)

**Files:**
- Create: `backend/internal/handler/app_access.go`
- Modify: `backend/internal/handler/handler.go` (rotas)
- Test: `backend/internal/handler/app_access_test.go`

**Interfaces:**
- Consumes: `appFor`, `loadAccess`, `access.Grants`, consultas do Task 1.
- Produces:
  - `GET /api/apps/{id}/access` (precisa `can.Tech`) → `appAccessJSON{Restricted bool; Areas []nameRef; People []nameRef; Grants []grantJSON; AreaGrants []grantJSON}`; JSON `restricted, areas, people, grants, area_grants`; `grantJSON{PersonID int64 \`json:"person_id"\`; Name string \`json:"name"\`; Permissions []string \`json:"permissions"\`}`.
  - `PUT /api/apps/{id}/access` `{restricted, area_ids, person_ids}` (precisa `can.Access`) → 200 `appAccessJSON`.
  - `PUT /api/apps/{id}/grants` `{person_id, permissions, whole_area}` (precisa `can.Grants`) → 200 `appAccessJSON`. Com `whole_area: true`, grava em `area_grants` da área do app; senão em `app_grants`. `permissions: []` tira tudo.

- [ ] **Step 1: Escrever os testes**

`backend/internal/handler/app_access_test.go`:

```go
package handler

import (
	"net/http"
	"testing"
)

type appAccessOut struct {
	Restricted bool      `json:"restricted"`
	Areas      []nameRef `json:"areas"`
	People     []nameRef `json:"people"`
	Grants     []struct {
		PersonID    int64    `json:"person_id"`
		Permissions []string `json:"permissions"`
	} `json:"grants"`
	AreaGrants []struct {
		PersonID    int64    `json:"person_id"`
		Permissions []string `json:"permissions"`
	} `json:"area_grants"`
}

func TestRestrictToAreaAndPeople(t *testing.T) {
	admin, _, bia, id := team(t)
	exec(t, "INSERT INTO area_members (area_id, person_id) VALUES (2, 3)")
	var out appAccessOut
	if code := admin.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{2}, "person_ids": []int64{}}, &out); code != http.StatusOK || !out.Restricted || len(out.Areas) != 1 {
		t.Fatalf("restrict %d %+v", code, out)
	}
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusOK {
		t.Fatalf("member of a listed area = %d", code)
	}
	admin.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{1}, "person_ids": []int64{}}, nil)
	if code := bia.do("GET", appPath(id, ""), nil, nil); code != http.StatusNotFound {
		t.Fatalf("after leaving the list = %d", code)
	}
}

func TestOwnerRestrictsButCannotGrant(t *testing.T) {
	_, ana, _, id := team(t)
	ownerOf(t, id, 2)
	if code := ana.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true}, nil); code != http.StatusOK {
		t.Fatalf("owner restricts = %d", code)
	}
	if code := ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"edit"}}, nil); code != http.StatusForbidden {
		t.Fatalf("owner grants = %d", code)
	}
}

func TestLeaderGrantsPerAppAndWholeArea(t *testing.T) {
	_, ana, bia, id := team(t)
	makeLeader(t, 1, 2)
	var out appAccessOut
	if code := ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"edit", "voar"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown grant = %d", code)
	}
	if code := ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"edit"}}, &out); code != http.StatusOK || len(out.Grants) != 1 {
		t.Fatalf("grant %d %+v", code, out)
	}
	var d appDetail
	bia.do("GET", appPath(id, ""), nil, &d)
	if !d.Can.Edit {
		t.Fatal("grant not applied")
	}
	ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{}}, nil)
	ana.do("PUT", appPath(id, "/grants"), map[string]any{"person_id": 3, "permissions": []string{"runs"}, "whole_area": true}, &out)
	if len(out.AreaGrants) != 1 || len(out.Grants) != 0 {
		t.Fatalf("area grant %+v", out)
	}
	later := insertApp(t, filesPrepManifestWithID("outro"))
	bia.do("GET", appPath(later, ""), nil, &d)
	if !d.Can.Runs || d.Can.Edit {
		t.Fatalf("area grant on a later app: %+v", d.Can)
	}
}

func TestAccessNeedsTech(t *testing.T) {
	_, _, bia, id := team(t)
	if code := bia.do("GET", appPath(id, "/access"), nil, nil); code != http.StatusForbidden {
		t.Fatalf("colleague reads access = %d", code)
	}
}
```

`filesPrepManifestWithID(id string)` é um ajudante novo em `runs_test.go`: chama `filesPrepManifest()` e troca `ID` e `Name` por `id`. (`insertApp` já usa uma pasta por slug desde o Task 1.)

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/handler/ -run 'RestrictTo|OwnerRestricts|LeaderGrants|AccessNeeds'` → FAIL.

- [ ] **Step 2: Escrever `app_access.go`**

```go
package handler

import (
	"context"
	"net/http"
	"slices"

	"github.com/HenryKropp/app-hub/backend/internal/access"
	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

type grantJSON struct {
	PersonID    int64    `json:"person_id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type appAccessJSON struct {
	Restricted bool        `json:"restricted"`
	Areas      []nameRef   `json:"areas"`
	People     []nameRef   `json:"people"`
	Grants     []grantJSON `json:"grants"`
	AreaGrants []grantJSON `json:"area_grants"`
}
```

O agrupamento por pessoa é escrito duas vezes (os tipos gerados pelo sqlc são diferentes):
```go
func grantsFromApp(rows []sqlc.ListAppGrantsRow) []grantJSON {
	out := []grantJSON{}
	for _, x := range rows {
		if n := len(out); n > 0 && out[n-1].PersonID == x.PersonID {
			out[n-1].Permissions = append(out[n-1].Permissions, x.Permission)
			continue
		}
		out = append(out, grantJSON{PersonID: x.PersonID, Name: x.PersonName, Permissions: []string{x.Permission}})
	}
	return out
}

func grantsFromArea(rows []sqlc.ListAreaGrantsRow) []grantJSON {
	out := []grantJSON{}
	for _, x := range rows {
		if n := len(out); n > 0 && out[n-1].PersonID == x.PersonID {
			out[n-1].Permissions = append(out[n-1].Permissions, x.Permission)
			continue
		}
		out = append(out, grantJSON{PersonID: x.PersonID, Name: x.PersonName, Permissions: []string{x.Permission}})
	}
	return out
}
```

```go
func (h *Handler) accessOf(ctx context.Context, appID, areaID int64) (appAccessJSON, error) {
	a, err := h.q.GetApp(ctx, appID)
	if err != nil {
		return appAccessJSON{}, err
	}
	out := appAccessJSON{Restricted: a.Restricted, Areas: []nameRef{}, People: []nameRef{}}
	viewers, err := h.q.ListAppViewers(ctx, appID)
	if err != nil {
		return out, err
	}
	for _, v := range viewers {
		if v.AreaID.Valid {
			out.Areas = append(out.Areas, nameRef{ID: v.AreaID.Int64, Name: v.AreaName.String})
		} else {
			out.People = append(out.People, nameRef{ID: v.PersonID.Int64, Name: v.PersonName.String})
		}
	}
	g, err := h.q.ListAppGrants(ctx, appID)
	if err != nil {
		return out, err
	}
	out.Grants = grantsFromApp(g)
	ag, err := h.q.ListAreaGrants(ctx, areaID)
	if err != nil {
		return out, err
	}
	out.AreaGrants = grantsFromArea(ag)
	return out, nil
}

func (h *Handler) getAppAccess(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Tech {
		writeError(w, http.StatusForbidden, "Você não pode ver o acesso deste app.")
		return
	}
	out, err := h.accessOf(r.Context(), a.ID, a.AreaID)
	if err != nil {
		internalError(w, "app access", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setAppAccess(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Access {
		writeError(w, http.StatusForbidden, "Você não pode mudar quem vê este app.")
		return
	}
	var b struct {
		Restricted bool    `json:"restricted"`
		AreaIDs    []int64 `json:"area_ids"`
		PersonIDs  []int64 `json:"person_ids"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	ctx := r.Context()
	if err := h.q.SetAppRestricted(ctx, sqlc.SetAppRestrictedParams{ID: a.ID, Restricted: b.Restricted}); err != nil {
		internalError(w, "set restricted", err)
		return
	}
	if err := h.q.ClearAppViewers(ctx, a.ID); err != nil {
		internalError(w, "clear viewers", err)
		return
	}
	for _, id := range b.AreaIDs {
		if err := h.q.AddAppViewerArea(ctx, sqlc.AddAppViewerAreaParams{AppID: a.ID, AreaID: pgInt8(id)}); err != nil {
			internalError(w, "add viewer area", err)
			return
		}
	}
	for _, id := range b.PersonIDs {
		if err := h.q.AddAppViewerPerson(ctx, sqlc.AddAppViewerPersonParams{AppID: a.ID, PersonID: pgInt8(id)}); err != nil {
			internalError(w, "add viewer person", err)
			return
		}
	}
	out, err := h.accessOf(ctx, a.ID, a.AreaID)
	if err != nil {
		internalError(w, "app access", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setAppGrants(w http.ResponseWriter, r *http.Request) {
	a, _, _, can, ok := h.appFor(w, r)
	if !ok {
		return
	}
	if !can.Grants {
		writeError(w, http.StatusForbidden, "Só o líder da área dá acessos extras.")
		return
	}
	var b struct {
		PersonID    int64    `json:"person_id"`
		Permissions []string `json:"permissions"`
		WholeArea   bool     `json:"whole_area"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	for _, p := range b.Permissions {
		if !slices.Contains(access.Grants, p) {
			writeError(w, http.StatusBadRequest, "Acesso desconhecido.")
			return
		}
	}
	ctx := r.Context()
	if _, err := h.q.GetPerson(ctx, b.PersonID); err != nil {
		writeError(w, http.StatusBadRequest, "Escolha a pessoa.")
		return
	}
	// One place per person: choosing "whole area" moves the accesses there, and back.
	if err := h.q.ClearPersonAppGrants(ctx, sqlc.ClearPersonAppGrantsParams{AppID: a.ID, PersonID: b.PersonID}); err != nil {
		internalError(w, "clear grants", err)
		return
	}
	if err := h.q.ClearPersonAreaGrants(ctx, sqlc.ClearPersonAreaGrantsParams{AreaID: a.AreaID, PersonID: b.PersonID}); err != nil {
		internalError(w, "clear area grants", err)
		return
	}
	for _, p := range b.Permissions {
		var err error
		if b.WholeArea {
			err = h.q.AddAreaGrant(ctx, sqlc.AddAreaGrantParams{AreaID: a.AreaID, PersonID: b.PersonID, Permission: p})
		} else {
			err = h.q.AddAppGrant(ctx, sqlc.AddAppGrantParams{AppID: a.ID, PersonID: b.PersonID, Permission: p})
		}
		if err != nil {
			internalError(w, "add grant", err)
			return
		}
	}
	out, err := h.accessOf(ctx, a.ID, a.AreaID)
	if err != nil {
		internalError(w, "app access", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
```

Se ainda não existir, criar em `access.go`:

```go
func pgInt8(n int64) pgtype.Int8 { return pgtype.Int8{Int64: n, Valid: true} }
```

Rotas:

```go
	mux.HandleFunc("GET /api/apps/{id}/access", h.getAppAccess)
	mux.HandleFunc("PUT /api/apps/{id}/access", h.setAppAccess)
	mux.HandleFunc("PUT /api/apps/{id}/grants", h.setAppGrants)
```

- [ ] **Step 3: Rodar os testes**

Run: `cd backend && set -a && . ../.env && set +a && mise x -- go vet ./... && mise x -- go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend
git commit -m "App access API: open or restricted, who sees it, extra accesses per app or area"
```

---
### Task 7: Frontend: tipos, permissões em português e o fim do `role`

**Files:**
- Modify: `frontend/src/lib/api.ts`, `frontend/src/lib/kinds.ts`, `frontend/src/test/fixtures.ts`
- Create: `frontend/src/lib/permissions.ts`, `frontend/src/lib/permissions.test.ts`
- Modify: `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`, `frontend/src/pages/CatalogPage.tsx`, `frontend/src/pages/MyRunsPage.tsx`, `frontend/src/pages/VersionPage.tsx`, `frontend/src/pages/AppPage.tsx` (só a troca de `canEdit`), `frontend/src/components/HistoryTimeline.tsx`, e os testes que montam `Person` com `role`.

**Interfaces:**
- Consumes: os formatos JSON dos Tasks 1–6.
- Produces (em `api.ts`):

```ts
export type Person = {
  id: number; name: string; email: string
  role_id: number; role_name: string; permissions: string[]
  leads: number[]; areas: number[]; sees_others_runs: boolean
}
export type PersonRow = { id: number; name: string; email: string; role_id: number; role_name: string; area_ids: number[] }
export type Can = { view: boolean; edit: boolean; restore: boolean; tech: boolean; runs: boolean; access: boolean; grants: boolean }
export type RoleInfo = { id: number; key: string; name: string; permissions: string[]; people_count: number }
export type NameRef = { id: number; name: string }
export type Area = { id: number; name: string; app_count: number; leaders: NameRef[]; members: NameRef[] }
export type Grant = { person_id: number; name: string; permissions: string[] }
export type AppAccess = { restricted: boolean; areas: NameRef[]; people: NameRef[]; grants: Grant[]; area_grants: Grant[] }
export type AppLink = { id: number; slug: string; name: string; visible: boolean }
```

`AppSummary` ganha `area_id: number; restricted: boolean; can: Can`. `AppDetail` ganha `area_id: number; restricted: boolean; can: Can; depends_on_apps?: AppLink[]; used_by?: AppLink[]`.
`api.people` passa a devolver `PersonRow[]`. `createPerson` recebe `{name, email, password, role_id}`. Chamadas novas:

```ts
  updateApp: (id: number, b: { owner_id: number; backup_id: number | null; manifest: Manifest; updated_at: string }) =>
    request<AppDetail>("PUT", `/api/apps/${id}`, b),
  setPersonRole: (id: number, role_id: number) => request<PersonRow>("PUT", `/api/people/${id}`, { role_id }),
  roles: () => request<RoleInfo[]>("GET", "/api/roles"),
  createRole: (b: { name: string; permissions: string[] }) => request<RoleInfo>("POST", "/api/roles", b),
  updateRole: (id: number, b: { name: string; permissions: string[] }) => request<RoleInfo>("PUT", `/api/roles/${id}`, b),
  deleteRole: (id: number) => request<void>("DELETE", `/api/roles/${id}`),
  areas: () => request<Area[]>("GET", "/api/areas"),
  createArea: (name: string) => request<Area>("POST", "/api/areas", { name }),
  renameArea: (id: number, name: string) => request<Area>("PUT", `/api/areas/${id}`, { name }),
  deleteArea: (id: number) => request<void>("DELETE", `/api/areas/${id}`),
  setAreaLeaders: (id: number, person_ids: number[]) => request<void>("PUT", `/api/areas/${id}/leaders`, { person_ids }),
  setAreaMembers: (id: number, person_ids: number[]) => request<void>("PUT", `/api/areas/${id}/members`, { person_ids }),
  appAccess: (id: number) => request<AppAccess>("GET", `/api/apps/${id}/access`),
  setAppAccess: (id: number, b: { restricted: boolean; area_ids: number[]; person_ids: number[] }) =>
    request<AppAccess>("PUT", `/api/apps/${id}/access`, b),
  setGrants: (id: number, b: { person_id: number; permissions: string[]; whole_area: boolean }) =>
    request<AppAccess>("PUT", `/api/apps/${id}/grants`, b),
```

`request` precisa aceitar 200 sem corpo (as rotas de líderes/membros respondem 200 vazio): se `res.status === 204` **ou** o corpo vier vazio, devolver `undefined`. Conferir como `request` trata hoje e só acrescentar a leitura de texto vazio.

- Produces (em `permissions.ts`):

```ts
import type { Person } from "./api"

export const PERMISSIONS: { key: string; label: string; hint: string }[] = [
  { key: "register_apps", label: "Registrar apps", hint: "Cadastra apps novos no catálogo" },
  { key: "edit_any_app", label: "Editar qualquer app", hint: "Muda a ficha de todos os apps" },
  { key: "restore_any_app", label: "Voltar versões de qualquer app", hint: "Desfaz mudanças no código de todos os apps" },
  { key: "see_tech_any_app", label: "Ver a parte técnica de qualquer app", hint: "Código, chaves, bibliotecas e histórico" },
  { key: "see_all_runs", label: "Ver execuções de todos", hint: "Quem rodou o quê, em todos os apps" },
  { key: "see_all_apps", label: "Ver todos os apps", hint: "Inclusive os restritos" },
  { key: "manage_people", label: "Cuidar das pessoas", hint: "Cadastra pessoas e escolhe o cargo delas" },
  { key: "manage_areas", label: "Cuidar das áreas", hint: "Cria áreas e escolhe os líderes" },
  { key: "manage_roles", label: "Cuidar dos cargos", hint: "Cria cargos e escolhe o que cada um pode" },
]

export const GRANTS: { key: string; label: string }[] = [
  { key: "edit", label: "Editar a ficha" },
  { key: "restore", label: "Voltar versões" },
  { key: "tech", label: "Ver a parte técnica" },
  { key: "runs", label: "Ver execuções de todos" },
]

export const has = (p: Person | null | undefined, perm: string) => !!p?.permissions.includes(perm)
export const canRegister = (p: Person | null | undefined) => has(p, "register_apps") || (p?.leads.length ?? 0) > 0
export const canSeeTeam = (p: Person | null | undefined) =>
  has(p, "manage_people") || has(p, "manage_areas") || has(p, "manage_roles") || (p?.leads.length ?? 0) > 0
export const permLabel = (key: string) => PERMISSIONS.find((x) => x.key === key)?.label ?? GRANTS.find((x) => x.key === key)?.label ?? key
```

- [ ] **Step 1: Teste de `permissions.ts`**

```ts
import { describe, expect, it } from "vitest"
import { canRegister, canSeeTeam, has } from "./permissions"
import { ana, bia, henry } from "@/test/fixtures"

describe("permissions", () => {
  it("reads what the backend sent", () => {
    expect(has(henry, "manage_roles")).toBe(true)
    expect(canRegister(ana)).toBe(true)
    expect(canRegister(bia)).toBe(false)
    expect(canSeeTeam(bia)).toBe(false)
    expect(canSeeTeam({ ...bia, leads: [1] })).toBe(true)
    expect(canRegister({ ...bia, leads: [1] })).toBe(true)
  })
})
```

`fixtures.ts`:

```ts
const ALL = ["register_apps", "edit_any_app", "restore_any_app", "see_tech_any_app", "see_all_runs", "see_all_apps", "manage_people", "manage_areas", "manage_roles"]
export const henry: Person = { id: 1, name: "Henry", email: "henry@alis.com", role_id: 1, role_name: "Administrador", permissions: ALL, leads: [], areas: [], sees_others_runs: true }
export const ana: Person = { id: 2, name: "Ana", email: "ana@alis.com", role_id: 2, role_name: "Criador", permissions: ["register_apps"], leads: [], areas: [1], sees_others_runs: false }
export const bia: Person = { id: 3, name: "Bia", email: "bia@alis.com", role_id: 3, role_name: "Usuário", permissions: [], leads: [], areas: [2], sees_others_runs: false }
export const ALL_CAN: Can = { view: true, edit: true, restore: true, tech: true, runs: true, access: true, grants: true }
export const VIEW_ONLY: Can = { view: true, edit: false, restore: false, tech: false, runs: false, access: false, grants: false }
```

`manifest()` ganha `area: "Research"`; as fábricas de `AppSummary`/`AppDetail` (se existirem nos testes) ganham `area_id: 1, restricted: false, can: ALL_CAN`.

Run: `cd frontend && mise x -- npx vitest run src/lib/permissions.test.ts` → FAIL.

- [ ] **Step 2: Escrever `api.ts` e `permissions.ts`**; tirar `Role`, `ROLES` e `roleLabel` de `api.ts`/`kinds.ts` e `AREAS` de `kinds.ts` (a lista agora vem de `api.areas()`; quem usa `AREAS` é só o `ReviewForm`, que muda no Task 8 — até lá, deixar `AREAS` no lugar e apagá-lo no Task 8).

Run: `cd frontend && mise x -- npx tsc --noEmit -p .` → a lista de erros é o mapa do Step 3.

- [ ] **Step 3: Trocar cada uso de `role`**

- `App.tsx`: `CreatorsOnly` → `if (!canRegister(person)) return <Navigate to="/" replace />`. Rota nova `equipe` (TeamPage entra no Task 10; até lá, aponte para `PeoplePage`), e `pessoas` vira `<Navigate to="/equipe" replace />`.
- `Layout.tsx`: `NAV` perde `roles` e ganha `show: (p: Person) => boolean`:

```ts
const NAV = [
  { to: "/", label: "Catálogo", icon: LibraryBig, show: () => true },
  { to: "/execucoes", label: "Minhas execuções", icon: History, show: () => true },
  { to: "/equipe", label: "Equipe", icon: Users, show: canSeeTeam },
]
```

  `roleLabel(person.role)` → `person.role_name` (duas vezes). O bloco "Em breve" (`person.role !== "usuario"`) → `canRegister(person) || canSeeTeam(person)`.
- `CatalogPage.tsx`: `canRegister(person)`. Um app com `restricted` mostra um cadeado pequeno (`Lock` do lucide, `size-3.5 text-muted-foreground`, `aria-label="Restrito"`) ao lado do nome.
- `MyRunsPage.tsx`: `person?.role === "admin"` → `person?.sees_others_runs`. O rótulo do botão continua o mesmo de hoje.
- `VersionPage.tsx`: buscar o app (`api.app(id)`, já há `id` na rota) e usar `app.can.edit` para descrever e `app.can.restore` para "Voltar a esta versão". Sem `can.tech`, o backend já responde 403; a página mostra o `LoadError` com a mensagem dele.
- `AppPage.tsx`: `canEdit` → `app.can.edit` (o desenho novo vem no Task 9).
- `HistoryTimeline.tsx`: a prop `canEdit` vira duas — `canEdit` (descrever versão) e `canRestore` (botão de voltar). Passar as duas em cada lugar que usa.

- [ ] **Step 4: Rodar tudo**

Run: `cd frontend && mise x -- npx tsc --noEmit -p . && mise x -- npx vitest run`
Expected: PASS. Testes que esperavam "Pessoas" no menu passam a esperar "Equipe" só para quem `canSeeTeam`; acrescentar ao `Layout.test.tsx`:

```tsx
it("shows Equipe only to who takes care of the team", () => {
  renderLayout(bia)
  expect(screen.queryByRole("link", { name: /Equipe/ })).toBeNull()
  renderLayout(henry)
  expect(screen.getAllByRole("link", { name: /Equipe/ }).length).toBeGreaterThan(0)
})
```

(usar o ajudante de render que `Layout.test.tsx` já tem; o nome pode ser outro.)

- [ ] **Step 5: Commit**

```bash
git add frontend
git commit -m "Frontend reads permissions from the Hub: Equipe in the menu, can per app"
```

---

### Task 8: Editores compartilhados da ficha (cadastro e ficha usam os mesmos)

**Files:**
- Create: `frontend/src/components/ficha/editors.tsx`, `frontend/src/components/ficha/editors.test.tsx`
- Modify: `frontend/src/components/ReviewForm.tsx` + `ReviewForm.test.tsx`, `frontend/src/pages/RegisterPage.tsx`, `frontend/src/lib/kinds.ts` (apaga `AREAS`)

**Interfaces:**
- Consumes: `Manifest`, `Input`, `Area`, `AppSummary`, `PersonRow` (Task 7).
- Produces (todos controlados: recebem o valor e devolvem o novo por `onChange`, sem estado próprio de dados):

```ts
type Edit<T> = { value: T; onChange: (v: T) => void }
export function IdentityEditor(p: Edit<Manifest> & { areas: Area[] }): JSX.Element      // nome, resumo, tipo, área (select da lista)
export function InputsEditor(p: Edit<Input[]>): JSX.Element                              // + opções e "Pode marcar mais de uma" quando type === "choice"
export function OutputsEditor(p: Edit<Output[]>): JSX.Element
export function NotesEditor(p: Edit<string[]>): JSX.Element                              // um Textarea, uma observação por linha
export function DependsEditor(p: Edit<string[]> & { apps: AppSummary[]; selfSlug: string }): JSX.Element
export function BuildEditor(p: Edit<Manifest["build"]>): JSX.Element                    // pasta do código-fonte + comando
export function PeopleEditor(p: { people: PersonRow[]; ownerId: number; backupId: number; onChange: (ownerId: number, backupId: number) => void }): JSX.Element
export function choiceProblems(inputs: Input[]): string[]                                // ["as opções de \"X\""] para cada choice sem opção
```

- [ ] **Step 1: Testes dos editores**

`editors.test.tsx`:

```tsx
import { useState } from "react"
import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { Input } from "@/lib/api"
import { DependsEditor, InputsEditor, NotesEditor, choiceProblems } from "./editors"
import { appSummary } from "@/test/fixtures"

function Harness({ start }: { start: Input[] }) {
  const [v, setV] = useState(start)
  return (
    <>
      <InputsEditor value={v} onChange={setV} />
      <pre data-testid="out">{JSON.stringify(v)}</pre>
    </>
  )
}

const out = () => JSON.parse(screen.getByTestId("out").textContent!)

describe("InputsEditor", () => {
  it("asks for options when the input is a list", async () => {
    render(<Harness start={[{ id: "docs", label: "Documentos", type: "file", source: "window", required: true }]} />)
    await userEvent.selectOptions(screen.getByLabelText("Tipo de Documentos"), "choice")
    await userEvent.type(screen.getByLabelText("Opções de Documentos"), "FRE{enter}DFP")
    await userEvent.click(screen.getByLabelText("Pode marcar mais de uma"))
    expect(out()[0]).toMatchObject({ type: "choice", options: ["FRE", "DFP"], multiple: true })
  })

  it("drops options when the input stops being a list", async () => {
    render(<Harness start={[{ id: "d", label: "D", type: "choice", source: "window", required: true, options: ["A"], multiple: true }]} />)
    await userEvent.selectOptions(screen.getByLabelText("Tipo de D"), "text")
    expect(out()[0].options).toBeUndefined()
    expect(out()[0].multiple).toBeUndefined()
  })
})

describe("choiceProblems", () => {
  it("names each list without options", () => {
    expect(choiceProblems([{ id: "d", label: "Docs", type: "choice", source: "window", required: true }])).toEqual(['as opções de "Docs"'])
  })
})

describe("NotesEditor", () => {
  it("keeps one note per line and drops empty lines", async () => {
    let got: string[] = []
    render(<NotesEditor value={[]} onChange={(v) => (got = v)} />)
    await userEvent.type(screen.getByLabelText("Observações"), "Precisa da VPN.{enter}{enter}Roda às 9h.")
    expect(got).toEqual(["Precisa da VPN.", "Roda às 9h."])
  })
})

describe("DependsEditor", () => {
  it("lists other apps, not itself, and keeps unknown ids", async () => {
    let got: string[] = []
    const apps = [appSummary({ id: 1, slug: "files-prep", name: "Files Prep" }), appSummary({ id: 2, slug: "eu", name: "Eu" })]
    render(<DependsEditor value={["sumiu"]} onChange={(v) => (got = v)} apps={apps} selfSlug="eu" />)
    expect(screen.queryByLabelText("Eu")).toBeNull()
    expect(screen.getByText(/sumiu/)).toBeInTheDocument()
    await userEvent.click(screen.getByLabelText("Files Prep"))
    expect(got).toEqual(["sumiu", "files-prep"])
  })
})
```

(`appSummary` já existe em `fixtures.ts`; acrescente `area_id: 1, restricted: false, can: ALL_CAN` aos padrões dela no Task 7.)

Run: `cd frontend && mise x -- npx vitest run src/components/ficha` → FAIL.

- [ ] **Step 2: Escrever `editors.tsx`**

Mover para cá, sem mudar a aparência, os trechos de `ReviewForm.tsx` que hoje editam identidade (linhas do bloco "Identidade"), pessoas (bloco "Pessoas"), entradas ("O que ele recebe") e resultados ("O que ele devolve"). Mudanças:

- Área: `NativeSelect` com `areas.map(a => <option value={a.name}>)` e uma primeira opção vazia "Escolha a área". Se `value.area` não está na lista, mostrar a opção dele também, com o texto `"<nome> (fora da lista)"`, para a pessoa ver o que havia e trocar.
- Entradas: cada linha ganha `aria-label={\`Tipo de ${inp.label || "entrada"}\`}` no select de tipo. Quando `type === "choice"`, abaixo da linha:

```tsx
<div className="col-span-full flex flex-col gap-2 pl-1">
  <Textarea
    aria-label={`Opções de ${inp.label || "entrada"}`}
    rows={Math.max(2, (inp.options ?? []).length + 1)}
    placeholder="Uma opção por linha"
    value={(inp.options ?? []).join("\n")}
    onChange={(e) => setInput(i, { options: e.target.value.split("\n") })}
    onBlur={(e) => setInput(i, { options: e.target.value.split("\n").map((s) => s.trim()).filter(Boolean) })}
  />
  <label className="flex items-center gap-2 text-sm">
    <Checkbox checked={!!inp.multiple} onCheckedChange={(c) => setInput(i, { multiple: c === true })} />
    Pode marcar mais de uma
  </label>
</div>
```

  Trocar o tipo para outro que não `choice` remove `options` e `multiple` do objeto (`const { options, multiple, ...rest } = inp`).
  Como o `onChange` guarda as linhas cruas (inclusive vazias) enquanto se digita, o teste acima espera `["FRE","DFP"]` porque não há linha vazia; o `onBlur` limpa. `choiceProblems` considera só opções não vazias.
- Observações: um `Textarea` (`aria-label="Observações"`), valor `value.join("\n")`, `onChange` → `split("\n").map(trim).filter(Boolean)`. Para não engolir a linha que a pessoa está digitando, guardar o texto cru num `useState` local e só repassar a lista limpa (o teste digita `{enter}{enter}` no meio).
- Outros apps: lista de `Checkbox` com o nome de cada app de `apps` (menos `selfSlug`), marcados se `value.includes(slug)`. Ids em `value` que não estão em `apps` aparecem acima, como texto: `"<id> · fora do catálogo"` com um botão "Tirar". Ordem do resultado: os que já estavam, depois os novos, na ordem em que foram marcados.
- Código-fonte: dois campos, "Pasta do código-fonte" (`source`) e "Comando que gera o programa" (`command`). Os dois vazios → `onChange(null)`.
- Pessoas: os dois `NativeSelect` de hoje, com `people: PersonRow[]`.

- [ ] **Step 3: `ReviewForm` usa os editores**

`ReviewForm` passa a receber `areas: Area[]` e `apps: AppSummary[]` nas props e troca os blocos pelos editores. Blocos novos, depois de "O que ele devolve": "Outros apps que ele usa" (`DependsEditor`, `selfSlug={m.id}`) e "Código-fonte" (`BuildEditor`), os dois só quando `m.kind === "executable"` **ou** o campo já tem valor (é onde fazem sentido; o resto não vê o bloco). `missingForPublish` ganha `...choiceProblems(m.inputs ?? [])` e "a área" quando `!m.area || !areas.some(a => a.name === m.area)`.

`RegisterPage`: carregar `api.areas()` e `api.apps()` junto com `api.people()` e passar ao `ReviewForm`. Apagar `AREAS` de `kinds.ts`.

Teste novo em `ReviewForm.test.tsx`:

```tsx
it("blocks publishing until the area comes from the list", async () => {
  renderForm({ item: scanItem({ manifest: manifest({ area: "Marketing" }) }), areas: [{ id: 1, name: "Research", app_count: 0, leaders: [], members: [] }] })
  expect(screen.getByText(/a área/)).toBeInTheDocument()
  await userEvent.selectOptions(screen.getByLabelText("Área"), "Research")
  expect(screen.queryByText(/a área/)).toBeNull()
})
```

(`renderForm` = o ajudante que o arquivo já usa; ajustar para aceitar `areas` e `apps`, com padrão `[{id:1,name:"Research",...}]` e `[]`.)

- [ ] **Step 4: Rodar**

Run: `cd frontend && mise x -- npx tsc --noEmit -p . && mise x -- npx vitest run`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend
git commit -m "Ficha editors shared by registration and ficha: area from the list, options, other apps, source code"
```

---

### Task 9: A ficha no desenho B (uma página, técnico recolhido no fim, "Editar" por parte)

**Files:**
- Create: `frontend/src/components/ficha/Part.tsx`, `frontend/src/components/ficha/AccessSection.tsx`, `frontend/src/components/ficha/AccessSection.test.tsx`
- Modify: `frontend/src/pages/AppPage.tsx`, `frontend/src/pages/AppPage.test.tsx`

**Interfaces:**
- Consumes: editores do Task 8, `api.updateApp`, `api.appRuns`, `api.appAccess`, `api.setAppAccess`, `api.setGrants`, `api.areas`, `api.people`, `api.apps`, `AppDetail.can`, `depends_on_apps`, `used_by`.
- Produces:

```ts
// Part shows one piece of the ficha. With canEdit, a quiet "Editar" opens edit(),
// and Salvar/Cancelar close it. Cancel with changes asks first.
export function Part(p: {
  title: string
  canEdit: boolean
  dirty: boolean
  editing: boolean              // the page decides which part is open
  onEdit: () => void
  onSave: () => Promise<void>   // throws ApiError to show its message
  onCancel: () => void
  view: ReactNode
  edit: ReactNode
  extra?: ReactNode             // e.g. the "Recarregar" button on a conflict
}): JSX.Element
export function AccessSection(p: { app: AppDetail; me: Person }): JSX.Element
```

**Ordem da página (desenho B, como no protótipo `docs/superpowers/specs/2026-10-05-prototipo-ficha.html`):**

1. Cabeçalho de hoje (nome, tipo, linha de meta, botão Rodar). O resumo e a área fazem parte de "Sobre" — o "Editar" de "Sobre" abre `IdentityEditor`.
2. **O que ele pede** (`InputsEditor`) — leitura: a lista de hoje (`InputList`), e para `choice` as opções em linha separadas por " · ", com "(pode marcar mais de uma)" quando `multiple`.
3. **O que ele entrega** (`OutputsEditor`).
4. **Execuções** — `api.appRuns(id)`: as 5 últimas (data, quem, estado, nº de arquivos), cada uma com link para `/execucoes/<id>` como hoje em Minhas execuções. Título: "Suas execuções" quando `!can.runs`, "Execuções" quando `can.runs`. Vazio: "Ninguém rodou ainda." / "Você ainda não rodou este app." Sem "Editar".
5. **Quem cuida** (`PeopleEditor`) — responsável e reserva.
6. `<details>` **"Para quem cuida do app"** — só se `can.tech`; fechado ao abrir a página; abrir/fechar fica lembrado por app em `localStorage` (`apphub-tech-open-<id>`, com try/catch). Dentro, nesta ordem:
   - Observações (`NotesEditor`; leitura = `Notes` de hoje, 3 + "Ver mais").
   - Outros apps (`DependsEditor`) — leitura: "Usa:" com links para `depends_on_apps` (link só se `visible`; `id === 0` → "<slug> · fora do catálogo"; não visível → só o nome) e "É usado por:" com `used_by`. Some quando as duas listas estão vazias e não há "Editar".
   - Código-fonte (`BuildEditor`) — some quando vazio e `kind !== "executable"`.
   - Chaves e senhas, bibliotecas, como roda (o que já existe hoje na página, sem "Editar").
   - Histórico (`HistoryTimeline` com `canEdit={can.edit}` e `canRestore={can.restore}`).
   - Onde está (o bloco de hoje).
   - **Acesso** (`AccessSection`).

**Salvar uma parte:** cada parte guarda um rascunho (`useState`) iniciado do `app` ao clicar "Editar". Salvar = `api.updateApp(app.id, { owner_id, backup_id, manifest: { ...app.manifest, ...rascunhoDaParte }, updated_at: app.updated_at })` → `setApp(resposta)` e `setHistoryKey(k => k + 1)`. Só uma parte aberta por vez: abrir outra com mudanças pede para descartar (mesmo `confirm` do Cancelar). Erros: a mensagem do backend aparece dentro da parte (`role="alert"`), a parte continua aberta; para 409 de conflito a mensagem já diz para recarregar, e um botão "Recarregar" ao lado chama o mesmo carregamento da página.

- [ ] **Step 1: `Part.tsx`**

```tsx
import { useState, type ReactNode } from "react"
import { Button } from "@/components/ui/button"

export function Part({ title, canEdit, dirty, onSave, onCancel, view, edit, editing, onEdit, extra }: {
  title: string
  canEdit: boolean
  dirty: boolean
  editing: boolean
  onEdit: () => void
  onSave: () => Promise<void>
  onCancel: () => void
  view: ReactNode
  edit: ReactNode
  extra?: ReactNode
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const save = async () => {
    setBusy(true)
    setError("")
    try {
      await onSave()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não consegui salvar.")
    } finally {
      setBusy(false)
    }
  }
  const cancel = () => {
    if (dirty && !window.confirm("Descartar o que você mudou?")) return
    setError("")
    onCancel()
  }
  return (
    <section className="border-t border-border py-5" aria-labelledby={`part-${title}`}>
      <div className="mb-3 flex items-baseline justify-between gap-3">
        <h2 id={`part-${title}`} className="text-sm font-semibold">{title}</h2>
        {canEdit && !editing && (
          <button type="button" className="text-sm text-muted-foreground hover:text-foreground" onClick={onEdit} aria-label={`Editar ${title}`}>
            Editar
          </button>
        )}
      </div>
      {editing ? (
        <div className="flex flex-col gap-3">
          {edit}
          {error && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error} {extra}</p>}
          <div className="flex gap-2">
            <Button size="sm" onClick={save} disabled={busy}>{busy ? "Salvando…" : "Salvar"}</Button>
            <Button size="sm" variant="ghost" onClick={cancel} disabled={busy}>Cancelar</Button>
          </div>
        </div>
      ) : (
        view
      )}
    </section>
  )
}
```


- [ ] **Step 2: Testes da ficha**

Em `AppPage.test.tsx`: trocar o `detail(can_run)` local por `detail(over: Partial<AppDetail> = {})` em `fixtures.ts` (padrão `can_run: true`, `can: ALL_CAN`; os testes antigos passam a chamar `detail({ can_run: false })`). `mockApp(d)` = o `base(d)` que o arquivo já tem, mais `vi.spyOn(api, "appRuns").mockResolvedValue([])`. `renderPage(person)` = o `renderAt` de hoje com a pessoa no `AuthContext`.

```tsx
it("a colleague sees use and runs, no Editar and no technical block", async () => {
  mockApp(detail({ can: VIEW_ONLY }))
  renderPage(bia)
  expect(await screen.findByText("O que ele pede")).toBeInTheDocument()
  expect(screen.queryByText("Editar")).toBeNull()
  expect(screen.queryByText("Para quem cuida do app")).toBeNull()
  expect(screen.getByText("Suas execuções")).toBeInTheDocument()
})

it("saves one part and shows the new ficha", async () => {
  const d = detail()
  mockApp(d)
  const update = vi.spyOn(api, "updateApp").mockResolvedValue({ ...d, manifest: { ...d.manifest, outputs: [{ label: "Relatório", pattern: "*.pdf" }] }, updated_at: "2026-10-05T12:00:00Z" })
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  const label = screen.getByLabelText("Descrição do resultado")
  await userEvent.clear(label)
  await userEvent.type(label, "Relatório")
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(update).toHaveBeenCalledWith(d.id, expect.objectContaining({ updated_at: d.updated_at }))
  expect(await screen.findByText("Relatório")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Salvar" })).toBeNull()
})

it("keeps the part open and offers reload on a conflict", async () => {
  mockApp(detail())
  vi.spyOn(api, "updateApp").mockRejectedValue(new ApiError(409, "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova."))
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Recarregue")
  expect(screen.getByRole("button", { name: "Recarregar" })).toBeInTheDocument()
})

it("asks before discarding changes", async () => {
  mockApp(detail())
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false)
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  await userEvent.type(screen.getByLabelText("Descrição do resultado"), "x")
  await userEvent.click(screen.getByRole("button", { name: "Cancelar" }))
  expect(confirm).toHaveBeenCalled()
  expect(screen.getByRole("button", { name: "Salvar" })).toBeInTheDocument()
})

it("shows a lost app as not found", async () => {
  vi.spyOn(api, "app").mockRejectedValue(new ApiError(404, "App não encontrado."))
  renderPage(bia)
  expect(await screen.findByText("App não encontrado.")).toBeInTheDocument()
})

it("links other apps and marks the unknown ones", async () => {
  mockApp(detail({ depends_on_apps: [{ id: 7, slug: "files-prep", name: "Files Prep", visible: true }, { id: 0, slug: "sumiu", name: "sumiu", visible: false }], used_by: [] }))
  renderPage(henry)
  await userEvent.click(await screen.findByText("Para quem cuida do app"))
  expect(screen.getByRole("link", { name: "Files Prep" })).toHaveAttribute("href", "/apps/7")
  expect(screen.getByText(/sumiu · fora do catálogo/)).toBeInTheDocument()
})
```

(`ApiError` — conferir a ordem dos argumentos do construtor em `api.ts` e usar a mesma.)

Run: `cd frontend && mise x -- npx vitest run src/pages/AppPage.test.tsx` → FAIL.

- [ ] **Step 3: Reescrever `AppPage.tsx`** seguindo a ordem acima. O carregamento, `checkApp`, o aviso de pasta e o `RunDialog` ficam como estão. Estado novo:

```ts
type PartKey = "sobre" | "pede" | "entrega" | "pessoas" | "notas" | "outros" | "codigo"
const [open, setOpen] = useState<PartKey | null>(null)
const [draft, setDraft] = useState<{ manifest: Manifest; ownerId: number; backupId: number } | null>(null)
const dirty = !!draft && !!app && (JSON.stringify(draft.manifest) !== JSON.stringify(app.manifest) || draft.ownerId !== app.owner_id || draft.backupId !== (app.backup_id ?? 0))
const startEdit = (k: PartKey) => {
  if (open && dirty && !window.confirm("Descartar o que você mudou?")) return
  setDraft({ manifest: structuredClone(app!.manifest), ownerId: app!.owner_id, backupId: app!.backup_id ?? 0 })
  setOpen(k)
}
const save = async () => {
  const saved = await api.updateApp(app!.id, {
    owner_id: draft!.ownerId,
    backup_id: draft!.backupId || null,
    manifest: draft!.manifest,
    updated_at: app!.updated_at,
  })
  setApp(saved)
  setOpen(null)
  setDraft(null)
  setHistoryKey((k) => k + 1)
}
```

O rascunho é a ficha inteira, mas só a parte aberta tem editor, então só ela muda. Áreas, pessoas e apps (para os editores) carregam na primeira vez que alguém clica "Editar" (`Promise.all([api.areas(), api.people(), api.apps()])`), e não ao abrir a página.

- [ ] **Step 4: `AccessSection`**

Leitura (quem tem `can.tech`): `api.appAccess(app.id)` ao abrir o bloco técnico.

- "Quem vê: Todos" ou "Quem vê: só <áreas> e <pessoas>" (+ "Líderes, responsável e reserva sempre veem.").
- "Acessos extras:" lista `grants` (`"<nome>: Editar a ficha, Ver execuções de todos"` com `permLabel`) e `area_grants` com o sufixo "· todos os apps da área". Vazio: "Ninguém tem acesso extra."

Edição:
- `can.access` → "Editar" em "Quem vê": rádio "Todos" / "Só algumas áreas e pessoas"; com o segundo, caixas das áreas (`api.areas()`) e um select + lista de pessoas. Salvar → `api.setAppAccess`.
- `can.grants` → "Dar acesso extra": select da pessoa, 4 caixas (`GRANTS`), caixa "Em todos os apps da área <nome>", botão "Salvar". Cada linha de acesso existente tem "Mudar" (preenche o formulário) e "Tirar" (`setGrants` com `permissions: []`).

Teste em `AccessSection.test.tsx`:

```tsx
it("the owner restricts but does not see the grants form", async () => {
  vi.spyOn(api, "appAccess").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [] })
  vi.spyOn(api, "areas").mockResolvedValue([{ id: 2, name: "Investimentos", app_count: 1, leaders: [], members: [] }])
  vi.spyOn(api, "people").mockResolvedValue([])
  const set = vi.spyOn(api, "setAppAccess").mockResolvedValue({ restricted: true, areas: [{ id: 2, name: "Investimentos" }], people: [], grants: [], area_grants: [] })
  render(<AccessSection app={detail({ can: { ...ALL_CAN, grants: false } })} me={ana} />)
  expect(await screen.findByText(/Quem vê: Todos/)).toBeInTheDocument()
  expect(screen.queryByText("Dar acesso extra")).toBeNull()
  await userEvent.click(screen.getByRole("button", { name: "Editar Quem vê" }))
  await userEvent.click(screen.getByLabelText("Só algumas áreas e pessoas"))
  await userEvent.click(screen.getByLabelText("Investimentos"))
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(set).toHaveBeenCalledWith(expect.any(Number), { restricted: true, area_ids: [2], person_ids: [] })
  expect(await screen.findByText(/só Investimentos/)).toBeInTheDocument()
})

it("the leader gives an extra access for the whole area", async () => {
  vi.spyOn(api, "appAccess").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [] })
  vi.spyOn(api, "areas").mockResolvedValue([])
  vi.spyOn(api, "people").mockResolvedValue([{ id: 3, name: "Bia", email: "bia@alis.com", role_id: 3, role_name: "Usuário", area_ids: [] }])
  const set = vi.spyOn(api, "setGrants").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [{ person_id: 3, name: "Bia", permissions: ["runs"] }] })
  render(<AccessSection app={detail()} me={henry} />)
  await userEvent.selectOptions(await screen.findByLabelText("Pessoa"), "3")
  await userEvent.click(screen.getByLabelText("Ver execuções de todos"))
  await userEvent.click(screen.getByLabelText(/Em todos os apps da área/))
  await userEvent.click(screen.getByRole("button", { name: "Dar acesso" }))
  expect(set).toHaveBeenCalledWith(expect.any(Number), { person_id: 3, permissions: ["runs"], whole_area: true })
  expect(await screen.findByText(/todos os apps da área/)).toBeInTheDocument()
})
```

- [ ] **Step 5: Rodar**

Run: `cd frontend && mise x -- npx tsc --noEmit -p . && mise x -- npx vitest run`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend
git commit -m "Ficha design B: use first, technical block folded at the end, Editar per part, access block"
```

---

### Task 10: Tela Equipe (Pessoas · Áreas · Cargos)

**Files:**
- Create: `frontend/src/pages/TeamPage.tsx`, `frontend/src/pages/TeamPage.test.tsx`
- Delete: `frontend/src/pages/PeoplePage.tsx`
- Modify: `frontend/src/App.tsx` (rota `equipe` → `TeamPage`)

**Interfaces:**
- Consumes: `api.people`, `api.createPerson`, `api.setPersonRole`, `api.roles`, `api.createRole`, `api.updateRole`, `api.deleteRole`, `api.areas`, `api.createArea`, `api.renameArea`, `api.deleteArea`, `api.setAreaLeaders`, `api.setAreaMembers`, `PERMISSIONS`, `has`, `useAuth().refresh` (já existe em `lib/auth.tsx`; usar depois de mudar o próprio cargo).
- Produces: `export function TeamPage(): JSX.Element`; abas na URL `?aba=pessoas|areas|cargos`.

**Quem vê cada aba:**
- Pessoas: `manage_people` (lista, cadastrar, trocar cargo) — líder sem `manage_people` vê a aba só para ler.
- Áreas: todos que veem a Equipe. Criar, renomear, apagar e escolher líderes: `manage_areas`. Membros: `manage_areas` ou líder daquela área.
- Cargos: `manage_roles`.
A primeira aba visível é a padrão.

**Conteúdo:**
- **Pessoas** — linhas com nome, e-mail, áreas (nomes separados por vírgula) e o cargo como `NativeSelect` (para `manage_people`; para os outros, texto). Mudar o select chama `setPersonRole` na hora; erro (o 409 do último gestor) aparece numa linha `role="alert"` abaixo da pessoa e o select volta ao valor antigo. Se a pessoa mudada é `me`, chamar `refresh()`. Formulário "Cadastrar pessoa" de hoje, com o select de cargos vindo de `api.roles()` (padrão: o cargo de key `usuario`).
- **Áreas** — uma linha por área: nome, "N apps", líderes, membros. Ações em texto discreto: "Renomear" (vira campo + Salvar), "Apagar" (só se `app_count === 0`, com `confirm`), "Líderes" e "Membros" (abrem uma lista de caixas com as pessoas; Salvar chama `setAreaLeaders`/`setAreaMembers`). Formulário "Nova área" no fim.
- **Cargos** — uma linha por cargo: nome, "N pessoas", e as permissões em texto (`permLabel`, separadas por " · "; "Nenhuma permissão" quando vazio). "Editar" abre nome + as 9 caixas com `label` e `hint` de `PERMISSIONS`. "Apagar" aparece só quando `key === ""` e `people_count === 0`. "Novo cargo" no fim, com o mesmo formulário.

- [ ] **Step 1: Testes**

```tsx
describe("TeamPage", () => {
  beforeEach(() => {
    vi.spyOn(api, "people").mockResolvedValue([
      { id: 1, name: "Henry", email: "henry@alis.com", role_id: 1, role_name: "Administrador", area_ids: [] },
      { id: 3, name: "Bia", email: "bia@alis.com", role_id: 3, role_name: "Usuário", area_ids: [2] },
    ])
    vi.spyOn(api, "roles").mockResolvedValue([
      { id: 1, key: "admin", name: "Administrador", permissions: PERMISSIONS.map((p) => p.key), people_count: 1 },
      { id: 3, key: "usuario", name: "Usuário", permissions: [], people_count: 1 },
      { id: 9, key: "", name: "Sub-admin", permissions: ["see_all_runs"], people_count: 0 },
    ])
    vi.spyOn(api, "areas").mockResolvedValue([{ id: 2, name: "Investimentos", app_count: 0, leaders: [], members: [{ id: 3, name: "Bia" }] }])
  })

  it("shows the three tabs to an admin", async () => {
    renderAt("/equipe", henry)
    for (const t of ["Pessoas", "Áreas", "Cargos"]) expect(await screen.findByRole("tab", { name: t })).toBeInTheDocument()
  })

  it("a leader sees only Áreas and can set members of their area", async () => {
    const set = vi.spyOn(api, "setAreaMembers").mockResolvedValue()
    renderAt("/equipe", { ...bia, leads: [2] })
    expect(await screen.findByRole("tab", { name: "Áreas" })).toBeInTheDocument()
    expect(screen.queryByRole("tab", { name: "Cargos" })).toBeNull()
    expect(screen.queryByRole("button", { name: "Renomear Investimentos" })).toBeNull()
    await userEvent.click(screen.getByRole("button", { name: "Membros de Investimentos" }))
    await userEvent.click(screen.getByLabelText("Henry"))
    await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
    expect(set).toHaveBeenCalledWith(2, [3, 1])
  })

  it("puts the role back when the Hub refuses the change", async () => {
    vi.spyOn(api, "setPersonRole").mockRejectedValue(new ApiError(409, "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos."))
    renderAt("/equipe", henry)
    const select = await screen.findByLabelText("Cargo de Henry")
    await userEvent.selectOptions(select, "3")
    expect(await screen.findByRole("alert")).toHaveTextContent("ao menos uma pessoa")
    expect(select).toHaveValue("1")
  })

  it("edits a role's permissions with plain names", async () => {
    const upd = vi.spyOn(api, "updateRole").mockResolvedValue({ id: 9, key: "", name: "Sub-admin", permissions: ["see_all_runs", "edit_any_app"], people_count: 0 })
    renderAt("/equipe?aba=cargos", henry)
    await userEvent.click(await screen.findByRole("button", { name: "Editar Sub-admin" }))
    await userEvent.click(screen.getByLabelText("Editar qualquer app"))
    await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
    expect(upd).toHaveBeenCalledWith(9, { name: "Sub-admin", permissions: ["see_all_runs", "edit_any_app"] })
    expect(screen.getByRole("button", { name: "Apagar Sub-admin" })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Apagar Administrador" })).toBeNull()
  })
})
```

(`renderAt(path, person)` = `MemoryRouter` + `AuthContext` com a pessoa, como os outros testes de página fazem.)

Run: `cd frontend && mise x -- npx vitest run src/pages/TeamPage.test.tsx` → FAIL.

- [ ] **Step 2: Escrever `TeamPage.tsx`** como descrito, reaproveitando `PageHeader`, `Field`, `NativeSelect`, `LoadError` e o formulário de cadastro de `PeoplePage`. As abas usam o mesmo estilo dos botões de aba de `VersionPage` (`role="tab"`, `aria-selected`). Apagar `PeoplePage.tsx`.

- [ ] **Step 3: Rodar**

Run: `cd frontend && mise x -- npx tsc --noEmit -p . && mise x -- npx vitest run`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A frontend
git commit -m "Equipe screen: people with roles, areas with leaders and members, custom roles"
```

---

### Task 11: Conferência visual, documentos, Hub real e entrega

**Files:**
- Create: `e2e/etapa2.mjs`
- Create: `docs/adr/009-cargos-areas-e-acessos.md`
- Modify: `docs/PRD.md`, `docs/TASKS.md`, `DESIGN.md`

- [ ] **Step 1: Todos os testes**

```bash
cd backend && set -a && . ../.env && set +a && mise x -- go vet ./... && mise x -- go test ./...
cd ../frontend && mise x -- npx tsc --noEmit -p . && mise x -- npx vitest run && mise x -- npm run build
```

Expected: tudo PASS e o build sem erro.

- [ ] **Step 2: Reiniciar o Hub real na 8080** para aplicar a migração 007. Ver o comando em uso (`ps -ef | grep -v grep | grep server` ou o histórico em `backend/hub.log`), parar o processo e subir de novo com o mesmo comando e `DEV_MODE=1`. Conferir no log que a migração 007 rodou e `curl -s localhost:8080/up` → 200. **Não** criar dados de teste nesse Hub: ele é o do Henry.

- [ ] **Step 3: Conferência visual** — `e2e/etapa2.mjs`, só leitura, contra o Vite (porta do `.env`, `BASE_URL`), logado como o Henry (o mesmo jeito de logar dos scripts `e2e/*.mjs` que já existem). Capturas em `e2e/shots/etapa2/`:

- Catálogo; ficha do Files Prep (bloco técnico fechado e aberto); ficha com a parte "O que ele pede" em edição (clicar "Editar", capturar, clicar "Cancelar" — nada é salvo); Equipe nas três abas.
- Cada uma em claro, escuro (`colorScheme: "dark"`) e celular (390×844).

Abrir cada imagem e conferir: alinhamento, "Editar" discreto, violeta só no Rodar, o `<details>` legível, nada estourando a largura no celular. Corrigir e repetir até ficar bom.

- [ ] **Step 4: Documentos**

- `docs/adr/009-cargos-areas-e-acessos.md` no modelo das outras ADRs: contexto (um "role" fixo não dava conta), decisão (cargo por pessoa com 9 permissões, áreas com líderes, acessos extras por app ou área, app aberto ou restrito, regra pura em `internal/access`, o frontend só lê `can`), alternativas (perfis fixos; permissão por app para tudo; "Usar/Cuidar" em abas — descartado depois do protótipo).
- `docs/PRD.md`: nova seção 3.7 "Equipe e permissões" (o que cada pessoa pode, em linguagem de produto) e a ficha no desenho B na seção da ficha.
- `docs/TASKS.md`: Etapa 2 — itens feitos marcados como Done; o que ficou para depois (Parar no Agente 0.2.4, Etapa 3) como Pending.
- `DESIGN.md`: a ficha B (ordem das partes, "Editar" em texto cinza, `<details>` "Para quem cuida do app") e a tela Equipe (abas).

- [ ] **Step 5: Commit e push**

```bash
git add -A
git commit -m "Etapa 2: roles, areas, restricted apps and the editable ficha"
git push
```
