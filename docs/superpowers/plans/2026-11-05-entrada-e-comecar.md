# Entrada por e-mail, Começar em 6 passos e Configurações só com status — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um cliente entra só com e-mail e senha, cai no Começar (lista de 6 passos que se marcam sozinhos) e nunca vê a empresa principal; as Configurações só mostram o status das conexões; o cabeçalho da página inicial fica maior.

**Architecture:**
- **Backend:** ganha três coisas.
  - Uma busca de login entre empresas, feita pelo `Portal` e exposta pela principal em `POST /api/entry/login`, no mesmo padrão do `registerCompany`.
  - Dois fatos novos no `/api/onboarding`.
  - Um código obrigatório no primeiro acesso da principal.
- **Frontend:**
  - Reescreve `CompanyEntryPage` (e-mail e senha), `SetupPage` (lista com um passo aberto) e `SettingsPage` (só status).
  - Move o "conectar" do Agente e do Claude para dentro do Começar.

**Tech Stack:** Go (net/http, pgx, bcrypt), React + TypeScript + Vite + Tailwind, Vitest, Go tests com o Postgres de teste (`testdb_test.go`).

**Spec:** `docs/superpowers/specs/2026-11-05-entrada-e-comecar-design.md`

## Global Constraints

- Textos da interface em português do Brasil, no tom das telas atuais.
- Login com falha sempre devolve 401 com "E-mail ou senha não conferem.", sem revelar se o e-mail existe.
- A principal (`slug` `principal`, schema `public`) nunca entra na busca de login por e-mail nem aparece em telas públicas.
- `/api/entry/*` já passa pelo limite de tentativas do `Secure` (`security_middleware.go:159`). A rota nova precisa ficar sob esse prefixo.
- O progresso do Começar vem só de fatos do banco, nunca de caixas marcadas à mão.
- Cabeçalho da página inicial: logo `h-12` (48px), barra de cerca de 84px, menu `text-base` (16px).
- Nunca subir um Ziv de teste na porta 8080 (o Agente real do Henry fala com ela).
- Publicar com `bash scripts/deploy-gcp.sh`.

## Review Focus

- **Mesmo e-mail em duas empresas com senhas diferentes:** só aparece a empresa cuja senha bateu. Teste na Task 2.
- **E-mail que existe só na principal:** 401, como se não existisse. Teste na Task 2.
- **Membro sem permissão de registrar:** não vê "Subir app" nem "Convidar", e a contagem é "N de 4". Teste na Task 5.
- **Código do Claude Code gerado e nunca usado:** o passo 2 continua aberto. Teste na Task 1.
- **Logo maior no celular (390px):** o logo e os dois botões cabem numa linha. Conferir no print da Task 7.

---

### Task 1: Fatos novos no onboarding

**Files:**
- Modify: `backend/internal/database/sqlc/onboarding.go`
- Modify: `backend/internal/handler/onboarding.go`
- Modify: `frontend/src/lib/api.ts` (tipo `Onboarding`)
- Test: `backend/internal/handler/script_features_test.go` (ao lado do teste de onboarding existente)

**Interfaces:**
- Produces: `OnboardingProgress` ganha `ClaudeConnected, HasArtifacts bool`. O JSON de `GET /api/onboarding` ganha `"claude_connected"` e `"has_artifacts"`. O tipo TS `Onboarding` ganha `claude_connected: boolean; has_artifacts: boolean`.

- [ ] **Step 1: Write the failing test** `TestOnboardingClaudeAndArtifacts`.
  - Pessoa nova: `claude_connected=false`, `has_artifacts=false`.
  - Depois de `POST /api/mcp/codes`: ainda `false`.
  - Depois de marcar `last_used_at=now()` naquela conexão (UPDATE direto no teste): `true`.
  - Depois de revogar a conexão: `false`.
  - Depois de publicar um artifact (o helper que `artifacts_test.go` já usa): `has_artifacts=true`.
- [ ] **Step 2:** `cd backend && mise x -- go test ./internal/handler -run TestOnboardingClaudeAndArtifacts`. Expected: FAIL.
- [ ] **Step 3:** No SELECT de `OnboardingProgress`, acrescente:
  - `EXISTS(SELECT 1 FROM mcp_connections WHERE person_id=$1 AND revoked_at IS NULL AND last_used_at IS NOT NULL)`
  - `EXISTS(SELECT 1 FROM artifacts)`

  Exponha os dois campos no handler e no tipo TS.
- [ ] **Step 4:** Rode o mesmo teste e o pacote inteiro. Expected: PASS.
- [ ] **Step 5:** Commit: `Onboarding: Claude conectado e artifact publicado`.

### Task 2: Login só com e-mail e senha (backend)

