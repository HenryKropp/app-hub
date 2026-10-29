# Nova identidade Kova — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** O App Hub vira Kova. Muda o que a pessoa vê (nome, símbolo, cores, textos) e os nomes técnicos `app-hub`/`apphub`, com convivência, para que nenhum app já registrado ou compartilhado quebre.

**Architecture:** O nome do produto passa a morar numa constante só, uma no frontend (`lib/brand.ts`) e outra no backend (`internal/brand`). As cores trocam nos tokens de `index.css`. O coral (`--brand`) passa a ser só preenchimento, e todo uso de `text-brand` em palavra ou ícone vira neutro. Nos nomes técnicos, cada constante nova ganha uma irmã `Legacy*`: o código escreve o nome novo e reconhece os dois. Na primeira subida, o servidor adota sozinho o contêiner de banco e a pasta de dados antigos, renomeando sem recriar.

**Tech Stack:** React 19, TypeScript, Tailwind v4, Vitest e Testing Library, Playwright (e2e e fotos); Go stdlib, Podman CLI, pgx.

**Spec:** `docs/superpowers/specs/2026-10-28-kova-identidade-design.md`

## Global Constraints

- Nome: **Kova**. No logo, "kova" em minúsculas; nos textos, "o Kova", "do Kova", "no Kova", "Agente do Kova".
- Símbolo: o SVG oficial do spec, sem alteração. O quadrado é `#0E3B43` no claro e `#256A75` no escuro; o arco é `#F4EFE6`, traço 5, ponta redonda; o ponto é `#FF6B4A`, raio 5.
- Coral `#FF6B4A` (hover `#F0562F`) só como preenchimento: o botão Rodar (texto `#11161A`), as barrinhas tocando e o marcador do item ativo da navegação. **Nunca** em texto, ícone ou decoração. O mesmo valor nos dois temas.
- Neutros:
  - claro: fundo `#FFFFFF`, sidebar `#FAFAFA`, card `#FFFFFF`, texto `#11161A`, cinza `#6B6F73`, borda `#EAEAEA`;
  - escuro: fundo `#11161A`, sidebar `#141A1F`, card `#161C21`, texto `#EEF0F1`, cinza `#A3A9AE`, borda `#252C33`.
- Foco: anel neutro, `#11161A` no claro e `#EEF0F1` no escuro, com afastamento. Petróleo só no logo.
- Erro `#B4232F` no claro e `#F2707B` no escuro. Verde ok inalterado. Âmbar `oklch(0.55 0.12 85)` no claro e `oklch(0.8 0.13 85)` no escuro.
- Contraste WCAG AA nos dois temas: texto a 4.5:1 (3:1 a partir de 24px); bordas de campo, foco e ícones a 3:1.
- Convivência: o código **escreve** o nome novo e **lê** o novo e o antigo. Se existirem os dois, vale o novo. Nada é recriado: adotar é sempre renomear.
- **Ficam como estão de propósito**, porque são invisíveis e trocar apagaria estado das pessoas:
  - chaves de `localStorage` (`apphub-*`);
  - pasta de instalação do agente (`%LOCALAPPDATA%\AppHub`, `~/.apphub-agente`), a chave de registro `AppHubAgente` e os nomes `AppHubAgente*.exe` no disco;
  - a variável `APPHUB_INITIAL` e a classe `AppHubFolderPicker`.
  Só muda o nome do arquivo baixado: `KovaAgente.exe`.
- Documentos históricos (`docs/superpowers/plans|specs` antigos, ADRs 001–012, `Documentação/`) não são reescritos.
- Gates do cofounder em cada task: `cd frontend && npm test && npm run lint && npm run build`; `cd backend && go test ./...`; commit em português.

## Review Focus

1. **App registrado só com `app-hub.yaml`**: ao reabrir, editar a ficha e salvar, continua sendo lido e é salvo no **mesmo** arquivo, sem criar um `kova.yaml` duplicado (teste na Task 5).
2. **Pasta com `app-hub.yaml` e `kova.yaml`**: vale o `kova.yaml`. O antigo não entra no histórico como código, nem nas entradas de script (Task 5).
3. **App compartilhado no ar com etiquetas antigas** (`app-hub.app`, rede `app-hub-apps-<id>`, imagem `localhost/apphub/…`): depois do deploy, o vigia ainda o reconhece, consegue parar e religar, e não o trata como contêiner alheio (Task 7).
4. **Servidor sobe com `app-hub-db` e `~/.local/share/app-hub/shared` existentes** e sem variáveis definidas: adota os dois (rename) em vez de criar banco e pasta vazios. Se já existirem os novos, não toca em nada (Task 8).
5. **Restaurar uma versão num app que já tem `.app-hub-lixeira`**: a pasta antiga continua fora das versões e do histórico, e a nova lixeira é `.kova-lixeira` (Task 6).

