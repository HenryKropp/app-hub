# Governança (quem aprova, quem usa, por onde sobe) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Trocar o "aprovado sim/não" de hoje por pedidos de aprovação com etapas (líder sempre; TI se a empresa ligar), com regra de versões configurável, público escolhido pelo líder, restrição de canal ("só pelo Claude") e chaves de MCP, valendo para apps e artifacts.

**Architecture:** Dois pacotes puros novos — `internal/risk` (impressão de risco de uma versão e o que ela acrescenta) e `internal/governance` (decide se precisa aprovar, quais etapas, quem pode aprovar cada etapa). Um handler novo `governance.go` guarda e resolve pedidos (`approval_requests` + `approval_steps`) e aplica o resultado reaproveitando os caminhos que já existem: rascunho (ADR 043) para versão de app com código no Ziv, `setAppAccess` para o público, `UpsertApp`/`UpdateArtifactCare` para a área. A trava de execução (`executionBlock`) passa a ler `app_governance` em vez de `app_security.approved`.

**Tech Stack:** Go (net/http, pgx, sqlc, migrações embutidas), React + TS + Tailwind (shadcn), Playwright (`e2e/*.mjs`).

**Spec:** `docs/superpowers/specs/2026-11-17-governanca-aprovacoes-design.md`

## Global Constraints

- Uma empresa = um schema; a migração `030_governance.sql` roda em todos (principal e `tenant_*`) e tem de ser idempotente.
- **Empresa nova** começa: líder sempre, TI desligada, regra `risk` ("só quando muda o risco"), canal `any`, MCP sem restrição.
- **Empresas existentes** que tinham `require_app_approval = TRUE` (a Empresa Piloto): `it_approval = TRUE`, regra `every`, e o cargo `admin` ganha `approve_it`. As que tinham FALSE ficam com o padrão leve.
- O líder **sempre** aprova (não há chave para desligar). Área sem líder: quem tem `manage_roles` aprova no lugar; com líder, o admin **não** passa por cima.
- Quem pede e é líder da área: a etapa do líder já nasce aprovada por ele. Quem pede e tem `approve_it`: a etapa de TI já nasce aprovada por ele. Empresa de uma pessoa: o admin tem as duas → um clique ("Aprovar") fecha.
- Rigor das regras: `every` > `risk` > `first`. Um item pode ser mais rígido que a empresa (quem cuida ou o líder muda); mais frouxo que a empresa, só `manage_roles`.
- Versão esperando aprovação **não** derruba a aprovada: o código fica no rascunho até aprovar. Exceção: app de pasta no PC (o código roda da pasta), que trava como hoje.
- Mudança só de texto na ficha nunca pede aprovação; mudança de ficha que acrescenta risco (chave do Cofre, 24h) segue a regra.
- Regra de acesso só em `backend/internal/access`; a tela só lê `can`. Item não visível → 404, como hoje.
- Texto de tela e de MCP em português, tom do Ziv, sem jargão. Mensagens fixas abaixo vêm do desenho e devem ser usadas como estão.
- Testes Go do handler compartilham o banco de teste: **nunca** rodar dois `go test` do handler ao mesmo tempo. Testes que dependem da trava precisam de `h.SetWorkspace(...)`.
- Sem e-mail: aviso aos aprovadores só dentro do Ziv.

Mensagens fixas:
- `msgOnlyClaude` = "Esta empresa só aceita versões enviadas pelo Claude."
- `msgWaitingLeader` = "Aguardando o líder de %s." (nome da área)
- `msgWaitingIT` = "Aguardando a TI."
- `msgRejected` = "Recusado por %s: %s"
- `msgNoMCP` = "Seu cargo não tem permissão para conectar o Claude ao Ziv. Fale com quem administra a empresa."

## Review Focus

1. **Versão chegando com pedido aberto** (o Baixar Relatórios teve 6 versões num dia): o pedido antigo é substituído, nunca dois abertos no mesmo item; quem já aprovou a antiga não vê a nova como aprovada. Teste em Task 5.
2. **App ainda não aprovado e o agendamento dispara**: não roda, e a execução registra o motivo (`msgWaitingLeader`). Teste em Task 5.
3. **Líder removido da área com pedido aberto**: a etapa passa a poder ser aprovada pelo admin (a regra "área sem líder" é avaliada na hora de aprovar, não ao abrir). Teste em Task 4.
4. **Link externo "sempre a mais recente" com versão nova esperando**: o cliente de fora continua vendo a última aprovada. Teste em Task 7.
5. **Aprovar duas vezes ao mesmo tempo** (duplo clique, dois aprovadores): só uma aplicação do resultado; a segunda recebe o pedido já fechado (409). Teste em Task 4.

---

## File Structure

- Create `backend/internal/database/migrations/030_governance.sql` — tabelas e colunas novas, migração dos dados de hoje.
- Create `backend/internal/database/queries/governance.sql` — consultas sqlc.
- Create `backend/internal/risk/risk.go`, `risk_test.go` — impressão de risco.
- Create `backend/internal/governance/governance.go`, `governance_test.go` — decisões puras.
- Modify `backend/internal/access/access.go` (+ teste) — permissões `approve_it`, `use_mcp`; item pendente; `Care` separado de `Access`.
- Create `backend/internal/handler/governance.go`, `governance_test.go` — pedidos, aprovar/recusar, aplicar, configurações, histórico.
- Modify handlers listados em cada tarefa.
- Create `frontend/src/pages/ApprovalsPage.tsx` (+ teste), `frontend/src/components/governance/*`.
- Modify `HomePage.tsx`, `AdminPage.tsx`, `AppPage.tsx`, `ArtifactPage.tsx`, `Layout.tsx`, `App.tsx`, `lib/api.ts`, `components/ficha/AppControl.tsx`, `components/ClaudeConnections.tsx`.
- Create `e2e/governanca.mjs`; `docs/adr/046-governanca.md`.

---

### Task 1: Banco — migração 030, consultas e padrões de empresa nova

**Files:**
- Create: `backend/internal/database/migrations/030_governance.sql`
- Create: `backend/internal/database/queries/governance.sql`
- Modify: `backend/internal/database/queries/artifacts.sql:95` (`DisableAppApprovalByDefault` → renomear para `ApplyNewCompanyGovernance`)
- Modify: `backend/internal/workspace/portal.go:361` (chamar a consulta renomeada)
- Modify: `backend/internal/handler/handler_test.go` (`resetSQL`: incluir as tabelas novas)
- Test: `backend/internal/workspace/portal_test.go`

