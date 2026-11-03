# 018 - Artifacts

**Status:** Accepted

## Context
Henry quer guardar e compartilhar no Ziv as páginas que o Claude gera (gráficos, painéis em HTML), com quem já tem acesso por área e cargo. Uma página dessas roda scripts: abrir o HTML no mesmo endereço do Ziv deixaria o código dela usar o login de quem abre.

## Decision
- Um artifact é um **link** (`https://`) ou **um arquivo HTML** (até 16 MB), com versões, etiquetas e área. O HTML fica em `code_blobs`, como o código dos apps.
- O conteúdo é servido com `Content-Security-Policy: sandbox allow-scripts allow-popups allow-forms allow-downloads` (sem `allow-same-origin`) e `nosniff`, e aparece num iframe com sandbox. A origem fica **opaca**: a página roda, mas não lê o cookie nem chama a API do Ziv. O cookie de sessão é `SameSite=Lax`, então uma chamada vinda dessa origem não leva o login. Além disso, o filtro de origem (`security_middleware.go`, cabeçalhos `Origin` e `Sec-Fetch-Site`) recusa pedidos que mudam algo vindos dessa página, que chegam com `Origin: null`.
- **Permissões compartilhadas com os apps:** registrar = `CanRegister`; as permissões de cargo `edit_any_app`, `restore_any_app`, `see_all_apps` e os acessos dados na área toda valem também para os artifacts da área. O acesso por artifact oferece só editar e voltar versões. Dos acessos dados na área toda, só editar e voltar versões alcançam artifacts; ver a parte técnica e ver execuções são só de apps e não mostram um artifact restrito.
- **Link mostrado dentro da página (2026-11-03, pedido do Henry):** ao salvar um link, o servidor faz um `HEAD` ao site (ou `GET`, se o site recusar `HEAD`; só `https`, 5 s, até 5 redirecionamentos, sem proxy, sem cookies, no máximo 4 conferências ao mesmo tempo no servidor) e guarda em `artifact_versions.embeddable` se o navegador deixaria outro site mostrá-lo num frame: `frame-ancestors`, quando existe, decide (só `*` ou `https:` contam como aberto); senão, qualquer `X-Frame-Options` fecha. A conexão recusa endereços da rede interna e reservados (loopback, privados, link-local, 100.64/10, 0/8, 192.0.0/24, 198.18/15, 240/4 e as formas IPv6 que embutem IPv4: NAT64, 6to4, Teredo), conferidos a cada conexão, depois do DNS, para um link colado não fazer o Ziv sondar serviços internos. Links para o próprio Ziv (`BASE_URL`, `SHARED_APPS_DOMAIN` e subdomínios), ou que redirecionam para ele, nunca são marcados; a tela também não mostra num frame um link da mesma origem da página. E o Ziv agora responde a tudo com `frame-ancestors 'none'` e `X-Frame-Options: DENY` (o conteúdo de artifact usa `'self'`, para aparecer só na página do próprio Ziv): mesmo que um link mude depois e redirecione para o Ziv, o navegador não o mostra no frame. Isso também protege contra clickjacking. O frame do link usa sandbox **com** `allow-same-origin`, porque o site de fora precisa da própria origem (login, armazenamento) e, sendo outra origem, continua sem acesso à página do Ziv. Voltar uma versão copia a marcação. Links do Claude recusam frame (`frame-ancestors 'self'`): a tela orienta baixar o HTML e subir como arquivo.
- **Sem aprovação** para artifacts, e "exigir aprovação" passa a começar **desligado** em empresa nova (decisão do Henry em 2026-11-03); as empresas existentes mantêm a opção.

## Rationale
Sandbox com origem opaca dá o isolamento que importa (a página não age como a pessoa) sem novo serviço nem novo domínio. Reaproveitar as regras dos apps evita um segundo modelo de acesso para explicar. Artifact não roda nem muda o computador de ninguém, então aprovação não protege nada.

## Trade-offs
**Pros:**
- Nada novo para instalar ou hospedar.
- Mesmas regras de acesso que a equipe já conhece.

**Cons:**
- Isolamento mais fraco que um domínio separado: depende do navegador respeitar o sandbox.
- Páginas que precisam do login do Ziv não funcionam (por desenho).
- Blobs de versões antigas não são limpos.
- A marcação de "mostrar dentro" é feita uma vez, ao salvar: se o site mudar a regra depois, o link aparece em branco no frame (o "Abrir em tela cheia" continua funcionando) até alguém enviar o link de novo. Sites que liberam só uma lista de domínios nomeados ficam como "não aceita". Salvar um link pode levar até 5 s a mais.
- **Imitação de tela do Ziv:** "Abrir em tela cheia" mostra o HTML de quem subiu como página inteira no endereço do Ziv; esse HTML poderia imitar a tela de login do Ziv para pegar senhas. Aceito por enquanto porque só quem pode registrar sobe artifacts; o domínio separado para conteúdo segue na lista do deploy na nuvem.

## Alternatives Considered
- **Subdomínio separado só para conteúdo:** isolamento mais forte (origem de verdade, cookies separados). Fica para o deploy na nuvem, quando houver domínio próprio.
- **Tentar mostrar todo link num frame e deixar o navegador recusar:** o navegador não avisa a página quando recusa, e a pessoa veria uma caixa vazia ou um erro; descartado.
- **Baixar o HTML do link e guardar como arquivo:** links do Claude e de painéis montam a página com scripts e login, então a cópia sairia vazia; descartado.
- **Passar artifacts por aprovação como os apps:** atrasaria algo que só se abre; descartado.
