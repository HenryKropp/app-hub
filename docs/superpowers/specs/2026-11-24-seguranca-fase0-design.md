# Segurança, Fase 0: fechar furos (desenho)

**Data:** 2026-11-24 · **Roteiro:** `2026-11-23-roteiro-seguranca.md` (aprovado
pelo Henry em 2026-11-23). **Pedido do Henry (2026-11-24):** "de resto tudo
faz sentido, pode fazer" sobre os 7 itens da Fase 0.

## Por quê

Segurança é o principal argumento de venda da Ziv, e quem compra é a TI. Um
prospect do setor financeiro já avisou que a TI dele é rígida. Antes de
construir coisas novas (Fase 1 em diante), os furos que já conhecemos
precisam estar fechados: hoje um atacante consegue travar o login de todos,
um app consegue chamar o Ziv por dentro, a cópia do banco fica aberta no
mesmo disco, e quem tomar o servidor consegue rodar código em todos os PCs
pareados.

## Decisões do Henry (2026-11-24)

- **Fase 1 logo depois da Fase 0.** Login com Google/Microsoft e
  verificação em 2 passos vêm em seguida, mas **não** entram nesta fase. O
  limite de tentativas continua necessário mesmo com eles: senha, cadastro,
  pareamento do Agente e registro do Claude continuam existindo.
- **Empresa nova: escolher na criação.** Quem cria a empresa escolhe entre
  "Começar com aprovações ligadas (recomendado)" e "Começar livre para
  testar". A escolha fica registrada.
- **Cópia do banco no Google Cloud, por enquanto.** O destino fica numa
  configuração; quando houver clientes (servidor no Brasil, nuvem do
  cliente), troca-se o destino sem reescrever o script.
- Abordagens A nos três pontos com escolha (cópia, chave do cofre, endereços
  de retorno do OAuth), ver abaixo.

## Objetivo

Fechar os furos abaixo, cada um com teste automático, sem mudar o que a
pessoa faz no dia a dia (salvo onde dito).

1. **Limite de tentativas por endereço de verdade.** O limitador
   (`handler/security_middleware.go`) passa a usar o endereço real de quem
   pede. Hoje usa `r.RemoteAddr`, que atrás do Caddy é sempre `127.0.0.1`,
   então o limite é de todo mundo junto (300 logins ou 20 cadastros por 15
   min travam todos). O helper `remoteHost()` (hoje em
   `handler/external_page.go`) já faz certo: aceita `X-Real-IP` só quando o
   pedido vem da própria máquina. Ele vai para um arquivo comum e é usado nos
   dois lugares. O limitador continua em memória (zera ao reiniciar; é
   aceitável). O Caddyfile já manda `X-Real-IP`.
2. **Registro de ações com endereço e navegador.** `security_audit` ganha
   `ip` e `user_agent` (cortado em 300 caracteres), gravados em todo evento.
   A exportação (`/api/security/export`) passa a incluir os dois.
3. **O Ziv só atende pedidos de dentro da máquina.** Hoje o servidor escuta
   em `:8080` (todas as interfaces), e a caixa de um app chega nele por
   `host.containers.internal:8080` (ADR 025, conferido na VM). Passa a
   escutar em `127.0.0.1` por padrão; `LISTEN_HOST` abre de novo quando
   preciso (ex.: um ambiente de teste). O Caddy já fala com `localhost:8080`.
   Os apps **continuam** chegando ao Postgres (é onde fica o banco de cada
   app), e cada app só entra no próprio banco (já é assim, `appdb.LockDown`).
   **A conferir antes de fechar:** com o Podman sem root, o caminho
   `host-gateway` pode chegar ao servidor como se fosse de dentro da máquina.
   A ADR 025 indica que não (o `tls-ask`, que exige `localhost`, recusa a
   caixa), mas isso é testado de novo numa caixa de teste local: de dentro
   dela, (a) o Ziv em `127.0.0.1` não responde; (b) um `X-Real-IP` falso não
   é aceito; (c) o Postgres ainda responde. O endereço onde publicar o
   Postgres na VM sai desse mesmo teste, refeito na VM com o "pode" do Henry.