**Interfaces:**
- Produces (SQL):
  - `security_settings` + `it_approval BOOLEAN NOT NULL DEFAULT FALSE`, `version_rule TEXT NOT NULL DEFAULT 'risk' CHECK (version_rule IN ('every','risk','first'))`, `code_channel TEXT NOT NULL DEFAULT 'any' CHECK (code_channel IN ('any','mcp'))`, `mcp_restricted BOOLEAN NOT NULL DEFAULT FALSE`.
  - `app_governance(app_id PK → apps ON DELETE CASCADE, approved_version INT, approved_risk JSONB NOT NULL DEFAULT '{}', version_rule TEXT NULL CHECK (…mesmos 3…), ever_approved BOOLEAN NOT NULL DEFAULT FALSE)`.
  - `artifacts` + `version_rule TEXT NULL`, `ever_approved BOOLEAN NOT NULL DEFAULT TRUE`; `artifact_versions` + `approved BOOLEAN NOT NULL DEFAULT TRUE`, `channel TEXT NOT NULL DEFAULT 'manual' CHECK (channel IN ('manual','mcp'))`.
  - `approval_requests(id, item_kind TEXT CHECK IN ('app','artifact'), app_id NULL FK, artifact_id NULL FK, reason TEXT CHECK IN ('new','version','ficha','area'), draft_id NULL → app_drafts, app_version INT NULL, artifact_version_id NULL → artifact_versions, target_area_id NULL → areas, risk JSONB NOT NULL DEFAULT '{}', risk_added JSONB NOT NULL DEFAULT '[]', channel TEXT, status TEXT CHECK IN ('open','approved','rejected','replaced','auto'), requested_by NULL → people, created_at, closed_at, CHECK exatamente um de app_id/artifact_id)`; índice único parcial `(item_kind, COALESCE(app_id,artifact_id)) WHERE status='open'`.
  - `approval_steps(request_id FK CASCADE, step TEXT CHECK IN ('leader','it'), decided_by NULL → people, decided_at, decision TEXT NULL CHECK IN ('approved','rejected'), reason TEXT NOT NULL DEFAULT '', on_behalf TEXT NOT NULL DEFAULT '' /* '' | 'admin_no_leader' | 'self' */, PRIMARY KEY(request_id, step))`.
  - `governance_events(id, item_kind, app_id NULL, artifact_id NULL, kind TEXT /* audience, area, rule, settings, mcp */, actor_id, detail JSONB, created_at)`.
  - `app_drafts.status` CHECK ganha `'aguardando'`.
  - Dados: `app_governance` para todo app: `approved_version` = última versão quando `approvedSQL` era verdadeiro **ou** a empresa não exigia aprovação; senão NULL e um `approval_requests` aberto (`reason='new'` se nunca aprovado, `'version'` se já foi) com etapas conforme a empresa. `ever_approved` = `app_security.reviewed_at IS NOT NULL AND approved` ou empresa sem exigência. Configuração conforme Global Constraints; `UPDATE roles … array_append(permissions,'approve_it') WHERE key='admin'` só quando `require_app_approval`; `use_mcp` acrescentado a **todos** os cargos (nada muda para quem já usa).
- Produces (sqlc, nomes exatos): `GetGovernanceSettings`, `UpdateGovernanceSettings`, `GetAppGovernance`, `UpsertAppGovernance`, `SetAppVersionRule`, `SetArtifactVersionRule`, `InsertApprovalRequest`, `InsertApprovalStep`, `GetOpenRequestForApp`, `GetOpenRequestForArtifact`, `GetApprovalRequestForUpdate` (`FOR UPDATE`), `ListApprovalSteps`, `DecideApprovalStep`, `CloseApprovalRequest`, `ReplaceOpenRequests`, `ListOpenRequests`, `ListItemRequests` (histórico de um item), `ListRecentRequests` (empresa, 90 dias), `InsertGovernanceEvent`, `ListGovernanceEvents`, `LatestApprovedArtifactVersion`, `SetArtifactVersionApproved`, `ApplyNewCompanyGovernance`.
- `ApplyNewCompanyGovernance`: `require_app_approval=FALSE, it_approval=FALSE, version_rule='risk', code_channel='any', mcp_restricted=FALSE` e tira `approve_it` do admin.

- [ ] **Step 1: Teste que falha** — em `portal_test.go`, `TestNewCompanyGovernanceDefaults`: cria empresa com `testPortal`, lê `GetGovernanceSettings` → `ItApproval=false`, `VersionRule="risk"`, `CodeChannel="any"`; o cargo admin não tem `approve_it` e tem `use_mcp`. E `TestMigrationKeepsStrictCompany`: num schema com `require_app_approval=TRUE` e um app aprovado, depois de migrar: `it_approval=true`, `version_rule="every"`, admin com `approve_it`, `app_governance.approved_version` = última versão, nenhum pedido aberto; um app não aprovado ganha um pedido aberto com etapas `leader` e `it`.
- [ ] **Step 2:** `cd backend && mise x -- go test ./internal/workspace -run 'TestNewCompanyGovernanceDefaults|TestMigrationKeepsStrictCompany'` → FAIL (tabela não existe).
- [ ] **Step 3:** Escrever a migração, as consultas, `mise x -- sqlc generate`, ler os tipos gerados, trocar a chamada em `portal.go`, incluir tabelas em `resetSQL`. `TestNewCompanySchemaMatchesMain` (portal_test.go:390) tem de continuar passando.
- [ ] **Step 4:** Rodar os dois testes + `TestNewCompanySchemaMatchesMain` → PASS.
- [ ] **Step 5:** Commit `governança: banco (pedidos, etapas, regra de versões, canal)`.

---

### Task 2: Pacote `risk`

**Files:**
- Create: `backend/internal/risk/risk.go`, `backend/internal/risk/risk_test.go`

