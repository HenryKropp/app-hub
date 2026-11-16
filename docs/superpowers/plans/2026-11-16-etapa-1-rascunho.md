# Etapa 1 — Rascunho e "não testada" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Uma versão enviada pode ficar guardada como rascunho sem mudar o app no ar. Ela só vai ao ar quando alguém publica, e toda versão que vai ao ar sem teste fica marcada "não testada".

**Architecture:**
- Uma tabela nova, `app_drafts`, guarda os arquivos e as mudanças de ficha de um rascunho.
- Publicar um rascunho reaproveita o miolo atual de `sendSources`, extraído para `applySources`. Esse miolo troca `script_sources`, reconstrói a pasta, grava a versão e salva a ficha. Assim `app_versions` continua sendo só o histórico do que esteve no ar, e a aprovação de segurança, `checkVersionLocked` e o histórico não mudam.
- `app_versions.tested` (nulo / falso / verdadeiro) alimenta a marca na linha do tempo.

**Tech Stack:** Go (net/http, pgx, sqlc), Postgres migrations embutidas, React + Vitest.

**Spec:** `docs/superpowers/specs/2026-11-16-testar-e-publicar-sem-o-henry-design.md` (seção 1). Etapas 2–4 (teste, apps com tela, troca no PC) terão planos próprios.

## Global Constraints

- Migração nova: `backend/internal/database/migrations/027_drafts.sql`. Ela só avança e é idempotente (`IF NOT EXISTS`).
- SQL só em `backend/internal/database/queries/*.sql` com `sqlc generate`. Nunca SQL cru em handler. Exceção: `SecurityDB()`, que já existe.
- Testes de handler em Go não podem rodar em paralelo. Rode com `-p 1`.
- Mensagens ao usuário em português, curtas, sempre dizendo o próximo passo.
- Situações do rascunho (texto exato): `rascunho`, `testando`, `aprovada`, `reprovada`, `no_ar`, `cancelada`.
- Texto exato do motivo de cancelamento: `substituída por versão mais nova`.
- Marca na linha do tempo (texto exato): `não testada` / `testada`. Para `tested` nulo, nenhuma marca.
- Nenhuma rota nova entra na lista de rotas do MCP (`handler/mcp_server.go:97-125`) nesta etapa.

## Review Focus

1. Um rascunho criado e **nunca publicado** não pode mudar nada do que roda: nem `script_sources`, nem a pasta, nem `apps.manifest`, nem `max(number)` de `app_versions` (a aprovação depende disso).
2. Rascunho criado **durante uma execução** é aceito. Publicar durante uma execução é recusado com mensagem, como hoje.
3. Uma ficha editada no site **entre o rascunho e a publicação** não é sobrescrita. As mudanças de ficha do rascunho valem sobre a ficha atual, campo a campo.
4. Rascunho de app de **pasta do PC** (sem `script_sources`) é recusado com `no_stored_code`, como hoje.
5. Rascunho com **senhas no código** passa pela mesma conferência (`checkSentFiles`/`confirm_secrets`) na criação, não só na publicação.

---

### Task 1: Migração e consultas

**Files:**
- Create: `backend/internal/database/migrations/027_drafts.sql`
- Create: `backend/internal/database/queries/drafts.sql`
- Modify: `backend/internal/database/queries/versions.sql` (`InsertAppVersion` recebe `tested`; `ListVersionsBefore` devolve `tested`)
- Regenerate: `backend/internal/database/sqlc/`

**Interfaces:**
- Produces:
  - tabela `app_drafts` com as colunas `id BIGSERIAL PK`, `app_id BIGINT NOT NULL REFERENCES apps ON DELETE CASCADE`, `author_id BIGINT REFERENCES people`, `channel TEXT NOT NULL CHECK (channel IN ('manual','mcp'))`, `files JSONB NOT NULL`, `ficha JSONB` (nulo = sem mudança), `note TEXT NOT NULL DEFAULT ''`, `status TEXT NOT NULL DEFAULT 'rascunho' CHECK (status IN (...as seis...))`, `status_reason TEXT NOT NULL DEFAULT ''`, `version_number INT` (preenchida ao publicar), `created_at`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()`. Índice em `(app_id, created_at DESC)`.
  - `ALTER TABLE app_versions ADD COLUMN IF NOT EXISTS tested BOOLEAN` (nulo nas antigas).
  - Consultas sqlc:
    - `InsertDraft :one`;
    - `GetDraft :one (app_id, id)`;
    - `CancelOpenDrafts :exec (app_id, reason)`, que muda para `cancelada` os rascunhos do app em `rascunho`, `testando` ou `aprovada`;
    - `MarkDraftPublished :exec (id, version_number)`, que muda para `status='no_ar'`.

- [ ] **Step 1:** Escrever `027_drafts.sql` e `drafts.sql`, ajustar `versions.sql`.
- [ ] **Step 2:** `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- sqlc generate'`. Ler os `.go` gerados e anotar os nomes exatos (`InsertAppVersionParams.Tested` deve ser `pgtype.Bool`).
- [ ] **Step 3:** Ajustar `insertVersion` (`handler/versions.go:123`) para receber `tested pgtype.Bool` e repassar. Chamadores:
  - `checkVersionLocked`, origem `folder`: `tested` falso. Origem `original`: nulo.
  - `restore.go:156`: nulo, porque voltar versão não testa de novo.
  - `recordSentVersion`: recebe o valor do chamador.
