# Apps compartilhados — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um app marcado como compartilhado roda uma vez só no servidor do Hub, num contêiner, com banco próprio, configurações e senhas guardadas, porteiro com o login do Hub, vigia (religar, dormir, medir), cópias de segurança, troca de versão com volta, e uma tela de servidor e gastos para o administrador. Tudo roda e é testado no PC do Henry (WSL + Podman).

**Architecture:** Pacotes Go novos sob `backend/internal/shared/` (motor, banco por app, receitas, checagem, implantação, porteiro, vigia, custos) mais `backend/internal/vault` (cofre). Nenhum pacote além de `shared/engine` conhece o Podman; nenhum pacote além de `shared/appdb` fala SQL de administração. O servidor do Hub passa a olhar o `Host` de cada pedido: subdomínio de app compartilhado vai para o porteiro, o resto segue para o mux de hoje. O vigia é uma goroutine única iniciada no `main.go`. As telas novas reaproveitam a ficha (desenho B, `Section`/`Part`) e o estilo do `DESIGN.md`.

**Tech Stack:** Go stdlib (`net/http`, `net/http/httputil`, `crypto/aes`, `crypto/cipher`, `os/exec`), pgx/v5, sqlc, migrações embutidas; Podman 4.9 (CLI); React 19 + TypeScript + Tailwind + shadcn/ui; Vitest + Testing Library; Playwright para fotos.

**Spec:** `docs/superpowers/specs/2026-10-06-apps-compartilhados-design.md`

## Global Constraints

- Todo texto de tela em português simples, sem jargão; erro sempre com o próximo passo. Detalhe técnico vai para o `hub.log` (`slog`) e, para quem vê o técnico, para "Ver detalhes".
- SQL só em `backend/internal/database/queries/*.sql` + `cd backend && mise x -- sqlc generate`; nunca SQL de dados no handler. Exceção: `shared/appdb` executa SQL de administração (`CREATE DATABASE`, `CREATE ROLE`) com a conexão `APPS_DB_ADMIN_URL`. Ler os `.go` gerados pelo sqlc antes de usar os nomes; ajuste o uso, não a intenção.
- Migração nova: `backend/internal/database/migrations/008_shared.sql` (roda numa transação; ver `database/migrate.go`). `CREATE DATABASE` **não** pode ir na migração.
- Comandos Go/sqlc rodam de `backend/` com `mise x --` e `set -a && . ../.env && set +a` antes dos testes. Frontend em `frontend/` com `mise x -- npm`.
- Banco local: container `app-hub-db`, porta 5433. **Nunca subir um Hub de teste na porta 8080** (o Agente real do Henry fala com ele). Testes de handler usam `apphub_test`.
- Contêineres e imagens criados pelo Hub sempre com o rótulo `app-hub.app=<app_id>` e nome `apphub-<slug>-<deployment_id>`; imagens `localhost/apphub/<slug>:<deployment_id>`; rede `app-hub-apps`. O Hub **nunca** mexe em contêiner sem esse rótulo (o `app-hub-db` e o `sistema-alis-db` do Henry ficam intocados).
- Padrões: memória 512 MB, 1 processador, dormir após 30 min sem acesso, saúde em `/`, vigia a cada 30 s, até 3 tentativas de religar em 10 min, cópia diária às 3h guardada 14 dias, cópias `pre_deploy`/`pre_restore` guardadas 30 dias, uso medido a cada minuto e guardado 90 dias, caixa de teste desligada em 30 min, código de entrada válido 60 s, cookie do app 12 h, acesso reconferido a cada 5 min.
- Status do app compartilhado, nomes exatos: `draft`, `starting`, `running`, `sleeping`, `crashed`, `stopped`. Status da implantação: `building`, `failed`, `ready`, `live`, `retired`. Tipos de cópia: `daily`, `pre_deploy`, `pre_restore`, `manual`.
- Permissão nova de cargo: `manage_server` ("Cuidar do servidor"), dada ao `admin` na migração e no `resetSQL` dos testes.
- Quem publica/volta/configura/restaura/para = `can.Restore` do app. Quem vê mensagens e uso = `can.Tech`. Quem abre = `can.View`. Servidor e gastos = `manage_server`.
- Cabeçalhos para o app: `X-AppHub-User-Id`, `X-AppHub-User-Email`, `X-AppHub-User-Name`, `X-AppHub-User-Role`; qualquer `X-AppHub-*` vindo do navegador é apagado.
- Variáveis que o Hub injeta no app: `PORT`, `BASE_URL`, `APPHUB_DATA_DIR=/data`, e `DATABASE_URL` se o app pede banco. Valores de senha nunca voltam em resposta da API.
- Estilo: seguir `DESIGN.md` (violeta só no que se pode iniciar — Abrir, Testar, Publicar; "Editar" discreto; claro/escuro; celular com 16 px de margem e sem rolagem lateral).

## Review Focus

1. **Dois cliques em "Publicar versão nova" (duas abas, ou duas pessoas)** → só uma implantação roda por app de cada vez; a segunda recebe 409 "Já existe uma publicação em andamento para este app." Teste no Task 10 (`TestDeployOneAtATime`).
2. **App que liga mas nunca responde na porta** (porta errada, app que trava na subida) → a saúde desiste em 90 s, a implantação vira `failed` com a mensagem "O app ligou, mas não respondeu na porta N" e, numa versão nova, a anterior continua no ar. Teste no Task 10 (`TestNewVersionUnhealthyKeepsOld`).
3. **Pessoa perde o acesso com o app aberto** (app restrito depois, ou pessoa removida) → em até 5 min o próximo pedido recebe a página "Sem acesso", sem esperar o cookie de 12 h. Teste no Task 11 (`TestGatewayRechecksAccess`).
4. **Navegador manda `X-AppHub-User-Email` forjado** → o app recebe só o cabeçalho do Hub. Teste no Task 11 (`TestGatewayStripsForgedHeaders`).
5. **O Hub reinicia com apps no ar** (queda de luz, atualização) → ao ligar, o vigia religa os que deviam estar no ar, remove contêineres órfãos com o rótulo do Hub e não toca nos demais. Teste no Task 12 (`TestReconcileOnStart`).

---

## File Structure

**Backend — novo**
- `backend/internal/database/migrations/008_shared.sql` — tabelas `shared_apps`, `shared_deployments`, `app_config`, `shared_backups`, `shared_usage`, `server_settings`; `manage_server`; `app_changes.person_id` opcional.
- `backend/internal/database/queries/shared.sql` — consultas das tabelas novas.
- `backend/internal/vault/vault.go` + `vault_test.go` — cifra/decifra AES-256-GCM.
- `backend/internal/shared/engine/engine.go` — interface `Engine`, tipos.
- `backend/internal/shared/engine/fake.go` + `fake_test.go` — motor de mentira.
- `backend/internal/shared/engine/podman.go` + `podman_test.go` — motor Podman (teste real com `-tags podman`).
- `backend/internal/shared/appdb/appdb.go` + `appdb_test.go` — banco e usuário por app, cópia e restauração.
- `backend/internal/shared/recipes/recipes.go` + `recipes_test.go` — receitas automáticas.
- `backend/internal/shared/readiness/readiness.go` + `readiness_test.go` — checagem.
- `backend/internal/shared/stage/stage.go` + `stage_test.go` — copia a pasta para a área de montagem.
- `backend/internal/shared/deploy/deploy.go`, `store.go` + `deploy_test.go` — testar, publicar, versão nova, voltar, parar, religar, deixar de compartilhar.
- `backend/internal/shared/health/health.go` + test — espera a saúde responder.
- `backend/internal/shared/gateway/gateway.go`, `pages.go`, `pages/*.html` + `gateway_test.go` — porteiro.
- `backend/internal/shared/supervisor/supervisor.go` + `supervisor_test.go` — vigia.
- `backend/internal/shared/costs/costs.go` + `costs_test.go` — custo estimado.
- `backend/internal/handler/shared.go`, `shared_config.go`, `shared_backups.go`, `server.go` + testes — API.
- `backend/testdata/shared/` — pastas de exemplo (streamlit, flask, node, dockerfile, sqlite, windows-path).

**Backend — modificado**
- `backend/internal/manifest/manifest.go` + test — bloco `shared`, validação.
- `backend/internal/access/access.go` — `ManageServer`.
- `backend/internal/config/config.go` — variáveis novas.
- `backend/internal/handler/handler.go` — rotas, `Handler` ganha `shared *deploy.Service` e `vault`.
- `backend/internal/handler/apps.go` — `appSummary`/`appDetail` com `shared`.
- `backend/internal/handler/history.go` + `queries/versions.sql` — eventos sem pessoa ("Hub").
- `backend/internal/handler/handler_test.go` — `resetSQL` com tabelas novas e `manage_server`.
- `backend/cmd/server/main.go` — roteamento por `Host`, vigia, porteiro.
- `.env.example`, `docs/INFRASTRUCTURE.md`.

**Frontend**
- `src/lib/api.ts` — tipos e chamadas novas.
- `src/lib/permissions.ts` — `manage_server`.
- `src/lib/shared.ts` + test — rótulos de status, formatação de memória e dinheiro.
- `src/components/SharedStatus.tsx` — pontinho + texto.
- `src/pages/CatalogPage.tsx` + test — Abrir e status.
- `src/components/ficha/SharedSection.tsx` + test — parte "Compartilhado".
- `src/components/ficha/SharedLine.tsx` — linha "No ar em … · Abrir" da parte de uso.
- `src/pages/AppPage.tsx` + test — encaixe das duas peças.
- `src/pages/ServerPage.tsx` + test — Servidor e gastos.
- `src/components/Layout.tsx`, `src/App.tsx` — item e rota `/servidor`.
- `src/test/fixtures.ts` — fixtures novas.
- `e2e/compartilhados.mjs` — fotos.

**Docs**
- `docs/adr/010-apps-compartilhados-no-servidor-do-hub.md`, `docs/TASKS.md`, `docs/PRD.md`, `docs/INFRASTRUCTURE.md`, `DESIGN.md` (se surgir componente novo).

---

# Fase 1 — Fundações

### Task 1: Migração 008, consultas e a permissão `manage_server`

**Files:**
- Create: `backend/internal/database/migrations/008_shared.sql`
- Create: `backend/internal/database/queries/shared.sql`
- Modify: `backend/internal/database/queries/versions.sql` (`ListChangesBefore`)
- Modify: `backend/internal/access/access.go`
- Modify: `backend/internal/handler/handler_test.go` (`resetSQL`)
- Modify: `backend/internal/handler/history.go` (nome "Hub" quando não há pessoa)
- Modify: `frontend/src/lib/permissions.ts`
- Test: `backend/internal/handler/shared_migration_test.go`

**Interfaces:**
- Produces: `access.ManageServer = "manage_server"` (em `RolePerms`); tabelas e consultas sqlc listadas abaixo; `app_changes.person_id` passa a aceitar NULL (eventos do Hub).

- [ ] **Step 1: Escrever a migração**

```sql
-- Apps compartilhados: rodam uma vez só no servidor do Hub (desenho 2026-10-06).

UPDATE roles SET permissions = array_append(permissions, 'manage_server')
WHERE key = 'admin' AND NOT ('manage_server' = ANY(permissions));

-- Eventos do Hub (o vigia marcou "caiu") não têm pessoa.
ALTER TABLE app_changes ALTER COLUMN person_id DROP NOT NULL;

CREATE TABLE IF NOT EXISTS shared_apps (
    app_id                 BIGINT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    slug                   TEXT NOT NULL UNIQUE,
    status                 TEXT NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft','starting','running','sleeping','crashed','stopped')),
    always_on              BOOLEAN NOT NULL DEFAULT false,
    idle_minutes           INT NOT NULL DEFAULT 30 CHECK (idle_minutes BETWEEN 5 AND 1440),
    memory_mb              INT NOT NULL DEFAULT 512 CHECK (memory_mb BETWEEN 128 AND 16384),
    cpus                   NUMERIC(4,2) NOT NULL DEFAULT 1 CHECK (cpus > 0 AND cpus <= 16),
    health_path            TEXT NOT NULL DEFAULT '/',
    port                   INT NOT NULL DEFAULT 0,
    wants_db               BOOLEAN NOT NULL DEFAULT false,
    db_name                TEXT NOT NULL DEFAULT '',
    current_deployment_id  BIGINT,
    previous_deployment_id BIGINT,
    restart_count          INT NOT NULL DEFAULT 0,
    restart_window_start   TIMESTAMPTZ,
    last_access_at         TIMESTAMPTZ,
    status_changed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    status_message         TEXT NOT NULL DEFAULT '',
    acks                   TEXT[] NOT NULL DEFAULT '{}', -- avisos da checagem marcados "estou ciente"
    stopped_by             BIGINT REFERENCES people(id),
    unshared_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS shared_deployments (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version_id    BIGINT REFERENCES app_versions(id),
    kind          TEXT NOT NULL CHECK (kind IN ('test','live')),
    recipe        TEXT NOT NULL DEFAULT '',
    image_id      TEXT NOT NULL DEFAULT '',
    container_id  TEXT NOT NULL DEFAULT '',
    test_db_name  TEXT NOT NULL DEFAULT '',
    port          INT NOT NULL DEFAULT 0, -- porta dentro da caixa desta versão
    status        TEXT NOT NULL DEFAULT 'building'
                  CHECK (status IN ('building','failed','ready','live','retired')),
    build_log     TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ,
    created_by    BIGINT REFERENCES people(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS shared_deployments_app_idx ON shared_deployments(app_id, id DESC);
-- Uma publicação em andamento por app.
CREATE UNIQUE INDEX IF NOT EXISTS shared_deployments_one_building
    ON shared_deployments(app_id) WHERE status = 'building';

ALTER TABLE shared_apps
    ADD CONSTRAINT shared_apps_current_fk  FOREIGN KEY (current_deployment_id)  REFERENCES shared_deployments(id),
    ADD CONSTRAINT shared_apps_previous_fk FOREIGN KEY (previous_deployment_id) REFERENCES shared_deployments(id);

CREATE TABLE IF NOT EXISTS app_config (
    app_id       BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    is_secret    BOOLEAN NOT NULL DEFAULT false,
    value_plain  TEXT NOT NULL DEFAULT '',
    value_cipher BYTEA,
    updated_by   BIGINT REFERENCES people(id),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (app_id, name)
);

CREATE TABLE IF NOT EXISTS shared_backups (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('daily','pre_deploy','pre_restore','manual')),
    path          TEXT NOT NULL,
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    deployment_id BIGINT REFERENCES shared_deployments(id),
    created_by    BIGINT REFERENCES people(id),
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS shared_backups_app_idx ON shared_backups(app_id, created_at DESC);

CREATE TABLE IF NOT EXISTS shared_usage (
    app_id    BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    at        TIMESTAMPTZ NOT NULL,
    mem_bytes BIGINT NOT NULL,
    cpu_pct   REAL NOT NULL,
    PRIMARY KEY (app_id, at)
);

CREATE TABLE IF NOT EXISTS server_settings (
    id                 INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    monthly_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (monthly_cost_cents >= 0),
    updated_by         BIGINT REFERENCES people(id),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO server_settings (id) VALUES (1) ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Escrever as consultas** em `queries/shared.sql`

```sql
-- name: GetSharedApp :one
SELECT * FROM shared_apps WHERE app_id = $1;

-- name: GetSharedAppBySlug :one
SELECT * FROM shared_apps WHERE slug = $1 AND unshared_at IS NULL;

-- name: ListSharedApps :many
SELECT s.*, a.name AS app_name, a.owner_id, a.backup_id
FROM shared_apps s JOIN apps a ON a.id = s.app_id
WHERE s.unshared_at IS NULL
ORDER BY a.name;

-- name: UpsertSharedApp :one
INSERT INTO shared_apps (app_id, slug, always_on, idle_minutes, memory_mb, cpus, health_path, port, wants_db)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (app_id) DO UPDATE SET
    always_on = EXCLUDED.always_on, idle_minutes = EXCLUDED.idle_minutes, memory_mb = EXCLUDED.memory_mb,
    cpus = EXCLUDED.cpus, health_path = EXCLUDED.health_path, port = EXCLUDED.port, wants_db = EXCLUDED.wants_db,
    unshared_at = NULL
RETURNING *;

-- name: SetSharedStatus :exec
UPDATE shared_apps
SET status = sqlc.arg(status), status_message = sqlc.arg(status_message), status_changed_at = now(),
    stopped_by = sqlc.narg(stopped_by)
WHERE app_id = sqlc.arg(app_id);

-- name: SetSharedDeployments :exec
UPDATE shared_apps SET current_deployment_id = sqlc.narg(current_id), previous_deployment_id = sqlc.narg(previous_id)
WHERE app_id = sqlc.arg(app_id);

-- name: SetSharedAcks :exec
UPDATE shared_apps SET acks = $2 WHERE app_id = $1;

-- name: SetSharedDBName :exec
UPDATE shared_apps SET db_name = $2 WHERE app_id = $1;

-- name: SetSharedRestarts :exec
UPDATE shared_apps SET restart_count = $2, restart_window_start = $3 WHERE app_id = $1;

-- name: TouchSharedAccess :exec
UPDATE shared_apps SET last_access_at = $2 WHERE app_id = $1 AND (last_access_at IS NULL OR last_access_at < $2);

-- name: SetSharedAccessNow :exec
UPDATE shared_apps SET last_access_at = now() WHERE app_id = $1;

-- name: MarkUnshared :exec
UPDATE shared_apps SET unshared_at = now(), status = 'stopped', current_deployment_id = NULL, previous_deployment_id = NULL
WHERE app_id = $1;

-- name: ListUnsharedBefore :many
SELECT * FROM shared_apps WHERE unshared_at IS NOT NULL AND unshared_at < $1;

-- name: DeleteSharedApp :exec
DELETE FROM shared_apps WHERE app_id = $1;

