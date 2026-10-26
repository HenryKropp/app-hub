# TASKS — App Hub (Módulo 1, piloto local)

> Registro histórico do piloto. Os estados abaixo não substituem as tarefas
> atuais do projeto. Para o estado geral em 26/10/2026, consulte o
> [README](../README.md). Em particular, não considere o fluxo Windows completo
> validado nem a execução gerenciada na nuvem entregue.

Status: Pending / In Progress / Done / Blocked

## Fase 0 — Planejamento

| Tarefa | Status | Notas |
|---|---|---|
| Ler especificação e apps de teste | Done | Pasta `8. Programação` analisada: scripts, Streamlit, desktop, agendados, `.exe`, documentos |
| PRD do Módulo 1 | Done | `docs/PRD.md` |
| Decidir onde os apps executam (Windows vs WSL) | Done | ADR 001: Hub no WSL lendo `/mnt/c`; execução pelo Agente do Hub no Windows (ADR 004) |

## Fase 1 — Fundação

| Tarefa | Status | Notas |
|---|---|---|
| Esqueleto Go + React + Postgres local | Done | Banco `app-hub-db` na porta 5433; Dockerfile pronto |
| Layout base, identidade visual, navegação | Done | Estilo novo (2026-10-02): limpo e neutro como Notion/Vercel, um violeta vivo só no Rodar e no que está aberto agora; claro por padrão, com escuro; Painel e Cofre marcados "em breve" |
| Registrar apps pelo + do catálogo | Done | 2026-10-02, pedido do Henry: saiu da barra lateral; virou o botão "+ Registrar apps" ao lado da busca (só "+" no celular), só para administrador e criador. A página tem "← Catálogo" e o Catálogo fica marcado enquanto se registra |
| Configurações | Done | 2026-10-02, pedido do Henry: item no pé da barra lateral (no celular, no menu do avatar) com Meu computador e Aparência (claro, escuro, igual ao sistema). O endereço antigo `/meu-computador` leva para lá |
| Cadastro de pessoas e login simples (protótipo) | Done | ADR 003. Primeiro acesso cria o administrador; perfis admin/criador/usuário (substituídos por cargos na Etapa 2, ADR 009) |

## Fase 2 — Manifesto e registro (1.1, 1.5)

| Tarefa | Status | Notas |
|---|---|---|
| Especificar o formato do manifesto (genérico, versionado) | Done | ADR 002, `app-hub.yaml` v1 (`internal/manifest`) |
| Diagnóstico por regras | Done | `internal/scanner`; testado nos 19 apps da pasta de teste (só leitura) |
| Detecção de segredos no código | Done | Mostra arquivo e linha, nunca o valor. "Mover para o cofre" entra na Fase 6 |
| Varredura em lote de uma pasta | Done | Um app por subpasta; ignora `.venv`, `node_modules`, pastas ocultas; sugere pular pastas "Old/arquivo" |
| Tela de revisão do diagnóstico + perguntas do incerto | Done | Tela Registrar apps |
| Salvar manifesto e ficha na pasta do app | Done | Cria só `app-hub.yaml` e `FICHA.md`; recusa sobrescrever arquivo que não é do Hub |
| Aviso de duplicado | Done | Por palavras do nome, contra o catálogo e dentro do lote |

## Prioridade atual (2026-10-01)

