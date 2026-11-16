# Etapa 2 — Teste de script no PC e `enviar_versao` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um script enviado pelo MCP com `enviar_versao` vira rascunho. O Ziv testa esse rascunho num PC pelo Agente, em modo teste e com as chaves do Cofre. Se o teste passa, o Ziv publica sozinho como "testada" e grava um aviso. Se falha, nada muda no ar e quem enviou recebe a evidência pelo `ver_versao`.

**Architecture:**
- O rascunho da Etapa 1 (`app_drafts`) ganha uma **etapa** (`stage`) e os campos do teste.
- Uma máquina de estados guardada no banco anda pelas etapas: esperando confirmação → esperando o PC → rodando → publicando. Ela é movida de dois jeitos:
  - por um relógio por empresa (`StartTestPipeline`, igual ao `StartScheduler`);
  - pelo resultado que o Agente manda.
- O teste é um trabalho novo do Agente (`kind=test`), separado de `runs`, para não aparecer em Execuções nem mexer em agendamentos.
- A conferência ("passou?") é uma função pura num pacote novo, `internal/testplan`.
- Publicar reaproveita `publishDraft(..., tested=true)` da Etapa 1.

**Tech Stack:** Go (net/http, pgx, sqlc, MCP go-sdk), Agente Go (Windows; testes rodam no Linux via `platform_other.go`), React + Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-16-testar-e-publicar-sem-o-henry-design.md`. Esta etapa cobre:
- §2 plano de teste, só a parte de script;
- §3 confirmação;
- §4 rodada de teste, só no PC;
- §5 conferência e publicação;
- §7 avisos;
- §8 MCP, só para scripts.

Ficam para depois: apps com tela (Etapa 3: `acao`, `tela_contem`, foto, "Testar e publicar" no site) e a troca no PC (Etapa 4).

## Global Constraints

- Migração nova: `backend/internal/database/migrations/028_test_runs.sql`. Ela só avança e é idempotente.
- SQL só em `queries/*.sql` + `sqlc generate`. Exceção: `SecurityDB()`, que já existe. Testes de handler em Go rodam com `-p 1`.
- Versão do Agente: `agentproto.Version = "0.4.0"`. O teste só vai para um PC com essa versão. Abaixo dela, o Ziv espera, porque a troca automática do Agente já existe.
- Receita, nomes exatos (YAML e JSON): `teste`, `acessa`, `pula`, `entradas`, `espera`, `arquivos`, `no_minimo`, `log_contem`, `tempo_maximo_min`. Nesta etapa, `tela_contem` e `acao` são recusados em script com a mensagem `tela_contem e acao são só para apps com tela.`
- Modo teste, nomes exatos: `ZIV_TESTE=1`, `ZIV_PASTA_TESTE=<pasta>`. Pasta no PC: `<StateDir>\testes\app-<appID>\rascunho-<draftID>`.
- Etapas do rascunho (`stage`, texto exato): `esperando_confirmacao`, `esperando_pc`, `rodando`, `publicando`. A etapa fica vazia fora de `testando`/`aprovada`.
- Rótulos para pessoas e para o MCP (texto exato):
  - `esperando confirmação do plano de teste`
  - `esperando o PC de teste ligar`
  - `esperando o Agente do PC de teste se atualizar`
  - `escolha o PC de teste na confirmação do plano`
  - `testando no PC — N min`
  - `publicando`
- Limites:
  - evidência de até 20 MB e 200 arquivos por teste;
  - evidência guardada por 7 dias;
  - 24 h esperando o PC;
  - `tempo_maximo_min` de 1 a 15, com padrão 15 (o Agente já corta scripts em 15 min);
  - folga de 5 min além do tempo máximo antes de reprovar por tempo.
- Texto exato da confirmação: `Os próximos testes deste app serão automáticos. Só pergunto de novo se o plano de teste mudar.`
- Texto exato do aviso: `<Nome> versão <N> no ar — testada: <resumo>`. O resumo vem do primeiro item `arquivos`: `1 arquivo salvo` / `N arquivos salvos`.
- Recusa sem plano (texto exato): `Falta a seção teste da receita. Escreva a seção teste; veja receita_app_ziv.`
- Uma senha do Cofre nunca aparece em `test_log`, no `ver_versao`, num aviso ou na evidência do console. O conteúdo dos arquivos gerados não é mascarado (spec §4).
- A confirmação do plano e a troca do PC de teste são só pelo site: as rotas **não** entram em `mcpRoutes`, e o handler recusa quando `viaMCP(ctx)`.

## Review Focus

1. **Senha no print do script.** Um script de teste que faz `print(os.environ["SENHA"])` não pode deixar o valor em `app_drafts.test_log`, no JSON do `GET .../drafts/{d}`, no texto do `ver_versao` nem em `draft_evidence`. O console não vai na evidência. A máscara roda no Agente e de novo no servidor. Testes: Task 3 (Agente) e Task 4 (servidor).
2. **Teste reprovado não muda nada.** Depois de um teste que falha, o app fica igual: `script_sources`, a pasta, `apps.manifest` e a contagem de `app_versions`. Nenhum aviso é gravado. Teste: Task 5.
3. **Reinício do servidor no meio.** Um rascunho em `esperando_pc` ou `publicando` precisa continuar depois que um `Handler` novo roda `testPipelineTick` sobre o mesmo banco. Teste: Task 5.
4. **Plano mudou depois da confirmação.** Mudar `acessa`, `pula`, os nomes de `keys` ou o lugar faz o rascunho parar em `esperando_confirmacao`. Mudar `espera`, `entradas` ou `tempo_maximo_min` não faz. Testes: Task 1 (impressão) e Task 6 (fluxo).
5. **O MCP não confirma o próprio plano.** As rotas de confirmação não podem ser chamadas pelo `toolAPI`, e `POST .../test-plan/confirm` com contexto MCP responde 403. Teste: Task 6.

---

### Task 1: Seção `teste` na receita e o pacote `testplan`

**Files:**
- Create: `backend/internal/manifest/teste.go`, `backend/internal/manifest/teste_test.go`
- Modify: `backend/internal/manifest/manifest.go` (`Manifest.Teste *Teste`, `Validate` chama `testeProblems`)
- Create: `backend/internal/testplan/testplan.go`, `backend/internal/testplan/testplan_test.go`
- Modify: `backend/internal/handler/stored_code.go` (`fichaUpdate.Teste *manifest.Teste`, aplicado em `applyFicha`)
- Modify: `backend/internal/handler/mcp_registrar.go` (`fichaChanges` reconhece `teste`)

**Interfaces:**
- Produces:
  - `type Teste struct { Acessa []string; Pula string; Entradas map[string]string; Espera []Espera; Acao string; TempoMaximoMin int }`, com as tags yaml/json da Global Constraints.
  - `type Espera struct { Arquivos string; NoMinimo int; LogContem string; TelaContem string }`.
  - `func (t Teste) Timeout() time.Duration`: 15 min quando `TempoMaximoMin` é 0.
  - `testplan.Fingerprint(m manifest.Manifest, place string) string`: SHA-256 hex do JSON de `{acessa, pula, nomes de keys ordenados, place}`. `place` é `"pc"` nesta etapa.
  - `testplan.Check{Item, Detail string; OK bool}` com tags json `item`, `detail`, `ok`.
  - `testplan.Evaluate(t manifest.Teste, exit int, files []agentproto.FileEntry, log string) []testplan.Check`.
  - `testplan.Passed([]Check) bool`.
  - `testplan.Summary(checks []Check) string` (o resumo do aviso).
  - `testplan.ResolveInputs(m manifest.Manifest, now time.Time) map[string][]string`: `"hoje"` numa entrada de data vira `now` em America/Sao_Paulo no formato `2006-01-02`. As outras entradas passam como estão.

- [ ] **Step 1: Testes que falham**
  - `TestTesteProblems`, em tabela. Cada linha abaixo é recusada:
    - sem `acessa`;
    - `pula` vazio (`"nada"` é aceito);
    - `espera` sem `arquivos` (script);
    - `tela_contem` ou `acao` em script;
    - `tempo_maximo_min` 16 ou -1;
    - `arquivos` com glob inválido (`"[a"`);
    - `no_minimo` negativo.
    
    Uma receita completa passa. Um script **sem** `teste` continua válido, porque a obrigação vem no `enviar_versao`.
  - `TestFingerprint`. A impressão não muda quando mudam `espera`, `entradas` ou `tempo_maximo_min`. Ela muda quando mudam `acessa`, `pula`, um nome de `keys` ou `place`. A ordem de `keys` não importa.
  - `TestEvaluate`:
    - `arquivos: "*.pdf", no_minimo: 2` com `a.pdf` e `sub/b.pdf` dá `OK` com Detail `arquivos *.pdf: 2 (mínimo 2)`;
    - `.apphub-console.txt` nunca conta;
    - `log_contem: "Concluído"` confere o log;
    - todo resultado tem o item `código de saída 0`, que falha com saída 2;
    - `Summary` de 12 arquivos é `12 arquivos salvos`; de 1 é `1 arquivo salvo`.
  - `TestResolveInputs`: `"hoje"` numa entrada `date` vira a data do `now` dado.
- [ ] **Step 2:** `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/manifest/ ./internal/testplan/'`. Esperado: FAIL (tipos não existem).
- [ ] **Step 3:** Implementar. O glob vale para o caminho relativo **ou** só o nome (`path.Match`), então `*.pdf` pega `sub/b.pdf`. `no_minimo` 0 conta como 1.
- [ ] **Step 4:** O mesmo comando, mais `go test -p 1 ./internal/handler/ -run "Ficha|Registrar|Sources|Draft"`. Esperado: PASS.
- [ ] **Step 5:** Commit `Etapa 2: seção teste na receita e conferência do teste`.

### Task 2: Banco

**Files:**
- Create: `backend/internal/database/migrations/028_test_runs.sql`
- Modify: `backend/internal/database/queries/drafts.sql`, `agents.sql`, `apps.sql`, `versions.sql`
- Create: `backend/internal/database/queries/test_plans.sql`, `notices.sql`
- Regenerate: `backend/internal/database/sqlc/`

**Interfaces:**
- Produces (colunas):
  - `app_drafts` + `stage TEXT NOT NULL DEFAULT ''` com CHECK nos quatro valores ou `''`.
  - `app_drafts` + `stage_since TIMESTAMPTZ`.
  - `app_drafts` + `test_job_id BIGINT`.
  - `app_drafts` + `test_started_at TIMESTAMPTZ`.
  - `app_drafts` + `test_exit_code INT`.
  - `app_drafts` + `test_log TEXT NOT NULL DEFAULT ''`.
  - `app_drafts` + `test_checks JSONB`.
  - `app_drafts` + `test_files JSONB` (lista `{path,size,saved}`).
  - `app_test_plans(app_id BIGINT PK REFERENCES apps ON DELETE CASCADE, fingerprint TEXT NOT NULL, computer_id BIGINT REFERENCES computers ON DELETE SET NULL, confirmed_by BIGINT REFERENCES people, confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now())`.
  - `draft_evidence(id BIGSERIAL, draft_id BIGINT NOT NULL REFERENCES app_drafts ON DELETE CASCADE, path TEXT NOT NULL, size BIGINT NOT NULL, content BYTEA NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(draft_id, path))`.
  - `notices(id BIGSERIAL, app_id BIGINT NOT NULL REFERENCES apps ON DELETE CASCADE, version INT NOT NULL, text TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), read_at TIMESTAMPTZ)`.
  - `apps` + `in_test BOOLEAN NOT NULL DEFAULT false`.
  - `agent_jobs` + `draft_id BIGINT REFERENCES app_drafts ON DELETE CASCADE`.
  - `agent_jobs_kind_check` recriada com `'test'` (o padrão de `011_stop.sql`).
- Produces (consultas sqlc):
  - `SetDraftStage :execrows (id, from_stage, to_stage)`: muda só se a etapa atual é `from_stage` e atualiza `stage_since`. Assim o relógio e o resultado do Agente nunca andam a mesma etapa duas vezes.
  - `StartDraftTest :execrows (id)`: `rascunho` → `testando`/`esperando_confirmacao`.
  - `SetDraftTestJob :exec (id, job_id)`.
  - `SaveDraftResult :exec (id, exit_code, log, checks, files)`.
  - `FailDraft :exec (id, reason)`: `reprovada`, etapa vazia.
  - `ApproveDraft :execrows (id)`: `testando`/`rodando` → `aprovada`/`publicando`.
  - `ListActiveDrafts :many`: `testando` ou `aprovada`, com a etapa.
  - `GetTestPlan`, `UpsertTestPlan`, `SetTestPlanComputer`.
  - `InsertEvidence`, `GetEvidence (draft_id, path)`, `DeleteOldEvidence :exec` (mais de 7 dias), `DeleteOldClosedDrafts :exec` (`cancelada`/`reprovada` com mais de 30 dias).
  - `InsertNotice`, `ListNotices :many (app_ids)`, `ReadNotice :exec`.
  - `ClearInTest :exec (app_id)`: `in_test=false, restricted=false WHERE in_test`.
  - `MarkVersionTested :exec (app_id, number)`.
  - `GetJob` já existe; a nova `CreateTestJob :one (computer_id, draft_id, payload)` cria `kind='test'`.

- [ ] **Step 1:** Escrever a migração e as consultas.
- [ ] **Step 2:** `sqlc generate`, `go build ./...`. Anotar os nomes gerados para as Tasks 4–6.
- [ ] **Step 3:** `go test -p 1 ./internal/handler/ -run "Draft|Agent|Migration"`. Esperado: PASS (a migração aplica no banco de teste e nada muda de comportamento).
- [ ] **Step 4:** Commit `Etapa 2: tabelas do teste, avisos e confirmação`.

### Task 3: Agente roda o teste

**Files:**
- Modify: `backend/internal/agentproto/proto.go`
- Modify: `backend/internal/agent/scripts.go`, `agent.go`, `client.go`
- Create: `backend/internal/agent/test_run.go`, `backend/internal/agent/test_run_test.go`
- Create: `backend/internal/mask/mask.go`, `mask_test.go`

**Interfaces:**
- Produces:
  - `agentproto.Version = "0.4.0"` e `agentproto.KindTest = "test"`.
  - `agentproto.TestPayload{DraftID, AppID int64; AppName string; Script ScriptPayload; Keys bool; TimeoutMin int; KeyValues map[string]string json:"-"}`.
  - `agentproto.Result.Log string json:"log,omitempty"`: só no teste, já mascarado.
  - `agentproto.MaxEvidenceBytes = 20 << 20`, `MaxEvidenceFiles = 200`.
  - `mask.Values(text string, values []string) string`: troca cada valor com 4 caracteres ou mais por `••••`, do mais longo para o mais curto. É a mesma regra de `deploy.MaskSecrets`; `MaskSecrets` passa a usar essa função.
  - `func (r *Runner) RunTest(ctx context.Context, p agentproto.TestPayload, folder string) agentproto.Result`.
  - `func (c *Client) UploadEvidence(ctx context.Context, jobID int64, body io.Reader) error` → `POST /api/agent/jobs/{id}/evidence` (zip, `application/zip`).
- `runScript` é dividido em `runScriptIn(ctx, p RunPayload, o scriptOpts)`, com `scriptOpts{Subdir string; ExtraEnv []string; Timeout time.Duration; SaveRecord bool}`. A execução normal continua idêntica: `Subdir "Execucao-N"`, 15 min, `SaveRecord`. O teste usa:
  - `OutputFolder = folder`;
  - `Subdir ""`;
  - `ExtraEnv` com `ZIV_TESTE=1` e `ZIV_PASTA_TESTE=folder`;
  - `Timeout = TimeoutMin`;
  - nada gravado em `saveRecord`.

- [ ] **Step 1: Testes que falham** (em `test_run_test.go`, com script Python, como os de `scripts_test.go`):
  - `TestRunTestModeEnv`: o script grava `env.txt` com `ZIV_TESTE`, escreve `b.csv` em `ZIV_PASTA_TESTE` e `a.csv` na pasta de trabalho. `Result.Files` traz os dois, e `env.txt` diz `1`.
  - `TestRunTestMasksKeys`: com `KeyValues{"SENHA":"segredo123"}`, o script faz `print(os.environ["SENHA"])`. `Result.Log` contém `••••` e não contém `segredo123`.
  - `TestRunTestFreshFolder`: a pasta de um teste anterior com o mesmo `rascunho-N` é limpa antes.
  - `TestEvidenceZipLimit`: `evidenceZip(folder, limit)` não inclui `.apphub-console.txt`. Ao passar do limite, ele para de incluir arquivos e devolve os nomes deixados de fora.
  - `TestMaskValues` em `mask_test.go`: um valor curto (`abc`) não é trocado, e `segredo123` dentro de `xsegredo123y` é.
- [ ] **Step 2:** `bash -c 'cd "$(git rev-parse --show-toplevel)/backend" && mise x -- go test ./internal/agent/ ./internal/mask/'`. Esperado: FAIL.
- [ ] **Step 3:** Implementar.
  - `handle` ganha `case KindTest`. Ele busca as chaves se `Keys` (o mesmo `Client.Keys`) e monta a pasta da Global Constraints.
  - Depois chama `RunTest`, envia `UploadEvidence` (por pipe, com `evidenceZip`) e, por fim, `report`. A evidência vai **antes** do resultado.
  - O log é `.apphub-console.txt` mascarado com os valores de `KeyValues`.
- [ ] **Step 4:** O mesmo comando, mais `go test ./internal/shared/deploy/`. Esperado: PASS.
- [ ] **Step 5:** Commit `Etapa 2: Agente 0.4.0 roda o teste em modo teste`.

### Task 4: Servidor recebe o teste

**Files:**
- Create: `backend/internal/handler/test_runs.go`, `backend/internal/handler/test_runs_test.go`
- Modify: `handler/agents.go` (`agentResult`: `case KindTest`), `handler/app_keys.go` (`agentKeys` aceita `kind='test'`), `handler/handler.go` (rota da evidência)

**Interfaces:**
- Consumes: Task 1 (`testplan.Evaluate`, `Passed`), Task 2 (consultas), Task 3 (payload, `Result.Log`).
- Produces:
  - `func (h *Handler) agentEvidence(w, r)`: `POST /api/agent/jobs/{id}/evidence`.
    - O trabalho precisa ser `kind=test`, `status=taken` e do computador.
    - O zip chega com até `MaxEvidenceBytes` + 1 MB de folga.
    - Cada arquivo com `manifest.SafeRelative` vira uma linha em `draft_evidence`. O resto é ignorado.
  - `func (h *Handler) onTestResult(ctx context.Context, j sqlc.AgentJob, res agentproto.Result) error`:
    - `started`: nada.
    - `failed`: `FailDraft` com o motivo `O teste não rodou: <res.Error>`.
    - `finished`:
      1. mascara `res.Log` de novo com `h.keyValues(app)` (`mask.Values`);
      2. avalia o rascunho com o manifesto (`applyFicha(atual, draft.ficha)`) e `testplan.Evaluate`;
      3. grava com `SaveDraftResult`, com `test_files` marcando `saved` para quem está em `draft_evidence`;
      4. se `Passed` → `ApproveDraft` e `h.advanceDraft` (Task 5). Senão → `FailDraft` com o motivo `O teste não passou: <Detail do primeiro item que falhou>`.
  - `agentKeys` entrega as chaves de um trabalho `test` pelo `draft_id` → app. O registro de auditoria fica em nome do autor do rascunho, com a rota `GET /api/agent/jobs/{id}/keys`.

- [ ] **Step 1: Testes que falham.** Os testes enfileiram o trabalho direto com `CreateTestJob`, porque a Task 5 ainda não existe, e usam o agente falso (`pairComputer`, `c.agent`) como em `drafts_test.go`.
  - `TestTestResultPassesAndApproves`: resultado `finished` com saída 0 e `a.pdf` → o rascunho fica `aprovada` e `test_checks` traz 2 itens OK.
  - `TestTestResultFailsKeepsLive`: saída 1 → `reprovada`, com motivo começando por `O teste não passou:`. `script_sources` e `versionCount` não mudam.
  - `TestTestLogMaskedOnServer`: o app tem a chave `SENHA=segredo123` no Cofre e o Agente manda um `Log` com `segredo123` sem máscara. `test_log` não contém o valor.
  - `TestEvidenceStoredAndScoped`: um zip com `a.pdf` e `../x` grava só `a.pdf`. Um outro computador recebe 404. Um trabalho `run` recebe 400.
  - `TestAgentKeysForTestJob`: um trabalho `test` tomado recebe as chaves uma vez. A segunda chamada dá 404.
- [ ] **Step 2:** `go test -p 1 ./internal/handler/ -run "TestResult|TestLog|Evidence|AgentKeysForTest"`. Esperado: FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** O mesmo comando, mais `-run "Agent|Keys|Run"`. Esperado: PASS.
- [ ] **Step 5:** Commit `Etapa 2: servidor recebe evidência e confere o teste`.

### Task 5: A sequência (relógio, publicação, aviso)

**Files:**
- Create: `backend/internal/handler/test_pipeline.go`, `backend/internal/handler/test_pipeline_test.go`
- Modify: `handler/drafts.go` (`publishDraft` confere a situação **dentro** da trava; rascunho sem mudança marca a versão atual como testada)
- Modify: `handler/stored_code.go` (`sendSources`) e `handler/restore.go`: os dois cancelam rascunhos abertos com o motivo `substituída por outra versão no ar`
- Modify: `handler/launch.go` (recusa app `in_test`)
- Modify: `internal/workspace/portal.go:194` (`h.StartTestPipeline(p.clock)` ao lado de `StartScheduler`)

**Interfaces:**
- Consumes: Task 2 (consultas), Task 4 (`onTestResult` chama `advanceDraft`).
- Produces:
  - `func (h *Handler) StartTestPipeline(ctx context.Context)`: relógio de 10 s que chama `testPipelineTick`.
  - `func (h *Handler) testPipelineTick(ctx context.Context, now time.Time)`: `advanceDraft` em cada `ListActiveDrafts`, mais `DeleteOldEvidence` e `DeleteOldClosedDrafts`.
  - `func (h *Handler) advanceDraft(ctx context.Context, d sqlc.AppDraft, now time.Time) error`, que anda uma etapa por vez com `SetDraftStage` condicional:
    - `esperando_confirmacao`:
      - a impressão do manifesto do rascunho igual a `app_test_plans.fingerprint` → `esperando_pc`.
    - `esperando_pc`:
      - mais de 24 h nesta etapa → `FailDraft("O PC de teste ficou 24 h sem ligar ao Ziv.")`;
      - plano sem `computer_id`, computador desconectado ou Agente em versão diferente → fica, e o rótulo diz qual;
      - pronto → `executionBlock` vazio, chaves completas, `CreateTestJob` com `TestPayload` (manifesto do rascunho, arquivos, `testplan.ResolveInputs`, `Keys` pela mesma regra de `launchRun`) → `rodando`.
    - `rodando`:
      - o trabalho expirou sem ser tomado (`status='failed'` sem resultado) → volta para `esperando_pc`;
      - passou de `test_started_at + Timeout() + 5 min` → `FailDraft("O teste passou do tempo máximo de N min.")`.
    - `publicando`:
      - `h.publishDraftBG(ctx, d)`;
      - deu certo → `ClearInTest`, `InsertNotice` com o texto da Global Constraints;
      - foi recusado por execução em andamento → fica e tenta de novo no próximo tique;
      - outra recusa → `FailDraft("Não consegui publicar: <mensagem>")`.
  - `func (h *Handler) publishDraftBG(ctx context.Context, d sqlc.AppDraft) (version int32, refusal string, retry bool, err error)`:
    - chama `publishDraft(..., tested=true)` com um `httptest.ResponseRecorder` e um `*http.Request` feito sobre `ctx`;
    - a pessoa é o **autor do rascunho**;
    - quando `d.Channel=="mcp"`, o contexto leva `mcpCtxKey{}` com o id do autor, para o canal ficar `mcp`;
    - lê status e corpo do recorder para devolver `refusal` e `retry` (409 com `Espere a execução terminar`).
  - `drafts.go` ganha `func draftStageLabel(d sqlc.AppDraft, plan *sqlc.AppTestPlan, pc *sqlc.Computer, now time.Time) string`, com os rótulos da Global Constraints.

- [ ] **Step 1: Testes que falham.** O relógio é chamado à mão (`h.testPipelineTick(ctx, now)`), e `now` é avançado para simular 24 h.
  - `TestPipelineWaitsConfirmation`: sem plano confirmado, o rascunho fica em `esperando_confirmacao` e nenhum trabalho `test` é criado.
  - `TestPipelineQueuesWhenPCOnline`: com o plano confirmado e o PC ligado na versão `0.4.0`, um tique gera `rodando` e um `agent_jobs` `kind=test` com `draft_id`.
  - `TestPipelineWaitsOldAgent`: com o Agente em `0.3.5`, o rascunho fica em `esperando_pc` com o rótulo `esperando o Agente do PC de teste se atualizar`.
  - `TestPipelineFailsAfter24h`.
  - `TestPipelineResumesAfterRestart`: um rascunho em `publicando` no banco é publicado por um `Handler` novo (outro `newPoolClient`) no primeiro tique. Isso cobre o item 3 da Review Focus.
  - `TestPassedTestPublishesAndNotifies`, ponta a ponta com o agente falso:
    - `app_versions` ganha a versão N com `tested=true`, `origin` e canal do autor;
    - `notices` traz `<Nome> versão N no ar — testada: 1 arquivo salvo`;
    - o rascunho fica `no_ar`.
  - `TestFailedTestChangesNothing`: item 2 da Review Focus, incluindo zero avisos.
  - `TestPublishWaitsForRun`: com uma execução normal ativa, o rascunho fica em `publicando`. Quando ela termina, o próximo tique publica.
  - `TestSourcesCancelsOpenDraft` e `TestRestoreCancelsOpenDraft`.
  - `TestUnchangedDraftMarksCurrentTested`: um rascunho com o mesmo código do app no ar, depois do teste, gera `MarkVersionTested` na versão atual e nenhuma versão nova.
  - `TestInTestAppRefusesRuns`: `launchRun` de um app `in_test` responde `refuse(409, "in_test", "Este app ainda está em teste. Ele fica disponível depois que o primeiro teste passar.")`.
- [ ] **Step 2:** `go test -p 1 ./internal/handler/ -run "Pipeline|PassedTest|FailedTest|PublishWaits|CancelsOpenDraft|Unchanged|InTest"`. Esperado: FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** O mesmo comando, mais a suíte inteira de handler (`go test -p 1 ./...`). Esperado: PASS.
- [ ] **Step 5:** Commit `Etapa 2: sequência do teste, publicação e aviso`.

### Task 6: Rotas do site, tela de confirmação, faixa no app e avisos na Home

**Files:**
- Modify: `handler/drafts.go` (`POST /api/apps/{id}/drafts/{d}/test`; `getDraft` devolve a evidência)
- Create: `handler/test_plans.go`, `handler/test_plans_test.go`, `handler/notices.go`, `handler/notices_test.go`
- Modify: `handler/handler.go` (rotas), `handler/script_import.go` (aceita `in_test`)
- Create: `frontend/src/pages/TestPlanPage.tsx` + `.test.tsx`; `frontend/src/components/DraftBanner.tsx` + `.test.tsx`; `frontend/src/components/NoticesCard.tsx` + `.test.tsx`
- Modify: `frontend/src/App.tsx` (rota `apps/:id/plano-de-teste`), `pages/AppPage.tsx` (faixa), `pages/HomePage.tsx` (card), `lib/api.ts`

**Interfaces:**
- Produces (API):
  - `POST /api/apps/{id}/drafts/{d}/test` (`can.Edit`):
    - recusa um rascunho que não está em `rascunho` (409 `draft_closed`);
    - recusa kind que não é script com 409 `test_kind`: `Por enquanto o teste automático é só para scripts. Apps com tela ainda publicam pelo caminho de hoje.`;
    - recusa rascunho sem `teste` com 409 `test_plan_missing` e o texto da Global Constraints;
    - recusa um `teste` inválido (400 com os problemas);
    - recusa entradas inválidas (400 `invalid_inputs`);
    - recusa chaves faltando (409 `keys_missing`, como `launchRun`);
    - se tudo passa: `StartDraftTest`, um `advanceDraft` na hora, e devolve a visão do rascunho.
  - `GET /api/apps/{id}/drafts/{d}` passa a devolver:
    - o que já devolvia;
    - `stage`, `stage_label`, `checks`, `exit_code`;
    - `log`, só para `can.Edit`;
    - `files` (`path`, `size`, `saved`);
    - `confirm_url` (`/apps/{id}/plano-de-teste`) quando está em `esperando_confirmacao`.
  - `GET /api/apps/{id}/drafts/{d}/files?path=`: o conteúdo de um arquivo de evidência. Só o dono do app ou admin da empresa (a mesma regra de admin do `access`). Os outros recebem 403.
  - `GET /api/apps/{id}/test-plan` (`can.Edit`) devolve:
    - `plan{acessa, pula, keys, place}` do rascunho que espera confirmação; sem ele, do app;
    - `fingerprint`;
    - `confirmed{by, at, computer_id} | null` e `matches`;
    - `computers`: os computadores de **quem está vendo**;
    - `can_confirm`: dono ou admin.
  - `POST /api/apps/{id}/test-plan/confirm {fingerprint, computer_id}`:
    - dono ou admin;
    - recusa `viaMCP` com 403;
    - `fingerprint` diferente do atual → 409 `Este plano mudou. Recarregue a página.`;
    - o computador precisa ser de quem confirma;
    - depois de gravar, chama `advanceDraft` no rascunho que espera.
  - `PUT /api/apps/{id}/test-plan/computer {computer_id}`: as mesmas regras, sem nova confirmação.
  - `GET /api/notices`: os avisos dos apps em que a pessoa é dona ou admin, os 20 mais novos. `POST /api/notices/{id}/read`.
  - `POST /api/scripts/import` aceita `in_test: true`, que grava `in_test=true, restricted=true`.
- Produces (telas, preto e branco, DESIGN.md):
  - **`TestPlanPage`.** Título "Plano de teste". Mostra:
    - "O teste acessa", em lista;
    - "O teste pula";
    - "Chaves usadas", só os nomes;
    - "Onde roda: no PC", com seleção de computador.
    
    Em destaque, o texto exato da confirmação. Botão "Confirmar plano de teste". Já confirmado e igual: "Plano confirmado por X em DD/MM" e a troca do PC. Sem permissão: `Só o dono do app ou um administrador confirma este plano.`
  - **`DraftBanner`** no topo da `AppPage` quando há rascunho `testando` ou `aprovada`: "Versão nova em teste: <stage_label>". Em `esperando_confirmacao`, traz o link "Confirmar plano de teste". Os dados vêm de um campo novo `open_draft{id, stage, stage_label}` em `GET /api/apps/{id}` (adicionar em `apps.go`).
  - **`NoticesCard`** na Home: aparece só se há aviso não lido. Mostra a lista e "Marcar como lido".

- [ ] **Step 1: Testes que falham (Go).**
  - `TestStartTestNeedsPlan`, com o texto exato.
  - `TestStartTestRefusesWeb`.
  - `TestConfirmPlanSiteOnly`: com contexto MCP dá 403; `mcpRouteAllowed("POST", "/api/apps/1/test-plan/confirm")` é falso.
  - `TestConfirmWrongFingerprint`.
  - `TestConfirmOtherPersonsComputer`.
  - `TestPlanChangeNeedsNewConfirm`: um segundo rascunho com `acessa` diferente para em `esperando_confirmacao`. Um terceiro, só com `espera` diferente, vai direto para `esperando_pc`. Isso cobre o item 4 da Review Focus.
  - `TestEvidenceDownloadOwnerOnly`: o dono recebe o conteúdo, e um editor que não é dono nem admin recebe 403.
  - `TestDraftViewHidesSecret`: item 1 da Review Focus, pelo JSON do `GET`.
  - `TestNoticesScope`.
  - `TestImportInTest`: o app nasce restrito e `in_test`.
- [ ] **Step 2: Testes que falham (Vitest).**
  - `TestPlanPage` mostra o texto exato e chama `POST .../confirm` com a impressão e o computador escolhido. Sem `can_confirm`, mostra a mensagem e esconde o botão.
  - `DraftBanner` mostra o rótulo e o link só em `esperando_confirmacao`.
  - `NoticesCard` some sem avisos e marca como lido.
- [ ] **Step 3:** Rodar. Esperado: FAIL. Implementar. Rodar `go test -p 1 ./internal/handler/` e `bash -c 'cd "$(git rev-parse --show-toplevel)/frontend" && mise x -- npx vitest run --maxWorkers=2'`. Esperado: PASS.
- [ ] **Step 4:** Commit `Etapa 2: confirmação do plano, faixa de teste e avisos`.

### Task 7: MCP — `enviar_versao`, `ver_versao`, `baixar_arquivo_teste`

**Files:**
- Create: `backend/internal/handler/mcp_versao.go`, `backend/internal/handler/mcp_versao_test.go`
- Modify: `handler/mcp_tools.go` (as 3 ferramentas; as descrições de `registrar_app` e `preparar_envio` apontam para `enviar_versao`), `handler/mcp_server.go` (`mcpRoutes`), `handler/mcp_registrar.go` (script recusado), `handler/mcp_receita.md` e o texto de `formato_do_app` (seção `teste`, `ZIV_TESTE`, `ZIV_PASTA_TESTE`)

**Interfaces:**
- Consumes: as rotas da Task 6.
- Produces:
  - `mcpRoutes` + `POST /api/apps/{id}/drafts`, `POST /api/apps/{id}/drafts/{id}/test`, `GET /api/apps/{id}/drafts/{id}`, `GET /api/apps/{id}/drafts/{id}/files`. **Não** entram as rotas de `test-plan`, nem `POST .../drafts/{id}/publish`.
  - `enviarVersaoIn{Envio int64; AppID int64; Manifesto map[string]any; Nota string; ConfirmarSegredos bool}`:
    - **Sem `envio`:** a mesma resposta de `preparar_envio`. O texto termina com `Quando o curl mostrar "recebido": true, chame enviar_versao de novo com envio N`, com os mesmos app_id/manifesto.
    - **Com `envio`:**
      1. kind que não é script (do app ou do manifesto) → recusa: `Apps com tela ainda usam registrar_app e publicar_app_com_tela; enviar_versao chega para eles na próxima etapa.`;
      2. app novo sem `teste` no manifesto → recusa com o texto da Global Constraints;
      3. app novo → `POST /api/scripts/import` com `in_test: true`;
      4. depois, nos dois casos → `POST .../drafts` (a ficha vem de `fichaChanges`, sem `updated_at`) → `POST .../drafts/{d}/test` → `UseMCPUpload`;
      5. devolve `{app_id, versao: draft_id, situacao, etapa, confirmar}`.
      
      O texto diz o próximo passo:
      - em `esperando_confirmacao`: `Mande este link ao responsável pelo app para confirmar o plano de teste: <link>. Depois acompanhe com ver_versao.`;
      - nos outros casos: `Acompanhe com ver_versao (app_id X, versao D, esperar 30).`
  - `verVersaoIn{AppID, Versao int64; Esperar int}`: espera até 30 s por mudança de `status`/`stage`, como `toolVerExecucao`. No fim, devolve:
    - cada conferência (`✓`/`✗` + Detail);
    - o log mascarado;
    - a lista de arquivos;
    - o link de confirmação, quando há.
  - `baixarArquivoTesteIn{AppID, Versao int64; Caminho string}`: texto (UTF-8 válido, até 200 KB) vem no conteúdo. Binário ou maior vem como link para a página do app. A regra de dono/admin vem da rota.
  - `toolRegistrarApp`: quando o kind (novo ou atual) é script, recusa com `Scripts agora vão por enviar_versao, que testa antes de pôr no ar. Escreva a seção teste da receita (veja receita_app_ziv) e chame enviar_versao com o mesmo envio.`

- [ ] **Step 1: Testes que falham**, no estilo de `mcp_registrar_test.go`:
  - `TestEnviarVersaoCommandFirst`;
  - `TestEnviarVersaoNewAppInTest`: o app nasce `in_test`, e o rascunho fica em `esperando_confirmacao` com o link no texto;
  - `TestEnviarVersaoNeedsTeste`;
  - `TestEnviarVersaoRefusesWeb`;
  - `TestVerVersaoShowsChecks`: depois de um resultado de teste que passa, mostra `✓ arquivos *.pdf: 1 (mínimo 1)`, sem o valor da chave;
  - `TestBaixarArquivoTesteText`;
  - `TestRegistrarRefusesScript`;
  - `TestMCPCannotConfirmPlan`.
- [ ] **Step 2:** `go test -p 1 ./internal/handler/ -run "EnviarVersao|VerVersao|BaixarArquivo|RegistrarRefusesScript|MCPCannotConfirm|MCP"`. Esperado: FAIL e depois, implementado, PASS. Os testes antigos de `registrar_app` que usam script passam a esperar a recusa, ou trocam para `kind: web`; cada troca é anotada como Ruling no ledger.
- [ ] **Step 3:** Commit `Etapa 2: enviar_versao, ver_versao e baixar_arquivo_teste`.

### Task 8: Fluxo inteiro de verdade, docs e entrega

**Files:**
- Create: `e2e/teste-script.mjs` (visual) e `scripts/e2e-teste-script.sh` (fluxo real)
- Create: `docs/adr/044-teste-de-script-no-pc.md`
- Modify: `docs/TASKS.md`, `docs/PRD.md`

- [ ] **Step 1: Fluxo real.** Usar o Hub de teste em 8092 (**nunca 8080**) e o Vite em 5199. Compilar e rodar o Agente de verdade no Linux (`go build ./cmd/agent`), pareado com uma pessoa de teste. Pela API, o script:
  1. cria um script com `teste` e uma chave no Cofre;
  2. cria o rascunho e inicia o teste;
  3. confirma o plano;
  4. espera.
  
  Esperado:
  - o Agente real roda com `ZIV_TESTE=1`;
  - a versão vai ao ar `tested=true`;
  - o aviso aparece;
  - o valor da chave não está em `test_log`.
  
  Depois, repetir com um script que sai com código 1. Esperado: `reprovada`, nada no ar.
- [ ] **Step 2: Visual.** O Playwright tira foto de `TestPlanPage` (antes e depois de confirmar), da faixa na `AppPage` e do card de avisos na Home. Conferir alinhamento, contraste e o texto exato da confirmação.
- [ ] **Step 3: Docs.**
  - ADR 044, cobrindo:
    - o trabalho `test` separado de `runs`;
    - o estado no banco movido pelo relógio;
    - `publishDraftBG` com o recorder;
    - a confirmação só pelo site;
    - o PC de teste ser do confirmador;
    - o envio em duas chamadas.
  - TASKS: Etapa 2 Done (os itens adiados da Etapa 1 que entraram aqui saem da lista). Etapa 3 Pending.
  - PRD.
- [ ] **Step 4:** Commit `Etapa 2: ADR 044 e docs`. Suíte inteira verde (Go `-p 1`, Vitest `--maxWorkers=2`).

---

## Decisões tomadas neste plano (calls do cofundador, para o Henry)

1. **`enviar_versao` em duas chamadas** (comando, depois `envio`). Não é "o Ziv começa sozinho quando o upload chega". Assim, recusas de senha e de ficha voltam para quem enviou do jeito de hoje. Custo: uma chamada a mais para o Claude.
2. **O PC de teste é sempre um computador de quem confirma.** O Ziv não sugere o PC do agendamento, que pode ser de outra pessoa: rodar código não publicado no PC de outro sem ele saber pesa mais. Custo: quem confirma precisa ter o Agente.
3. **Pelo MCP, script só vai por `enviar_versao`.** `registrar_app` recusa scripts, então o plano de teste fica obrigatório para scripts já nesta etapa. Apps com tela continuam no caminho de hoje até a Etapa 3. Custo: um Claude que estava no meio de um envio recebe uma recusa e precisa escrever a seção `teste`.
4. **App novo nasce restrito e "em teste".** Só o dono e os admins o veem, e ele não roda até o primeiro teste passar. Isso aproveita o "restrito" que já existe, em vez de criar uma regra de visibilidade nova. Custo: se o teste nunca passar, o app fica parado e escondido. Dá para apagar ou arquivar.
5. **O teste respeita a aprovação de segurança do app** (`executionBlock`). Uma versão publicada que precisa de aprovação vai ao ar bloqueada, como hoje. O aviso diz isso.
6. **Publicar sozinho usa `publishDraft` por um recorder HTTP**, sem reescrever `applySources`. Custo: um padrão estranho no código, explicado na ADR.
7. **Entram aqui quatro pontos adiados da Etapa 1:**
   - situação do rascunho conferida dentro da trava;
   - `/sources` e voltar versão cancelam rascunho aberto;
   - autor e canal vindos do rascunho;
   - rascunho igual ao código no ar ganha a marca "testada".
   
   Também entra a limpeza de rascunhos fechados com mais de 30 dias.