-- name: InsertDeployment :one
INSERT INTO shared_deployments (app_id, version_id, kind, recipe, test_db_name, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetDeployment :one
SELECT * FROM shared_deployments WHERE id = $1;

-- name: FinishDeployment :exec
UPDATE shared_deployments
SET status = sqlc.arg(status), image_id = sqlc.arg(image_id), container_id = sqlc.arg(container_id),
    port = sqlc.arg(port), build_log = sqlc.arg(build_log), error_message = sqlc.arg(error_message), finished_at = now()
WHERE id = sqlc.arg(id);

-- name: SetDeploymentStatus :exec
UPDATE shared_deployments SET status = $2 WHERE id = $1;

-- name: SetDeploymentContainer :exec
UPDATE shared_deployments SET container_id = $2 WHERE id = $1;

-- name: SetDeploymentTestDB :exec
UPDATE shared_deployments SET test_db_name = $2 WHERE id = $1;

-- name: ListAllOpenTests :many
SELECT * FROM shared_deployments WHERE kind = 'test' AND status IN ('building','ready');

-- name: ListOpenTests :many
SELECT * FROM shared_deployments WHERE app_id = $1 AND kind = 'test' AND status IN ('building','ready');

-- name: ListDeployments :many
SELECT d.*, v.number AS version_number, p.name AS created_by_name
FROM shared_deployments d
LEFT JOIN app_versions v ON v.id = d.version_id
LEFT JOIN people p ON p.id = d.created_by
WHERE d.app_id = $1 ORDER BY d.id DESC LIMIT $2;

-- name: ListExpiredTests :many
SELECT * FROM shared_deployments
WHERE kind = 'test' AND status IN ('ready','building') AND expires_at < $1;

-- name: ListLabelledContainersExpected :many
-- Containers the Hub expects to exist: live deployments of shared apps that are not unshared.
SELECT d.container_id FROM shared_deployments d
JOIN shared_apps s ON s.current_deployment_id = d.id
WHERE s.unshared_at IS NULL AND d.container_id <> ''
UNION
SELECT container_id FROM shared_deployments WHERE kind = 'test' AND status = 'ready' AND container_id <> '';

-- name: ListAppConfig :many
SELECT * FROM app_config WHERE app_id = $1 ORDER BY name;

-- name: UpsertAppConfig :exec
INSERT INTO app_config (app_id, name, is_secret, value_plain, value_cipher, updated_by, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (app_id, name) DO UPDATE SET
    is_secret = EXCLUDED.is_secret, value_plain = EXCLUDED.value_plain, value_cipher = EXCLUDED.value_cipher,
    updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: DeleteAppConfig :exec
DELETE FROM app_config WHERE app_id = $1 AND name = $2;

-- name: InsertBackup :one
INSERT INTO shared_backups (app_id, kind, path, size_bytes, deployment_id, created_by, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetBackup :one
SELECT * FROM shared_backups WHERE id = $1 AND app_id = $2;

-- name: ListBackups :many
SELECT b.*, p.name AS created_by_name, v.number AS version_number
FROM shared_backups b
LEFT JOIN people p ON p.id = b.created_by
LEFT JOIN shared_deployments d ON d.id = b.deployment_id
LEFT JOIN app_versions v ON v.id = d.version_id
WHERE b.app_id = $1 ORDER BY b.created_at DESC;

-- name: LatestPreDeployBackup :one
SELECT * FROM shared_backups WHERE app_id = $1 AND kind = 'pre_deploy' AND deployment_id = $2
ORDER BY created_at DESC LIMIT 1;

-- name: LastDailyBackup :one
SELECT * FROM shared_backups WHERE app_id = $1 AND kind = 'daily' ORDER BY created_at DESC LIMIT 1;

-- name: ListExpiredBackups :many
SELECT * FROM shared_backups WHERE expires_at < $1;

-- name: DeleteBackup :exec
DELETE FROM shared_backups WHERE id = $1;

-- name: DeleteBackupsOfApp :exec
DELETE FROM shared_backups WHERE app_id = $1;

-- name: SumBackupBytes :many
SELECT app_id, COALESCE(SUM(size_bytes), 0)::bigint AS total FROM shared_backups GROUP BY app_id;

-- name: InsertUsage :exec
INSERT INTO shared_usage (app_id, at, mem_bytes, cpu_pct) VALUES ($1, $2, $3, $4)
ON CONFLICT (app_id, at) DO NOTHING;

-- name: UsageSince :many
SELECT * FROM shared_usage WHERE app_id = $1 AND at >= $2 ORDER BY at;

-- name: UsageTotalsSince :many
-- Minutes on, average and peak memory, average CPU per app since a moment.
SELECT app_id, COUNT(*)::bigint AS minutes, COALESCE(AVG(mem_bytes), 0)::bigint AS avg_mem,
       COALESCE(MAX(mem_bytes), 0)::bigint AS max_mem, COALESCE(AVG(cpu_pct), 0)::real AS avg_cpu
FROM shared_usage WHERE at >= $1 GROUP BY app_id;

-- name: DeleteUsageBefore :exec
DELETE FROM shared_usage WHERE at < $1;

-- name: GetServerSettings :one
SELECT * FROM server_settings WHERE id = 1;

-- name: SetServerCost :exec
UPDATE server_settings SET monthly_cost_cents = $1, updated_by = $2, updated_at = now() WHERE id = 1;

-- name: InsertHubChange :exec
-- An event in the app's timeline without a person (the Hub itself).
INSERT INTO app_changes (app_id, person_id, changes) VALUES ($1, NULL, $2);
```

Em `queries/versions.sql`, acrescentar `-- name: GetAppVersionByID :one` / `SELECT * FROM app_versions WHERE id = $1;` e trocar `ListChangesBefore` por `LEFT JOIN` e nome padrão:

```sql
-- name: ListChangesBefore :many
SELECT c.changes, c.created_at, COALESCE(p.name, 'Hub')::text AS person_name
FROM app_changes c LEFT JOIN people p ON p.id = c.person_id
WHERE c.app_id = sqlc.arg(app_id) AND c.created_at < sqlc.arg(before)
ORDER BY c.created_at DESC
LIMIT sqlc.arg(lim);
```

- [ ] **Step 3: Gerar o código** — `cd backend && mise x -- sqlc generate`. Ler `internal/database/sqlc/shared.sql.go` e `models.go` e anotar os nomes reais (ex.: `SharedApp`, `SharedDeployment`, `Cpus pgtype.Numeric`). Se `history.go` deixar de compilar porque `PersonName` mudou de tipo, ajustar o uso.

- [ ] **Step 4: Acrescentar a permissão** em `access/access.go`:

```go
	ManageRoles   = "manage_roles"
	ManageServer  = "manage_server"
)

var RolePerms = []string{RegisterApps, EditAnyApp, RestoreAnyApp, SeeTechAnyApp, SeeAllRuns, SeeAllApps, ManagePeople, ManageAreas, ManageRoles, ManageServer}
```

E em `frontend/src/lib/permissions.ts`, no fim de `PERMISSIONS`:

```ts
  { key: "manage_server", label: "Cuidar do servidor", hint: "Vê gastos e uso do servidor, muda limites e para qualquer app compartilhado" },
```

mais `export const canSeeServer = (p: Person | null | undefined) => has(p, "manage_server")`.

- [ ] **Step 5: Atualizar `resetSQL`** em `handler_test.go`: acrescentar `shared_usage, shared_backups, app_config, shared_apps, shared_deployments` no `TRUNCATE` (antes de `apps`), `'manage_server'` no array do `admin`, e `UPDATE server_settings SET monthly_cost_cents = 0;`.

- [ ] **Step 6: Teste da migração** (`shared_migration_test.go`)

```go
package handler

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestAdminHasManageServer(t *testing.T) {
	c := newClient(t, false)
	p := setupAdmin(t, c)
	if !slices.Contains(p.Permissions, "manage_server") {
		t.Fatalf("admin permissions %v", p.Permissions)
	}
}

func TestHubChangeShowsAsHub(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	id := insertApp(t, filesPrepManifest()) // helpers existentes em runs_test.go
	if err := h.q.InsertHubChange(context.Background(), sqlcInsertHubChange(id, "Caiu")); err != nil {
		t.Fatal(err)
	}
	var hist struct {
		Items []struct{ Person string `json:"person_name"` } `json:"items"`
	}
	if code := c.do("GET", "/api/apps/"+itoa(id)+"/history?type=ficha", nil, &hist); code != http.StatusOK {
		t.Fatalf("history %d", code)
	}
	if len(hist.Items) == 0 || hist.Items[0].Person != "Hub" {
		t.Fatalf("history %+v", hist.Items)
	}
}
```

`sqlcInsertHubChange` monta os parâmetros gerados (`sqlc.InsertHubChangeParams{AppID: id, Changes: json([]manifest.Change{{Field: "shared", Label: label}})}`); escreva-o no mesmo arquivo depois de ler a assinatura gerada. `insertApp(t, manifest)` fica em `runs_test.go` e `itoa` em `team_test.go`; `newPoolClient` devolve o cliente e o `*Handler` (confira a assinatura). Se o JSON de histórico chamar o campo de outro jeito, confira em `history.go` e ajuste o teste.

- [ ] **Step 7: Rodar** — `cd backend && set -a && . ../.env && set +a && mise x -- go test ./...` → PASS (os testes antigos continuam passando).

- [ ] **Step 8: Commit**

```bash
git add backend/internal/database backend/internal/access backend/internal/handler frontend/src/lib/permissions.ts
git commit -m "Shared apps: migration 008, queries and the manage_server permission"
```

---

### Task 2: Manifesto v0.3 com o bloco `shared`

**Files:**
- Modify: `backend/internal/manifest/manifest.go`, `backend/internal/manifest/diff.go`
- Test: `backend/internal/manifest/manifest_test.go`
- Modify: `frontend/src/lib/api.ts` (tipo `Manifest`)

**Interfaces:**
- Produces:

```go
type Shared struct {
	Enabled    bool        `yaml:"enabled" json:"enabled"`
	Port       int         `yaml:"port,omitempty" json:"port"`
	HealthPath string      `yaml:"health_path,omitempty" json:"health_path"`
	Database   string      `yaml:"database,omitempty" json:"database"` // "" ou "postgres"
	AlwaysOn   bool        `yaml:"always_on,omitempty" json:"always_on"`
	IdleMin    int         `yaml:"idle_minutes,omitempty" json:"idle_minutes"`
	MemoryMB   int         `yaml:"memory_mb,omitempty" json:"memory_mb"`
	CPUs       float64     `yaml:"cpus,omitempty" json:"cpus"`
	Python     string      `yaml:"python,omitempty" json:"python"`   // versão para a receita automática; padrão 3.12
	Node       string      `yaml:"node,omitempty" json:"node"`       // padrão 22
	Command    string      `yaml:"command,omitempty" json:"command"` // comando de início confirmado (Python web)
	Env        []EnvVar    `yaml:"env,omitempty" json:"env"`
}

type EnvVar struct {
	Name     string `yaml:"name" json:"name"`
	Label    string `yaml:"label" json:"label"`
	Secret   bool   `yaml:"secret,omitempty" json:"secret"`
	Required bool   `yaml:"required,omitempty" json:"required"`
	Default  string `yaml:"default,omitempty" json:"default"`
}

// Manifest ganha: Shared *Shared `yaml:"shared,omitempty" json:"shared"`
func (s *Shared) WithDefaults() Shared // copia com padrões aplicados
var ReservedEnv = []string{"PORT", "BASE_URL", "APPHUB_DATA_DIR", "DATABASE_URL"}
```

- [ ] **Step 1: Teste que falha**

```go
func TestSharedValidation(t *testing.T) {
	base := func() Manifest {
		return Manifest{ManifestVersion: 1, ID: "x", Name: "X", Kind: KindWeb, Owner: &PersonRef{Name: "Henry", Email: "h@a.com"}, Area: "Research",
			Shared: &Shared{Enabled: true}}
	}
	cases := []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{"ok", func(*Manifest) {}, ""},
		{"only web", func(m *Manifest) { m.Kind = KindScript }, "Só apps com tela no navegador podem ser compartilhados."},
		{"bad db", func(m *Manifest) { m.Shared.Database = "mysql" }, "O banco do app compartilhado precisa ser postgres."},
		{"bad port", func(m *Manifest) { m.Shared.Port = 70000 }, "A porta do app precisa estar entre 1 e 65535."},
		{"memory", func(m *Manifest) { m.Shared.MemoryMB = 64 }, "A memória do app precisa estar entre 128 e 16384 MB."},
		{"reserved env", func(m *Manifest) { m.Shared.Env = []EnvVar{{Name: "DATABASE_URL", Label: "x"}} }, "DATABASE_URL é preenchida pelo Hub; tire da lista de configurações."},
		{"bad env name", func(m *Manifest) { m.Shared.Env = []EnvVar{{Name: "minha var", Label: "x"}} }, "O nome da configuração \"minha var\" só pode ter letras maiúsculas, números e _."},
		{"dup env", func(m *Manifest) { m.Shared.Env = []EnvVar{{Name: "A", Label: "a"}, {Name: "A", Label: "b"}} }, "A configuração A aparece duas vezes."},
		{"health", func(m *Manifest) { m.Shared.HealthPath = "health" }, "O endereço de saúde precisa começar com /."},
	}
	for _, c := range cases {
		m := base()
		c.edit(&m)
		got := strings.Join(m.Validate(), " | ")
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestSharedDefaults(t *testing.T) {
	d := (&Shared{Enabled: true}).WithDefaults()
	if d.HealthPath != "/" || d.IdleMin != 30 || d.MemoryMB != 512 || d.CPUs != 1 || d.Python != "3.12" || d.Node != "22" {
		t.Fatalf("%+v", d)
	}
}

func TestSharedRoundTripYAML(t *testing.T) {
	m := Manifest{ManifestVersion: 1, ID: "x", Name: "X", Kind: KindWeb, Shared: &Shared{Enabled: true, Database: "postgres",
		Env: []EnvVar{{Name: "SMTP_PASSWORD", Label: "Senha do e-mail", Secret: true}}}}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "secret: true") || strings.Contains(string(out), "value") {
		t.Fatalf("yaml:\n%s", out)
	}
}
```

Ajuste `base()` para passar no `Validate` atual (ler o `Validate` para saber os campos obrigatórios; se exigir `entrypoint` para `web`, preencha).

- [ ] **Step 2: Rodar** — `cd backend && mise x -- go test ./internal/manifest/ -run Shared` → FAIL (`Shared` indefinido).

- [ ] **Step 3: Implementar** — acrescentar os tipos acima ao `manifest.go`, o campo `Shared` em `Manifest`, e no `Validate`:

```go
var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func (s *Shared) WithDefaults() Shared {
	d := *s
	if d.HealthPath == "" {
		d.HealthPath = "/"
	}
	if d.IdleMin == 0 {
		d.IdleMin = 30
	}
	if d.MemoryMB == 0 {
		d.MemoryMB = 512
	}
	if d.CPUs == 0 {
		d.CPUs = 1
	}
	if d.Python == "" {
		d.Python = "3.12"
	}
	if d.Node == "" {
		d.Node = "22"
	}
	return d
}

func (m Manifest) sharedProblems() []string {
	s := m.Shared
	if s == nil || !s.Enabled {
		return nil
	}
	var out []string
	if m.Kind != KindWeb {
		out = append(out, "Só apps com tela no navegador podem ser compartilhados.")
	}
	if s.Database != "" && s.Database != "postgres" {
		out = append(out, "O banco do app compartilhado precisa ser postgres.")
	}
	if s.Port < 0 || s.Port > 65535 {
		out = append(out, "A porta do app precisa estar entre 1 e 65535.")
	}
	if s.MemoryMB != 0 && (s.MemoryMB < 128 || s.MemoryMB > 16384) {
		out = append(out, "A memória do app precisa estar entre 128 e 16384 MB.")
	}
	if s.CPUs < 0 || s.CPUs > 16 {
		out = append(out, "O processamento do app precisa estar entre 0,1 e 16.")
	}
	if s.IdleMin != 0 && (s.IdleMin < 5 || s.IdleMin > 1440) {
		out = append(out, "O tempo para dormir precisa estar entre 5 e 1440 minutos.")
	}
	if s.HealthPath != "" && !strings.HasPrefix(s.HealthPath, "/") {
		out = append(out, "O endereço de saúde precisa começar com /.")
	}
	seen := map[string]bool{}
	for _, e := range s.Env {
		switch {
		case slices.Contains(ReservedEnv, e.Name):
			out = append(out, e.Name+" é preenchida pelo Hub; tire da lista de configurações.")
		case !envName.MatchString(e.Name):
			out = append(out, fmt.Sprintf("O nome da configuração %q só pode ter letras maiúsculas, números e _.", e.Name))
		case seen[e.Name]:
			out = append(out, "A configuração "+e.Name+" aparece duas vezes.")
		}
		seen[e.Name] = true
	}
	return out
}
```

Chamar `out = append(out, m.sharedProblems()...)` no fim do `Validate`. Em `Diff`, acrescentar `add("shared", "Compartilhado", jsonText(a.Shared), jsonText(b.Shared), true)`. Em `cmd/validate-manifest` nada muda (usa `Validate`).

- [ ] **Step 4: Tipo no frontend** — em `api.ts`:

```ts
export type EnvVar = { name: string; label: string; secret?: boolean; required?: boolean; default?: string }
export type SharedSpec = {
  enabled: boolean; port?: number; health_path?: string; database?: "" | "postgres"
  always_on?: boolean; idle_minutes?: number; memory_mb?: number; cpus?: number
  python?: string; node?: string; command?: string; env?: EnvVar[] | null
}
```

e `shared?: SharedSpec | null` em `Manifest`.

- [ ] **Step 5: Rodar** — `go test ./internal/manifest/` e `cd frontend && mise x -- npx tsc -b` → PASS.

- [ ] **Step 6: Commit** — `git commit -am "Manifest v0.3: shared block with validation"`

---

### Task 3: Cofre (`internal/vault`) e configuração

**Files:**
- Create: `backend/internal/vault/vault.go`, `backend/internal/vault/vault_test.go`
- Modify: `backend/internal/config/config.go`, `.env.example`

**Interfaces:**
- Produces:

```go
package vault
type Vault struct{ /* aead cipher.AEAD */ }
var ErrNoKey = errors.New("cofre sem chave")
func New(keyB64 string) (*Vault, error)        // "" → (nil, ErrNoKey); tamanho errado → erro
func (v *Vault) Seal(plain string) ([]byte, error)  // nonce(12) || ciphertext
func (v *Vault) Open(sealed []byte) (string, error)
func NewKey() string                              // 32 bytes aleatórios em base64, para o .env
```

e em `config.Config`: `SecretKey`, `AppsDBAdminURL`, `AppsDBHost` (para contêineres), `AppsDBContainer`, `SharedDomain`, `SharedDataDir`, `ContainerEngine`.

- [ ] **Step 1: Teste que falha**

```go
package vault

import (
	"strings"
	"testing"
)

func TestSealOpen(t *testing.T) {
	v, err := New(NewKey())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := v.Seal("senha-123")
	b, _ := v.Seal("senha-123")
	if string(a) == string(b) {
		t.Fatal("same nonce twice")
	}
	if strings.Contains(string(a), "senha-123") {
		t.Fatal("plain text inside")
	}
	got, err := v.Open(a)
	if err != nil || got != "senha-123" {
		t.Fatalf("open %q %v", got, err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	v1, _ := New(NewKey())
	v2, _ := New(NewKey())
	s, _ := v1.Seal("x")
	if _, err := v2.Open(s); err == nil {
		t.Fatal("opened with another key")
	}
}

func TestNoKey(t *testing.T) {
	if _, err := New(""); err != ErrNoKey {
		t.Fatalf("err %v", err)
	}
	if _, err := New("curta"); err == nil {
		t.Fatal("accepted short key")
	}
}

func TestTamperedFails(t *testing.T) {
	v, _ := New(NewKey())
	s, _ := v.Seal("x")
	s[len(s)-1] ^= 1
	if _, err := v.Open(s); err == nil {
		t.Fatal("opened tampered")
	}
}
```

- [ ] **Step 2: Rodar** → FAIL.

- [ ] **Step 3: Implementar**

```go
// Package vault keeps app settings that are secrets: AES-256-GCM with the
// key in HUB_SECRET_KEY. Values never leave the Hub except into the app's
// environment when it starts.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

var ErrNoKey = errors.New("cofre sem chave")

type Vault struct{ aead cipher.AEAD }

func New(keyB64 string) (*Vault, error) {
	if keyB64 == "" {
		return nil, ErrNoKey
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(key) != 32 {
		return nil, errors.New("HUB_SECRET_KEY precisa ter 32 bytes em base64")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

func (v *Vault) Seal(plain string) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return v.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (v *Vault) Open(sealed []byte) (string, error) {
	n := v.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("valor guardado inválido")
	}
	out, err := v.aead.Open(nil, sealed[:n], sealed[n:], nil)
	return string(out), err
}

func NewKey() string {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	return base64.StdEncoding.EncodeToString(k)
}
```

- [ ] **Step 4: Configuração** — em `config.go`, acrescentar campos e leitura com padrões:

```go
	SecretKey       string // HUB_SECRET_KEY
	AppsDBAdminURL  string // APPS_DB_ADMIN_URL; padrão: DATABASE_URL
	AppsDBHost      string // APPS_DB_HOST_FOR_CONTAINERS; padrão host.containers.internal:5433
	AppsDBContainer string // APPS_DB_CONTAINER; padrão app-hub-db (cópias via exec)
	SharedDomain    string // SHARED_APPS_DOMAIN; padrão localhost:8080
	SharedDataDir   string // SHARED_DATA_DIR; padrão ~/.local/share/app-hub/shared
	ContainerEngine string // CONTAINER_ENGINE; padrão podman
```

com uma função `envOr(name, def string) string`. `SharedDataDir` usa `os.UserHomeDir()`. Acrescentar ao `.env.example` (sem valores reais) com um comentário: `# gerar com: cd backend && mise x -- go run ./cmd/newkey`. Criar `backend/cmd/newkey/main.go` que imprime `vault.NewKey()`.

- [ ] **Step 5: Rodar** — `go test ./internal/vault/ ./internal/config/` → PASS.

- [ ] **Step 6: Commit** — `git add backend/internal/vault backend/internal/config backend/cmd/newkey .env.example && git commit -m "Vault for app secrets and shared-apps settings"`

---

# Fase 2 — Motor e banco por app

### Task 4: Interface do motor e motor de mentira

**Files:**
- Create: `backend/internal/shared/engine/engine.go`
- Create: `backend/internal/shared/engine/fake.go`, `backend/internal/shared/engine/fake_test.go`

**Interfaces:**
- Produces (usado por appdb, deploy, supervisor, gateway, handler):

```go
package engine

// Label marks every container and image the Hub creates. The Hub never
// touches anything without it.
const Label = "app-hub.app"
const Network = "app-hub-apps"

var ErrUnavailable = errors.New("o servidor de apps não está disponível")
var ErrNotFound = errors.New("caixa não encontrada")

type BuildSpec struct {
	ContextDir     string            // área de montagem (cópia da pasta)
	DockerfilePath string            // caminho dentro de ContextDir; "" = "Dockerfile"
	Tag            string            // localhost/apphub/<slug>:<deployment_id>
	Labels         map[string]string // sempre inclui Label
}

type StartSpec struct {
	Name     string            // apphub-<slug>-<deployment_id>
	Image    string
	Labels   map[string]string
	Env      map[string]string // nunca vai para a linha de comando (arquivo 0600 apagado em seguida)
	Port     int               // porta dentro da caixa
	MemoryMB int
	CPUs     float64
	DataDir  string // pasta no servidor montada em /data
}

type State struct {
	Exists   bool
	Running  bool
	ExitCode int
	HostPort int // porta em 127.0.0.1 que leva à porta da caixa
}

type Usage struct {
	ContainerID string
	MemBytes    int64
	CPUPct      float64
}

type Container struct {
	ID      string
	Name    string
	Running bool
	Labels  map[string]string
}

type Engine interface {
	Available(ctx context.Context) error
	Build(ctx context.Context, b BuildSpec) (imageID string, log string, err error)
	Start(ctx context.Context, s StartSpec) (containerID string, err error)
	Stop(ctx context.Context, containerID string) error
	Remove(ctx context.Context, containerID string) error // para e remove; sem erro se não existe
	RemoveImage(ctx context.Context, imageID string) error
	Inspect(ctx context.Context, containerID string) (State, error)
	Stats(ctx context.Context, containerIDs []string) ([]Usage, error)
	Logs(ctx context.Context, containerID string, lines int) (string, error)
	Exec(ctx context.Context, container string, stdin io.Reader, stdout io.Writer, cmd ...string) error
	List(ctx context.Context) ([]Container, error) // só as que têm Label
}
```

- Produces (motor de mentira, só para testes, mas exportado para outros pacotes):

```go
type Fake struct {
	mu         sync.Mutex
	Images     map[string]BuildSpec
	Containers map[string]*FakeContainer
	BuildErr   error        // próxima montagem falha com isto (e volta a nil)
	BuildLog   string
	Handler    http.Handler // o que as caixas respondem; padrão: 200 "ok <nome>" + eco dos X-AppHub-*
	Down       bool         // Available devolve ErrUnavailable
	Calls      []string     // "build <tag>", "start <name>", "stop <id>", "remove <id>", "exec <container> <cmd>"
	ExecFn     func(container string, stdin io.Reader, stdout io.Writer, cmd []string) error
}

type FakeContainer struct {
	Spec    StartSpec
	Running bool
	Exit    int
	srv     *httptest.Server
}

func NewFake() *Fake
func (f *Fake) Crash(containerID string)            // para a caixa com código 1 (o servidor de teste fecha)
func (f *Fake) SetHandler(containerID string, h http.Handler) // ex.: uma caixa que só responde 500
func (f *Fake) Running() []string                     // nomes das caixas ligadas, em ordem
```

Cada `Start` do motor de mentira liga um `httptest.Server` de verdade (com o `Handler`), e `Inspect` devolve a porta dele em `HostPort`. Assim a saúde e o porteiro são testados de ponta a ponta sem Podman.

- [ ] **Step 1: Teste que falha** (`fake_test.go`)

```go
package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestFakeStartServesAndCrashes(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	img, _, err := f.Build(ctx, BuildSpec{Tag: "localhost/apphub/x:1", Labels: map[string]string{Label: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.Start(ctx, StartSpec{Name: "apphub-x-1", Image: img, Port: 8501, Labels: map[string]string{Label: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	st, _ := f.Inspect(ctx, id)
	if !st.Running || st.HostPort == 0 {
		t.Fatalf("state %+v", st)
	}
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", st.HostPort))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != "ok apphub-x-1" {
		t.Fatalf("body %q", body)
	}
	f.Crash(id)
	st, _ = f.Inspect(ctx, id)
	if st.Running || st.ExitCode != 1 {
		t.Fatalf("after crash %+v", st)
	}
	if err := f.Remove(ctx, id); err != nil {
		t.Fatal(err)
	}
	if st, _ := f.Inspect(ctx, id); st.Exists {
		t.Fatal("still exists")
	}
}

func TestFakeBuildErrOnce(t *testing.T) {
	f := NewFake()
	f.BuildErr = fmt.Errorf("pip falhou")
	if _, _, err := f.Build(context.Background(), BuildSpec{Tag: "a"}); err == nil {
		t.Fatal("want error")
	}
	if _, _, err := f.Build(context.Background(), BuildSpec{Tag: "a"}); err != nil {
		t.Fatal("second build should work")
	}
}

func TestFakeListOnlyLabelled(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	f.Start(ctx, StartSpec{Name: "a", Labels: map[string]string{Label: "1"}})
	f.Start(ctx, StartSpec{Name: "b"}) // sem rótulo: não pode aparecer
	list, _ := f.List(ctx)
	if len(list) != 1 || list[0].Name != "a" {
		t.Fatalf("%+v", list)
	}
}
```

- [ ] **Step 2: Rodar** — `cd backend && mise x -- go test ./internal/shared/engine/` → FAIL (pacote vazio).

- [ ] **Step 3: Implementar** `engine.go` (tipos acima, só declarações) e `fake.go`:

```go
package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

func NewFake() *Fake {
	return &Fake{Images: map[string]BuildSpec{}, Containers: map[string]*FakeContainer{}}
}

func (f *Fake) record(s string) { f.Calls = append(f.Calls, s) }

func (f *Fake) Available(context.Context) error {
	if f.Down {
		return ErrUnavailable
	}
	return nil
}

func (f *Fake) Build(_ context.Context, b BuildSpec) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("build " + b.Tag)
	if err := f.BuildErr; err != nil {
		f.BuildErr = nil
		return "", f.BuildLog + "\n" + err.Error(), err
	}
	id := "img-" + strconv.Itoa(len(f.Images)+1)
	f.Images[id] = b
	return id, f.BuildLog, nil
}

func (f *Fake) defaultHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Apphub-") {
				w.Header().Set("Echo-"+k, strings.Join(v, ","))
			}
		}
		fmt.Fprint(w, "ok "+name)
	})
}

func (f *Fake) Start(_ context.Context, s StartSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("start " + s.Name)
	h := f.Handler
	if h == nil {
		h = f.defaultHandler(s.Name)
	}
	id := "ctr-" + strconv.Itoa(len(f.Containers)+1) + "-" + s.Name
	f.Containers[id] = &FakeContainer{Spec: s, Running: true, srv: httptest.NewServer(h)}
	return id, nil
}

func (f *Fake) stopLocked(id string, exit int) {
	if c := f.Containers[id]; c != nil && c.Running {
		c.Running, c.Exit = false, exit
		c.srv.Close()
	}
}

func (f *Fake) Stop(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("stop " + id)
	if f.Containers[id] == nil {
		return ErrNotFound
	}
	f.stopLocked(id, 0)
	return nil
}

func (f *Fake) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("remove " + id)
	f.stopLocked(id, 0)
	delete(f.Containers, id)
	return nil
}

func (f *Fake) RemoveImage(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Images, id)
	return nil
}

func (f *Fake) Crash(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopLocked(id, 1)
}

func (f *Fake) SetHandler(id string, h http.Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c := f.Containers[id]; c != nil && c.Running {
		c.srv.Config.Handler = h
	}
}

func (f *Fake) Inspect(_ context.Context, id string) (State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.Containers[id]
	if c == nil {
		return State{}, nil
	}
	st := State{Exists: true, Running: c.Running, ExitCode: c.Exit}
	if c.Running {
		u, _ := url.Parse(c.srv.URL)
		st.HostPort, _ = strconv.Atoi(u.Port())
	}
	return st, nil
}

func (f *Fake) Stats(_ context.Context, ids []string) ([]Usage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Usage
	for _, id := range ids {
		if c := f.Containers[id]; c != nil && c.Running {
			out = append(out, Usage{ContainerID: id, MemBytes: int64(c.Spec.MemoryMB) << 19, CPUPct: 2.5}) // metade do limite
		}
	}
	return out, nil
}

func (f *Fake) Logs(_ context.Context, id string, lines int) (string, error) {
	return "linha 1\nlinha 2\n", nil
}

func (f *Fake) Exec(_ context.Context, container string, stdin io.Reader, stdout io.Writer, cmd ...string) error {
	f.mu.Lock()
	f.record("exec " + container + " " + strings.Join(cmd, " "))
	fn := f.ExecFn
	f.mu.Unlock()
	if fn != nil {
		return fn(container, stdin, stdout, cmd)
	}
	return nil
}

func (f *Fake) List(context.Context) ([]Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Container
	for id, c := range f.Containers {
		if _, ok := c.Spec.Labels[Label]; ok {
			out = append(out, Container{ID: id, Name: c.Spec.Name, Running: c.Running, Labels: c.Spec.Labels})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *Fake) Running() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.Containers {
		if c.Running {
			out = append(out, c.Spec.Name)
		}
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Rodar** — `go test ./internal/shared/engine/` → PASS. Também `go vet ./internal/shared/engine/`.

- [ ] **Step 5: Commit** — `git add backend/internal/shared/engine && git commit -m "Shared apps: engine interface and fake engine"`

---

### Task 5: Motor Podman

**Files:**
- Create: `backend/internal/shared/engine/podman.go`
- Create: `backend/internal/shared/engine/podman_parse_test.go` (sempre roda)
- Create: `backend/internal/shared/engine/podman_test.go` (`//go:build podman`, Podman de verdade)
- Create: `backend/internal/shared/engine/testdata/podman-stats.json`, `podman-inspect.json`, `podman-ps.json`

**Interfaces:**
- Consumes: tipos do Task 4.
- Produces: `func NewPodman(bin string) *Podman` (`bin` = `CONTAINER_ENGINE`, padrão `podman`), que satisfaz `Engine`. Também `func ParseSize(s string) (int64, error)`.

Regras: toda chamada usa `exec.CommandContext` com tempo limite (montagem 15 min; `Exec` só o `ctx`, porque cópias grandes demoram; o resto 30 s). Erro de "comando não encontrado" ou "cannot connect" vira `ErrUnavailable`. A saída de erro vai para o `slog`, nunca para a tela. Env nunca vai na linha de comando: `--env-file` num arquivo temporário 0600 apagado logo após o `run`. Valores com quebra de linha são recusados antes (Task 13 valida).

- [ ] **Step 1: Capturar saídas reais do Podman** do Henry para usar nos testes de leitura (não exige nada novo instalado):

```bash
podman run -d --name apphub-probe --label app-hub.app=0 -p 127.0.0.1::80 docker.io/library/busybox:latest httpd -f -p 80
podman inspect apphub-probe > backend/internal/shared/engine/testdata/podman-inspect.json
podman stats --no-stream --format json apphub-probe > backend/internal/shared/engine/testdata/podman-stats.json
podman ps -a --filter label=app-hub.app --format json > backend/internal/shared/engine/testdata/podman-ps.json
podman rm -f apphub-probe
```

Abrir os três arquivos e conferir os nomes dos campos (em Podman 4.9: `inspect` → `[0].State.Running`, `[0].State.ExitCode`, `[0].NetworkSettings.Ports["80/tcp"][0].HostPort`; `stats` → `[].id`, `[].cpu_percent` como `"0.12%"`, `[].mem_usage` como `"1.2MB / 8.2GB"`; `ps` → `[].Id`, `[].Names[0]`, `[].State`, `[].Labels`). Se forem outros, ajuste os structs abaixo, não os testes.

- [ ] **Step 2: Teste de leitura que falha** (`podman_parse_test.go`)

```go
package engine

import (
	"os"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := map[string]int64{"0B": 0, "512B": 512, "1.5kB": 1500, "12.3MB": 12_300_000, "2GB": 2_000_000_000,
		"1KiB": 1024, "3MiB": 3 << 20, "1.5GiB": 1610612736}
	for in, want := range cases {
		got, err := ParseSize(in)
		if err != nil || got != want {
			t.Errorf("%s: %d %v", in, got, err)
		}
	}
	if _, err := ParseSize("muito"); err == nil {
		t.Error("accepted junk")
	}
}

func TestParseInspect(t *testing.T) {
	raw, _ := os.ReadFile("testdata/podman-inspect.json")
	st, err := parseInspect(raw, 80)
	if err != nil || !st.Exists || !st.Running || st.HostPort == 0 {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestParseStats(t *testing.T) {
	raw, _ := os.ReadFile("testdata/podman-stats.json")
	u, err := parseStats(raw)
	if err != nil || len(u) != 1 || u[0].MemBytes <= 0 || u[0].ContainerID == "" {
		t.Fatalf("%+v %v", u, err)
	}
}

func TestParsePS(t *testing.T) {
	raw, _ := os.ReadFile("testdata/podman-ps.json")
	list, err := parsePS(raw)
	if err != nil || len(list) != 1 || list[0].Labels[Label] != "0" {
		t.Fatalf("%+v %v", list, err)
	}
}
```

- [ ] **Step 3: Rodar** → FAIL.

- [ ] **Step 4: Implementar `podman.go`**

```go
package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Podman struct{ bin string }

func NewPodman(bin string) *Podman {
	if bin == "" {
		bin = "podman"
	}
	return &Podman{bin: bin}
}

const shortTimeout = 30 * time.Second
const buildTimeout = 15 * time.Minute

func (p *Podman) run(ctx context.Context, timeout time.Duration, stdin io.Reader, args ...string) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, p.bin, args...)
	cmd.Stdin = stdin
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err != nil {
		msg := errb.String()
		slog.Warn("podman", "args", args[0], "err", err, "stderr", tail(msg, 2000))
		if errors.Is(err, exec.ErrNotFound) || strings.Contains(msg, "cannot connect") {
			return nil, ErrUnavailable
		}
		return out.Bytes(), fmt.Errorf("%s: %w: %s", args[0], err, tail(msg, 500))
	}
	return out.Bytes(), nil
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func (p *Podman) Available(ctx context.Context) error {
	_, err := p.run(ctx, 5*time.Second, nil, "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func (p *Podman) ensureNetwork(ctx context.Context) error {
	if _, err := p.run(ctx, shortTimeout, nil, "network", "exists", Network); err == nil {
		return nil
	}
	_, err := p.run(ctx, shortTimeout, nil, "network", "create", Network)
	return err
}

func labelArgs(l map[string]string) []string {
	var out []string
	for k, v := range l {
		out = append(out, "--label", k+"="+v)
	}
	return out
}

func (p *Podman) Build(ctx context.Context, b BuildSpec) (string, string, error) {
	df := b.DockerfilePath
	if df == "" {
		df = "Dockerfile"
	}
	iid := filepath.Join(os.TempDir(), "apphub-iid-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	defer os.Remove(iid)
	args := append([]string{"build", "--iidfile", iid, "-t", b.Tag, "-f", filepath.Join(b.ContextDir, df)}, labelArgs(b.Labels)...)
	args = append(args, b.ContextDir)
	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, args...)
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	err := cmd.Run()
	text := tail(log.String(), 64<<10)
	if errors.Is(err, exec.ErrNotFound) {
		return "", text, ErrUnavailable
	}
	if err != nil {
		return "", text, err
	}
	id, err := os.ReadFile(iid)
	return strings.TrimSpace(string(id)), text, err
}

func (p *Podman) Start(ctx context.Context, s StartSpec) (string, error) {
	if err := p.ensureNetwork(ctx); err != nil {
		return "", err
	}
	envFile, err := os.CreateTemp("", "apphub-env-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(envFile.Name())
	for k, v := range s.Env {
		fmt.Fprintf(envFile, "%s=%s\n", k, v)
	}
	envFile.Close()
	args := []string{"run", "-d", "--name", s.Name, "--network", Network,
		"--add-host", "host.containers.internal:host-gateway",
		"-p", fmt.Sprintf("127.0.0.1::%d", s.Port),
		"--memory", fmt.Sprintf("%dm", s.MemoryMB), "--cpus", strconv.FormatFloat(s.CPUs, 'f', 2, 64),
		"--env-file", envFile.Name()}
	if s.DataDir != "" {
		if err := os.MkdirAll(s.DataDir, 0o700); err != nil {
			return "", err
		}
		args = append(args, "-v", s.DataDir+":/data:Z")
	}
	args = append(args, labelArgs(s.Labels)...)
	args = append(args, s.Image)
	out, err := p.run(ctx, shortTimeout, nil, args...)
	return strings.TrimSpace(string(out)), err
}

func (p *Podman) Stop(ctx context.Context, id string) error {
	_, err := p.run(ctx, shortTimeout, nil, "stop", "-t", "10", id)
	return err
}

func (p *Podman) Remove(ctx context.Context, id string) error {
	_, err := p.run(ctx, shortTimeout, nil, "rm", "-f", "--ignore", id)
	return err
}

func (p *Podman) RemoveImage(ctx context.Context, id string) error {
	_, err := p.run(ctx, shortTimeout, nil, "rmi", "--ignore", id)
	return err
}

func (p *Podman) Inspect(ctx context.Context, id string) (State, error) {
	out, err := p.run(ctx, shortTimeout, nil, "inspect", "--type", "container", id)
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return State{}, err
		}
		return State{}, nil // não existe
	}
	return parseInspect(out, 0)
}

// parseInspect reads `podman inspect`. port 0 = the first published port.
func parseInspect(raw []byte, port int) (State, error) {
	var v []struct {
		State struct {
			Running  bool
			ExitCode int
		}
		NetworkSettings struct {
			Ports map[string][]struct{ HostPort string }
		}
	}
	if err := json.Unmarshal(raw, &v); err != nil || len(v) == 0 {
		return State{}, err
	}
	st := State{Exists: true, Running: v[0].State.Running, ExitCode: v[0].State.ExitCode}
	for k, binds := range v[0].NetworkSettings.Ports {
		if (port == 0 || k == fmt.Sprintf("%d/tcp", port)) && len(binds) > 0 {
			st.HostPort, _ = strconv.Atoi(binds[0].HostPort)
			break
		}
	}
	return st, nil
}

func (p *Podman) Stats(ctx context.Context, ids []string) ([]Usage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out, err := p.run(ctx, shortTimeout, nil, append([]string{"stats", "--no-stream", "--format", "json"}, ids...)...)
	if err != nil {
		return nil, err
	}
	return parseStats(out)
}

func parseStats(raw []byte) ([]Usage, error) {
	var v []struct {
		ID       string `json:"id"`
		CPU      string `json:"cpu_percent"`
		MemUsage string `json:"mem_usage"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	out := make([]Usage, 0, len(v))
	for _, s := range v {
		mem, _ := ParseSize(strings.TrimSpace(strings.SplitN(s.MemUsage, "/", 2)[0]))
		cpu, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s.CPU), "%"), 64)
		out = append(out, Usage{ContainerID: s.ID, MemBytes: mem, CPUPct: cpu})
	}
	return out, nil
}

var sizeRe = regexp.MustCompile(`^([0-9.]+)\s*([kKMGT]?i?B)$`)

// ParseSize reads sizes as Podman prints them: decimal (kB, MB) or binary (KiB, MiB).
func ParseSize(s string) (int64, error) {
	m := sizeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("tamanho inválido %q", s)
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}
	mult := map[string]float64{"B": 1, "kB": 1e3, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12,
		"KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "TiB": 1 << 40}[m[2]]
	if mult == 0 {
		return 0, fmt.Errorf("unidade inválida %q", s)
	}
	return int64(n*mult + 0.5), nil
}

func (p *Podman) Logs(ctx context.Context, id string, lines int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, shortTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, "logs", "--tail", strconv.Itoa(lines), id)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (p *Podman) Exec(ctx context.Context, container string, stdin io.Reader, stdout io.Writer, cmd ...string) error {
	args := append([]string{"exec", "-i", container}, cmd...)
	c := exec.CommandContext(ctx, p.bin, args...)
	c.Stdin, c.Stdout = stdin, stdout
	var errb bytes.Buffer
	c.Stderr = &errb
	if err := c.Run(); err != nil {
		return fmt.Errorf("exec %s: %w: %s", cmd[0], err, tail(errb.String(), 500))
	}
	return nil
}

func (p *Podman) List(ctx context.Context) ([]Container, error) {
	out, err := p.run(ctx, shortTimeout, nil, "ps", "-a", "--filter", "label="+Label, "--format", "json")
	if err != nil {
		return nil, err
	}
	return parsePS(out)
}

func parsePS(raw []byte) ([]Container, error) {
	var v []struct {
		ID     string `json:"Id"`
		Names  []string
		State  string
		Labels map[string]string
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(v))
	for _, c := range v {
		if _, ok := c.Labels[Label]; !ok {
			continue // nunca mexer no que não é do Hub
		}
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		out = append(out, Container{ID: c.ID, Name: name, Running: c.State == "running", Labels: c.Labels})
	}
	return out, nil
}

var _ Engine = (*Podman)(nil)
var _ Engine = (*Fake)(nil)
```

- [ ] **Step 5: Teste com o Podman de verdade** (`podman_test.go`, só com `-tags podman`). Prova também que a caixa alcança o Postgres do Hub pelo `host.containers.internal` (o banco do app depende disso):

```go
//go:build podman

package engine

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPodmanRoundTrip(t *testing.T) {
	ctx := context.Background()
	p := NewPodman("")
	if err := p.Available(ctx); err != nil {
		t.Skip("podman indisponível")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM docker.io/library/busybox:latest\nCMD httpd -f -p $PORT\n"), 0o644)
	labels := map[string]string{Label: "999999"}
	img, log, err := p.Build(ctx, BuildSpec{ContextDir: dir, Tag: "localhost/apphub/probe:1", Labels: labels})
	if err != nil {
		t.Fatalf("%v\n%s", err, log)
	}
	t.Cleanup(func() { p.RemoveImage(ctx, img) })
	id, err := p.Start(ctx, StartSpec{Name: "apphub-probe-1", Image: img, Labels: labels, Port: 8080, MemoryMB: 128, CPUs: 0.5,
		Env: map[string]string{"PORT": "8080"}, DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Remove(ctx, id) })
	st, _ := p.Inspect(ctx, id)
	if !st.Running || st.HostPort == 0 {
		t.Fatalf("%+v", st)
	}
	var code int
	for i := 0; i < 20; i++ {
		res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", st.HostPort))
		if err == nil {
			code = res.StatusCode
			res.Body.Close()
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if code == 0 {
		t.Fatal("never answered")
	}
	u, err := p.Stats(ctx, []string{id})
	if err != nil || len(u) != 1 || u[0].MemBytes == 0 {
		t.Fatalf("stats %+v %v", u, err)
	}
	var out strings.Builder
	if err := p.Exec(ctx, id, nil, &out, "nc", "-z", "-w", "3", "host.containers.internal", "5433"); err != nil {
		t.Fatalf("container cannot reach Postgres on host.containers.internal:5433: %v", err)
	}
	list, _ := p.List(ctx)
	found := false
	for _, c := range list {
		found = found || strings.HasPrefix(id, c.ID) || strings.HasPrefix(c.ID, id)
	}
	if !found {
		t.Fatalf("list %+v", list)
	}
}
```

Rodar: `cd backend && mise x -- go test -tags podman ./internal/shared/engine/ -run Podman -v`. Se o `nc` falhar: o Postgres do `app-hub-db` precisa escutar na rede do WSL, não só em `127.0.0.1`. Conferir com `podman port app-hub-db`; se mostrar `127.0.0.1:5433`, anotar em `docs/INFRASTRUCTURE.md` que o `app-hub-db` precisa ser recriado com `-p 5433:5432` (perguntar ao Henry antes de recriar, porque mexe no banco de verdade) e, enquanto isso, usar `APPS_DB_HOST_FOR_CONTAINERS=<ip do WSL>:5433` (`hostname -I | cut -d' ' -f1`).

- [ ] **Step 6: Rodar tudo** — `go test ./internal/shared/engine/` (sem tag) → PASS; com tag → PASS.

- [ ] **Step 7: Commit** — `git add backend/internal/shared/engine && git commit -m "Shared apps: Podman engine"`

---

### Task 6: Banco por app (`shared/appdb`) e banco de testes compartilhado

**Files:**
- Create: `backend/internal/shared/sharedtest/db.go` — prepara um banco de testes próprio (`apphub_test_shared`) para os pacotes `shared/*`, migrado e com o mesmo reset do handler.
- Create: `backend/internal/shared/appdb/appdb.go`, `backend/internal/shared/appdb/appdb_test.go`

**Interfaces:**
- Consumes: `engine.Engine.Exec` (cópias rodam `pg_dump`/`pg_restore` dentro do contêiner do Postgres, porque o WSL não tem essas ferramentas).
- Produces:

```go
package appdb

// DBs is what deploy and the supervisor need; Admin is the real one, Fake the test one.
type DBs interface {
	Ensure(ctx context.Context, name, password string) error // cria (ou acerta a senha de) usuário e banco
	Drop(ctx context.Context, name string) error             // apaga banco e usuário; sem erro se não existem
	URL(name, password string) string                         // DATABASE_URL para a caixa
	Dump(ctx context.Context, name string, w io.Writer) error // formato custom do pg_dump
	Restore(ctx context.Context, name string, r io.Reader) error
	Size(ctx context.Context, name string) (int64, error)
}

type Admin struct { /* pool *pgxpool.Pool; host string; eng engine.Engine; container, adminUser string; Prefix string */ }
func New(pool *pgxpool.Pool, hostForContainers string, eng engine.Engine, dbContainer, adminUser string) *Admin
func (a *Admin) Name(appID int64) string                        // Prefix + id, ex.: "app_12"
func (a *Admin) TestName(appID, deploymentID int64) string       // "app_12_test_7"
func (a *Admin) LockDown(ctx context.Context, hubDB string) error // REVOKE CONNECT ON DATABASE <hub> FROM PUBLIC
func NewPassword() string                                        // 24 bytes aleatórios em hex

type Fake struct{ /* mu; DBs map[string]string (nome→senha); Dumps map[string][]byte; FailDump error */ }
func NewFake() *Fake
```

`Prefix` padrão `app_`; os testes usam `apptest_` para nunca encostar num banco de app de verdade. Nomes passam por `^[a-z][a-z0-9_]{0,40}$` antes de entrar no SQL (identificadores não podem ir como parâmetro).

- [ ] **Step 1: Banco de testes compartilhado** (`sharedtest/db.go`). Copiar a lógica de `ensureTestDatabase` (`handler/testdb_test.go`) para um pacote comum, já que `_test.go` não é importável:

```go
// Package sharedtest gives the shared/* packages their own test database,
// so they never race the handler tests over apphub_test.
package sharedtest

import (
	"context"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database"
)

const Name = "apphub_test_shared"

// Reset empties the tables the shared packages use and puts back the
// built-in admin (id 1, "Henry") and area Research (id 1).
const Reset = `
TRUNCATE shared_usage, shared_backups, app_config, shared_apps, shared_deployments, app_changes, app_versions,
         code_blobs, app_grants, app_viewers, area_grants, area_leaders, area_members, apps, sessions, people, areas
         RESTART IDENTITY CASCADE;
INSERT INTO areas (name) VALUES ('Research');
INSERT INTO people (name, email, password_hash, role_id)
SELECT 'Henry', 'henry@alis.com', 'x', id FROM roles WHERE key = 'admin';
`

// Pool opens (creating and migrating if needed) the shared test database.
// It returns nil when DATABASE_URL is not set, so callers can skip.
func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		return nil, nil
	}
	u, err := url.Parse(dev)
	if err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(ctx, dev)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", Name).Scan(&exists); err != nil {
		conn.Close(ctx)
		return nil, err
	}
	if !exists {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+Name); err != nil {
			conn.Close(ctx)
			return nil, err
		}
	}
	conn.Close(ctx)
	u.Path = "/" + Name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return nil, err
	}
	return pool, database.Migrate(ctx, pool)
}
```

Conferir as colunas obrigatórias de `people` (ler a migração 001/002) e ajustar o `INSERT` do `Reset`.

- [ ] **Step 2: Teste que falha** (`appdb_test.go`)

```go
package appdb

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

func admin(t *testing.T) (*Admin, string) {
	t.Helper()
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dev)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	u, _ := url.Parse(dev)
	a := New(pool, u.Host, engine.NewPodman(""), "app-hub-db", u.User.Username())
	a.Prefix = "apptest_"
	return a, dev
}

// connectAs tries to open db as user (on the host port, as a test can't use host.containers.internal).
func connectAs(dev, user, pw, db string) error {
	u, _ := url.Parse(dev)
	u.User = url.UserPassword(user, pw)
	u.Path = "/" + db
	c, err := pgx.Connect(context.Background(), u.String())
	if err == nil {
		c.Close(context.Background())
	}
	return err
}

func TestEnsureIsolatesApps(t *testing.T) {
	ctx := context.Background()
	a, dev := admin(t)
	one, two := a.Name(1), a.Name(2)
	pw1, pw2 := NewPassword(), NewPassword()
	for name, pw := range map[string]string{one: pw1, two: pw2} {
		if err := a.Ensure(ctx, name, pw); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { a.Drop(ctx, name) })
	}
	if err := a.Ensure(ctx, one, pw1); err != nil { // de novo: não pode falhar
		t.Fatal(err)
	}
	if err := connectAs(dev, one, pw1, one); err != nil {
		t.Fatalf("own db: %v", err)
	}
	if err := connectAs(dev, one, pw1, two); err == nil {
		t.Fatal("app 1 opened app 2's database")
	}
	hub := strings.TrimPrefix(mustURL(dev).Path, "/")
	if err := a.LockDown(ctx, hub); err != nil {
		t.Fatal(err)
	}
	if err := connectAs(dev, one, pw1, hub); err == nil {
		t.Fatal("app 1 opened the Hub's database")
	}
}

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }

func TestRejectsBadNames(t *testing.T) {
	a, _ := admin(t)
	if err := a.Ensure(context.Background(), "x; DROP DATABASE apphub", "p"); err == nil {
		t.Fatal("accepted a dangerous name")
	}
}

func TestDumpRestore(t *testing.T) {
	ctx := context.Background()
	a, dev := admin(t)
	if a.eng.Available(ctx) != nil {
		t.Skip("podman indisponível")
	}
	name, pw := a.Name(3), NewPassword()
	if err := a.Ensure(ctx, name, pw); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Drop(ctx, name) })
	u := mustURL(dev)
	u.User, u.Path = url.UserPassword(name, pw), "/"+name
	c, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	c.Exec(ctx, "CREATE TABLE notas (t text); INSERT INTO notas VALUES ('antes')")
	var buf bytes.Buffer
	if err := a.Dump(ctx, name, &buf); err != nil || buf.Len() == 0 {
		t.Fatalf("dump %v (%d bytes)", err, buf.Len())
	}
	c.Exec(ctx, "UPDATE notas SET t = 'depois'")
	c.Close(ctx)
	if err := a.Restore(ctx, name, &buf); err != nil {
		t.Fatal(err)
	}
	c, _ = pgx.Connect(ctx, u.String())
	defer c.Close(ctx)
	var got string
	c.QueryRow(ctx, "SELECT t FROM notas").Scan(&got)
	if got != "antes" {
		t.Fatalf("after restore %q", got)
	}
	if n, err := a.Size(ctx, name); err != nil || n == 0 {
		t.Fatalf("size %d %v", n, err)
	}
}
```

- [ ] **Step 3: Rodar** — `set -a && . ../.env && set +a && mise x -- go test ./internal/shared/appdb/` → FAIL.

- [ ] **Step 4: Implementar `appdb.go`**

```go
// Package appdb gives each shared app its own database and user on the
// Hub's Postgres server, and copies and restores them.
package appdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
)

var safeName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,40}$`)
var errBadName = errors.New("nome de banco inválido")

type Admin struct {
	pool      *pgxpool.Pool
	host      string
	eng       engine.Engine
	container string
	adminUser string
	Prefix    string
}

func New(pool *pgxpool.Pool, hostForContainers string, eng engine.Engine, dbContainer, adminUser string) *Admin {
	return &Admin{pool: pool, host: hostForContainers, eng: eng, container: dbContainer, adminUser: adminUser, Prefix: "app_"}
}

func (a *Admin) Name(appID int64) string { return fmt.Sprintf("%s%d", a.Prefix, appID) }
func (a *Admin) TestName(appID, depID int64) string {
	return fmt.Sprintf("%s%d_test_%d", a.Prefix, appID, depID)
}

func NewPassword() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ident(s string) string { return pgx.Identifier{s}.Sanitize() }

func (a *Admin) Ensure(ctx context.Context, name, password string) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	var roleExists, dbExists bool
	if err := a.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", name).Scan(&roleExists); err != nil {
		return err
	}
	pw := "'" + password + "'" // hex only (NewPassword); never user text
	if roleExists {
		if _, err := a.pool.Exec(ctx, "ALTER ROLE "+ident(name)+" WITH LOGIN PASSWORD "+pw+" CONNECTION LIMIT 20"); err != nil {
			return err
		}
	} else if _, err := a.pool.Exec(ctx, "CREATE ROLE "+ident(name)+" WITH LOGIN PASSWORD "+pw+" CONNECTION LIMIT 20"); err != nil {
		return err
	}
	if err := a.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", name).Scan(&dbExists); err != nil {
		return err
	}
	if !dbExists {
		if _, err := a.pool.Exec(ctx, "CREATE DATABASE "+ident(name)+" OWNER "+ident(name)); err != nil {
			return err
		}
	}
	_, err := a.pool.Exec(ctx, "REVOKE CONNECT ON DATABASE "+ident(name)+" FROM PUBLIC")
	return err
}

func (a *Admin) Drop(ctx context.Context, name string) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	if _, err := a.pool.Exec(ctx, "DROP DATABASE IF EXISTS "+ident(name)+" WITH (FORCE)"); err != nil {
		return err
	}
	_, err := a.pool.Exec(ctx, "DROP ROLE IF EXISTS "+ident(name))
	return err
}

func (a *Admin) LockDown(ctx context.Context, hubDB string) error {
	if !safeName.MatchString(hubDB) {
		return errBadName
	}
	_, err := a.pool.Exec(ctx, "REVOKE CONNECT ON DATABASE "+ident(hubDB)+" FROM PUBLIC")
	return err
}

func (a *Admin) URL(name, password string) string {
	u := url.URL{Scheme: "postgres", User: url.UserPassword(name, password), Host: a.host, Path: "/" + name, RawQuery: "sslmode=disable"}
	return u.String()
}

func (a *Admin) Dump(ctx context.Context, name string, w io.Writer) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	return a.eng.Exec(ctx, a.container, nil, w, "pg_dump", "-U", a.adminUser, "-Fc", "--no-owner", name)
}

func (a *Admin) Restore(ctx context.Context, name string, r io.Reader) error {
	if !safeName.MatchString(name) {
		return errBadName
	}
	return a.eng.Exec(ctx, a.container, r, io.Discard, "pg_restore", "-U", a.adminUser, "--clean", "--if-exists",
		"--no-owner", "--role="+name, "-d", name)
}

func (a *Admin) Size(ctx context.Context, name string) (int64, error) {
	var n int64
	err := a.pool.QueryRow(ctx, "SELECT pg_database_size($1)", name).Scan(&n)
	return n, err
}
```

E o `Fake` (mesmo arquivo, ou `fake.go`), guardando bancos e cópias em memória:

```go
type Fake struct {
	mu       sync.Mutex
	DBs      map[string]string // nome → senha
	Data     map[string]string // conteúdo "do banco", para provar restaurações
	FailDump error
}

func NewFake() *Fake { return &Fake{DBs: map[string]string{}, Data: map[string]string{}} }
func (f *Fake) Ensure(_ context.Context, n, pw string) error { f.mu.Lock(); defer f.mu.Unlock(); f.DBs[n] = pw; return nil }
func (f *Fake) Drop(_ context.Context, n string) error { f.mu.Lock(); defer f.mu.Unlock(); delete(f.DBs, n); delete(f.Data, n); return nil }
func (f *Fake) URL(n, pw string) string { return "postgres://" + n + ":" + pw + "@db/" + n }
func (f *Fake) Dump(_ context.Context, n string, w io.Writer) error {
	f.mu.Lock(); defer f.mu.Unlock()
	if f.FailDump != nil { return f.FailDump }
	_, err := io.WriteString(w, "dump:"+f.Data[n])
	return err
}
func (f *Fake) Restore(_ context.Context, n string, r io.Reader) error {
	b, err := io.ReadAll(r)
	f.mu.Lock(); defer f.mu.Unlock()
	f.Data[n] = strings.TrimPrefix(string(b), "dump:")
	return err
}
func (f *Fake) Size(_ context.Context, n string) (int64, error) { return 8 << 20, nil }

var _ DBs = (*Admin)(nil)
var _ DBs = (*Fake)(nil)
```

(Formatar com `gofmt` — as funções de uma linha acima são só para caber no plano.)

- [ ] **Step 5: Rodar** — `go test ./internal/shared/appdb/ -v` → PASS (o `TestDumpRestore` pula se o Podman não estiver ligado; no PC do Henry deve rodar).

- [ ] **Step 6: Commit** — `git add backend/internal/shared/sharedtest backend/internal/shared/appdb && git commit -m "Shared apps: one database and user per app, copies via the DB container"`

---

# Fase 3 — Receita e checagem

### Task 7: Receitas automáticas

**Files:**
- Create: `backend/internal/shared/recipes/recipes.go`, `backend/internal/shared/recipes/recipes_test.go`
- Create: `backend/testdata/shared/streamlit/` (`app.py`, `requirements.txt`), `flask/` (`app.py`, `requirements.txt`), `fastapi/` (`main.py`, `requirements.txt`), `node/` (`package.json`, `package-lock.json`, `server.js`), `dockerfile/` (`Dockerfile` com `EXPOSE 9000`, `main.py`), `streamlit-noreq/` (`app.py` importando `streamlit` e `pandas`, sem `requirements.txt`), `script/` (`run.py` sem web)

**Interfaces:**
- Consumes: `versions.Folder` (`Walk`, `Open`) — a mesma interface que o Agente vai implementar na nuvem; `manifest.Manifest`, `manifest.Shared.WithDefaults()`.
- Produces:

```go
package recipes

type Recipe struct {
	Kind         string            // "streamlit", "flask", "fastapi", "django", "node", "dockerfile", "" (nenhuma)
	Dockerfile   string            // conteúdo; vazio quando Kind == "dockerfile" (usa o do app)
	Port         int               // porta dentro da caixa
	Command      string            // comando de início (mostrado ao criador)
	NeedsConfirm bool              // Python web: o criador confirma o comando
	ExtraFiles   map[string]string // gravados só na área de montagem (ex.: requirements gerado)
	Requirements []string          // bibliotecas usadas quando o Hub gerou a lista
}

const DockerfileName = ".apphub.Dockerfile" // nome na área de montagem, nunca na pasta do app

func Detect(src versions.Folder, m manifest.Manifest) (Recipe, error)
```

- [ ] **Step 1: Criar as pastas de exemplo** em `backend/testdata/shared/`, por exemplo `streamlit/app.py`:

```python
import streamlit as st
st.title("Notas da equipe")
```

`streamlit/requirements.txt`: `streamlit==1.39.0`. `flask/app.py`: `from flask import Flask\napp = Flask(__name__)\n@app.get("/")\ndef home():\n    return "ok"\n`, requirements `flask`. `fastapi/main.py`: `from fastapi import FastAPI\napp = FastAPI()\n`, requirements `fastapi\nuvicorn`. `node/package.json`: `{"name":"x","scripts":{"start":"node server.js","build":"echo ok"}}`, `package-lock.json` mínimo `{"lockfileVersion":3}`, `server.js` lendo `process.env.PORT`. `dockerfile/Dockerfile`: `FROM python:3.12-slim\nEXPOSE 9000\nCMD ["python","main.py"]`. `streamlit-noreq/app.py`: `import streamlit as st\nimport pandas as pd\n`. `script/run.py`: `print("oi")`.

- [ ] **Step 2: Teste que falha**

```go
package recipes

import (
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func detect(t *testing.T, dir string, m manifest.Manifest) Recipe {
	t.Helper()
	if m.Shared == nil {
		m.Shared = &manifest.Shared{Enabled: true}
	}
	r, err := Detect(versions.LocalFolder{Root: "../../../testdata/shared/" + dir}, m)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStreamlit(t *testing.T) {
	r := detect(t, "streamlit", manifest.Manifest{Entrypoint: "app.py"})
	if r.Kind != "streamlit" || r.Port != 8501 || r.NeedsConfirm {
		t.Fatalf("%+v", r)
	}
	for _, want := range []string{"FROM docker.io/library/python:3.12-slim", "pip install --no-cache-dir -r requirements.txt",
		"streamlit run app.py", "--server.address 0.0.0.0", "--server.headless true"} {
		if !strings.Contains(r.Dockerfile, want) {
			t.Errorf("missing %q in\n%s", want, r.Dockerfile)
		}
	}
}

func TestStreamlitEntrypointFoundWithoutManifest(t *testing.T) {
	r := detect(t, "streamlit", manifest.Manifest{})
	if !strings.Contains(r.Dockerfile, "streamlit run app.py") {
		t.Fatal(r.Dockerfile)
	}
}

func TestStreamlitWithoutRequirementsGeneratesList(t *testing.T) {
	r := detect(t, "streamlit-noreq", manifest.Manifest{Entrypoint: "app.py"})
	req := r.ExtraFiles["requirements.txt"]
	if !strings.Contains(req, "streamlit") || !strings.Contains(req, "pandas") || len(r.Requirements) != 2 {
		t.Fatalf("%+v", r)
	}
}

func TestPythonVersionFromManifest(t *testing.T) {
	r := detect(t, "streamlit", manifest.Manifest{Entrypoint: "app.py", Shared: &manifest.Shared{Enabled: true, Python: "3.11"}})
	if !strings.Contains(r.Dockerfile, "python:3.11-slim") {
		t.Fatal(r.Dockerfile)
	}
}

func TestFlaskAndFastAPI(t *testing.T) {
	f := detect(t, "flask", manifest.Manifest{Entrypoint: "app.py"})
	if f.Kind != "flask" || !f.NeedsConfirm || !strings.Contains(f.Command, "gunicorn -b 0.0.0.0:$PORT app:app") {
		t.Fatalf("%+v", f)
	}
	a := detect(t, "fastapi", manifest.Manifest{Entrypoint: "main.py"})
	if a.Kind != "fastapi" || !strings.Contains(a.Command, "uvicorn main:app --host 0.0.0.0 --port $PORT") {
		t.Fatalf("%+v", a)
	}
}

func TestConfirmedCommandWins(t *testing.T) {
	r := detect(t, "flask", manifest.Manifest{Entrypoint: "app.py", Shared: &manifest.Shared{Enabled: true, Command: "gunicorn -b 0.0.0.0:$PORT wsgi:app"}})
	if r.NeedsConfirm || !strings.Contains(r.Dockerfile, "wsgi:app") {
		t.Fatalf("%+v", r)
	}
}

func TestNode(t *testing.T) {
	r := detect(t, "node", manifest.Manifest{})
	if r.Kind != "node" || r.Port != 3000 {
		t.Fatalf("%+v", r)
	}
	for _, want := range []string{"FROM docker.io/library/node:22-slim", "npm ci", "npm run build --if-present", `CMD ["npm", "start"]`} {
		if !strings.Contains(r.Dockerfile, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestOwnDockerfile(t *testing.T) {
	r := detect(t, "dockerfile", manifest.Manifest{})
	if r.Kind != "dockerfile" || r.Port != 9000 || r.Dockerfile != "" {
		t.Fatalf("%+v", r)
	}
	r = detect(t, "dockerfile", manifest.Manifest{Shared: &manifest.Shared{Enabled: true, Port: 9100}})
	if r.Port != 9100 {
		t.Fatalf("manifest port should win: %+v", r)
	}
}

func TestUnknown(t *testing.T) {
	r := detect(t, "script", manifest.Manifest{})
	if r.Kind != "" {
		t.Fatalf("%+v", r)
	}
}
```

- [ ] **Step 3: Rodar** → FAIL.

- [ ] **Step 4: Implementar `recipes.go`**

```go
// Package recipes builds a container recipe for the kinds of app the Hub
// knows, so the creator does not have to write a Dockerfile. Recipes live in
// the Hub; they are never written into the app folder.
package recipes

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

const DockerfileName = ".apphub.Dockerfile"

type Recipe struct {
	Kind         string            `json:"kind"`
	Dockerfile   string            `json:"dockerfile"`
	Port         int               `json:"port"`
	Command      string            `json:"command"`
	NeedsConfirm bool              `json:"needs_confirm"`
	ExtraFiles   map[string]string `json:"-"`
	Requirements []string          `json:"requirements"`
}

var (
	exposeRe   = regexp.MustCompile(`(?mi)^\s*EXPOSE\s+(\d+)`)
	pyImportRe = regexp.MustCompile(`(?m)^\s*(?:from|import)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	skip       = map[string]bool{".git": true, ".venv": true, "venv": true, "node_modules": true, "__pycache__": true}
	// import name → pip name, for the common ones whose names differ.
	pipName = map[string]string{"sklearn": "scikit-learn", "cv2": "opencv-python", "PIL": "pillow", "yaml": "pyyaml", "dotenv": "python-dotenv", "docx": "python-docx", "bs4": "beautifulsoup4"}
)

type files struct {
	src  versions.Folder
	list []string // top two levels, slash paths
}

func list(src versions.Folder) (files, error) {
	f := files{src: src}
	err := src.Walk(func(rel string, dir bool, _ int64, _ time.Time) error {
		if dir {
			if skip[path.Base(rel)] || strings.Count(rel, "/") >= 2 {
				return fsSkipDir
			}
			return nil
		}
		f.list = append(f.list, rel)
		return nil
	})
	return f, err
}

func (f files) has(rel string) bool {
	for _, p := range f.list {
		if p == rel {
			return true
		}
	}
	return false
}

func (f files) read(rel string) string {
	r, err := f.src.Open(rel)
	if err != nil {
		return ""
	}
	defer r.Close()
	b, _ := io.ReadAll(io.LimitReader(r, 1<<20))
	return string(b)
}

func Detect(src versions.Folder, m manifest.Manifest) (Recipe, error) {
	s := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		s = *m.Shared
	}
	s = s.WithDefaults()
	f, err := list(src)
	if err != nil {
		return Recipe{}, err
	}
	if f.has("Dockerfile") {
		r := Recipe{Kind: "dockerfile", Port: s.Port}
		if r.Port == 0 {
			if m := exposeRe.FindStringSubmatch(f.read("Dockerfile")); m != nil {
				r.Port, _ = strconv.Atoi(m[1])
			}
		}
		return r, nil
	}
	if f.has("package.json") {
		var pkg struct{ Scripts map[string]string }
		if json.Unmarshal([]byte(f.read("package.json")), &pkg) == nil && pkg.Scripts["start"] != "" {
			return nodeRecipe(s, f.has("package-lock.json")), nil
		}
	}
	reqText := f.read("requirements.txt")
	imports := f.pythonImports()
	libs := strings.ToLower(reqText)
	uses := func(lib string) bool { return strings.Contains(libs, lib) || imports[lib] }
	entry := m.Entrypoint
	switch {
	case uses("streamlit"):
		if entry == "" || !strings.HasSuffix(entry, ".py") {
			entry = f.fileImporting("streamlit")
		}
		r := pyBase(s, f, reqText, imports)
		r.Kind, r.Port = "streamlit", 8501
		r.Command = "streamlit run " + entry + " --server.port $PORT --server.address 0.0.0.0 --server.headless true"
		r.Dockerfile += "CMD " + r.Command + "\n"
		return r, nil
	case uses("fastapi"), uses("flask"), uses("django"):
		r := pyBase(s, f, reqText, imports)
		r.Port = 8000
		mod := strings.TrimSuffix(path.Base(entry), ".py")
		if mod == "" {
			mod = "app"
		}
		switch {
		case uses("fastapi"):
			r.Kind, r.Command = "fastapi", "uvicorn "+mod+":app --host 0.0.0.0 --port $PORT"
		case uses("flask"):
			r.Kind, r.Command = "flask", "gunicorn -b 0.0.0.0:$PORT "+mod+":app"
		default:
			r.Kind, r.Command = "django", "gunicorn -b 0.0.0.0:$PORT "+f.djangoProject()+".wsgi"
		}
		r.NeedsConfirm = true
		if s.Command != "" {
			r.Command, r.NeedsConfirm = s.Command, false
		}
		server := "gunicorn"
		if r.Kind == "fastapi" {
			server = "uvicorn"
		}
		r.Dockerfile += "RUN pip install --no-cache-dir " + server + "\nCMD " + r.Command + "\n"
		return r, nil
	}
	return Recipe{}, nil
}

func pyBase(s manifest.Shared, f files, reqText string, imports map[string]bool) Recipe {
	r := Recipe{ExtraFiles: map[string]string{}}
	if strings.TrimSpace(reqText) == "" {
		local := map[string]bool{}
		for _, p := range f.list {
			if strings.HasSuffix(p, ".py") {
				local[strings.TrimSuffix(path.Base(p), ".py")] = true
			}
		}
		for imp := range imports {
			if !local[imp] && !stdlib[imp] {
				name := imp
				if n, ok := pipName[imp]; ok {
					name = n
				}
				r.Requirements = append(r.Requirements, name)
			}
		}
		sort.Strings(r.Requirements)
		r.ExtraFiles["requirements.txt"] = strings.Join(r.Requirements, "\n") + "\n"
	}
	r.Dockerfile = fmt.Sprintf(`FROM docker.io/library/python:%s-slim
WORKDIR /app
ENV PYTHONUNBUFFERED=1
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
`, s.Python)
	return r
}

func nodeRecipe(s manifest.Shared, lock bool) Recipe {
	install := "npm ci"
	if !lock {
		install = "npm install"
	}
	df := fmt.Sprintf(`FROM docker.io/library/node:%s-slim
WORKDIR /app
COPY package*.json ./
RUN %s
COPY . .
RUN npm run build --if-present
CMD ["npm", "start"]
`, s.Node, install)
	return Recipe{Kind: "node", Port: 3000, Command: "npm start", Dockerfile: df}
}

// pythonImports lists top-level module names imported by .py files in the
// first two folder levels.
func (f files) pythonImports() map[string]bool {
	out := map[string]bool{}
	for _, p := range f.list {
		if !strings.HasSuffix(p, ".py") {
			continue
		}
		sc := bufio.NewScanner(strings.NewReader(f.read(p)))
		for sc.Scan() {
			if m := pyImportRe.FindStringSubmatch(sc.Text()); m != nil {
				out[m[1]] = true
			}
		}
	}
	return out
}

func (f files) fileImporting(mod string) string {
	for _, p := range f.list {
		if strings.HasSuffix(p, ".py") && regexp.MustCompile(`(?m)^\s*(import|from)\s+`+mod+`\b`).MatchString(f.read(p)) {
			return p
		}
	}
	return "app.py"
}

func (f files) djangoProject() string {
	for _, p := range f.list {
		if path.Base(p) == "wsgi.py" {
			return path.Dir(p)
		}
	}
	return "config"
}
```

Mais `var fsSkipDir = fs.SkipDir` (importar `io/fs`) e um `stdlib` com os módulos padrão mais comuns do Python (`os, sys, re, json, csv, datetime, time, math, pathlib, typing, collections, itertools, functools, subprocess, logging, sqlite3, io, shutil, glob, random, uuid, hashlib, base64, threading, dataclasses, enum, urllib, http, email, zipfile, tempfile, argparse, decimal, statistics, string, textwrap, traceback, copy, pickle, unittest, asyncio, socket, ssl, smtplib, xml, html, calendar, locale, platform, signal, struct, queue, contextlib, abc, inspect, warnings, operator, secrets, getpass, configparser, pprint`).

- [ ] **Step 5: Rodar** — `go test ./internal/shared/recipes/` → PASS.

- [ ] **Step 6: Commit** — `git add backend/internal/shared/recipes backend/testdata/shared && git commit -m "Shared apps: automatic recipes for Streamlit, Python web and Node"`

---

### Task 8: Checagem (`shared/readiness`)

**Files:**
- Create: `backend/internal/shared/readiness/readiness.go`, `backend/internal/shared/readiness/readiness_test.go`
- Create: `backend/testdata/shared/sqlite/` (`app.py` com `import streamlit` e `sqlite3.connect("dados.db")`), `windows-path/` (`app.py` com `import streamlit` e `open(r"C:\Users\henry\OneDrive\base.xlsx")`), `env-reads/` (`app.py` com `import streamlit`, `os.getenv("SMTP_PASSWORD")`, `os.environ["ADMIN_EMAILS"]`, `os.environ.get("DATABASE_URL")`), `playwright/` (`app.py` com `import streamlit` e `subprocess.run(["node", "gerar_pdf.js"])`, mais `gerar_pdf.js` com `require("playwright")`)

**Interfaces:**
- Consumes: `recipes.Detect`, `versions.Folder`, `manifest.Manifest`, `scanner.SecretFinding`.
- Produces:

```go
package readiness

type Item struct {
	Key        string `json:"key"`         // "recipe","port","database","file_data","windows_paths","settings","secrets","container_limits"
	Title      string `json:"title"`
	Status     string `json:"status"`      // "ok" | "auto" (o Hub resolveu) | "missing" (bloqueia) | "warn" (não bloqueia)
	Detail     string `json:"detail"`
	ClaudeText string `json:"claude_text"` // texto pronto para colar no Claude ("" se não houver)
	Acked      bool   `json:"acked"`       // aviso marcado "estou ciente"
}

type Report struct {
	Ready       bool               `json:"ready"`        // nenhum "missing" e todo "warn" com Acked
	Recipe      recipes.Recipe     `json:"recipe"`
	WantsDB     bool               `json:"wants_db"`
	SuggestEnv  []manifest.EnvVar  `json:"suggest_env"`  // configurações lidas no código que faltam no manifesto
	Items       []Item             `json:"items"`
}

func Check(src versions.Folder, m manifest.Manifest, secrets []scanner.SecretFinding, acks []string) (Report, error)
```

- [ ] **Step 1: Criar as pastas de exemplo** acima.

- [ ] **Step 2: Teste que falha**

```go
package readiness

import (
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func check(t *testing.T, dir string, m manifest.Manifest, secrets []scanner.SecretFinding, acks ...string) Report {
	t.Helper()
	if m.Shared == nil {
		m.Shared = &manifest.Shared{Enabled: true}
	}
	m.Kind = manifest.KindWeb
	r, err := Check(versions.LocalFolder{Root: "../../../testdata/shared/" + dir}, m, secrets, acks)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func item(r Report, key string) Item {
	for _, it := range r.Items {
		if it.Key == key {
			return it
		}
	}
	return Item{}
}

func TestStreamlitIsReady(t *testing.T) {
	r := check(t, "streamlit", manifest.Manifest{Entrypoint: "app.py"}, nil)
	if !r.Ready || item(r, "recipe").Status != "auto" || item(r, "port").Status != "auto" {
		t.Fatalf("%+v", r)
	}
}

func TestUnknownNeedsDockerfile(t *testing.T) {
	r := check(t, "script", manifest.Manifest{}, nil)
	it := item(r, "recipe")
	if r.Ready || it.Status != "missing" || !strings.Contains(it.ClaudeText, "Dockerfile") {
		t.Fatalf("%+v", it)
	}
}

func TestDockerfileWithoutPortAsksPort(t *testing.T) {
	// a pasta "dockerfile" tem EXPOSE; aqui o manifesto não diz a porta e o Dockerfile vem sem EXPOSE
	r := check(t, "dockerfile-noexpose", manifest.Manifest{}, nil)
	if r.Ready || item(r, "port").Status != "missing" {
		t.Fatalf("%+v", item(r, "port"))
	}
}

func TestSQLiteWarnsUntilAcked(t *testing.T) {
	r := check(t, "sqlite", manifest.Manifest{Entrypoint: "app.py"}, nil)
	it := item(r, "file_data")
	if r.Ready || it.Status != "warn" || !strings.Contains(it.ClaudeText, "DATABASE_URL") {
		t.Fatalf("%+v", it)
	}
	r = check(t, "sqlite", manifest.Manifest{Entrypoint: "app.py"}, nil, "file_data")
	if !r.Ready || !item(r, "file_data").Acked {
		t.Fatalf("acked: %+v", r)
	}
}

func TestWindowsPaths(t *testing.T) {
	it := item(check(t, "windows-path", manifest.Manifest{Entrypoint: "app.py"}, nil), "windows_paths")
	if it.Status != "warn" || !strings.Contains(it.ClaudeText, "APPHUB_DATA_DIR") || !strings.Contains(it.Detail, "app.py") {
		t.Fatalf("%+v", it)
	}
}

func TestSettingsReadInCode(t *testing.T) {
	r := check(t, "env-reads", manifest.Manifest{Entrypoint: "app.py"}, nil)
	if !r.WantsDB || item(r, "database").Status != "auto" {
		t.Fatalf("DATABASE_URL should mean a database: %+v", r)
	}
	var names []string
	for _, e := range r.SuggestEnv {
		names = append(names, e.Name)
	}
	if strings.Join(names, ",") != "ADMIN_EMAILS,SMTP_PASSWORD" {
		t.Fatalf("suggest %v", names)
	}
	if !r.SuggestEnv[1].Secret {
		t.Fatal("SMTP_PASSWORD should be a secret")
	}
	// já declaradas no manifesto: não sugere de novo
	m := manifest.Manifest{Entrypoint: "app.py", Shared: &manifest.Shared{Enabled: true,
		Env: []manifest.EnvVar{{Name: "ADMIN_EMAILS", Label: "x"}, {Name: "SMTP_PASSWORD", Label: "y", Secret: true}}}}
	if r := check(t, "env-reads", m, nil); len(r.SuggestEnv) != 0 || item(r, "settings").Status != "ok" {
		t.Fatalf("%+v", r.SuggestEnv)
	}
}

func TestSecretInCode(t *testing.T) {
	r := check(t, "streamlit", manifest.Manifest{Entrypoint: "app.py"}, []scanner.SecretFinding{{Name: "API_KEY", File: "app.py", Line: 3}})
	it := item(r, "secrets")
	if it.Status != "warn" || !strings.Contains(it.ClaudeText, "API_KEY") {
		t.Fatalf("%+v", it)
	}
}

func TestPlaywrightOutsideRecipe(t *testing.T) {
	it := item(check(t, "playwright", manifest.Manifest{Entrypoint: "app.py"}, nil), "container_limits")
	if it.Status != "warn" || !strings.Contains(it.Detail, "PDF") && !strings.Contains(it.Detail, "navegador") {
		t.Fatalf("%+v", it)
	}
}
```

Criar também `testdata/shared/dockerfile-noexpose/Dockerfile` (`FROM python:3.12-slim\nCMD ["python","main.py"]`).

- [ ] **Step 3: Rodar** → FAIL.

- [ ] **Step 4: Implementar `readiness.go`**

```go
// Package readiness checks whether an app folder can run as a shared app,
// says what the Hub resolves by itself, and writes the text to paste into
// Claude for what needs a code change.
package readiness

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

var (
	envReadRe  = regexp.MustCompile(`(?:os\.getenv\(|os\.environ\.get\(|os\.environ\[|process\.env\.)\s*["']?([A-Z][A-Z0-9_]*)`)
	fileDataRe = regexp.MustCompile(`sqlite3\.connect|\.to_csv\(|\.to_excel\(|openpyxl\.Workbook\(|\.save\(\s*["'][^"']+\.xlsx|open\([^)]*,\s*["'](?:w|a)`)
	winPathRe  = regexp.MustCompile(`(?i)[A-Z]:\\\\|[A-Z]:\\[A-Za-z]|/mnt/[a-z]/|OneDrive`)
	browserRe  = regexp.MustCompile(`playwright|puppeteer|chromium|wkhtmltopdf|selenium`)
	secretWord = regexp.MustCompile(`(?i)PASS|SECRET|TOKEN|KEY`)
	codeExt    = map[string]bool{".py": true, ".js": true, ".mjs": true, ".ts": true}
)

type hit struct{ file string; line int }

func Check(src versions.Folder, m manifest.Manifest, secrets []scanner.SecretFinding, acks []string) (Report, error) {
	rec, err := recipes.Detect(src, m)
	if err != nil {
		return Report{}, err
	}
	s := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		s = *m.Shared
	}
	r := Report{Recipe: rec, WantsDB: s.Database == "postgres"}
	var fileData, winPaths, browser []hit
	envSeen := map[string]bool{}
	err = src.Walk(func(rel string, dir bool, _ int64, _ time.Time) error {
		if dir {
			if b := path.Base(rel); b == ".git" || b == "node_modules" || b == ".venv" || b == "venv" || b == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}
		if !codeExt[path.Ext(rel)] {
			return nil
		}
		f, err := src.Open(rel)
		if err != nil {
			return nil
		}
		b, _ := io.ReadAll(io.LimitReader(f, 1<<20))
		f.Close()
		for i, line := range strings.Split(string(b), "\n") {
			for _, mm := range envReadRe.FindAllStringSubmatch(line, -1) {
				envSeen[mm[1]] = true
			}
			if fileDataRe.MatchString(line) {
				fileData = append(fileData, hit{rel, i + 1})
			}
			if winPathRe.MatchString(line) {
				winPaths = append(winPaths, hit{rel, i + 1})
			}
			if browserRe.MatchString(line) {
				browser = append(browser, hit{rel, i + 1})
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	add := func(it Item) {
		if it.Status == "warn" && slices.Contains(acks, it.Key) {
			it.Acked = true
		}
		r.Items = append(r.Items, it)
	}

	// Receita
	switch rec.Kind {
	case "":
		add(Item{Key: "recipe", Title: "Receita para ligar o app", Status: "missing",
			Detail: "O Hub não reconheceu o tipo do app (Streamlit, Flask, FastAPI, Django ou Node).",
			ClaudeText: "Escreva um Dockerfile na raiz deste app para rodá-lo num contêiner Linux. O app deve escutar em 0.0.0.0 na porta da variável de ambiente PORT. Não copie senhas para a imagem."})
	case "dockerfile":
		add(Item{Key: "recipe", Title: "Receita para ligar o app", Status: "ok", Detail: "O app tem Dockerfile próprio."})
	default:
		detail := "O Hub monta a receita sozinho (" + rec.Kind + ")."
		if len(rec.Requirements) > 0 {
			detail += " Sem requirements.txt; bibliotecas encontradas: " + strings.Join(rec.Requirements, ", ") + "."
		}
		st := "auto"
		if rec.NeedsConfirm {
			st, detail = "missing", detail+" Confirme o comando de início: "+rec.Command
		}
		add(Item{Key: "recipe", Title: "Receita para ligar o app", Status: st, Detail: detail})
	}

	// Porta
	switch {
	case rec.Kind != "" && rec.Kind != "dockerfile":
		add(Item{Key: "port", Title: "Porta", Status: "auto", Detail: fmt.Sprintf("O Hub usa a porta %d.", rec.Port)})
	case rec.Port > 0:
		add(Item{Key: "port", Title: "Porta", Status: "ok", Detail: fmt.Sprintf("Porta %d.", rec.Port)})
	case rec.Kind == "dockerfile":
		add(Item{Key: "port", Title: "Porta", Status: "missing", Detail: "Diga em qual porta o app escuta.",
			ClaudeText: "Acrescente EXPOSE <porta> ao Dockerfile com a porta em que o app escuta."})
	}

	// Banco
	if envSeen["DATABASE_URL"] {
		r.WantsDB = true
	}
	if r.WantsDB {
		add(Item{Key: "database", Title: "Banco de dados", Status: "auto", Detail: "O Hub cria um banco só deste app e entrega em DATABASE_URL."})
	}

	// Dados em arquivo
	if len(fileData) > 0 {
		add(Item{Key: "file_data", Title: "Dados gravados em arquivo", Status: "warn",
			Detail:     "O app grava dados em arquivo (" + where(fileData) + "). Com várias pessoas ao mesmo tempo, gravações podem se perder.",
			ClaudeText: "Este app vai rodar uma vez só para várias pessoas. Troque a gravação em arquivo (" + where(fileData) + ") por um banco Postgres lido da variável DATABASE_URL, criando as tabelas na primeira execução."})
	}

	// Pastas do Windows
	if len(winPaths) > 0 {
		add(Item{Key: "windows_paths", Title: "Pastas do computador no código", Status: "warn",
			Detail:     "O código usa pastas do Windows ou do OneDrive (" + where(winPaths) + "), que não existem no servidor.",
			ClaudeText: "Este app vai rodar num servidor Linux. Troque os caminhos de pastas do Windows/OneDrive (" + where(winPaths) + ") pela pasta da variável APPHUB_DATA_DIR, ou por um envio de arquivo na própria tela do app."})
	}

	// Configurações
	declared := map[string]bool{}
	for _, e := range s.Env {
		declared[e.Name] = true
	}
	var names []string
	for n := range envSeen {
		if !declared[n] && !slices.Contains(manifest.ReservedEnv, n) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		r.SuggestEnv = append(r.SuggestEnv, manifest.EnvVar{Name: n, Label: n, Secret: secretWord.MatchString(n), Required: true})
	}
	if len(names) > 0 {
		add(Item{Key: "settings", Title: "Configurações", Status: "auto",
			Detail: "O código lê " + strings.Join(names, ", ") + ". O Hub acrescenta à lista para você preencher."})
	} else if len(envSeen) > 0 || len(s.Env) > 0 {
		add(Item{Key: "settings", Title: "Configurações", Status: "ok", Detail: "Todas as configurações estão na lista."})
	}

	// Senhas no código
	if len(secrets) > 0 {
		var ss []string
		for _, f := range secrets {
			ss = append(ss, fmt.Sprintf("%s (%s, linha %d)", f.Name, f.File, f.Line))
		}
		add(Item{Key: "secrets", Title: "Senha escrita no código", Status: "warn",
			Detail:     "Encontramos o que parece ser senha no código: " + strings.Join(ss, "; ") + ".",
			ClaudeText: "Tire do código as senhas " + strings.Join(ss, "; ") + " e leia cada uma de uma variável de ambiente com o mesmo nome."})
	}

	// O que não cabe na caixa
	if len(browser) > 0 && rec.Kind != "dockerfile" {
		add(Item{Key: "container_limits", Title: "Recurso que não vem na receita", Status: "warn",
			Detail:     "O app chama um navegador automático (" + where(browser) + "), usado por exemplo para gerar PDF. A receita automática não traz navegador; essa parte não vai funcionar no servidor.",
			ClaudeText: "Este app vai rodar num contêiner Linux sem navegador. Escreva um Dockerfile que instale o que " + where(browser) + " precisa (Node, Playwright e o Chromium com as dependências do sistema), ou troque a geração de PDF por uma biblioteca Python que não precise de navegador."})
	}

	r.Ready = true
	for _, it := range r.Items {
		if it.Status == "missing" || it.Status == "warn" && !it.Acked {
			r.Ready = false
		}
	}
	return r, nil
}

// where lists up to three places, "app.py:12, db.py:3 e mais 2".
func where(h []hit) string {
	var out []string
	for i, x := range h {
		if i == 3 {
			out = append(out, fmt.Sprintf("e mais %d", len(h)-3))
			break
		}
		out = append(out, fmt.Sprintf("%s:%d", x.file, x.line))
	}
	return strings.Join(out, ", ")
}
```

- [ ] **Step 5: Rodar** — `go test ./internal/shared/readiness/ ./internal/shared/recipes/` → PASS.

- [ ] **Step 6: Commit** — `git add backend/internal/shared/readiness backend/testdata/shared && git commit -m "Shared apps: readiness check with text for Claude"`

---

# Fase 4 — Publicar

### Task 9: Área de montagem e espera pela saúde

**Files:**
- Create: `backend/internal/shared/stage/stage.go`, `backend/internal/shared/stage/stage_test.go`
- Create: `backend/internal/shared/health/health.go`, `backend/internal/shared/health/health_test.go`

**Interfaces:**
- Consumes: `versions.Folder`, `versions.Included(rel, outs)`, `recipes.Recipe`, `recipes.DockerfileName`.
- Produces:

```go
package stage
var ErrTooBig = errors.New("a pasta do app passa de 500 MB")
// Copy copies the app folder into dir (which must not exist), skipping what
// versions.Included rejects, .git, and what .dockerignore lists; then writes
// the recipe's extra files and Dockerfile (as recipes.DockerfileName).
// It returns the Dockerfile path relative to dir.
func Copy(src versions.Folder, dir string, outs []manifest.Output, rec recipes.Recipe) (dockerfile string, err error)

package health
// Wait polls url every 500 ms until it answers with a status below 500, or
// timeout passes. The error says "não respondeu" in plain Portuguese.
func Wait(ctx context.Context, url string, timeout time.Duration) error
// Check is one try with a 5 s limit.
func Check(ctx context.Context, url string) bool
```

- [ ] **Step 1: Testes que falham**

`stage_test.go`:

```go
package stage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopySkipsJunkAndOutputs(t *testing.T) {
	src := t.TempDir()
	write(t, src, "app.py", "import streamlit")
	write(t, src, ".git/config", "x")
	write(t, src, ".venv/lib/a.py", "x")
	write(t, src, "node_modules/a/index.js", "x")
	write(t, src, "saida/relatorio.xlsx", "x")
	write(t, src, "segredos/chave.txt", "x")
	write(t, src, ".dockerignore", "segredos/\n*.log\n")
	write(t, src, "debug.log", "x")
	dst := filepath.Join(t.TempDir(), "stage")
	rec := recipes.Recipe{Kind: "streamlit", Dockerfile: "FROM x\n", ExtraFiles: map[string]string{"requirements.txt": "streamlit\n"}}
	df, err := Copy(versions.LocalFolder{Root: src}, dst, []manifest.Output{{Path: "saida/*.xlsx"}}, rec)
	if err != nil {
		t.Fatal(err)
	}
	if df != recipes.DockerfileName {
		t.Fatalf("dockerfile %q", df)
	}
	for _, want := range []string{"app.py", "requirements.txt", recipes.DockerfileName} {
		if _, err := os.Stat(filepath.Join(dst, want)); err != nil {
			t.Errorf("missing %s", want)
		}
	}
	for _, gone := range []string{".git", ".venv", "node_modules", "saida/relatorio.xlsx", "segredos/chave.txt", "debug.log"} {
		if _, err := os.Stat(filepath.Join(dst, gone)); err == nil {
			t.Errorf("should skip %s", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(src, recipes.DockerfileName)); err == nil {
		t.Fatal("wrote into the app folder")
	}
}

func TestCopyOwnDockerfile(t *testing.T) {
	src := t.TempDir()
	write(t, src, "Dockerfile", "FROM y\n")
	dst := filepath.Join(t.TempDir(), "stage")
	df, err := Copy(versions.LocalFolder{Root: src}, dst, nil, recipes.Recipe{Kind: "dockerfile"})
	if err != nil || df != "Dockerfile" {
		t.Fatalf("%q %v", df, err)
	}
}
```

Ajustar o campo de `manifest.Output` (ler o struct; pode ser `Pattern` em vez de `Path`).

`health_test.go`:

```go
package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitUntilHealthy(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			w.WriteHeader(502)
			return
		}
		w.WriteHeader(404) // < 500 conta como "respondeu"
	}))
	defer srv.Close()
	if err := Wait(context.Background(), srv.URL, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWaitGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	err := Wait(context.Background(), srv.URL, 1200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "não respondeu") {
		t.Fatalf("err %v", err)
	}
}
```

- [ ] **Step 2: Rodar** — `go test ./internal/shared/stage/ ./internal/shared/health/` → FAIL.

- [ ] **Step 3: Implementar**

`stage.go`:

```go
// Package stage copies an app folder into the Hub's build area, so builds
// never touch (or write into) the app folder itself.
package stage

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

var ErrTooBig = errors.New("a pasta do app passa de 500 MB")

const maxBytes = 500 << 20

func Copy(src versions.Folder, dir string, outs []manifest.Output, rec recipes.Recipe) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	ignore := readIgnore(src)
	var total int64
	err := src.Walk(func(rel string, isDir bool, size int64, _ time.Time) error {
		if path.Base(rel) == ".git" || ignored(ignore, rel, isDir) {
			if isDir {
				return fs.SkipDir
			}
			return nil
		}
		if isDir {
			return nil
		}
		if !versions.Included(rel, outs) {
			return nil
		}
		if total += size; total > maxBytes {
			return ErrTooBig
		}
		r, err := src.Open(rel)
		if err != nil {
			return err
		}
		defer r.Close()
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return err
		}
		w, err := os.Create(dst)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
			w.Close()
			return err
		}
		return w.Close()
	})
	if err != nil {
		return "", err
	}
	for name, body := range rec.ExtraFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			return "", err
		}
	}
	if rec.Kind == "dockerfile" {
		return "Dockerfile", nil
	}
	return recipes.DockerfileName, os.WriteFile(filepath.Join(dir, recipes.DockerfileName), []byte(rec.Dockerfile), 0o600)
}

func readIgnore(src versions.Folder) []string {
	r, err := src.Open(".dockerignore")
	if err != nil {
		return nil
	}
	defer r.Close()
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "!") {
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(l, "/"), "/"))
		}
	}
	return out
}

