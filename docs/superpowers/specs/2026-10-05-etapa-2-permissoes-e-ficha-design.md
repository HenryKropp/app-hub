# Etapa 2: cargos, permissões e ficha editável — desenho

**Data:** 2026-10-05 · **Status:** aprovado em conversa pelo Henry; aguardando revisão deste documento
**Protótipo da ficha:** `2026-10-05-prototipo-ficha.html` (desenho B escolhido)
**Substitui:** a linha "Etapa 2, ficha por perfil (abas Usar/Cuidar…)" do desenho da Etapa 1. As abas Usar/Cuidar foram descartadas.

## 1. Objetivo

1. Quem pode o quê no Hub deixa de ser fixo em 3 papéis. Passam a existir cargos personalizáveis, áreas com líderes, acessos extras por app e apps restritos.
2. A ficha mostra a parte técnica só para quem pode vê-la. Cada parte ganha um "Editar" próprio, inclusive os campos novos do formato v0.2.

Uma entrega só (pedido do Henry), construída em fases dentro do plano.

## 2. Decisões do Henry (2026-10-05)

| Tema | Decisão |
|---|---|
| Quem edita um app | Responsável, reserva, líder da área, quem recebeu o acesso, e cargos com "editar qualquer app" |
| Cargo | **Um** por pessoa; vale para a empresa toda; personalizável |
| Acessos extras | O líder da área dá a uma pessoa, **app por app** (com atalho "todos os apps da área") |
| Quais extras | Editar a ficha · Voltar versões · Ver a parte técnica · Ver execuções de todos |
| Líder | O admin cria as áreas e escolhe os líderes (pode haver vários). Nos apps da área, o líder pode tudo o que um admin pode. Ele dá e tira acessos só nesses apps |
| Áreas | Lista fixa, gerenciada; todo app pertence a uma área da lista |
| Pessoas e áreas | Cada pessoa faz parte de uma ou mais áreas |
| App restrito | Já nesta entrega. Aberto (todos veem e rodam) ou restrito (só as áreas e pessoas escolhidas). Quem não tem acesso nem enxerga o app. Todo app novo começa aberto |
| Execuções | Cada um vê só as suas. Veem as de todos: quem tem a permissão no cargo, o líder da área e quem recebeu o acesso extra no app. Responsável e reserva **não** veem as dos outros sem esse acesso |
| Ficha | Desenho B: uma página; a parte técnica fica num bloco recolhido no fim, que aparece só para quem pode vê-la |

## 3. Modelo de permissões

### 3.1 Permissões de cargo (empresa toda)

| Código | Nome na tela |
|---|---|
| `register_apps` | Registrar apps novos |
| `edit_any_app` | Editar a ficha de qualquer app |
| `restore_any_app` | Voltar versões de qualquer app |
| `see_tech_any_app` | Ver a parte técnica de qualquer app |
| `see_all_runs` | Ver as execuções de todos |
| `see_all_apps` | Ver todos os apps, inclusive os restritos |
| `manage_people` | Cadastrar pessoas e dar cargos |
| `manage_areas` | Cuidar das áreas: criar áreas, escolher líderes, colocar pessoas |
| `manage_roles` | Criar e mudar cargos |

Cargos que já vêm prontos (editáveis): **Administrador** (todas), **Criador** (`register_apps`), **Usuário** (nenhuma).
Regra de segurança: sempre existe ao menos uma pessoa com um cargo que tenha `manage_roles` e `manage_people`. O Hub recusa a mudança que deixaria a empresa sem ninguém assim.

### 3.2 Permissões num app

Para uma pessoa P e um app A, as quatro ações de app são:

| Ação | Quem pode |
|---|---|
| **ver e rodar** | app aberto: todos · app restrito: áreas/pessoas liberadas + todos da linha "cuidam" abaixo + cargo `see_all_apps` |
| **editar** a ficha | "cuidam" + cargo `edit_any_app` + acesso extra `edit` |
| **voltar versões** | "cuidam" + cargo `restore_any_app` + acesso extra `restore` |
| **ver a parte técnica** | "cuidam" + quem pode editar + cargo `see_tech_any_app` + acesso extra `tech` |
| **ver execuções de todos** | líder da área de A + cargo `see_all_runs` + acesso extra `runs` (responsável e reserva só com o acesso extra) |
| **cuidar do acesso** (aberto/restrito, quem vê, extras) | líder da área de A + cargo `manage_areas`. Responsável e reserva mudam aberto/restrito e quem vê, mas não dão acessos extras |