**Interfaces:**
- Produces:
  - `type Set struct { Keys []string \`json:"keys"\`; Domains []string \`json:"domains"\`; Browser bool \`json:"browser"\`; AlwaysOn bool \`json:"always_on"\`; SendsOut bool \`json:"sends_out"\` }` (listas ordenadas, sem repetição).
  - `func Of(m manifest.Manifest, files map[string][]byte) Set` — `files` = caminho → conteúdo (nil para mudança só de ficha: aí Domains/Browser/SendsOut ficam vazios e `Merge` mantém os anteriores).
  - `func (s Set) Added(prev Set) []string` — itens novos, legíveis em português: `"chave do Cofre: BTG_SENHA"`, `"site: research.btgpactual.com"`, `"usa navegador"`, `"roda 24h no servidor"`, `"manda coisas para fora"`.
  - `func Merge(codeless Set, prev Set) Set` — para ficha sem código: Keys/AlwaysOn da ficha nova, o resto do anterior.
- Regras: Keys = `m.Keys[].Name` ∪ `m.Shared.Env` com `Secret`; AlwaysOn = `m.Shared != nil && m.Shared.Enabled`; Browser = `recipes.Detect(...).Browser` (ver `shared/recipes/recipes.go:96` para a assinatura); Domains = hosts de `https?://([a-z0-9.-]+)` nos arquivos de texto (`.py .js .ts .mjs .html .yaml .yml .json .toml .txt`), minúsculos, sem `localhost`, `127.0.0.1`, `0.0.0.0`, `example.com`, `schemas.*`, `www.w3.org`; até 200 KB por arquivo; SendsOut = import/require de `smtplib`, `yagmail`, `win32com.client` com `Outlook`, `sendgrid`, `resend`, `nodemailer`.

- [ ] **Step 1: Testes que falham:** `TestOfReadsKeysDomainsBrowser` (manifesto com chave `BTG_SENHA`, arquivo `main.py` com `requests.get("https://research.btgpactual.com/x")` e `from playwright.sync_api import`, → `Keys=["BTG_SENHA"]`, `Domains=["research.btgpactual.com"]`, `Browser=true`); `TestOfIgnoresLocalhost`; `TestAddedListsOnlyNew` (prev com 1 domínio, novo com 2 → 1 item `"site: xp.com"`; remover não aparece); `TestMergeKeepsCodeRisk`; `TestSendsOutSmtplib`.
- [ ] **Step 2:** `mise x -- go test ./internal/risk` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** → PASS.
- [ ] **Step 5:** Commit `governança: impressão de risco`.

---

### Task 3: Pacote `governance` + regras de acesso

**Files:**
- Create: `backend/internal/governance/governance.go`, `governance_test.go`
- Modify: `backend/internal/access/access.go`, `backend/internal/access/access_test.go`
- Modify: `backend/internal/handler/access.go:72` (`loadAccess` carrega pendentes e áreas sem líder)
- Modify: `backend/internal/handler/team.go:56` (aceitar as permissões novas)

**Interfaces:**
- Produces (`governance`):
  - `type Rule string` com `Every="every"`, `Risk="risk"`, `First="first"`; `func Stricter(a, b Rule) Rule`; `func Effective(company Rule, item *Rule) Rule` (item nulo → empresa; item mais frouxo nunca vence a empresa: devolve `Stricter`).
  - `func CanLoosen(perms map[string]bool) bool` → `perms["manage_roles"]`.
  - `type Need struct { Required bool; Steps []string /* "leader", "it" */ }`
  - `func Decide(rule Rule, everApproved bool, added []string, itOn bool, reason string) Need` — `reason` "new"/"area" → sempre requerido; "version"/"ficha": `Every` → requerido; `Risk` → requerido se `len(added)>0`; `First` → requerido se `!everApproved`. `reason=="area"` → só `"leader"` (do destino). Senão `"leader"` + `"it"` se `itOn`.
  - `type Approver struct { PersonID int64; Perms map[string]bool; Leads map[int64]bool }`
  - `func MayDecide(step string, areaID int64, areaHasLeader bool, a Approver) (ok bool, onBehalf string)` — leader: líder da área → ("",true); área sem líder e `manage_roles` → ("admin_no_leader", true). it: `approve_it` → true.
  - `func AutoSteps(steps []string, areaID int64, requester Approver) map[string]string` — etapas que já nascem aprovadas por quem pede (`"self"`).
- Produces (`access`): constantes `ApproveIT = "approve_it"`, `UseMCP = "use_mcp"` em `RolePerms`; `App.Pending bool` (nunca aprovado); `Person.ApproverIT` derivado de `Has(ApproveIT)`; `Can.Care bool \`json:"care"\`` (= o `Access` de antes: quem cuida + líder + `manage_areas`); `Can.Access` passa a ser só `leader || ManageAreas` (público); `Can.Approve bool \`json:"approve"\`` só preenchido pelo handler.
  - Item `Pending`: `View` só para dono, reserva, líder da área, `SeeAllApps`, `ManageRoles` e (se a empresa liga TI) `ApproveIT`; acessos extras e `ManageAreas` **não** revelam. O carregamento de "TI ligada" vai em `Person.ITOn bool`.

- [ ] **Step 1: Testes que falham** (`governance_test.go`): tabela para `Decide` cobrindo as 3 regras × versão sem risco / com risco / primeira, `new`, `area`, TI on/off; `TestEffectiveNeverLooser` (empresa `every`, item `first` → `every`); `TestMayDecideAdminOnlyWithoutLeader` (com líder → false para admin); `TestAutoStepsSelf` (pedinte líder + `approve_it` → as duas). (`access_test.go`): `TestPendingHiddenFromGrantsAndManageAreas`; `TestPendingVisibleToOwnerLeaderAdmin`; `TestAccessOnlyLeaderOrManageAreas` (dono tem `Care` mas não `Access`).
- [ ] **Step 2:** `mise x -- go test ./internal/governance ./internal/access` → FAIL.
- [ ] **Step 3:** Implementar; em `loadAccess`, ler `security_settings.it_approval` para `ITOn` e marcar `Pending` nos `App` (via `app_governance.ever_approved=false` / `artifacts.ever_approved=false`) — onde o handler monta `access.App` (procurar todos os `access.App{` e preencher `Pending`).
- [ ] **Step 4:** Rodar os pacotes + `go test ./internal/handler -run 'Permission|Access'` → PASS. Trocar no handler os usos de `can.Access` que significam "trocar dono/reserva/área" (`handsOver`, apps.go:509; `updateArtifact`, artifacts.go:468) para `can.Care`.
- [ ] **Step 5:** Commit `governança: decisões e regras de acesso`.

---

### Task 4: Pedidos — abrir, aprovar, recusar, aplicar