// ignored is a simple .dockerignore: a pattern matches the path, its base
// name, or any parent folder. Negations ("!") are not supported.
func ignored(patterns []string, rel string, isDir bool) bool {
	for _, p := range patterns {
		if ok, _ := path.Match(p, rel); ok {
			return true
		}
		if ok, _ := path.Match(p, path.Base(rel)); ok {
			return true
		}
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}
```

`health.go`:

```go
// Package health waits for a shared app to answer on its port.
package health

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var ErrNoAnswer = errors.New("o app ligou, mas não respondeu")

var client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func Check(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode < 500
}

func Wait(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if Check(ctx, url) {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrNoAnswer
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
```

- [ ] **Step 4: Rodar** → PASS.

- [ ] **Step 5: Commit** — `git add backend/internal/shared/stage backend/internal/shared/health && git commit -m "Shared apps: build area copy and health wait"`

---

### Task 10: Serviço de implantação (`shared/deploy`)

O coração da entrega: testar, publicar, versão nova lado a lado, voltar, parar, ligar, dormir, deixar de compartilhar, cópias e restauração. Tudo o que muda o estado de um app compartilhado passa por aqui; o porteiro, o vigia e a API só chamam estes métodos.

**Files:**
- Create: `backend/internal/shared/deploy/deploy.go` (serviço, publicar, testar, voltar)
- Create: `backend/internal/shared/deploy/lifecycle.go` (ligar, dormir, parar, deixar de compartilhar)
- Create: `backend/internal/shared/deploy/backups.go` (cópia, restauração)
- Create: `backend/internal/shared/deploy/env.go` (variáveis da caixa, senha do banco no cofre)
- Create: `backend/internal/shared/deploy/deploy_test.go`, `backend/internal/shared/deploy/main_test.go`
- Create: `backend/internal/shared/deploy/deploytest/deploytest.go` (preparo exportado, reaproveitado pelo vigia)
- Modify: `backend/internal/shared/appdb/appdb.go` — `DBs` ganha `Name(appID int64) string` e `TestName(appID, depID int64) string` (o `Fake` devolve `app_<id>` e `app_<id>_test_<dep>`).

**Interfaces:**
- Consumes: `engine.Engine`, `appdb.DBs`, `*vault.Vault`, `stage.Copy`, `health.Wait`, `recipes.Detect`, `*sqlc.Queries` (consultas do Task 1), `manifest.Manifest` (`Shared.WithDefaults()`).
- Produces:

```go
package deploy

var (
	ErrBusy        = errors.New("Já existe uma publicação em andamento para este app.")
	ErrNoVault     = errors.New("Os apps compartilhados precisam da chave HUB_SECRET_KEY no servidor. Peça ao administrador para configurá-la.")
	ErrNoPrevious  = errors.New("Não há versão anterior para voltar.")
	ErrNotShared   = errors.New("Este app ainda não é compartilhado.")
	ErrBadSlug     = errors.New("O nome curto do app só pode ter letras minúsculas, números e hífens (sem dois hífens seguidos) para virar endereço.")
)

// MissingSettings lists required settings without a value.
type MissingSettings struct{ Names []string }
func (e MissingSettings) Error() string // "Falta preencher: E-mails dos administradores, Senha do e-mail."

type Options struct {
	DataDir       string        // SHARED_DATA_DIR
	Domain        string        // SHARED_APPS_DOMAIN, ex.: "localhost:8080"
	Scheme        string        // "http" ou "https" (do BASE_URL do Hub)
	HealthTimeout time.Duration // 90 s; testes usam 1 s
}

type Service struct { /* q, eng, dbs, vault, opt, spawn, mu, busy, ports, testPorts, now */ }

func New(q *sqlc.Queries, eng engine.Engine, dbs appdb.DBs, v *vault.Vault, opt Options) *Service
func (s *Service) SetSpawn(fn func(func()))          // testes: controla quando o trabalho em segundo plano roda
func (s *Service) SetNow(fn func() time.Time)

func (s *Service) Slug(appSlug string) (string, error) // valida e devolve o slug do endereço
func (s *Service) AppURL(slug string) string           // "http://notas.localhost:8080"
func (s *Service) TestURL(slug string) string          // "http://notas--teste.localhost:8080"
func (s *Service) Target(appID int64, test bool) (hostPort int, ok bool)

// Configure creates or updates the shared_apps row from the manifest (marking the app shared, status draft if new).
func (s *Service) Configure(ctx context.Context, app sqlc.App, m manifest.Manifest) (sqlc.SharedApp, error)
func (s *Service) Test(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64) (sqlc.SharedDeployment, error)
func (s *Service) Publish(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64) (sqlc.SharedDeployment, error)
func (s *Service) Rollback(ctx context.Context, appID int64, withData bool, by int64) error
func (s *Service) EndTest(ctx context.Context, depID int64) error

func (s *Service) Run(ctx context.Context, appID int64) error   // liga a versão no ar e espera a saúde (síncrono)
func (s *Service) Wake(appID int64)                              // Run em segundo plano, uma vez só por app
func (s *Service) Start(ctx context.Context, appID int64) error  // "Ligar"/"Religar": zera tentativas e chama Wake
func (s *Service) Sleep(ctx context.Context, appID int64) error  // dormir (sem uso)
func (s *Service) Stop(ctx context.Context, appID, by int64) error
func (s *Service) Unshare(ctx context.Context, appID, by int64) error
func (s *Service) Purge(ctx context.Context, appID int64) error  // 30 dias depois: apaga banco, cópias e a linha

func (s *Service) Backup(ctx context.Context, appID int64, kind string, depID, by *int64) (sqlc.SharedBackup, error)
func (s *Service) Restore(ctx context.Context, appID, backupID, by int64) error
func (s *Service) BackupPath(b sqlc.SharedBackup) string
```

Regras que os testes cobram:
- **Uma operação por app** (`busy` em memória + o índice único `shared_deployments_one_building`): `Test`, `Publish`, `Rollback`, `Restore` e `Unshare` devolvem `ErrBusy` se outra estiver em andamento.
- `Publish` grava a implantação (`building`) e devolve na hora; o resto roda em `spawn`: copia a pasta (`stage.Copy` para `DataDir/stage/<dep>`), se já há versão no ar e banco → cópia `pre_deploy` (`deployment_id` = a nova), monta (`Tag: localhost/apphub/<slug>:<dep>`), garante o banco, liga a caixa nova **ao lado** (`apphub-<slug>-<dep>`), espera a saúde (`HealthTimeout`), troca o `Target`, marca a nova `live` e a antiga `retired`, para e remove a caixa antiga (a imagem fica), apaga a imagem da implantação de antes da anterior, grava `current/previous`, status `running`, apaga a área de montagem e registra na linha do tempo "Publicou a versão N" (`InsertAppChange` com a pessoa; `manifest.Change{Field: "shared", Label: "Publicou a versão N"}`).
- Falha em qualquer passo → implantação `failed` com `error_message` em português e as últimas 40 linhas da montagem em `build_log`; a caixa nova (se ligou) é removida; **a versão no ar não muda**. Se era a primeira publicação, status do app volta para `draft`.
- `Test` faz o mesmo com `kind = test`, banco de teste vazio (`TestName`, apagado em `EndTest`), caixa `apphub-<slug>-teste-<dep>`, `expires_at = agora + 30 min`, status final `ready`, sem cópia e sem trocar a versão no ar. Um teste novo encerra o anterior do mesmo app. `Publish` encerra o teste em andamento.
- Variáveis da caixa: `PORT`, `BASE_URL` (= `AppURL`, ou `TestURL` no teste), `APPHUB_DATA_DIR=/data`, `DATABASE_URL` se `wants_db`; mais `app_config` (senhas decifradas pelo cofre; nomes começando com `_` são internos e não vão); mais o `default` do manifesto para o que não foi preenchido. Obrigatória sem valor → `MissingSettings` (devolvido na hora, antes de criar a implantação). A senha do banco fica em `app_config` com nome `_db_password` (`is_secret`), gerada na primeira vez com `appdb.NewPassword()`.
- Pasta que não se perde: `DataDir/apps/<app_id>/data` (teste: `DataDir/apps/<app_id>/test-data`, apagada no `EndTest`).
- `Vault == nil` → `ErrNoVault` em `Test`, `Publish`, `Run`.

- [ ] **Step 1: Ajustar `appdb.DBs`** — acrescentar `Name`/`TestName` à interface e ao `Fake` (`fmt.Sprintf("app_%d", id)`, `fmt.Sprintf("app_%d_test_%d", id, dep)`). `go test ./internal/shared/appdb/` continua passando.

- [ ] **Step 2: Ajudantes de teste** — pacote exportado `backend/internal/shared/deploy/deploytest/deploytest.go`, usado pelos testes do `deploy` (pacote externo `deploy_test`, para não haver ciclo de imports) e, no Task 12, pelos do vigia:

```go
// Package deploytest prepares a shared app on the shared test database,
// with the fake engine and fake databases, for deploy and supervisor tests.
package deploytest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/sharedtest"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

type Env struct {
	S   *deploy.Service
	Q   *sqlc.Queries
	Eng *engine.Fake
	DBs *appdb.Fake
	App sqlc.App
	M   manifest.Manifest

	key     string
	dataDir string
}

// testdata finds backend/testdata/shared from this file, wherever the test runs.
func testdata(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "../../../../testdata/shared", name)
}

// Setup resets the test database and registers a Streamlit app (with a
// database) whose folder is a copy of testdata/shared/streamlit.
func Setup(t *testing.T, pool *pgxpool.Pool) *Env {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, sharedtest.Reset); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	CopyDir(t, testdata("streamlit"), dir)
	m := manifest.Manifest{ManifestVersion: 1, ID: "notas", Name: "Notas", Kind: manifest.KindWeb, Entrypoint: "app.py",
		Shared: &manifest.Shared{Enabled: true, Database: "postgres"}}
	raw, _ := json.Marshal(m)
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO apps (slug, name, kind, area, area_id, folder_path, owner_id, manifest, created_by)
		VALUES ('notas', 'Notas', 'web', 'Research', 1, $1, 1, $2, 1) RETURNING id`, dir, raw).Scan(&id); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(pool)
	app, err := q.GetApp(ctx, id) // conferir o nome e os parâmetros reais da consulta
	if err != nil {
		t.Fatal(err)
	}
	e := &Env{Q: q, Eng: engine.NewFake(), DBs: appdb.NewFake(), App: app, M: m, key: vault.NewKey(), dataDir: t.TempDir()}
	e.S = e.NewService()
	if _, err := e.S.Configure(ctx, app, m); err != nil {
		t.Fatal(err)
	}
	return e
}

// NewService is a fresh service on the same database, engine, databases and
// vault key, with nothing in memory: what the Hub looks like after a restart.
func (e *Env) NewService() *deploy.Service {
	v, _ := vault.New(e.key)
	return deploy.New(e.Q, e.Eng, e.DBs, v, deploy.Options{DataDir: e.dataDir, Domain: "localhost:8080", Scheme: "http", HealthTimeout: time.Second})
}

// NewServiceWithoutVault is the Hub started without HUB_SECRET_KEY.
func (e *Env) NewServiceWithoutVault() *deploy.Service {
	return deploy.New(e.Q, e.Eng, e.DBs, nil, deploy.Options{DataDir: e.dataDir, Domain: "localhost:8080", Scheme: "http", HealthTimeout: time.Second})
}

func CopyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// WaitDeployment polls until the deployment leaves "building".
func (e *Env) WaitDeployment(t *testing.T, id int64) sqlc.SharedDeployment {
	t.Helper()
	for i := 0; i < 100; i++ {
		d, err := e.Q.GetDeployment(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if d.Status != "building" {
			return d
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("deployment never finished")
	return sqlc.SharedDeployment{}
}

func (e *Env) Shared(t *testing.T) sqlc.SharedApp {
	t.Helper()
	sa, err := e.Q.GetSharedApp(context.Background(), e.App.ID)
	if err != nil {
		t.Fatal(err)
	}
	return sa
}

func (e *Env) Publish(t *testing.T) sqlc.SharedDeployment {
	t.Helper()
	d, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	return e.WaitDeployment(t, d.ID)
}

func (e *Env) StartTest(t *testing.T) sqlc.SharedDeployment {
	t.Helper()
	d, err := e.S.Test(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	return e.WaitDeployment(t, d.ID)
}

func Itoa(n int64) string { return strconv.FormatInt(n, 10) }
```

E `backend/internal/shared/deploy/main_test.go`:

```go
package deploy_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/shared/sharedtest"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	var err error
	pool, err = sharedtest.Pool(context.Background())
	if err != nil {
		panic(err)
	}
	if pool == nil {
		os.Exit(0) // sem banco: os testes deste pacote não rodam
	}
	os.Exit(m.Run())
}

func ptr[T any](v T) *T { return &v }
```

- [ ] **Step 3: Testes que falham** (`deploy_test.go`)

```go
package deploy_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/jackc/pgx/v5/pgtype"
)

var itoa = deploytest.Itoa

func setup(t *testing.T) *deploytest.Env { return deploytest.Setup(t, pool) }

func TestPublishFirstTime(t *testing.T) {
	e := setup(t)
	d := e.Publish(t)
	if d.Status != "live" {
		t.Fatalf("deployment %s: %s\n%s", d.Status, d.ErrorMessage, d.BuildLog)
	}
	sa := e.Shared(t)
	if sa.Status != "running" || !sa.CurrentDeploymentID.Valid || sa.CurrentDeploymentID.Int64 != d.ID || sa.DbName != "app_"+itoa(e.App.ID) {
		t.Fatalf("%+v", sa)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "apphub-notas-"+itoa(d.ID) {
		t.Fatalf("running %v", got)
	}
	var env map[string]string
	for _, c := range e.Eng.Containers {
		env = c.Spec.Env
	}
	if env["PORT"] != "8501" || env["BASE_URL"] != "http://notas.localhost:8080" || env["APPHUB_DATA_DIR"] != "/data" ||
		!strings.HasPrefix(env["DATABASE_URL"], "postgres://app_") {
		t.Fatalf("env %v", env)
	}
	if _, ok := e.S.Target(e.App.ID, false); !ok {
		t.Fatal("no target")
	}
	if !strings.Contains(e.Eng.Images[d.ImageID].Tag, "localhost/apphub/notas:") {
		t.Fatalf("tag %+v", e.Eng.Images)
	}
}

func TestDeployOneAtATime(t *testing.T) {
	e := setup(t)
	release := make(chan struct{})
	e.S.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	first, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.S.Publish(context.Background(), e.App, e.M, 1); !errors.Is(err, deploy.ErrBusy) {
		t.Fatalf("second publish err %v", err)
	}
	close(release)
	e.WaitDeployment(t, first.ID)
	if _, err := e.S.Publish(context.Background(), e.App, e.M, 1); err != nil {
		t.Fatalf("after finishing, publish again: %v", err)
	}
}

func TestNewVersionUnhealthyKeepsOld(t *testing.T) {
	e := setup(t)
	old := e.Publish(t)
	oldPort, _ := e.S.Target(e.App.ID, false)
	e.Eng.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	d := e.Publish(t)
	if d.Status != "failed" || !strings.Contains(d.ErrorMessage, "não respondeu na porta 8501") {
		t.Fatalf("%s %q", d.Status, d.ErrorMessage)
	}
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 != old.ID || sa.Status != "running" {
		t.Fatalf("%+v", sa)
	}
	if port, _ := e.S.Target(e.App.ID, false); port != oldPort {
		t.Fatal("target moved to the broken version")
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "apphub-notas-"+itoa(old.ID) {
		t.Fatalf("running %v", got)
	}
}

func TestNewVersionSwapsAndBacksUp(t *testing.T) {
	e := setup(t)
	old := e.Publish(t)
	d := e.Publish(t)
	if d.Status != "live" {
		t.Fatalf("%s %s", d.Status, d.ErrorMessage)
	}
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 != d.ID || sa.PreviousDeploymentID.Int64 != old.ID {
		t.Fatalf("%+v", sa)
	}
	if got := e.Eng.Running(); len(got) != 1 || got[0] != "apphub-notas-"+itoa(d.ID) {
		t.Fatalf("old still running: %v", got)
	}
	if _, err := e.Q.LatestPreDeployBackup(context.Background(), sqlc.LatestPreDeployBackupParams{AppID: e.App.ID, DeploymentID: pgtype.Int8{Int64: d.ID, Valid: true}}); err != nil {
		t.Fatalf("no pre_deploy backup: %v", err)
	}
	if _, ok := e.Eng.Images[old.ImageID]; !ok {
		t.Fatal("previous image removed; rollback needs it")
	}
}

func TestBuildFailureKeepsLogTail(t *testing.T) {
	e := setup(t)
	e.Eng.BuildLog = strings.Repeat("linha\n", 100)
	e.Eng.BuildErr = errors.New("ERROR: No matching distribution found for pandsa")
	d := e.Publish(t)
	if d.Status != "failed" || !strings.Contains(d.ErrorMessage, "montar") || strings.Count(d.BuildLog, "\n") > 41 {
		t.Fatalf("%q (%d lines)", d.ErrorMessage, strings.Count(d.BuildLog, "\n"))
	}
	if e.Shared(t).Status != "draft" {
		t.Fatal("first publish failed: app should stay draft")
	}
}

func TestRollbackWithData(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	old := e.Publish(t)
	db := e.Shared(t).DbName
	e.DBs.Data[db] = "v1"
	d := e.Publish(t)
	e.DBs.Data[db] = "v2"
	if err := e.S.Rollback(ctx, e.App.ID, true, 1); err != nil {
		t.Fatal(err)
	}
	sa := e.Shared(t)
	if sa.CurrentDeploymentID.Int64 != old.ID || sa.PreviousDeploymentID.Int64 != d.ID || e.DBs.Data[db] != "v1" {
		t.Fatalf("%+v data=%q", sa, e.DBs.Data[db])
	}
	if err := e.S.Rollback(ctx, e.App.ID, false, 1); err != nil { // volta de novo para a nova, sem mexer nos dados
		t.Fatal(err)
	}
	if e.DBs.Data[db] != "v1" {
		t.Fatal("rollback without data changed the data")
	}
}

func TestRollbackWithoutPrevious(t *testing.T) {
	e := setup(t)
	e.Publish(t)
	if err := e.S.Rollback(context.Background(), e.App.ID, false, 1); !errors.Is(err, deploy.ErrNoPrevious) {
		t.Fatalf("err %v", err)
	}
}

func TestMissingRequiredSetting(t *testing.T) {
	e := setup(t)
	e.M.Shared.Env = []manifest.EnvVar{{Name: "ADMIN_EMAILS", Label: "E-mails dos administradores", Required: true}}
	_, err := e.S.Publish(context.Background(), e.App, e.M, 1)
	var miss deploy.MissingSettings
	if !errors.As(err, &miss) || miss.Error() != "Falta preencher: E-mails dos administradores." {
		t.Fatalf("err %v", err)
	}
}

func TestNoVault(t *testing.T) {
	e := setup(t)
	if _, err := e.NewServiceWithoutVault().Publish(context.Background(), e.App, e.M, 1); !errors.Is(err, deploy.ErrNoVault) {
		t.Fatalf("err %v", err)
	}
}

func TestTestDeploymentIsSeparate(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	d, err := e.S.Test(ctx, e.App, e.M, 1)
	if err != nil {
		t.Fatal(err)
	}
	d = e.WaitDeployment(t, d.ID)
	if d.Status != "ready" || d.TestDbName == "" || e.DBs.DBs[d.TestDbName] == "" {
		t.Fatalf("%+v", d)
	}
	if _, ok := e.S.Target(e.App.ID, true); !ok {
		t.Fatal("no test target")
	}
	if e.Shared(t).Status != "draft" {
		t.Fatal("test changed the app status")
	}
	if err := e.S.EndTest(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.DBs.DBs[d.TestDbName]; ok || len(e.Eng.Running()) != 0 {
		t.Fatal("test leftovers")
	}
}

func TestStopSleepStartUnshare(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	if err := e.S.Sleep(ctx, e.App.ID); err != nil || e.Shared(t).Status != "sleeping" || len(e.Eng.Running()) != 0 {
		t.Fatalf("sleep %v %s", err, e.Shared(t).Status)
	}
	if err := e.S.Run(ctx, e.App.ID); err != nil || e.Shared(t).Status != "running" {
		t.Fatalf("run %v", err)
	}
	if err := e.S.Stop(ctx, e.App.ID, 1); err != nil || e.Shared(t).Status != "stopped" || !e.Shared(t).StoppedBy.Valid {
		t.Fatalf("stop %v", err)
	}
	if err := e.S.Unshare(ctx, e.App.ID, 1); err != nil {
		t.Fatal(err)
	}
	sa := e.Shared(t)
	if !sa.UnsharedAt.Valid || len(e.Eng.Running()) != 0 || len(e.Eng.Images) != 0 {
		t.Fatalf("%+v images=%d", sa, len(e.Eng.Images))
	}
	if _, ok := e.DBs.DBs[sa.DbName]; !ok {
		t.Fatal("the database must stay for 30 days")
	}
}

func TestBackupAndRestore(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.Publish(t)
	db := e.Shared(t).DbName
	e.DBs.Data[db] = "antes"
	b, err := e.S.Backup(ctx, e.App.ID, "manual", nil, ptr(int64(1)))
	if err != nil || b.SizeBytes == 0 {
		t.Fatalf("%+v %v", b, err)
	}
	e.DBs.Data[db] = "depois"
	if err := e.S.Restore(ctx, e.App.ID, b.ID, 1); err != nil {
		t.Fatal(err)
	}
	if e.DBs.Data[db] != "antes" || e.Shared(t).Status != "running" {
		t.Fatalf("data %q status %s", e.DBs.Data[db], e.Shared(t).Status)
	}
	list, _ := e.Q.ListBackups(ctx, e.App.ID)
	if len(list) != 2 || list[0].Kind != "pre_restore" {
		t.Fatalf("%+v", list)
	}
}
```

Os nomes dos campos gerados pelo sqlc (`DbName`, `TestDbName`, `StoppedBy`, `UnsharedAt`…) seguem o padrão do `sqlc.yaml`; confira em `models.go` e ajuste o teste se diferirem.

- [ ] **Step 4: Rodar** — `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/shared/deploy/` → FAIL (pacote vazio).

- [ ] **Step 5: Implementar `deploy.go`** (serviço, publicar, testar, voltar)

```go
// Package deploy runs shared apps: test, publish (side by side), roll back,
// start, sleep, stop, unshare, copies. Everything that changes a shared
// app's state goes through here.
package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/health"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/shared/stage"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Service struct {
	q     *sqlc.Queries
	eng   engine.Engine
	dbs   appdb.DBs
	vault *vault.Vault
	opt   Options
	spawn func(func())
	now   func() time.Time

	mu        sync.Mutex
	busy      map[int64]bool
	waking    map[int64]bool
	ports     map[int64]int // app → porta da versão no ar
	testPorts map[int64]int // app → porta da caixa de teste
}

func New(q *sqlc.Queries, eng engine.Engine, dbs appdb.DBs, v *vault.Vault, opt Options) *Service {
	if opt.HealthTimeout == 0 {
		opt.HealthTimeout = 90 * time.Second
	}
	if opt.Scheme == "" {
		opt.Scheme = "http"
	}
	return &Service{q: q, eng: eng, dbs: dbs, vault: v, opt: opt,
		spawn: func(f func()) { go f() }, now: time.Now,
		busy: map[int64]bool{}, waking: map[int64]bool{}, ports: map[int64]int{}, testPorts: map[int64]int{}}
}

func (s *Service) SetSpawn(fn func(func()))      { s.spawn = fn }
func (s *Service) SetNow(fn func() time.Time)     { s.now = fn }

func (s *Service) Slug(appSlug string) (string, error) {
	if len(appSlug) > 40 || !slugRe.MatchString(appSlug) {
		return "", ErrBadSlug
	}
	return appSlug, nil
}

func (s *Service) AppURL(slug string) string  { return s.opt.Scheme + "://" + slug + "." + s.opt.Domain }
func (s *Service) TestURL(slug string) string { return s.opt.Scheme + "://" + slug + "--teste." + s.opt.Domain }

func (s *Service) Target(appID int64, test bool) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.ports
	if test {
		m = s.testPorts
	}
	p, ok := m[appID]
	return p, ok
}

func (s *Service) setTarget(appID int64, test bool, port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.ports
	if test {
		m = s.testPorts
	}
	if port == 0 {
		delete(m, appID)
	} else {
		m[appID] = port
	}
}

// lock marks the app busy; the caller must call the returned func when done.
func (s *Service) lock(appID int64) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy[appID] {
		return nil, ErrBusy
	}
	s.busy[appID] = true
	return func() { s.mu.Lock(); delete(s.busy, appID); s.mu.Unlock() }, nil
}

func (s *Service) Configure(ctx context.Context, app sqlc.App, m manifest.Manifest) (sqlc.SharedApp, error) {
	slug, err := s.Slug(app.Slug)
	if err != nil {
		return sqlc.SharedApp{}, err
	}
	sh := manifest.Shared{Enabled: true}
	if m.Shared != nil {
		sh = *m.Shared
	}
	sh = sh.WithDefaults()
	return s.q.UpsertSharedApp(ctx, sqlc.UpsertSharedAppParams{
		AppID: app.ID, Slug: slug, AlwaysOn: sh.AlwaysOn, IdleMinutes: int32(sh.IdleMin), MemoryMb: int32(sh.MemoryMB),
		Cpus: numeric(sh.CPUs), HealthPath: sh.HealthPath, Port: int32(sh.Port), WantsDb: sh.Database == "postgres",
	})
}

type job struct {
	app  sqlc.App
	sa   sqlc.SharedApp
	m    manifest.Manifest
	dep  sqlc.SharedDeployment
	rec  recipes.Recipe
	env  map[string]string
	by   int64
	test bool
}

func (s *Service) Publish(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64) (sqlc.SharedDeployment, error) {
	return s.begin(ctx, app, m, by, false)
}

func (s *Service) Test(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64) (sqlc.SharedDeployment, error) {
	return s.begin(ctx, app, m, by, true)
}

// begin checks everything that can be checked right away (so the person sees
// the error on the button), records the deployment and runs the rest in the
// background.
func (s *Service) begin(ctx context.Context, app sqlc.App, m manifest.Manifest, by int64, test bool) (sqlc.SharedDeployment, error) {
	if s.vault == nil {
		return sqlc.SharedDeployment{}, ErrNoVault
	}
	if err := s.eng.Available(ctx); err != nil {
		return sqlc.SharedDeployment{}, err
	}
	unlock, err := s.lock(app.ID)
	if err != nil {
		return sqlc.SharedDeployment{}, err
	}
	sa, err := s.Configure(ctx, app, m)
	if err != nil {
		unlock()
		return sqlc.SharedDeployment{}, err
	}
	rec, err := recipes.Detect(versions.LocalFolder{Root: app.FolderPath}, m)
	if err != nil {
		unlock()
		return sqlc.SharedDeployment{}, err
	}
	if rec.Kind == "" {
		unlock()
		return sqlc.SharedDeployment{}, errors.New("O Hub não sabe ligar este app. Veja a checagem para o que falta.")
	}
	if rec.Port == 0 {
		rec.Port = int(sa.Port)
	}
	if missing, err := s.missingSettings(ctx, app.ID, m); err != nil || len(missing) > 0 {
		unlock()
		if err != nil {
			return sqlc.SharedDeployment{}, err
		}
		return sqlc.SharedDeployment{}, MissingSettings{Names: missing}
	}
	kind := "live"
	var expires pgtype.Timestamptz
	if test {
		kind = "test"
		expires = pgtype.Timestamptz{Time: s.now().Add(30 * time.Minute), Valid: true}
	}
	var versionID pgtype.Int8
	if v, err := s.q.LatestAppVersion(ctx, app.ID); err == nil {
		versionID = pgtype.Int8{Int64: v.ID, Valid: true}
	}
	recipeText := rec.Dockerfile
	dep, err := s.q.InsertDeployment(ctx, sqlc.InsertDeploymentParams{AppID: app.ID, VersionID: versionID, Kind: kind,
		Recipe: recipeText, CreatedBy: pgtype.Int8{Int64: by, Valid: true}, ExpiresAt: expires})
	if err != nil {
		unlock()
		if isUniqueViolation(err) {
			return sqlc.SharedDeployment{}, ErrBusy
		}
		return sqlc.SharedDeployment{}, err
	}
	if !test && !sa.CurrentDeploymentID.Valid {
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: app.ID, Status: "starting"})
	}
	j := job{app: app, sa: sa, m: m, dep: dep, rec: rec, by: by, test: test}
	s.spawn(func() {
		defer unlock()
		s.runJob(context.Background(), j)
	})
	return dep, nil
}

// failure is what went wrong, in Portuguese, plus the log for "Ver detalhes".
type failure struct {
	msg string
	log string
}

func (s *Service) runJob(ctx context.Context, j job) {
	var f *failure
	if j.test {
		f = s.runTest(ctx, j)
	} else {
		f = s.runPublish(ctx, j)
	}
	if f == nil {
		return
	}
	slog.Warn("shared deploy failed", "app", j.app.ID, "deployment", j.dep.ID, "msg", f.msg)
	_ = s.q.FinishDeployment(ctx, sqlc.FinishDeploymentParams{ID: j.dep.ID, Status: "failed",
		BuildLog: lastLines(f.log, 40), ErrorMessage: f.msg})
	if !j.test && !j.sa.CurrentDeploymentID.Valid {
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: j.app.ID, Status: "draft", StatusMessage: f.msg})
	}
}

// build copies the folder and builds the image. It returns the image ID and the log.
func (s *Service) build(ctx context.Context, j job) (string, string, *failure) {
	dir := filepath.Join(s.opt.DataDir, "stage", strconv.FormatInt(j.dep.ID, 10))
	defer os.RemoveAll(dir)
	df, err := stage.Copy(versions.LocalFolder{Root: j.app.FolderPath}, dir, j.m.Outputs, j.rec)
	if errors.Is(err, stage.ErrTooBig) {
		return "", "", &failure{msg: "A pasta do app passa de 500 MB. Tire arquivos grandes que não são do app (ou liste-os no .dockerignore)."}
	}
	if err != nil {
		return "", "", &failure{msg: "Não foi possível ler a pasta do app. Confira se ela existe e está sincronizada.", log: err.Error()}
	}
	labels := map[string]string{engine.Label: strconv.FormatInt(j.app.ID, 10)}
	img, log, err := s.eng.Build(ctx, engine.BuildSpec{ContextDir: dir, DockerfilePath: df,
		Tag: fmt.Sprintf("localhost/apphub/%s:%d", j.sa.Slug, j.dep.ID), Labels: labels})
	if err != nil {
		return "", log, &failure{msg: "Não foi possível montar o app. " + explainBuild(log+"\n"+err.Error()), log: log + "\n" + err.Error()}
	}
	return img, log, nil
}

// start starts a container for the deployment and waits for health. On
// failure the container is removed.
func (s *Service) start(ctx context.Context, j job, img, dbName string) (string, int, *failure) {
	env, err := s.env(ctx, j.app.ID, j.m, j.sa, dbName, j.rec.Port, j.test)
	if err != nil {
		return "", 0, &failure{msg: "Não foi possível preparar as configurações do app.", log: err.Error()}
	}
	name := fmt.Sprintf("apphub-%s-%d", j.sa.Slug, j.dep.ID)
	data := filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(j.app.ID, 10), "data")
	if j.test {
		name = fmt.Sprintf("apphub-%s-teste-%d", j.sa.Slug, j.dep.ID)
		data = filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(j.app.ID, 10), "test-data")
	}
	cid, err := s.eng.Start(ctx, engine.StartSpec{Name: name, Image: img, Env: env, Port: j.rec.Port,
		MemoryMB: int(j.sa.MemoryMb), CPUs: float(j.sa.Cpus), DataDir: data,
		Labels: map[string]string{engine.Label: strconv.FormatInt(j.app.ID, 10), "app-hub.deployment": strconv.FormatInt(j.dep.ID, 10)}})
	if err != nil {
		return "", 0, &failure{msg: "Não foi possível ligar o app no servidor.", log: err.Error()}
	}
	st, _ := s.eng.Inspect(ctx, cid)
	if err == nil && st.HostPort != 0 {
		err = health.Wait(ctx, fmt.Sprintf("http://127.0.0.1:%d%s", st.HostPort, j.sa.HealthPath), s.opt.HealthTimeout)
	} else {
		err = health.ErrNoAnswer
	}
	if err != nil {
		logs, _ := s.eng.Logs(ctx, cid, 40)
		_ = s.eng.Remove(ctx, cid)
		return "", 0, &failure{msg: fmt.Sprintf("O app ligou, mas não respondeu na porta %d. Confira se ele escuta em 0.0.0.0 na porta da variável PORT.", j.rec.Port), log: logs}
	}
	return cid, st.HostPort, nil
}

func (s *Service) runPublish(ctx context.Context, j job) *failure {
	if t, err := s.q.ListOpenTests(ctx, j.app.ID); err == nil { // encerra o teste em andamento
		for _, d := range t {
			_ = s.EndTest(ctx, d.ID)
		}
	}
	live := j.sa.CurrentDeploymentID.Valid
	if live && j.sa.WantsDb && j.sa.DbName != "" {
		depID := j.dep.ID
		if _, err := s.Backup(ctx, j.app.ID, "pre_deploy", &depID, &j.by); err != nil {
			return &failure{msg: "Não foi possível fazer a cópia de segurança antes de publicar. Nada mudou no app.", log: err.Error()}
		}
	}
	img, log, f := s.build(ctx, j)
	if f != nil {
		return f
	}
	dbName := ""
	if j.sa.WantsDb {
		dbName = s.dbs.Name(j.app.ID)
		pw, err := s.dbPassword(ctx, j.app.ID)
		if err == nil {
			err = s.dbs.Ensure(ctx, dbName, pw)
		}
		if err != nil {
			_ = s.eng.RemoveImage(ctx, img)
			return &failure{msg: "Não foi possível preparar o banco do app.", log: err.Error()}
		}
		_ = s.q.SetSharedDBName(ctx, sqlc.SetSharedDBNameParams{AppID: j.app.ID, DbName: dbName})
	}
	cid, port, f := s.start(ctx, j, img, dbName)
	if f != nil {
		_ = s.eng.RemoveImage(ctx, img)
		f.log = log + "\n" + f.log
		return f
	}
	// Troca: o porteiro passa a mandar para a nova, depois a antiga para.
	s.setTarget(j.app.ID, false, port)
	_ = s.q.FinishDeployment(ctx, sqlc.FinishDeploymentParams{ID: j.dep.ID, Status: "live", ImageID: img, ContainerID: cid, BuildLog: lastLines(log, 40)})
	var prev pgtype.Int8
	if live {
		old, err := s.q.GetDeployment(ctx, j.sa.CurrentDeploymentID.Int64)
		if err == nil {
			_ = s.eng.Remove(ctx, old.ContainerID)
			_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: old.ID, Status: "retired"})
			prev = pgtype.Int8{Int64: old.ID, Valid: true}
		}
		if j.sa.PreviousDeploymentID.Valid { // só a anterior guarda imagem
			if older, err := s.q.GetDeployment(ctx, j.sa.PreviousDeploymentID.Int64); err == nil {
				_ = s.eng.RemoveImage(ctx, older.ImageID)
			}
		}
	}
	_ = s.q.SetSharedDeployments(ctx, sqlc.SetSharedDeploymentsParams{AppID: j.app.ID,
		CurrentID: pgtype.Int8{Int64: j.dep.ID, Valid: true}, PreviousID: prev})
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: j.app.ID, Status: "running"})
	_ = s.q.SetSharedAccessNow(ctx, j.app.ID)
	s.event(ctx, j.app.ID, &j.by, "Publicou "+s.versionLabel(ctx, j.dep))
	return nil
}

func (s *Service) runTest(ctx context.Context, j job) *failure {
	if t, err := s.q.ListOpenTests(ctx, j.app.ID); err == nil {
		for _, d := range t {
			if d.ID != j.dep.ID {
				_ = s.EndTest(ctx, d.ID)
			}
		}
	}
	img, log, f := s.build(ctx, j)
	if f != nil {
		return f
	}
	dbName := ""
	if j.sa.WantsDb {
		dbName = s.dbs.TestName(j.app.ID, j.dep.ID)
		if err := s.dbs.Ensure(ctx, dbName, appdb.NewPassword()); err != nil {
			_ = s.eng.RemoveImage(ctx, img)
			return &failure{msg: "Não foi possível preparar o banco de teste.", log: err.Error()}
		}
		_ = s.q.SetDeploymentTestDB(ctx, sqlc.SetDeploymentTestDBParams{ID: j.dep.ID, TestDbName: dbName})
	}
	cid, port, f := s.start(ctx, j, img, dbName)
	if f != nil {
		_ = s.eng.RemoveImage(ctx, img)
		if dbName != "" {
			_ = s.dbs.Drop(ctx, dbName)
		}
		f.log = log + "\n" + f.log
		return f
	}
	s.setTarget(j.app.ID, true, port)
	_ = s.q.FinishDeployment(ctx, sqlc.FinishDeploymentParams{ID: j.dep.ID, Status: "ready", ImageID: img, ContainerID: cid, BuildLog: lastLines(log, 40)})
	return nil
}

func (s *Service) EndTest(ctx context.Context, depID int64) error {
	d, err := s.q.GetDeployment(ctx, depID)
	if err != nil {
		return err
	}
	if d.ContainerID != "" {
		_ = s.eng.Remove(ctx, d.ContainerID)
	}
	if d.ImageID != "" {
		_ = s.eng.RemoveImage(ctx, d.ImageID)
	}
	if d.TestDbName != "" {
		if err := s.dbs.Drop(ctx, d.TestDbName); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(d.AppID, 10), "test-data"))
	s.setTarget(d.AppID, true, 0)
	return s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: d.ID, Status: "retired"})
}

func (s *Service) Rollback(ctx context.Context, appID int64, withData bool, by int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	if !sa.PreviousDeploymentID.Valid || !sa.CurrentDeploymentID.Valid {
		return ErrNoPrevious
	}
	cur, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil {
		return err
	}
	prev, err := s.q.GetDeployment(ctx, sa.PreviousDeploymentID.Int64)
	if err != nil {
		return err
	}
	if withData {
		b, err := s.q.LatestPreDeployBackup(ctx, sqlc.LatestPreDeployBackupParams{AppID: appID, DeploymentID: pgtype.Int8{Int64: cur.ID, Valid: true}})
		if err != nil {
			return errors.New("Não há cópia de segurança de antes desta versão para voltar os dados.")
		}
		if err := s.restoreFile(ctx, sa, b, by); err != nil {
			return err
		}
	}
	app, err := s.q.GetApp(ctx, appID)
	if err != nil {
		return err
	}
	m := manifestOf(app)
	rec, _ := recipes.Detect(versions.LocalFolder{Root: app.FolderPath}, m)
	if rec.Port == 0 {
		rec.Port = int(sa.Port)
	}
	j := job{app: app, sa: sa, m: m, dep: prev, rec: rec, by: by}
	cid, port, f := s.start(ctx, j, prev.ImageID, sa.DbName)
	if f != nil {
		return errors.New(f.msg)
	}
	s.setTarget(appID, false, port)
	_ = s.eng.Remove(ctx, cur.ContainerID)
	_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: prev.ID, ContainerID: cid})
	_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: prev.ID, Status: "live"})
	_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: cur.ID, Status: "retired"})
	_ = s.q.SetSharedDeployments(ctx, sqlc.SetSharedDeploymentsParams{AppID: appID,
		CurrentID: pgtype.Int8{Int64: prev.ID, Valid: true}, PreviousID: pgtype.Int8{Int64: cur.ID, Valid: true}})
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "running"})
	label := "Voltou para " + s.versionLabel(ctx, prev)
	if withData {
		label += " (com os dados de antes)"
	}
	s.event(ctx, appID, &by, label)
	return nil
}
```

Cada implantação guarda a porta da caixa (`shared_deployments.port`, gravada por `FinishDeployment` com `Port: int32(j.rec.Port)`): na `Rollback`, troque o `rec.Port` da receita de hoje por `prev.Port` quando ele não for zero, porque a porta pode ter mudado entre as versões. Os três `FinishDeployment` do Step 5 passam `Port: int32(j.rec.Port)`.

Ajudantes (no fim de `deploy.go`):

```go
func numeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', 2, 64))
	return n
}

func float(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func isUniqueViolation(err error) bool { return strings.Contains(err.Error(), "SQLSTATE 23505") }

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// explainBuild recognizes common build errors and says what to do.
func explainBuild(log string) string {
	switch {
	case strings.Contains(log, "No matching distribution found"), strings.Contains(log, "Could not find a version"):
		return "Uma biblioteca da lista não foi encontrada. Confira os nomes no requirements.txt."
	case strings.Contains(log, "npm ERR!") && strings.Contains(log, "ci"):
		return "As bibliotecas do Node não instalaram. Rode npm install no app e salve o package-lock.json."
	case strings.Contains(log, "no such file or directory"), strings.Contains(log, "COPY failed"):
		return "Falta um arquivo que a receita usa. Veja os detalhes."
	case strings.Contains(log, "toomanyrequests"), strings.Contains(log, "dial tcp"):
		return "O servidor não conseguiu baixar a base da receita. Tente de novo em alguns minutos."
	}
	return "Veja os detalhes para o motivo."
}

func manifestOf(app sqlc.App) manifest.Manifest {
	var m manifest.Manifest
	_ = json.Unmarshal(app.Manifest, &m)
	return m
}

func (s *Service) versionLabel(ctx context.Context, d sqlc.SharedDeployment) string {
	if d.VersionID.Valid {
		if v, err := s.q.GetAppVersionByID(ctx, d.VersionID.Int64); err == nil {
			return fmt.Sprintf("a versão %d", v.Number)
		}
	}
	return "a versão atual"
}

func (s *Service) event(ctx context.Context, appID int64, by *int64, label string) {
	raw, _ := json.Marshal([]manifest.Change{{Field: "shared", Label: label}})
	if by == nil {
		_ = s.q.InsertHubChange(ctx, sqlc.InsertHubChangeParams{AppID: appID, Changes: raw})
		return
	}
	_ = s.q.InsertAppChange(ctx, sqlc.InsertAppChangeParams{AppID: appID, PersonID: pgtype.Int8{Int64: *by, Valid: true}, Changes: raw})
}
```

(Depois que `person_id` virou opcional, o parâmetro gerado de `InsertAppChange` muda para `pgtype.Int8`; ajustar os usos antigos em `handler/` — `grep -rn InsertAppChange backend/internal/handler`.)

- [ ] **Step 6: Implementar `env.go`**

```go
package deploy

import (
	"context"
	"strconv"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
)

const dbPasswordName = "_db_password"

type MissingSettings struct{ Names []string }

func (e MissingSettings) Error() string {
	out := "Falta preencher: "
	for i, n := range e.Names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out + "."
}

// values reads app_config, opening secrets with the vault.
func (s *Service) values(ctx context.Context, appID int64) (map[string]string, error) {
	rows, err := s.q.ListAppConfig(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		if r.IsSecret {
			v, err := s.vault.Open(r.ValueCipher)
			if err != nil {
				return nil, err
			}
			out[r.Name] = v
		} else {
			out[r.Name] = r.ValuePlain
		}
	}
	return out, nil
}

func (s *Service) missingSettings(ctx context.Context, appID int64, m manifest.Manifest) ([]string, error) {
	if m.Shared == nil {
		return nil, nil
	}
	vals, err := s.values(ctx, appID)
	if err != nil {
		return nil, err
	}
	var miss []string
	for _, e := range m.Shared.Env {
		if e.Required && vals[e.Name] == "" && e.Default == "" {
			miss = append(miss, e.Label)
		}
	}
	return miss, nil
}

func (s *Service) dbPassword(ctx context.Context, appID int64) (string, error) {
	vals, err := s.values(ctx, appID)
	if err != nil {
		return "", err
	}
	if pw := vals[dbPasswordName]; pw != "" {
		return pw, nil
	}
	pw := appdb.NewPassword()
	sealed, err := s.vault.Seal(pw)
	if err != nil {
		return "", err
	}
	return pw, s.q.UpsertAppConfig(ctx, sqlc.UpsertAppConfigParams{AppID: appID, Name: dbPasswordName, IsSecret: true, ValueCipher: sealed})
}

func (s *Service) env(ctx context.Context, appID int64, m manifest.Manifest, sa sqlc.SharedApp, dbName string, port int, test bool) (map[string]string, error) {
	vals, err := s.values(ctx, appID)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	if m.Shared != nil {
		for _, e := range m.Shared.Env {
			if e.Default != "" {
				env[e.Name] = e.Default
			}
		}
	}
	for k, v := range vals {
		if k != "" && k[0] != '_' {
			env[k] = v
		}
	}
	env["PORT"] = strconv.Itoa(port)
	env["APPHUB_DATA_DIR"] = "/data"
	env["BASE_URL"] = s.AppURL(sa.Slug)
	if test {
		env["BASE_URL"] = s.TestURL(sa.Slug)
	}
	if dbName != "" {
		pw := vals[dbPasswordName]
		if test {
			pw = s.testPasswords[dbName] // guardada ao criar o banco de teste
		}
		env["DATABASE_URL"] = s.dbs.URL(dbName, pw)
	}
	return env, nil
}
```

Para o banco de teste, `runTest` guarda a senha gerada em `s.testPasswords[dbName]` (mapa novo no `Service`, protegido por `s.mu`) antes de chamar `start`, e `EndTest` apaga a entrada.

- [ ] **Step 7: Implementar `lifecycle.go`**

```go
package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/recipes"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

// Run starts the live version (new container from the kept image), waits for
// health and marks the app running. Used to wake a sleeping app, by
// "Ligar"/"Religar" and by the supervisor to restart a crashed one.
func (s *Service) Run(ctx context.Context, appID int64) error {
	if s.vault == nil {
		return ErrNoVault
	}
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil || !sa.CurrentDeploymentID.Valid {
		return ErrNotShared
	}
	dep, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil {
		return err
	}
	app, err := s.q.GetApp(ctx, appID)
	if err != nil {
		return err
	}
	if dep.ContainerID != "" {
		_ = s.eng.Remove(ctx, dep.ContainerID)
	}
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "starting"})
	m := manifestOf(app)
	port := int(dep.Port)
	if port == 0 {
		rec, _ := recipes.Detect(versions.LocalFolder{Root: app.FolderPath}, m)
		port = rec.Port
	}
	j := job{app: app, sa: sa, m: m, dep: dep, rec: recipes.Recipe{Port: port}}
	cid, hostPort, f := s.start(ctx, j, dep.ImageID, sa.DbName)
	if f != nil {
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "crashed", StatusMessage: f.msg})
		return errors.New(f.msg)
	}
	s.setTarget(appID, false, hostPort)
	_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: dep.ID, ContainerID: cid})
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "running"})
	return nil
}

// Wake runs the app in the background, once at a time per app.
func (s *Service) Wake(appID int64) {
	s.mu.Lock()
	if s.waking[appID] {
		s.mu.Unlock()
		return
	}
	s.waking[appID] = true
	s.mu.Unlock()
	s.spawn(func() {
		defer func() { s.mu.Lock(); delete(s.waking, appID); s.mu.Unlock() }()
		if err := s.Run(context.Background(), appID); err != nil {
			slog.Warn("shared wake", "app", appID, "err", err)
		}
	})
}

func (s *Service) Start(ctx context.Context, appID int64) error {
	if err := s.q.SetSharedRestarts(ctx, sqlc.SetSharedRestartsParams{AppID: appID}); err != nil {
		return err
	}
	_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "starting"})
	s.Wake(appID)
	return nil
}

func (s *Service) removeLive(ctx context.Context, appID int64) error {
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	if sa.CurrentDeploymentID.Valid {
		if d, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64); err == nil && d.ContainerID != "" {
			if err := s.eng.Remove(ctx, d.ContainerID); err != nil {
				return err
			}
			_ = s.q.SetDeploymentContainer(ctx, sqlc.SetDeploymentContainerParams{ID: d.ID})
		}
	}
	s.setTarget(appID, false, 0)
	return nil
}

func (s *Service) Sleep(ctx context.Context, appID int64) error {
	if err := s.removeLive(ctx, appID); err != nil {
		return err
	}
	return s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "sleeping"})
}

func (s *Service) Stop(ctx context.Context, appID, by int64) error {
	if err := s.removeLive(ctx, appID); err != nil {
		return err
	}
	s.event(ctx, appID, &by, "Parou o app")
	return s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: appID, Status: "stopped",
		StoppedBy: pgtype.Int8{Int64: by, Valid: true}})
}

func (s *Service) Unshare(ctx context.Context, appID, by int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	if tests, err := s.q.ListOpenTests(ctx, appID); err == nil {
		for _, d := range tests {
			_ = s.EndTest(ctx, d.ID)
		}
	}
	for _, id := range []pgtype.Int8{sa.CurrentDeploymentID, sa.PreviousDeploymentID} {
		if !id.Valid {
			continue
		}
		if d, err := s.q.GetDeployment(ctx, id.Int64); err == nil {
			_ = s.eng.Remove(ctx, d.ContainerID)
			_ = s.eng.RemoveImage(ctx, d.ImageID)
			_ = s.q.SetDeploymentStatus(ctx, sqlc.SetDeploymentStatusParams{ID: d.ID, Status: "retired"})
		}
	}
	s.setTarget(appID, false, 0)
	s.event(ctx, appID, &by, "Deixou de compartilhar (banco e cópias guardados por 30 dias)")
	return s.q.MarkUnshared(ctx, appID)
}

// Purge deletes what Unshare kept, 30 days later (called by the supervisor).
func (s *Service) Purge(ctx context.Context, appID int64) error {
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return err
	}
	if sa.DbName != "" {
		if err := s.dbs.Drop(ctx, sa.DbName); err != nil {
			return fmt.Errorf("drop %s: %w", sa.DbName, err)
		}
	}
	_ = os.RemoveAll(filepath.Join(s.opt.DataDir, "backups", strconv.FormatInt(appID, 10)))
	_ = os.RemoveAll(filepath.Join(s.opt.DataDir, "apps", strconv.FormatInt(appID, 10)))
	if err := s.q.DeleteBackupsOfApp(ctx, appID); err != nil {
		return err
	}
	return s.q.DeleteSharedApp(ctx, appID)
}
```

`SetSharedRestarts` com `RestartCount: 0` e `RestartWindowStart` nulo zera a contagem.

- [ ] **Step 8: Implementar `backups.go`**

```go
package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

var keep = map[string]time.Duration{"daily": 14 * 24 * time.Hour, "pre_deploy": 30 * 24 * time.Hour,
	"pre_restore": 30 * 24 * time.Hour, "manual": 30 * 24 * time.Hour}

func (s *Service) BackupPath(b sqlc.SharedBackup) string { return b.Path }

func (s *Service) Backup(ctx context.Context, appID int64, kind string, depID, by *int64) (sqlc.SharedBackup, error) {
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return sqlc.SharedBackup{}, ErrNotShared
	}
	if sa.DbName == "" {
		return sqlc.SharedBackup{}, fmt.Errorf("Este app não tem banco para copiar.")
	}
	dir := filepath.Join(s.opt.DataDir, "backups", strconv.FormatInt(appID, 10))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return sqlc.SharedBackup{}, err
	}
	now := s.now()
	path := filepath.Join(dir, now.Format("20060102-150405")+"-"+kind+".dump")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return sqlc.SharedBackup{}, err
	}
	err = s.dbs.Dump(ctx, sa.DbName, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return sqlc.SharedBackup{}, err
	}
	st, _ := os.Stat(path)
	p := sqlc.InsertBackupParams{AppID: appID, Kind: kind, Path: path, SizeBytes: st.Size(),
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(keep[kind]), Valid: true}, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}}
	if depID != nil {
		p.DeploymentID = pgtype.Int8{Int64: *depID, Valid: true}
	}
	if by != nil {
		p.CreatedBy = pgtype.Int8{Int64: *by, Valid: true}
	}
	return s.q.InsertBackup(ctx, p)
}

func (s *Service) Restore(ctx context.Context, appID, backupID, by int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	b, err := s.q.GetBackup(ctx, sqlc.GetBackupParams{ID: backupID, AppID: appID})
	if err != nil {
		return fmt.Errorf("Cópia não encontrada.")
	}
	if err := s.restoreFile(ctx, sa, b, by); err != nil {
		return err
	}
	if sa.CurrentDeploymentID.Valid && sa.Status != "stopped" {
		if err := s.Run(ctx, appID); err != nil {
			return err
		}
	}
	s.event(ctx, appID, &by, "Restaurou a cópia de "+b.CreatedAt.Time.Format("02/01/2006 15:04"))
	return nil
}

// restoreFile copies the current data first ("antes de restaurar"), stops
// the app and loads the file into its database. It does not start the app.
func (s *Service) restoreFile(ctx context.Context, sa sqlc.SharedApp, b sqlc.SharedBackup, by int64) error {
	if _, err := s.Backup(ctx, sa.AppID, "pre_restore", nil, &by); err != nil {
		return fmt.Errorf("Não foi possível copiar os dados atuais antes de restaurar. Nada mudou.")
	}
	if err := s.removeLive(ctx, sa.AppID); err != nil {
		return err
	}
	f, err := os.Open(b.Path)
	if err != nil {
		return fmt.Errorf("O arquivo da cópia não foi encontrado no servidor.")
	}
	defer f.Close()
	return s.dbs.Restore(ctx, sa.DbName, f)
}
```

`Rollback` com `withData` chama `restoreFile` antes de ligar a versão anterior (já está no código do Step 5).

- [ ] **Step 9: Rodar** — `go test ./internal/shared/deploy/ -v -count=1` → PASS. Rodar 5 vezes com `-race` para pegar corridas: `go test -race -count=5 ./internal/shared/deploy/`.

- [ ] **Step 10: Commit** — `git add backend/internal/shared backend/internal/database && git commit -m "Shared apps: deploy service (test, publish side by side, roll back, copies)"`

---

# Fase 5 — Porteiro, vigia e ligação no servidor

### Task 11: Porteiro (`shared/gateway`)

**Files:**
- Create: `backend/internal/shared/gateway/gateway.go` (roteamento por endereço, entrada, sessões, repasse)
- Create: `backend/internal/shared/gateway/codes.go` (códigos de uso único)
- Create: `backend/internal/shared/gateway/pages.go` + `backend/internal/shared/gateway/page.html` (páginas do porteiro, embutidas)
- Create: `backend/internal/shared/gateway/gateway_test.go`

**Interfaces:**
- Consumes: nada de outros pacotes do Hub (só a interface `Backend` abaixo, que o handler implementa no Task 13). Assim o porteiro é testado sozinho.
- Produces:

```go
package gateway

type Identity struct {
	ID    int64
	Email string
	Name  string
	Role  string
}

type Target struct {
	AppID     int64
	Name      string // nome do app
	Owner     string // nome de quem cuida
	Status    string // draft, starting, running, sleeping, crashed, stopped
	StoppedBy string
	Port      int // 0 = não há caixa para onde mandar
}

type Backend interface {
	Lookup(ctx context.Context, slug string, test bool) (Target, bool, error)
	Allowed(ctx context.Context, personID, appID int64, test bool) (Identity, bool, error) // test: pode publicar
	Wake(appID int64)
	Touch(appID int64)
}

type Gateway struct {
	Codes   *Codes
	Recheck time.Duration // 5 min
	TTL     time.Duration // 12 h
	// privados: domain, hubURL, secure, b Backend, now func() time.Time, mu, sessions map[string]*session
}

func New(domain, hubURL string, secure bool, b Backend) *Gateway
func (g *Gateway) SetNow(fn func() time.Time)
func (g *Gateway) Match(host string) (slug string, test bool, ok bool)
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request)

type Codes struct{ /* mu; m map[string]code */ }
func NewCodes() *Codes
func (c *Codes) Issue(personID, appID int64, test bool) string
func (c *Codes) Redeem(code string, appID int64, test bool) (personID int64, ok bool)
```

Endereços: `<slug>.<domain>` é o app; `<slug>--teste.<domain>` é a caixa de teste. Caminhos reservados do porteiro (nunca repassados): `/__apphub/entrar` e `/__apphub/sem-acesso`. Cookie `apphub_app` só daquele endereço (sem `Domain`), `HttpOnly`, `SameSite=Lax`, `Secure` quando o Hub está em https, 12 h. Sessões ficam em memória: se o Hub reiniciar, a pessoa passa de novo pelo Hub sem perceber (já está logada lá).

- [ ] **Step 1: Teste que falha** (`gateway_test.go`)

```go
package gateway

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fakeBackend struct {
	target  Target
	allowed bool
	woken   []int64
	touched []int64
}

func (f *fakeBackend) Lookup(_ context.Context, slug string, test bool) (Target, bool, error) {
	if slug != "notas" {
		return Target{}, false, nil
	}
	return f.target, true, nil
}
func (f *fakeBackend) Allowed(_ context.Context, pid, aid int64, test bool) (Identity, bool, error) {
	return Identity{ID: pid, Email: "ana@alis.com", Name: "Ana", Role: "Usuário"}, f.allowed, nil
}
func (f *fakeBackend) Wake(id int64)  { f.woken = append(f.woken, id) }
func (f *fakeBackend) Touch(id int64) { f.touched = append(f.touched, id) }

// upstream echoes what the app receives.
func upstream(t *testing.T) (*httptest.Server, int) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s email=%s cookie=%s host=%s", r.URL.RequestURI(), r.Header.Get("X-AppHub-User-Email"), r.Header.Get("Cookie"), r.Host)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return srv, port
}

func setup(t *testing.T) (*Gateway, *fakeBackend) {
	_, port := upstream(t)
	b := &fakeBackend{target: Target{AppID: 7, Name: "Notas", Owner: "Henry", Status: "running", Port: port}, allowed: true}
	return New("localhost:8080", "http://localhost:5173", false, b), b
}

func get(g *Gateway, host, path string, cookie *http.Cookie, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://"+host+path, nil)
	r.Host = host
	r.Header.Set("Accept", "text/html")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	return w
}

// enter does the handshake and returns the app cookie.
func enter(t *testing.T, g *Gateway, host string, test bool) *http.Cookie {
	t.Helper()
	code := g.Codes.Issue(1, 7, test)
	w := get(g, host, "/__apphub/entrar?code="+code+"&volta=/notas?x=1", nil, nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/notas?x=1" {
		t.Fatalf("enter %d %s", w.Code, w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "apphub_app" && c.HttpOnly {
			return c
		}
	}
	t.Fatal("no cookie")
	return nil
}

func TestMatch(t *testing.T) {
	g, _ := setup(t)
	cases := map[string][3]any{
		"notas.localhost:8080":        {"notas", false, true},
		"notas--teste.localhost:8080": {"notas", true, true},
		"localhost:8080":              {"", false, false},
		"a.b.localhost:8080":          {"", false, false},
		"notas.localhost:5173":        {"", false, false},
	}
	for host, want := range cases {
		slug, test, ok := g.Match(host)
		if slug != want[0] || test != want[1] || ok != want[2] {
			t.Errorf("%s: %q %v %v", host, slug, test, ok)
		}
	}
}

func TestNoSessionRedirectsToHub(t *testing.T) {
	g, _ := setup(t)
	w := get(g, "notas.localhost:8080", "/pagina?a=1", nil, nil)
	want := "http://localhost:5173/api/shared/authorize?app=notas&volta=%2Fpagina%3Fa%3D1"
	if w.Code != http.StatusFound || w.Header().Get("Location") != want {
		t.Fatalf("%d %s", w.Code, w.Header().Get("Location"))
	}
	w = get(g, "notas--teste.localhost:8080", "/", nil, nil)
	if !strings.Contains(w.Header().Get("Location"), "teste=1") {
		t.Fatalf("test host: %s", w.Header().Get("Location"))
	}
}

func TestEnterThenProxies(t *testing.T) {
	g, b := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	w := get(g, "notas.localhost:8080", "/notas?x=1", c, nil)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "path=/notas?x=1") || !strings.Contains(body, "email=ana@alis.com") {
		t.Fatalf("%d %s", w.Code, body)
	}
	if strings.Contains(body, "apphub_app") {
		t.Fatal("the app must not see the gateway cookie")
	}
	if !strings.Contains(body, "host=notas.localhost:8080") {
		t.Fatal("the app should see its own address as Host")
	}
	if len(b.touched) == 0 {
		t.Fatal("access not recorded")
	}
}

func TestGatewayStripsForgedHeaders(t *testing.T) {
	g, _ := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	w := get(g, "notas.localhost:8080", "/", c, map[string]string{"X-AppHub-User-Email": "chefe@alis.com", "X-Apphub-User-Role": "Administrador"})
	if !strings.Contains(w.Body.String(), "email=ana@alis.com") {
		t.Fatalf("%s", w.Body.String())
	}
	// sem sessão, o cabeçalho forjado também não passa: vai para o Hub
	if w := get(g, "notas.localhost:8080", "/", nil, map[string]string{"X-AppHub-User-Email": "chefe@alis.com"}); w.Code != http.StatusFound {
		t.Fatalf("forged header without session got %d", w.Code)
	}
}

func TestCodeSingleUseAndExpires(t *testing.T) {
	g, _ := setup(t)
	code := g.Codes.Issue(1, 7, false)
	if _, ok := g.Codes.Redeem(code, 8, false); ok {
		t.Fatal("code for app 7 worked on app 8")
	}
	if _, ok := g.Codes.Redeem(code, 7, false); !ok {
		t.Fatal("valid code refused")
	}
	if _, ok := g.Codes.Redeem(code, 7, false); ok {
		t.Fatal("code used twice")
	}
	now := time.Now()
	g.Codes.now = func() time.Time { return now }
	old := g.Codes.Issue(1, 7, false)
	g.Codes.now = func() time.Time { return now.Add(61 * time.Second) }
	if _, ok := g.Codes.Redeem(old, 7, false); ok {
		t.Fatal("expired code worked")
	}
}

func TestUnsafeVoltaGoesHome(t *testing.T) {
	g, _ := setup(t)
	for _, volta := range []string{"//evil.com/x", "https://evil.com", "/\\evil.com"} {
		code := g.Codes.Issue(1, 7, false)
		w := get(g, "notas.localhost:8080", "/__apphub/entrar?code="+code+"&volta="+url.QueryEscape(volta), nil, nil)
		if w.Header().Get("Location") != "/" {
			t.Errorf("%s → %s", volta, w.Header().Get("Location"))
		}
	}
}

func TestGatewayRechecksAccess(t *testing.T) {
	g, b := setup(t)
	now := time.Now()
	g.SetNow(func() time.Time { return now })
	c := enter(t, g, "notas.localhost:8080", false)
	b.allowed = false // a pessoa perdeu o acesso
	if w := get(g, "notas.localhost:8080", "/", c, nil); w.Code != 200 {
		t.Fatalf("within 5 min should still pass, got %d", w.Code)
	}
	now = now.Add(5*time.Minute + time.Second)
	w := get(g, "notas.localhost:8080", "/", c, nil)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "Sem acesso") || !strings.Contains(w.Body.String(), "Henry") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// e a sessão morreu: mesmo voltando o acesso, passa pelo Hub de novo
	b.allowed = true
	if w := get(g, "notas.localhost:8080", "/", c, nil); w.Code != http.StatusFound {
		t.Fatalf("dead session got %d", w.Code)
	}
}

func TestCookieOfOtherAppDoesNotWork(t *testing.T) {
	g, _ := setup(t)
	c := enter(t, g, "notas--teste.localhost:8080", true) // sessão da caixa de teste
	if w := get(g, "notas.localhost:8080", "/", c, nil); w.Code != http.StatusFound {
		t.Fatalf("test session opened the live app: %d", w.Code)
	}
}

func TestSleepingWakes(t *testing.T) {
	g, b := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	b.target.Status, b.target.Port = "sleeping", 0
	w := get(g, "notas.localhost:8080", "/", c, nil)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Ligando o app") ||
		!strings.Contains(w.Body.String(), `http-equiv="refresh"`) || len(b.woken) != 1 {
		t.Fatalf("%d %s woken=%v", w.Code, w.Body.String(), b.woken)
	}
	r := httptest.NewRequest("GET", "http://notas.localhost:8080/api/dados", nil)
	r.Host = "notas.localhost:8080"
	r.Header.Set("Accept", "application/json")
	r.AddCookie(c)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, r)
	if rec.Code != 503 || rec.Header().Get("Retry-After") == "" || strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("api while sleeping: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStoppedAndCrashedPages(t *testing.T) {
	g, b := setup(t)
	c := enter(t, g, "notas.localhost:8080", false)
	b.target.Status, b.target.Port, b.target.StoppedBy = "stopped", 0, "Maria"
	if w := get(g, "notas.localhost:8080", "/", c, nil); !strings.Contains(w.Body.String(), "Parado por Maria") {
		t.Fatalf("%s", w.Body.String())
	}
	b.target.Status = "crashed"
	if w := get(g, "notas.localhost:8080", "/", c, nil); !strings.Contains(w.Body.String(), "App com problema") || len(b.woken) != 0 {
		t.Fatalf("%s woken=%v", w.Body.String(), b.woken)
	}
}

func TestUnknownApp(t *testing.T) {
	g, _ := setup(t)
	if w := get(g, "outro.localhost:8080", "/", nil, nil); w.Code != 404 || !strings.Contains(w.Body.String(), "não encontrado") {
		t.Fatalf("%d", w.Code)
	}
}

func TestWebSocketPassesThrough(t *testing.T) {
	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			w.WriteHeader(400)
			return
		}
		conn, buf, _ := w.(http.Hijacker).Hijack()
		defer conn.Close()
		buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		buf.Flush()
		line, _ := buf.ReadString('\n')
		buf.WriteString("eco:" + line)
		buf.Flush()
	}))
	defer ws.Close()
	u, _ := url.Parse(ws.URL)
	port, _ := strconv.Atoi(u.Port())
	b := &fakeBackend{target: Target{AppID: 7, Status: "running", Port: port}, allowed: true}
	g := New("localhost:8080", "http://localhost:5173", false, b)
	c := enter(t, g, "notas.localhost:8080", false)
	front := httptest.NewServer(g)
	defer front.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(front.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET /_stcore/stream HTTP/1.1\r\nHost: notas.localhost:8080\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nCookie: apphub_app=%s\r\n\r\n", c.Value)
	rd := bufio.NewReader(conn)
	status, _ := rd.ReadString('\n')
	if !strings.Contains(status, "101") {
		t.Fatalf("status %q", status)
	}
	for l, _ := rd.ReadString('\n'); l != "\r\n"; l, _ = rd.ReadString('\n') {
	}
	fmt.Fprint(conn, "oi\n")
	got, _ := rd.ReadString('\n')
	if got != "eco:oi\n" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Rodar** — `go test ./internal/shared/gateway/` → FAIL.

- [ ] **Step 3: Implementar `codes.go`**

```go
package gateway

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type code struct {
	personID, appID int64
	test            bool
	expires         time.Time
}

// Codes are one-time tickets the Hub gives after checking the person, so
// the app address can open a session without a second login.
type Codes struct {
	mu  sync.Mutex
	m   map[string]code
	now func() time.Time
}

func NewCodes() *Codes { return &Codes{m: map[string]code{}, now: time.Now} }

func token() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (c *Codes) Issue(personID, appID int64, test bool) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for k, v := range c.m { // limpeza dos vencidos
		if now.After(v.expires) {
			delete(c.m, k)
		}
	}
	t := token()
	c.m[t] = code{personID: personID, appID: appID, test: test, expires: now.Add(60 * time.Second)}
	return t
}

func (c *Codes) Redeem(t string, appID int64, test bool) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[t]
	if !ok || v.appID != appID || v.test != test {
		return 0, false
	}
	delete(c.m, t)
	if c.now().After(v.expires) {
		return 0, false
	}
	return v.personID, true
}
```

- [ ] **Step 4: Implementar `pages.go` e `page.html`**

`page.html` (uma página só, no estilo do Hub: fundo claro/escuro pelo sistema, violeta só no botão):

```html
<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
{{if .Refresh}}<meta http-equiv="refresh" content="2">{{end}}
<title>{{.Title}} · App Hub</title>
<style>
  :root { --bg:#fafafa; --fg:#18181b; --muted:#71717a; --card:#fff; --line:#e4e4e7; --accent:#6d28d9; }
  @media (prefers-color-scheme: dark) { :root { --bg:#09090b; --fg:#fafafa; --muted:#a1a1aa; --card:#18181b; --line:#27272a; --accent:#a78bfa; } }
  body { margin:0; min-height:100vh; display:grid; place-items:center; background:var(--bg); color:var(--fg);
         font-family: Geist, ui-sans-serif, system-ui, sans-serif; padding:16px; box-sizing:border-box; }
  main { max-width:420px; width:100%; background:var(--card); border:1px solid var(--line); border-radius:12px; padding:28px; }
  .app { color:var(--muted); font-size:14px; margin:0 0 8px; }
  h1 { font-size:22px; margin:0 0 12px; }
  p { line-height:1.5; margin:0 0 16px; }
  a.btn { display:inline-block; background:var(--accent); color:#fff; text-decoration:none; padding:8px 14px; border-radius:8px; font-size:14px; }
  .spin { width:18px; height:18px; border:2px solid var(--line); border-top-color:var(--accent); border-radius:50%;
          animation:s 1s linear infinite; display:inline-block; vertical-align:middle; margin-right:8px; }
  @keyframes s { to { transform:rotate(360deg) } }
</style>
</head>
<body>
<main>
  {{if .App}}<p class="app">{{.App}}</p>{{end}}
  <h1>{{if .Refresh}}<span class="spin"></span>{{end}}{{.Title}}</h1>
  <p>{{.Message}}</p>
  {{if .HubURL}}<a class="btn" href="{{.HubURL}}">Voltar ao Hub</a>{{end}}
</main>
</body>
</html>
```

`pages.go`:

```go
package gateway

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"
)

//go:embed page.html
var pageHTML string

var pageTmpl = template.Must(template.New("p").Parse(pageHTML))

type page struct {
	App, Title, Message, HubURL string
	Refresh                     bool
}

func wantsHTML(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html")
}

// show writes a gateway page, or a short JSON error for API calls.
func (g *Gateway) show(w http.ResponseWriter, r *http.Request, status int, p page) {
	if status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "5")
	}
	w.Header().Set("Cache-Control", "no-store")
	if !wantsHTML(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":` + jsonString(p.Title+". "+p.Message) + `}`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTmpl.Execute(w, p)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func owner(t Target) string {
	if t.Owner == "" {
		return "quem cuida do app"
	}
	return t.Owner
}
```

(importar `encoding/json`.)

- [ ] **Step 5: Implementar `gateway.go`**

```go
// Package gateway is the door of shared apps: it answers on each app's own
// address, checks the person with the Hub, wakes sleeping apps and passes
// requests (WebSocket included) to the app's container.
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const cookieName = "apphub_app"

type session struct {
	personID, appID int64
	test            bool
	id              Identity
	expires         time.Time
	checked         time.Time
}

type Gateway struct {
	Codes   *Codes
	Recheck time.Duration
	TTL     time.Duration

	domain string
	hubURL string
	secure bool
	b      Backend
	now    func() time.Time

	mu       sync.Mutex
	sessions map[string]*session
}

func New(domain, hubURL string, secure bool, b Backend) *Gateway {
	return &Gateway{Codes: NewCodes(), Recheck: 5 * time.Minute, TTL: 12 * time.Hour,
		domain: domain, hubURL: strings.TrimRight(hubURL, "/"), secure: secure, b: b, now: time.Now,
		sessions: map[string]*session{}}
}

func (g *Gateway) SetNow(fn func() time.Time) { g.now = fn; g.Codes.now = fn }

func (g *Gateway) Match(host string) (string, bool, bool) {
	sub, ok := strings.CutSuffix(strings.ToLower(host), "."+g.domain)
	if !ok || sub == "" || strings.Contains(sub, ".") {
		return "", false, false
	}
	if slug, isTest := strings.CutSuffix(sub, "--teste"); isTest {
		return slug, true, slug != ""
	}
	return sub, false, true
}

func safeVolta(v string) string {
	if !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.HasPrefix(v, "/\\") {
		return "/"
	}
	return v
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug, test, _ := g.Match(r.Host)
	t, ok, err := g.b.Lookup(r.Context(), slug, test)
	if err != nil {
		slog.Error("gateway lookup", "slug", slug, "err", err)
		g.show(w, r, http.StatusInternalServerError, page{Title: "Algo deu errado", Message: "Tente de novo em instantes.", HubURL: g.hubURL})
		return
	}
	if !ok {
		g.show(w, r, http.StatusNotFound, page{Title: "App não encontrado", Message: "Este endereço não é de nenhum app compartilhado do Hub.", HubURL: g.hubURL})
		return
	}
	switch r.URL.Path {
	case "/__apphub/entrar":
		g.enter(w, r, t, test)
		return
	case "/__apphub/sem-acesso":
		g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
		return
	}
	s := g.session(r, t.AppID, test)
	if s == nil {
		if !wantsHTML(r) {
			g.show(w, r, http.StatusUnauthorized, page{Title: "Entre pelo Hub", Message: "Abra o app pelo Hub."})
			return
		}
		q := url.Values{"app": {slug}, "volta": {r.URL.RequestURI()}}
		if test {
			q.Set("teste", "1")
		}
		http.Redirect(w, r, g.hubURL+"/api/shared/authorize?"+q.Encode(), http.StatusFound)
		return
	}
	if g.now().Sub(s.checked) >= g.Recheck {
		id, allowed, err := g.b.Allowed(r.Context(), s.personID, t.AppID, test)
		if err != nil {
			slog.Error("gateway recheck", "app", t.AppID, "err", err)
		} else if !allowed {
			g.drop(r)
			g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem mais acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
			return
		} else {
			g.mu.Lock()
			s.id, s.checked = id, g.now()
			g.mu.Unlock()
		}
	}
	if t.Port == 0 {
		g.notRunning(w, r, t, test)
		return
	}
	if !test {
		g.b.Touch(t.AppID)
	}
	g.proxy(w, r, t, s)
}

func (g *Gateway) notRunning(w http.ResponseWriter, r *http.Request, t Target, test bool) {
	switch {
	case test:
		g.show(w, r, http.StatusNotFound, page{App: t.Name, Title: "O teste terminou", Message: "A caixa de teste foi desligada. Faça um teste novo pela ficha do app.", HubURL: g.hubURL})
	case t.Status == "stopped":
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "App parado", Message: "Parado por " + t.StoppedBy + ". Peça para " + owner(t) + " ligá-lo de novo.", HubURL: g.hubURL})
	case t.Status == "crashed":
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "App com problema", Message: "O app parou de responder e o Hub não conseguiu religá-lo. " + owner(t) + " já vê o aviso na ficha do app.", HubURL: g.hubURL})
	case t.Status == "draft":
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "Ainda não publicado", Message: "Este app ainda não foi publicado. Fale com " + owner(t) + ".", HubURL: g.hubURL})
	default: // sleeping, starting, running sem porta (Hub acabou de ligar)
		g.b.Wake(t.AppID)
		g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "Ligando o app…", Message: "O app estava desligado para economizar. Ele abre sozinho em alguns segundos.", Refresh: true})
	}
}

