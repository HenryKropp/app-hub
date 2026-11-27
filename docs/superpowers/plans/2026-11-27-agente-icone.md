# Agente visível (ícone e desinstalar): plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** o Agente do Windows mostra um ícone com o estado perto do relógio, aparece em Aplicativos instalados e sabe se desinstalar. Quando é desconectado no Ziv, deixa reconectar pelo ícone.

**Architecture:**
- O ciclo de conexão (parear → hello → loop) sai do `main` e vai para `internal/agent/session.go`, num `Session` testável. Ele publica o estado num `Status` observável.
- `cmd/agent/tray_windows.go` desenha o ícone e o menu a partir desse estado.
- `cmd/agent/uninstall_windows.go` faz a desinstalação.
- O servidor ganha `POST /api/agent/goodbye`, com que o computador desconecta a si mesmo.

**Tech Stack:** Go (Agente compilado com `CGO_ENABLED=0 GOOS=windows`), `fyne.io/systray` v1.12.2 (no Windows é Go puro), `golang.org/x/sys/windows` (registro e `MessageBoxW`), servidor Go com Postgres (sqlc).

**Spec:** `docs/superpowers/specs/2026-11-27-agente-icone-design.md`

## Global Constraints

- `agentproto.Version` passa a `"0.5.0"`.
- Todo texto visível em português, com estes textos exatos:
  - estado: `Ziv Agente 0.5.0 — Conectado (NOME)`, `Ziv Agente 0.5.0 — Sem conexão, tentando de novo sozinho`, `Ziv Agente 0.5.0 — Desconectado do Ziv`, `Ziv Agente 0.5.0 — Conectando...`;
  - menu: `Abrir o Ziv`, `Conectar de novo`, `Sair`, `Desinstalar...`.
- Confirmação de **Sair**: `Fechar o Ziv Agente?\n\nApps agendados neste computador não vão rodar até você entrar de novo no Windows.`
- Confirmação de **Desinstalar**: `Desinstalar o Ziv Agente deste computador?\n\nEle para de rodar, sai do início do Windows e este computador é desconectado do Ziv.`
- Segunda abertura do `.exe` com o Agente já rodando: caixa de mensagem `O Ziv Agente já está rodando. Veja o ícone perto do relógio.` (Mudança do desenho: o `fyne.io/systray` não mostra balão.)
- Nome na lista de Aplicativos instalados: `Ziv Agente`, editor `Ziv`. Chave `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\ZivAgente`, com `UninstallString` = `"<pasta>\AppHubAgente.exe" --desinstalar`.
- Pasta e nome do executável não mudam: `%LOCALAPPDATA%\AppHub\AppHubAgente.exe` (ADR 013/014).
- Nenhum nome proibido (ver regra de nomes da memória) em arquivos versionados.

## Review Focus

1. **PC desconectado e reiniciado.** O Agente sobe com o Windows sem chave. Ele **não** pode abrir o navegador sozinho a cada boot: fica cinza, "Desconectado do Ziv", até a pessoa clicar em Conectar de novo. Teste: `TestSessionStartsDisconnectedWhenMarked` (Tarefa 2).
2. **Ziv fora do ar no meio do uso.** O estado vira Offline e volta a Connected sozinho, sem nenhum clique. Teste: `TestSessionOfflineThenBack` (Tarefa 2).
3. **Desinstalar sem internet.** A desinstalação termina mesmo com o goodbye falhando. Teste: `TestUninstallStepsContinueWhenGoodbyeFails` (Tarefa 3).
4. **Conectar de novo clicado duas vezes, ou com o pareamento já aberto.** Só um pareamento roda por vez. Teste: `TestSessionReconnectIgnoredWhilePairing` (Tarefa 2).
5. **Goodbye com chave de outro computador ou já revogada.** Devolve 401 e não mexe em nada. Teste: `TestAgentGoodbye` (Tarefa 1).

---

### Task 1: Servidor: `POST /api/agent/goodbye` e `Client.Goodbye`

**Files:**
- Modify: `backend/internal/handler/handler.go` (registrar a rota junto das outras `/api/agent/`).
- Modify: `backend/internal/handler/agents.go` (novo `agentGoodbye`).
- Modify: `backend/internal/agent/client.go` (novo `Goodbye`).
- Test: `backend/internal/handler/agents_test.go`.

**Interfaces:**
- Produces: `func (c *Client) Goodbye(ctx context.Context) error`, que faz POST em `/api/agent/goodbye` sem corpo. Com 204 devolve `nil`; com 401 devolve `ErrUnauthorized`.

- [ ] **Step 1: Escrever o teste que falha.** `TestAgentGoodbye`:
  - pareia "PC", e `c.agent("POST","/api/agent/goodbye","errado",nil,nil)` dá 401;
  - com a chave certa dá 204;
  - depois disso, hello com a mesma chave dá 401 e `GET /api/computers/me` não lista mais o PC (mesmo efeito de `DELETE /api/computers/{id}`);
  - um segundo goodbye com a mesma chave dá 401.
