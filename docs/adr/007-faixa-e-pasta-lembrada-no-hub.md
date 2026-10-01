# 007 - Faixa de andamento e pasta lembrada guardadas no Hub; conferência da pasta separada da ficha

**Status:** Accepted

## Context
A Etapa 1 da UX (2026-10-01) pede rodar um app em poucos cliques: botão Rodar no cartão, pasta de resultados lembrada, uma faixa embaixo da tela que acompanha o app aberto e uma página Minhas execuções. Também pede que a ficha abra na hora: a conferência da pasta (ADR 006) levava uns 2 s no OneDrive e a ficha esperava por ela, e o histórico conferia de novo logo em seguida.

## Decision
- A pasta lembrada fica no banco, por pessoa e app (`run_folders`). O pedido de rodar diz se é para lembrar (`remember: true`), esquecer (`false`) ou não mexer (ausente). Só grava depois que a execução entrou na fila.
- A faixa vem do Hub: cada execução da pessoa ainda não dispensada (`runs.dismissed_at` vazio) vira um cartão. O site pergunta `GET /api/runs/active` a cada 3 s com app aberto e a cada 30 s sem. Dispensar grava no Hub; "ocultar" é só da aba. As execuções de antes da migração 006 entram já dispensadas.
- Um único lugar no site (`RunsProvider`) dispara as execuções: decide entre abrir a janela da pasta ou rodar direto e mostra "Abrindo…" e erros na faixa.
- `GET /api/apps/{id}` deixa de conferir a pasta e devolve a última versão registrada. A ficha chama `POST /api/apps/{id}/check` logo depois de aparecer; se achar versão nova, atualiza o campo e o histórico. O histórico não confere mais. Rodar e publicar continuam conferindo antes (ADR 006).

## Rationale
- Guardar no Hub faz a pasta lembrada e a faixa valerem em qualquer navegador e computador da pessoa, e sobreviverem a recarregar a página.
- Separar a conferência tira a espera da tela sem perder o registro de versões.

## Trade-offs
**Pros:**
- A ficha aparece na hora; a conferência deixa de rodar duas vezes seguidas.
- Um clique para rodar quando a pasta está lembrada; a pessoa não sai da página.

**Cons:**
- Consulta a cada 3 s enquanto há app aberto (barato para um piloto, mas é uma consulta por aba).
- O aviso de pasta inacessível aparece um pouco depois da ficha.

## Alternatives Considered
- Guardar no navegador (`localStorage`): não vale em outro navegador nem em outro computador, e some ao limpar o navegador.
- SSE (o servidor avisa na hora): mais peças para manter; consultar a cada 3 s basta no piloto. Dá para trocar depois sem mudar a tela.
- Conferir a pasta em segundo plano no servidor, sem a ficha pedir: a ficha não saberia quando terminou para mostrar a versão nova.