func (g *Gateway) enter(w http.ResponseWriter, r *http.Request, t Target, test bool) {
	personID, ok := g.Codes.Redeem(r.URL.Query().Get("code"), t.AppID, test)
	volta := safeVolta(r.URL.Query().Get("volta"))
	if !ok {
		// código vencido ou já usado: passa pelo Hub de novo
		q := url.Values{"app": {strings.TrimSuffix(strings.Split(r.Host, ".")[0], "--teste")}, "volta": {volta}}
		if test {
			q.Set("teste", "1")
		}
		http.Redirect(w, r, g.hubURL+"/api/shared/authorize?"+q.Encode(), http.StatusFound)
		return
	}
	id, allowed, err := g.b.Allowed(r.Context(), personID, t.AppID, test)
	if err != nil || !allowed {
		g.show(w, r, http.StatusForbidden, page{App: t.Name, Title: "Sem acesso", Message: "Você não tem acesso a este app. Peça acesso a " + owner(t) + ".", HubURL: g.hubURL})
		return
	}
	tok := token()
	now := g.now()
	g.mu.Lock()
	for k, s := range g.sessions {
		if now.After(s.expires) {
			delete(g.sessions, k)
		}
	}
	g.sessions[tok] = &session{personID: personID, appID: t.AppID, test: test, id: id, expires: now.Add(g.TTL), checked: now}
	g.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: tok, Path: "/", MaxAge: int(g.TTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: g.secure})
	http.Redirect(w, r, volta, http.StatusFound)
}

func (g *Gateway) session(r *http.Request, appID int64, test bool) *session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.sessions[c.Value]
	if s == nil || s.appID != appID || s.test != test || g.now().After(s.expires) {
		return nil
	}
	return s
}

func (g *Gateway) drop(r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		g.mu.Lock()
		delete(g.sessions, c.Value)
		g.mu.Unlock()
	}
}

func (g *Gateway) proxy(w http.ResponseWriter, r *http.Request, t Target, s *session) {
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(t.Port)}
	g.mu.Lock()
	id := s.id
	g.mu.Unlock()
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = pr.In.Host // o app vê o próprio endereço
			pr.SetXForwarded()
			for k := range pr.Out.Header {
				if strings.HasPrefix(strings.ToLower(k), "x-apphub-") {
					pr.Out.Header.Del(k)
				}
			}
			pr.Out.Header.Set("X-AppHub-User-Id", strconv.FormatInt(id.ID, 10))
			pr.Out.Header.Set("X-AppHub-User-Email", id.Email)
			pr.Out.Header.Set("X-AppHub-User-Name", url.QueryEscape(id.Name)) // nomes com acento: o app decodifica
			pr.Out.Header.Set("X-AppHub-User-Role", url.QueryEscape(id.Role))
			stripCookie(pr.Out)
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("gateway proxy", "app", t.AppID, "err", err)
			g.b.Wake(t.AppID)
			g.show(w, r, http.StatusServiceUnavailable, page{App: t.Name, Title: "Ligando o app…", Message: "O app está reiniciando. Ele abre sozinho em alguns segundos.", Refresh: true})
		},
	}
	rp.ServeHTTP(w, r)
}

// stripCookie removes the gateway cookie, keeping the app's own cookies.
func stripCookie(r *http.Request) {
	cs := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cs {
		if c.Name != cookieName {
			r.AddCookie(c)
		}
	}
}

var _ = context.Background
var _ = fmt.Sprint
```

(Tirar as duas últimas linhas se `context`/`fmt` não forem usados — elas só existem para o plano não esquecer os imports; o `gofmt`/compilador avisa.) O cabeçalho `X-AppHub-User-Name` vai codificado (`url.QueryEscape`) porque cabeçalhos HTTP não levam acentos com segurança; documentar isso no ADR 010 e no texto para o Claude ("decodifique com `urllib.parse.unquote`").

- [ ] **Step 6: Rodar** — `go test -race ./internal/shared/gateway/` → PASS.

- [ ] **Step 7: Commit** — `git add backend/internal/shared/gateway && git commit -m "Shared apps: gateway with Hub sign-in, access recheck, wake pages and proxy"`

---

### Task 12: Vigia (`shared/supervisor`)

**Files:**
- Modify: `backend/internal/shared/deploy/lifecycle.go` — acrescentar `Adopt` e `HubEvent` (abaixo)
- Create: `backend/internal/shared/supervisor/supervisor.go`, `backend/internal/shared/supervisor/supervisor_test.go`

**Interfaces:**
- Consumes: `*deploy.Service` (`Run`, `Sleep`, `Backup`, `EndTest`, `Purge`, `Adopt`, `HubEvent`, `SetNow`), `engine.Engine` (`Inspect`, `Stats`, `List`, `Remove`, `Available`), `health.Check`, consultas do Task 1.
- Produces:

```go
// em deploy:
// Adopt looks at the live container after a Hub restart: if it is running,
// the gateway target is restored and true is returned.
func (s *Service) Adopt(ctx context.Context, appID int64) (bool, error)
func (s *Service) HubEvent(ctx context.Context, appID int64, label string) // linha do tempo sem pessoa ("Hub")

package supervisor

type Supervisor struct {
	Interval   time.Duration // 30 s
	BackupHour int           // 3
	// privados: q, eng, svc, now, mu, access map[int64]time.Time, lastUsage, lastCleanup time.Time, backupDay string, down bool
}

func New(q *sqlc.Queries, eng engine.Engine, svc *deploy.Service) *Supervisor
func (s *Supervisor) SetNow(fn func() time.Time)
func (s *Supervisor) Touch(appID int64)         // porteiro: houve acesso agora
func (s *Supervisor) Run(ctx context.Context)   // Reconcile e depois Tick a cada Interval, até ctx acabar
func (s *Supervisor) Reconcile(ctx context.Context) error
func (s *Supervisor) Tick(ctx context.Context)
```

Regras do `Tick` (nesta ordem):
1. `Available` falhou → loga uma vez (`slog.Warn`, não repete a cada 30 s) e sai.
2. Grava os acessos guardados em memória (`TouchSharedAccess`).
3. Para cada app com status `running`: `Inspect` da caixa da versão no ar; se não está rodando, ou `health.Check` falha → religar. Janela de 10 min (`restart_window_start`): se passou, zera. Até 3 religadas na janela (`SetSharedRestarts` e `svc.Run`); na 4ª queda → status `crashed` com "O app parou de responder 3 vezes em 10 minutos." e `HubEvent("O app caiu e o Hub parou de tentar religar")`.
4. Para cada app `running` sem `always_on` cujo `last_access_at` (ou, se nulo, `status_changed_at`) passou de `idle_minutes` → `svc.Sleep`.
5. Uma vez por minuto: `Stats` das caixas ligadas → `InsertUsage` com `at` truncado no minuto.
6. Depois de `BackupHour` (3h), uma vez por dia: cópia `daily` de cada app com banco que não teve cópia diária hoje (`LastDailyBackup`).
7. Uma vez por hora: `ListExpiredTests` → `EndTest`; `ListExpiredBackups` → apaga o arquivo e a linha; `DeleteUsageBefore(agora − 90 dias)`; `ListUnsharedBefore(agora − 30 dias)` → `Purge`.

`Reconcile` (ao ligar o Hub):
- `List` (só caixas com o rótulo do Hub) × `ListLabelledContainersExpected`: o que não é esperado é removido. Caixas sem o rótulo nunca aparecem na lista (garantido pelo motor).
- Testes abertos são encerrados (`EndTest`): a caixa de teste não sobrevive a um reinício.
- Apps `running`/`starting`: `Adopt`; se a caixa não está rodando → `svc.Run`.

- [ ] **Step 1: `Adopt` e `HubEvent` em `lifecycle.go`**

```go
func (s *Service) Adopt(ctx context.Context, appID int64) (bool, error) {
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil || !sa.CurrentDeploymentID.Valid {
		return false, err
	}
	d, err := s.q.GetDeployment(ctx, sa.CurrentDeploymentID.Int64)
	if err != nil || d.ContainerID == "" {
		return false, err
	}
	st, err := s.eng.Inspect(ctx, d.ContainerID)
	if err != nil || !st.Running || st.HostPort == 0 {
		return false, err
	}
	s.setTarget(appID, false, st.HostPort)
	return true, nil
}