- [ ] **Step 2: Rodar e ver falhar.** `cd backend && mise x -- go test ./internal/handler -run TestAgentGoodbye -p 1`. Esperado: FAIL (404).
- [ ] **Step 3: Implementar `agentGoodbye`.** Usa `h.currentComputer` e `h.q.RevokeComputer(ctx, c.ID)`, e responde 204. Também implementar `Client.Goodbye`, com os mesmos helpers de `Hello`.
- [ ] **Step 4: Rodar e ver passar.** Mesmo comando. Esperado: PASS.
- [ ] **Step 5: Commit.** `git commit -m "agente: rota goodbye para o computador se desconectar"`

### Task 2: `Status` e `Session` em `internal/agent`

**Files:**
- Create: `backend/internal/agent/session.go`.
- Modify: `backend/internal/agent/agent.go`: campo `OnReachable func(bool)` no `Agent`, chamado em `Loop` com `false` quando `Next` falha por rede e `true` quando funciona (só quando o valor muda).
- Test: `backend/internal/agent/session_test.go` (servidor `httptest` falso, no estilo dos testes atuais do pacote).

**Interfaces:**
- Produces:
  ```go
  type State int
  const (StateConnecting State = iota; StateConnected; StateOffline; StateDisconnected)
  type Status struct{ /* mu, state */ }
  func (s *Status) Set(State)
  func (s *Status) Get() State
  func (s *Status) Subscribe(fn func(State))   // chamada a cada mudança (em goroutine do chamador de Set)
  type Session struct {
      Client    *Client
      Name      string
      StateDir  string
      Open      func(url string) error
      Status    *Status
      Reconnect <-chan struct{}
      Run       func(ctx context.Context) error // roda o Agent.Loop; devolve ErrUnauthorized quando revogado
      Retry     time.Duration // 5s em produção; curto nos testes
  }
  func (s *Session) Start(ctx context.Context) error // só volta quando ctx acaba
  func MarkDisconnected(stateDir string) error  // grava <stateDir>/desconectado
  func IsMarkedDisconnected(stateDir string) bool
  ```
- O comportamento que `Start` precisa ter:
  1. Se `IsMarkedDisconnected`, o estado vira Disconnected e ele espera `Reconnect`.
  2. Sem chave, faz `Pair` em estado Connecting; ao parear, salva a chave e apaga a marca.
  3. Faz hello: com erro de rede, estado Offline e nova tentativa a cada `Retry`; com sucesso, Connected.
  4. Roda `Run`. Se `Run` devolver `ErrUnauthorized` (ou o hello der 401), salva a chave vazia, chama `MarkDisconnected`, o estado vira Disconnected e ele espera `Reconnect` para voltar ao passo 2.
  5. Um `Reconnect` que chega fora do estado Disconnected é ignorado.

- [ ] **Step 1: Escrever os testes que falham:**
  - `TestSessionStartsDisconnectedWhenMarked`: com a marca, o estado é Disconnected, `Open` não é chamado em 300 ms e nenhum pair/start chega ao servidor falso. Depois de um `Reconnect`, `Open` é chamado com a `ApproveURL`.
  - `TestSessionOfflineThenBack`: o servidor falso responde 503 no hello duas vezes e depois 204. Os estados vistos por `Subscribe` são Offline e depois Connected.
  - `TestSessionRevokedGoesDisconnected`: `Run` devolve `ErrUnauthorized`. O estado vai a Disconnected, a marca existe e `LoadToken` volta `""`.
  - `TestSessionReconnectIgnoredWhilePairing`: dois `Reconnect` seguidos geram um único pair/start.
  - `TestLoopReportsReachable`: `Agent.Loop` com `Next` falhando e depois funcionando chama `OnReachable(false)` e depois `OnReachable(true)`.
- [ ] **Step 2: Rodar e ver falhar.** `cd backend && mise x -- go test ./internal/agent -run 'TestSession|TestLoopReportsReachable'`. Esperado: FAIL (não compila).
- [ ] **Step 3: Implementar** `session.go` e `OnReachable`. A marca é o arquivo vazio `desconectado` na pasta do Agente.
- [ ] **Step 4: Rodar o pacote inteiro.** `mise x -- go test ./internal/agent`. Esperado: PASS.
- [ ] **Step 5: Commit.** `git commit -m "agente: sessão com estado e reconexão sem abrir o navegador sozinho"`

### Task 3: Ícone, menu, desinstalar e Aplicativos instalados (Windows)

