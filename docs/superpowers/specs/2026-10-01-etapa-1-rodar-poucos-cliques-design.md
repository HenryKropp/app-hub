# Etapa 1 da UX: rodar em poucos cliques (desenho)

**Data:** 2026-10-01 · **Status:** desenho aprovado em conversa; aguardando revisão do Henry
**Contexto:** item 3 ("Acabamento") da prioridade atual em `docs/TASKS.md`. Antes de mexer, o Henry pediu para planejar a UX do Hub inteiro, para as duas jornadas (colega que só usa e criador). Protótipo clicável com 12 apps da pasta de programação (`2026-10-01-prototipo-ux.html`, abre direto no navegador) aprovado como direção ("esse é o caminho").

## 1. Objetivo e princípio

**Rodar em poucos cliques; detalhes só para quem quiser aprofundar.** O colega acha o app, clica em Rodar e recebe os arquivos, sem passar pela ficha. Quem quer entender ou cuidar do app entra na ficha.

### O desenho inteiro, em 3 etapas

| Etapa | O que entra | Status |
|---|---|---|
| **1. Rodar em poucos cliques** | Ficha abre na hora, barra lateral até o fim, Rodar no cartão, pasta lembrada, faixa de andamento, Minhas execuções | **Este documento** |
| 2. Ficha por perfil | Abas **Usar** (todos) e **Cuidar** (responsável, reserva e admin); editar por seção, inclusive os campos da v0.2 (opções, várias escolhas, `depends_on`, `build`) | Próxima, desenho próprio |
| 3. Catálogo e primeiro uso | Favoritos, filtro por área, status e "usado há…" no cartão; guia do Agente quando um colega sem Agente clica em Rodar | Depois, desenho próprio |

O formulário automático para scripts continua no item seguinte da prioridade ("Rodar scripts"), fora destas etapas.

### Decisões do Henry (2026-10-01)

| Tema | Decisão |
|---|---|
| Para quem | As duas jornadas, colega e criador |
| Onde rodar | Botão Rodar direto no cartão do catálogo |
| Onde salvar | Na 1ª vez pergunta, com a opção "Lembrar esta pasta"; lembrada, abre direto |
| Enquanto o app está aberto | Faixa de andamento embaixo; a pessoa continua onde estava |
| Ficha | Duas abas Usar/Cuidar (Etapa 2) |
| Editar a ficha | Por seção, cada bloco com seu "editar" (Etapa 2) |
| Ordem | Etapas 1 → 2 → 3 |

## 2. Como fica para a pessoa

### 2.1 Ficha abre na hora
- A ficha aparece assim que a página abre, com o que o Hub já sabe (manifesto, responsável, última versão registrada).
- A conferência da pasta (ADR 006) continua acontecendo ao abrir a ficha, mas por trás. Se ela achar mudança no código, a versão nova aparece sozinha no campo Versão e no Histórico ("versão 4 · agora"), sem recarregar.
- Se a pasta não estiver acessível, o aviso de hoje aparece quando a conferência termina.

### 2.2 Barra lateral até o fim
- Conferir num navegador de verdade (não só em captura de página inteira) se a barra acompanha a rolagem em páginas compridas, como a ficha do Files Prep. Corrigir se não acompanhar.

### 2.3 Rodar no cartão
- Todo cartão de app que pode rodar pelo Hub ganha um botão **Rodar** no rodapé. Clicar no resto do cartão continua abrindo a ficha. Apps que não rodam (documento, sistema complexo) não ganham botão nesta etapa.
- **1ª vez da pessoa naquele app:** abre a janela "Onde salvar os resultados?" (a de hoje), com a pasta do app preenchida, "Escolher outra…" e a caixinha **Lembrar esta pasta**, que vem marcada.
- **Com pasta lembrada:** Rodar abre o app direto, sem janela. Um clique.
- **Na ficha:** ao lado do Rodar, o texto "salva em `<pasta>` · trocar". "trocar" abre a mesma janela, já com a pasta lembrada. Sem pasta lembrada, só o Rodar.
- A pasta lembrada é **de cada pessoa, por app**, e fica guardada no Hub (vale em qualquer navegador e computador da pessoa).
- Desmarcar "Lembrar esta pasta" faz o Hub esquecer a pasta lembrada daquele app; da próxima vez pergunta de novo.