---

### Task 1: Nome num lugar só e `clsx`

**Files:**
- Create: `frontend/src/lib/brand.ts`, `backend/internal/brand/brand.go`
- Test: `frontend/src/lib/brand.test.ts`, `backend/internal/brand/brand_test.go`
- Modify: `frontend/package.json` (`"name": "kova"`; `clsx` em `dependencies`, na versão já presente em `package-lock.json`)

**Interfaces:**
- Produces: `export const PRODUCT_NAME = "Kova"` e `export const AGENT_NAME = "Agente do Kova"` (frontend); `brand.Name = "Kova"` e `brand.AgentFile = "KovaAgente.exe"` (Go).

- [ ] **Step 1:** Testes: `expect(PRODUCT_NAME).toBe("Kova")` e `expect(AGENT_NAME).toBe("Agente do Kova")`; em Go, `brand.Name == "Kova"` e `brand.AgentFile == "KovaAgente.exe"`.
- [ ] **Step 2:** Rodar os testes; devem falhar porque os módulos não existem.
- [ ] **Step 3:** Criar as constantes; acertar o `package.json` e rodar `npm install` para atualizar o lock.
- [ ] **Step 4:** Rodar os gates; devem passar.
- [ ] **Step 5:** Commit `Nome do produto numa constante só (Kova) e clsx declarado`.

### Task 2: Cores novas e coral só como preenchimento

**Files:**
- Modify:
  - `frontend/src/index.css` (tokens claro e escuro);
  - `frontend/src/components/ui/button.tsx:20` (`run`);
  - os 18 usos de cor da marca: `Layout.tsx:106,109,133`, `ScriptImportForm.tsx:174,176`, `SharedStatus.tsx:7`, `bits.tsx:46,164`, `ficha/shared/Publish.tsx:109-111,120,137`, `CatalogPage.tsx:81,82,143`, `MyRunsPage.tsx:101`, `SetupPage.tsx:92,98,99`, `VersionPage.tsx:50`.
- Create: `scripts/contrast-check.mjs`
- Test: `frontend/src/components/ui/button.test.tsx` (criar se não existir), `Publish.test.tsx:99` (ajustar), `SetupPage.test.tsx`

**Interfaces:**
- Produces: tokens `--brand: #FF6B4A`, `--brand-hover: #F0562F` e `--brand-foreground: #11161A`, iguais nos dois temas. `--ring` é `#11161A` no claro e `#EEF0F1` no escuro. `--brand-soft` é removido.

**Regra por uso:**
- **Continuam coral** (preenchimento): `bits.tsx:46` (barrinhas, via `currentColor` em `.playing`), `SharedStatus.tsx:7` (ponto "no ar"), o botão `run` e o marcador do item ativo em `Layout.tsx`. O marcador é um traço coral de 3px à esquerda do item ativo; o ícone ativo fica `text-foreground`.
- **Viram neutros** (palavra ou ícone):
  - `text-brand` em `bits.tsx:164`, `Publish.tsx:120,137`, `CatalogPage.tsx:143`, `MyRunsPage.tsx:101`, `VersionPage.tsx:50` e `Layout.tsx:109,133` vira `text-foreground`, com `font-medium` onde já havia destaque;
  - nos banners `SetupPage.tsx:98-99`, `CatalogPage.tsx:81-82` e `ScriptImportForm.tsx:174-176`, saem `border-brand*`, `bg-brand-soft*` e `bg-brand*`; ficam `border-border`, `bg-muted/40` e ícone com `bg-muted text-foreground`;
  - o rótulo acima do título ("Primeiros passos", "Configuração inicial") é removido;
  - "Abrir catálogo" (`SetupPage.tsx:92`) usa `variant="default"`;
  - em `Publish.tsx:109-111`, só Publicar fica `run`; Testar vira `outline`.

