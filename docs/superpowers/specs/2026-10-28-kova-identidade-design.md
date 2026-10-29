# Nova identidade: Kova

Conversa em 28 e 29/10/2026. O nome, o visual, o escopo técnico e o tom dos textos estão decididos pelo Henry. Aprovado pelo Henry em 29/10 e implementado no branch `kova-identidade`.

## Por que mudar
- "App Hub" foi improvisado: o nome e o logo são genéricos e não ficam na memória.
- O plano é vender para pequenas empresas sem TI, então o nome precisa se sustentar sozinho e funcionar **em inglês**, sendo fácil de falar para um brasileiro.
- A sensação desejada: um app **confiável**, que ajuda a **economizar** e traz **governança** num mundo em que a IA facilitou criar apps e isso virou bagunça. É mais "o lugar seguro onde os apps da empresa moram" do que uma loja de apps.
- Referências de que o Henry gosta: Magie, Spotify, Brex. São nomes curtos, que não descrevem o produto, e cada um tem uma cor marcante.

## Objetivos
- Trocar o nome e o visual que a pessoa vê: nome, logo, cores, título da aba e textos da interface.
- Trocar também os nomes técnicos `app-hub` por `kova`, sem quebrar nenhum app já registrado.
- Manter tudo o que funciona hoje: comportamento, telas, fluxos, regras do DESIGN.md (Voz Única, Sem Arco-íris, Mono é Dado, Só Flutua). Só a cor da "voz" muda, de violeta para coral.

## Fora do escopo
- Redesenhar telas ou mudar layout.
- Desenhar um nome escrito próprio (por exemplo, o "o" ecoando o arco da enseada). Fica para uma segunda rodada; por enquanto o nome é escrito na fonte do app.
- Domínio, site de vendas e registro no INPI.

## Seria um fracasso se
- Algum app já registrado parasse de funcionar por causa da troca de nome.
- O coral aparecesse como cor de texto ou de decoração, ou o violeta antigo sobrasse em algum lugar.
- O tema escuro ficasse de segunda categoria.

## Decidido pelo Henry
- **Nome:** kova, de *cove* (enseada protegida, porto seguro). Fala-se "kôva" em português e inglês. Escrito em minúsculas no logo. Checagem pública feita em 28/10:
  - Já usam o nome: MiTek KOVA (construtoras, EUA), uma pequena agência de automação "Kova", KOVA Painter (tintas, Vietnã), Kova Marketplace (Gana).
  - Nomes parecidos: Kovaion (low-code) e Kovi (aluguel de carros, Brasil).
  - Ninguém forte usa Kova na área do Hub. Domínios candidatos: kova.app e kova.com.br.
  - Busca no INPI ainda pendente, por conta do Henry.
- **Símbolo:** quadrado petróleo `#0E3B43` com cantos arredondados (rx 12 em 48), um arco cor de areia `#F4EFE6` (a enseada, aberto à direita, traço 5 de ponta redonda, do ponto (33, 13.5) até (33, 34.5), raio 14) e um ponto coral `#FF6B4A` de raio 5 no centro (o app protegido). No tema escuro o quadrado usa `#256A75` para não sumir no fundo. SVG de referência: `.superpowers/brainstorm/574943-1793221158/content/kova-mix-v3.html`.
- **Nomes técnicos também mudam (29/10):** `app-hub` vira `kova` nos identificadores (manifesto, pastas internas, banco, deploy), **com período de convivência**:
  - o Kova cria `kova.yaml` nos apps novos e continua lendo `app-hub.yaml` nos já registrados (se houver os dois, vale o `kova.yaml`);
  - as pastas `.app-hub-lixeira` e `.app-hub-tmp` passam a ser `.kova-lixeira` e `.kova-tmp`, e as antigas continuam sendo reconhecidas;
  - banco, container e deploy são trocados com cuidado para não perder dados (renomear, não recriar);
  - o repositório no GitHub pode ser renomeado (o GitHub redireciona o endereço antigo).
  - Nenhum app registrado pode quebrar; a migração forçada dos arquivos antigos fica para depois.
  - Ficam como estão, por serem invisíveis e porque trocar apagaria estado (decisão do plano, 29/10): preferências salvas no navegador (`apphub-*`), pasta de instalação e chave de inicialização do agente no Windows. Só o nome do arquivo baixado vira `KovaAgente.exe`.
- **Como a interface fala do produto:** "**o** Kova", com artigo masculino: "o Kova confirma", "salvo no Kova", "Agente do Kova".
- **App:** neutro em preto e branco.

  | Papel | Claro | Escuro |
  |---|---|---|
  | Fundo | `#FFFFFF` | `#11161A` |
  | Sidebar | `#FAFAFA` | `#141A1F` |
  | Card | `#FFFFFF` | `#161C21` |
  | Texto | `#11161A` | `#EEF0F1` |
  | Cinza secundário | `#6B6F73` | `#A3A9AE` |
  | Borda | `#EAEAEA` | `#252C33` |

- **Coral `#FF6B4A` só como preenchimento, nunca como texto:** Rodar (texto `#11161A` sobre o coral, 6.5:1), barrinhas "tocando agora", marcador do item ativo da navegação. Em "aberto agora" as barrinhas são coral e a palavra fica na cor do texto. O coral assume exatamente o papel que o violeta tem hoje. O coral é o mesmo nos dois temas.
- **Hover do Rodar:** `#F0562F` (texto 5.3:1).
- **Foco de teclado:** anel neutro, preto `#11161A` no claro e `#EEF0F1` no escuro, com afastamento do elemento. O Henry **não gostou do anel petróleo/verde** em volta do coral.
- **Petróleo só no logo.**
- **Erro:** `#B4232F` no claro (6.5:1 no branco) e `#F2707B` no escuro, sempre com triângulo e palavra.

