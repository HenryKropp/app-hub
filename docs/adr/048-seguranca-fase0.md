# 048 - Segurança, Fase 0: fechar furos

**Status:** Accepted

## Context
Segurança é o principal argumento de venda da Ziv, e quem compra é a TI. O
roteiro de segurança (`docs/superpowers/specs/2026-11-23-roteiro-seguranca.md`)
começa pela Fase 0: furos já conhecidos. O Henry aprovou o desenho em
2026-11-24 (`docs/superpowers/specs/2026-11-24-seguranca-fase0-design.md`).
Este ADR registra as decisões de arquitetura; os detalhes estão no desenho.

## Decision
- **Endereço real atrás do Caddy.** O limite de tentativas e o registro de
  ações usam `remoteHost`: `X-Real-IP` só quando o pedido vem da própria
  máquina. `security_audit` ganha `ip` e `user_agent`.
- **O Ziv só atende pedidos de dentro da máquina** (`LISTEN_HOST`, padrão
  `127.0.0.1`). As caixas dos apps deixam de chegar ao Ziv por
  `host.containers.internal:8080`. O Postgres continua ao alcance delas (é
  onde fica o banco de cada app). As caixas sobem com `--cap-drop=ALL`,
  `no-new-privileges` e `--pids-limit 256`.
- **Assinatura do Agente com chave fora do servidor.** ed25519
  (`cmd/agentkey`). A chave privada fica no PC do Henry
  (`~/.ziv-producao/agente-assinatura.key`) e nunca vai para o servidor. O
  deploy assina o Agente e embute a chave pública (`main.updateKey`). O
  Agente só instala atualização com assinatura válida, que cobre o programa
  sem o bloco da empresa (`ZIVHUB01`). Sem o `.sig` ao lado do binário, o
  servidor não pede atualização.
- **Cópia do banco cifrada com chave pública** (`age`). O servidor grava, mas
  não consegue abrir as próprias cópias. Uma cópia vai para fora da máquina
  (`BACKUP_DEST`: hoje um bucket do Google Cloud com trava de retenção de 30
  dias, e uma conta que só grava). Trocar o destino é mudar uma linha.
- **Chave do cofre com identidade.** Todo valor e toda cópia cifrados com a
  `HUB_SECRET_KEY` levam 4 bytes do SHA-256 da chave. `HUB_SECRET_KEYS_OLD`
  mantém chaves antigas válidas, e `cmd/trocarchave` recifra tudo na chave
  atual. O cofre de credenciais (`CREDENTIAL_VAULT_KEYRING`) segue separado e
  desligado em produção.
- **OAuth do MCP com lista fechada de retorno:** `localhost`/`127.0.0.1`
  (http) e `claude.ai`/`claude.com` (https), mais `OAUTH_REDIRECT_HOSTS`.
  Conexões "código" vencem em 90 dias.
- **Pessoas e sessões:** desativar pessoa, sem apagar (apaga as sessões e
  revoga computadores e conexões do Claude). A sessão cai após 7 dias sem uso.
  O computador sem contato há 90 dias perde o acesso.
- **Empresa nova escolhe o ponto de partida.** Ligadas é o padrão quando a
  escolha não vem.
- **Checagem a cada push** no GitHub (testes, govulncheck, npm audit) e Go
  1.27.4 fixado (`toolchain`).

## Rationale
- Quem tomar o servidor não deve conseguir trocar o programa do Agente nos
  PCs pareados, nem ler cópias antigas. Por isso as duas chaves privadas
  ficam fora dele. **Risco que fica:** as execuções, os testes e os apps de
  tela levam o código do app até o Agente, e esse código não é assinado.
  Quem tomar o servidor ainda pode agendar ou mandar rodar código nos PCs.
  Fechar isso pede código aprovado e assinado, numa fase seguinte.
- A troca de chave precisa ser possível antes de ser necessária. Um KMS
  agora ganharia pouco, porque quem toma a VM pega a chave do mesmo jeito.
  Ele vem na Fase 3, com o servidor no Brasil.
- Uma lista fechada no OAuth não depende de a pessoa ler um aviso.

## Consequences
- **Perder a chave de assinatura do Agente** obriga os PCs a reinstalar o
  Agente uma vez. Por isso há uma segunda cópia num lugar do Henry.
- **Perder a chave privada das cópias** deixa as cópias ilegíveis. Mesma
  regra: duas cópias.
- O deploy para se faltar a chave de assinatura. A primeira versão assinada
  ainda chega aos PCs pelo caminho antigo (só o hash); dali em diante tudo é
  conferido.
- Clientes OAuth registrados com outro endereço deixam de renovar e precisam
  conectar de novo.
- Fica pendente o teste na VM (precisa do "pode" do Henry): caixas sem acesso
  ao Ziv, com acesso ao Postgres, e se `X-Real-IP` falso de uma caixa é
  recusado. O Ziv confia em `X-Real-IP` só quando o pedido vem de
  `127.0.0.1`. Dependendo da versão e da configuração de rede do Podman, uma
  caixa pode chegar como `127.0.0.1`. Se o teste mostrar isso, a confiança
  passa a ser um segredo que só o Caddy manda. Também ficam pendentes o Postgres fechado para fora e
  `--read-only` nas caixas.

Relacionados: ADR 010, 016, 022, 025, 027, 046.