**Files:**
- Create: `backend/internal/handler/governance.go`, `backend/internal/handler/governance_test.go`
- Modify: `backend/internal/handler/handler.go` (rotas)

**Interfaces:**
- Consumes: Task 1 (sqlc), Task 2 (`risk`), Task 3 (`governance`, `access`).
- Produces:
  - `func (h *Handler) openRequest(ctx context.Context, tx pgx.Tx, in requestIn) (requestOut, error)` com `type requestIn struct { Kind string; AppID, ArtifactID int64; AreaID int64; Reason string; DraftID, ArtifactVersionID, TargetAreaID int64; AppVersion int32; Risk risk.Set; Added []string; Channel string; Requester access.Person }`. Substitui (status `replaced`) o pedido aberto do item; se `Decide` diz não requerido, grava status `auto` (registro) e devolve `Auto=true`; senão grava as etapas e aplica `AutoSteps`; se todas ficam aprovadas, aplica na hora (`applyRequest`) e fecha `approved`.
  - `type requestOut struct { ID int64; Auto, Applied bool; Waiting string /* msgWaitingLeader/msgWaitingIT ou "" */ }`
  - `func (h *Handler) applyRequest(ctx context.Context, tx pgx.Tx, req sqlc.ApprovalRequest, audience *audienceIn) error` — `version`/`ficha` com `draft_id` → publicar o rascunho (`publishDraft` de drafts.go:153, no mesmo tx ou logo após o commit; seguir o que `publishDraft` exige); `version` sem rascunho (pasta do PC) → `approved_version = app_version`; `new` → `approved_version`, `ever_approved=true` e público; `area` → mudar área (apps: `UpsertApp` com a área nova; artifacts: `UpdateArtifactCare`); artifact `new`/`version` → `SetArtifactVersionApproved`, `ever_approved=true`. Atualiza `approved_risk`.
  - Rotas:
    - `GET /api/approvals` → abertos que a pessoa pode decidir (`MayDecide` em alguma etapa ainda sem decisão), mais "foram ao ar com aviso" (status `auto` de 30 dias, para líderes e TI). JSON: `[{id, kind, item_id, item_name, area, reason, risk_added[], channel, requested_by, created_at, steps:[{step, decision, decided_by, on_behalf}], tested: bool}]`.
    - `GET /api/approvals/count` → `{count}` (para o selo no menu).
    - `POST /api/approvals/{id}/approve` body `{audience?: {mode: "area"|"company"|"custom", area_ids[], person_ids[]}}` — `audience` obrigatório quando `reason=="new"` e quem decide é a etapa `leader`; 400 "Escolha quem vai usar." se faltar.
    - `POST /api/approvals/{id}/reject` body `{reason}` — motivo obrigatório (400 "Diga o motivo."); fecha `rejected`; rascunho vai a `reprovada` com `status_reason = msgRejected`.
    - `GET /api/{apps|artifacts}/{id}/governance` → `{rule_company, rule_item, rule_effective, it_on, open_request?, history:[…], can_loosen}`.
    - `PUT /api/{apps|artifacts}/{id}/governance/rule` body `{rule|null}` — `can.Care` ou `can.Access` para apertar; afrouxar abaixo da empresa → 403 a menos que `manage_roles`; grava `governance_events` kind `rule`.
  - Concorrência: aprovar/recusar abre tx, `GetApprovalRequestForUpdate`; se não está `open` → 409 "Este pedido já foi resolvido.".
  - Área sem líder é avaliada em `MayDecide` na hora de decidir.

- [ ] **Step 1: Testes que falham** (com `SetWorkspace`, usando `setRole`, `makeLeader` de permissions_test.go):
  - `TestNewAppOpensLeaderRequest` (empresa leve; criador sem liderança → pedido aberto `leader`; app invisível para pessoa comum; líder vê em `/api/approvals`).
  - `TestLeaderApprovesWithAudience` (`audience.mode="area"` → app `restricted` com a área; pessoa da área roda; pessoa de fora 404).
  - `TestApproveNeedsAudienceForNew` (400).
  - `TestITStepWhenCompanyTurnsItOn` (duas etapas; só líder aprovou → ainda bloqueado; TI aprova → liberado).
  - `TestRequesterLeaderAutoStep` e `TestOnePersonCompanyOneClick`.
  - `TestAdminApprovesOnlyWithoutLeader` (com líder → 403; tira o líder com o pedido aberto → admin aprova com `on_behalf="admin_no_leader"`).
  - `TestRejectNeedsReasonAndKeepsOld`.
  - `TestDoubleApproveConflict` (duas aprovações em sequência no mesmo pedido de uma etapa → segunda 409; aplicação feita uma vez).
  - `TestItemRuleCannotLoosenBelowCompany` (403 sem `manage_roles`; 200 com).
- [ ] **Step 2:** `mise x -- go test ./internal/handler -run 'Approv|Leader|Request|ItemRule|OnePerson|DoubleApprove'` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** → PASS.
- [ ] **Step 5:** Commit `governança: pedidos de aprovação`.

---

### Task 5: Apps — ligar todos os caminhos de versão e a trava

**Files:**
- Modify: `backend/internal/handler/security.go:297` (`executionBlock`), `admin.go:17` (`approvedSQL`), `admin.go:147,165` (`appControlOf`, `setAppControl`: aprovar sai daqui; suspender fica)
- Modify: `script_import.go:23` (app novo → `openRequest reason=new`)
- Modify: `stored_code.go:197` (`sendSources`: se `Decide` pede aprovação, criar rascunho `aguardando` em vez de aplicar; senão aplicar como hoje e gravar pedido `auto`)
- Modify: `drafts.go:133,153` (publicar rascunho manual passa por `openRequest`), `test_pipeline.go:256` (`publishTested`: se precisa, rascunho vai a `aguardando` com pedido `tested=true`; notice diz "testada, esperando aprovação")
- Modify: `versions.go:64,71` (`checkVersion`: versão nova de pasta → `openRequest reason=version` sem rascunho; trava até aprovar)
- Modify: `apps.go:465` (`updateApp`: ficha que acrescenta risco → rascunho só de ficha + pedido `ficha`; troca de área → pedido `area` para o líder do destino, sem aplicar; pedinte líder do destino → aplica na hora)
- Modify: `restore.go:110` (voltar para versão já aprovada não pede; para versão nunca aprovada abre pedido)
- Modify: `home.go:228` (aviso `approval_pending` usa `app_governance`)
- Test: `governance_test.go` (mesmo arquivo da Task 4), `launch_test.go`, `scheduler_test.go`, `drafts_test.go`