- [ ] **Step 4:** `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go build ./... && mise x -- go test -p 1 ./internal/handler/ -run "Version|Restore|Sources"'`. Esperado: PASS (comportamento igual).
- [ ] **Step 5:** Commit `Etapa 1: tabela de rascunhos e marca tested nas versões`.

### Task 2: Extrair `applySources`

**Files:**
- Modify: `backend/internal/handler/stored_code.go:192-323`
- Test: `backend/internal/handler/stored_code_test.go` (os testes `TestSources*` existentes são a rede de proteção)

**Interfaces:**
- Produces: `func (h *Handler) applySources(w http.ResponseWriter, r *http.Request, a sqlc.App, me sqlc.Person, files map[string]manifest.InputFile, ficha *fichaUpdate, note string, tested pgtype.Bool) (sqlc.App, sqlc.AppVersion, bool, bool)`, que devolve o app salvo, a versão, se mudou e se deu certo (falha já respondida em `w`). Ela engloba:
  - o `Hold` do serviço compartilhado;
  - a conferência de `script_sources` (`no_stored_code`);
  - o `lockApp`;
  - a recusa por execução ativa;
  - `ReplaceScriptSources`, a pasta e `recordSentVersion`;
  - `writeApp` da ficha.
  
  Ela **não** faz o `checkSentFiles` (quem chama faz antes) nem a conferência de `updated_at`.
- `sendSources` passa a ser: decodificar → conferir `updated_at` se houver ficha → `applyFicha` + `Validate` → `checkSentFiles` → `applySources(..., tested=false)` → responder como hoje.

- [ ] **Step 1:** Adicionar a `TestSourcesNewVersion` a asserção `SELECT tested FROM app_versions WHERE app_id=$1 AND number=2` → `false`, e a de que a versão 1 (import) tem `tested` nulo.
- [ ] **Step 2:** Rodar `go test -p 1 ./internal/handler/ -run TestSourcesNewVersion`. Esperado: FAIL (`tested` nulo).
- [ ] **Step 3:** Extrair `applySources` e reescrever `sendSources` sobre ela.
- [ ] **Step 4:** `go test -p 1 ./internal/handler/ -run "Sources|Import|Registrar"`. Esperado: PASS.
- [ ] **Step 5:** Commit `Etapa 1: applySources e versões do Claude marcadas não testadas`.

### Task 3: Criar e publicar rascunho

**Files:**
- Create: `backend/internal/handler/drafts.go`
- Create: `backend/internal/handler/drafts_test.go`
- Modify: `backend/internal/handler/handler.go:154` (rotas)

**Interfaces:**
- Consumes: `applySources` (Task 2), consultas da Task 1.
- Produces, para as Etapas 2–4:
  - `POST /api/apps/{id}/drafts`: o corpo é igual ao de `/sources` (`files`, `confirm_secrets`, `note`, `ficha`). Exige `can.Edit`. Recusa `no_stored_code` como hoje. Valida a ficha resultante (`applyFicha` sobre a ficha atual + `Validate`) e roda `checkSentFiles`. Em seguida faz `CancelOpenDrafts(app, "substituída por versão mais nova")` e `InsertDraft`. **Não** recusa por execução ativa. Responde `201 {"draft_id", "status":"rascunho"}`.
  - `GET /api/apps/{id}/drafts/{d}`: devolve `{"id","status","status_reason","version_number","note","created_at"}`. Exige `can.Edit`.
  - `func (h *Handler) publishDraft(w, r, a sqlc.App, me sqlc.Person, d sqlc.AppDraft, tested bool) (sqlc.AppVersion, bool)`: decodifica `files` e `ficha` do rascunho e chama `applySources(..., tested)`. Se der certo, `MarkDraftPublished(d.id, v.Number)`. Recusa (409 `draft_closed`, "Este rascunho não está mais aberto.") quando o status é `no_ar` ou `cancelada`.
  - `POST /api/apps/{id}/drafts/{d}/publish`: exige `can.Edit`. Chama `publishDraft(..., tested=false)`. Nesta etapa é a porta manual. A Etapa 2 chama `publishDraft(..., true)` depois do teste.