func (s *Service) HubEvent(ctx context.Context, appID int64, label string) { s.event(ctx, appID, nil, label) }
```

- [ ] **Step 2: Teste que falha** (`supervisor_test.go`). Reaproveita o `deploytest` do Task 10.

```go
package supervisor

import (
	"context"
	"os"
	"testing"
	"time"

	"net/http"

	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/sharedtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	var err error
	pool, err = sharedtest.Pool(context.Background())
	if err != nil {
		panic(err)
	}
	if pool == nil {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time         { return c.t }
func (c *clock) add(d time.Duration)    { c.t = c.t.Add(d) }

func live(t *testing.T) (*deploytest.Env, *Supervisor, *clock) {
	e := deploytest.Setup(t, pool)
	e.Publish(t)
	c := &clock{t: time.Now()}
	e.S.SetNow(c.now)
	s := New(e.Q, e.Eng, e.S)
	s.SetNow(c.now)
	return e, s, c
}

func currentContainer(t *testing.T, e *deploytest.Env) string {
	d, err := e.Q.GetDeployment(context.Background(), e.Shared(t).CurrentDeploymentID.Int64)
	if err != nil {
		t.Fatal(err)
	}
	return d.ContainerID
}

func TestRestartsThreeTimesThenCrashed(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	for i := 1; i <= 3; i++ {
		e.Eng.Crash(currentContainer(t, e))
		s.Tick(ctx)
		if st := e.Shared(t).Status; st != "running" {
			t.Fatalf("restart %d: status %s", i, st)
		}
		c.add(time.Minute)
	}
	e.Eng.Crash(currentContainer(t, e))
	s.Tick(ctx)
	sa := e.Shared(t)
	if sa.Status != "crashed" || sa.StatusMessage == "" {
		t.Fatalf("%+v", sa)
	}
	var who string
	pool.QueryRow(ctx, `SELECT COALESCE(p.name,'Hub') FROM app_changes c LEFT JOIN people p ON p.id = c.person_id
		WHERE c.app_id = $1 ORDER BY c.id DESC LIMIT 1`, e.App.ID).Scan(&who)
	if who != "Hub" {
		t.Fatalf("last event by %q", who)
	}
	s.Tick(ctx) // caiu: não tenta mais
	if len(e.Eng.Running()) != 0 {
		t.Fatal("kept restarting after crashed")
	}
}

func TestRestartWindowResets(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	for i := 0; i < 3; i++ {
		e.Eng.Crash(currentContainer(t, e))
		s.Tick(ctx)
	}
	c.add(11 * time.Minute)
	e.Eng.Crash(currentContainer(t, e))
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("a new 10-minute window should allow restarting again")
	}
}

func TestUnhealthyButRunningIsRestarted(t *testing.T) {
	ctx := context.Background()
	e, s, _ := live(t)
	old := currentContainer(t, e)
	e.Eng.SetHandler(old, http500{})
	s.Tick(ctx)
	if currentContainer(t, e) == old {
		t.Fatal("an app answering 500 should be restarted")
	}
}

func TestSleepsWhenIdleUnlessAlwaysOn(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	c.add(29 * time.Minute)
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("slept too early")
	}
	s.Touch(e.App.ID)
	c.add(20 * time.Minute)
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("a recent access should keep it awake")
	}
	c.add(31 * time.Minute)
	s.Tick(ctx)
	if e.Shared(t).Status != "sleeping" || len(e.Eng.Running()) != 0 {
		t.Fatalf("status %s", e.Shared(t).Status)
	}
	// sempre ligado
	pool.Exec(ctx, "UPDATE shared_apps SET always_on = true WHERE app_id = $1", e.App.ID)
	e.S.Run(ctx, e.App.ID)
	c.add(5 * time.Hour)
	s.Tick(ctx)
	if e.Shared(t).Status != "running" {
		t.Fatal("always_on app slept")
	}
}

func TestUsageEveryMinute(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	s.Tick(ctx)
	c.add(30 * time.Second)
	s.Tick(ctx) // mesmo minuto: não grava de novo
	c.add(40 * time.Second)
	s.Tick(ctx)
	var n int
	pool.QueryRow(ctx, "SELECT count(*) FROM shared_usage WHERE app_id = $1", e.App.ID).Scan(&n)
	if n != 2 {
		t.Fatalf("usage rows %d", n)
	}
}

func TestDailyBackupAfterThree(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	day := time.Date(2026, 10, 7, 2, 59, 0, 0, time.Local)
	c.t = day
	s.Tick(ctx)
	count := func() int {
		var n int
		pool.QueryRow(ctx, "SELECT count(*) FROM shared_backups WHERE app_id = $1 AND kind = 'daily'", e.App.ID).Scan(&n)
		return n
	}
	if count() != 0 {
		t.Fatal("backup before 3h")
	}
	c.t = day.Add(2 * time.Minute)
	s.Tick(ctx)
	s.Tick(ctx)
	if count() != 1 {
		t.Fatalf("daily backups %d", count())
	}
}

func TestExpiredTestEnded(t *testing.T) {
	ctx := context.Background()
	e, s, c := live(t)
	d := e.StartTest(t)
	c.add(31 * time.Minute)
	c.add(time.Hour) // a limpeza roda de hora em hora
	s.Tick(ctx)
	got, _ := e.Q.GetDeployment(ctx, d.ID)
	if got.Status != "retired" {
		t.Fatalf("test status %s", got.Status)
	}
}

func TestReconcileOnStart(t *testing.T) {
	ctx := context.Background()
	e, _, _ := live(t)
	orphan, _ := e.Eng.Start(ctx, engine.StartSpec{Name: "apphub-velho-1", Labels: map[string]string{engine.Label: "99"}})
	foreign, _ := e.Eng.Start(ctx, engine.StartSpec{Name: "sistema-alis-db"}) // não é do Hub
	// "reinício": serviço novo, sem portas em memória
	svc2 := e.NewService()
	s := New(e.Q, e.Eng, svc2)
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.Eng.Inspect(ctx, orphan); st.Exists {
		t.Fatal("orphan Hub container kept")
	}
	if st, _ := e.Eng.Inspect(ctx, foreign); !st.Running {
		t.Fatal("touched a container that is not the Hub's")
	}
	if _, ok := svc2.Target(e.App.ID, false); !ok {
		t.Fatal("live app not adopted")
	}
	// caixa sumiu durante o reinício: liga de novo
	e.Eng.Remove(ctx, currentContainer(t, e))
	svc3 := e.NewService()
	if err := New(e.Q, e.Eng, svc3).Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc3.Target(e.App.ID, false); !ok || e.Shared(t).Status != "running" {
		t.Fatal("missing container not restarted")
	}
}

type http500 struct{}

func (http500) ServeHTTP(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }
```

- [ ] **Step 3: Rodar** → FAIL.

- [ ] **Step 4: Implementar `supervisor.go`**

```go
// Package supervisor is the single loop that keeps shared apps healthy:
// restart, sleep when idle, measure, daily copies, cleanup.
package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/health"
)

type Supervisor struct {
	Interval   time.Duration
	BackupHour int

	q   *sqlc.Queries
	eng engine.Engine
	svc *deploy.Service
	now func() time.Time

	mu          sync.Mutex
	access      map[int64]time.Time
	lastUsage   time.Time
	lastCleanup time.Time
	backupDay   string
	down        bool
}

func New(q *sqlc.Queries, eng engine.Engine, svc *deploy.Service) *Supervisor {
	return &Supervisor{Interval: 30 * time.Second, BackupHour: 3, q: q, eng: eng, svc: svc, now: time.Now, access: map[int64]time.Time{}}
}

func (s *Supervisor) SetNow(fn func() time.Time) { s.now = fn }

func (s *Supervisor) Touch(appID int64) {
	s.mu.Lock()
	s.access[appID] = s.now()
	s.mu.Unlock()
}

func (s *Supervisor) Run(ctx context.Context) {
	if err := s.Reconcile(ctx); err != nil {
		slog.Warn("supervisor reconcile", "err", err)
	}
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

func (s *Supervisor) Reconcile(ctx context.Context) error {
	if err := s.eng.Available(ctx); err != nil {
		return err
	}
	if tests, err := s.q.ListAllOpenTests(ctx); err == nil {
		for _, d := range tests {
			_ = s.svc.EndTest(ctx, d.ID)
		}
	}
	expected := map[string]bool{}
	ids, err := s.q.ListLabelledContainersExpected(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		expected[id] = true
	}
	list, err := s.eng.List(ctx)
	if err != nil {
		return err
	}
	for _, c := range list {
		if !expected[c.ID] {
			slog.Info("supervisor removes orphan", "name", c.Name)
			_ = s.eng.Remove(ctx, c.ID)
		}
	}
	apps, err := s.q.ListSharedApps(ctx)
	if err != nil {
		return err
	}
	for _, a := range apps {
		if a.Status != "running" && a.Status != "starting" {
			continue
		}
		if ok, _ := s.svc.Adopt(ctx, a.AppID); !ok {
			if err := s.svc.Run(ctx, a.AppID); err != nil {
				slog.Warn("supervisor start on boot", "app", a.AppID, "err", err)
			}
		}
	}
	return nil
}

func (s *Supervisor) Tick(ctx context.Context) {
	if err := s.eng.Available(ctx); err != nil {
		if !s.down {
			slog.Warn("supervisor: container engine unavailable", "err", err)
		}
		s.down = true
		return
	}
	s.down = false
	now := s.now()
	s.flushAccess(ctx)
	apps, err := s.q.ListSharedApps(ctx)
	if err != nil {
		slog.Error("supervisor list", "err", err)
		return
	}
	for _, a := range apps {
		if a.Status != "running" {
			continue
		}
		if !s.healthy(ctx, a) {
			s.restart(ctx, a, now)
			continue
		}
		last := a.StatusChangedAt.Time
		if a.LastAccessAt.Valid && a.LastAccessAt.Time.After(last) {
			last = a.LastAccessAt.Time
		}
		if !a.AlwaysOn && now.Sub(last) > time.Duration(a.IdleMinutes)*time.Minute {
			if err := s.svc.Sleep(ctx, a.AppID); err != nil {
				slog.Warn("supervisor sleep", "app", a.AppID, "err", err)
			}
		}
	}
	if now.Sub(s.lastUsage) >= time.Minute {
		s.lastUsage = now
		s.measure(ctx, apps, now)
	}
	if day := now.Format("2006-01-02"); now.Hour() >= s.BackupHour && s.backupDay != day {
		s.backupDay = day
		s.dailyBackups(ctx, apps, now)
	}
	if now.Sub(s.lastCleanup) >= time.Hour {
		s.lastCleanup = now
		s.cleanup(ctx, now)
	}
}

func (s *Supervisor) flushAccess(ctx context.Context) {
	s.mu.Lock()
	acc := s.access
	s.access = map[int64]time.Time{}
	s.mu.Unlock()
	for id, t := range acc {
		_ = s.q.TouchSharedAccess(ctx, sqlc.TouchSharedAccessParams{AppID: id, LastAccessAt: pgtype.Timestamptz{Time: t, Valid: true}})
	}
}

func (s *Supervisor) container(ctx context.Context, a sqlc.ListSharedAppsRow) string {
	if !a.CurrentDeploymentID.Valid {
		return ""
	}
	d, err := s.q.GetDeployment(ctx, a.CurrentDeploymentID.Int64)
	if err != nil {
		return ""
	}
	return d.ContainerID
}

func (s *Supervisor) healthy(ctx context.Context, a sqlc.ListSharedAppsRow) bool {
	id := s.container(ctx, a)
	if id == "" {
		return false
	}
	st, err := s.eng.Inspect(ctx, id)
	if err != nil || !st.Running || st.HostPort == 0 {
		return false
	}
	return health.Check(ctx, fmt.Sprintf("http://127.0.0.1:%d%s", st.HostPort, a.HealthPath))
}

func (s *Supervisor) restart(ctx context.Context, a sqlc.ListSharedAppsRow, now time.Time) {
	count, start := a.RestartCount, a.RestartWindowStart
	if !start.Valid || now.Sub(start.Time) > 10*time.Minute {
		count, start = 0, pgtype.Timestamptz{Time: now, Valid: true}
	}
	if count >= 3 {
		_ = s.q.SetSharedStatus(ctx, sqlc.SetSharedStatusParams{AppID: a.AppID, Status: "crashed",
			StatusMessage: "O app parou de responder 3 vezes em 10 minutos."})
		s.svc.HubEvent(ctx, a.AppID, "O app caiu e o Hub parou de tentar religar")
		return
	}
	_ = s.q.SetSharedRestarts(ctx, sqlc.SetSharedRestartsParams{AppID: a.AppID, RestartCount: count + 1, RestartWindowStart: start})
	slog.Info("supervisor restarts app", "app", a.AppID, "try", count+1)
	if err := s.svc.Run(ctx, a.AppID); err != nil {
		slog.Warn("supervisor restart", "app", a.AppID, "err", err)
	}
}

func (s *Supervisor) measure(ctx context.Context, apps []sqlc.ListSharedAppsRow, now time.Time) {
	byContainer := map[string]int64{}
	var ids []string
	for _, a := range apps {
		if a.Status == "running" {
			if id := s.container(ctx, a); id != "" {
				byContainer[id] = a.AppID
				ids = append(ids, id)
			}
		}
	}
	usage, err := s.eng.Stats(ctx, ids)
	if err != nil {
		slog.Warn("supervisor stats", "err", err)
		return
	}
	at := pgtype.Timestamptz{Time: now.Truncate(time.Minute), Valid: true}
	for _, u := range usage {
		appID, ok := byContainer[u.ContainerID]
		if !ok { // o Podman pode devolver o ID curto
			for full, id := range byContainer {
				if len(u.ContainerID) >= 12 && len(full) >= 12 && full[:12] == u.ContainerID[:12] {
					appID, ok = id, true
				}
			}
		}
		if ok {
			_ = s.q.InsertUsage(ctx, sqlc.InsertUsageParams{AppID: appID, At: at, MemBytes: u.MemBytes, CpuPct: float32(u.CPUPct)})
		}
	}
}

func (s *Supervisor) dailyBackups(ctx context.Context, apps []sqlc.ListSharedAppsRow, now time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, a := range apps {
		if a.DbName == "" {
			continue
		}
		if b, err := s.q.LastDailyBackup(ctx, a.AppID); err == nil && !b.CreatedAt.Time.Before(today) {
			continue
		}
		if _, err := s.svc.Backup(ctx, a.AppID, "daily", nil, nil); err != nil {
			slog.Error("supervisor daily backup", "app", a.AppID, "err", err)
		}
	}
}

func (s *Supervisor) cleanup(ctx context.Context, now time.Time) {
	ts := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	if tests, err := s.q.ListExpiredTests(ctx, ts(now)); err == nil {
		for _, d := range tests {
			_ = s.svc.EndTest(ctx, d.ID)
		}
	}
	if old, err := s.q.ListExpiredBackups(ctx, ts(now)); err == nil {
		for _, b := range old {
			_ = os.Remove(b.Path)
			_ = s.q.DeleteBackup(ctx, b.ID)
		}
	}
	_ = s.q.DeleteUsageBefore(ctx, ts(now.Add(-90*24*time.Hour)))
	if gone, err := s.q.ListUnsharedBefore(ctx, ts(now.Add(-30*24*time.Hour))); err == nil {
		for _, a := range gone {
			if err := s.svc.Purge(ctx, a.AppID); err != nil {
				slog.Error("supervisor purge", "app", a.AppID, "err", err)
			}
		}
	}
}
```

O `Tick` usa `ListSharedApps`, que já ignora apps que deixaram de ser compartilhados.

O `Backup` do Task 10 grava `created_at = s.now()`, para que a cópia diária funcione com o relógio falso dos testes.

- [ ] **Step 5: Rodar** — `go test -race ./internal/shared/...` → PASS.

- [ ] **Step 6: Commit** — `git add backend/internal/shared backend/internal/database && git commit -m "Shared apps: supervisor (restart, sleep, usage, daily copies, reconcile)"`

---

### Task 13: Ligação no servidor — entrada pelo Hub, porteiro no `main.go`, login com volta

**Files:**
- Create: `backend/internal/handler/shared_gateway.go` (dependências dos compartilhados no `Handler`, `GatewayBackend`, `/api/shared/authorize`)
- Create: `backend/internal/handler/shared_test_helpers_test.go`, `backend/internal/handler/shared_gateway_test.go`
- Modify: `backend/internal/handler/handler.go` (campo `shared`, rota)
- Modify: `backend/cmd/server/main.go`
- Modify: `frontend/src/pages/LoginPage.tsx` + `LoginPage.test.tsx` (voltar para o app depois do login)
- Modify: `.env` (só acrescentar `HUB_SECRET_KEY`, sem mostrar o valor), `.env.example`, `docs/INFRASTRUCTURE.md`

**Interfaces:**
- Consumes: `deploy.Service`, `supervisor.Supervisor`, `gateway.New/Codes/Backend/Identity/Target`, `appdb.Admin`, `vault.New`, `engine.NewPodman`.
- Produces:

```go
// handler
type Shared struct {
	Svc     *deploy.Service
	Sup     *supervisor.Supervisor // nil nos testes
	Vault   *vault.Vault           // nil = sem HUB_SECRET_KEY
	DBs     appdb.DBs
	Eng     engine.Engine
	Codes   *gateway.Codes
	DataDir string
}
func (h *Handler) SetShared(s Shared)
func (h *Handler) GatewayBackend() gateway.Backend
// GET /api/shared/authorize?app=<slug>&teste=1&volta=<caminho>
```

- [ ] **Step 1: Ajudantes de teste do handler** (`shared_test_helpers_test.go`)

```go
package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

type sharedDeps struct {
	svc   *deploy.Service
	eng   *engine.Fake
	dbs   *appdb.Fake
	codes *gateway.Codes
}

// withShared plugs fake shared-app parts into h (use with newPoolClient, since
// the deploy service works in the background).
func withShared(t *testing.T, h *Handler) sharedDeps {
	t.Helper()
	v, _ := vault.New(vault.NewKey())
	d := sharedDeps{eng: engine.NewFake(), dbs: appdb.NewFake(), codes: gateway.NewCodes()}
	d.svc = deploy.New(h.q, d.eng, d.dbs, v, deploy.Options{DataDir: t.TempDir(), Domain: "localhost:8080", Scheme: "http", HealthTimeout: time.Second})
	h.SetShared(Shared{Svc: d.svc, Vault: v, DBs: d.dbs, Eng: d.eng, Codes: d.codes, DataDir: t.TempDir()})
	return d
}

// sharedApp registers a web app (folder = copy of testdata/shared/streamlit) and marks it shared.
func sharedApp(t *testing.T, h *Handler, d sharedDeps, slug string) int64 {
	t.Helper()
	dir := t.TempDir()
	copyTree(t, "../../testdata/shared/streamlit", dir)
	m := manifest.Manifest{ManifestVersion: 1, ID: slug, Name: "Notas", Kind: manifest.KindWeb, Entrypoint: "app.py",
		Shared: &manifest.Shared{Enabled: true}}
	id := insertApp(t, m)
	if _, err := testPool.Exec(context.Background(), "UPDATE apps SET folder_path = $1 WHERE id = $2", dir, id); err != nil {
		t.Fatal(err)
	}
	app, _ := h.q.GetApp(context.Background(), id)
	if _, err := d.svc.Configure(context.Background(), app, m); err != nil {
		t.Fatal(err)
	}
	return id
}

// noRedirect makes the client stop at the first redirect, to read Location.
func (c *client) noRedirect() { c.c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse } }
```

`copyTree` é a mesma função do `deploytest.CopyDir` — use `deploytest.CopyDir` direto (o handler pode importar `deploytest`). Se `insertApp` gravar `owner_id = 1` (o admin), está certo para estes testes.

- [ ] **Step 2: Teste que falha** (`shared_gateway_test.go`)

```go
package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSharedAuthorizeIssuesCode(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&volta=%2Fpagina", nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != http.StatusFound || loc.Host != "notas.localhost:8080" || loc.Path != "/__apphub/entrar" || loc.Query().Get("volta") != "/pagina" {
		t.Fatalf("%d %s", res.StatusCode, loc)
	}
	if _, ok := d.codes.Redeem(loc.Query().Get("code"), id, false); !ok {
		t.Fatal("code not valid for the app")
	}
}

func TestSharedAuthorizeWithoutSessionGoesToLogin(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	sharedApp(t, h, d, "notas")
	c.do("POST", "/api/auth/logout", nil, nil)
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&volta=%2F", nil)
	if res.StatusCode != http.StatusFound || !strings.HasPrefix(res.Header.Get("Location"), "/entrar?volta=%2Fapi%2Fshared%2Fauthorize") {
		t.Fatalf("%d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestSharedAuthorizeRestrictedApp(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	if code := c.do("PUT", appPath(id, "/access"), map[string]any{"restricted": true, "area_ids": []int64{}, "person_ids": []int64{}}, nil); code != 200 {
		t.Fatalf("restrict %d", code)
	}
	ana := createPersonAs(t, c, "Ana", "ana@alis.com", "usuario") // helper de agents_test.go; conferir a assinatura
	loginAs(t, c, ana.email, ana.password)
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&volta=%2F", nil)
	if !strings.HasSuffix(res.Header.Get("Location"), "notas.localhost:8080/__apphub/sem-acesso") {
		t.Fatalf("%s", res.Header.Get("Location"))
	}
}

func TestSharedAuthorizeTestBoxNeedsRestore(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	sharedApp(t, h, d, "notas")
	ana := createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	loginAs(t, c, ana.email, ana.password)
	c.noRedirect()
	res := c.raw("GET", "/api/shared/authorize?app=notas&teste=1&volta=%2F", nil)
	if !strings.HasSuffix(res.Header.Get("Location"), "notas--teste.localhost:8080/__apphub/sem-acesso") {
		t.Fatalf("%s", res.Header.Get("Location"))
	}
}

func TestGatewayBackendAllowed(t *testing.T) {
	c, h := newPoolClient(t)
	admin := setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	b := h.GatewayBackend()
	ident, ok, err := b.Allowed(context.Background(), admin.ID, id, false)
	if err != nil || !ok || ident.Email != admin.Email || ident.Role != "Administrador" {
		t.Fatalf("%+v %v %v", ident, ok, err)
	}
	tg, found, _ := b.Lookup(context.Background(), "notas", false)
	if !found || tg.AppID != id || tg.Status != "draft" || tg.Owner == "" {
		t.Fatalf("%+v", tg)
	}
	if _, found, _ := b.Lookup(context.Background(), "nada", false); found {
		t.Fatal("found an app that does not exist")
	}
	if _, ok, _ := b.Allowed(context.Background(), 9999, id, false); ok {
		t.Fatal("unknown person allowed")
	}
}
```

Se o `client` não tiver `raw` (pedido que devolve o `*http.Response`), acrescente em `handler_test.go`:

```go
func (c *client) raw(method, path string, body io.Reader) *http.Response {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.srv.URL+path, body)
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { res.Body.Close() })
	return res
}
```

- [ ] **Step 3: Rodar** — `cd backend && set -a && . ../.env && set +a && mise x -- go test ./internal/handler/ -run 'SharedAuthorize|GatewayBackend'` → FAIL.

- [ ] **Step 4: Implementar `shared_gateway.go`**

```go
package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"

	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/gateway"
	"github.com/HenryKropp/app-hub/backend/internal/shared/supervisor"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

type Shared struct {
	Svc     *deploy.Service
	Sup     *supervisor.Supervisor
	Vault   *vault.Vault
	DBs     appdb.DBs
	Eng     engine.Engine
	Codes   *gateway.Codes
	DataDir string
}

func (h *Handler) SetShared(s Shared) { h.shared = s }

// sharedReady answers 503 when the shared-app parts are not running.
func (h *Handler) sharedReady(w http.ResponseWriter) bool {
	if h.shared.Svc == nil {
		writeError(w, http.StatusServiceUnavailable, "O servidor de apps não está disponível.")
		return false
	}
	return true
}

func (h *Handler) sharedAuthorize(w http.ResponseWriter, r *http.Request) {
	if !h.sharedReady(w) {
		return
	}
	q := r.URL.Query()
	slug, test := q.Get("app"), q.Get("teste") == "1"
	volta := q.Get("volta")
	me, found, err := h.sessionPerson(r)
	if err != nil {
		internalError(w, "authorize session", err)
		return
	}
	if !found {
		http.Redirect(w, r, "/entrar?volta="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	sa, err := h.q.GetSharedAppBySlug(r.Context(), slug)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "App não encontrado.")
		return
	}
	if err != nil {
		internalError(w, "authorize shared app", err)
		return
	}
	app, err := h.q.GetApp(r.Context(), sa.AppID)
	if err != nil {
		internalError(w, "authorize app", err)
		return
	}
	acc, err := h.loadAccess(r.Context(), me)
	if err != nil {
		internalError(w, "authorize access", err)
		return
	}
	can := acc.For(appRef(app))
	base := h.shared.Svc.AppURL(slug)
	allowed := can.View
	if test {
		base, allowed = h.shared.Svc.TestURL(slug), can.Restore
	}
	if !allowed {
		http.Redirect(w, r, base+"/__apphub/sem-acesso", http.StatusFound)
		return
	}
	code := h.shared.Codes.Issue(me.ID, app.ID, test)
	http.Redirect(w, r, base+"/__apphub/entrar?"+url.Values{"code": {code}, "volta": {volta}}.Encode(), http.StatusFound)
}

// sessionPerson is currentPerson without writing a 401.
func (h *Handler) sessionPerson(r *http.Request) (sqlc.Person, bool, error) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return sqlc.Person{}, false, nil
	}
	p, err := h.q.GetSessionPerson(r.Context(), c.Value)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Person{}, false, nil
	}
	return p, err == nil, err
}

type gatewayBackend struct{ h *Handler }

func (h *Handler) GatewayBackend() gateway.Backend { return gatewayBackend{h} }

func (b gatewayBackend) Lookup(ctx context.Context, slug string, test bool) (gateway.Target, bool, error) {
	sa, err := b.h.q.GetSharedAppBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return gateway.Target{}, false, nil
	}
	if err != nil {
		return gateway.Target{}, false, err
	}
	app, err := b.h.q.GetApp(ctx, sa.AppID)
	if err != nil {
		return gateway.Target{}, false, err
	}
	t := gateway.Target{AppID: app.ID, Name: app.Name, Status: sa.Status}
	if owner, err := b.h.q.GetPerson(ctx, app.OwnerID); err == nil {
		t.Owner = owner.Name
	}
	if sa.StoppedBy.Valid {
		if p, err := b.h.q.GetPerson(ctx, sa.StoppedBy.Int64); err == nil {
			t.StoppedBy = p.Name
		}
	}
	if port, ok := b.h.shared.Svc.Target(app.ID, test); ok {
		t.Port = port
	}
	return t, true, nil
}

func (b gatewayBackend) Allowed(ctx context.Context, personID, appID int64, test bool) (gateway.Identity, bool, error) {
	p, err := b.h.q.GetPerson(ctx, personID)
	if errors.Is(err, pgx.ErrNoRows) {
		return gateway.Identity{}, false, nil
	}
	if err != nil {
		return gateway.Identity{}, false, err
	}
	app, err := b.h.q.GetApp(ctx, appID)
	if err != nil {
		return gateway.Identity{}, false, err
	}
	acc, err := b.h.loadAccess(ctx, p)
	if err != nil {
		return gateway.Identity{}, false, err
	}
	can := acc.For(appRef(app))
	id := gateway.Identity{ID: p.ID, Email: p.Email, Name: p.Name}
	if role, err := b.h.q.GetRole(ctx, p.RoleID); err == nil {
		id.Role = role.Name
	}
	if test {
		return id, can.Restore, nil
	}
	return id, can.View, nil
}

func (b gatewayBackend) Wake(appID int64) { b.h.shared.Svc.Wake(appID) }

func (b gatewayBackend) Touch(appID int64) {
	if b.h.shared.Sup != nil {
		b.h.shared.Sup.Touch(appID)
	}
}
```

Em `handler.go`: campo `shared Shared` no `Handler` e `mux.HandleFunc("GET /api/shared/authorize", h.sharedAuthorize)`. Conferir os nomes de `GetSessionPerson`, `GetPerson`, `GetRole` e do campo `RoleID` de `sqlc.Person`.

- [ ] **Step 5: Rodar** — os testes do Step 2 → PASS; `go test ./internal/handler/` inteiro → PASS.

- [ ] **Step 6: Login que volta para o app** — em `LoginPage.tsx`, depois do login (e também ao abrir a página já logado), se `?volta=` começa com `/api/shared/authorize`, ir para lá com `window.location.assign(volta)` (é um endereço do servidor, não uma rota do React). Qualquer outro `volta` é ignorado (segue o comportamento de hoje). Teste em `LoginPage.test.tsx`:

```tsx
it("goes back to the shared app after signing in", async () => {
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  mockApi({ login: person })            // usar o mesmo jeito de simular a API que os outros testes deste arquivo
  renderAt("/entrar?volta=%2Fapi%2Fshared%2Fauthorize%3Fapp%3Dnotas")
  await userEvent.type(screen.getByLabelText("E-mail"), "henry@alis.com")
  await userEvent.type(screen.getByLabelText("Senha"), "segredo123")
  await userEvent.click(screen.getByRole("button", { name: "Entrar" }))
  await waitFor(() => expect(assign).toHaveBeenCalledWith("/api/shared/authorize?app=notas"))
})

it("ignores a volta that is not a shared app", async () => {
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  mockApi({ login: person })
  renderAt("/entrar?volta=https%3A%2F%2Fevil.com")
  // … mesmo login …
  await waitFor(() => expect(assign).not.toHaveBeenCalled())
})
```

(Adapte `mockApi`/`renderAt` aos ajudantes que `LoginPage.test.tsx` já usa.) Implementação:

```tsx
const [params] = useSearchParams()
const volta = params.get("volta") ?? ""
const sharedVolta = volta.startsWith("/api/shared/authorize") ? volta : ""
// depois do login bem-sucedido, antes do navigate de hoje:
if (sharedVolta) { window.location.assign(sharedVolta); return }
```

e um `useEffect` que faz o mesmo quando `person` já existe.

- [ ] **Step 7: `main.go`** — montar as peças e mandar os subdomínios para o porteiro:

```go
	q := sqlc.New(pool)
	h := handler.New(q, cfg)
	h.Register(mux)

	// Apps compartilhados (desenho 2026-10-06). Sem HUB_SECRET_KEY ou sem
	// Podman o Hub liga normalmente; só os compartilhados ficam indisponíveis.
	v, err := vault.New(cfg.SecretKey)
	if err != nil {
		slog.Warn("shared apps without vault", "err", err)
		v = nil
	}
	eng := engine.NewPodman(cfg.ContainerEngine)
	adminPool := pool
	if cfg.AppsDBAdminURL != cfg.DatabaseURL {
		if adminPool, err = pgxpool.New(ctx, cfg.AppsDBAdminURL); err != nil {
			slog.Error("apps db admin", "err", err)
			os.Exit(1)
		}
	}
	adminURL, _ := url.Parse(cfg.AppsDBAdminURL)
	dbs := appdb.New(adminPool, cfg.AppsDBHost, eng, cfg.AppsDBContainer, adminURL.User.Username())
	hubURL, _ := url.Parse(cfg.DatabaseURL)
	if err := dbs.LockDown(ctx, strings.TrimPrefix(hubURL.Path, "/")); err != nil {
		slog.Warn("lock hub database", "err", err)
	}
	scheme := "http"
	if cfg.SecureCookies() {
		scheme = "https"
	}
	svc := deploy.New(q, eng, dbs, v, deploy.Options{DataDir: cfg.SharedDataDir, Domain: cfg.SharedDomain, Scheme: scheme})
	sup := supervisor.New(q, eng, svc)
	gw := gateway.New(cfg.SharedDomain, cfg.BaseURL, cfg.SecureCookies(), h.GatewayBackend())
	h.SetShared(handler.Shared{Svc: svc, Sup: sup, Vault: v, DBs: dbs, Eng: eng, Codes: gw.Codes, DataDir: cfg.SharedDataDir})
	go sup.Run(ctx)
	// … bloco do frontendDist como hoje …
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := gw.Match(r.Host); ok {
			gw.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: root, ReadHeaderTimeout: 10 * time.Second}
```

`cfg.BaseURL` é o endereço do Hub que a pessoa usa (no PC, `http://localhost:5173`, o Vite, que repassa `/api` ao Hub). O cookie de sessão do Hub vale para `localhost` em qualquer porta, então o `/api/shared/authorize` chamado pelo Vite enxerga a sessão; os subdomínios `*.localhost:8080` vão direto ao Hub na 8080.

- [ ] **Step 8: Chave e ambiente** — gerar a chave sem mostrar na tela e acrescentar ao `.env`:

```bash
cd backend && printf '\nHUB_SECRET_KEY="%s"\n' "$(mise x -- go run ./cmd/newkey)" >> ../.env
```

Conferir com `grep -c '^HUB_SECRET_KEY=' ../.env` (deve dar 1; nunca `cat` o arquivo). As outras variáveis ficam no padrão. Em `docs/INFRASTRUCTURE.md`, acrescentar uma seção "Apps compartilhados" com: Podman (rede `app-hub-apps`, rótulo `app-hub.app`), `SHARED_DATA_DIR` (`stage/`, `apps/<id>/data`, `backups/<id>/`), bancos `app_<id>` no `app-hub-db`, as variáveis do Task 3, e "subdomínios `*.localhost:8080` funcionam no Chrome/Edge sem configurar nada".

- [ ] **Step 9: Conferir de verdade (sem tocar no Hub do Henry)** — **não** reiniciar o Hub da porta 8080 nesta etapa. Compilar e rodar os testes: `go build ./... && go test ./...`; `cd frontend && mise x -- npx vitest run src/pages/LoginPage.test.tsx`. O teste de ponta a ponta é no Task 19, com o Henry.

- [ ] **Step 10: Commit** — `git add backend frontend/src/pages/LoginPage.tsx frontend/src/pages/LoginPage.test.tsx .env.example docs/INFRASTRUCTURE.md && git commit -m "Shared apps: Hub sign-in for app addresses and server wiring"` (o `.env` não vai para o git).

---

# Fase 6 — API

### Task 14: API dos apps compartilhados

**Files:**
- Create: `backend/internal/handler/shared.go` (estado, checagem, ajustes, testar, publicar, voltar, parar, ligar, deixar de compartilhar, andamento, mensagens, uso)
- Create: `backend/internal/handler/shared_config.go` (configurações e senhas)
- Create: `backend/internal/handler/shared_backups.go` (cópias)
- Create: `backend/internal/handler/shared_test.go`
- Modify: `backend/internal/handler/handler.go` (rotas), `backend/internal/handler/apps.go` (`shared` no resumo e na ficha)

**Interfaces:**
- Consumes: `h.shared` (Task 13), `readiness.Check`, `scanner.Scan`, `h.writeApp`, `h.appFor`, `deploy.*`.
- Produces (rotas; toda resposta de erro é `{"error": "<frase em português>"}`):

| Método e caminho | Quem | O que faz |
|---|---|---|
| `GET /api/apps/{id}/shared` | `can.View` | Estado: `{shared, url, test_url, test_open, live_version, latest_version, deployments}`; `deployments` e `build_log` só com `can.Tech` |
| `GET /api/apps/{id}/shared/check` | `can.Restore` | `readiness.Report` (lê a pasta) |
| `PUT /api/apps/{id}/shared` | `can.Restore` | Corpo `SharedSettings` (abaixo): grava o bloco `shared` do manifesto pelo `writeApp` (ficha + `app-hub.yaml` + histórico) e as marcações "estou ciente" |
| `POST /api/apps/{id}/shared/test` | `can.Restore` | Checagem precisa estar pronta → `deploy.Test` → `202 {deployment}` |
| `POST /api/apps/{id}/shared/publish` | `can.Restore` | Idem → `deploy.Publish` (primeira vez ou versão nova) |
| `GET /api/apps/{id}/shared/deployments/{dep}` | `can.Restore` | Andamento (`status`, `error_message`, `build_log` com `can.Tech`) |
| `POST /api/apps/{id}/shared/rollback` | `can.Restore` | `{with_data: bool}` |
| `POST /api/apps/{id}/shared/stop` · `/start` | `can.Restore` | Parar · Ligar/Religar |
| `POST /api/apps/{id}/shared/unshare` | `can.Restore` | Deixar de compartilhar |
| `GET /api/apps/{id}/shared/logs` | `can.Tech` | Últimas 200 linhas, com senhas mascaradas (`scanner.MaskSecrets`) |
| `GET /api/apps/{id}/shared/usage` | `can.Tech` | Pontos das últimas 24 h `{at, mem_mb, cpu}` |
| `GET /api/apps/{id}/shared/config` · `PUT` | `can.Restore` | Lista `{name, label, secret, required, has_value, value}` (`value` só das não secretas) · `{name, value}` grava (vazio apaga) e diz `needs_restart` |
| `GET /api/apps/{id}/shared/backups` · `POST` | `can.Restore` | Lista · fazer cópia agora |
| `POST /api/apps/{id}/shared/backups/{bid}/restore` | `can.Restore` | Restaurar |
| `GET /api/apps/{id}/shared/backups/{bid}/download` | `can.Restore` | Baixa o arquivo (`<slug>-<data>.dump`) |

```go
type SharedSettings struct {
	Enabled    bool              `json:"enabled"`
	AlwaysOn   bool              `json:"always_on"`
	IdleMin    int               `json:"idle_minutes"`
	MemoryMB   int               `json:"memory_mb"`
	CPUs       float64           `json:"cpus"`
	Port       int               `json:"port"`
	HealthPath string            `json:"health_path"`
	Database   string            `json:"database"`
	Command    string            `json:"command"`
	Env        []manifest.EnvVar `json:"env"`
	Acks       []string          `json:"acks"`
	UpdatedAt  pgtype.Timestamptz `json:"updated_at"` // mesma trava contra edição simultânea da ficha
}

type sharedState struct {
	Status        string  `json:"status"`
	StatusMessage string  `json:"status_message"`
	StoppedBy     string  `json:"stopped_by"`
	AlwaysOn      bool    `json:"always_on"`
	IdleMinutes   int32   `json:"idle_minutes"`
	MemoryMB      int32   `json:"memory_mb"`
	CPUs          float64 `json:"cpus"`
	WantsDB       bool    `json:"wants_db"`
	URL           string  `json:"url"`
	Acks          []string `json:"acks"`
	HasPrevious   bool    `json:"has_previous"`
}
// appSummary e appDetail ganham `Shared *sharedBrief json:"shared"`:
type sharedBrief struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	Slug   string `json:"slug"`
}
```