**Interfaces:**
- Consumes: `openRequest`, `applyRequest` (Task 4); `risk.Of`, `risk.Merge`.
- Produces: `func (h *Handler) appApproved(ctx context.Context, appID int64) (approved bool, waiting string, err error)` — aprovado = `approved_version IS NOT NULL AND approved_version >= última app_versions.number`; `waiting` = texto do pedido aberto. `executionBlock` usa isso no lugar de `RequireApproval && !approved` e deixa passar o dono e quem pode decidir o pedido aberto (para testar antes), recebendo a pessoa: `executionBlock(ctx, appID, personID int64)` — atualizar os chamadores (launch.go:80,135; runs.go:72; credential_use.go:13; agents.go:352; scheduler.go:116; shared.go:331; test_pipeline.go:206). No agendamento, a pessoa é a do agendamento.
- `app_security.approved` deixa de ser lido; `PUT /api/apps/{id}/control` aceita só `suspended`.

- [ ] **Step 1: Testes que falham:**
  - `TestVersionUnderEveryWaitsAndOldKeepsRunning` (regra `every`: `POST /sources` → 200 com `waiting`; o código guardado e `app_versions` não mudam; rodar continua liberado e usa a versão anterior; aprovar → versão nova no ar).
  - `TestVersionUnderRiskWithoutNewRiskGoesLive` (vai ao ar; pedido `auto` gravado e listado em `/api/approvals` como aviso).
  - `TestVersionAddingDomainNeedsApproval` (regra `risk`, código novo com `https://xp.com` → espera; `risk_added` contém "site: xp.com").
  - `TestNewVersionReplacesOpenRequest` (Review Focus 1).
  - `TestScheduledRunBlockedWhileWaiting` (Review Focus 2: app novo não aprovado, agendamento dispara → execução `interrupted` com `msgWaitingLeader`).
  - `TestFolderVersionBlocksUntilApproved` (`checkVersion` com regra `every`).
  - `TestFichaTextEditNoApproval` e `TestFichaNewKeyNeedsApproval`.
  - `TestAreaMoveWaitsForTargetLeader` (fica na área antiga até aprovar).
  - `TestTestedDraftWaitsWhenRuleSaysSo` (pipeline do ADR 044).
- [ ] **Step 2:** `mise x -- go test ./internal/handler -run 'Version|Scheduled|Folder|Ficha|AreaMove|TestedDraft'` → FAIL.
- [ ] **Step 3:** Implementar, nesta ordem: `appApproved` + `executionBlock`; `importScript`; `sendSources`; drafts/pipeline; `checkVersion`; `updateApp`; `restore`; `home`.
- [ ] **Step 4:** `mise x -- go test ./internal/handler` inteiro (um processo só) → PASS. Testes antigos que contavam com `require_app_approval`/`setAppControl{approved}` são atualizados para o fluxo novo, não apagados.
- [ ] **Step 5:** Commit `governança: versões de apps passam pelos pedidos`.

---

### Task 6: Canal "só pelo Claude" e chaves do MCP

**Files:**
- Modify: `script_import.go`, `stored_code.go`, `drafts.go`, `apps.go` (`scan`/`saveApp` → `writeApp` para app novo), `versions.go` (`checkVersion`), `artifacts.go` (`createArtifact`, `addArtifactVersion`)
- Modify: `mcp_auth.go` (`createMCPCode`, `mcpConnection`), `mcp_oauth.go:351`
- Create rotas em `governance.go`: `GET /api/admin/mcp-connections`, `DELETE /api/admin/mcp-connections/{id}` (`manage_roles`)
- Test: `governance_test.go`

**Interfaces:**
- Produces: `func (h *Handler) channelAllowed(ctx context.Context) (bool, error)` — `code_channel=='any' || viaMCP(ctx)`. Recusa com 403 `msgOnlyClaude` em: importar script, mandar código, criar rascunho, registrar app por pasta (novo), criar artifact e versão de artifact. Versão de pasta (`checkVersion`) com canal `mcp`: grava pedido e `executionBlock` devolve `msgOnlyClaude`; aprovar esse pedido → 409 `msgOnlyClaude`. O teste no PC do ADR 044 continua (é comandado pelo MCP).
- `artifact_versions.channel` = `originOf(ctx)` em `addVersion` (artifacts.go:217).
- MCP: com `mcp_restricted`, `createMCPCode` e a troca OAuth recusam quem não tem `use_mcp` (403 `msgNoMCP`), e `mcpConnection` recusa conexão de quem perdeu `use_mcp` (401). Lista da empresa: pessoa, nome da conexão (o computador), criada, último uso, cancelada; cancelar grava `governance_events` kind `mcp`.

- [ ] **Step 1: Testes que falham:** `TestOnlyClaudeRefusesSiteUpload` (403 `msgOnlyClaude`), `TestOnlyClaudeAcceptsMCP` (mesma rota com `withMCPPerson` → ok), `TestOnlyClaudeKeepsAppRunning` (app aprovado continua rodando depois de ligar), `TestOnlyClaudeArtifact`, `TestMCPRestrictedRefusesWithoutPerm`, `TestMCPConnectionStopsWhenPermRemoved`, `TestAdminListsAndRevokesMCP`.
- [ ] **Step 2:** → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** → PASS (handler inteiro, um processo).
- [ ] **Step 5:** Commit `governança: só pelo Claude e chaves do MCP`.

---

### Task 7: Artifacts e links externos

**Files:**
- Modify: `artifacts.go` (`createArtifact` → pedido `new` com público; `addArtifactVersion` → versão com `approved=false` quando precisa; leitura de "versão atual" para quem não pode decidir = última aprovada), `artifact_embed.go` (servir a aprovada), `external_links.go:77,101,214`
- Modify: `governance.go` (risco de artifact: `risk_added = ["tem link externo ativo"]` quando há link ativo)
- Test: `governance_test.go`, `external_links_test.go`

**Interfaces:**
- Consumes: `openRequest`/`applyRequest`; `LatestApprovedArtifactVersion`.
- Produces: `linkState` usa `LatestApprovedArtifactVersion` quando `VersionID` é nulo; link com versão fixa não aprovada → página "não disponível" (mesma de hoje). `createArtifactLink` → 409 "Este artifact ainda espera aprovação." se `ever_approved=false`.