**Files:**
- Modify: `backend/internal/handler/security.go` (`WorkspaceOptions`)
- Modify: `backend/internal/handler/entry.go` (rota e handler)
- Modify: `backend/internal/workspace/portal.go` (implementação)
- Test: `backend/internal/workspace/portal_test.go`

**Interfaces:**
- Produces:
  - `type LoginMatch struct { Slug, Name, URL string }` em `handler`.
  - `WorkspaceOptions.FindLogins func(ctx context.Context, email, password string) ([]LoginMatch, error)`.
  - Rota `POST /api/entry/login`, body `{email, password}`:
    - 200 com `{"companies":[{"slug","name","url"}]}`, ordenado por nome.
    - 401 com "E-mail ou senha não conferem." quando a lista é vazia.
    - 403 fora da principal, igual ao `registerCompany`.
  - A rota **não abre sessão**: o frontend chama `api.loginAt(basePath, …)` na empresa escolhida.

- [ ] **Step 1: Write the failing test** `TestPortalFindLogins`. Monte três empresas:
  - A com `ana@x.com` e senha `senha-a-123`;
  - B com `ana@x.com` e senha `senha-b-123`;
  - C com `ana@x.com` e senha `senha-a-123`.

  A principal tem `chefe@x.com`. Asserções:
  - `senha-a-123` → A e C, nessa ordem de nome, com URL `/w/<slug>/`.
  - `senha-b-123` → só B.
  - Senha errada → 401.
  - `chefe@x.com` com a senha certa → 401.
  - `POST /w/<a>/api/entry/login` → 403.
- [ ] **Step 2:** `cd backend && mise x -- go test ./internal/workspace -run TestPortalFindLogins`. Expected: FAIL.
- [ ] **Step 3:** Implemente `(p *Portal) findLogins` e passe-o em `attach` dentro de `WorkspaceOptions`.
  - Percorra `p.entries`, menos `principal`, e carregue antes do registro as empresas `ready` que ainda não estão em memória (use `p.load`).
  - Em cada uma, faça `sqlc.New(e.pool).GetPersonByEmail`. Se a pessoa existe e tem hash, faça `bcrypt.CompareHashAndPassword`.
  - Se nenhum e-mail foi encontrado, rode um `bcrypt.CompareHashAndPassword` contra um hash fixo de custo 12, para o tempo ficar parecido.
  - O handler normaliza o e-mail como `credentials.normalize()`.
- [ ] **Step 4:** Rode o teste e `go test ./...`. Expected: PASS.
- [ ] **Step 5:** Commit: `Entrar só com e-mail e senha: busca entre empresas`.

### Task 3: Código de primeiro acesso da principal

**Files:**
- Modify: `backend/internal/config/config.go` (`SetupCode string // SETUP_CODE`)
- Modify: `backend/internal/handler/handler.go` (`setup`)
- Modify: `frontend/src/lib/api.ts` (`setup` aceita `setup_code?: string`)
- Modify: `frontend/src/pages/LoginPage.tsx` (campo "Código de instalação" quando `needsSetup`)
- Modify: `.env.example` (documentar `SETUP_CODE`)
- Test: `backend/internal/handler/auth_test.go`, `frontend/src/pages/LoginPage.test.tsx`

**Interfaces:**
- Produces: `POST /api/auth/setup` aceita `setup_code`.
  - Com `cfg.SetupCode` definido, o código precisa bater; a comparação usa `subtle.ConstantTimeCompare`. Se não bater: 403 com "Código de instalação inválido.".
  - Sem `SetupCode` e sem `DevMode`: 403 com "Defina SETUP_CODE no servidor para o primeiro acesso.".
  - Com `DevMode`: o código não é pedido.

- [ ] **Step 1: Write the failing tests:**
  - `TestSetupRequiresCode`: sem código → 403; código errado → 403; código certo → 201.
  - `TestSetupWithoutCodeConfiguredRefused`.

  Ajuste o helper de teste que chama setup para mandar o código configurado, para os testes atuais seguirem passando.
- [ ] **Step 2:** Rode os dois testes. Expected: FAIL.
- [ ] **Step 3:** Implemente no `setup`, antes de contar pessoas.
  - Frontend: o campo só aparece com `needsSetup`. O texto de ajuda é "Está no arquivo de configuração do servidor (SETUP_CODE)."
- [ ] **Step 4:** Rode `go test ./...` e `cd frontend && mise x -- npx vitest run src/pages/LoginPage.test.tsx`. Expected: PASS.
- [ ] **Step 5:** Commit: `Primeiro acesso da principal pede código de instalação`.

### Task 4: Telas de entrada (Entrar e Cadastre-se)