### 2.4 Faixa de andamento
- Ao clicar Rodar (no cartão, na ficha, em Minhas execuções ou na página da execução), aparece uma faixa fixa embaixo da tela, e a pessoa fica na página onde estava.
- Textos, por momento:
  - Pedido enviado, Agente ainda não pegou: "Abrindo **Files Prep** no seu computador…"
  - App aberto: "**Files Prep** aberto no seu computador. Use a janela e feche quando terminar."
  - Concluída: "✓ **Files Prep** terminou: 4 arquivos novos" + botão **Ver e baixar**.
  - Sem arquivos novos: "**Files Prep** terminou sem arquivos novos" + **Ver detalhes**.
  - Interrompida: "**Files Prep** foi interrompido: o computador parou de responder" + **Ver detalhes**.
  - Erro: "Não deu para abrir o **Files Prep**: `<mensagem>`" + **Ver detalhes**; se for o Agente desligado, **Ver Meu computador**.
- Enquanto o app está aberto, a faixa tem **ocultar** (some da tela, a execução continua e reaparece quando terminar). Depois que termina, tem **✕** (dispensar de vez). Clicar em Ver e baixar também dispensa.
- A faixa vem do Hub, não do navegador: sobrevive a recarregar a página e aparece igual em outra aba. Cada execução da pessoa ainda não dispensada vira uma faixa; várias ficam empilhadas (no máximo 3 visíveis, mais um "+N" que leva a Minhas execuções).
- Execuções antigas, de antes desta etapa, não aparecem como faixa.
- Erro ao pedir para rodar (ex.: nenhum computador conectado) aparece na própria faixa, sem abrir página.

### 2.5 Minhas execuções
- Item novo **Minhas execuções** na barra lateral, para todos os perfis, entre Catálogo e Registrar apps. Mostra um contador quando há app aberto agora.
- Página `/execucoes`: lista das execuções da pessoa, mais novas primeiro, 50 por vez com "Carregar mais". Cada linha: app, quando, quanto durou, resultado (mesmos textos de hoje) e número de arquivos; **Ver e baixar** (abre a página da execução) e **Rodar de novo**.
- **Rodar de novo** usa a mesma pasta daquela execução, sem perguntar, e não muda a pasta lembrada.
- **Administrador** ganha um seletor "Minhas / De todos". "De todos" mostra também quem rodou.
- **Página da execução:** ganha o botão **Rodar de novo** e o "voltar" leva a Minhas execuções (hoje leva à ficha). O nome do app vira link para a ficha.

## 3. Como funciona

### 3.1 Ficha rápida
- `GET /api/apps/{id}` deixa de chamar a conferência da pasta. Devolve a última versão registrada e `folder_readable` como desconhecido.
- Endpoint novo `POST /api/apps/{id}/check`: roda a conferência (a mesma `checkVersion` de hoje) e devolve `{folder_readable, version}`. A ficha chama logo depois de abrir e atualiza o campo Versão e o Histórico se a versão mudou.
- A conferência antes de rodar e ao publicar continua como está (ADR 006): a execução precisa da versão certa. Rodar pode levar os mesmos ~2 s de hoje no servidor, mas a faixa aparece na hora ("Abrindo…"), então a pessoa não espera olhando para nada.
- O Histórico (`GET /api/apps/{id}/history`) também deixa de conferir a pasta; ele é recarregado depois do `check` se a versão mudou. Isso resolve a sobra da Fase 5 "a ficha e o histórico conferem a pasta duas vezes seguidas".

### 3.2 Pasta lembrada
- Migração nova `006_run_prefs.sql`: tabela `run_folders (person_id, app_id, folder, updated_at)`, chave primária `(person_id, app_id)`, apagada em cascata com a pessoa ou o app.
- `GET /api/apps` e `GET /api/apps/{id}` passam a devolver, para a pessoa logada, `can_run` (mesma regra `canRun` de hoje) e `remembered_folder` (ou vazio).
- `POST /api/apps/{id}/runs` ganha o campo `remember` (verdadeiro/falso, opcional). Verdadeiro: grava a pasta usada em `run_folders`. Falso: apaga a pasta lembrada. Ausente (Rodar direto, Rodar de novo): não mexe.
- Rodar sem janela manda `output_folder` = pasta lembrada. Se a pasta não servir mais (Agente responde erro), o erro aparece na faixa, com **trocar pasta**.

