# 036 - MCP: `problemas`, uma lista só do que deu errado (F3a)

**Status:** Accepted

## Context
O Henry quer que o Claude veja sozinho os erros do Ziv e reaja (adendo 2 do plano de 2026-11-11). Antes, o Claude precisava juntar `minhas_execucoes`, `ver_execucao`, `ver_app` e `ver_log_app`, app por app, e nem via agendas puladas nem chaves faltando.

## Decision
Ferramenta nova `problemas(app_id?, dias?)`, só de leitura. Ela devolve, do mais novo ao mais velho:
- execução com erro ou interrompida (padrão: últimos 7 dias, até 30), com a mensagem de erro e o link;
- agenda da pessoa pausada ou pulada, com o motivo;
- app com tela fora do ar (quando deveria estar no ar) com o fim do log, e a última publicação que falhou, com o fim do log dela;
- ajuste obrigatório ainda vazio em Configurações.

Tudo passa pela API como a pessoa: ela só vê o que já veria no site. Apps com tela e Configurações entram só nos apps cuja parte técnica a pessoa vê; com `app_id` de um app que ela não cuida, a resposta é uma recusa clara. Para não pesar, olha no máximo 30 apps e detalha 10 execuções por vez, e avisa quando corta. A rota `GET /api/me/schedules` entrou na lista de rotas liberadas para o Claude.

## Rationale
Uma chamada só, barata e segura, é o que permite à apresentação do MCP mandar o Claude olhar antes de mexer num app e conferir depois de publicar ou rodar (F3b.1).

## Trade-offs
**Pros:**
- O Claude acha o problema sem a pessoa contar.
- Nenhuma permissão nova: a mesma visão do site.

**Cons:**
- Várias chamadas internas por app (detalhe e app com tela); por isso o limite de 30 apps.

## Alternatives Considered
- Uma consulta SQL única: descartada; repetiria as regras de acesso que a API já aplica.
- Avisar fora da conversa (e-mail, vigia agendada): fica para F3b.2 e F3b.3.
