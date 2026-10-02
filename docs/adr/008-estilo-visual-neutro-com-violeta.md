# 008 - Estilo visual neutro com um violeta de destaque

**Status:** Accepted

## Context
O primeiro visual do Hub imitava um fichário antigo (papel creme, serifa, carimbo vermelho, abas coloridas por tipo de app). O Henry achou o visual antigo, genérico e pouco claro: era difícil ver o que importa (o Rodar, onde os arquivos ficam). O Hub é produto próprio, a Alis é só o cliente zero, então o visual não precisa combinar com a Alis. A sensação pedida é "ágil e direto", claro por padrão.

## Decision
Seguir o padrão das ferramentas de trabalho atuais (Notion, Vercel): fundo branco, cinzas neutros, linhas finas, letra Geist. Uma só cor viva, violeta, usada como o Spotify usa o verde: no botão Rodar (arredondado), no ícone do item ativo do menu, no foco e no que está aberto agora (barrinhas animadas de "tocando"). Tipos de app viram etiquetas cinza com ícone. Verde, vermelho e âmbar ficam só em pequenos sinais de estado. Modo escuro mantido; o tema abre claro por padrão.

## Rationale
Quem usa o Hub está com pressa: o olho precisa achar o Rodar e o estado das execuções sem competir com decoração. Reservar a cor para "começar" e "rodando agora" faz dela um sinal, não um enfeite.

## Trade-offs
**Pros:**
- Leitura rápida; o Rodar e o que está aberto se destacam sozinhos
- Visual familiar para quem usa ferramentas atuais
- Cores centralizadas em tokens (`frontend/src/index.css`), fáceis de ajustar

**Cons:**
- Parecido com outras ferramentas de propósito; a identidade vem do violeta e das barrinhas, não do layout

## Alternatives Considered
- Direções sorteadas (programa de identidade, sinalização de metrô, etiqueta de entrega e outras): descartadas pelo Henry por serem coloridas ou temáticas demais
- Grade suíça, extrato de banco, placas de aeroporto: ele preferiu a base Notion/Vercel com destaque mais forte
- Verde, laranja ou azul como destaque: escolheu violeta
