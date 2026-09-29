# 002 - Manifesto genérico `app-hub.yaml` salvo ao lado do app

**Status:** Accepted

## Context
O Hub precisa aceitar qualquer app (scripts, apps web, desktop, tarefas recorrentes, executáveis, documentos, sistemas), incluindo apps criados depois pela IA via MCP. Cada tela do Hub (ficha, botão Rodar, formulário, histórico) precisa de uma descrição uniforme do app.

## Decision
Cada app tem um manifesto versionado (`manifest_version: 1`) em `app-hub.yaml`, salvo dentro da pasta do app, com: id, nome, resumo, tipo, linguagem, arquivo de início, comando/endereço, bibliotecas, entradas (tipo + origem: arquivo fixo, janela de seleção, pergunta, parâmetro), saídas (padrão de arquivo), referências a segredos (arquivo e linha, **nunca o valor**), agenda, área, responsável, reserva e pontos de atenção. Ao lado, o Hub gera `FICHA.md`, legível por humanos.

O diagnóstico inicial é feito por regras (`internal/scanner`), sem IA; o que fica incerto vira pergunta para a pessoa confirmar. O banco guarda uma cópia do manifesto (JSONB) para o catálogo.

O Hub só cria ou substitui esses dois arquivos e recusa substituir um arquivo com o mesmo nome que ele não criou (marcador no cabeçalho).

## Rationale
- Um formato só para todos os tipos mantém o Hub genérico.
- YAML comentado é legível fora do Hub (requisito do PRD).
- O arquivo junto do app sobrevive a cópias da pasta e serve de contrato para o MCP.

## Trade-offs
**Pros:**
- Apps novos entram sem código novo no Hub.
- Reabrir a pasta reaproveita o manifesto (edições humanas não se perdem).

**Cons:**
- Duas fontes (arquivo e banco) podem divergir se alguém editar o arquivo à mão. A detecção de mudança entra na Fase 5 (versões).
- As regras erram em casos atípicos. Mitigado pelas perguntas de confirmação e pela IA na Fase 8.

## Alternatives Considered
- Guardar só no banco: descartado, porque o app deixaria de ser compreensível fora do Hub.
- JSON: descartado por ser menos legível e não aceitar comentários.
- Diagnóstico por IA desde já: adiado, porque a chave da API ainda não foi configurada.