Pedido do Henry: **deixar o Hub redondo com um app só (Eq. Research Files Prep) antes de registrar mais apps.** Fase 4 para apps com janela e `.exe` feita e testada de verdade (2026-09-30). Ordem combinada depois da decisão do App Hub (ADR 005):
1. ~~Atualização automática do Agente~~ Feita e testada de verdade (2026-09-30)
2. ~~**Fase 5:** histórico de mudanças no código e na ficha, e voltar atrás~~ Feita e testada de verdade no Files Prep (2026-10-01)
3. **UX em 3 etapas** (desenho: `docs/superpowers/specs/2026-10-01-etapa-1-rodar-poucos-cliques-design.md`; protótipo: `2026-10-01-prototipo-ux.html`):
   - **Etapa 1, rodar em poucos cliques:** **feita** (código 2026-10-01; telas e teste de verdade com o Henry 2026-10-02). Ficha abre na hora, Rodar no cartão, pasta lembrada, faixa de andamento, Minhas execuções
   - ~~**Estilo visual novo do zero**~~ **feito** (2026-10-02). O Henry escolheu, depois de dois sorteios, o padrão Notion/Vercel com um violeta vivo de destaque (como o verde do Spotify). Letra Geist; barra lateral clara; Rodar como botão violeta arredondado; barrinhas violeta animadas no que está aberto agora; tipos de app em etiqueta cinza. Todas as telas trocadas; no celular, menu da conta (tema e Sair) no avatar. `DESIGN.md` gravado. De quebra: o aviso "Abrindo…" às vezes ficava até 3 s junto com o cartão da execução; corrigido. Decisão em `docs/adr/008-estilo-visual-neutro-com-violeta.md`
   - **Acabamento da Etapa 1** (avaliação Impeccable do catálogo em 2026-10-02, nota 25/40; relatório em `.impeccable/critique/`): ← próxima, já no estilo novo. O Henry escolheu corrigir tudo o que foi achado, começando por "onde ficam os arquivos":
     1. ~~Onde ficam os arquivos~~ **feito** (2026-10-02): "Salva em … · trocar" no pé do cartão (nome da pasta; caminho inteiro ao passar o mouse), "Salva em …" na faixa desde o "Abrindo…", e o caminho inteiro na ficha e na janela. Janela sem pasta padrão: na 1ª vez começa vazia com "Escolher pasta…"; depois lembra
     2. ~~Faixa~~ **feito** (2026-10-02): ordena antes de cortar em 3 (erro e interrompido > terminou > sem arquivos > aberto/abrindo), então o que fica no "+N" são os apps ainda abertos; a faixa fica sempre montada como região que o leitor de tela anuncia, e os erros são alerta; "Ver e baixar" já estava legível no escuro com o estilo novo (conferido em foto)
     3. ~~Mensagens~~ **feito** (2026-10-05): "sem arquivos novos" cinza e explicado na faixa e na página da execução; erros do navegador ("Failed to fetch") e respostas 5xx viram português claro; o erro do Windows ao abrir o app vira uma frase simples (o detalhe técnico fica no `hub.log`); "Tentar de novo" no catálogo, Minhas execuções, ficha e página da execução
     4. ~~Saídas~~ **feito** (2026-10-05), sem o Parar: Cancelar na janela de rodar; "Ver" na faixa enquanto o app está aberto (não tira da faixa); depois de "Ocultar", um lembrete "N aberto oculto · Mostrar". **Parar** ficou para depois, a pedido do Henry: precisa do Agente 0.2.4 e de teste de verdade
     5. ~~Acabamento~~ **feito** (2026-10-05): busca com nome para leitor de tela, X e "Limpar busca", Esc limpa; "Fechar" em vez de "Close"; "Em breve" só para administrador e criador; catálogo vazio com texto próprio para quem só usa; link "Pular para o conteúdo"; contador "abertos agora" no plural; no celular a faixa mostra um cartão só e conta o resto. O selo "Janela", as legendas, o vermelho do título e o contador vermelho já tinham sido resolvidos pelo estilo novo (conferido em foto). Fotos: `e2e/acabamento.mjs` (só leitura)
   - **Parar um app aberto pelo Hub** (pendente, pedido do Henry em 2026-10-05): comando novo no Agente (0.2.4), botão Parar na faixa e na página da execução, teste de verdade no PC do Henry
   - Ficha mais limpa (2026-10-02, pedido do Henry): quadro de 5 caixas virou uma linha discreta abaixo do resumo; entradas dizem uma vez de onde vêm ("Você preenche tudo na janela do app"); saídas sem o padrão técnico (fica no passar o mouse); pontos de atenção mostram 3 e "Ver mais N"; histórico mostra os 5 mais novos e "Ver histórico completo", com filtro em abas pequenas e ações das versões discretas; pasta sem fonte de código
   - ~~**Etapa 2, cargos, permissões e ficha editável**~~ **feita** (2026-10-05; desenho: `docs/superpowers/specs/2026-10-05-etapa-2-permissoes-e-ficha-design.md`; ADR 009). As abas Usar/Cuidar foram descartadas; a ficha ficou no desenho B:
     1. **Done:** cargos personalizáveis (Administrador, Criador, Usuário prontos) com 9 permissões; áreas com líderes e membros; acessos extras por app ou área; app aberto ou restrito (quem não vê recebe "App não encontrado"); regra única em `internal/access`; migração 007
     2. **Done:** tela Equipe (Pessoas, Áreas, Cargos); ficha no desenho B com bloco recolhido "Para quem cuida do app" e "Editar" por parte, inclusive os campos do formato v0.2 (opções, "pode marcar mais de uma", outros apps, código-fonte); registro com os mesmos editores; fotos em `e2e/etapa2.mjs` (só leitura)
     3. **Done:** correções da revisão final (2026-10-05): ninguém cadastra com cargo maior que o seu nem muda o cargo de quem tem mais; cargos da empresa toda não revelam app restrito sem "ver todos os apps"; só quem cuida do acesso troca responsável, reserva e área; acessos de app e de área não se apagam; Equipe mais clara no celular
     4. **Pending:** teste de verdade com o Henry (criar área com líder, restringir um app e conferir com um usuário de teste que ele some do catálogo)
     5. **Pending (fora desta etapa):** **Parar** um app aberto (Agente 0.2.4); visibilidade por time como conta separada; aprovação antes de publicar; código de acesso pessoal e MCP
   - Etapa 3, catálogo e primeiro uso (favoritos, filtro por área, status e "usado há…", guia do Agente): **Pending**, depois da Etapa 2