- [ ] **Step 1:** Testes:
  - o `Button variant="run"` tem `bg-brand` e `text-brand-foreground`;
  - em `Publish.test.tsx`, Testar **não** contém `bg-brand` e Publicar contém;
  - em `SetupPage.test.tsx`, há um único `h1` e "Abrir catálogo" não contém `bg-brand`.
- [ ] **Step 2:** Rodar `npm test`; os testes novos devem falhar.
- [ ] **Step 3:** Trocar os tokens:
  - aplicar a tabela de neutros do Global Constraints nos papéis existentes (`--background`, `--sidebar`, `--card`, `--foreground`, `--muted-foreground`, `--border`);
  - derivar `--muted`, `--accent` e `--secondary` (fundo de pílula e hover) a partir de `#F2F2F2`/`#F0F0F0` no claro e `#232A30`/`#1E252B` no escuro, conforme o mockup aprovado;
  - `--input` com pelo menos 3:1 no fundo;
  - `--destructive`, `--warn` e `--ok` com os valores do spec.
  - Hex ou OKLCH fica a critério de quem implementa.
- [ ] **Step 4:** No `run`, usar `hover:bg-brand-hover`, foco no anel neutro com `ring-offset-2` e manter `[&_svg]:fill-current`.
- [ ] **Step 5:** Aplicar a regra por uso nos 18 pontos.
- [ ] **Step 6:** Escrever `scripts/contrast-check.mjs`:
  - lê os pares (texto ou borda sobre fundo) dos dois temas a partir de `index.css`, converte OKLCH ou hex e calcula a razão WCAG;
  - mistura alfa em espaço sRGB gama;
  - sai com código 1 se algum par ficar abaixo do mínimo.
  - Pares checados: foreground, muted-foreground, destructive, warn e ok sobre background e card; brand-foreground sobre brand e sobre brand-hover; input e ring sobre background.
- [ ] **Step 7:** Rodar `node scripts/contrast-check.mjs`; tudo deve passar. Rodar `grep -rn "brand-soft\|text-brand" frontend/src`; não pode sobrar nada.
- [ ] **Step 8:** Rodar os gates e o gate visual: fotos de Catálogo, Primeiros passos e ficha (aba Compartilhar) em claro e escuro, desktop e celular, com Playwright.
- [ ] **Step 9:** Commit `Cores do Kova: app neutro, coral só no Rodar e no que está rodando`.

### Task 3: Símbolo e favicon

**Files:**
- Modify: `Logo` em `frontend/src/components/bits.tsx`, `frontend/index.html:5`
- Test: `frontend/src/components/bits.test.tsx` (criar ou estender)

**Interfaces:**
- Consumes: `PRODUCT_NAME` (Task 1).
- Produces: `Logo({ className }: { className?: string })`, um SVG com `role="img"` e `aria-label={PRODUCT_NAME}`. O quadrado usa `fill="var(--logo-square)"`, com `--logo-square` igual a `#0E3B43` no claro e `#256A75` no escuro, definido em `index.css`.

- [ ] **Step 1:** Teste: `render(<Logo />)`, depois `getByRole("img", { name: "Kova" })`. Conferir que o `path` tem `d="M33 13.5A14 14 0 1 0 33 34.5"` e o `circle` tem `fill="#FF6B4A"`.
- [ ] **Step 2:** Rodar o teste; deve falhar.
- [ ] **Step 3:** Reescrever `Logo` com a geometria oficial e trocar o favicon em `index.html` pelo SVG oficial do tema claro (data URI, `#` escapado como `%23`).
- [ ] **Step 4:** Rodar os gates; devem passar. Conferir nas fotos o logo nos dois temas e a aba do navegador.
- [ ] **Step 5:** Commit `Símbolo do Kova no app e na aba do navegador`.

### Task 4: Textos da interface dizem "Kova"

**Files:**
- Modify:
  - frontend: `index.html:7`, `App.tsx:36`, `Layout.tsx:71`, `LoginPage.tsx:58,73`, `SetupPage.tsx`, `lib/shared.ts:37`, `ComputerSection.tsx:49` e os demais 43 arquivos com `\bHub\b` em texto de tela;
  - backend: as 67 strings com `Hub` em Go (por exemplo `readiness.go`), `internal/shared/gateway/page.html`, a linha final da ficha em `manifest/files.go:181` e `handler/agents.go:439` (`filename="` + `brand.AgentFile` + `"`);
  - testes que conferem esses textos (por exemplo `SetupPage.test.tsx:29`).

