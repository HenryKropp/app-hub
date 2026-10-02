# TASKS — App Hub (Módulo 1, piloto local)

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
| Layout base, identidade visual, navegação | Done | Visual de fichário; claro/escuro/sistema; Painel e Cofre marcados "em breve" |
| Cadastro de pessoas e login simples (protótipo) | Done | ADR 003. Primeiro acesso cria o administrador; perfis admin/criador/usuário |

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
   - Etapa 2, ficha por perfil (abas Usar/Cuidar, editar por seção, campos da v0.2): ← próxima, desenho próprio
   - Etapa 3, catálogo e primeiro uso (favoritos, filtro por área, status e "usado há…", guia do Agente): depois
4. Rodar scripts: formulário automático e ambiente preparado sozinho
5. App Hub: ícone perto do relógio, janela própria e instalador único (etapas 2 e 3), antes de o primeiro colega entrar
6. Assinatura digital do App Hub (etapa 4), antes de vender
7. Depois: retomar a Fase 2b (teste com mais 4 apps e MCP)

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
| Tela de revisão: mostrar/editar `depends_on`, `build` e várias escolhas | Pending | Hoje o formato aceita e a ficha mostra; a tela de revisão ainda não edita esses campos |
| ADR: servidor MCP dentro do binário do Hub | Pending | HTTP (streamable) em `/mcp`, SDK oficial de MCP em Go; mesmas regras dos handlers |
| Código de acesso pessoal (token) | Pending | Gerado/revogado na tela Pessoas; guardado com hash; a IA age com o perfil da pessoa |
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
| Ficha do app (visualização e edição) | In Progress | Visualização básica pronta; edição direto na ficha pendente (hoje: reanalisar a pasta e "Atualizar ficha") |

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
| Empresa e times (time = área; pessoa em uma empresa só) | Pending | Combinado em 2026-09-30: fazer depois do app funcionando (registro, catálogo, execução). Área do app vira time |
| Permissões por app (usar / editar) | Pending | Visível para a empresa toda, alguns times ou pessoas escolhidas |
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

