# Agendar apps — Entrega 1 (agenda + rodar pelo Agente) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** qualquer pessoa que pode rodar um Script cria agendamentos (opções prontas, mínimo 5 min). O Ziv dispara cada agendamento sozinho pelo Agente no computador escolhido. Toda execução, perdida ou pulada, fica visível.

**Architecture:**
- Uma tabela `schedules` por empresa, ou seja, por esquema do portal.
- Um pacote Go puro `internal/schedule` calcula as datas e o texto por extenso.
- A lógica do Rodar sai de `startRun` para `launchRun`, que o HTTP e o relógio usam.
- Cada `Handler` (um por empresa) liga o seu próprio relógio de 30 s quando o portal o anexa.
- No front: um diálogo "Agendar", que reaproveita os campos do Rodar, e uma seção "Agendamentos" na página do app.

**Tech Stack:** Go 1.x, sqlc, pgx/v5, Postgres 17; React + Vite + Tailwind, Vitest. Comandos via `mise x --`.

**Spec:** `docs/superpowers/specs/2026-11-06-agendar-apps-design.md` (leia a Peça 1 inteira, incluindo a tabela "O que acontece em cada situação").

## Global Constraints

- Só apps `kind: script` com `manifest.ScriptRuntime(entrypoint) != ""` podem ser agendados.
- Nenhum campo pode ter `source: window`. Se tiver, recuse com a mensagem "Este app pergunta coisas depois de abrir, então não dá para agendar."
- **Tipos de frequência:**
  - `minutes`: N ≥ 5 e ≤ 1440.
  - `hours`: N de 1 a 24.
  - `daily`: HH:MM.
  - `weekdays`: um ou mais dias de 0 a 6, onde 0 é domingo, mais HH:MM.
  - `monthly`: dia de 1 a 28, mais HH:MM.
- O fuso é sempre `America/Sao_Paulo`.
- Arquivos guardados no agendamento têm o mesmo limite de 10 MB do Rodar (`MaxBytesReader` de 15 MB no corpo).
- **Tolerância de atraso:** 1 hora. Se o atraso for de até 1 h, o horário roda uma vez. Se for maior, o agendamento pula para o próximo horário futuro e registra que perdeu.
- **Falhas seguidas antes de pausar:** 5. Contam como falha:
  - execução terminada em `error`;
  - `interrupted` sem ter sido parada à mão;
  - `missed`.

  Não contam `stopped`, nem pulos ou esperas.
- Execução nova: `runs.origin = 'schedule'`. Status novo: `missed`, mostrado como "Perdida".
- **Textos fixos:**
  - "Pulada: a anterior ainda estava rodando"
  - "Esperando aprovação do app"
  - "A empresa pausou todos os apps no Admin."
  - "O computador {nome} estava desligado"
  - "Atualize o Agente em Configurações › Meu computador"
  - "Falhou 5 vezes seguidas"
  - "Você não tem mais acesso a este app"
  - "Este app foi suspenso pela empresa."
  - "O app mudou e os campos guardados não servem mais. Edite o agendamento."
- **Permissões:**
  - Criar: `can.View` sobre o app.
  - Ver, editar e reativar: o próprio dono do agendamento.
  - Ver todos, pausar e apagar qualquer um: `can.Access` ou administrador.
  - `inputs`, `files` e a pasta de quem não é o dono **nunca** saem na API.
- Toda escrita grava `security_audit`. As do relógio usam `actor_id` NULL e `route` igual a `schedule`.
- Visual: siga `DESIGN.md` (preto e branco, ADR 020). A raiz da fonte é 14px, então use `text-[12px]` e não `text-xs` para textos pequenos.
- Tudo em português do Brasil, no tom do Ziv.

## Review Focus