**Interfaces:**
- Consumes: `PRODUCT_NAME`, `AGENT_NAME`, `brand.Name`, `brand.AgentFile` (Task 1).

**Regras de troca:**
- "App Hub" vira `{PRODUCT_NAME}`.
- "o Hub" vira "o Kova", "do Hub" vira "do Kova", "no Hub" vira "no Kova", "ao Hub" vira "ao Kova".
- "Agente do Hub" vira `AGENT_NAME`.
- "AppHubAgente.exe", quando aparece em texto de tela, vira "KovaAgente.exe".
- Comentários e logs (`slog`) também podem trocar, mas não é obrigatório.

- [ ] **Step 1:** Ajustar primeiro os testes que conferem textos ("Prepare o Kova para sua equipe" e afins). Em Go, adicionar ao teste do download do agente o header `Content-Disposition` com `KovaAgente.exe`.
- [ ] **Step 2:** Rodar os testes; devem falhar.
- [ ] **Step 3:** Aplicar as regras de troca.
- [ ] **Step 4:** Rodar `grep -rnw "Hub" frontend/src backend --include=*.tsx --include=*.ts --include=*.go --include=*.html | grep -v "^\S*:\s*//"`. Só pode sobrar comentário ou identificador de código.
- [ ] **Step 5:** Rodar os gates; devem passar. Commit `Textos da interface: "o Kova"`.

### Task 5: Manifesto `kova.yaml` convivendo com `app-hub.yaml`

**Files:**
- Modify:
  - `backend/internal/manifest/manifest.go:18-24`, `manifest/files.go`, `manifest/execution.go:78`;
  - `internal/scanner/scanner.go:243`, `internal/versions/rules.go:25`, `handler/script_import.go:78`, `handler/apps.go`, `handler/team.go`;
  - `cmd/validate-manifest/main.go`;
  - textos com `app-hub.yaml` em `ReviewForm.tsx:261`, `RegisterPage.tsx:236` e `lib/shared.ts:39`.
- Test: `backend/internal/manifest/files_test.go`, `internal/scanner/scanner_test.go`, `internal/versions/rules_test.go` (criar se não existir)

**Interfaces:**
- Produces:
  - constantes `FileName = "kova.yaml"` e `LegacyFileName = "app-hub.yaml"`;
  - `headerMarker = "# Kova — manifesto do app"`, com `legacyHeaderMarker = "# App Hub — manifesto do app"`;
  - `fichaMarker = "<!-- kova:ficha -->"`, com `legacyFichaMarker = "<!-- app-hub:ficha -->"`;
  - `func Locate(dir string) string`, que devolve o caminho do manifesto em uso: `kova.yaml` se existir, senão `app-hub.yaml` se existir, senão o caminho de `kova.yaml`;
  - `func IsManifestName(name string) bool`, verdadeiro para os dois nomes, sem diferenciar maiúsculas.
  - `WriteToFolder` grava no caminho de `Locate` e aceita os dois marcadores como "escrito pelo Kova".

- [ ] **Step 1:** Testes:
  - `TestWriteToFolder_LegacyOnly_KeepsLegacy`: a pasta só tem um `app-hub.yaml` escrito pelo Hub antigo; depois de `WriteToFolder`, ele foi atualizado e `kova.yaml` **não** existe;
  - `TestWriteToFolder_NewApp_WritesKova`;
  - `TestLocate_BothPresent_PrefersKova`;
  - `TestWriteToFolder_ForeignKovaYaml_Refuses`: um `kova.yaml` sem marcador devolve `ErrForeignFile`;
  - no scanner, uma pasta com `app-hub.yaml` volta com `HasManifest` verdadeiro;
  - em `versions`, `Included` e o filtro de arquivos do Hub tratam os dois nomes como arquivos do Hub;
  - em `script_import`, um arquivo `kova.yaml` é recusado como entrada.
- [ ] **Step 2:** Rodar `go test ./internal/manifest/... ./internal/scanner/... ./internal/versions/... ./internal/handler/...`; os testes novos devem falhar.
- [ ] **Step 3:** Implementar `Locate` e `IsManifestName`. Trocar todos os usos de `manifest.FileName` como nome fixo e os literais `"app-hub.yaml"`: na leitura, `Locate`; na comparação, `IsManifestName`. `hubFiles` passa a incluir os dois nomes.
- [ ] **Step 4:** Atualizar os textos de tela e o `validate-manifest` (uso: `<caminho/kova.yaml>`, aceitando os dois nomes).
- [ ] **Step 5:** Rodar os gates; devem passar. Rodar `e2e/etapa1.mjs`, o fluxo de registrar app; deve passar.
- [ ] **Step 6:** Commit `Manifesto kova.yaml, ainda lendo app-hub.yaml dos apps já registrados`.

