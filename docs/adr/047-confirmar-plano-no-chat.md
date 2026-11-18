# 047 - Confirmar o plano de teste no chat do Claude Code

**Status:** Accepted

## Context
Desde o ADR 044, o plano de teste de um app (o que o teste acessa, pula, as
chaves e o PC) só é confirmado pelo site, para garantir que uma pessoa, e não
o Claude, aceita o que o teste vai tocar. No teste da governança (2026-11-18)
o Henry pediu para confirmar ali mesmo, no chat do Claude Code, em vez de
abrir um link.

## Decision
- Quando `enviar_versao` ou `ver_versao` chegam em "esperando confirmação", o
  Ziv pede ao Claude Code uma **caixa** (elicitation do MCP) com o resumo do
  plano e, se a pessoa tiver mais de um PC, a escolha do computador.
  Confirmar grava o plano (`confirmed_via = 'claude_code'`) e começa o teste.
- A caixa só vai para o dono do app ou um administrador com PC próprio, e só
  para programas que anunciam a caixa. Recusa, fechar, erro ou programa sem
  caixa: o link do site, como antes.
- Nenhuma ferramenta de "confirmar" para o Claude; a rota do site segue
  recusando o MCP.
- O MCP deixou de ser "sem sessão": as sessões duram até 30 minutos parado e
  ficam presas à pessoa do token (o SDK recusa a mesma sessão com outro token).
- Ao aceitar, o Ziv relê o app e confere a impressão do plano de novo: se o
  plano mudou com a caixa aberta, recusa.

## Rationale
A resposta da caixa vem do programa (Claude Code), digitada pela pessoa; o
modelo não consegue responder por ela. É a mesma garantia do site sem sair
do chat.

## Trade-offs
**Pros:**
- Confirmar sem trocar de janela.
- O link continua valendo; nada quebra onde não há caixa.

**Cons:**
- Sessões em memória: se o Ziv reiniciar, o Claude abre outra sozinho.
- Um gancho (hook) que a própria pessoa configure no Claude Code pode
  responder caixas sozinho; é escolha dela.
- Há relatos de versões do Claude Code que recusam caixas sozinhas; nelas
  continua o link.

## Alternatives Considered
- **"Sim" no chat e o Claude confirma:** o Claude poderia se liberar sozinho.
- **Código digitado pela pessoa no chat:** o Claude veria o código.
