# 029 - Ficha do app em duas faces: Usar e Cuidar

**Status:** Accepted (substitui o "Desenho B" da ficha, ADR 009, na organização da página)

## Context
A página de cada app virou uma pilha de doze seções com o mesmo peso. O que
interessa a quem só usa (o que pede, o que entrega, execuções) ficava misturado
com o que interessa a quem cuida (aprovação, servidor, cópias dos dados, cofre,
código, histórico, acesso). Havia avisos soltos ("Sem reserva", "Esperando
aprovação") e até três botões coral na mesma tela. Henry pediu opções; das cinco
mostradas, escolheu misturar a "Usar · Cuidar" com a "Menu de seções"
(2026-11-09).

## Decision
- O topo não muda: nome, resumo, tipo, área, responsável, versão e o botão de
  Rodar/Abrir, o único coral da página.
- Embaixo, uma troca **Usar · Cuidar**. Quem só usa o app não vê a troca, só a
  face Usar. Quem cuida (edita, vê a parte técnica, cuida do servidor ou aprova)
  abre a página já em Cuidar.
- **Usar:** o que ele pede, o que ele entrega, execuções, agendamentos e, para o
  app compartilhado, a linha de situação. Sem "Editar".
- **Cuidar:** menu à esquerda, uma seção por vez: Pendências; App (Ficha,
  Servidor); Ajustes (Cofre, Acesso); Técnico (Código, Histórico).
- **Pendências** abre com a linha de etapas (registrado → aprovado → publicado →
  com reserva, só as que valem para o app), a barra de aprovação e uma lista do
  que falta, cada item com o botão que resolve (Escolher reserva, Escrever o
  resumo, Descrever o que entrega, Ir para Servidor). O número de pendências
  aparece no botão Cuidar.
- Seções já abertas ficam montadas (escondidas) ao trocar de seção, para não
  perder o que estava sendo editado.
- No Servidor, "Publicar" e o "Abrir" da linha de situação deixam de ser coral.

## Rationale
A raiz da bagunça era misturar dois públicos. Separar as faces deixa a página de
quem usa curta, e o menu dá a quem cuida um lugar fixo para cada coisa, com o
que falta resolvido numa lista.

## Trade-offs
**Pros:**
- Quem usa vê quatro blocos; quem cuida vê primeiro o que falta.
- Cada seção pode crescer sem empurrar as outras.

**Cons:**
- Quem cuida precisa clicar para ver uma seção que antes estava rolando a página.
- Dois menus laterais (o do Ziv e o da página) no computador; no celular o menu
  da página vira uma linha que rola para o lado.

## Alternatives Considered
- Abas (tipo GitHub): esconde a mistura, mas não separa os públicos.
- Coluna de fatos à direita: menor mudança, mas a página continuava longa.
- Painel de cartões: bom para quem cuida, pesado para quem usa.
- Só "Usar · Cuidar" com grupos em cartões, ou só "Menu de seções": Henry
  preferiu juntar os dois.