1. **Relógio rodando em mais de uma cópia, ou reinício no meio do disparo.** Cada horário dispara exatamente uma vez. A trava é `UPDATE … SET next_run_at = $novo WHERE id = $1 AND next_run_at = $antigo`. Teste: `TestSchedulerClaimsOnce` (Task 5).
2. **Agendamento de 5 min com o PC desligado o dia inteiro.** Devem aparecer 5 Perdidas e depois a pausa, não 288 linhas. A Home mostra um aviso de "agendamento pausado", não 5 avisos de execução. Testes: `TestSchedulerPausesAfterFiveFailures` (Task 5) e `TestHomeSkipsScheduledMissed` (Task 5).
3. **O app muda depois de agendado.** Pode ganhar um campo obrigatório novo ou perder um campo guardado. O disparo valida com `ValidateRunInputs` e pausa com o texto fixo, em vez de mandar entradas erradas ao Agente. Teste: `TestSchedulerPausesWhenInputsInvalid` (Task 5).
4. **Virada de mês e de horário.** Exemplos: `monthly` dia 28 em fevereiro, `weekdays` no sábado às 23:59, `hours` 24. Teste de tabela em `TestNext` (Task 2).
5. **Outra pessoa abre a lista.** Ela não pode ver `inputs`, `files` nem `output_folder` dos agendamentos alheios. Teste: `TestListSchedulesHidesOthersValues` (Task 4).

---

### Task 1: Banco — tabela `schedules`, status `missed`, origem `schedule`

**Files:**
- Create: `backend/internal/database/migrations/020_schedules.sql`
- Create: `backend/internal/database/queries/schedules.sql`
- Modify: `backend/internal/database/queries/runs.sql` (CreateRun ganha `schedule_id`)
- Generated: `backend/internal/database/sqlc/*` (via `mise x -- sqlc generate`; atenção: `people.sql.go` foi editado à mão — confira o diff dele e restaure as edições manuais se o generate as apagar)

**Interfaces:**
- Produces (SQL): a tabela `schedules`, com as colunas:
  - `id BIGSERIAL PK`
  - `app_id BIGINT NOT NULL REFERENCES apps ON DELETE CASCADE`
  - `person_id BIGINT NOT NULL REFERENCES people ON DELETE CASCADE`
  - `computer_id BIGINT NOT NULL REFERENCES computers ON DELETE CASCADE`
  - `name TEXT NOT NULL DEFAULT ''`
  - `frequency JSONB NOT NULL`
  - `inputs JSONB NOT NULL DEFAULT '{}'`
  - `files JSONB NOT NULL DEFAULT '{}'`
  - `output_folder TEXT NOT NULL`
  - `active BOOLEAN NOT NULL DEFAULT true`
  - `paused_reason TEXT NOT NULL DEFAULT ''`
  - `situation TEXT NOT NULL DEFAULT ''`
  - `failures INT NOT NULL DEFAULT 0`
  - `email JSONB NOT NULL DEFAULT '{}'`
  - `next_run_at TIMESTAMPTZ NOT NULL`
  - `last_run_at TIMESTAMPTZ`
  - `last_run_id BIGINT REFERENCES runs ON DELETE SET NULL`
  - `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
  - `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

  Índices: `(active, next_run_at)` e `(app_id)`.

  A tabela `runs` também muda:
  - ganha `schedule_id BIGINT REFERENCES schedules ON DELETE SET NULL`;
  - o check de status ganha `missed`, seguindo o padrão de `011_stop.sql`;
  - o check de origem ganha `schedule`. Derrube e recrie o check criado em `019_mcp.sql`, procurando o nome real com `\d runs`.
- Produces (sqlc): `CreateSchedule`, `UpdateSchedule`, `GetSchedule`, `ListAppSchedules(app_id)`, `ListPersonSchedules(person_id)`, `DeleteSchedule`, `SetScheduleActive(id, active, paused_reason)`, `DueSchedules(now) → []Schedule` (`active AND next_run_at <= now`, limite 50), `ClaimSchedule(id, old_next, new_next) → rows affected`, `RecordScheduleOutcome(id, situation, failures, last_run_id, last_run_at)`, `CountPausedSchedulesForPerson`. `CreateRun` recebe `ScheduleID pgtype.Int8`.

