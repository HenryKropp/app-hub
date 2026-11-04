# Guia visual do Ziv para artifacts

Use este guia para o HTML ficar com a cara do Ziv e funcionar dentro dele.

## Visual

- Fundo branco (#FFFFFF). No modo escuro, fundo #11161A.
- Texto em tinta #11161A (no escuro, #EEF0F1). Texto secundário em cinza #6B6F73 (no escuro, #A3A9AE).
- Fonte Geist, pelo Google Fonts:
  `<link href="https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600&family=Geist+Mono&display=swap" rel="stylesheet">`
  Use `font-family: "Geist", ui-sans-serif, system-ui, sans-serif`. Números em tabela com `font-variant-numeric: tabular-nums`.
- Tamanhos: texto 14px, títulos 18 a 28px com peso 600 e `letter-spacing: -0.02em`, legendas 12px com peso 500.
- Bordas finas de 1px em #EAEAEA (no escuro, #252C33). Nada de sombras fortes.
- Cantos arredondados de 8 a 11px em cartões, tabelas e botões comuns.
- Espaços: 6, 10, 16, 20 e 32px. Margem da página 32 a 48px.
- Coral #FF6B4A só em botões de ação (fazer algo), com texto #11161A e forma de pílula. Nunca em títulos, fundos ou gráficos.
- Botão comum: fundo #11161A e texto branco, ou fundo branco com borda fina.
- Verde para "deu certo": oklch(0.52 0.13 150). Vermelho para erro: #B4232F.
- Gráficos em tons de cinza e tinta; use uma cor de destaque só quando ela explicar algo.

## Regras do HTML

- Um arquivo só: CSS e JavaScript dentro do próprio HTML.
- Bibliotecas de CDN são permitidas (por exemplo cdnjs, jsdelivr ou unpkg), com a versão fixa no endereço.
- Não peça senhas, chaves ou outros segredos. Nada de formulário de login.
- Não escreva chaves, senhas ou tokens dentro do HTML.
- O artifact abre dentro de um quadro protegido (iframe com sandbox) no Ziv:
  - não dependa de localStorage, sessionStorage ou cookies;
  - não dependa de estar na mesma origem do Ziv nem de chamar a API do Ziv;
  - links externos devem abrir em nova aba (`target="_blank"`).
- Responsivo: funcione bem do celular (360px) até uma tela larga, sem rolagem lateral.
- Escreva em português simples, com um título claro no topo e a data dos dados quando houver.

## Depois de pronto

Publique com a ferramenta publicar_artifact. Para mudar um artifact que já existe, mande o artifact_id dele: o Ziv guarda uma versão nova.
