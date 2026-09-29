# TASKS — App Hub (Módulo 1, piloto local)

Status: Pending / In Progress / Done / Blocked

## Fase 0 — Planejamento

| Tarefa | Status | Notas |
|---|---|---|
| Ler especificação e apps de teste | Done | Pasta `8. Programação` analisada: scripts, Streamlit, desktop, agendados, `.exe`, documentos |
| PRD do Módulo 1 | Done | `docs/PRD.md` — aguardando validação do Henry |
| Decidir onde os apps executam (Windows vs WSL) | Pending | Apps usam `.bat`, `pythonw`, janelas `tkinter`, caminhos do Windows. Registrar em ADR |

## Fase 1 — Fundação

| Tarefa | Status | Notas |
|---|---|---|
| Esqueleto Go + React + Postgres local | Pending | Stack padrão do cofundador |
| Layout base, identidade visual, navegação | Pending | Catálogo, Registrar, Painel, Pessoas, Cofre |
| Cadastro de pessoas e login simples (protótipo) | Pending | Usuário e senha só no piloto local |

## Fase 2 — Manifesto e registro (1.1, 1.5)

| Tarefa | Status | Notas |
|---|---|---|
| Especificar o formato do manifesto (genérico, versionado) | Pending | Tipos: script, web, desktop, agendado, executável, referência. ADR |
| Diagnóstico por regras | Pending | Linguagem, ponto de entrada, `requirements.txt`/imports, arquivos fixos lidos, `tkinter`, servidores web, `.bat` |
| Detecção de segredos no código | Pending | Padrões de chave/senha; oferecer mover para o cofre |
| Varredura em lote de uma pasta | Pending | Um app por subpasta, ignorando `.venv`, `dist`, saídas |
| Tela de revisão do diagnóstico + perguntas do incerto | Pending | |
| Salvar manifesto e ficha na pasta do app | Pending | Nunca altera código |
| Aviso de duplicado | Pending | Por nome/descrição (sem IA) |

## Fase 3 — Catálogo e ficha (1.2, 1.4)

| Tarefa | Status | Notas |
|---|---|---|
| Catálogo em cartões com busca e filtro por área/status | Pending | |
| Favoritos por pessoa | Pending | |
| Ficha do app (visualização e edição) | Pending | Texto humano tem prioridade |

## Fase 4 — Execução (1.3), feature central

| Tarefa | Status | Notas |
|---|---|---|
| Executor genérico isolado da pasta original | Pending | Roda numa cópia de trabalho; coleta arquivos novos como saídas |
| Ambiente reproduzível por app | Pending | Versão da linguagem e bibliotecas fixadas, preparado na 1ª vez |
| Formulário automático a partir do manifesto | Pending | Arquivo, data, número, texto, lista |
| Progresso ao vivo e resultado (texto, tabela, downloads) | Pending | |
| Apps web: subir e abrir; desktop/executável: abrir | Pending | |
| Registro e repetição de execuções | Pending | |

## Fase 5 — Versões (1.6)

| Tarefa | Status | Notas |
|---|---|---|
| Versão 1 no registro + detecção de mudança na pasta | Pending | |
| Histórico, comparação e voltar atrás | Pending | |

## Fase 6 — Acesso e cofre (1.7)

| Tarefa | Status | Notas |
|---|---|---|
| Permissões por app (usar / editar) | Pending | |
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
| Diagnóstico e ficha escritos por IA | Blocked | Depende da chave |
| Explicação de erros em português | Blocked | Depende da chave |
| Busca em linguagem natural, "pergunte ao app" | Blocked | Depende da chave |

## Fase 9 — MCP (V2)

| Tarefa | Status | Notas |
|---|---|---|
| Servidor MCP expondo catálogo, padrões, projeto, execução, publicação | Pending | V2; operações do Hub já desenhadas para isso |