**Files:**
- Modify: `frontend/src/pages/CompanyEntryPage.tsx`
- Modify: `frontend/src/pages/CompanySignupPage.tsx:69`
- Modify: `frontend/src/lib/api.ts`
- Create: `frontend/src/lib/landing.ts`
- Test: `frontend/src/pages/CompanyEntryPage.test.tsx`, `frontend/src/pages/CompanySignupPage.test.tsx`

**Interfaces:**
- Consumes: `POST /api/entry/login` (Task 2) e os campos de onboarding (Task 1).
- Produces:
  - `api.findLogins(b: {email: string; password: string}) => Promise<{companies: {slug: string; name: string; url: string}[]}>`.
  - `api.onboardingAt(basePath: string) => Promise<Onboarding>`.
  - `landingAfterLogin(basePath: string, person: Person): Promise<string>` em `lib/landing.ts`: devolve `${basePath}/comecar` se faltar algum passo visível para a pessoa (usa `visibleSteps` da Task 5) e `${basePath}/` se não faltar. Se a busca do onboarding falhar, devolve `${basePath}/comecar`.

- [ ] **Step 1: Write the failing tests** em `CompanyEntryPage.test.tsx`:
  - O formulário tem "E-mail" e "Senha" e não tem "Endereço da empresa".
  - Uma empresa → chama `loginAt("/w/a", …)` e `window.location.assign` com o destino de `landingAfterLogin`.
  - Duas → mostra "Em qual empresa?" com os dois nomes; clicar num deles faz login nele.
  - 401 → mostra "E-mail ou senha não conferem.".
  - A página não contém "espaço principal".

  Em `CompanySignupPage.test.tsx`: depois de criar, `assign` vai para `/w/<slug>/comecar`.
- [ ] **Step 2:** `cd frontend && mise x -- npx vitest run src/pages/CompanyEntryPage.test.tsx src/pages/CompanySignupPage.test.tsx`. Expected: FAIL.
- [ ] **Step 3:** Implemente.
  - O título continua "Entre no espaço da sua equipe".
  - Some o parágrafo do endereço e o link da principal.
  - Fica "Ainda não tem um espaço? Cadastre-se".
- [ ] **Step 4:** Rode os testes. Expected: PASS.
- [ ] **Step 5:** Commit: `Entrar só com e-mail e senha; cadastro leva ao Começar`.

### Task 5: Começar em lista com 6 passos

**Files:**
- Create: `frontend/src/lib/steps.ts`
- Create: `frontend/src/components/ClaudeConnect.tsx` (conector e "Gerar código", tirados de `ClaudeConnections.tsx`)
- Modify: `frontend/src/pages/SetupPage.tsx` (reescrita)
- Modify: `frontend/src/App.tsx` (`instalar-agente` → `<Navigate to="/comecar#computador" replace />`)
- Delete: `frontend/src/pages/InstallAgentPage.tsx` (o conteúdo vai para o passo 1)
- Modify: `frontend/src/components/Layout.tsx:29` (o item "Começar" fica escondido na principal: `show` usa `workspaceBasePath() !== ""`)
- Modify: `frontend/src/pages/LoginPage.tsx` (depois do login numa empresa, destino = `landingAfterLogin`, a menos que exista um `from` explícito)
- Test: `frontend/src/pages/SetupPage.test.tsx`, `frontend/src/lib/steps.test.ts`

**Interfaces:**
- Consumes: `Onboarding` (Task 1) e `canRegister(person)` (já existe).
- Produces:
  - `type StepId = "computador" | "claude" | "app" | "rodar" | "artifact" | "equipe"`.
  - `visibleSteps(o: Onboarding, person: Person): {id: StepId; done: boolean}[]`: a ordem é a do spec; `app` só aparece com `canRegister(person)` e `equipe` só com `o.can_manage`. Mapeamento de `done`:
    - `computer_connected` → computador
    - `claude_connected` → claude
    - `has_apps` → app
    - `has_successful_run` → rodar
    - `has_artifacts` → artifact
    - `has_team` → equipe

- [ ] **Step 1: Write the failing tests.**
  - `steps.test.ts`: um admin vê 6 passos; um membro sem `canRegister` nem `can_manage` vê `["computador","claude","rodar","artifact"]`.
  - `SetupPage.test.tsx`:
    - Com o computador feito, o passo "Conectar o Claude" é o único expandido (`aria-expanded="true"`) e o computador mostra "✓" e "Agente ligado".
    - O topo mostra "1 de 6 feitos".
    - Clicar em "Rodar o app" expande ele.
    - O hash `#computador` abre o passo 1.
    - Com tudo feito, aparece "Tudo pronto".