- [ ] **Step 1: Testes que falham** em `drafts_test.go`, usando os helpers de `stored_code_test.go` (`importFiles`, `storedNames`, `appFolder`, `versionCount`):
  - `TestDraftDoesNotChangeLiveApp`: depois do import v1, criar rascunho `print('v2')` → 201. `storedNames(...)["main.py"] == "print('v1')"`, o arquivo da pasta continua v1, `versionCount == 1` e `apps.manifest` fica igual.
  - `TestDraftAcceptedDuringRun`: com uma execução criada (padrão de `TestSourcesRefusedDuringRun`), criar rascunho → 201. Publicar → 409 contendo "Espere a execução terminar".
  - `TestDraftPublishMakesVersion`: rascunho com nota "Corrige" → publicar → 200. `versionCount == 2`, v2 com `origin='upload'`, `description='Corrige'` e `tested=false`. O rascunho fica com `status='no_ar'` e `version_number=2`. Publicar de novo → 409 `draft_closed`.
  - `TestNewDraftCancelsOpenOne`: rascunho A, depois rascunho B. A fica `cancelada` com motivo `substituída por versão mais nova`. Publicar A → 409 `draft_closed`.
  - `TestDraftFichaAppliesOverCurrent`: rascunho com `ficha.summary="Novo resumo"`. Depois, editar no site outro campo da ficha (por exemplo `outputs`). Publicar: o manifest final tem o resumo novo **e** as saídas editadas.
  - `TestDraftFolderAppRefused`: app de pasta (`insertApp(t, filesPrepManifest())`) → 409 `no_stored_code`.
  - `TestDraftSecretsNeedConfirm`: código com uma senha → mesma recusa de `TestSourcesSecretsNeedConfirm`, e nenhum rascunho gravado.
  - `TestDraftNeedsEdit`: usuário sem edição → 403.
- [ ] **Step 2:** `go test -p 1 ./internal/handler/ -run Draft`. Esperado: FAIL (rotas 404).
- [ ] **Step 3:** Implementar `drafts.go` e registrar as três rotas ao lado de `/sources`.
- [ ] **Step 4:** `go test -p 1 ./internal/handler/ -run "Draft|Sources"`. Esperado: PASS.
- [ ] **Step 5:** Commit `Etapa 1: rascunho de versão (criar, ver, publicar)`.

### Task 4: Marca "não testada" / "testada" na linha do tempo

**Files:**
- Modify: `backend/internal/handler/history.go:66` (`versionItem` ganha `Tested *bool json:"tested"`; `newVersionItem` recebe `pgtype.Bool`)
- Modify: `frontend/src/lib/api.ts` (tipo da versão ganha `tested?: boolean | null`)
- Modify: `frontend/src/components/HistoryTimeline.tsx:57` (marca ao lado do `ByClaude`)
- Test: `backend/internal/handler/history_test.go` (ou o teste de histórico existente) e `frontend/src/components/HistoryTimeline.test.tsx`

**Interfaces:**
- Consumes: coluna `tested` (Task 1).

- [ ] **Step 1: Testes que falham.**
  - Go: depois de `send(...)` de uma v2, o histórico do app traz `tested: false` na v2 e `tested: null` na v1.
  - Vitest: `HistoryTimeline` com uma versão `tested:false` mostra o texto `não testada`, com `tested:true` mostra `testada`, e com `tested:null` não mostra nenhum dos dois.
- [ ] **Step 2:** Rodar os dois. Esperado: FAIL.
- [ ] **Step 3:** Implementar. A marca segue DESIGN.md (preto e branco): texto pequeno e discreto, em `text-muted-foreground`, com borda fina para "não testada". Sem cor de alerta.
- [ ] **Step 4:** `go test -p 1 ./internal/handler/ -run History` e `cd frontend && mise x -- npx vitest run HistoryTimeline && mise x -- npx tsc -b`. Esperado: PASS.
- [ ] **Step 5:** Visual: subir o app local (DEV_MODE), mandar uma versão para um app de teste, tirar print da aba Histórico com o Playwright e conferir alinhamento e contraste.
- [ ] **Step 6:** Commit `Etapa 1: marca não testada na linha do tempo`.

### Task 5: Aviso "vai ao ar sem teste" no Publicar do servidor

**Files:**
- Modify: `frontend/src/components/ficha/shared/Publish.tsx:112`
- Test: `frontend/src/components/ficha/shared/Publish.test.tsx`

Nesta etapa nenhum app tem plano de teste, então **todo** "Publicar versão nova" mostra o aviso. A Etapa 2 troca o botão para "Testar e publicar" quando o plano existir.

- [ ] **Step 1: Teste que falha:** clicar em "Publicar versão nova" abre uma confirmação com o texto exato `Esta versão vai ao ar sem teste.` e os botões `Publicar mesmo assim` e `Cancelar`. Só `Publicar mesmo assim` chama a publicação.
- [ ] **Step 2:** `npx vitest run Publish`. Esperado: FAIL.
- [ ] **Step 3:** Implementar com o diálogo já usado no arquivo (o de "Voltar para a anterior"). Isso vale para o primeiro "Publicar" e para "Publicar versão nova".
- [ ] **Step 4:** `npx vitest run Publish && npx tsc -b`. Esperado: PASS. Tirar print do diálogo.
- [ ] **Step 5:** Docs: em `docs/TASKS.md`, criar a linha "Testar e publicar — Etapa 1 rascunho: Done". Escrever o ADR `docs/adr/043-rascunho-de-versao.md` ("rascunho em tabela própria; app_versions continua só o que esteve no ar"). Commit `Etapa 1: aviso de publicar sem teste` e push.