4. **Travas nas caixas dos apps** (`shared/engine/podman.go`, `Start`):
   `--cap-drop=ALL`, `--security-opt no-new-privileges`, `--pids-limit 256`.
   `--read-only` (com `--tmpfs /tmp`) entra se as receitas do Ziv
   (`shared/recipes`) continuarem rodando; se alguma quebrar, fica de fora e
   anotado em TASKS. `--user` não entra: o Podman roda sem root no servidor,
   então o "root" da caixa já é um usuário comum da máquina.
5. **Cabeçalhos do site.** `Strict-Transport-Security` (1 ano) quando o
   `BASE_URL` é https, e `Content-Security-Policy` no site React com
   `script-src 'self'` (conferir no navegador que nada do site quebra).
6. **Prazo para o "código" do Claude.** Conexões MCP do tipo código
   (`mcp_auth.go`, `createMCPCode`) passam a vencer em 90 dias:
   `access_expires_at` preenchido na criação, recusado depois. A tela
   Conectar o Claude e a lista no Admin mostram a data. Códigos que já
   existem ganham 90 dias a contar do dia em que esta versão subir. O login
   pelo navegador (OAuth) continua sendo o caminho recomendado e já se renova
   sozinho.
7. **Endereços de retorno do OAuth numa lista fechada** (`mcp_oauth.go`,
   `validRedirectURI`): aceita `http://localhost` e `http://127.0.0.1` (com
   qualquer porta) e `https://claude.ai` e `https://claude.com` (e
   subdomínios). Outros entram só por `OAUTH_REDIRECT_HOSTS` no servidor.
   Fecha o golpe de um app falso que se registra com o endereço dele e pega o
   acesso de quem aprovar. Clientes já registrados com outro endereço deixam
   de conseguir renovar (registrar de novo resolve).
8. **Desativar pessoa.** Hoje não há como tirar alguém da empresa. O
   administrador ganha "Desativar" e "Reativar" na lista de pessoas do Admin.
   Desativada, a pessoa não entra (senha nem entrada por convite), as sessões
   dela são apagadas, as conexões do Claude e os computadores pareados são
   revogados, e ela some das escolhas de acesso. Os dados dela ficam (apagar
   de vez é a Fase 2, LGPD). Não dá para desativar a si mesmo nem o último
   administrador (`CountManagersExceptPerson` já existe). Fica no registro de
   ações.
9. **Sessão que cai por inatividade.** Além dos 30 dias fixos de hoje
   (`handler.go`, `sessionTTL`), a sessão cai depois de 7 dias sem uso
   (`sessions.last_used_at`, atualizado no máximo uma vez por hora para não
   gravar a cada pedido; precisa de migração). Vale só para as sessões do
   Ziv, não para as dos apps com tela. O tempo por empresa e a lista de
   sessões ficam para a Fase 1.
10. **Computador parado perde o acesso.** Um computador que não fala com o
    Ziv há 90 dias tem o token revogado (o vigia que já roda faz isso) e
    precisa ser pareado de novo. O Agente ligado fala o tempo todo, então
    quem usa não percebe.
11. **Empresa nova escolhe o ponto de partida** (decisão do Henry). A
    criação (`POST /api/workspaces`, `workspace/portal.go` →
    `ApplyNewCompanyGovernance`) recebe a escolha. "Aprovações ligadas" (a
    marcada por padrão): `require_app_approval` e `it_approval` ligados, o
    cargo administrador **mantém** `approve_it`, `mcp_restricted` ligado
    (chaves do MCP limitadas por cargo), `version_rule = 'risk'` e
    `code_channel = 'any'`; o administrador pode afrouxar depois. Quem cria a
    empresa sozinho não trava: pela ADR 046, quem é líder e TI aprova o
    próprio pedido. "Livre para testar": como é hoje (a consulta atual). A
    escolha entra em `governance_events`.