### 3.3 Faixa
- `runs` ganha a coluna `dismissed_at TIMESTAMPTZ` (mesma migração). Execuções já existentes ficam marcadas como dispensadas na migração, para não virar faixa.
- `GET /api/runs/active`: execuções da pessoa logada com `dismissed_at` vazio, com app, situação, número de arquivos e mensagem de erro. Limite de 10.
- `POST /api/runs/{id}/dismiss`: só a pessoa que rodou; grava `dismissed_at`. Abrir a página da execução pela faixa (Ver e baixar / Ver detalhes) também dispensa.
- "ocultar" (enquanto está aberto) é só do navegador, por aba; não grava nada.
- Um componente da faixa fica no `Layout`, em todas as páginas logadas. Pergunta `GET /api/runs/active` a cada 3 s enquanto existe execução aberta, a cada 30 s quando não, e na hora depois de um Rodar. Mesmo ritmo de consulta que a página da execução usa hoje.
- Um único lugar dispara a execução no navegador (função `rodar(app, opções)`), usado pelo cartão, pela ficha, por Minhas execuções e pela página da execução. Ela decide se abre a janela da pasta ou roda direto, chama a API e avisa a faixa.

### 3.4 Minhas execuções
- `GET /api/runs?scope=mine|all&before=<id>`: lista paginada (50), mais novas primeiro, com nome do app, quem rodou, situação, datas, número de arquivos e pasta. `scope=all` só para admin (os outros recebem 403).
- O contador da barra lateral usa a mesma resposta de `GET /api/runs/active` (quantas estão aguardando ou rodando).
- Quem vê uma execução continua igual (PRD 3.3: quem rodou e administradores).

## 4. Erros e casos de borda
- **Agente desligado ou não conectado:** a faixa mostra o erro com **Ver Meu computador**. (O guia de instalação no primeiro Rodar é da Etapa 3.)
- **Pasta lembrada sumiu ou sem permissão:** erro na faixa com **trocar pasta**, que abre a janela com a pasta do app.
- **Duas abas rodando o mesmo app:** cada clique é uma execução; o Agente já trata a fila. A faixa mostra as duas.
- **Pasta do app inacessível na hora de rodar:** mesmo comportamento de hoje, mas o erro aparece na faixa.
- **Conferência em segundo plano falha:** a ficha continua aparecendo; só o aviso de pasta inacessível aparece.
- **Execução apagada junto com o app:** some da faixa e da lista (cascata que já existe).

## 5. Testes
- **Go:** `GET /api/apps/{id}` não confere a pasta; `POST /check` confere e registra versão nova; pasta lembrada gravada, apagada e intacta conforme `remember`; `runs/active` só da pessoa e sem as dispensadas; `dismiss` só por quem rodou; `GET /api/runs` com `scope=all` negado a não-admin; paginação.
- **Vitest:** cartão com e sem Rodar; 1ª vez abre a janela, com pasta lembrada roda direto; caixinha "Lembrar"; faixa em cada situação, ocultar e dispensar; Minhas execuções com Rodar de novo; seletor "De todos" só para admin; ficha aparece antes do `check` responder.
- **Visual:** capturas do catálogo com Rodar, faixa aberta e concluída, Minhas execuções, ficha com "salva em … · trocar", no claro, no escuro e no celular. Barra lateral conferida rolando de verdade.
- **Teste de verdade com o Henry** no Files Prep, passo a passo: 1ª vez com Lembrar, 2ª vez direto do cartão, faixa até "Ver e baixar", Rodar de novo por Minhas execuções.

## 6. Fora desta etapa
- Abas Usar/Cuidar e editar por seção (Etapa 2).
- Favoritos, filtro por área, status e "usado há…" no cartão, guia do Agente no primeiro Rodar (Etapa 3).
- Formulário para scripts, apps no navegador, "Rodar agora" de tarefas automáticas (item "Rodar scripts" e seguintes).
- Aviso do Windows quando termina.