4. Rodar scripts: formulário automático e ambiente preparado sozinho
5. App Hub: ícone perto do relógio, janela própria e instalador único (etapas 2 e 3), antes de o primeiro colega entrar
6. Assinatura digital do App Hub (etapa 4), antes de vender
7. Depois: retomar a Fase 2b (teste com mais 4 apps e MCP)
8. **A decidir: apps para vários ao mesmo tempo** (dúvida do Henry, 2026-10-02). Ideia no PRD 3.3: opção "pessoal" ou "compartilhado" no manifesto; o compartilhado roda uma vez só no servidor central, com um banco de dados só e a senha no cofre. Decidir junto com "Rodar apps web" e antes do primeiro colega usar um app desses
9. **Ideia: compartilhar skills do Claude pelo Hub** (Henry, 2026-10-26). Ver PRD 3.12. Antes de desenhar: pesquisar como publicar skills no claude.ai
10. **Ideia: gestão de gastos com IA** (Henry, 2026-10-26). Ver PRD 3.13. O nível 2 (chave centralizada no Hub) é o que mais vale

**Para a próxima sessão (deixado em 2026-10-01, à noite):**
- Etapa 1: Hub da 8080 reiniciado em 2026-10-02 (migração 006 aplicada, `DEV_MODE=1`, registro em `backend/hub.log`). Telas conferidas com `e2e/etapa1.mjs` (só leitura; faixa e pasta lembrada simuladas): catálogo com Rodar, janela com "Lembrar esta pasta", faixa, Minhas execuções e "De todos", ficha com "salva em … · trocar", claro/escuro/celular. A barra lateral acompanha a rolagem. Sobras visuais: no celular, 3 cartões na faixa cobrem quase meia tela; o espaço embaixo da página (`pb-32`) é menor que a faixa com 3 cartões.
- Teste de verdade no Files Prep com o Henry (2026-10-02), conferido nos dados do Hub: sem faixa ao abrir; 1ª vez pela janela com "Lembrar esta pasta" (pasta gravada); dispensar com ✕ não volta no F5; 2ª vez direto do cartão, sem janela; Rodar de novo por Minhas execuções; "terminou: 4 arquivos novos" → Ver e baixar abre a página e tira a faixa. Execuções 3 a 5 deram "sem arquivos novos" porque o app foi fechado antes de baixar algo novo (comportamento certo).
- Observar: na 1ª tentativa o ✕ sumiu com a faixa, mas a dispensa não chegou ao Hub (não se repetiu). A faixa "sem arquivos novos" usa o ícone vermelho de alerta, que parece erro para um resultado normal.
- Seguir o fluxo: desenho → plano → implementação para a Etapa 2.
- O Agente no PC do Henry está na **0.2.3** e se atualiza sozinho. O código do Hub também está na 0.2.3, e `dist/agent/AppHubAgente.exe` da pasta principal já é essa versão.
- Para publicar um Agente novo: subir `agentproto.Version`, rodar `scripts/build-agent.sh` e reiniciar o Hub.
- Nunca ligar um Hub de teste (com outro banco) na porta 8080: o Agente real fala com ele e esquece a conexão.
- Ainda falta o Henry conferir a janela de pastas moderna (item da Fase 4).

