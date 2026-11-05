# Entrada, Começar e Configurações — desenho

Data: 2026-11-05 · Pedido do Henry depois do primeiro uso do Ziv no servidor (ADR 022).

## Por quê

No primeiro uso de verdade em `https://zivapp.duckdns.org`, o Henry achou três coisas confusas:

1. **Login.** Ele criou a empresa "Teste" e já saiu logado. Depois teve que entrar de novo, porque a empresa *principal* é outra empresa. A existência de duas empresas é a confusão.
2. **Página inicial.** O logo e o cabeçalho estão pequenos demais.
3. **Depois de entrar.** Não fica claro o que fazer. Ele quer cair num **Começar** com passos em ordem: Agente, Claude e depois como subir app e artifact. As **Configurações** devem só mostrar o status das conexões, não ensinar a conectar.

Sucesso: um cliente novo cria a empresa, cai direto no Começar já logado, segue os passos de cima para baixo sem se perder e nunca ouve falar da "empresa principal".

## O que o Henry decidiu

### 1. Empresa principal escondida

- **Cliente só vê uma porta:** a página inicial (`/boas-vindas`) com "Cadastre-se" e "Entrar".
  - Quem cria a empresa já sai logado e vai direto para o Começar da empresa nova (`/w/<empresa>/comecar`). Hoje vai para `/instalar-agente`.
- **"Entrar" pede só e-mail e senha** (decisão do Henry). A tela `/acesso` deixa de pedir o endereço da empresa.
  - O servidor procura o e-mail nas empresas de clientes, com exceção da principal, e confere a senha em cada uma.
  - Se a senha bate em uma empresa só, a sessão abre nela e a pessoa vai para lá.
  - Se bate em mais de uma, aparece "Em qual empresa?" com os nomes, e a pessoa escolhe. Só aparecem empresas em que a senha bateu, e a sessão só abre depois da escolha.
  - Se não bate em nenhuma, a resposta é a mesma mensagem genérica de hoje, sem revelar se o e-mail existe. O tempo de resposta deve ser parecido nos dois casos, e a tela usa o limite de tentativas que já existe para `/api/entry/`.
  - A entrada pelo endereço da empresa (`/w/<empresa>/entrar`) continua funcionando para quem tem o link.
  - **Depois, fora deste trabalho:** entrar com Google e Microsoft. O Henry quer isso "eventualmente", então o desenho do login não deve fechar essa porta.
- **A principal vira o painel do dono do Ziv** (o Henry). Ela some de tudo que o cliente vê.
  - Sai o link "Usa o espaço principal deste portal? Entrar no espaço principal" da tela `/acesso`.
  - Nenhuma página pública aponta para a principal.
- O Henry, como pessoa, usa uma empresa normal (Alis ou Teste) e entra na principal só para cuidar do servidor.
- **Fora deste trabalho, como projeto separado e maior:** apps compartilhados nas empresas dos clientes. Hoje eles só funcionam na principal (ADR 015). O Henry está ciente de que, até lá, a empresa dele fica sem compartilhados.

### 2. Cabeçalho da página inicial maior (opção B)

| | Hoje | Novo |
|---|---|---|
| Logo (`Wordmark`) | 32px (`h-8`) | 48px (`h-12`) |
| Barra | 64px (`h-16`) | ~84px (`h-[84px]` ou `h-20`/`h-[5.25rem]`) |
| Menu | 14px | 16px |
| Botões Entrar/Cadastre-se | padrão | um pouco maiores, proporcionais |

Vale só para a página inicial pública (`LandingPage`). No celular (menu escondido), o logo maior precisa caber ao lado dos dois botões sem quebrar a linha.

### 3. Começar em lista, com 6 passos

**Forma (opção A):** uma lista em que todos os passos aparecem e **o primeiro passo não feito fica aberto**, com as instruções e os botões dentro. Os feitos ficam fechados, com ✓ e um resumo curto (ex.: "Agente ligado"). Os demais ficam fechados. Clicar num passo fechado abre ele. No topo, "N de 6 feitos". O progresso é confirmado pelo Ziv, como hoje: nada é marcado à mão.

**Passos, nesta ordem:**

1. **Conectar seu computador.** Baixar o Agente, com o passo a passo de instalação dentro do próprio passo: aviso do Windows → "Mais informações" → "Executar assim mesmo" → aprovar a conexão. Feito quando há um computador conectado (`computer_connected`).
2. **Conectar o Claude.** Explica em uma frase para que serve ("peça 'roda o app X' ou 'publica esse relatório' direto na conversa").
   - **claude.ai ou app do Claude:** endereço do conector com botão Copiar e o caminho (Configurações → Conectores → Adicionar).
   - **Claude Code:** botão "Gerar código" (o mesmo fluxo que hoje está em Configurações).
   - Feito quando a pessoa tem pelo menos uma conexão do Claude ativa. **Campo novo** no `/api/onboarding`: `claude_connected`.