- [ ] **Step 1: Testes que falham:** `TestNewArtifactWaitsForLeader`, `TestArtifactVersionRiskOnlyWithExternalLink` (regra `risk`: sem link → vai ao ar; com link ativo → espera), `TestExternalLatestShowsLastApproved` (Review Focus 4), `TestNoExternalLinkBeforeApproval`.
- [ ] **Step 2:** → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** → PASS (handler inteiro).
- [ ] **Step 5:** Commit `governança: artifacts e links externos`.

---

### Task 8: Configurações da empresa, histórico e MCP falando a situação

**Files:**
- Modify: `governance.go` (rotas `GET/PUT /api/governance/settings` com `manage_roles`; `GET /api/governance/history` com `manage_roles`), `app_access.go:90` e `artifact_access.go:68` (gravar `governance_events` kind `audience` com antes/depois), `security.go:186` (o PUT antigo para de mexer em `require_app_approval`)
- Modify: `mcp_registrar.go:388,417,425`, `mcp_publicar.go:187`, `mcp_versao.go`, `mcp_tools.go:841` — resposta com `situacao: "esperando_aprovacao"` e `mensagem` = `waiting` do pedido (ex.: "Aguardando o líder de Research."); recusado → `msgRejected`
- Modify: `mcp_guia.md` (uma seção curta: "Aprovação": o que dizer à pessoa)
- Test: `governance_test.go`, `mcp_*_test.go` existentes

**Interfaces:**
- `PUT /api/governance/settings` body `{it_approval, version_rule, code_channel, mcp_restricted}`; grava evento `settings` com antes/depois. Ligar TI com pedidos abertos: as etapas `it` são acrescentadas a eles.
- `GET /api/governance/history?item=app:12` (ou toda a empresa): lista unificada de pedidos (com etapas) e eventos, mais novos primeiro, 200 por página.

- [ ] **Step 1: Testes que falham:** `TestSettingsOnlyAdmin`, `TestTurningITOnAddsStepToOpen`, `TestAudienceChangeRecorded`, `TestHistoryAnswersWhoApproved` (pedido aprovado aparece com quem e quando), `TestMCPRegistrarSaysWaitingLeader` (mensagem exata com o nome da área).
- [ ] **Step 2:** → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** → PASS (handler inteiro).
- [ ] **Step 5:** Commit `governança: configurações, histórico e respostas do MCP`.

---

### Task 9: Tela — Aprovações, Início e menu

**Files:**
- Create: `frontend/src/pages/ApprovalsPage.tsx`, `ApprovalsPage.test.tsx`
- Create: `frontend/src/components/governance/ApproveDialog.tsx` (aprovar com escolha de público), `RejectDialog.tsx`, `RequestCard.tsx`
- Modify: `frontend/src/App.tsx` (rota `/aprovacoes`), `components/Layout.tsx:27` (item "Aprovações" com selo do `count`, visível para quem tem algo ou é líder/TI/admin), `pages/HomePage.tsx` (bloco "Esperando sua aprovação", até 5, com "Ver todas"), `lib/api.ts`

**Interfaces:**
- Consumes: rotas da Task 4.
- Cartão do pedido (texto):
  ```
  Baixar Relatórios · versão nova · Research
  Pedido por Ana, há 2 h · pelo Claude · testada ✓
  O que muda no risco: site: xp.com · chave do Cofre: XP_SENHA
  Líder ✓ Paulo   TI — esperando
  [Aprovar]  [Recusar]
  ```
  App novo: "Quem vai usar" com três opções (Só Research · Research e mais… · Empresa toda); "mais…" abre a escolha de áreas e pessoas (reaproveitar o seletor de `AccessSection.tsx`).
- "Foram ao ar com aviso" numa seção recolhida embaixo.

- [ ] **Step 1: Testes que falham** (Vitest): `ApprovalsPage` mostra o cartão com `risk_added`; Aprovar de app novo sem escolher público fica desabilitado; Recusar sem motivo fica desabilitado; `HomePage` mostra o bloco quando `count>0`.
- [ ] **Step 2:** `cd frontend && mise x -- npx vitest run ApprovalsPage HomePage` → FAIL.
- [ ] **Step 3:** Implementar (seguir `DESIGN.md`, preto e branco, ADR 020).
- [ ] **Step 4:** `mise x -- npx vitest run && mise x -- npx tsc -b` → PASS.
- [ ] **Step 5:** Commit `governança: página Aprovações e Início`.

---

### Task 10: Tela — página do app/artifact e Admin

**Files:**
- Modify: `components/ficha/AppControl.tsx` (sem Aprovar; mostra situação: `msgWaitingLeader`/`msgWaitingIT`/recusado com motivo; Suspender continua), `pages/AppPage.tsx:208,275,410,513` (selo e faixas a partir de `/governance`), `pages/ArtifactPage.tsx:163,202-249`
- Create: `components/governance/RuleSection.tsx` (Configurações → "Aprovação de versões": padrão da empresa e o do item; opções mais frouxas só aparecem para quem pode), `components/governance/HistoryList.tsx` (aba Histórico: quem pediu, quem aprovou, quando, mudanças de público e área)
- Modify: `components/ficha/editors.tsx:29-61` (`areaLocked={!can.care}`; ao trocar a área, aviso "A mudança de área espera o líder da área nova.")
- Modify: `components/ficha/AccessSection.tsx` (editar público só com `can.access`; dono vê, sem editar, com a frase "Quem decide quem usa é o líder da área.")
- Modify: `pages/AdminPage.tsx:182,227` — `CompanyControls` perde "Aprovar apps novos antes de rodarem" e ganha a seção **Governança**:
  ```
  Governança
  O líder da área sempre aprova.
  [ ] A TI também aprova (quem tem "Aprovar pela TI" no cargo)
  Versão nova precisa de aprovação:
    ( ) Toda versão   (•) Só quando muda o risco   ( ) Só a primeira
  Por onde o código pode chegar:
    (•) Qualquer caminho   ( ) Só pelo Claude
  [ ] Só cargos com "Conectar o Claude" podem ter chave do MCP
  ```
  Mais "Chaves do MCP" (lista da empresa com Cancelar) e "Histórico de governança".