## Fase 2b — Registro pelo Claude via MCP (3.11) — pausada

Antecipada a pedido do Henry (2026-09-30), pausada no mesmo dia em favor da prioridade acima. Primeiro o protocolo, testado localmente; depois o MCP. Substitui a necessidade de chave da API para o diagnóstico por IA.

| Tarefa | Status | Notas |
|---|---|---|
| Protocolo de registro pelo Claude | In Progress | `docs/PROTOCOLO-REGISTRO.md` v0.2 (8 melhorias da rodada 1 aprovadas); vira as instruções do MCP |
| Teste local do protocolo em 4 apps da Alis | In Progress | Rodada 1 (Eq. Research Files Prep, app extra) aprovada. Faltam: Python Rentabilidade fundos, Research Platform, transcritor-calls, CVM Downloader (decidir aí a relação com o Files Prep) |
| Formato v0.2 do manifesto | Done | Origem `window`, `multiple`, `depends_on`, `build`, todos opcionais; comando `cmd/validate-manifest` para validar rascunhos |
| Tela de revisão: mostrar/editar `depends_on`, `build` e várias escolhas | Done | Etapa 2 (2026-10-05): a tela de registro usa os mesmos editores da ficha |
| ADR: servidor MCP dentro do binário do Hub | Pending | HTTP (streamable) em `/mcp`, SDK oficial de MCP em Go; mesmas regras dos handlers |
| Código de acesso pessoal (token) | Pending | Gerado/revogado na tela Equipe (aba Pessoas); guardado com hash; a IA age com o perfil da pessoa |
| Ferramenta: listar catálogo e ler ficha | Pending | Para achar duplicados antes de registrar |
| Ferramenta: formato do manifesto e regras da empresa | Pending | Esquema + explicação em português (padrões 1.5) |
| Ferramenta: listar pessoas | Pending | Para escolher responsável e reserva |
| Ferramenta: varrer pasta e diagnóstico por regras | Pending | Reaproveita `internal/scanner` como ponto de partida |
| Ferramenta: ler arquivos de um app com segredos mascarados | Pending | Só dentro da pasta registrada/varrida; limite de tamanho; nunca devolve valor de chave |
| Ferramenta: registrar/atualizar app | Pending | Mesmo caminho do botão Publicar; marca "via IA" (coluna nova) |
| Marca "via IA" no catálogo e na ficha | Pending | |
| Instruções de conexão (Claude Code e Claude Desktop) | Pending | Tela "Conectar o Claude" com o comando pronto |
| Testes (Go) + teste de ponta a ponta com um cliente MCP | Pending | |

## Fase 3 — Catálogo e ficha (1.2, 1.4)