12. **Chave do cofre com versão e troca.** Cada valor cifrado com a
    `HUB_SECRET_KEY` (`vault.Seal` e `vault.SealStream`) passa a levar um
    cabeçalho com a identidade da chave (os primeiros bytes do SHA-256 dela,
    sem número para alguém manter). Ler aceita a chave atual, as chaves
    antigas em `HUB_SECRET_KEYS_OLD` e o formato antigo sem cabeçalho. Um
    comando (`cmd/trocarchave`) recifra com a chave atual tudo o que está
    guardado: as colunas cifradas no banco (chaves dos apps, configurações
    secretas, chaves do MCP, senhas de banco dos apps) e as cópias por app em
    disco. Depois disso a chave antiga pode sair do `ziv.env`. A chave segue
    no `ziv.env`, que já fica fora do banco e das cópias.
    O cofre de credenciais (`vault/envelope.go`, `CREDENTIAL_VAULT_KEYRING`)
    é outro sistema, já tem versão e troca (`Rewrap`) e está **desligado** em
    produção (ADR 027). Fica separado e fora do `cmd/trocarchave`; juntar os
    dois não vale o risco agora.
13. **Atualização do Agente assinada.** Chave ed25519 de assinatura:
    - a **privada** fica no PC do Henry, ao lado da cópia da
      `HUB_SECRET_KEY` (`~/.ziv-producao/`), com uma segunda cópia num lugar
      dele (gerenciador de senhas ou pendrive). Nunca vai para o servidor;
    - a **pública** fica no código do Agente, no repositório;
    - `scripts/deploy-gcp.sh` assina o Agente ao compilar e envia o arquivo
      de assinatura junto. Sem a chave privada, o deploy para com um aviso;
    - o servidor manda a assinatura na ordem de atualização
      (`agentproto.UpdatePayload`); o Agente confere com a chave pública
      antes de trocar de versão e recusa se não bater ou faltar;
    - a assinatura cobre o programa **sem** o bloco da empresa que o servidor
      anexa ao fim do EXE (`agentproto/trailer.go`, marca `ZIVHUB01`, ADR
      016); o Agente tira o bloco antes de conferir;
    - em desenvolvimento, `scripts/run-local.sh` gera um par de chaves só de
      teste e compila o Agente com a pública dele.

    A primeira versão assinada ainda chega aos PCs pelo caminho antigo (só o
    hash); dali em diante toda atualização é conferida. Se a chave privada se
    perder, os PCs precisam reinstalar o Agente uma vez.
14. **Token do Agente protegido no PC.** O `agente.json`
    (`agent/agent.go`, `SaveToken`/`LoadToken`) passa a guardar o token
    cifrado com a proteção de dados do Windows (DPAPI, ligada ao usuário do
    PC). Um arquivo antigo em texto é lido e regravado protegido na primeira
    vez. Fora do Windows (testes), segue como hoje.
15. **Cópia diária do banco principal cifrada e fora da máquina.** Hoje um
    cron feito à mão (`pg_dumpall` 06:30 UTC, 7 dias, `~/backups`) grava
    cópias abertas no mesmo disco, sem script no repositório. Passa a ser:
    - `deploy/backup-banco.sh` no repositório, rodado por um timer do
      systemd do usuário `ziv` (arquivos também no repositório);
    - `pg_dumpall` → compactado → cifrado com a chave **pública** do `age`
      (só a privada, que fica com o Henry, abre) → guarda as últimas 7 no
      disco (já cifradas) → envia para o destino em `BACKUP_DEST` (hoje um
      bucket do Google Cloud, `gs://...`);
    - o bucket tem trava de retenção de 30 dias (nada é apagado antes), e a
      conta que o servidor usa só consegue gravar, não apagar nem ler;
    - um passo a passo de restauração em `docs/INFRASTRUCTURE.md`, testado
      uma vez numa máquina local com uma cópia de verdade.
16. **Checagem automática no GitHub.** `.github/workflows/ci.yml` roda a
    cada push: testes Go (com Postgres de serviço), `govulncheck`, testes,
    lint e build do site, e `npm audit --audit-level=high`. Falha fica
    vermelha no GitHub; nada é bloqueado por enquanto.

## Fora do escopo

- Login com Google/Microsoft, verificação em 2 passos, trocar e recuperar a
  senha, tempo de sessão por empresa, lista de sessões ativas (Fase 1, logo
  depois desta).
- Bloquear a internet dos apps e liberar só os sites de cada um (Fase 2).
- Apagar pessoa ou empresa de vez (Fase 2, LGPD).
- Gerenciador de chaves do Google (KMS), servidor no Brasil (Fase 3).
- Assinatura Authenticode do EXE (já prevista em "Ziv no Windows").
- Conexão do app ao banco com SSL (`sslmode=disable` em `appdb.go`): o
  tráfego não sai da máquina. Volta na Fase 3, se o banco for para outro
  lugar.