## Decisões minhas (o Henry pode revisar)
- **Verde de "concluída":** mantém o atual, `oklch(0.52 0.13 150)` no claro e `oklch(0.77 0.14 150)` no escuro. Não briga com coral nem com petróleo.
- **Âmbar de atenção:** o atual (matiz 65) fica perto do coral, então vai para um âmbar mais amarelo:
  - claro: `oklch(0.55 0.12 85)`, 4.9:1 no branco;
  - escuro: `oklch(0.8 0.13 85)`.
- **Tons suaves** (fundo de linha de diff, avisos de revisão): mesma regra de hoje, derivados dessas cores.
- **Cinzas neutros:** os da tabela acima substituem os cinzas atuais; os demais tons (hover, pílula, campo) são derivados no mesmo espírito, com contraste conferido.
- **Bordas de campo:** passam a ter pelo menos 3:1 no fundo, para resolver o achado da auditoria.

- **Nome num lugar só:** o nome do produto fica numa constante única no frontend (e uma no backend), para uma eventual troca por causa do INPI custar uma linha. A troca começa sem esperar o INPI.
- **Pendências de cor da auditoria entram na troca:** "Abrir catálogo" vira botão neutro; os quatro pontos de violeta decorativo (`SetupPage.tsx:98`, `CatalogPage.tsx:81`, `ScriptImportForm.tsx:174`) ficam neutros, sem coral; o rótulo acima do título nesses banners sai.
- **`clsx`** entra no `package.json` (está em uso sem estar declarado; risco de quebrar o build). O resto do código morto fica para depois.
- **Critério de aceite de cor:** WCAG AA, com texto a 4.5:1 (3:1 a partir de 24px) e bordas de controle, foco e ícones a 3:1, nos dois temas; conferido por cálculo e por screenshots de claro e escuro, desktop e celular.
- **Favicon:** usa o símbolo do tema claro (`#0E3B43`), que tem fundo próprio e funciona em abas claras e escuras.
- **Design system publicado** é atualizado como parte deste trabalho.
- **Geometria do símbolo** (fonte oficial, já que o mockup está fora do git):

  ```svg
  <svg viewBox="0 0 48 48">
    <rect width="48" height="48" rx="12" fill="#0E3B43"/><!-- escuro: #256A75 -->
    <path d="M33 13.5A14 14 0 1 0 33 34.5" fill="none" stroke="#F4EFE6" stroke-width="5" stroke-linecap="round"/>
    <circle cx="24" cy="24" r="5" fill="#FF6B4A"/>
  </svg>
  ```

## Para o construtor decidir
- Converter os hex para OKLCH no `index.css`, ou mantê-los em hex.
- Como estruturar o componente do logo e os ícones do navegador (favicon), desde que reproduzam o símbolo acima.
- A ordem dos commits.

## Onde muda (levantado no código em 29/10)
- **Cores:** `frontend/src/index.css` (tokens claro e escuro) e `components/ui/button.tsx` (variante `run`: hover e foco).
- **Logo:** `Logo` em `frontend/src/components/bits.tsx`.
- **"App Hub" na interface:**
  - `frontend/index.html` (título da aba);
  - `App.tsx`, `Layout.tsx`, `LoginPage.tsx`, `SetupPage.tsx`, `lib/shared.ts` e testes;
  - a página do gateway de apps compartilhados, `backend/internal/shared/gateway/page.html`.
- **"o Hub", "do Hub", "no Hub", "Agente do Hub":** cerca de 80 ocorrências em textos da interface.
- **Identificadores técnicos com `app-hub`** (cerca de 350): arquivo de manifesto `app-hub.yaml` (backend, frontend e `validate-manifest`), banco `app-hub-db`, pastas `app-hub-lixeira` e `app-hub-tmp`, deploy e repositório. Trocados com convivência, conforme decidido acima; a leitura do nome antigo precisa de teste.
- **Documentos:** `PRODUCT.md`, `DESIGN.md`, `README.md` e `replit.md`.
- **Design system publicado:** https://claude.ai/artifact/97gdjWrmLxUx4QMYJGaQcW (tokens, logo, capa, README).

## Pendências da auditoria de 28/10
Algumas se resolvem com a troca de cor e outras ficam para depois:
- **Resolvidas pela nova paleta:**
  - contraste do texto do Rodar no escuro;
  - foco fraco no Rodar;
  - bordas de campo abaixo de 3:1.
- **Viram coral fora da regra se não forem corrigidas na troca:**
  - violeta decorativo em `SetupPage.tsx:92,98`, `CatalogPage.tsx:81` e `ScriptImportForm.tsx:174`;
  - "Abrir catálogo" com a variante `run`.
- **Ficam para depois, salvo decisão em contrário:**
  - dois `h1` na `SetupPage`;
  - chip verde em `RegisterPage.tsx:198`;
  - mono em contadores;
  - código morto: `--kind-*` em `lib/kinds.ts`, `ui/select`, `ui/tooltip`, `ui/badge` e `ui/separator` sem uso, a dependência `cn` sem uso e `clsx` usado sem estar no `package.json`.

## Mockups
Ficam em `.superpowers/brainstorm/`, fora do git:
- `562487-1793218214/content/`: as três direções, as quatro paletas e as versões v1 e v2 da mistura.
- `574943-1793221158/content/kova-mix-v3.html`: a versão aprovada.