3. **Subir seu primeiro app.** Feito pelo site, na tela Registrar, porque registrar pelo Claude é a Etapa 2 do MCP, ainda não feita. Explica o que conta como app (script, app web, programa) e leva a `/registrar`. Feito com `has_apps`.
4. **Rodar o app.** Abre o catálogo e roda um app. Feito com `has_successful_run`.
5. **Publicar um artifact.** Explica que o caminho principal é pedir ao Claude ("publica isso no Ziv"), com a alternativa pelo site (`/artifacts/novo`). Feito quando a empresa tem pelo menos um artifact. **Campo novo:** `has_artifacts`.
6. **Convidar a equipe.** Leva a Equipe → Pessoas. Feito com `has_team`.

Quando os 6 estiverem feitos, aparece a mensagem de "tudo pronto", como hoje.

### 4. Configurações só com o status

A página `Configurações` passa a mostrar só uma lista de conexões, cada uma com o status e uma ação de desfazer:

- **Cada computador:** nome, Agente ligado/desligado, Desconectar.
- **Cada conexão do Claude** (claude.ai/app e cada código do Claude Code): nome, último uso, Desconectar.
- **O que não estiver conectado:** "não conectado · Conectar no Começar", com link para o passo certo do Começar.
- **Aparência** (claro/escuro) continua, no fim.

Saem das Configurações: o botão de baixar o Agente, o passo a passo, o endereço do conector e o "Gerar código". Tudo isso passa a morar no Começar.

## Escolhas minhas (o Henry pode mudar)

- **Para onde a pessoa vai ao entrar:** quem ainda não terminou o Começar cai nele ao entrar. Quem terminou cai na Home, como hoje. O item "Começar" continua no menu lateral.
- **Por papel:** os passos 3 (subir app) e 6 (convidar) só aparecem para quem tem permissão (`canRegister` e `can_manage`). A contagem "N de M" usa só os passos que a pessoa vê. Um membro comum vê Computador, Claude, Rodar e Artifact.
- **`/instalar-agente` e a faixa do Agente:** o conteúdo da página vai para dentro do passo 1. A rota passa a levar ao Começar com o passo 1 aberto (`/comecar#computador`). O link "Pular por agora" da janela de download continua levando ao Começar.
- **Primeiro acesso da principal protegido:** num servidor novo, hoje qualquer pessoa vira admin da principal abrindo `/entrar` (ver TASKS e ADR 022). Junto com esconder a principal, o primeiro acesso dela passa a exigir um código definido no servidor (`SETUP_CODE` no `.env`). Sem o código configurado, o primeiro acesso só funciona em `DEV_MODE`. O servidor de produção já tem dono, então não muda nada para ele.
- **Endereço do conector:** o passo 2 mostra o endereço da empresa da pessoa (`/w/<empresa>/mcp`), como o `ClaudeConnections` já faz.

- **Principal só sem link, sem bloqueio:** as rotas da raiz continuam acessíveis para quem digitar o endereço. A proteção é o login, mais o código de primeiro acesso.
- **A principal fica fora da regra "cai no Começar ao entrar" e não mostra o Começar no menu.** Ela é painel de dono, não empresa de cliente.
- **Quando o passo 2 conta como feito (`claude_connected`):** quando a pessoa tem uma conexão do Claude ativa que **já foi usada pelo menos uma vez** (`last_used_at` preenchido). Gerar um código e nunca usar não conta. O passo explica: "fica ✓ quando o Claude falar com o Ziv pela primeira vez".
- **Dados da principal de produção:** ela está vazia (criada em 2026-11-05), então não há nada para mover.

## Fora deste trabalho

- Entrar com Google e Microsoft.

- Apps compartilhados nas empresas dos clientes.
- Registrar apps pelo Claude (MCP Etapa 2).
- Mudar o endereço da principal ou criar um painel de dono diferente do que já existe.
- Mudar o resto da página inicial (título, seções).

## Feito quando

- Entrar com só e-mail e senha; com a mesma senha em duas empresas, escolher entre elas.
- Criar empresa → já logado → Começar da empresa nova, com o passo 1 aberto.
- `/acesso` não menciona a principal.
- Começar em lista com os 6 passos, cada um ficando ✓ sozinho quando o Ziv confirma.
- Configurações só com status e Desconectar.
- Cabeçalho da página inicial no tamanho B, conferido no computador e no celular.
- Testes Go e Vitest cobrindo o login por e-mail (uma empresa, várias, nenhuma, principal excluída), os campos novos do onboarding, o código de primeiro acesso, o Começar (passo aberto, passos por papel) e as Configurações.
- Publicado em `https://zivapp.duckdns.org` com `scripts/deploy-gcp.sh` e testado pelo Henry.