| Tarefa | Status | Notas |
|---|---|---|
| Catálogo em cartões com busca e filtro por área/status | In Progress | Cartões, busca simples e botão Rodar no cartão prontos (Etapa 1 da UX); faltam filtros por área/status (Etapa 3) |
| Favoritos por pessoa | Pending | |
| Ficha do app (visualização e edição) | Done | Etapa 2 (2026-10-05): desenho B, "Editar" por parte, grava `app-hub.yaml` e `FICHA.md` e registra no histórico |

## Fase 4 — Execução (1.3), feature central

| Tarefa | Status | Notas |
|---|---|---|
| Agente do Hub no Windows (instalar, conectar, ligar com o Windows) | Done | ADR 004. `scripts/build-agent.sh`; Meu computador e Conectar. Testado de verdade em 2026-09-30 (A-HKROPP01) |
| Rodar app com janela/`.exe` pelo Agente, com pasta escolhida | Done | Padrão: pasta do app; outra pasta: cópia temporária do `.exe`. Eq. Research Files Prep rodado de verdade em 2026-09-30 na pasta do app. Janela de pastas moderna (Agente 0.1.1): falta o Henry conferir |
| Resultados agrupados pelo manifesto | Done | Arquivos novos ou alterados; grupos pelas `outputs` + "Outros arquivos". Padrões aceitam `**`; grupos calculados pela ficha atual. Ficha do Files Prep corrigida após o 1º teste (`*/*/**/*.pdf`, `*_secoes.pdf`) |
| Baixar resultados pelo Agente (arquivo e .zip) | Done | Sob demanda; só arquivos da execução. Um arquivo, grupo inteiro, ou arquivos marcados ("Baixar selecionados", pedido do Henry). Erros viram página legível |
| Atualização automática do Agente | Done | 2026-09-30, testada no PC do Henry (0.2.0 → 0.2.1 sozinho; 0.2.2 com defeito voltou 3 vezes e o Hub parou; 0.2.3 entrou). Para publicar: subir `agentproto.Version`, rodar `scripts/build-agent.sh` e reiniciar o Hub. Sem número novo, nada atualiza. A 0.1.x precisa de uma última instalação manual |
| App Hub: ícone, janela própria e instalador único (B) | Pending | Decidido na ADR 005 (2026-09-30): o Agente vira o App Hub, mostrando as mesmas telas do site numa janela, com o Hub sempre num servidor central. Etapas 2 e 3, antes do primeiro colega |
| Assinatura digital do App Hub | Pending | Etapa 4, antes de vender (certificado pago) |
| Pendências menores da atualização automática | Pending | "Atualizando…" aparece enquanto o Agente espera um app fechar (pode durar horas); espera pelo app não é cancelada se o Agente fechar; uma execução nova pode passar na frente da atualização; `install()` chama `/quit` sem limite de tempo; poucos testes para falha ao abrir a versão nova e para download 404/401; uma atualização bem-sucedida antiga pode virar "falha" se outra entrar em menos de 10 min (só histórico) |
| Pendências menores da revisão da Fase 4 | Pending | Cópia do `.exe` que sobra bloqueia a próxima execução; `entrypoint` com subpasta; pasta sem permissão interrompe a leitura; nomes com `..`; trava rara no download; trava de instância única por porta TCP; erro de uma consulta fica na tela; espera de 5 min na janela de pastas se o Agente caiu; apps em pasta do WSL; limpeza de pedidos de conexão vencidos |
| Registro das execuções | Done | Quem, quando, duração, pasta, computador, versão do app (data e tamanho do `.exe`); Interrompida quando o Agente reinicia ou o computador não responde |
| Executor genérico isolado da pasta original | Pending | Para scripts: roda numa cópia de trabalho; coleta arquivos novos como saídas |
| Ambiente reproduzível por app | Pending | Versão da linguagem e bibliotecas fixadas, preparado na 1ª vez |
| Formulário automático a partir do manifesto | Pending | Arquivo, data, número, texto, lista |
| Progresso ao vivo e resultado (texto, tabela, downloads) | Pending | |
| Apps web: subir e abrir; desktop/executável: abrir | Pending | |
| Repetir execução | In Progress | Apps com janela: **Rodar de novo** na mesma pasta, em Minhas execuções e na página da execução (Etapa 1 da UX). Scripts: falta repetir com as mesmas entradas |
| Rodar em poucos cliques (Etapa 1 da UX) | Done | Feito: Rodar no cartão, pasta lembrada por pessoa e app ("Lembrar esta pasta"), faixa de andamento embaixo (sobrevive a recarregar), Minhas execuções (50 por vez, "De todos" para admin), ficha abre sem esperar a pasta (ADR 007). Telas e teste de verdade com o Henry feitos em 2026-10-02 |

