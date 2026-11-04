# 020 - Visual preto e branco

**Status:** Accepted

## Context
O `DESIGN.md` descreve um visual neutro (tinta, papel branco, cinzas, Geist) com coral só como acento de ação. Em 2026-10-30 o commit 942c2d9 trouxe, sem decisão registrada, um visual creme com petróleo, serifa e degradê nas páginas públicas. Em 2026-11-04, com os dois lados a lado, o Henry escolheu o neutro como oficial. Uma crítica de design do app inteiro deu 22/40 (`.impeccable/critique/2026-11-04T14-11-03Z__frontend-src-app-tsx.md`): o coral aparecia em coisas demais, as páginas públicas mostravam um produto que não existe e havia palavras técnicas e erros sem próximo passo.

## Decision
- **O visual neutro é o oficial.** Os tokens do `DESIGN.md` voltaram; coral só em botões de rodar, "+ Registrar apps", marca da navegação ativa, seleção e indicadores ao vivo.
- **Páginas públicas refeitas** (`/boas-vindas`, `/entrar`, `/cadastre-se`, `/acesso`, `/convite`) no mesmo sistema, com `components/AuthLayout.tsx` compartilhado: botões de tinta, sem serifa, degradê nem rótulos de apresentação. O catálogo da página de boas-vindas é um exemplo marcado como ilustrativo, com apps e pessoas inventados.
- **Palavras simples:** "Hub" vira "Ziv", "Escolher arquivos", "Endereço da empresa no Ziv".
- **Erros com próximo passo:** a tela Conectar diz o que fazer em cada falha; o texto geral é "Algo deu errado. Tente de novo; se continuar, avise quem administra o Ziv."
- **O nome "Artifacts" fica** (decisão do Henry em 2026-11-04).
- O achado "Rodar com o computador desligado" já era tratado por um diálogo existente; nada a mudar.
- **Fica para a próxima rodada:** banners demais e faixa de execução velha, Home sem Rodar, nomes duplicados, tela Admin longa, botão de voltar versão (linhas Pending em `docs/TASKS.md`).

## Rationale
O `DESIGN.md` já era a fonte de verdade e o neutro deixa o coral dizer "isto roda". Uma só marca visual para páginas públicas e app evita duas identidades. Mostrar apenas o que existe (ou marcar como exemplo) mantém a promessa honesta.

## Trade-offs
**Pros:**
- Uma só linguagem visual, documentada e verificável (`e2e/visual-neutro.mjs`, claro, escuro, celular).
- Menos cor competindo com a ação principal.

**Cons:**
- O visual é mais sóbrio e menos marcante nas páginas públicas.
- A conferência visual só cobre as telas listadas; outras telas não foram revistas.

## Alternatives Considered
- **Manter creme e petróleo:** nunca foi decidido, contradizia o `DESIGN.md` e levava a serifa e degradê para uma ferramenta de trabalho; descartado pelo Henry depois de ver os dois lado a lado.
- **Misturar (público creme, app neutro):** duas identidades para manter e uma quebra ao entrar; descartado.