### Task 6: Lixeira e temporários `.kova-*`

**Files:**
- Modify: `backend/internal/versions/rules.go:14-22`, `versions/restore.go:82`, `versions/folder.go:71`, `frontend/src/components/RestoreDialog.tsx:66`
- Test: `backend/internal/versions/restore_test.go`, `rules_test.go`

**Interfaces:**
- Produces: `TrashDir = ".kova-lixeira"`, `LegacyTrashDir = ".app-hub-lixeira"`, `tempSuffix = ".kova-tmp"`, `legacyTempSuffix = ".app-hub-tmp"`. As duas lixeiras entram em `junkDirs` e os dois sufixos são excluídos por `Included`.

- [ ] **Step 1:** Testes:
  - `Included(".app-hub-lixeira/x.py")` e `Included(".kova-lixeira/x.py")` são falsos;
  - `Included("a.py.app-hub-tmp")` é falso;
  - a restauração move os arquivos para `.kova-lixeira/<data>`.
- [ ] **Step 2:** Rodar os testes; devem falhar.
- [ ] **Step 3:** Implementar e trocar o texto do `RestoreDialog` para ".kova-lixeira".
- [ ] **Step 4:** Rodar os gates; devem passar. Commit `Lixeira e temporários com nome do Kova; os antigos seguem ignorados`.

### Task 7: Etiquetas, redes e imagens dos apps compartilhados

**Files:**
- Modify:
  - `backend/internal/shared/engine/engine.go:14-18`;
  - `engine/podman.go:91-97,165,172,188,486,505` e os prefixos temporários `apphub-iid-` e `apphub-env-` (podem virar `kova-`);
  - `engine/fake.go:273`;
  - `deploy/deploy.go:349-378`, `supervisor/supervisor.go:138,203`.
- Test: `engine/podman_guard_test.go`, `engine/podman_test.go`, `supervisor/supervisor_test.go`, `deploy/deploy_test.go`

**Interfaces:**
- Produces:
  - `Label = "kova.app"` e `LegacyLabel = "app-hub.app"`;
  - `DeploymentLabel = "kova.deployment"` e `LegacyDeploymentLabel = "app-hub.deployment"`;
  - `NetworkPrefix = "kova-apps-"` e `LegacyNetworkPrefix = "app-hub-apps-"`;
  - `ImagePrefix = "localhost/kova/"` e `LegacyImagePrefix = "localhost/apphub/"`;
  - `func AppLabel(labels map[string]string) (string, bool)`, que devolve o valor de `Label` ou, se não houver, o de `LegacyLabel`;
  - `func DeploymentID(labels map[string]string) string`, com a mesma regra.
  - `HubNetwork` aceita os dois prefixos. A listagem (`ps --filter label=`) consulta os dois rótulos e junta os resultados.
  - Contêineres novos se chamam `kova-<slug>-<id>` e `kova-<slug>-teste-<id>`.

- [ ] **Step 1:** Testes:
  - o guarda aceita parar e remover um contêiner que só tem `app-hub.app` e está numa rede `app-hub-apps-5`;
  - o guarda continua recusando um contêiner sem nenhum dos dois rótulos;
  - a listagem do `Fake` devolve contêineres antigos e novos;
  - o supervisor acha o deployment de um contêiner antigo pelo `app-hub.deployment`;
  - um deploy novo cria o contêiner com `kova.app` e `kova.deployment`.
- [ ] **Step 2:** Rodar `go test ./internal/shared/...`; os testes novos devem falhar.
- [ ] **Step 3:** Implementar `AppLabel` e `DeploymentID` e usar nos pontos listados. Trocar os nomes que o deploy escreve.
- [ ] **Step 4:** Rodar os gates; devem passar. Rodar `e2e/compartilhados.mjs`; deve passar.
- [ ] **Step 5:** Commit `Apps compartilhados com etiquetas do Kova, reconhecendo os já no ar`.

### Task 8: Banco e pasta de dados adotados na subida

