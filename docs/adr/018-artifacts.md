# 018 - Artifacts

**Status:** Accepted

## Context
Henry quer guardar e compartilhar no Ziv as páginas que o Claude gera (gráficos, painéis em HTML), com quem já tem acesso por área e cargo. Uma página dessas roda scripts: abrir o HTML no mesmo endereço do Ziv deixaria o código dela usar o login de quem abre.

## Decision
- Um artifact é um **link** (`https://`) ou **um arquivo HTML** (até 16 MB), com versões, etiquetas e área. O HTML fica em `code_blobs`, como o código dos apps.
- O conteúdo é servido com `Content-Security-Policy: sandbox allow-scripts allow-popups allow-forms allow-downloads` (sem `allow-same-origin`) e `nosniff`, e aparece num iframe com sandbox. A origem fica **opaca**: a página roda, mas não lê o cookie nem chama a API do Ziv. O cookie de sessão é `SameSite=Lax`, então uma chamada vinda dessa origem não leva o login. Além disso, o filtro de origem (`security_middleware.go`, cabeçalhos `Origin` e `Sec-Fetch-Site`) recusa pedidos que mudam algo vindos dessa página, que chegam com `Origin: null`.
- **Permissões compartilhadas com os apps:** registrar = `CanRegister`; as permissões de cargo `edit_any_app`, `restore_any_app`, `see_all_apps` e os acessos dados na área toda valem também para os artifacts da área. O acesso por artifact oferece só editar e voltar versões. Dos acessos dados na área toda, só editar e voltar versões alcançam artifacts; ver a parte técnica e ver execuções são só de apps e não mostram um artifact restrito.
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
- **Imitação de tela do Ziv:** "Abrir em tela cheia" mostra o HTML de quem subiu como página inteira no endereço do Ziv; esse HTML poderia imitar a tela de login do Ziv para pegar senhas. Aceito por enquanto porque só quem pode registrar sobe artifacts; o domínio separado para conteúdo segue na lista do deploy na nuvem.

## Alternatives Considered
- **Subdomínio separado só para conteúdo:** isolamento mais forte (origem de verdade, cookies separados). Fica para o deploy na nuvem, quando houver domínio próprio.
- **Passar artifacts por aprovação como os apps:** atrasaria algo que só se abre; descartado.