- Modify: `components/ClaudeConnections.tsx` (mostrar `msgNoMCP` quando 403)
- Modify: tela de Cargos (`TeamPage`) — rótulos: `approve_it` = "Aprovar pela TI", `use_mcp` = "Conectar o Claude"

- [ ] **Step 1: Testes que falham** (Vitest): `RuleSection` esconde opções mais frouxas sem `can_loosen`; `AppControlBar` não tem Aprovar; `AdminPage` envia `PUT /api/governance/settings` com os 4 campos.
- [ ] **Step 2:** → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `vitest run` + `tsc -b` → PASS.
- [ ] **Step 5:** Commit `governança: página do app, artifact e Admin`.

---

### Task 11: Fluxo inteiro no navegador, documentos

**Files:**
- Create: `e2e/governanca.mjs`
- Create: `docs/adr/046-governanca.md`
- Modify: `docs/PRD.md`, `docs/TASKS.md`, `backend/internal/handler/mcp_guia.md` (se ainda faltar)

- [ ] **Step 1:** Escrever `e2e/governanca.mjs` (Playwright puro, `POST /api/dev/login` como nos outros): numa empresa de teste isolada (servidor próprio em porta livre, **não** 8080), Ana (criadora, Research) registra app → vê "Aguardando o líder de Research."; Paulo (líder) abre Aprovações, escolhe "Só Research", aprova; Ana roda; Bia (Investimentos) não vê o app; admin liga TI e "Toda versão"; Ana manda versão nova → app continua rodando a versão anterior; Paulo aprova, TI aprova → versão nova no ar; admin liga "Só pelo Claude" → upload pelo site recusado com `msgOnlyClaude`; histórico mostra quem aprovou cada coisa. Screenshots de Aprovações, Início, página do app (Configurações e Histórico) e Admin → Governança, em 1280×720 e 375×812.
- [ ] **Step 2:** Rodar: `BASE=http://localhost:<porta> OUT=<scratchpad> node e2e/governanca.mjs` → termina sem erro; ler os screenshots e corrigir o que estiver desalinhado.
- [ ] **Step 3:** ADR 046 (contexto, decisões do Henry, minhas decisões do desenho, alternativas descartadas: remendo e motor genérico); PRD (seção de governança); TASKS (projeto feito, pendências: e-mail aos aprovadores depois do domínio).
- [ ] **Step 4:** Testes completos: `cd backend && mise x -- go test ./...` (um processo) e `cd frontend && mise x -- npx vitest run && mise x -- npx tsc -b` → PASS.
- [ ] **Step 5:** Commit `governança: teste no navegador, ADR 046 e documentos` e push do branch.

---

## Registro da execução (2026-11-17/18)

Decisões tomadas durante a construção e a revisão final (cada uma com o custo se estiver errada), e pontos menores adiados.

### Decisões