- [ ] **Step 1:** Escreva a migração e as consultas.
- [ ] **Step 2:** Rode `cd backend && mise x -- sqlc generate && git diff --stat internal/database/sqlc`. Esperado: muda `schedules.sql.go`, `models.go` e `runs.sql.go`. Se `people.sql.go` mudar, restaure as partes feitas à mão.
- [ ] **Step 3:** Rode `cd backend && mise x -- go build ./... && mise x -- go test ./internal/database/... ./internal/handler/ -run 'Migration|TestUsesSeparateDatabase'`. Esperado: PASS, com as migrações aplicando num esquema limpo.
- [ ] **Step 4:** Ajuste as chamadas existentes de `CreateRun` para passar `ScheduleID: pgtype.Int8{}`. Depois `mise x -- go build ./...`.
- [ ] **Step 5:** Commit `Agendar: tabela de agendamentos e status Perdida`.

### Task 2: Pacote `internal/schedule` — frequência, próximo horário, texto

**Files:**
- Create: `backend/internal/schedule/schedule.go`
- Test: `backend/internal/schedule/schedule_test.go`

**Interfaces:**
- Produces:
  ```go
  type Frequency struct {
      Kind    string `json:"kind"`              // minutes|hours|daily|weekdays|monthly
      Every   int    `json:"every,omitempty"`   // minutes, hours
      At      string `json:"at,omitempty"`      // "HH:MM" for daily, weekdays, monthly
      Days    []int  `json:"days,omitempty"`    // weekdays: 0=domingo..6
      Day     int    `json:"day,omitempty"`     // monthly: 1..28
  }
  var Location *time.Location // America/Sao_Paulo, loaded once (embed tzdata via import _ "time/tzdata")
  func (f Frequency) Validate() error            // mensagens em português
  func (f Frequency) Next(after time.Time) time.Time // primeiro horário estritamente depois de after
  func (f Frequency) Upcoming(after time.Time, n int) []time.Time
  func (f Frequency) Describe() string           // "A cada 5 minutos", "Todo dia às 7h", "Seg, qua e sex às 7h30", "Todo mês no dia 5 às 9h"
  const Grace = time.Hour
  ```
  Para `minutes` e `hours`, conte a partir da meia-noite local e alinhe nos múltiplos. Exemplo: a cada 15 min dá 10:00, 10:15, e assim por diante. Assim o horário é previsível e não depende de quando o agendamento foi criado.

- [ ] **Step 1: Write the failing test.** `TestNext` deve ser uma tabela com, no mínimo, estes casos, todos no horário de São Paulo:
  - minutes 5, depois de 10:02 → 10:05
  - minutes 60, depois de 23:30 → 00:00 do dia seguinte
  - hours 24, depois de 10:00 → 00:00 do dia seguinte
  - daily "07:00", depois de 07:00 → 07:00 do dia seguinte (estritamente depois)
  - weekdays [1,3,5] "07:30", depois de sexta 08:00 → segunda 07:30
  - weekdays [6] "23:59", depois de sábado 23:59 → sábado seguinte 23:59
  - monthly 28 "09:00", depois de 28/01 10:00 → 28/02 09:00
  - monthly 1 "00:00", depois de 31/12 23:00 → 01/01 00:00

  `TestValidate` deve recusar:
  - minutes 4;
  - hours 25;
  - daily "7h";
  - weekdays sem dias;
  - monthly 29;
  - kind desconhecido.

  `TestDescribe` deve conferir os 4 textos de exemplo acima.
- [ ] **Step 2:** Rode `cd backend && mise x -- go test ./internal/schedule/`. Esperado: FAIL, com o pacote ainda inexistente.
- [ ] **Step 3:** Implemente.
- [ ] **Step 4:** Rode de novo. Esperado: PASS.
- [ ] **Step 5:** Commit `Agendar: cálculo de horários`.

### Task 3: Separar a lógica do Rodar (`launchRun`)

**Files:**
- Modify: `backend/internal/handler/runs.go` (startRun vira um invólucro fino)
- Create: `backend/internal/handler/launch.go`
- Test: `backend/internal/handler/runs_test.go` (os testes existentes devem continuar passando sem mudança) + `launch_test.go`

