# 017 - Aba Admin no lugar de Segurança

**Status:** Accepted

## Context
A aba Segurança misturava controles que funcionam (aprovar, suspender, pausar tudo, limpar histórico) com campos sem efeito (classificação, finalidade, domínios, "bloquear segredos embutidos"), um cofre de credenciais travado por falta de chave no servidor e um "criar outra empresa" repetido. Henry achou a página estranha e pediu um painel de administração com os apps por área, deixando o controle de cada app na página do próprio app.

## Decision
- Nova aba **Admin** (`/admin`, permissão `manage_roles`) substitui Segurança e o "Painel — em breve". Mostra: números do topo, apps esperando aprovação, apps por área com uso e erros em 30 dias, computadores da equipe, quem mais usa e atividade recente (montada das tabelas de apps, versões, revisões e pessoas, não do registro HTTP).
- Aprovar, suspender e reativar ficam numa faixa na página do app (`PUT /api/apps/{id}/control`). Quem não administra vê só por que o app não roda.
- Os controles da empresa (pausar todos, exigir aprovação, apagar histórico) ficam no fim do Admin e usam os endpoints de segurança que já existiam.
- O cofre de credenciais e os campos de governança saem da tela; o código e os endpoints continuam.

## Rationale
O administrador pensa em "meus apps", não em "política de segurança". Ações de um app pertencem à página do app, alcançável pelo catálogo ou pelo painel.

## Trade-offs
**Pros:**
- Só aparece o que tem efeito; textos em linguagem simples.
- Um lugar para ver uso e situação de todos os apps.

**Cons:**
- Classificação, finalidade, domínios e exportação JSON ficam sem tela.
- Aprovar pela página do app não pede mais "finalidade".

## Alternatives Considered
- Enxugar a página Segurança: resolvia o ruído, mas não dava a visão geral de uso que Henry quer.
- Agrupar Equipe e Servidor dentro de Admin: menu mais curto, mas mais cliques para tarefas frequentes.