- [ ] **Step 2:** `cd frontend && mise x -- npx vitest run src/lib/steps.test.ts src/pages/SetupPage.test.tsx`. Expected: FAIL.
- [ ] **Step 3:** Implemente a lista. Cada passo é um `<li>` com um botão de cabeçalho (`aria-expanded`) e um corpo:
  - **computador:** `AgentDownloadButton` + os passos do Windows: "Mais informações" → "Executar assim mesmo" → aprovar a conexão.
  - **claude:** frase de uso + `<ClaudeConnect />` + "Fica ✓ quando o Claude falar com o Ziv pela primeira vez."
  - **app:** o que conta como app (script, app web, programa) + link para `/registrar`.
  - **rodar:** link para `/apps`.
  - **artifact:** "Peça ao Claude: 'publica isso no Ziv'" + alternativa `/artifacts/novo`.
  - **equipe:** link para `/equipe?aba=pessoas`.

  Recarregue o onboarding a cada 5 s, como o `ComputerSection` faz, para os ✓ aparecerem sozinhos.
- [ ] **Step 4:** Rode os testes e a suíte toda do frontend. Expected: PASS. Atualize testes que apontavam para `/instalar-agente` ou `/configuracoes#claude`.
- [ ] **Step 5:** Commit: `Começar em lista com 6 passos`.

### Task 6: Configurações só com status

**Files:**
- Modify: `frontend/src/pages/SettingsPage.tsx`
- Modify: `frontend/src/pages/ComputerSection.tsx` (sem download nem explicação)
- Modify: `frontend/src/components/ClaudeConnections.tsx` (só a lista de conexões com Desconectar; o resto foi para `ClaudeConnect`)
- Test: `frontend/src/pages/SettingsPage.test.tsx`, `ComputerSection.test.tsx`, `ClaudeConnections.test.tsx`

**Interfaces:**
- Consumes: `ClaudeConnect` (Task 5) não é usado aqui.

- [ ] **Step 1: Write the failing tests:**
  - `SettingsPage` não tem botão "Baixar" nem "Gerar código" nem o endereço do conector.
  - Um computador conectado mostra o nome, "Agente ligado" e "Desconectar".
  - Sem computador: "Nenhum computador conectado" com link "Conectar no Começar" para `/comecar#computador`.
  - Sem conexão do Claude: link para `/comecar#claude`.
  - "Aparência" continua.
- [ ] **Step 2:** Rode os três arquivos de teste. Expected: FAIL.
- [ ] **Step 3:** Implemente.
- [ ] **Step 4:** Rode a suíte do frontend. Expected: PASS.
- [ ] **Step 5:** Commit: `Configurações só com o status das conexões`.

### Task 7: Cabeçalho maior, conferência visual, docs e publicação

**Files:**
- Modify: `frontend/src/pages/LandingPage.tsx:61-73`
- Modify: `docs/TASKS.md`, `docs/PRD.md` (se cita login por endereço), `docs/INFRASTRUCTURE.md` (`SETUP_CODE`)
- Create: `docs/adr/023-entrada-por-email-e-comecar.md`
- Test: `frontend/src/pages/LandingPage.test.tsx` (deve continuar passando)

- [ ] **Step 1:** No cabeçalho:
  - barra `h-16` → `h-[84px]`;
  - `Wordmark` `h-8` → `h-12`;
  - `nav` `text-sm` → `text-base`;
  - botões Entrar/Cadastre-se `size="lg"`.
- [ ] **Step 2:** `cd frontend && mise x -- npx vitest run` e `cd backend && mise x -- go test ./...`. Expected: tudo PASS.
- [ ] **Step 3:** Suba o Ziv local numa porta que não seja 8080 (ex.: `PORT=8090`).
  - Tire prints com o Playwright de `/boas-vindas` (1280px e 390px), `/acesso`, `/w/<empresa>/comecar` e `/w/<empresa>/configuracoes`.
  - Confira: alinhamento, o logo cabendo no celular e só um passo aberto.
- [ ] **Step 4:** Escreva o ADR 023 com as decisões do spec, incluindo as "escolhas minhas", e atualize TASKS e INFRASTRUCTURE.
- [ ] **Step 5:** Commit e push: `Cabeçalho maior; docs da entrada e do Começar`.
- [ ] **Step 6:** Defina `SETUP_CODE` em `~/ziv/ziv.env` no servidor, com um valor aleatório de `openssl rand -hex 16`. A principal de produção já tem dono, então ele só vale para um servidor novo.
  - Rode `bash scripts/deploy-gcp.sh`. Expected: `active` e `site: 200`.
- [ ] **Step 7:** Teste com as mãos do Henry, um passo por vez: criar empresa → cai no Começar → sair → Entrar só com e-mail e senha → Configurações.