**Interfaces:**
- Produces:
  ```go
  type launchRequest struct {
      App          sqlc.App
      Person       sqlc.Person
      Computer     sqlc.Computer          // já escolhido pelo chamador
      Manifest     manifest.Manifest
      OutputFolder string
      Inputs       map[string][]string
      Files        map[string]manifest.InputFile
      Origin       string                 // "manual" | "mcp" | "schedule"
      ScheduleID   pgtype.Int8
      SourcesFrom  *http.Request          // nil fora de HTTP; ver importedSources
  }
  type launchError struct{ Status int; Msg string; Code string } // Code: "blocked","folder_busy","agent_outdated","code_changed","invalid_inputs"
  func (h *Handler) launchRun(ctx context.Context, req launchRequest) (sqlc.Run, *launchError, error)
  ```
  `launchRun` faz, na ordem de hoje:
  1. `executionBlock` (`Code` "blocked");
  2. `ValidateRunInputs` ("invalid_inputs");
  3. versão do Agente ("agent_outdated");
  4. `folderInUse` ("folder_busy");
  5. trava do app;
  6. `checkVersionLocked`;
  7. `executionBlock` de novo;
  8. comparação do pacote salvo ("code_changed");
  9. `CreateRun`;
  10. `enqueue`.

  `startRun` continua cuidando do JSON do corpo, de escolher o computador (`ConnectedComputerForPerson`), da pasta lembrada e da resposta. Se `importedSources` depender do `*http.Request`, troque para receber `ctx`. Confira a assinatura e mude todos os chamadores.

- [ ] **Step 1:** Escreva `TestLaunchRunBlockedWhenAppSuspended` e `TestLaunchRunSetsScheduleOrigin` em `launch_test.go`. Este último confere `run.Origin == "schedule"` e `run.ScheduleID`. Use os ajudantes de `runs_test.go` para app e computador.
- [ ] **Step 2:** Rode `cd backend && mise x -- go test ./internal/handler/ -run 'LaunchRun'`. Esperado: FAIL, por não compilar.
- [ ] **Step 3:** Extraia a lógica.
- [ ] **Step 4:** Rode `cd backend && mise x -- go test ./internal/handler/ ./internal/workspace/`. Esperado: PASS em tudo, incluindo os testes antigos de runs e de MCP.
- [ ] **Step 5:** Commit `Rodar: lógica separada para o agendamento usar`.

### Task 4: API de agendamentos

**Files:**
- Create: `backend/internal/handler/schedules.go`
- Modify: `backend/internal/handler/handler.go` (rotas)
- Test: `backend/internal/handler/schedules_test.go`

**Interfaces:**
- Rotas:
  - `GET /api/apps/{id}/schedules`: os do próprio usuário, ou todos quando `can.Access`/administrador.
  - `POST /api/apps/{id}/schedules`
  - `GET /api/schedules/{sid}`: só o dono.
  - `PUT /api/schedules/{sid}`: só o dono.
  - `POST /api/schedules/{sid}/pause`: dono ou quem cuida do app.
  - `POST /api/schedules/{sid}/resume`: só o dono.
  - `DELETE /api/schedules/{sid}`: dono ou quem cuida do app.
  - `GET /api/me/schedules`
- Corpo de criação e edição:
  ```json
  { "name": "", "frequency": {Frequency}, "computer_id": 3, "output_folder": "C:\\...", "inputs": {...}, "files": {...} }
  ```
  - Valide com `Frequency.Validate` e com `canRunKind` mais o teste de `window`.
  - Valide `ValidateRunInputs`, o padrão `windowsAbs` da pasta e se `computer_id` é um computador não revogado da própria pessoa.
  - O computador não precisa estar ligado na hora de salvar.
  - Na criação e na reativação, calcule `next_run_at = Next(now)`. A reativação também zera `failures`, `paused_reason` e `situation`.
