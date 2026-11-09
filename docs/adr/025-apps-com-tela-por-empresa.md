# 025 - Apps com tela em todas as empresas

**Status:** Accepted (substitui a decisão 1 da ADR 015)

## Context
Desde a ADR 015, os apps com tela (apps compartilhados, ADR 010) só rodavam na empresa principal. Os containers, os bancos e os endereços tinham nomes por app, sem empresa. O mesmo número de app em duas empresas iria se misturar.

Em 2026-11-06 o Henry pediu apps com tela em todas as empresas, e o Claude subindo esses apps (ADR 026). Ele decidiu três coisas:
- a chave de liga/desliga é por empresa e fica com o dono do Ziv;
- os limites são do tamanho da VM;
- o endereço pode ter dois níveis (`app.empresa.dominio`).

Desenho: `docs/superpowers/specs/2026-11-06-mcp-registrar-apps-design.md` (§2, item 5). Plano: `docs/superpowers/plans/2026-11-06-apps-com-tela-por-empresa.md`.

## Decision
- **Uma chave por empresa gera todos os nomes técnicos** (`internal/shared/tenant`). A chave é `w` mais os 11 primeiros caracteres hexadecimais do schema da empresa (`tenant_<24 hex>`). A principal tem a chave vazia e mantém exatamente os nomes de antes, então nada dela é migrado.

  | Coisa | Principal | Outra empresa (chave `wcf041d6ed12`) |
  |---|---|---|
  | Caixa | `ziv-<app>-<publicação>` | `ziv-wcf041d6ed12-<app>-<publicação>` |
  | Banco e usuário do app | `app_<id>` | `app_wcf041d6ed12_<id>` |
  | Pasta de dados | `<raiz>` | `<raiz>/workspaces/wcf041d6ed12` |
  | Endereço | `app.zivapp.duckdns.org` | `app.teste.zivapp.duckdns.org` |

  As caixas também levam o rótulo da empresa. Um motor "restrito" (`engine.Scoped`) recusa parar, remover ou ler logs de uma caixa de outra empresa. Caixas sem rótulo, ou com os rótulos antigos `kova.app` e `app-hub.app`, são da principal: a faxina de uma empresa nunca as trata como órfãs.
- **Endereço `app.empresa.dominio`.**
  - Motivos: o nome da empresa já identifica quem é, um app com o mesmo nome em duas empresas não colide, e o DuckDNS resolve qualquer profundidade.
  - O porteiro (`gateway.Router`) escolhe a empresa pelo endereço. Um endereço que não existe dá 404: empresa inexistente, `x.principal.dominio`, ou três níveis.
- **O Caddy só pede certificado para endereços que existem.**
  - `on_demand_tls` pergunta a `/__ziv/tls-ask`, que só aprova o endereço de um app real e só atende chamadas de `localhost` sem `X-Forwarded-For`. Por fora, `/__ziv/*` responde 404.
  - O `Caddyfile` agora fica no repositório (`deploy/Caddyfile`), e `scripts/deploy-gcp.sh` o valida e recarrega quando ele muda.
  - Como um curinga `*.dominio` cobre um nível só, os apps usam um bloco `https://` que aceita qualquer endereço.
- **Desenho §2.3 (onde fica o código):** fora da principal, o código de um app fica na pasta de código da empresa (`Svc.CodeDir(id)`). Testar, publicar e checar recusam uma pasta de fora (`shared_folder_outside`), também por link simbólico, antes de ler qualquer arquivo. Uma empresa nunca lê nem escreve pastas do servidor pela ficha, pelas versões ou pelo registro.
- **Liga/desliga por empresa.**
  - Fica na coluna `workspace_registry.shared_apps` e é mudado no Admin da principal, na seção "Empresas", só pelo dono do Ziv.
  - Empresa nova começa desligada. Na migração, só a Teste foi ligada. A principal está sempre ligada.
  - Desligada, a empresa não testa, não publica, não liga nem volta versão. As caixas abertas dormem, e a API responde 503 `shared_runtime_unavailable` com "peça ao responsável pelo Ziv".
  - Se a leitura da chave falha, a empresa conta como desligada.
- **Limites.** O que cabe numa VM de 1 GB:
  - Por empresa: até 2 apps com tela (`SHARED_APPS_PER_COMPANY`), cada um com até 256 MB (`SHARED_APP_MAX_MEMORY_MB`). Um app sem memória escolhida recebe o limite da empresa, e não o padrão de 512 MB.
  - No servidor inteiro: até 2 apps ligados ao mesmo tempo (`SHARED_APPS_MAX_RUNNING`), com 512 MB para eles (`SHARED_APPS_MEMORY_BUDGET_MB`).
  - A conta: 2 × 256 MB = 512 MB, que cabe nos cerca de 553 MB livres medidos.
  - Medição em 2026-11-09, sem app ligado:
    - memória: 953 MB no total, 399 MB usados e 553 MB disponíveis;
    - maiores processos: agente de monitoramento do Google 81 MB, Ziv 26 MB, Caddy 22 MB, Postgres cerca de 19 MB por processo.
  - Com o app de teste ligado, sobraram 426 MB.
  - A principal não tem limite por empresa, mas entra no limite do servidor.
- **Um vigia para todas as empresas.** Ele percorre as empresas uma a uma (em série) para desligar caixas paradas e religar as que caíram. É mais simples e não aperta a VM, mas uma empresa com muitas caixas atrasa a próxima volta das outras.

## Consequences
- **Volta atrás é arriscada depois que outras empresas ligam apps.** A principal não muda de nome, então voltar o código da principal é seguro. Mas uma versão antiga do Ziv não conhece os rótulos de empresa e trataria as caixas das outras empresas como órfãs. Antes de voltar, desligue as empresas no Admin.
- **Isolamento conferido na VM (2026-11-09).** Teste feito de dentro da caixa do app de teste da empresa Teste (`ziv-wcf041d6ed12-contador-de-visitas-do-time-1`):
  - **Porta de outro app no servidor:** conexão recusada. As caixas publicam a porta só em `127.0.0.1`, e a rede da caixa não alcança o `localhost` do servidor. Uma empresa não fala com o app de outra sem passar pelo porteiro.
  - **Ziv (`host.containers.internal:8080`):** responde. É o mesmo Ziv de fora, sem sessão; o `ask` continua exigindo `localhost`.
  - **Postgres (`5433`):** responde, porque é por ele que o app usa o próprio banco. O acesso `CONNECT` ao banco do Ziv e aos bancos dos outros apps é retirado de todos, então o app só entra no próprio banco.
  - **Pendência pequena:** o Postgres está publicado em `0.0.0.0:5433` na VM. O firewall do Google fecha a porta, mas o certo é publicar só onde as caixas alcançam.
- Cada empresa nova precisa ser ligada pelo dono antes de ter apps com tela.

Relacionados: ADR 010 (apps compartilhados), ADR 015 (substituída na decisão 1), ADR 022 (servidor e Caddy), ADR 026 (registrar pelo Claude).