**Files:**
- Create: `backend/cmd/agent/tray_windows.go`, `backend/cmd/agent/tray_other.go` (sem ícone: só bloqueia até o ctx acabar).
- Create: `backend/cmd/agent/uninstall.go` (passos independentes de plataforma, testáveis), `backend/cmd/agent/uninstall_windows.go`, `backend/cmd/agent/uninstall_other.go`.
- Create: `backend/cmd/agent/icons/conectado.ico` e `icons/sem-conexao.ico` (16/32 px: "Z" branco em círculo, verde `#16a34a` e cinza `#9ca3af`), gerados por `backend/cmd/agent/icons/gerar.go` (`//go:build ignore`, só biblioteca padrão).
- Modify: `backend/cmd/agent/main.go`:
  - usar `Session`;
  - `--desinstalar` antes de `install`;
  - a segunda instância mostra a caixa de mensagem;
  - nova rota local `POST /quit` mantida.
- Modify: `backend/cmd/agent/platform_windows.go`: `install` também grava a chave de desinstalação; nova `messageBox(text string, yesNo bool) bool` com `MessageBoxW`.
- Modify: `backend/internal/agentproto/proto.go`: `Version = "0.5.0"`.
- Test: `backend/cmd/agent/uninstall_test.go`.

**Interfaces:**
- Consumes: `Session`, `Status`, `State` (Tarefa 2) e `Client.Goodbye` (Tarefa 1).
- Produces:
  ```go
  type uninstallSteps struct {
      QuitRunning     func()
      Goodbye         func(ctx context.Context) error
      RemoveAutostart func() error
      RemoveAppEntry  func() error
      RemoveFolder    func(dir string) error // no Windows: cmd /c "timeout /t 3 >nul & rmdir /s /q <dir>" com CREATE_NO_WINDOW, sem esperar
  }
  func (u uninstallSteps) run(ctx context.Context, dir string) []error // roda todos, na ordem, mesmo se algum falhar
  ```
- O menu é montado por `trayLabel(State, version, name string) string`, testável, com os textos das Global Constraints. O ícone e os itens visíveis mudam com o estado: Conectar de novo só aparece em Disconnected.

- [ ] **Step 1: Escrever os testes que falham:**
  - `TestUninstallStepsContinueWhenGoodbyeFails`: `Goodbye` devolve erro, e mesmo assim `RemoveAutostart`, `RemoveAppEntry` e `RemoveFolder` são chamados nessa ordem. `run` devolve 1 erro.
  - `TestTrayLabel`: para os 4 estados, o texto é exatamente o das Global Constraints (versão "0.5.0", nome "PC-1").
- [ ] **Step 2: Rodar e ver falhar.** `cd backend && mise x -- go test ./cmd/agent`. Esperado: FAIL.
- [ ] **Step 3: Implementar.**
  - `go get fyne.io/systray@v1.12.2`.
  - O `systray.Run` fica na thread principal (`runtime.LockOSThread`), e a `Session` roda em goroutine.
  - **Sair**: confirma, `cancel()` e `systray.Quit()`.
  - **Desinstalar**: confirma, faz `uninstallSteps.run` (com `QuitRunning` vazio, porque é ele mesmo) e sai.
  - `--desinstalar`, vindo do Windows: confirma, `QuitRunning` = `quitRunning`, depois o mesmo `run`.
  - A chave de desinstalação leva `DisplayName`, `Publisher`, `DisplayVersion`, `DisplayIcon` (o próprio exe), `UninstallString`, `NoModify=1` e `NoRepair=1`.
- [ ] **Step 4: Rodar os testes e compilar para Windows.**
  - `mise x -- go test ./cmd/agent ./internal/agent`.
  - `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 mise x -- go build -o /tmp/ag.exe ./cmd/agent`. Esperado: PASS e compila.
  - `mise x -- go vet ./...`.
- [ ] **Step 5: Commit.** `git commit -m "agente 0.5.0: ícone perto do relógio, desinstalar e reconectar"`

### Task 4: Documentos, deploy e teste no Windows

**Files:**
- Create: `docs/adr/049-agente-visivel.md`.
- Modify: `docs/TASKS.md`, `docs/INFRASTRUCTURE.md` (seção do Agente: ícone, chave Uninstall, marca `desconectado`).

- [ ] **Step 1:** Escrever o ADR 049 e atualizar TASKS e INFRASTRUCTURE.
- [ ] **Step 2:** Rodar a bateria inteira do backend: `cd backend && mise x -- go test -p 1 ./...`. Esperado: PASS.
- [ ] **Step 3:** Commit e push no main. Deploy com `bash scripts/deploy-gcp.sh`, com o "pode" do Henry. Esperado: `site: 200`.
- [ ] **Step 4:** Teste no PC do Henry, um passo por pergunta:
  - o Agente atualiza sozinho para 0.5.0 e o ícone aparece;
  - o menu mostra Conectado;
  - aparece em Aplicativos instalados;
  - desconectar na tela Computadores deixa o ícone cinza, e Conectar de novo pareia;
  - Desinstalar pelo Windows tira tudo, e baixar e abrir de novo reinstala.