- Produces (JSON):
  ```go
  type scheduleJSON struct {
      ID int64; AppID int64; AppName string; PersonID int64; PersonName string; Mine bool
      Name string; Frequency schedule.Frequency; Describe string
      ComputerID int64; ComputerName string
      Active bool; PausedReason string; Situation string
      NextRunAt *time.Time; LastRunAt *time.Time; LastRunID *int64; LastRunStatus string
      Upcoming []time.Time         // próximas 3
      // only when Mine:
      OutputFolder string `json:"output_folder,omitempty"`
      Inputs map[string][]string `json:"inputs,omitempty"`
      Files map[string]manifest.InputFile `json:"files,omitempty"` // conteúdo omitido: só nome
  }
  ```
  Nas respostas, `files` nunca leva `ContentBase64`, só o nome. Na edição, quando o corpo traz o arquivo só com o nome (sem conteúdo), mantenha o arquivo já guardado.
- Auditoria: as rotas passam pelo middleware de auditoria que já existe, igual às outras rotas que escrevem. Confira em `Secure` se basta registrar a rota.

- [ ] **Step 1: Write the failing tests.**
  - `TestCreateScheduleScriptOnly`: um app desktop `.exe` recebe 400.
  - `TestCreateScheduleRejectsWindowInput`: recebe 400 com o texto fixo.
  - `TestCreateScheduleRejectsFiveMinuteFloor`: `minutes` 4 recebe 400.
  - `TestCreateScheduleOtherPersonsComputer`: recebe 400.
  - `TestListSchedulesHidesOthersValues`: quem cuida do app vê o agendamento de B sem `inputs`, `files` e `output_folder`; uma pessoa comum só vê os seus.
  - `TestPauseByAppCarer`: dono ou reserva pausa o de outra pessoa e recebe 200; uma pessoa comum recebe 404.
  - `TestResumeResetsFailures`
  - `TestEditKeepsStoredFileWhenOnlyNameSent`
- [ ] **Step 2:** Rode `cd backend && mise x -- go test ./internal/handler/ -run 'Schedule'`. Esperado: FAIL.
- [ ] **Step 3:** Implemente.
- [ ] **Step 4:** Rode de novo. Esperado: PASS.
- [ ] **Step 5:** Commit `Agendar: API de agendamentos`.

### Task 5: O relógio

**Files:**
- Create: `backend/internal/handler/scheduler.go`
- Modify: `backend/internal/workspace/portal.go`
  - `attach` liga o relógio.
  - `Close` o desliga.
  - O `cmd` do servidor, se não usar o portal, liga o relógio do handler único (veja `backend/cmd/*/main.go`).
- Modify: `backend/internal/handler/agents.go` / `runs.go`. Quando uma execução com `schedule_id` termina em `onRunResult`, ou expira em `ExpireStaleRuns`, ela atualiza o agendamento:
  - `failures++` ou zera, conforme a regra;
  - a situação;
  - a pausa ao chegar em 5.
- Modify: `backend/internal/database/queries/home.sql`
  - `ListMyAttentionRuns` ignora `origin = 'schedule'`.
  - Uma consulta nova `ListMyPausedSchedules` alimenta um aviso `schedule_paused` na Home (`home.go`).
- Test: `backend/internal/handler/scheduler_test.go`, `home_test.go`

**Interfaces:**
- Produces:
  ```go
  func (h *Handler) StartScheduler(ctx context.Context)        // goroutine; tick 30s; para quando ctx acaba
  func (h *Handler) schedulerTick(ctx context.Context, now time.Time) // testável sem relógio real
  func (h *Handler) scheduleRunFinished(ctx context.Context, runID int64, status string) // chamado ao fim de uma execução
  ```
  `schedulerTick`:
  1. Rode `ExpireStaleRuns`. Se a execução expirada for de agendamento, use o texto "O computador {nome} estava desligado" e chame `scheduleRunFinished`.
  2. Rode `DueSchedules(now)`.
  3. Para cada agendamento, calcule `newNext = Next(now)`. Se `ClaimSchedule(id, old, newNext)` afetou 0 linhas, pule.
  4. Se `now - old > Grace`, registre a situação "Perdeu o horário de {hora}: o Ziv estava desligado". Não dispare e não conte falha.
  5. Recarregue o app, a pessoa e o acesso (`loadAccess` + `For`).
     - Sem `View`: pause com "Você não tem mais acesso a este app".
     - App suspenso: pause com o texto de suspensão.
  6. Se o agendamento tem uma execução anterior ainda `waiting` ou `running`, a situação fica "Pulada: a anterior ainda estava rodando". Não dispara.
  7. Se o computador não está conectado (`IsComputerConnected`), crie a execução direto com status `missed` e `error_message` "O computador {nome} estava desligado", e chame `scheduleRunFinished(missed)`.
  8. Chame `launchRun`. O resultado decide o que acontece:

     | Erro | Ação |
     |---|---|
     | `blocked` com aprovação ou Admin | situação com o motivo; sem falha e sem pausa |
     | `blocked` com suspensão | pausa |
     | `invalid_inputs` | pausa com o texto de "app mudou" |
     | `agent_outdated` | execução `missed` com o texto do Agente e falha |
     | `folder_busy` | situação "Pulada…" |
     | `code_changed` | situação com a mensagem; sem falha |
     | sucesso | `last_run_id` e `last_run_at` |

  Pausas e criação de `missed` gravam auditoria com `actor_id` NULL, `action` `schedule.paused` ou `schedule.missed` e `route` `schedule`.