- Task 1: Ruling: resetSQL keeps role perms without use_mcp — tests asserting exact perm lists broke; use_mcp only matters when mcp_restricted is on (Task 6 tests set it) — cost: none expected
- Task 2: Ruling: browser detected by own regex (playwright/selenium/pyppeteer/puppeteer) instead of recipes.Detect — Detect needs a versions.Folder and is for server recipes; cost if wrong: a browser use via another lib is missed by 'só risco'
- Task 3: Ruling: kept Can.Access = "cares" (old meaning) and added Can.Audience (leader||manage_areas) instead of plan's Care/Access rename — Access is used by archive, schedules, handsOver; renaming risked silent permission changes. Frontend must use can.audience for the audience editor (Task 10) — cost if wrong: owner UI shows an editor that 403s.
- Task 3: Ruling: pending items live in Person.PendingApps/PendingArtifacts (loaded in loadAccess) instead of App.Pending — avoids touching every appRef/artifactRef call — cost: one extra query per request.
- Task 3: Ruling: AutoSteps takes areaHasLeader too — needed for one-person company (admin alone, area without leader) — cost: none.
- Task 3: Ruling: pending + restricted is still visible to admin/TI approvers — they must see it to approve.
- Task 4: Ruling: governance.Effective now returns the item's own rule when set (was: stricter of the two) — spec says the admin may loosen an item below the company; a looser rule that never takes effect contradicts it. Guard is at save time (CanLoosen) — cost if wrong: none, only admin can save looser.
- Task 4: Ruling: openRequest/applyRequest take no pgx.Tx — handler uses pool queries; concurrency is a per-request lock (lockApp(-id), lockApp is non-reentrant and publishDraft takes lockApp(appID)) plus conditional UPDATEs (CloseApprovalRequest/DecideApprovalStep only on open/undecided) — cost if wrong: a crash between apply and record leaves the request open (re-approvable; apply is idempotent).
- Task 4: Ruling: migration 031 adds approval_requests.audience — the leader's audience must survive until TI approves; 030 was already applied in dev.
- Task 4: Ruling: requestIn.RequesterID int64 instead of access.Person — openRequest loads access itself — cost: one query.
- Task 4: Ruling: an item never approved keeps reason "new" whatever arrives (a version on a pending app replaces its open request as "new") — otherwise a no-risk version would auto-approve a never-approved app.
- Task 4: Ruling: when every step is auto (requester is the leader / one-person company), the item keeps the access it was created with (no forced "só a área") — leader can change it in Acesso; forcing "área" hid apps from the company in the principal-company credential test and changed today's behavior — cost if wrong: a leader's own new app is visible company-wide by default (as today).
- Task 4: Ruling: the old require_app_approval gate stays in executionBlock until Task 5 moves version paths to requests (TestSchedulerWaitsOnApproval/TestSourcesWaitsApproval/TestPublishWaitsApproval depend on it); governance tests set require_app_approval=false like a new company.
- Task 4: Ruling: applyRequest for draft and area requests returns errApplyLater — wired in Task 5 with its own tests.
- Task 4: Ruling: FailDraft also accepts status 'aguardando' (reject a waiting draft).
- Task 5: Ruling: executionBlock keeps its signature (no personID); the PC-draft test path uses baseBlock to skip approvals — simpler call sites — cost if wrong: a PC draft test of a held app could run unapproved code (only by its own creator, on their PC).
- Task 5: Ruling: run gate fails closed via approved_version >= latest version — any new version without approval holds runs — cost if wrong: an unexpected version row blocks runs until approved.
- Task 5: Ruling: a ficha edit that adds risk is saved and the app held until approved (no ficha-only draft) — simpler; mirrors folder versions — cost if wrong: app stops running while the ficha waits.
- Task 5: Ruling: restoring to a never-approved version follows the rule; restoring to an approved one is preapproved — cost if wrong: an extra approval round.
- Task 5: Ruling: PUT control still accepts `approved` but ignores it; admin approvedSQL now reads app_governance — cost if wrong: old admin toggle looks like it does nothing (frontend to drop it in Task 10).
- Task 5: Ruling: a never-approved app's new versions go into its folder and keep waiting on the "new" request — cost if wrong: leader approves a later version than the one first requested (they see the latest).
- Task 5: Ruling: a leader moving an app into an area they lead applies directly; otherwise an 'area' request to the target leader — cost if wrong: none expected.
- Task 6: Ruling: off-channel = a version with origin 'folder' (PC folder) in a "só pelo Claude" company; restores and sent versions are not off-channel — restore is an in-Ziv action and approved targets are preapproved — cost if wrong: a site restore to a never-approved version follows the normal rule instead of being refused.
- Task 6: Ruling: also check use_mcp on the OAuth consent screen (approveOAuthRequest), not only on the token exchange — clearer message to the person — cost if wrong: none.
- Task 6: Ruling: channel checks apply even without a workspace (default 'any' keeps old behavior) — cost if wrong: none.
- Task 7: Ruling: waiting/refused artifact versions are visible (detail list and ?v=) to who edits or cares for the item (can.Edit || can.Access), so the leader can preview before deciding; other viewers see only approved versions — cost if wrong: an IT approver without access can't preview the HTML.
- Task 7: Ruling: catalog list "version" = newest approved for everyone — cost if wrong: the author sees the older number in the list until approved.
- Task 7: Ruling: artifact restore to an approved version is preapproved (uses the version's approved flag) — cost if wrong: none.
- Task 8: Ruling: turning TI off leaves the IT steps already open (approve_it holders can still decide them) — removing them would need applying requests outside a decision — cost if wrong: some requests opened before the change still wait for TI.
- Task 8: Ruling: history is only for manage_roles (company-wide audit); the per-item history in /api/{apps|artifacts}/{id}/governance stays for who sees the item — cost if wrong: a leader can't open the company-wide history.
- Task 8: Ruling: added rule 6 to the always-read MCP instructions besides the guia section (the guia is the visual artifacts guide, read on demand) — cost if wrong: none.
- Task 8: Ruling: held drafts store the waiting message in status_reason so ver_versao says who must approve — cost if wrong: none.
- Task 9: Ruling: the card needs the request's area id (custom audience excludes it) and which steps the viewer decides (Aprovar shows only then; audience asked only of the leader) — added area_id and my_steps to the approvals JSON — cost if wrong: two extra fields, none.
- Task 9: Ruling: Aprovações menu shows for leaders, approve_it, manage_roles, or anyone with count>0; Home block lists only open requests with my_steps — cost if wrong: a non-approver sees the item once something waits for them, which is the point.
- Task 10: Ruling: brief says audience editing needs can.access, but the backend (Task 4) gates PUT access on can.audience — the UI uses can.audience (Can type gained optional audience) — cost if wrong: an owner who should edit sees read-only text.
- Task 10: Ruling: AppControlBar derives "waiting" from GET /api/apps/{id}/governance open_request (falls back to control.approved without it); the old require_app_approval toggle and the bar's Aprovar button are gone; a refused last request shows "<who> recusou: <reason>" — cost if wrong: one extra GET per app page.
- Task 10: Ruling: ClaudeConnect already shows the server's 403 message (msgNoMCP) on code creation — no change to ClaudeConnections — cost if wrong: none.
- Task 10: Ruling: artifact governance is fetched only in a company portal (workspaceBasePath() !== ""); the area-change notice reads "Se você não lidera a área nova, a mudança espera o líder dela." because a leader moving into an area they lead applies directly (Task 5 ruling) — cost if wrong: copy only.
- Task 10: Ruling: the governance UI got its own GovernancePanel component in Admin (settings, MCP keys, history) instead of growing CompanyControls — cost if wrong: none.
- Task 11: Ruling: e2e runs in empresa Teste on isolated DEV_MODE servers 8094/5194 (never 8080) and restores the company settings in finally — cost if wrong: none.
- Task 11: Ruling: "Ana roda" is checked via control.approved plus the run response not mentioning approval (no paired computer → 400) — cost if wrong: a run gate regression on a real PC is caught only by Henry's hands-on.
- Task 11: Ruling: after screenshots — on_behalf shown as labels (string union), the "Ver em Aprovações" link only for who may approve (mayApprove in permissions.ts), Home drops approval_pending warnings already in the approval block, RuleSection uses the ficha Section — cost if wrong: none.
- Final: Ruling: I7 implemented as "tightening clears looser item rules", which also clears an admin's deliberate loosening — simplest, no new column — cost if wrong: the admin re-loosens that item after tightening the company.
- Final: Ruling: I8 (risky ficha pauses the approved app) kept and documented in ADR 046 — the ficha is the folder file the app runs; holding it would need a ficha-only draft — cost if wrong: the app stops for users while the leader decides on a new key/24h.
- Final: Ruling: I11 re-graded Minor — artifact version is approved=true for milliseconds between commit and the hold, and stays approved only if that update errors — cost if wrong: an always-latest link may serve an unreviewed version in that window.
- Final: Ruling: governFicha skips while another (non-area) request is open and runs again after each approval — avoids a ficha request replacing a version request — cost if wrong: none expected.
- Final: Ruling: sharedrun test fixture now inserts an approved app_governance row (apps inserted by raw SQL had none and are now held) — cost if wrong: none.

### Menores adiados

- migration doesn't record "aprovado antes da governança" requests; never-approved Empresa Piloto apps become hidden instead of keeping today's audience
- AddMissingITStep is unused (AddITStepToOpen now excludes area moves too) — delete the duplicate
- a pending app can't be run by its creator or approvers to test (only the PC draft test) — confirm intended
- after an area move, a restricted app's viewer list keeps the old area id
- vault keys added on the site (putKey) don't count as risk
- approve_it holders see restricted app names in the "foram ao ar com aviso" list even with TI off
- decidableRequests does ~4 queries per open request on every route change
- openRequest replace+insert is not one transaction (concurrent insert → version held with no open request until the next version)
- artifact versions inserted approved=true until the post-commit hold (I11)