"Cuidam" = responsável de A, reserva de A, líder da área de A.

Quem pode editar também pode ver o técnico. Quem pode voltar versões precisa ver o técnico para achar a versão, então `restore` também libera `tech`.

O cálculo fica em **um lugar só** no backend (`internal/access`): uma função recebe a pessoa e o app e devolve o conjunto de ações permitidas. Todo handler usa essa função; o frontend recebe o conjunto pronto (`can: {...}`) junto com a ficha e com cada app do catálogo, e nunca deduz permissão sozinho.

### 3.3 Registrar um app

- Precisa do cargo `register_apps` **ou** ser líder de alguma área.
- A área do app vem da lista. Quem não é líder nem tem `edit_any_app` pode registrar em qualquer área. O app nasce aberto, e quem registrou vira o responsável, como hoje.
- **Atualizar** um app já registrado (reanalisar a pasta) exige poder editar aquele app.

## 4. Dados (migração 007)

Tabelas novas:

- `areas (id, name UNIQUE, created_at)`
- `area_leaders (area_id, person_id)` e `area_members (area_id, person_id)`
- `roles (id, name UNIQUE, permissions TEXT[], builtin BOOL)`
- `app_viewers (app_id, area_id NULL, person_id NULL)`: quem vê um app restrito; exatamente um dos dois é preenchido
- `app_grants (app_id, person_id, permission)` com `permission IN ('edit','restore','tech','runs')`. O atalho "todos os apps da área" vira `area_grants (area_id, person_id, permission)`, que vale também para apps que entrarem depois

Colunas novas:

- `people.role_id` → `roles`. A coluna `people.role` é removida depois de copiada
- `apps.area_id` → `areas` e `apps.restricted BOOL DEFAULT false`

Passagem dos dados de hoje:

1. Cria os 3 cargos prontos; `admin` → Administrador, `criador` → Criador, `usuario` → Usuário.
2. Cria uma área para cada nome distinto em `apps.area` e na lista atual de áreas do frontend (`AREAS`). Liga cada app à sua. App sem área vai para uma área "Sem área", que o admin renomeia ou troca.
3. Todos os apps ficam abertos. Ninguém é líder nem membro ainda: o admin configura na tela Equipe.

O `app-hub.yaml` continua guardando a área **pelo nome**, porque a pasta precisa fazer sentido fora do Hub. Ao salvar, o Hub grava o nome atual da área. Restrição e acessos ficam só no banco: são regras do Hub, não do app.

## 5. Telas

### 5.1 Equipe (substitui "Pessoas")

Aparece na barra lateral para quem tem `manage_people`, `manage_areas` ou `manage_roles`, ou é líder de alguma área. Três abas, cada uma só para quem pode usá-la:

- **Pessoas** (`manage_people`): lista com nome, e-mail, cargo e áreas. Cadastrar pessoa (como hoje) já escolhe o cargo. Trocar o cargo de alguém num seletor.
- **Áreas** (`manage_areas`, ou líder): uma linha por área, com líderes e membros. Criar, renomear e apagar área. Apagar só funciona com a área vazia de apps. O líder vê só as suas áreas e só coloca ou tira membros.
- **Cargos** (`manage_roles`): uma linha por cargo, com as 9 caixas da seção 3.1 com o nome em português. Criar, renomear e apagar cargo. Apagar só funciona se ninguém o usa.

### 5.2 Ficha do app (desenho B)

**Parte de uso, para todos que veem o app:**
- Cabeçalho como hoje (nome, resumo, linha de tipo · área · responsável · reserva · versão, Rodar e "salva em…"). "Editar" ao lado do resumo muda nome, resumo, tipo e área.
- **O que ele recebe**: lista de opções mostra "escolha uma" ou "marque uma ou mais" e as opções. "Editar" permite mudar nome e tipo de cada entrada, as opções (uma por linha) e "Pode marcar mais de uma". Também dá para adicionar e remover entradas.
- **O que ele devolve**: "Editar" muda descrição e padrão do arquivo, e adiciona ou remove resultados.
- **Execuções**: as suas, ou as de todos para quem pode (com o nome da pessoa).
- **Quem cuida**: responsável e reserva. "Editar" troca os dois.