- [ ] **Step 1: Write the failing tests.** Todos chamam `schedulerTick` com um `now` fixo.
  - `TestSchedulerFiresDueSchedule`: cria uma execução `origin=schedule`, um job para o computador e avança `next_run_at`.
  - `TestSchedulerClaimsOnce`: duas chamadas concorrentes de `schedulerTick` com o mesmo `now` criam 1 execução.
  - `TestSchedulerOfflineComputerCreatesMissed`
  - `TestSchedulerPausesAfterFiveFailures`: 5 ticks com o PC desligado deixam 5 `missed`, `active=false` e o motivo "Falhou 5 vezes seguidas".
  - `TestSchedulerSkipsWhilePreviousRunning`: sem execução nova e com a situação "Pulada…".
  - `TestSchedulerWaitsOnApproval`: continua ativo, sem execução e com a situação "Esperando aprovação do app".
  - `TestSchedulerPausesWhenAccessLost`
  - `TestSchedulerPausesWhenInputsInvalid`
  - `TestSchedulerLateBeyondGraceDoesNotFire`
  - `TestScheduledRunDoneResetsFailures`
  - `TestStoppedRunDoesNotCountAsFailure`
  - `TestHomeSkipsScheduledMissed`: em `home_test.go`, mostra o aviso `schedule_paused` e não os avisos de execução.
- [ ] **Step 2:** Rode `cd backend && mise x -- go test ./internal/handler/ -run 'Scheduler|Scheduled|StoppedRun|HomeSkips'`. Esperado: FAIL.
- [ ] **Step 3:** Implemente e ligue o relógio no portal.
- [ ] **Step 4:** Rode `cd backend && mise x -- go test ./...`. Esperado: PASS em tudo.
- [ ] **Step 5:** Commit `Agendar: relógio que dispara os agendamentos`.

### Task 6: Tela — Agendar, Agendamentos, selos e avisos

**Files:**
- Create: `frontend/src/components/ScriptFields.tsx`. Extraia `ScriptField` e o bloco de campos e pasta de `RunDialog.tsx`; o `RunDialog` passa a usar o componente novo, sem mudar o comportamento.
- Create: `frontend/src/components/ScheduleDialog.tsx`
- Create: `frontend/src/components/SchedulesSection.tsx`
- Create: `frontend/src/lib/schedules.ts` (texto e prévia locais só se o servidor não mandar; prefira `describe` e `upcoming` do servidor)
- Modify:
  - `frontend/src/lib/api.ts`: tipos `Schedule` e `Frequency`, chamadas das rotas da Task 4, `RunStatus` com `missed` e `origin` com `schedule`.
  - `frontend/src/lib/runs.ts`: `missed` com o rótulo "Perdida", tom `bad`.
  - `frontend/src/pages/AppPage.tsx`: botão "Agendar" ao lado de Rodar, só quando `m.kind === "script" && app.can_run`, e a seção "Agendamentos" logo abaixo de "Execuções".
  - `frontend/src/pages/RunPage.tsx` e `RunStrip.tsx`: selo "agendada" ao lado de `ByClaude`, no mesmo estilo.
  - `frontend/src/pages/HomePage.tsx`: aviso `schedule_paused`, com o texto "{app}: agendamento pausado — {motivo}", que leva à página do app.
