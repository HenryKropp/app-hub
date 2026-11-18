# Confirmar o plano de teste no chat do Claude Code (desenho curto)

**Data:** 2026-11-18 · **Pedido do Henry:** no teste da governança, o Claude
mandou um link para confirmar o plano de teste. O Henry quer confirmar ali
mesmo, no chat do Claude Code.

## Por quê
Sair do chat para o site só para clicar "Confirmar" quebra o fluxo. Mas a
confirmação existe para garantir que **uma pessoa, e não o Claude**, aceita
o que o teste vai tocar (ADR 044). Segurança é a proposta de valor do Ziv.

## Objetivo
- Quando `enviar_versao` precisa da confirmação, o Ziv abre uma **caixa no
  Claude Code** (o recurso "elicitation" do MCP) com o resumo do plano:
  chaves usadas, sites, o que o teste pula, e a escolha do PC de quem
  confirma. Botões Confirmar / Recusar.
- Confirmou: o teste começa na mesma chamada, como se tivesse confirmado no
  site. Recusou ou fechou: nada muda, e o link do site continua valendo.

## Fora do escopo
- Aprovação de líder e de TI pelo chat (o Henry escolheu só o plano).
- Mudar o que o plano contém ou quem pode confirmar (dono ou administrador).

## Anti-objetivo
- O modelo conseguir confirmar sozinho. A resposta da caixa vem do programa
  (Claude Code), não do texto do modelo. Nenhuma ferramenta nova de
  "confirmar" para o Claude chamar.

## Abordagens
1. **Caixa do Ziv no Claude Code (escolhida pelo Henry).** Só a pessoa
   responde. Custo: o servidor MCP deixa de ser "sem sessão" (hoje
   `Stateless: true`) para conseguir perguntar no meio de uma chamada; se o
   Ziv reiniciar, o Claude reconecta.
2. "Sim" no chat e o Claude confirma: mais simples, mas o Claude poderia se
   liberar. Descartada.

## Como fica
- Só quem pode confirmar (dono ou administrador) recebe a caixa; para os
  outros, segue a mensagem atual (mandar o link ao responsável).
- Programa sem caixa (claude.ai, outros): segue o link, como hoje.
- A confirmação pela caixa grava o mesmo registro e a mesma impressão do
  plano que a do site, e o histórico diz "pelo Claude Code".
- Se houver só um PC de quem confirma, ele já vem marcado.

## Minhas decisões (para o Henry conferir)
- Primeiro passo é uma prova rápida: confirmar que o Claude Code do Henry
  mostra a caixa vinda do Ziv no ar. Se não mostrar, volto antes de construir.
- Um gancho (hook) que a própria pessoa configure no Claude Code para
  responder caixas sozinho é escolha dela; o Ziv não tenta impedir.
- ADR nova registrando a exceção à regra "plano só pelo site".