- Registrar leituras no registro de ações.

## Anti-objetivo

- Quebrar o que já funciona: apps que rodam hoje, o Agente nos PCs, o
  Claude conectado. Cada trava nova é testada com as receitas e o fluxo de
  ponta a ponta antes de subir.
- O servidor conseguir abrir as cópias do banco ou assinar o Agente. As duas
  chaves privadas ficam fora dele.
- Uma mudança que exija o Henry fazer algo no servidor sem saber o quê:
  toda ação na VM vai num passo a passo.

## Abordagens (as três com escolha real)

**Cópia do banco.** A (escolhida): bucket do Google Cloud, cifrado com
chave pública, trava de retenção. B: o PC do Henry puxa a cópia todo dia;
depende de o PC estar ligado. Henry: "pode ser por enquanto, mas se a Ziv
começar a ter clientes talvez tenhamos que mudar". Por isso o destino é
configurável.

**Chave do cofre.** A (escolhida): versão e troca agora, chave no
`ziv.env`. B: KMS do Google já; ganho pequeno hoje, porque quem toma a VM
pega a chave do mesmo jeito. KMS vem com o servidor novo (Fase 3).

**Endereços de retorno do OAuth.** A (escolhida): lista fechada mais
configuração. B: aceitar qualquer https e mostrar o site na tela de
permissão; depende de a pessoa ler.

## O que é código e o que precisa do Henry

**Código (feito nesta fase, no branch `seguranca-fase0`):** itens 1 a 16,
com testes; os scripts e o passo a passo dos itens 13 e 15.

**Precisa do Henry fora do código (passo a passo vai no fim da entrega):**
- **Chave de assinatura do Agente:** rodar o comando que gera o par, guardar
  a privada em `~/.ziv-producao/` e uma segunda cópia num lugar seu.
- **Chave das cópias (`age`):** gerar o par no seu PC, guardar a privada
  (mesmo cuidado), mandar a pública para o servidor.
- **Google Cloud:** criar o bucket (região `us-east1`, trava de retenção de
  30 dias) e uma conta de serviço que só grava nele; dar a chave dela ao
  servidor.
- **Deploy:** um "pode" explícito para rodar `scripts/deploy-gcp.sh` e para
  mexer na VM: instalar `age`, ligar o timer da cópia, tirar o cron antigo e
  fechar o Postgres para fora (hoje publicado em `0.0.0.0:5433`; o firewall
  do Google fecha a porta, mas o certo é publicar só onde as caixas
  alcançam). Fechar o Postgres exige recriar o contêiner `ziv-db` com
  cuidado (o volume `ziv-pg` fica), testado antes numa caixa de teste.
- **Depois do deploy:** rodar `cmd/trocarchave` só quando quiser trocar a
  chave do cofre (não é obrigatório agora; o importante é que agora dá).

## Ordem de entrega

Cada passo é um commit com testes, nesta ordem (do mais grave e simples ao
que depende de ação do Henry):

1. Limite de tentativas por endereço real + IP e navegador no registro
   (itens 1, 2).
2. Ziv só para dentro, travas nas caixas, cabeçalhos (3, 4, 5).
3. Claude: prazo do código e lista de endereços (6, 7).
4. Pessoas e sessões: desativar, inatividade, computador parado (8, 9, 10).
5. Empresa nova escolhe o ponto de partida (11).
6. Chave do cofre com versão e troca (12).
7. Agente: atualização assinada e token protegido (13, 14).
8. Cópia do banco: script, timer e passo a passo (15).
9. Checagem automática no GitHub (16).
10. ADR 048 (assinatura do Agente, cópia fora da máquina, chave com versão,
    lista de endereços do OAuth) e `docs/TASKS.md` no "Roteiro de
    segurança", com a Fase 1 marcada como a próxima.

## Minhas decisões (para o Henry conferir)

- 90 dias para o código do Claude e para o computador parado; 7 dias sem uso
  para a sessão.
- `--read-only` só se as receitas aguentarem; `--user` e SSL do banco ficam
  de fora, com o motivo acima.
- Desativar, não apagar, pessoa.
- A checagem no GitHub avisa mas não bloqueia, por enquanto.