- Test: `ScheduleDialog.test.tsx`, `SchedulesSection.test.tsx`, ajustes em `RunDialog.test.tsx`, `AppPage.test.tsx`, `HomePage.test.tsx`

**Interfaces:**
- Consumes: as rotas e o `scheduleJSON` da Task 4.
- `ScheduleDialog({ app, schedule?, onClose, onSaved })`:
  - Os campos do app vêm de `ScriptFields`.
  - A pasta onde salvar vem do mesmo seletor do Rodar.
  - "Quando rodar" traz:
    - um select de frequência com as opções "A cada … minutos", "A cada … horas", "Todo dia", "Em dias da semana" e "Todo mês";
    - os campos de número, horário e dias, conforme a frequência;
    - a prévia "Próximas: …", calculada localmente com a mesma regra da Task 2 em `lib/schedules.ts`, ou ao salvar.
  - O computador vem de um select com os computadores da pessoa. O padrão é o conectado.
  - O nome é opcional.
  - O botão é "Salvar agendamento".
- `SchedulesSection({ appId, canManageAll })`: a lista da spec, com estas regras:
  - Os botões Pausar, Editar e Apagar aparecem só quando cabem. Editar e Reativar são só do dono.
  - Um agendamento pausado mostra o motivo em destaque e o botão "Reativar".
  - Em lista vazia, a seção some.

- [ ] **Step 1: Write the failing tests.**
  - `ScheduleDialog`:
    - `minutes` com 4 mostra o erro "No mínimo a cada 5 minutos" e não envia;
    - "Em dias da semana" sem dias desabilita Salvar;
    - salvar envia `{frequency:{kind:"daily",at:"07:00"}, inputs, output_folder, computer_id}`.
  - `SchedulesSection`:
    - mostra "Todo dia às 7h" e a próxima execução;
    - quando pausado, mostra o motivo e "Reativar";
    - o agendamento de outra pessoa não tem Editar.
  - `AppPage`: o botão Agendar aparece num script e não aparece num app desktop.
  - `HomePage`: o aviso de agendamento pausado.
- [ ] **Step 2:** Rode `cd frontend && mise x -- npx vitest run src/components/ScheduleDialog.test.tsx src/components/SchedulesSection.test.tsx`. Esperado: FAIL.
- [ ] **Step 3:** Implemente.
- [ ] **Step 4:** Rode `cd frontend && mise x -- npx tsc -b && mise x -- npx vitest run`. Esperado: PASS em tudo.
- [ ] **Step 5:** Commit `Agendar: telas de agendar e lista de agendamentos`.

### Task 7: Prévia, revisão visual e docs

**Files:**
- Create: `docs/adr/024-agendamento.md`
- Modify: `docs/TASKS.md` (seção nova; a Peça 2 e a Peça 3 ficam Pending), `docs/PRD.md` (agendamento sai de "futuro"), `docs/INFRASTRUCTURE.md` (só se algo mudou)

- [ ] **Step 1:** Suba o Ziv local com `bash scripts/run-local.sh`. Nunca use a 8080: confira a porta no script e use outra livre se precisar. Suba o Vite na 5180.
- [ ] **Step 2:** Com Playwright, a partir de um script temporário em `e2e/`, tire capturas em desktop e celular de:
  - a página de um script com a seção Agendamentos;
  - o diálogo Agendar aberto em "Em dias da semana";
  - um agendamento pausado.

  Corrija o que estiver desalinhado.
- [ ] **Step 3:** Escreva o ADR 024: decisões da spec, a tabela de situações e os motivos (computador fixo, 5 falhas, 1 h de atraso).
- [ ] **Step 4:** Rode todos os testes: `cd backend && mise x -- go test ./...` e `cd frontend && mise x -- npx vitest run`. Esperado: PASS.
- [ ] **Step 5:** Commit `Agendar: ADR 024 e documentação`.