**Files:**
- Modify: `backend/internal/config/config.go:56-59`, `.env.example:6,10`, `backend/cmd/server/main.go`
- Create: `backend/internal/shared/adopt/adopt.go`
- Test: `backend/internal/shared/adopt/adopt_test.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - na config, os valores padrão `AppsDBContainer = "kova-db"` e `SharedDataDir = ~/.local/share/kova/shared`;
  - os campos `AppsDBContainerExplicit bool` e `SharedDataDirExplicit bool`, verdadeiros quando a variável de ambiente estava definida;
  - `adopt.Dir(newPath, oldPath string) error`: se `newPath` não existe e `oldPath` existe, chama `os.Rename`; nos outros casos, não faz nada;
  - `adopt.Container(ctx, run func(ctx context.Context, args ...string) ([]byte, error), newName, oldName string) error`: se o contêiner `newName` não existe e `oldName` existe, roda `podman rename oldName newName`; nos outros casos, não faz nada. Se o contêiner antigo estiver rodando, o rename funciona assim mesmo.
- O `main.go` chama os dois antes de abrir o banco dos apps, só quando o campo `*Explicit` for falso, e registra a adoção no log com `slog.Info`.

- [ ] **Step 1:** Testes:
  - `Dir`: só o antigo existe, então é renomeado; os dois existem, então nada muda; nenhum existe, então nada acontece;
  - `Container` com um `run` falso que grava os argumentos: só `app-hub-db` existe, então roda `rename app-hub-db kova-db`; `kova-db` já existe, então não roda nada;
  - na config, sem variáveis, os padrões novos ficam com `Explicit` falso.
- [ ] **Step 2:** Rodar os testes; devem falhar.
- [ ] **Step 3:** Implementar e ligar no `main.go`.
- [ ] **Step 4:** Rodar os gates; devem passar. Prova real no PC: com o `app-hub-db` atual rodando, subir o servidor; `podman ps` deve mostrar `kova-db` com os mesmos dados e os apps compartilhados devem continuar abrindo.
- [ ] **Step 5:** Commit `Servidor adota o banco e a pasta de dados antigos com o nome do Kova`.

### Task 9: Documentos e design system

**Files:**
- Modify:
  - `PRODUCT.md` (nome e marca), `DESIGN.md` (nome, cores, coral só como preenchimento, foco neutro, símbolo);
  - `README.md`, `replit.md`, `docs/INFRASTRUCTURE.md` (`kova-db`, pasta de dados), `docs/PROTOCOLO-REGISTRO.md` (`kova.yaml`), `docs/TASKS.md`;
  - `.replit:15` (sem mudança de caminho; só o texto ao redor, se houver).
- Create: `docs/adr/013-nome-kova-com-convivencia.md`. Registra: os nomes novos e os legados lidos, o que fica de propósito (lista do Global Constraints) e quando a convivência pode acabar.

- [ ] **Step 1:** Atualizar os documentos e escrever o ADR.
- [ ] **Step 2:** Atualizar o design system publicado (https://claude.ai/artifact/97gdjWrmLxUx4QMYJGaQcW): `tokens.json`, README, a capa, o logo em `assets/Logos/` e as prévias dos componentes com as cores novas, a partir da pasta do projeto publicada. Publicar como nova versão.
- [ ] **Step 3:** Rodar `grep -rn "App Hub" PRODUCT.md DESIGN.md README.md replit.md docs/INFRASTRUCTURE.md docs/PROTOCOLO-REGISTRO.md`; só pode aparecer na frase histórica "antes chamado App Hub".
- [ ] **Step 4:** Commit `Documentos e design system com a identidade Kova`.

### Task 10: Repositório e módulo Go (depende do Henry)

**Files:**
- Modify: `backend/go.mod:1` (`module github.com/HenryKropp/kova/backend`) e todos os imports `github.com/HenryKropp/app-hub/backend/...`

- [ ] **Step 1:** O Henry renomeia o repositório no GitHub de `app-hub` para `kova` (Settings › Rename). O GitHub redireciona o endereço antigo.
- [ ] **Step 2:** Rodar `git remote set-url origin` com o endereço novo. Trocar o caminho do módulo em `go.mod` e nos imports com `grep -rl ... | xargs sed`.
- [ ] **Step 3:** Rodar `go build ./... && go test ./...`; deve passar. Conferir que o Replit continua puxando o repositório.
- [ ] **Step 4:** Commit `Módulo Go e repositório com o nome kova`.
