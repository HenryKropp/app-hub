# 045 - Links externos para artifacts

**Status:** Accepted

## Context
Uma empresa cliente do Ziv quer mandar um artifact para um cliente dela, que
não tem (nem deve ter) login no Ziv. Desenho aprovado em 2026-11-17
(`docs/superpowers/specs/2026-11-17-links-externos-artifacts-design.md`).
Outputs de apps ficam para o Projeto 2.

## Decision
- **Dois tipos de link.** "Código por e-mail" (padrão quando o e-mail está
  ligado): só a pessoa do e-mail abre, com um código de 6 dígitos que vale 10
  minutos e morre depois de 5 erros. A sessão fica num cookie `ziv_link`
  (HttpOnly, caminho só daquele link) por até 30 dias, nunca além do prazo do
  link. "Link aberto": quem tem o endereço abre.
- **Conteúdo:** sempre a versão mais recente, ou congelado numa versão.
  **Prazo:** 7, 30 (padrão), 90 dias ou 1 ano. Não existe "nunca vence".
- **Endereço próprio:** `link.<domínio>/<empresa>/<token>`. É outra origem:
  não carrega o cookie do Ziv. O roteador (`gateway.Router.SetLinks`) desvia
  esse host antes dos apps, e o `ask` do Caddy aceita `link.` (o HTTPS sai
  sozinho). A página é HTML do servidor, **sem JavaScript**, com o logo do
  Ziv que leva à home. O artifact vai num iframe com o mesmo sandbox da ADR 018.
- **Uma página só para tudo que não abre:** link desconhecido, vencido,
  cancelado ou fechado mostram a mesma tela ("Este link não está mais
  disponível"), com o mesmo código 404.
- **O link fecha sozinho** se quem criou perde a permissão "Compartilhar fora
  da empresa", deixa de ver o artifact ou é apagado. Isso é conferido a cada
  abertura (`extlink.Open`), não guardado.
- **Permissão `share_external`** (cargo administrador já tem). Quem tem a
  permissão e vê o artifact cria e cancela links dele; o Admin (quem gerencia
  cargos) vê a lista da empresa toda, com quem criou, estado e aberturas.
- **Registro de abertura:** hora e e-mail (quando há). Sem IP, sem navegador.
- **Token em texto puro no banco** (32 bytes aleatórios): a tela precisa
  mostrar o link de novo para copiar. O código e a sessão são guardados só como
  hash.
- **Proteção dos formulários:** o POST só vale do próprio site. Decide o
  `Sec-Fetch-Site` (posto pelo navegador); sem ele, o `Origin` tem de bater com
  o host. `Referrer-Policy: same-origin`: o token nunca vai para outro site.
  Com `no-referrer`, o Chrome manda `Origin: null` no próprio formulário: o
  teste no navegador de verdade pegou isso e o pedido de código era recusado.
- **Limites do código.** Cada código aceita 5 tentativas, e a tentativa é
  reservada no banco antes de comparar (`ReserveExternalLinkCodeTry`): uma
  rajada de palpites ao mesmo tempo não ganha mais que 5 comparações (a
  revisão final achou 37 em 40). Pedidos de código: 5 por link e 20 por
  endereço a cada 15 minutos (na memória). E, no banco, para sobreviver a um
  reinício: por link, 10 códigos e 20 tentativas em 24 h; por empresa, 200
  códigos em 24 h (uma só conta de e-mail serve todas). O endereço é o que o
  Caddy põe em `X-Real-IP`, aceito só quando o pedido vem do próprio servidor.
- **E-mail pela Resend** (`RESEND_API_KEY`, `MAIL_FROM`). Sem chave, o código
  por e-mail aparece como "Disponível em breve" e só o link aberto é criado.
  Em `DEV_MODE`, o e-mail vai para o log. **Em produção fica desligado até a
  compra de um domínio** (decisão do Henry, 2026-11-17).

## Consequences
- Cancelar é imediato; não há como "reabrir" um link: cria-se outro.
- Com 20 tentativas por dia, um link de 1 ano aguenta no máximo ~7.300
  palpites num espaço de 1 milhão: menos de 1% de chance para quem tem o link
  vazado e não tem o e-mail.
- Projeto 2 (outputs de apps: um output específico ou o mais recente) reusa as
  mesmas tabelas de regra, sessão e página.