**Bloco recolhido "Para quem cuida do app"** (só para quem pode ver o técnico). O resumo do bloco fechado diz "N pontos de atenção · código, outros apps, histórico, acesso":
- **Pontos de atenção**: um por linha no editar.
- **Outros apps**: "Usa: X" e "É usado por: Y", com link para cada ficha. "Editar" marca os apps do catálogo que este usa. A lista não inclui o próprio app. Quem cuida vê também os apps restritos que não pode ver, só com o nome e sem link.
- **Código-fonte** (`build`): onde está o código e como recompilar. Em app `.exe` ou de janela sem código-fonte informado, um aviso sugere preencher.
- **Chaves e senhas** e **Bibliotecas**: como hoje, sem editar (vêm da análise da pasta).
- **Histórico** e **Onde está**: como hoje. "Voltar a esta versão" aparece só para quem pode voltar versões.
- **Acesso**: "Aberto: todos veem e rodam" ou "Restrito: veem e rodam [áreas] e [pessoas]". Abaixo, os acessos extras por pessoa ("Marina: editar, ver o técnico"). "Editar" aparece para quem pode cuidar do acesso (seção 3.2). O editor dos extras é só do líder e de quem tem `manage_areas`.

**Editar por partes, regras comuns:**
- Uma parte em edição por vez. Abrir outra pergunta antes de descartar o que não foi salvo.
- Salvar manda o manifesto inteiro com só aquela parte mudada para `PUT /api/apps/{id}`. O Hub confere a permissão, valida, regrava `app-hub.yaml` e `FICHA.md` na pasta e registra o que mudou no histórico (reaproveita `manifest.Diff`).
- Se outra pessoa salvou a ficha depois que a página abriu, o Hub recusa com "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova." A conferência usa o `updated_at` do app.
- Se a pasta não puder ser lida ou escrita, nada é salvo, e aparece a mensagem de hoje ("Não consegui ler a pasta do app…").
- Erros de validação aparecem dentro da parte, em português.

### 5.3 Tela de registro

O formulário de revisão ganha os mesmos campos novos: opções e "pode marcar mais de uma" nas listas, outros apps e código-fonte. A área vira seletor da lista de áreas. Os componentes de edição são **os mesmos** da ficha, compartilhados.

### 5.4 Catálogo, Minhas execuções e o resto

- Catálogo e busca mostram só os apps que a pessoa pode ver. Um app restrito ganha um cadeado discreto para quem cuida dele.
- "Minhas execuções → De todos" aparece para quem tem `see_all_runs`. Nas fichas, "ver execuções de todos" segue a seção 3.2.
- "+ Registrar apps" aparece para quem pode registrar.
- Abrir pela URL a ficha ou a execução de um app que a pessoa não pode ver dá "App não encontrado", sem dizer que ele existe.
- O Agente continua rodando só o que a pessoa pode rodar: `POST /api/apps/{id}/runs` também confere.

## 6. Validações do manifesto (já previstas na v0.2, conferir no `Validate`)

- `multiple` só em `type: choice`. Lista de opções sem opções é recusada: "Escreva ao menos uma opção para 'X'."
- `depends_on` não contém o próprio app e só tem ids de apps do catálogo. Um id desconhecido vindo do YAML é mantido e mostrado como "app fora do catálogo"; não é recusado.
- `build.source` obrigatório quando `build` existe.

## 7. Testes

- **Go**: tabela de permissões de `internal/access` (cada linha da seção 3.2 com e sem cada fonte de permissão). Handlers: catálogo filtrado; ficha e execução de app restrito dão 404; `PUT /api/apps/{id}` recusado sem permissão, com conflito de `updated_at` e com pasta ilegível; registrar sem permissão; voltar versão sem permissão; regra do "último administrador"; migração 007 em cima de dados como os de hoje.
- **Vitest**: ficha como colega (sem bloco técnico, sem Editar), como responsável (bloco e Editar), como quem só tem `tech` (bloco sem Editar). Editar cada parte, Cancelar, erro de validação. Opções e "pode marcar mais de uma". Outros apps com "é usado por". Tela Equipe: abas por permissão, troca de cargo, caixas do cargo. Líder vê só a sua área.
- **Fotos** (Playwright, só leitura nas telas reais): ficha como admin, com o bloco aberto e uma parte em edição; tela Equipe nas 3 abas; claro, escuro e celular.
- **Teste de verdade com o Henry**: criar uma área com líder, restringir um app e conferir com um usuário de teste que o app some do catálogo dele.

## 8. Fora desta entrega

- Visibilidade por time como conta separada (empresa, vários times por pessoa com configurações próprias): o PRD 3.7 continua valendo para depois.
- Aprovação antes de publicar, limites por time.
- Código de acesso pessoal e MCP (Fase 2b): quando vier, a IA usa o mesmo `internal/access`.
- Parar um app aberto (Agente 0.2.4).
