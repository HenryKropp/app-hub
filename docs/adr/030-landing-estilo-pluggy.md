# 030 - Landing pública com tipografia e escala próprias

**Status:** Accepted

## Context
A landing (`/boas-vindas`) parecia "feita por IA": muitos cartões, muitos tamanhos de letra, negrito demais, texto e ícones demais, e explicava pouco o que o Ziv faz. O público é o CEO ou CTO de uma empresa pequena com muitos microapps e artifacts. O Henry usou o site da Pluggy como referência de ar limpo e de explicação clara.

## Decision
- A landing usa Poppins (400/500/600) com uma palavra de destaque em Instrument Serif itálico, e a escala fluida de tamanhos medida na Pluggy (título até 80 px, texto até 19,2 px).
- Na landing o laranja vira cor de destaque também em texto (palavra do título, rótulos das seções, sublinhados), em um tom mais escuro (#D23F1E) para texto pequeno.
- O resto do app continua em Geist e com a regra do DESIGN.md (coral só para ação). As regras da landing ficam em `frontend/src/pages/landing.css`, com prefixo `lp-`.
- As fontes vêm de pacotes `@fontsource` (só o subconjunto latino), servidas pelo próprio app.
- Estrutura: título + desenho animado, faixa de ferramentas, Catalogado, Documentado, Como funciona (passos verticais com imagem), chamada final.

## Rationale
O Henry comparou Geist e Poppins lado a lado (B4 e B5) e escolheu Poppins. Separar a landing do app deixa a página de venda com personalidade sem mexer no visual de trabalho, que já foi aprovado (ADR 020). Servir as fontes localmente evita depender do Google e enviar dados do visitante a terceiros.

## Trade-offs
**Pros:**
- Página mais limpa e fácil de entender; o desenho do topo explica o produto sem texto.
- O app não muda.

**Cons:**
- Duas famílias de fonte a mais no pacote (carregadas só quando a landing é aberta).
- A landing tem regras de cor diferentes das do app, que precisam ficar documentadas aqui.

## Alternatives Considered
- Manter Geist na landing (B4): mais coerente com o app, mas o Henry preferiu o ar da B5.
- Carregar as fontes do Google Fonts: mais simples, mas depende de terceiros e expõe o visitante.