## Fase 5 — Versões (1.6)

| Tarefa | Status | Notas |
|---|---|---|
| Versão 1 no registro + detecção de mudança na pasta | Done | Versão 1 na primeira conferência; conferência ao abrir a ficha e o histórico, antes de rodar e ao publicar. ADR 006 |
| Histórico, comparação e voltar atrás | Done | Página da versão (O que mudou / Como estava); voltar com prévia, versão de segurança e lixeira. Testado de verdade no Files Prep (2026-10-01) |
| Histórico de mudanças na ficha | Done | Campo por campo, só registrado (não se restaura) |
| Linha do tempo única: código + ficha + execuções | Done | Seção Histórico na ficha, com filtro e "Carregar mais"; execução mostra a versão usada |
| Sobras da Fase 5 | Pending | Manifesto do Files Prep não lista os `.zip` que o app baixa (entram no histórico como binários); limpar versões antigas; página da versão: respostas fora de ordem ao trocar de versão, erro esconde a página inteira, sem teste de quem vê Descrever/Voltar; leitura travada do OneDrive dentro da trava do app pode segurar a ficha; erro ao gravar a versão depois de voltar devolve um erro genérico; a máscara de chaves esconde um pouco a mais (ex.: `token = settings.TOKEN`); um arquivo que some no meio da conferência faz a conferência falhar; duas restaurações no mesmo segundo usam a mesma lixeira; a primeira conferência lê todos os arquivos, inclusive os que estão só na nuvem |

## Fase 6 — Acesso e cofre (1.7)

| Tarefa | Status | Notas |
|---|---|---|
| Empresa e times (time = área; pessoa em uma empresa só) | In Progress | Áreas com líderes e membros feitas na Etapa 2 (ADR 009). Falta a empresa como conta, vários times com configurações próprias, limites e aprovação antes de publicar |
| Permissões por app (usar / editar) | Done | Etapa 2 (2026-10-05, ADR 009): cargos, líderes de área, acessos extras e app restrito. Parar um app aberto (Agente 0.2.4) segue pendente |
| Cofre local criptografado + injeção na execução | Pending | |
| Registro de uso de chaves | Pending | |

## Fase 7 — Saúde, continuidade e painel (1.8, 1.9, 1.10)

| Tarefa | Status | Notas |
|---|---|---|
| Status por app a partir das execuções | Pending | |
| Responsável/reserva obrigatórios + lista de apps em risco | Pending | |
| Painel: inventário, uso, risco | Pending | |

## Fase 8 — IA

| Tarefa | Status | Notas |
|---|---|---|
| Configurar chave da Claude API | Blocked | Decisão adiada pelo Henry |
| Diagnóstico e ficha escritos por IA | Pending | Resolvido pelo MCP (Fase 2b) para o registro; dentro do Hub ainda depende da chave |
| Explicação de erros em português | Blocked | Depende da chave |
| Busca em linguagem natural, "pergunte ao app" | Blocked | Depende da chave |

## Fase 9 — MCP completo

| Tarefa | Status | Notas |
|---|---|---|
| MCP: criar app do zero, rodar, ler saúde, usar cofre | Pending | Depois das Fases 4–7; registro já coberto na Fase 2b |

## Apps compartilhados (2026-10-06, ADR 010)