Mapa de erros do serviço para HTTP (uma função `sharedError(w, err)` em `shared.go`): `deploy.ErrBusy` → 409; `deploy.MissingSettings` → 400; `deploy.ErrNoVault`, `engine.ErrUnavailable` → 503 com "O servidor de apps não está disponível. Confira se o Podman está ligado." (só para o 2º); `deploy.ErrNoPrevious`, `deploy.ErrNotShared`, `deploy.ErrBadSlug` → 400; erro com mensagem em português vinda do serviço (`errors.New("Não há cópia…")`) → 400 com a própria mensagem; o resto → `internalError`.

- [ ] **Step 1: Testes que falham** (`shared_test.go`)

```go
package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func waitShared(t *testing.T, c *client, id int64, want string) map[string]any {
	t.Helper()
	var st struct {
		Shared map[string]any `json:"shared"`
	}
	for i := 0; i < 100; i++ {
		c.do("GET", appPath(id, "/shared"), nil, &st)
		if st.Shared != nil && st.Shared["status"] == want {
			return st.Shared
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("status never became %s: %+v", want, st.Shared)
	return nil
}

func TestSharedCheckAndPublish(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var rep struct {
		Ready bool `json:"ready"`
		Items []struct{ Key, Status string } `json:"items"`
	}
	if code := c.do("GET", appPath(id, "/shared/check"), nil, &rep); code != 200 || !rep.Ready {
		t.Fatalf("check %d %+v", code, rep)
	}
	var dep struct{ ID int64 `json:"id"` }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &dep); code != http.StatusAccepted || dep.ID == 0 {
		t.Fatalf("publish %d", code)
	}
	sh := waitShared(t, c, id, "running")
	if sh["url"] != "http://notas.localhost:8080" {
		t.Fatalf("%+v", sh)
	}
	var list []struct {
		Shared *struct{ Status, URL string } `json:"shared"`
	}
	c.do("GET", "/api/apps", nil, &list)
	if len(list) != 1 || list[0].Shared == nil || list[0].Shared.Status != "running" {
		t.Fatalf("catalog %+v", list)
	}
}

func TestSharedPublishTwiceIs409(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	release := make(chan struct{})
	d.svc.SetSpawn(func(f func()) { go func() { <-release; f() }() })
	defer close(release)
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &e); code != http.StatusConflict || !strings.Contains(e.Error, "em andamento") {
		t.Fatalf("%d %s", code, e.Error)
	}
}

func TestSharedPermissions(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	ana := createPersonAs(t, c, "Ana", "ana@alis.com", "usuario")
	loginAs(t, c, ana.email, ana.password)
	if code := c.do("GET", appPath(id, "/shared"), nil, nil); code != 200 {
		t.Fatalf("view %d", code)
	}
	for _, p := range []string{"/shared/check", "/shared/config", "/shared/backups"} {
		if code := c.do("GET", appPath(id, p), nil, nil); code != http.StatusForbidden {
			t.Errorf("GET %s: %d", p, code)
		}
	}
	for _, p := range []string{"/shared/publish", "/shared/test", "/shared/stop", "/shared/start", "/shared/unshare", "/shared/rollback"} {
		if code := c.do("POST", appPath(id, p), map[string]any{}, nil); code != http.StatusForbidden {
			t.Errorf("POST %s: %d", p, code)
		}
	}
	if code := c.do("GET", appPath(id, "/shared/logs"), nil, nil); code != http.StatusForbidden {
		t.Errorf("logs: %d", code)
	}
}

func TestSharedSettingsWriteManifest(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct{ UpdatedAt string `json:"updated_at"` }
	c.do("GET", appPath(id, ""), nil, &app)
	body := map[string]any{"enabled": true, "always_on": true, "memory_mb": 1024, "cpus": 1, "database": "postgres",
		"env": []map[string]any{{"name": "ADMIN_EMAILS", "label": "E-mails dos administradores", "required": true}},
		"acks": []string{"file_data"}, "updated_at": app.UpdatedAt}
	if code := c.do("PUT", appPath(id, "/shared"), body, nil); code != 200 {
		t.Fatalf("put %d", code)
	}
	var det struct {
		Manifest struct {
			Shared struct {
				AlwaysOn bool `json:"always_on"`
				MemoryMB int  `json:"memory_mb"`
			} `json:"shared"`
		} `json:"manifest"`
	}
	c.do("GET", appPath(id, ""), nil, &det)
	if !det.Manifest.Shared.AlwaysOn || det.Manifest.Shared.MemoryMB != 1024 {
		t.Fatalf("%+v", det.Manifest.Shared)
	}
	// configuração obrigatória vazia barra a publicação com a frase certa
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &e); code != 400 || e.Error != "Falta preencher: E-mails dos administradores." {
		t.Fatalf("%d %q", code, e.Error)
	}
}

func TestSharedSecretNeverReturned(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct{ UpdatedAt string `json:"updated_at"` }
	c.do("GET", appPath(id, ""), nil, &app)
	c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "updated_at": app.UpdatedAt,
		"env": []map[string]any{{"name": "SMTP_PASSWORD", "label": "Senha do e-mail", "secret": true}}}, nil)
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "SMTP_PASSWORD", "value": "s3nh@-forte"}, nil); code != 200 {
		t.Fatalf("set %d", code)
	}
	res := c.raw("GET", appPath(id, "/shared/config"), nil)
	buf := new(strings.Builder)
	io.Copy(buf, res.Body)
	if strings.Contains(buf.String(), "s3nh@-forte") || !strings.Contains(buf.String(), `"has_value":true`) {
		t.Fatalf("%s", buf.String())
	}
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "SMTP_PASSWORD", "value": "a\nb"}, nil); code != 400 {
		t.Fatalf("newline accepted: %d", code)
	}
	if code := c.do("PUT", appPath(id, "/shared/config"), map[string]string{"name": "_db_password", "value": "x"}, nil); code != 400 {
		t.Fatalf("internal name accepted: %d", code)
	}
}

func TestSharedEngineDown(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	d.eng.Down = true
	var e struct{ Error string }
	if code := c.do("POST", appPath(id, "/shared/publish"), nil, &e); code != 503 || !strings.Contains(e.Error, "Podman") {
		t.Fatalf("%d %q", code, e.Error)
	}
}

func TestSharedBackupsFlow(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	var app struct{ UpdatedAt string `json:"updated_at"` }
	c.do("GET", appPath(id, ""), nil, &app)
	c.do("PUT", appPath(id, "/shared"), map[string]any{"enabled": true, "database": "postgres", "updated_at": app.UpdatedAt}, nil)
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	waitShared(t, c, id, "running")
	var b struct{ ID int64 `json:"id"` }
	if code := c.do("POST", appPath(id, "/shared/backups"), nil, &b); code != 201 {
		t.Fatalf("backup %d", code)
	}
	res := c.raw("GET", appPath(id, "/shared/backups/"+itoa(b.ID)+"/download"), nil)
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Disposition"), "notas-") {
		t.Fatalf("download %d %s", res.StatusCode, res.Header.Get("Content-Disposition"))
	}
	if code := c.do("POST", appPath(id, "/shared/backups/"+itoa(b.ID)+"/restore"), nil, nil); code != 200 {
		t.Fatalf("restore %d", code)
	}
}
```

(importar `io` no arquivo.)

- [ ] **Step 2: Rodar** → FAIL.

- [ ] **Step 3: Implementar `shared.go`**

```go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/scanner"
	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy"
	"github.com/HenryKropp/app-hub/backend/internal/shared/engine"
	"github.com/HenryKropp/app-hub/backend/internal/shared/readiness"
	"github.com/HenryKropp/app-hub/backend/internal/versions"
)

func sharedError(w http.ResponseWriter, err error) {
	var miss deploy.MissingSettings
	switch {
	case errors.Is(err, deploy.ErrBusy):
		writeError(w, http.StatusConflict, err.Error())
	case errors.As(err, &miss):
		writeError(w, http.StatusBadRequest, miss.Error())
	case errors.Is(err, engine.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "O servidor de apps não está disponível. Confira se o Podman está ligado.")
	case errors.Is(err, deploy.ErrNoVault):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, deploy.ErrNoPrevious), errors.Is(err, deploy.ErrNotShared), errors.Is(err, deploy.ErrBadSlug):
		writeError(w, http.StatusBadRequest, err.Error())
	case isPortuguese(err):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		internalError(w, "shared", err)
	}
}

// isPortuguese: the service's own messages start with a capital letter and end
// with a period; technical errors (pgx, exec) do not.
func isPortuguese(err error) bool {
	m := err.Error()
	return m != "" && m[0] >= 'A' && m[0] <= 'Z' && strings.HasSuffix(m, ".") && !strings.Contains(m, ": ")
}

// sharedFor loads the app, checks a permission, and the shared row (if any).
func (h *Handler) sharedFor(w http.ResponseWriter, r *http.Request, need func(c accessCan) bool) (sqlc.App, sqlc.Person, accessCan, *sqlc.SharedApp, bool) {
	if !h.sharedReady(w) {
		return sqlc.App{}, sqlc.Person{}, accessCan{}, nil, false
	}
	a, me, _, can, ok := h.appFor(w, r)
	if !ok {
		return a, me, can, nil, false
	}
	if !need(can) {
		writeError(w, http.StatusForbidden, "Você não tem permissão para isso.")
		return a, me, can, nil, false
	}
	sa, err := h.q.GetSharedApp(r.Context(), a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, me, can, nil, true
	}
	if err != nil {
		internalError(w, "get shared app", err)
		return a, me, can, nil, false
	}
	return a, me, can, &sa, true
}

func canView(c accessCan) bool    { return c.View }
func canRestore(c accessCan) bool { return c.Restore }
func canTech(c accessCan) bool    { return c.Tech }
```

(`accessCan` é só um apelido local: `type accessCan = access.Can`.)

Os handlers seguem o padrão abaixo — escreva todos com este formato:

```go
func (h *Handler) getShared(w http.ResponseWriter, r *http.Request) {
	a, _, can, sa, ok := h.sharedFor(w, r, canView)
	if !ok {
		return
	}
	out := map[string]any{"shared": nil}
	if sa != nil && !sa.UnsharedAt.Valid {
		st := sharedState{Status: sa.Status, StatusMessage: sa.StatusMessage, AlwaysOn: sa.AlwaysOn, IdleMinutes: sa.IdleMinutes,
			MemoryMB: sa.MemoryMb, CPUs: numericFloat(sa.Cpus), WantsDB: sa.WantsDb, URL: h.shared.Svc.AppURL(sa.Slug),
			Acks: sa.Acks, HasPrevious: sa.PreviousDeploymentID.Valid}
		if sa.StoppedBy.Valid {
			if p, err := h.q.GetPerson(r.Context(), sa.StoppedBy.Int64); err == nil {
				st.StoppedBy = p.Name
			}
		}
		out["shared"] = st
		if can.Restore {
			out["test_url"] = h.shared.Svc.TestURL(sa.Slug)
			_, out["test_open"] = h.shared.Svc.Target(a.ID, true)
		}
		if sa.CurrentDeploymentID.Valid {
			if d, err := h.q.GetDeployment(r.Context(), sa.CurrentDeploymentID.Int64); err == nil && d.VersionID.Valid {
				if v, err := h.q.GetAppVersionByID(r.Context(), d.VersionID.Int64); err == nil {
					out["live_version"] = v.Number
				}
			}
		}
		if v, err := h.q.LatestAppVersion(r.Context(), a.ID); err == nil {
			out["latest_version"] = v.Number
		}
		if can.Restore {
			deps, _ := h.q.ListDeployments(r.Context(), sqlc.ListDeploymentsParams{AppID: a.ID, Limit: 10})
			if !can.Tech {
				for i := range deps {
					deps[i].BuildLog = ""
				}
			}
			out["deployments"] = deps
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) checkShared(w http.ResponseWriter, r *http.Request) {
	a, _, _, sa, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	rep, err := h.readiness(r, a, sa)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Não consegui ler a pasta do app. Confira se ela existe e está sincronizada.")
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (h *Handler) readiness(r *http.Request, a sqlc.App, sa *sqlc.SharedApp) (readiness.Report, error) {
	var m manifest.Manifest
	_ = json.Unmarshal(a.Manifest, &m)
	diag, err := scanner.Scan(a.FolderPath)
	if err != nil {
		return readiness.Report{}, err
	}
	var acks []string
	if sa != nil {
		acks = sa.Acks
	}
	return readiness.Check(versions.LocalFolder{Root: a.FolderPath}, m, diag.Secrets, acks)
}

func (h *Handler) startShared(w http.ResponseWriter, r *http.Request, test bool) {
	a, me, _, sa, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	rep, err := h.readiness(r, a, sa)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Não consegui ler a pasta do app. Confira se ela existe e está sincronizada.")
		return
	}
	if !rep.Ready {
		writeError(w, http.StatusBadRequest, "A checagem ainda tem pendências. Resolva ou marque \"estou ciente\" antes de continuar.")
		return
	}
	var m manifest.Manifest
	_ = json.Unmarshal(a.Manifest, &m)
	var d sqlc.SharedDeployment
	if test {
		d, err = h.shared.Svc.Test(r.Context(), a, m, me.ID)
	} else {
		d, err = h.shared.Svc.Publish(r.Context(), a, m, me.ID)
	}
	if err != nil {
		sharedError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, d)
}
```

Ordem das conferências em `startShared`: o motor (`Eng.Available`) é conferido **antes** da checagem, para que "Podman desligado" apareça como tal (o teste `TestSharedEngineDown` cobra isso): acrescente `if err := h.shared.Eng.Available(r.Context()); err != nil { sharedError(w, err); return }` logo depois de `sharedFor`.

O `PUT /shared` (`putShared`):
1. `sharedFor(canRestore)`; lê `SharedSettings`.
2. Se `Enabled == false` e o app tem versão no ar → 409 "Para deixar de compartilhar, use o botão Deixar de compartilhar."
3. Monta o manifesto atual com `m.Shared = &manifest.Shared{...}` dos campos do corpo (ou `nil` se `Enabled == false` e não há versão no ar).
4. Chama `h.writeApp(w, r, me, a.FolderPath, appBody{FolderPath: a.FolderPath, OwnerID: a.OwnerID, BackupID: backupPtr(a), Manifest: m}, &a)` — o mesmo caminho da edição da ficha (valida, grava `app-hub.yaml`/`FICHA.md`, registra o que mudou). Conferir como `updateApp` passa o `updated_at` para a trava de edição simultânea e fazer igual.
5. Se `Enabled`: `h.shared.Svc.Configure(ctx, saved, m)` e `SetSharedAcks(acks)` (só chaves conhecidas da checagem: `file_data`, `windows_paths`, `secrets`, `container_limits`).
6. Responde `h.writeDetail(...)` como o `updateApp`.

Rollback/stop/start/unshare: `sharedFor(canRestore)`, `sa == nil` → `ErrNotShared`, chamada ao serviço, `200 {}`.

`getDeployment`: confere que a implantação é do app da URL (`d.AppID == a.ID`, senão 404).

`sharedLogs`: `canTech`; container da versão no ar; `h.shared.Eng.Logs(ctx, id, 200)`; `scanner.MaskSecrets(text)`; `{"text": ...}`. Sem caixa ligada → `{"text": ""}`.

`sharedUsage`: `canTech`; `UsageSince(app, agora − 24 h)` → `[{at, mem_mb, cpu}]`.

- [ ] **Step 4: Implementar `shared_config.go`**

```go
package handler

// getSharedConfig lists the settings the manifest declares, with their state.
// Secret values never leave the Hub.
func (h *Handler) getSharedConfig(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	var m manifest.Manifest
	_ = json.Unmarshal(a.Manifest, &m)
	rows, err := h.q.ListAppConfig(r.Context(), a.ID)
	if err != nil {
		internalError(w, "list config", err)
		return
	}
	have := map[string]sqlc.AppConfig{}
	for _, row := range rows {
		have[row.Name] = row
	}
	type item struct {
		Name     string `json:"name"`
		Label    string `json:"label"`
		Secret   bool   `json:"secret"`
		Required bool   `json:"required"`
		HasValue bool   `json:"has_value"`
		Value    string `json:"value"`
		Default  string `json:"default"`
	}
	out := []item{}
	if m.Shared != nil {
		for _, e := range m.Shared.Env {
			row, ok := have[e.Name]
			it := item{Name: e.Name, Label: e.Label, Secret: e.Secret, Required: e.Required, Default: e.Default,
				HasValue: ok && (row.ValuePlain != "" || len(row.ValueCipher) > 0)}
			if ok && !e.Secret {
				it.Value = row.ValuePlain
			}
			out = append(out, it)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) putSharedConfig(w http.ResponseWriter, r *http.Request) {
	a, me, _, sa, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	var body struct{ Name, Value string }
	if !readJSON(w, r, &body) {
		return
	}
	var m manifest.Manifest
	_ = json.Unmarshal(a.Manifest, &m)
	var decl *manifest.EnvVar
	if m.Shared != nil {
		for i := range m.Shared.Env {
			if m.Shared.Env[i].Name == body.Name {
				decl = &m.Shared.Env[i]
			}
		}
	}
	if decl == nil {
		writeError(w, http.StatusBadRequest, "Essa configuração não está na lista do app.")
		return
	}
	if strings.ContainsAny(body.Value, "\r\n") {
		writeError(w, http.StatusBadRequest, "O valor não pode ter quebra de linha.")
		return
	}
	if len(body.Value) > 4096 {
		writeError(w, http.StatusBadRequest, "O valor passa de 4.096 caracteres.")
		return
	}
	if body.Value == "" {
		err := h.q.DeleteAppConfig(r.Context(), sqlc.DeleteAppConfigParams{AppID: a.ID, Name: body.Name})
		if err != nil {
			internalError(w, "delete config", err)
			return
		}
	} else {
		p := sqlc.UpsertAppConfigParams{AppID: a.ID, Name: body.Name, IsSecret: decl.Secret, UpdatedBy: pgtype.Int8{Int64: me.ID, Valid: true}}
		if decl.Secret {
			if h.shared.Vault == nil {
				sharedError(w, deploy.ErrNoVault)
				return
			}
			sealed, err := h.shared.Vault.Seal(body.Value)
			if err != nil {
				internalError(w, "seal", err)
				return
			}
			p.ValueCipher = sealed
		} else {
			p.ValuePlain = body.Value
		}
		if err := h.q.UpsertAppConfig(r.Context(), p); err != nil {
			internalError(w, "upsert config", err)
			return
		}
	}
	running := sa != nil && sa.Status == "running"
	writeJSON(w, http.StatusOK, map[string]bool{"needs_restart": running})
}
```

Nomes começando com `_` nunca estão declarados no manifesto (a validação do Task 2 exige letra maiúscula no começo), então o `_db_password` é recusado pela regra "não está na lista".

- [ ] **Step 5: Implementar `shared_backups.go`** — `listSharedBackups` (`ListBackups`, sem o campo `path` na resposta: monte um struct `{id, kind, size_bytes, created_at, created_by_name, version_number}`), `createSharedBackup` (`Svc.Backup(ctx, id, "manual", nil, &me.ID)` → 201), `restoreSharedBackup` (`Svc.Restore`), `downloadSharedBackup`:

```go
func (h *Handler) downloadSharedBackup(w http.ResponseWriter, r *http.Request) {
	a, _, _, _, ok := h.sharedFor(w, r, canRestore)
	if !ok {
		return
	}
	bid, err := strconv.ParseInt(r.PathValue("bid"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "Cópia não encontrada.")
		return
	}
	b, err := h.q.GetBackup(r.Context(), sqlc.GetBackupParams{ID: bid, AppID: a.ID})
	if err != nil {
		writeError(w, http.StatusNotFound, "Cópia não encontrada.")
		return
	}
	// O arquivo precisa estar dentro da pasta de cópias do Hub.
	if !strings.HasPrefix(filepath.Clean(b.Path), filepath.Join(h.shared.DataDir, "backups")+string(filepath.Separator)) &&
		!strings.HasPrefix(filepath.Clean(b.Path), filepath.Clean(h.shared.DataDir)) {
		writeError(w, http.StatusNotFound, "Cópia não encontrada.")
		return
	}
	name := a.Slug + "-" + b.CreatedAt.Time.Format("2006-01-02-1504") + ".dump"
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, b.Path)
}
```

(Nos testes o `DataDir` do serviço e o do handler são pastas diferentes; faça `withShared` passar a **mesma** pasta para os dois.)

- [ ] **Step 6: Resumo e ficha** — em `apps.go`, `listApps` busca `ListSharedApps` uma vez e preenche `Shared *sharedBrief` dos apps compartilhados (status, `AppURL` e slug); `getApp`/`writeDetail` faz o mesmo com `GetSharedApp`. Sem `h.shared.Svc` (testes antigos), `Shared` fica `nil`.

- [ ] **Step 7: Rotas** em `handler.go`:

```go
	mux.HandleFunc("GET /api/apps/{id}/shared", h.getShared)
	mux.HandleFunc("PUT /api/apps/{id}/shared", h.putShared)
	mux.HandleFunc("GET /api/apps/{id}/shared/check", h.checkShared)
	mux.HandleFunc("POST /api/apps/{id}/shared/test", func(w http.ResponseWriter, r *http.Request) { h.startShared(w, r, true) })
	mux.HandleFunc("POST /api/apps/{id}/shared/publish", func(w http.ResponseWriter, r *http.Request) { h.startShared(w, r, false) })
	mux.HandleFunc("GET /api/apps/{id}/shared/deployments/{dep}", h.getSharedDeployment)
	mux.HandleFunc("POST /api/apps/{id}/shared/rollback", h.rollbackShared)
	mux.HandleFunc("POST /api/apps/{id}/shared/stop", h.stopShared)
	mux.HandleFunc("POST /api/apps/{id}/shared/start", h.startSharedApp)
	mux.HandleFunc("POST /api/apps/{id}/shared/unshare", h.unshareShared)
	mux.HandleFunc("GET /api/apps/{id}/shared/logs", h.sharedLogs)
	mux.HandleFunc("GET /api/apps/{id}/shared/usage", h.sharedUsage)
	mux.HandleFunc("GET /api/apps/{id}/shared/config", h.getSharedConfig)
	mux.HandleFunc("PUT /api/apps/{id}/shared/config", h.putSharedConfig)
	mux.HandleFunc("GET /api/apps/{id}/shared/backups", h.listSharedBackups)
	mux.HandleFunc("POST /api/apps/{id}/shared/backups", h.createSharedBackup)
	mux.HandleFunc("POST /api/apps/{id}/shared/backups/{bid}/restore", h.restoreSharedBackup)
	mux.HandleFunc("GET /api/apps/{id}/shared/backups/{bid}/download", h.downloadSharedBackup)
```

- [ ] **Step 8: Rodar** — `go test ./internal/handler/ -count=1` → PASS (os antigos também). `go vet ./...`.

- [ ] **Step 9: Commit** — `git add backend && git commit -m "Shared apps: API for the ficha (check, publish, roll back, settings, copies, logs, usage)"`

---

### Task 15: Servidor e gastos (API)

**Files:**
- Create: `backend/internal/shared/costs/costs.go`, `costs_test.go` — conta pura
- Create: `backend/internal/shared/hostinfo/hostinfo.go`, `hostinfo_test.go` — memória e disco do servidor
- Create: `backend/internal/handler/server.go`, `server_test.go`
- Modify: `backend/internal/handler/handler.go` (rotas)

**Interfaces:**
- Produces:

```go
package costs

type AppUse struct {
	AppID    int64
	MemoryMB int   // memória reservada
	Minutes  int64 // minutos ligado no período
}

// Estimate splits what the server cost in the period among the apps by
// reserved memory × time on. budget = monthly × elapsed / month; an app's
// share = budget × (mem × minutes) / (totalMem × elapsed). If the
// reservations add up to more than the server (overcommit), shares are
// scaled down so they never pass the budget. What is left is the Hub's.
func Estimate(monthlyCents int64, totalMemMB int, elapsed, month time.Duration, apps []AppUse) (map[int64]int64, int64)

package hostinfo
type Info struct{ MemTotal, MemUsed, DiskTotal, DiskUsed int64 }
func Read(dataDir string) (Info, error)         // /proc/meminfo + statfs da pasta
func parseMeminfo(text string) (total, used int64, err error)
```

Rotas (permissão `manage_server` via `h.requirePerm`):

| Método e caminho | O que faz |
|---|---|
| `GET /api/server` | `{memory_total, memory_used, disk_total, disk_used, disk_warning, engine_ok, monthly_cost_cents, hub_cents, period_start, apps: [...]}` |
| `PUT /api/server/cost` | `{monthly_cost_cents}` (≥ 0) |
| `PUT /api/server/apps/{id}` | `{memory_mb, cpus, always_on, idle_minutes}` → grava no manifesto (mesmo caminho do `PUT /shared`) e responde `needs_restart` se o app está ligado |
| `POST /api/server/apps/{id}/stop` · `/start` | Parar/ligar qualquer app compartilhado |

Cada item de `apps`: `{app_id, name, slug, status, always_on, memory_mb, cpus, hours_on, avg_mem_mb, max_mem_mb, avg_cpu, db_bytes, backup_bytes, cost_cents}`. O período é o mês corrente até agora (`period_start` = dia 1 às 00:00, hora local).

- [ ] **Step 1: Testes que falham**

`costs_test.go`:

```go
package costs

import (
	"testing"
	"time"
)

const day = 24 * time.Hour

func TestSplitByMemoryAndTime(t *testing.T) {
	// servidor de R$ 300/mês com 4 GB; metade do mês passou → orçamento R$ 150
	per, hub := Estimate(30000, 4096, 15*day, 30*day, []AppUse{
		{AppID: 1, MemoryMB: 1024, Minutes: int64((15 * day).Minutes())},     // ligado o tempo todo, 1/4 da memória
		{AppID: 2, MemoryMB: 512, Minutes: int64((15 * day).Minutes()) / 2}, // metade do tempo, 1/8
	})
	if per[1] != 3750 || per[2] != 938 || hub != 15000-3750-938 {
		t.Fatalf("per %v hub %d", per, hub)
	}
}

func TestOvercommitNeverPassesBudget(t *testing.T) {
	per, hub := Estimate(10000, 1024, 30*day, 30*day, []AppUse{
		{AppID: 1, MemoryMB: 1024, Minutes: int64((30 * day).Minutes())},
		{AppID: 2, MemoryMB: 1024, Minutes: int64((30 * day).Minutes())},
	})
	if per[1]+per[2] > 10000 || hub < 0 || per[1] != per[2] {
		t.Fatalf("per %v hub %d", per, hub)
	}
}

func TestNoCostOrNoTime(t *testing.T) {
	per, hub := Estimate(0, 4096, 10*day, 30*day, []AppUse{{AppID: 1, MemoryMB: 512, Minutes: 100}})
	if per[1] != 0 || hub != 0 {
		t.Fatal("zero cost should split nothing")
	}
	per, _ = Estimate(30000, 4096, 0, 30*day, []AppUse{{AppID: 1, MemoryMB: 512}})
	if per[1] != 0 {
		t.Fatal("no elapsed time")
	}
}
```

`hostinfo_test.go`:

```go
package hostinfo

import "testing"

func TestParseMeminfo(t *testing.T) {
	total, used, err := parseMeminfo("MemTotal:       16303072 kB\nMemFree:  1000 kB\nMemAvailable:   12000000 kB\n")
	if err != nil || total != 16303072*1024 || used != (16303072-12000000)*1024 {
		t.Fatalf("%d %d %v", total, used, err)
	}
}

func TestReadWorksHere(t *testing.T) {
	info, err := Read(t.TempDir())
	if err != nil || info.MemTotal == 0 || info.DiskTotal == 0 {
		t.Fatalf("%+v %v", info, err)
	}
}
```

`server_test.go`:

```go
package handler

import (
	"net/http"
	"testing"
)

func TestServerNeedsManageServer(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	withShared(t, h)
	if code := c.do("GET", "/api/server", nil, nil); code != 200 {
		t.Fatalf("admin %d", code)
	}
	ana := createPersonAs(t, c, "Ana", "ana@alis.com", "criador")
	loginAs(t, c, ana.email, ana.password)
	if code := c.do("GET", "/api/server", nil, nil); code != http.StatusForbidden {
		t.Fatalf("criador %d", code)
	}
}

func TestServerCostAndApps(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	if code := c.do("PUT", "/api/server/cost", map[string]int64{"monthly_cost_cents": 30000}, nil); code != 200 {
		t.Fatalf("cost %d", code)
	}
	if code := c.do("PUT", "/api/server/cost", map[string]int64{"monthly_cost_cents": -1}, nil); code != 400 {
		t.Fatalf("negative cost %d", code)
	}
	testPool.Exec(t.Context(), "INSERT INTO shared_usage (app_id, at, mem_bytes, cpu_pct) SELECT $1, date_trunc('minute', now()) - (g || ' minutes')::interval, 200000000, 3 FROM generate_series(0, 59) g", id)
	var out struct {
		MonthlyCostCents int64 `json:"monthly_cost_cents"`
		Apps             []struct {
			AppID     int64   `json:"app_id"`
			HoursOn   float64 `json:"hours_on"`
			CostCents int64   `json:"cost_cents"`
			MaxMemMB  int64   `json:"max_mem_mb"`
		} `json:"apps"`
	}
	c.do("GET", "/api/server", nil, &out)
	if out.MonthlyCostCents != 30000 || len(out.Apps) != 1 || out.Apps[0].HoursOn != 1 || out.Apps[0].MaxMemMB != 190 || out.Apps[0].CostCents <= 0 {
		t.Fatalf("%+v", out)
	}
}

func TestServerStopsAnyApp(t *testing.T) {
	c, h := newPoolClient(t)
	setupAdmin(t, c)
	d := withShared(t, h)
	id := sharedApp(t, h, d, "notas")
	c.do("POST", appPath(id, "/shared/publish"), nil, nil)
	waitShared(t, c, id, "running")
	if code := c.do("POST", "/api/server/apps/"+itoa(id)+"/stop", nil, nil); code != 200 {
		t.Fatalf("stop %d", code)
	}
	waitShared(t, c, id, "stopped")
}
```

(200000000 bytes = 190 MB arredondando para baixo por `/ (1 << 20)`.)

- [ ] **Step 2: Rodar** → FAIL.

- [ ] **Step 3: Implementar `costs.go`**

```go
// Package costs estimates how much of the server's monthly cost each shared
// app used, by reserved memory × time on.
package costs

import "time"

type AppUse struct {
	AppID    int64
	MemoryMB int
	Minutes  int64
}

func Estimate(monthlyCents int64, totalMemMB int, elapsed, month time.Duration, apps []AppUse) (map[int64]int64, int64) {
	per := map[int64]int64{}
	if monthlyCents <= 0 || elapsed <= 0 || month <= 0 || totalMemMB <= 0 {
		for _, a := range apps {
			per[a.AppID] = 0
		}
		return per, 0
	}
	budget := float64(monthlyCents) * elapsed.Minutes() / month.Minutes()
	capacity := float64(totalMemMB) * elapsed.Minutes()
	var used float64
	for _, a := range apps {
		used += float64(a.MemoryMB) * float64(a.Minutes)
	}
	scale := 1.0
	if used > capacity {
		scale = capacity / used
	}
	var sum int64
	for _, a := range apps {
		share := budget * float64(a.MemoryMB) * float64(a.Minutes) / capacity * scale
		per[a.AppID] = int64(share + 0.5)
		sum += per[a.AppID]
	}
	hub := int64(budget+0.5) - sum
	if hub < 0 {
		hub = 0
	}
	return per, hub
}
```

- [ ] **Step 4: Implementar `hostinfo.go`**

```go
// Package hostinfo reads the server's memory and disk (Linux).
package hostinfo

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type Info struct{ MemTotal, MemUsed, DiskTotal, DiskUsed int64 }

func Read(dataDir string) (Info, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return Info{}, err
	}
	total, used, err := parseMeminfo(string(b))
	if err != nil {
		return Info{}, err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return Info{}, err
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &st); err != nil {
		return Info{}, err
	}
	dt := int64(st.Blocks) * st.Bsize
	return Info{MemTotal: total, MemUsed: used, DiskTotal: dt, DiskUsed: dt - int64(st.Bavail)*st.Bsize}, nil
}

func parseMeminfo(text string) (int64, int64, error) {
	vals := map[string]int64{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 2 {
			n, _ := strconv.ParseInt(f[1], 10, 64)
			vals[strings.TrimSuffix(f[0], ":")] = n * 1024
		}
	}
	t, a := vals["MemTotal"], vals["MemAvailable"]
	if t == 0 {
		return 0, 0, errors.New("meminfo sem MemTotal")
	}
	return t, t - a, nil
}
```

(`hostinfo.go` com `//go:build linux`; o Hub roda em Linux no PC e na nuvem.)

- [ ] **Step 5: Implementar `server.go`** — `getServer`:

```go
func (h *Handler) getServer(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.requirePerm(w, r, access.ManageServer); !ok {
		return
	}
	if !h.sharedReady(w) {
		return
	}
	ctx := r.Context()
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthLen := start.AddDate(0, 1, 0).Sub(start)
	info, err := hostinfo.Read(h.shared.DataDir)
	if err != nil {
		slog.Warn("hostinfo", "err", err)
	}
	settings, err := h.q.GetServerSettings(ctx)
	if err != nil {
		internalError(w, "server settings", err)
		return
	}
	apps, err := h.q.ListSharedApps(ctx)
	if err != nil {
		internalError(w, "list shared", err)
		return
	}
	totals, _ := h.q.UsageTotalsSince(ctx, pgtype.Timestamptz{Time: start, Valid: true})
	byApp := map[int64]sqlc.UsageTotalsSinceRow{}
	for _, t := range totals {
		byApp[t.AppID] = t
	}
	backupBytes := map[int64]int64{}
	if sums, err := h.q.SumBackupBytes(ctx); err == nil {
		for _, s := range sums {
			backupBytes[s.AppID] = s.Total
		}
	}
	var uses []costs.AppUse
	for _, a := range apps {
		uses = append(uses, costs.AppUse{AppID: a.AppID, MemoryMB: int(a.MemoryMb), Minutes: byApp[a.AppID].Minutes})
	}
	per, hub := costs.Estimate(settings.MonthlyCostCents, int(info.MemTotal>>20), now.Sub(start), monthLen, uses)
	type row struct {
		AppID       int64   `json:"app_id"`
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Status      string  `json:"status"`
		AlwaysOn    bool    `json:"always_on"`
		MemoryMB    int32   `json:"memory_mb"`
		CPUs        float64 `json:"cpus"`
		IdleMinutes int32   `json:"idle_minutes"`
		HoursOn     float64 `json:"hours_on"`
		AvgMemMB    int64   `json:"avg_mem_mb"`
		MaxMemMB    int64   `json:"max_mem_mb"`
		AvgCPU      float32 `json:"avg_cpu"`
		DBBytes     int64   `json:"db_bytes"`
		BackupBytes int64   `json:"backup_bytes"`
		CostCents   int64   `json:"cost_cents"`
	}
	out := []row{}
	for _, a := range apps {
		u := byApp[a.AppID]
		var dbBytes int64
		if a.DbName != "" {
			dbBytes, _ = h.shared.DBs.Size(ctx, a.DbName)
		}
		out = append(out, row{AppID: a.AppID, Name: a.AppName, Slug: a.Slug, Status: a.Status, AlwaysOn: a.AlwaysOn,
			MemoryMB: a.MemoryMb, CPUs: numericFloat(a.Cpus), IdleMinutes: a.IdleMinutes,
			HoursOn: math.Round(float64(u.Minutes)/60*10) / 10, AvgMemMB: u.AvgMem >> 20, MaxMemMB: u.MaxMem >> 20, AvgCPU: u.AvgCpu,
			DBBytes: dbBytes, BackupBytes: backupBytes[a.AppID], CostCents: per[a.AppID]})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"memory_total": info.MemTotal, "memory_used": info.MemUsed, "disk_total": info.DiskTotal, "disk_used": info.DiskUsed,
		"disk_warning": info.DiskTotal > 0 && float64(info.DiskUsed)/float64(info.DiskTotal) > 0.9,
		"engine_ok":    h.shared.Eng.Available(ctx) == nil,
		"monthly_cost_cents": settings.MonthlyCostCents, "hub_cents": hub, "period_start": start, "apps": out,
	})
}
```

`putServerCost` (`requirePerm(ManageServer)`, valida ≥ 0 e ≤ 100 milhões de centavos, `SetServerCost`), `putServerApp` (valida os limites com as mesmas faixas do Task 2; carrega o app; troca `memory_mb/cpus/always_on/idle_minutes` no bloco `shared` do manifesto e grava pelo `writeApp`, como o `PUT /shared`; chama `Svc.Configure`), `stopServerApp`/`startServerApp` (`Svc.Stop`/`Svc.Start`). Quem tem `manage_server` age sobre qualquer app compartilhado, mesmo sem enxergar o app no catálogo (é o papel de cuidar do servidor) — registrar isso no ADR 010.

`numericFloat(pgtype.Numeric) float64` fica num `helpers` do handler (igual ao `float` do deploy).

- [ ] **Step 6: Rotas** — `GET /api/server`, `PUT /api/server/cost`, `PUT /api/server/apps/{id}`, `POST /api/server/apps/{id}/stop`, `POST /api/server/apps/{id}/start`.

- [ ] **Step 7: Rodar** — `go test ./internal/shared/costs/ ./internal/shared/hostinfo/ ./internal/handler/ -count=1` → PASS.

- [ ] **Step 8: Commit** — `git add backend && git commit -m "Shared apps: server usage and cost estimate API"`


---

# Fase 7 — Telas

Regras para todas as telas (vêm do `DESIGN.md` e do `.impeccable.md`): fonte Geist, cores pelos tokens do tema (nada de cor solta), claro e escuro, celular sem rolagem para o lado, frases em português simples, sem jargão ("caixa", "porta", "container" só aparecem dentro do "Para quem cuida do app"). A cor dos compartilhados é o violeta do tipo "web" de `lib/kinds.ts` — conferir o token e usar o mesmo.

### Task 16: Catálogo — "Abrir" e o pontinho de estado

**Files:**
- Create: `frontend/src/lib/shared.ts`, `frontend/src/lib/shared.test.ts`
- Create: `frontend/src/components/SharedStatus.tsx`, `frontend/src/components/SharedStatus.test.tsx`
- Modify: `frontend/src/lib/api.ts` (tipos e chamadas), `frontend/src/lib/permissions.ts` (`canSeeServer`), `frontend/src/pages/CatalogPage.tsx`, `frontend/src/pages/CatalogPage.test.tsx`

**Interfaces:**
- Consumes: as rotas do Task 14 e 15 (formato JSON exatamente como lá).
- Produces:

```ts
// api.ts
export type SharedStatus = "draft" | "building" | "running" | "sleeping" | "stopped" | "crashed"
export type SharedBrief = { status: SharedStatus; url: string; slug: string }
// AppSummary e AppDetail ganham: shared: SharedBrief | null
export type SharedState = {
  status: SharedStatus; status_message: string; stopped_by: string; always_on: boolean; idle_minutes: number
  memory_mb: number; cpus: number; wants_db: boolean; url: string; acks: string[]; has_previous: boolean
}
export type Deployment = {
  id: number; kind: "test" | "live"; status: "building" | "starting" | "live" | "failed" | "retired" | "ended"
  version_id: number | null; error_message: string; build_log: string; created_by_name: string; created_at: string
}
export type SharedInfo = {
  shared: SharedState | null; test_url?: string; test_open?: boolean
  live_version?: number; latest_version?: number; deployments?: Deployment[]
}
export type ReadinessItem = { key: string; status: "ok" | "auto" | "missing" | "warn"; title: string; detail: string; claude_text: string; acked: boolean }
export type Readiness = { ready: boolean; recipe: { kind: string; command: string }; wants_db: boolean; suggest_env: EnvVar[]; items: ReadinessItem[] }
export type SharedSettings = {
  enabled: boolean; always_on: boolean; idle_minutes: number; memory_mb: number; cpus: number; port: number
  health_path: string; database: "" | "postgres"; command: string; env: EnvVar[]; acks: string[]; updated_at: string
}
export type EnvVar = { name: string; label: string; secret: boolean; required: boolean; default: string }
export type ConfigItem = EnvVar & { has_value: boolean; value: string }
export type SharedBackup = { id: number; kind: "daily" | "manual" | "pre_deploy" | "pre_restore"; size_bytes: number; created_at: string; created_by_name: string; version_number: number | null }
export type UsagePoint = { at: string; mem_mb: number; cpu: number }
export type ServerApp = {
  app_id: number; name: string; slug: string; status: SharedStatus; always_on: boolean; memory_mb: number; cpus: number; idle_minutes: number
  hours_on: number; avg_mem_mb: number; max_mem_mb: number; avg_cpu: number; db_bytes: number; backup_bytes: number; cost_cents: number
}
export type ServerInfo = {
  memory_total: number; memory_used: number; disk_total: number; disk_used: number; disk_warning: boolean; engine_ok: boolean
  monthly_cost_cents: number; hub_cents: number; period_start: string; apps: ServerApp[]
}

// api.shared.*
shared: {
  get: (id: number) => request<SharedInfo>("GET", `/api/apps/${id}/shared`),
  check: (id: number) => request<Readiness>("GET", `/api/apps/${id}/shared/check`),
  save: (id: number, s: SharedSettings) => request<AppDetail>("PUT", `/api/apps/${id}/shared`, s),
  test: (id: number) => request<Deployment>("POST", `/api/apps/${id}/shared/test`),
  publish: (id: number) => request<Deployment>("POST", `/api/apps/${id}/shared/publish`),
  deployment: (id: number, dep: number) => request<Deployment>("GET", `/api/apps/${id}/shared/deployments/${dep}`),
  rollback: (id: number, withData: boolean) => request<object>("POST", `/api/apps/${id}/shared/rollback`, { with_data: withData }),
  stop: (id: number) => request<object>("POST", `/api/apps/${id}/shared/stop`),
  start: (id: number) => request<object>("POST", `/api/apps/${id}/shared/start`),
  unshare: (id: number) => request<object>("POST", `/api/apps/${id}/shared/unshare`),
  logs: (id: number) => request<{ text: string }>("GET", `/api/apps/${id}/shared/logs`),
  usage: (id: number) => request<UsagePoint[]>("GET", `/api/apps/${id}/shared/usage`),
  config: (id: number) => request<ConfigItem[]>("GET", `/api/apps/${id}/shared/config`),
  setConfig: (id: number, name: string, value: string) => request<{ needs_restart: boolean }>("PUT", `/api/apps/${id}/shared/config`, { name, value }),
  backups: (id: number) => request<SharedBackup[]>("GET", `/api/apps/${id}/shared/backups`),
  backup: (id: number) => request<SharedBackup>("POST", `/api/apps/${id}/shared/backups`),
  restore: (id: number, bid: number) => request<object>("POST", `/api/apps/${id}/shared/backups/${bid}/restore`),
},
server: {
  get: () => request<ServerInfo>("GET", "/api/server"),
  setCost: (cents: number) => request<object>("PUT", "/api/server/cost", { monthly_cost_cents: cents }),
  setApp: (id: number, l: { memory_mb: number; cpus: number; always_on: boolean; idle_minutes: number }) =>
    request<{ needs_restart: boolean }>("PUT", `/api/server/apps/${id}`, l),
  stop: (id: number) => request<object>("POST", `/api/server/apps/${id}/stop`),
  start: (id: number) => request<object>("POST", `/api/server/apps/${id}/start`),
},
export const backupDownloadUrl = (id: number, bid: number) => `/api/apps/${id}/shared/backups/${bid}/download`
export const openSharedUrl = (slug: string, test = false) =>
  `/api/shared/authorize?app=${encodeURIComponent(slug)}${test ? "&teste=1" : ""}&volta=%2F`

// lib/shared.ts
export const SHARED_STATUS: Record<SharedStatus, { label: string; tone: "ok" | "live" | "muted" | "bad" | "draft" }>
export function sharedLine(s: SharedState, liveVersion?: number): string
export function formatCents(cents: number): string   // "R$ 12,34"
export function formatBytes(n: number): string        // reaproveita formatSize de runs.ts
export function claudeTextFor(items: ReadinessItem[]): string

// permissions.ts
export const canSeeServer = (p: Person | null | undefined) => has(p, "manage_server")
```

O link "Abrir" passa sempre pelo Hub (`openSharedUrl`), não direto pelo endereço do app: o Hub confere a sessão e entrega o código de entrada. Isso evita a página do app pedir login de novo.

- [ ] **Step 1: Testes que falham** — `shared.test.ts`:

```ts
import { describe, expect, it } from "vitest"
import { SHARED_STATUS, claudeTextFor, formatCents, sharedLine } from "./shared"
import type { SharedState } from "./api"

const base: SharedState = { status: "running", status_message: "", stopped_by: "", always_on: false, idle_minutes: 30,
  memory_mb: 512, cpus: 0.5, wants_db: true, url: "http://notas.localhost:8080", acks: [], has_previous: false }

describe("sharedLine", () => {
  it("says it is on and which version", () => {
    expect(sharedLine(base, 4)).toBe("No ar · versão 4")
  })
  it("explains sleeping in plain words", () => {
    expect(sharedLine({ ...base, status: "sleeping" }, 4)).toBe("Descansando · acorda em segundos quando alguém abre")
  })
  it("names who stopped it", () => {
    expect(sharedLine({ ...base, status: "stopped", stopped_by: "Henry" })).toBe("Parado por Henry")
  })
  it("says when it fell", () => {
    expect(sharedLine({ ...base, status: "crashed" })).toBe("Caiu e não voltou sozinho")
  })
  it("has a label for every status", () => {
    for (const s of ["draft", "building", "running", "sleeping", "stopped", "crashed"] as const) expect(SHARED_STATUS[s].label).not.toBe("")
  })
})

it("formats money in reais", () => {
  expect(formatCents(123456)).toBe("R$ 1.234,56")
  expect(formatCents(0)).toBe("R$ 0,00")
})

it("joins the Claude texts of what is missing", () => {
  const t = claudeTextFor([
    { key: "port", status: "missing", title: "Porta", detail: "", claude_text: "Faça o app ler a porta da variável PORT.", acked: false },
    { key: "recipe", status: "ok", title: "Receita", detail: "", claude_text: "", acked: false },
    { key: "file_data", status: "warn", title: "Dados", detail: "", claude_text: "Guarde os dados em APPHUB_DATA_DIR.", acked: false },
  ])
  expect(t).toContain("Faça o app ler a porta")
  expect(t).toContain("APPHUB_DATA_DIR")
  expect(t.startsWith("Estou preparando este app para rodar compartilhado no App Hub.")).toBe(true)
})
```

`SharedStatus.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react"
import { it, expect } from "vitest"
import { SharedStatusDot } from "./SharedStatus"

it("shows the status in words for screen readers", () => {
  render(<SharedStatusDot status="sleeping" />)
  expect(screen.getByText("Descansando")).toBeInTheDocument()
})
```

`CatalogPage.test.tsx` (acrescentar, no mesmo jeito de simular a API que o arquivo já usa):

```tsx
it("shows Abrir for a shared app instead of Rodar", async () => {
  mockApps([{ ...webApp, can_run: true, shared: { status: "running", url: "http://notas.localhost:8080", slug: "notas" } }])
  renderCatalog()
  const link = await screen.findByRole("link", { name: "Abrir Notas" })
  expect(link).toHaveAttribute("href", "/api/shared/authorize?app=notas&volta=%2F")
  expect(link).toHaveAttribute("target", "_blank")
  expect(screen.queryByRole("button", { name: "Rodar Notas" })).not.toBeInTheDocument()
  expect(screen.getByText("No ar")).toBeInTheDocument()
})

it("does not offer Abrir for a draft shared app", async () => {
  mockApps([{ ...webApp, can_run: true, shared: { status: "draft", url: "http://notas.localhost:8080", slug: "notas" } }])
  renderCatalog()
  await screen.findByText("Notas")
  expect(screen.queryByRole("link", { name: "Abrir Notas" })).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Rodar Notas" })).toBeInTheDocument()
})
```


- [ ] **Step 2: Rodar** — `cd frontend && mise x -- npx vitest run src/lib/shared.test.ts src/components/SharedStatus.test.tsx src/pages/CatalogPage.test.tsx` → FAIL.

- [ ] **Step 3: Implementar `lib/shared.ts`**

```ts
import type { ReadinessItem, SharedState, SharedStatus } from "./api"
import { formatSize } from "./runs"

export const SHARED_STATUS: Record<SharedStatus, { label: string; tone: "ok" | "live" | "muted" | "bad" | "draft" }> = {
  draft: { label: "Ainda não publicado", tone: "draft" },
  building: { label: "Publicando", tone: "live" },
  running: { label: "No ar", tone: "ok" },
  sleeping: { label: "Descansando", tone: "muted" },
  stopped: { label: "Parado", tone: "muted" },
  crashed: { label: "Caiu", tone: "bad" },
}

export function sharedLine(s: SharedState, liveVersion?: number): string {
  switch (s.status) {
    case "running":
      return liveVersion ? `No ar · versão ${liveVersion}` : "No ar"
    case "sleeping":
      return "Descansando · acorda em segundos quando alguém abre"
    case "stopped":
      return s.stopped_by ? `Parado por ${s.stopped_by}` : "Parado"
    case "crashed":
      return "Caiu e não voltou sozinho"
    case "building":
      return "Publicando a versão nova"
    default:
      return "Ainda não publicado"
  }
}

const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" })
export const formatCents = (cents: number) => brl.format(cents / 100).replace(/ /g, " ")
export const formatBytes = formatSize

export function claudeTextFor(items: ReadinessItem[]): string {
  const todo = items.filter((i) => (i.status === "missing" || i.status === "warn") && i.claude_text)
  return [
    "Estou preparando este app para rodar compartilhado no App Hub. Faça estas mudanças sem quebrar o jeito de rodar no computador:",
    ...todo.map((i, n) => `${n + 1}. ${i.claude_text}`),
    "Depois, atualize o app-hub.yaml se precisar e me diga o que mudou.",
  ].join("\n")
}
```

(Conferir o nome exato do campo `claude_text` no `readiness.Item` do Task 8 — o JSON do Go deve usar estas mesmas chaves: `key`, `status`, `title`, `detail`, `claude_text`, `acked`. Se o Task 8 usou outros nomes, acerte as tags JSON lá, não aqui.)

- [ ] **Step 4: `SharedStatus.tsx`**

```tsx
import { SHARED_STATUS } from "@/lib/shared"
import type { SharedStatus } from "@/lib/api"
import { cn } from "@/lib/utils"

const TONE = {
  ok: "bg-emerald-500",          // trocar pelos tokens de estado que o RunStrip já usa
  live: "bg-brand animate-pulse",
  muted: "bg-muted-foreground/50",
  bad: "bg-destructive",
  draft: "border border-muted-foreground/60 bg-transparent",
} as const

export function SharedStatusDot({ status, className }: { status: SharedStatus; className?: string }) {
  const s = SHARED_STATUS[status]
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-xs", className)}>
      <span aria-hidden className={cn("size-2 rounded-full", TONE[s.tone])} />
      {s.label}
    </span>
  )
}
```

Antes de escrever as classes de cor, abrir `RunStrip.tsx` e `lib/runs.ts` e usar as mesmas classes/tokens de "ok/live/muted/bad" que eles usam.

- [ ] **Step 5: `CatalogPage.tsx`** — no cartão, quando `a.shared && a.shared.status !== "draft"`: no lugar do botão Rodar, um link `<a href={openSharedUrl(a.shared.slug)} target="_blank" rel="noopener" aria-label={`Abrir ${a.name}`}>` com o mesmo visual do botão (use `buttonVariants({ variant: "run" })` com o violeta do tipo web e o ícone `ExternalLink`), texto "Abrir"; e o `<SharedStatusDot>` no lugar do "aberto agora". Apps compartilhados ainda em rascunho continuam com o Rodar de hoje (quem cuida pode rodar no próprio PC enquanto prepara).

- [ ] **Step 6: Rodar** → PASS; `npx vitest run` inteiro → PASS; `npx tsc -b` sem erro.

- [ ] **Step 7: Commit** — `git add frontend && git commit -m "Shared apps: catalog Abrir link and status"`

---

### Task 17: Ficha — a parte "Compartilhado"

**Files:**
- Create: `frontend/src/components/ficha/shared/SharedSection.tsx` (monta tudo; decide o que cada pessoa vê)
- Create: `frontend/src/components/ficha/shared/Readiness.tsx` (lista da checagem, caixa "texto para o Claude", "estou ciente")
- Create: `frontend/src/components/ficha/shared/Publish.tsx` (Testar, Publicar, andamento, Voltar)
- Create: `frontend/src/components/ficha/shared/Settings.tsx` (configurações e senhas)
- Create: `frontend/src/components/ficha/shared/Backups.tsx`
- Create: `frontend/src/components/ficha/shared/Tech.tsx` (limites, sempre ligado, uso 24 h, mensagens, Parar/Ligar, Deixar de compartilhar)
- Create: `frontend/src/components/ficha/shared/SharedLine.tsx` (a linha curta na parte de uso, para todos)
- Create: `frontend/src/components/ficha/shared/SharedSection.test.tsx`, `Readiness.test.tsx`, `Publish.test.tsx`, `Settings.test.tsx`
- Modify: `frontend/src/pages/AppPage.tsx`, `frontend/src/pages/AppPage.test.tsx`

**Interfaces:**
- Consumes: `api.shared.*`, `SharedStatusDot`, `sharedLine`, `claudeTextFor`, `formatCents/formatBytes`, `Section` de `ficha/Part.tsx`, `can` da ficha (`view`, `restore`, `tech`).
- Produces: `<SharedLine app={detail} info={info} />`, `<SharedSection app={detail} can={can} onChanged={() => void} />`.

O que cada pessoa vê:

| Quem | Vê |
|---|---|
| Todos que enxergam o app | `SharedLine`: estado + botão "Abrir" (quando não é rascunho) |
| `can.restore` (quem cuida) | Parte "Compartilhado" inteira, fora do `<details>` técnico: checagem, Testar/Publicar, Voltar, configurações, cópias, Parar/Ligar/Religar, Deixar de compartilhar |
| `can.tech` | Dentro do "Para quem cuida do app": limites, sempre ligado, uso 24 h, mensagens do app, registro da montagem |
| App `web` ainda não compartilhado + `can.restore` | Um cartão "Tornar compartilhado" com uma frase ("Fica num endereço só, para várias pessoas usarem ao mesmo tempo, com os dados guardados no Hub.") e o botão |

- [ ] **Step 1: Testes que falham**

`Readiness.test.tsx`:

```tsx
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { expect, it, vi } from "vitest"
import { ReadinessList } from "./Readiness"
import type { Readiness } from "@/lib/api"

const rep: Readiness = { ready: false, recipe: { kind: "streamlit", command: "" }, wants_db: false, suggest_env: [], items: [
  { key: "recipe", status: "auto", title: "Jeito de rodar", detail: "Streamlit, o Hub monta sozinho.", claude_text: "", acked: false },
  { key: "port", status: "missing", title: "Porta", detail: "O app não lê a porta que o Hub dá.", claude_text: "Faça o app ler PORT.", acked: false },
  { key: "file_data", status: "warn", title: "Dados em arquivo", detail: "Grava notas.json na pasta.", claude_text: "Use APPHUB_DATA_DIR.", acked: false },
]}

it("lists items in plain words with the Claude text to copy", async () => {
  const onAck = vi.fn()
  render(<ReadinessList report={rep} acks={[]} onAck={onAck} />)
  expect(screen.getByText("Porta")).toBeInTheDocument()
  expect(screen.getByText("Streamlit, o Hub monta sozinho.")).toBeInTheDocument()
  const box = screen.getByRole("textbox", { name: "Texto para o Claude" })
  expect((box as HTMLTextAreaElement).value).toContain("Faça o app ler PORT.")
  await userEvent.click(screen.getByRole("checkbox", { name: /estou ciente/i }))
  expect(onAck).toHaveBeenCalledWith(["file_data"])
})

it("says it is ready when nothing is missing", () => {
  render(<ReadinessList report={{ ready: true, recipe: { kind: "flask", command: "" }, wants_db: false, suggest_env: [], items: [] }} acks={[]} onAck={() => {}} />)
  expect(screen.getByText("Tudo certo para publicar.")).toBeInTheDocument()
  expect(screen.queryByRole("textbox", { name: "Texto para o Claude" })).not.toBeInTheDocument()
})
```

`Publish.test.tsx`:

```tsx
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api } from "@/lib/api"
import { PublishBar } from "./Publish"

beforeEach(() => vi.restoreAllMocks())

it("publishes and follows the progress until it is live", async () => {
  vi.spyOn(api.shared, "publish").mockResolvedValue({ id: 7, kind: "live", status: "building" } as never)
  const dep = vi.spyOn(api.shared, "deployment")
    .mockResolvedValueOnce({ id: 7, kind: "live", status: "starting" } as never)
    .mockResolvedValueOnce({ id: 7, kind: "live", status: "live" } as never)
  const onDone = vi.fn()
  render(<PublishBar appId={1} ready live={false} latest={3} liveVersion={undefined} hasPrevious={false} interval={10} onDone={onDone} />)
  await userEvent.click(screen.getByRole("button", { name: "Publicar" }))
  expect(await screen.findByText("Publicando…")).toBeInTheDocument()
  await waitFor(() => expect(onDone).toHaveBeenCalled())
  expect(dep).toHaveBeenCalledWith(1, 7)
})

it("offers a new version only when there is one", () => {
  render(<PublishBar appId={1} ready live latest={4} liveVersion={4} hasPrevious interval={10} onDone={() => {}} />)
  expect(screen.queryByRole("button", { name: "Publicar versão nova" })).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Voltar para a anterior" })).toBeInTheDocument()
})

it("shows the failure in plain words and keeps the old version", async () => {
  vi.spyOn(api.shared, "publish").mockResolvedValue({ id: 8, kind: "live", status: "building" } as never)
  vi.spyOn(api.shared, "deployment").mockResolvedValue({ id: 8, kind: "live", status: "failed", error_message: "A versão nova não respondeu. A anterior continua no ar." } as never)
  render(<PublishBar appId={1} ready live latest={5} liveVersion={4} hasPrevious interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Publicar versão nova" }))
  expect(await screen.findByText("A versão nova não respondeu. A anterior continua no ar.")).toBeInTheDocument()
})

it("asks about the data before going back", async () => {
  const rb = vi.spyOn(api.shared, "rollback").mockResolvedValue({})
  render(<PublishBar appId={1} ready live latest={5} liveVersion={5} hasPrevious interval={10} onDone={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Voltar para a anterior" }))
  await userEvent.click(screen.getByRole("radio", { name: /voltar os dados também/i }))
  await userEvent.click(screen.getByRole("button", { name: "Voltar" }))
  expect(rb).toHaveBeenCalledWith(1, true)
})

it("disables publishing while the check has pending items", () => {
  render(<PublishBar appId={1} ready={false} live={false} latest={1} liveVersion={undefined} hasPrevious={false} interval={10} onDone={() => {}} />)
  expect(screen.getByRole("button", { name: "Publicar" })).toBeDisabled()
  expect(screen.getByRole("button", { name: "Testar" })).toBeDisabled()
})
```

`Settings.test.tsx`:

```tsx
it("never shows a saved secret and can replace it", async () => {
  vi.spyOn(api.shared, "config").mockResolvedValue([
    { name: "SMTP_PASSWORD", label: "Senha do e-mail", secret: true, required: true, default: "", has_value: true, value: "" },
    { name: "ADMIN_EMAILS", label: "E-mails dos administradores", secret: false, required: true, default: "", has_value: true, value: "henry@alis.com" },
  ])
  const set = vi.spyOn(api.shared, "setConfig").mockResolvedValue({ needs_restart: true })
  render(<SharedSettingsForm appId={1} />)
  expect(await screen.findByText("Guardada")).toBeInTheDocument()
  expect(screen.getByLabelText("E-mails dos administradores")).toHaveValue("henry@alis.com")
  await userEvent.click(screen.getByRole("button", { name: "Trocar Senha do e-mail" }))
  await userEvent.type(screen.getByLabelText("Senha do e-mail"), "nova")
  await userEvent.click(screen.getByRole("button", { name: "Salvar Senha do e-mail" }))
  expect(set).toHaveBeenCalledWith(1, "SMTP_PASSWORD", "nova")
  expect(await screen.findByText("Salvo. Religue o app para valer.")).toBeInTheDocument()
})
```

(com os mesmos imports dos outros arquivos de teste.)

`SharedSection.test.tsx`:

```tsx
it("shows the whole part to who takes care, only the line to others", async () => {
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: running, live_version: 2, latest_version: 2, deployments: [] })
  vi.spyOn(api.shared, "check").mockResolvedValue({ ready: true, recipe: { kind: "streamlit", command: "" }, wants_db: false, suggest_env: [], items: [] })
  vi.spyOn(api.shared, "config").mockResolvedValue([])
  vi.spyOn(api.shared, "backups").mockResolvedValue([])
  const { rerender } = render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  expect(await screen.findByRole("button", { name: "Parar" })).toBeInTheDocument()
  rerender(<SharedSection app={detail} can={{ ...can, restore: false }} onChanged={() => {}} />)
  expect(screen.queryByRole("button", { name: "Parar" })).not.toBeInTheDocument()
})

it("asks for confirmation before unsharing", async () => {
  // … mesmos mocks …
  const un = vi.spyOn(api.shared, "unshare").mockResolvedValue({})
  render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  await userEvent.click(await screen.findByRole("button", { name: "Deixar de compartilhar" }))
  expect(screen.getByText(/os dados ficam guardados por 30 dias/i)).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Sim, deixar de compartilhar" }))
  expect(un).toHaveBeenCalledWith(detail.id)
})
```

(`running`, `detail` e `can` são objetos de exemplo no topo do arquivo, montados com os tipos do Task 16.)

`AppPage.test.tsx`: um app `web` sem `shared`, com `can.restore`, mostra o botão "Tornar compartilhado"; sem `can.restore`, não mostra.

- [ ] **Step 2: Rodar** → FAIL.

- [ ] **Step 3: `Readiness.tsx`** — `ReadinessList({ report, acks, onAck })`:
  - Cada item numa linha: ícone por estado (`ok` ✓ verde, `auto` ✓ com "o Hub resolve", `missing` ✕ vermelho, `warn` ! âmbar), título em negrito e o `detail` embaixo.
  - Itens `warn`: `<Checkbox>` (marcado quando `acked`) "Estou ciente" (rótulo acessível "Estou ciente: <título>"); marcar/desmarcar chama `onAck(novaLista)`.
  - Se há algum `missing`/`warn` com `claude_text`: caixa `<Textarea readOnly aria-label="Texto para o Claude">` com `claudeTextFor(items)` e botão "Copiar" (`navigator.clipboard.writeText`, toast "Copiado. Cole no Claude, na pasta do app."). Embaixo: "Depois que o Claude mudar o app, clique em Conferir de novo." e o botão "Conferir de novo" (prop `onRecheck`).
  - Pronto e sem nada pendente: "Tudo certo para publicar."

- [ ] **Step 4: `Publish.tsx`** — `PublishBar({ appId, ready, live, latest, liveVersion, hasPrevious, testUrl, testOpen, slug, interval = 2000, onDone })`:
  - Botões: "Testar" (variant outline) e "Publicar" (primeira vez) ou "Publicar versão nova" (quando `live && latest > liveVersion`). Ambos desabilitados se `!ready` ou durante uma publicação; quando desabilitado por pendência, um texto curto ao lado: "Resolva a checagem primeiro."
  - Clicar: chama `api.shared.publish`/`test`; guarda o `id`; consulta `api.shared.deployment(appId, id)` a cada `interval` ms até `live`, `failed` ou `ended`. Enquanto isso: "Publicando…" / "Montando a caixa de teste…" com o pontinho animado.
  - `live` → toast "No ar." e `onDone()`; teste pronto → link "Abrir o teste" (`openSharedUrl(slug, true)`, nova aba) e a frase "O teste fica aberto por 30 minutos, com dados separados."
  - `failed` → a `error_message` num aviso vermelho e um link "Ver detalhes" que abre um `<Dialog>` com o `build_log` (só se veio; quem não é `tech` não recebe o log, então o link não aparece).
  - Erro da API (409 em andamento, 503 Podman) → toast com a frase do servidor.
  - "Voltar para a anterior" (só com `hasPrevious`): `<Dialog>` com dois rádios — "Só a versão (os dados de agora ficam)" (padrão) e "Voltar os dados também (para a cópia feita antes da versão nova)"; botão "Voltar" chama `api.shared.rollback(appId, comDados)` e depois `onDone()`.
  - Limpar o `setTimeout` ao desmontar.

- [ ] **Step 5: `Settings.tsx`** — `SharedSettingsForm({ appId })`: carrega `api.shared.config`. Para cada item: `<Label>` com o rótulo (e "obrigatório" se `required`); não secreto → `<Input>` com o valor e botão "Salvar <rótulo>"; secreto com valor → texto "Guardada" + botão "Trocar <rótulo>" que troca por um `<Input type="password">` + "Salvar <rótulo>"; secreto sem valor → `<Input type="password">` direto. Salvar → `api.shared.setConfig`; `needs_restart` → "Salvo. Religue o app para valer." senão "Salvo.". Lista vazia → "Este app não pede configurações." Para declarar configurações novas (nome, rótulo, secreta, obrigatória) há o botão "Adicionar configuração", que acrescenta ao `env` do `SharedSettings` e salva pelo `api.shared.save` (a mesma gravação do manifesto).

- [ ] **Step 6: `Backups.tsx`** — lista `api.shared.backups` (data por `formatWhen`, tipo em palavras: diária / feita por <nome> / antes da versão <n> / antes de restaurar; tamanho por `formatBytes`), botão "Fazer cópia agora", e em cada linha "Baixar" (`<a href={backupDownloadUrl(...)} download>`) e "Restaurar" com confirmação ("Os dados de agora são trocados pelos desta cópia. Antes disso o Hub guarda uma cópia de agora, para dar para desfazer."). App sem banco → a parte não aparece.

- [ ] **Step 7: `Tech.tsx`** — dentro do `<details>` técnico:
  - Limites: memória (select 256/512/1024/2048 MB), processador (0,25/0,5/1/2), "Sempre ligado" (checkbox, com a frase "Gasta mais: fica ligado mesmo sem ninguém usando."), "Descansar depois de" (15/30/60/120 min, escondido se sempre ligado). Salvar pelo `api.shared.save`.
  - Uso nas últimas 24 h: uma linha SVG simples de memória (`api.shared.usage`), sem biblioteca nova; legenda "Memória usada (MB)" e o máximo.
  - Mensagens do app: `<pre>` com `api.shared.logs` e botão "Atualizar".
  - Histórico de publicações: as `deployments` (data, quem, teste/no ar, estado), cada uma com "Ver registro" (log).

- [ ] **Step 8: `SharedSection.tsx` e `SharedLine.tsx`**
  - `SharedSection` carrega `api.shared.get` e, para quem cuida, `api.shared.check`. Monta: `<Section title="Compartilhado">` com `SharedLine` + `ReadinessList` (só se não está pronto ou ainda não publicou) + `PublishBar` + Parar/Ligar/Religar (Parar pede confirmação: "Ninguém consegue abrir até alguém ligar de novo.") + `SharedSettingsForm` + `Backups` + "Deixar de compartilhar" (confirmação: "O endereço para de funcionar. Os dados ficam guardados por 30 dias; depois são apagados."). Marcar "estou ciente" chama `api.shared.save` com as `acks` novas e reconfere.
  - `SharedLine`: `SharedStatusDot` + `sharedLine(...)` + "Abrir" (link `openSharedUrl`) quando não é rascunho. Se `crashed`, para quem cuida: "Religar".
  - "Tornar compartilhado": `api.shared.save(id, { enabled: true, ...padrões })` (padrões: memória 512, processador 0,5, descansar em 30 min, sem banco, `updated_at` da ficha) e depois carrega a checagem.

- [ ] **Step 9: `AppPage.tsx`** — onde hoje fica a parte de uso, colocar `SharedLine` para todos quando `app.shared`; logo depois de "Execuções", a `SharedSection` (quem cuida) ou o cartão "Tornar compartilhado" (app `web` sem `shared` e `can.restore`); dentro do `<details>` técnico, o `Tech` (só `can.tech` e `app.shared`). Para apps compartilhados fora do rascunho, o "Rodar" da ficha vira "Abrir", igual ao catálogo.

- [ ] **Step 10: Rodar** — os testes novos → PASS; `npx vitest run` inteiro → PASS; `npx tsc -b`.

- [ ] **Step 11: Commit** — `git add frontend && git commit -m "Shared apps: ficha part (check, publish, settings, copies, tech)"`

---

### Task 18: Página "Servidor e gastos"

**Files:**
- Create: `frontend/src/pages/ServerPage.tsx`, `frontend/src/pages/ServerPage.test.tsx`
- Modify: `frontend/src/App.tsx` (rota `servidor`), `frontend/src/components/Layout.tsx` (item no menu), `frontend/src/components/Layout.test.tsx`

**Interfaces:**
- Consumes: `api.server.*`, `canSeeServer`, `formatCents`, `formatBytes`, `SharedStatusDot`.

Conteúdo da página (de cima para baixo):
1. Título "Servidor e gastos" e uma frase: "Quanto o servidor usa e quanto cada app compartilhado custa, aproximadamente."
2. Três números grandes: Memória (`usado de total`, barra), Disco (`usado de total`, barra; âmbar e o aviso "O disco passou de 90%. Apague cópias antigas ou aumente o disco." quando `disk_warning`), Gasto do mês até agora (soma dos `cost_cents` + `hub_cents`).
3. "Quanto o servidor custa por mês": campo em reais (aceita "300" ou "300,50") + "Salvar". Com custo 0, a tabela mostra "—" na coluna de gasto e a frase "Informe o custo mensal para ver quanto cada app gasta."
4. Tabela dos apps (no celular, vira cartões): nome (link para a ficha), estado, horas ligado no mês, memória média/máxima vs reservada, banco + cópias, gasto. Por linha: "Parar"/"Ligar" e "Limites" (dialog com memória, processador, sempre ligado, descansar depois de; mesmos valores do Task 17; resposta `needs_restart` → "Salvo. Vale na próxima vez que ligar.").
5. Linha final "Hub e espaço livre" com `hub_cents`.
6. Rodapé em letra pequena: "A conta divide o custo do servidor pela memória reservada × tempo ligado de cada app. É uma estimativa."
7. Se `engine_ok` é falso: aviso no topo "O Podman não está respondendo. Os apps compartilhados não abrem até ele voltar."

- [ ] **Step 1: Testes que falham** (`ServerPage.test.tsx`)

```tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { beforeEach, expect, it, vi } from "vitest"
import { api, type ServerInfo } from "@/lib/api"
import { ServerPage } from "./ServerPage"

const info: ServerInfo = {
  memory_total: 8 * 2 ** 30, memory_used: 3 * 2 ** 30, disk_total: 100 * 2 ** 30, disk_used: 95 * 2 ** 30, disk_warning: true,
  engine_ok: true, monthly_cost_cents: 30000, hub_cents: 9000, period_start: "2026-10-01T00:00:00-03:00",
  apps: [{ app_id: 1, name: "Sistema Alis", slug: "sistema-alis", status: "running", always_on: true, memory_mb: 1024, cpus: 1,
    idle_minutes: 30, hours_on: 120, avg_mem_mb: 400, max_mem_mb: 700, avg_cpu: 4, db_bytes: 50 * 2 ** 20, backup_bytes: 200 * 2 ** 20, cost_cents: 1234 }],
}

beforeEach(() => { vi.restoreAllMocks(); vi.spyOn(api.server, "get").mockResolvedValue(info) })
const renderPage = () => render(<MemoryRouter><ServerPage /></MemoryRouter>)

it("shows usage, the disk warning and the cost per app", async () => {
  renderPage()
  expect(await screen.findByText("Sistema Alis")).toBeInTheDocument()
  expect(screen.getByText("R$ 12,34")).toBeInTheDocument()
  expect(screen.getByText("R$ 90,00")).toBeInTheDocument()
  expect(screen.getByText(/O disco passou de 90%/)).toBeInTheDocument()
})

it("saves the monthly cost in cents", async () => {
  const set = vi.spyOn(api.server, "setCost").mockResolvedValue({})
  renderPage()
  const input = await screen.findByLabelText("Quanto o servidor custa por mês")
  await userEvent.clear(input)
  await userEvent.type(input, "450,50")
  await userEvent.click(screen.getByRole("button", { name: "Salvar custo" }))
  expect(set).toHaveBeenCalledWith(45050)
})

it("stops an app", async () => {
  const stop = vi.spyOn(api.server, "stop").mockResolvedValue({})
  renderPage()
  await userEvent.click(await screen.findByRole("button", { name: "Parar Sistema Alis" }))
  await userEvent.click(screen.getByRole("button", { name: "Sim, parar" }))
  expect(stop).toHaveBeenCalledWith(1)
})

it("warns when Podman is down", async () => {
  vi.spyOn(api.server, "get").mockResolvedValue({ ...info, engine_ok: false })
  renderPage()
  expect(await screen.findByText(/O Podman não está respondendo/)).toBeInTheDocument()
})
```

`Layout.test.tsx`: pessoa com `manage_server` vê "Servidor" no menu; sem, não vê.

- [ ] **Step 2: Rodar** → FAIL.

- [ ] **Step 3: Implementar** — `ServerPage.tsx` com o conteúdo acima. Conversão do campo de reais: `Math.round(parseFloat(v.replace(/\./g, "").replace(",", ".")) * 100)`; valor inválido ou negativo → mensagem "Digite um valor em reais, por exemplo 300 ou 300,50." e não chama a API. Recarrega a cada 60 s enquanto a página está aberta.

- [ ] **Step 4: Menu e rota** — em `Layout.tsx`, item `{ to: "/servidor", label: "Servidor", icon: Server, show: canSeeServer }` depois de "Equipe"; em `App.tsx`, `<Route path="servidor" element={<ServerPage />} />` (a API já barra quem não tem `manage_server`; a página mostra "Você não tem acesso a esta página." no 403).

- [ ] **Step 5: Rodar** → PASS; `npx vitest run`; `npx tsc -b`.

- [ ] **Step 6: Commit** — `git add frontend && git commit -m "Shared apps: Servidor e gastos page"`

---

# Fase 8 — Fechamento

### Task 19: Fotos, documentos e o teste de verdade com o Henry

**Files:**
- Create: `e2e/compartilhados.mjs`
- Create: `docs/adr/010-apps-compartilhados.md`
- Modify: `docs/TASKS.md`, `docs/PRD.md`, `docs/INFRASTRUCTURE.md`, `DESIGN.md` (se surgiram peças novas)

- [ ] **Step 1: Fotos (somente leitura)** — `e2e/compartilhados.mjs` no mesmo formato do `e2e/etapa2.mjs` (claro, escuro, celular; `/api/dev/login`; nunca salva nada): catálogo, ficha de um app compartilhado (parte "Compartilhado" aberta e o "Para quem cuida do app" aberto), página `/servidor`. Avisa `OVERFLOW-X` se houver rolagem para o lado. Rodar contra o Hub do Henry **só depois** que ele reiniciar o Hub com o código novo (Step 4). Olhar cada foto: alinhamento, contraste, textos cortados; corrigir e repetir.

- [ ] **Step 2: ADR 010** — `docs/adr/010-apps-compartilhados.md` (formato dos ADRs existentes): contexto (apps usados por muitos ao mesmo tempo), decisão (Podman com motor trocável, um banco por app no `app-hub-db`, porteiro no próprio Hub por subdomínio, entrada pelo Hub com código de 60 s, cabeçalhos `X-AppHub-*` com nome e papel codificados em URL — o app precisa decodificar, publicar lado a lado com volta automática, descansar quando parado, cópia diária, custo estimado por memória × tempo), quem tem `manage_server` age sobre qualquer app compartilhado, alternativas recusadas (Kubernetes, um banco para todos, login próprio por app), consequências (precisa de Podman ligado; geração de PDF do Sistema Alis não funciona dentro da caixa — a checagem avisa).

- [ ] **Step 3: Documentos** — `docs/TASKS.md`: nova seção "Apps compartilhados" com os Tasks 1–19 marcados e o que ficou para depois (subir na nuvem por último; Sistema Alis V2 para a equipe). `docs/PRD.md`: a funcionalidade, quem usa, o que cada papel vê. `docs/INFRASTRUCTURE.md`: conferir a seção do Task 13. Commit: `git add docs e2e && git commit -m "Docs: shared apps ADR, tasks, PRD"` e `git push`.

- [ ] **Step 4: Teste de verdade com o Henry, um passo por pergunta** — seguir a regra da memória: uma ação por pergunta (AskUserQuestion), conferindo pelos dados do Hub. Antes, pedir ao Henry para reiniciar o Hub (o Agente dele está ligado a ele) com o comando de sempre e confirmar que o Podman responde (`podman info`). Roteiro:
  1. Copiar `backend/testdata/shared/streamlit` para uma pasta do Henry e cadastrar pelo Claude (registro normal, tipo web).
  2. Henry abre a ficha → "Tornar compartilhado" → conferir que a checagem aparece e está pronta.
  3. Henry clica "Testar" → abre o teste → escreve uma nota. Conferir pelo banco do teste que a nota existe.
  4. Henry clica "Publicar" → abre pelo catálogo ("Abrir") → escreve uma nota. Conferir que é outra base (a do teste não aparece).
  5. Abrir em uma janela anônima com outra pessoa da equipe (ou o Henry com outro perfil) → as duas veem a mesma nota.
  6. Esperar o descanso (diminuir para 15 min nos limites) → abrir de novo → página "acordando" e depois o app.
  7. Mudar uma linha do app → "Publicar versão nova" → "Voltar para a anterior" (só a versão).
  8. Página "Servidor e gastos" → informar R$ 300 → conferir a conta.
  9. Depois: Sistema Alis V2, só checagem (ver o que o Hub pede e o "texto para o Claude"), sem publicar para a equipe ainda.

- [ ] **Step 5: Fechar** — anotar o que o teste mostrou em `docs/TASKS.md`, corrigir o que apareceu (cada correção com teste), commit e push, e o resumo da sessão no formato do playbook.
