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

## Prioridade atual (2026-09-30)

Pedido do Henry: **deixar o Hub redondo com um app só (Eq. Research Files Prep) antes de registrar mais apps.** Ordem:
1. Fase 4 para apps com janela e `.exe`: rodar pelo Hub, pegar os resultados e registrar cada execução ← **feita e testada de verdade (2026-09-30)**. Próximo: atualização automática do Agente (A), depois desenhar o Hub como app de computador (B)
2. Fase 5: histórico de mudanças no código e na ficha, e voltar atrás (junto com as execuções numa linha do tempo)
3. Acabamento: tela da ficha com os campos da v0.2 (opções, várias escolhas, `depends_on`, `build`); barra lateral não vai até o fim em páginas compridas
4. Depois: retomar a Fase 2b (teste com mais 4 apps e MCP)

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
| Catálogo em cartões com busca e filtro por área/status | In Progress | Cartões e busca simples prontos; faltam filtros por área/status |
| Favoritos por pessoa | Pending | |
| Ficha do app (visualização e edição) | In Progress | Visualização básica pronta; edição direto na ficha pendente (hoje: reanalisar a pasta e "Atualizar ficha") |

## Fase 4 — Execução (1.3), feature central

| Tarefa | Status | Notas |
|---|---|---|
| Agente do Hub no Windows (instalar, conectar, ligar com o Windows) | Done | ADR 004. `scripts/build-agent.sh`; Meu computador e Conectar. Testado de verdade em 2026-09-30 (A-HKROPP01) |
| Rodar app com janela/`.exe` pelo Agente, com pasta escolhida | Done | Padrão: pasta do app; outra pasta: cópia temporária do `.exe`. Eq. Research Files Prep rodado de verdade em 2026-09-30 na pasta do app. Janela de pastas moderna (Agente 0.1.1): falta o Henry conferir |
| Resultados agrupados pelo manifesto | Done | Arquivos novos ou alterados; grupos pelas `outputs` + "Outros arquivos". Padrões aceitam `**`; grupos calculados pela ficha atual. Ficha do Files Prep corrigida após o 1º teste (`*/*/**/*.pdf`, `*_secoes.pdf`) |
| Baixar resultados pelo Agente (arquivo e .zip) | Done | Sob demanda; só arquivos da execução. Um arquivo, grupo inteiro, ou arquivos marcados ("Baixar selecionados", pedido do Henry). Erros viram página legível |
| Atualização automática do Agente | Pending | **Próxima sessão (A).** Pedido do Henry: não depender de baixar pelo navegador. O Agente vê a versão nova no Hub, baixa e troca sozinho |
| Hub como app de computador (B) | Pending | Ideia do Henry (2026-09-30): um instalador único "App Hub" com o Agente junto, janela própria sem navegador e botão "Procurar atualizações". Precisa de desenho próprio, pensando no Módulo 2 |
| Pendências menores da revisão da Fase 4 | Pending | Cópia do `.exe` que sobra bloqueia a próxima execução; `entrypoint` com subpasta; pasta sem permissão interrompe a leitura; nomes com `..`; trava rara no download; trava de instância única por porta TCP; erro de uma consulta fica na tela; espera de 5 min na janela de pastas se o Agente caiu; apps em pasta do WSL; limpeza de pedidos de conexão vencidos |
| Registro das execuções | Done | Quem, quando, duração, pasta, computador, versão do app (data e tamanho do `.exe`); Interrompida quando o Agente reinicia ou o computador não responde |
| Executor genérico isolado da pasta original | Pending | Para scripts: roda numa cópia de trabalho; coleta arquivos novos como saídas |
| Ambiente reproduzível por app | Pending | Versão da linguagem e bibliotecas fixadas, preparado na 1ª vez |
| Formulário automático a partir do manifesto | Pending | Arquivo, data, número, texto, lista |
| Progresso ao vivo e resultado (texto, tabela, downloads) | Pending | |
| Apps web: subir e abrir; desktop/executável: abrir | Pending | |
| Repetir execução | Pending | Registro já feito (acima); falta rodar de novo com as mesmas escolhas |

## Fase 5 — Versões (1.6)

| Tarefa | Status | Notas |
|---|---|---|
| Versão 1 no registro + detecção de mudança na pasta | Pending | |
| Histórico, comparação e voltar atrás | Pending | |
| Histórico de mudanças na ficha | Pending | Quem mudou o quê e quando (pedido 2026-09-30) |
| Linha do tempo única: código + ficha + execuções | Pending | Na página do app |

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