| Tarefa | Status | Notas |
|---|---|---|
| Tasks 1 a 18 (manifesto, cofre, motor Podman, bancos por app, receitas, checagem, implantação, porteiro, vigia, API da ficha, uso e custo, catálogo, parte "Compartilhado" da ficha, página Servidor e gastos) | Done | Juntado à versão principal (`main`) em 2026-10-26, com todos os testes passando |
| Task 19: ADR 010, PRD, TASKS, INFRASTRUCTURE e script de fotos (`e2e/compartilhados.mjs`) | Done | As fotos só rodam depois que o Henry reiniciar o Hub com o código novo |
| Teste de ponta a ponta com o Podman de verdade (2026-10-07, num Hub de teste separado) | Done | 12 de 12 passos passaram. Os 5 defeitos achados foram corrigidos: parada do app levava 10 s (o app não recebia o sinal de parar), o vigia religava depois de uma só checagem de saúde que falhava (agora pede 2 seguidas e confere de novo antes de religar), disco "usado" contava os blocos reservados, `PUT /api/server/cost` com corpo errado gravava zero, e a resposta de `POST /api/apps` vinha sem permissões nem versão |
| Task 19: teste de verdade com o Henry (Streamlit de exemplo, testar, publicar, descansar, voltar versão, custo, checagem do Sistema Alis V2) | Pending | Um passo por vez, conferindo pelos dados do Hub |

### Depois
- Subir na nuvem (por último).
- Sistema Alis V2 para a equipe (antes, só a checagem).
- Linha "Versão nova pronta para publicar" na ficha (spec 4.3).
- Nome da versão no ar montado a partir da pasta publicada (hoje vem do número da implantação).
- Cofre: amarrar cada segredo ao seu app (dados associados por app) e permitir trocar a chave (`HUB_SECRET_KEY`).
- Uso do computador (`hostinfo`) fora do Linux: hoje a página Servidor e gastos mostra zero em outros sistemas.
- Fotos do porteiro (páginas "acordando", "entre de novo", "teste encerrado") no script `e2e/compartilhados.mjs`.
- Rodar os testes com `-race` num computador com `gcc`.
- Pequenas sobras guardadas na revisão (baixa prioridade):
  - Motor: `Fake` com `sync.Mutex` sobrando; `Inspect`/`Logs`/`Exec` aceitam caixa desconhecida; "no such image" vira "não encontrado"; motor fora do ar soma até 10 s por comando.
  - Bancos: testes do `appdb` mais firmes (erros ignorados, sem teste de senha errada); `Fake` aceitar só nomes válidos; bancos de teste também trancados; `LockDown` tira `CONNECT` de `PUBLIC` no banco do Hub a cada início (avisar o Henry).
  - Receitas e checagem: subpasta no módulo (`src/app.py`), imports separados por vírgula, `requirements.txt` com comentários, `process.env["X"]`, arquivos acima de 1 MB ignorados sem aviso, porta 0 sem `EXPOSE`.
  - Montagem: `.dockerignore` simples (sem `**`); `Wait` pode passar do tempo em até 5 s.
  - Implantação: "Voltar para a anterior" não marca acesso (pode descansar logo); `Unshare` ignora erros ao remover caixa e imagem (registrar); `deploy.go` grande (separar); imagens de montagens que falharam ficam para trás.
  - Porteiro: `stripCookie` também tirar `hub_session`; vigia de WebSocket com tempo-limite; "fail closed" contar do primeiro erro; testes de não acordar em pedido cancelado e da página de teste encerrado.
  - Vigia: app que caiu depois de "Ligar" manual não é tentado de novo.
  - Telas: "Abrir" aparece também para app parado ou que caiu; ficha guarda informação velha sem avisar quando um GET falha depois; quem só vê o app recebe erro em vez da linha com "Abrir" se a primeira leitura falha; `hostinfo` com erro dá custo zero sem aviso (falta `host_ok`); testes de permissão e de "Ligar" na página Servidor e gastos.
