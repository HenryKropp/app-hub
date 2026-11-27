# Segurança Fase 0 (fechar furos) — Plano de construção

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fechar os 16 furos da Fase 0 sem quebrar apps, Agente nem Claude conectado.

**Architecture:** Mudanças pequenas e locais no backend Go (`backend/internal`), uma migração nova por assunto, duas telas no frontend (Admin › Pessoas e criação de empresa), scripts de VM em `deploy/`, e CI no GitHub. Nada roda na VM de produção nesta fase.

**Tech Stack:** Go 1.x (stdlib `crypto/ed25519`, `golang.org/x/sys/windows` para DPAPI), sqlc, pgx/v5, Postgres 17, Podman sem root, React + Vite + Vitest, GitHub Actions, `age`.

**Spec:** `docs/superpowers/specs/2026-11-24-seguranca-fase0-design.md`

## Global Constraints

- Testes Go: `cd backend && mise x -- go test ./...` (handlers precisam do Postgres `ziv-db` na 5433).
- Frontend: `cd frontend && npm test && npm run lint && npm run build`.
- SQL novo vai em `backend/internal/database/queries/*.sql` + `cd backend && mise x -- sqlc generate`; migração nova numerada a partir de `033_`.
- Textos para pessoas em português simples, sem jargão.
- Nunca rodar `scripts/deploy-gcp.sh` nem mexer na VM.
- Prazos: código do Claude 90 dias; computador parado 90 dias; sessão 7 dias sem uso (toque no máximo 1×/hora), teto 30 dias.
- Hosts OAuth permitidos: `localhost`, `127.0.0.1` (http, qualquer porta), `claude.ai`, `claude.com` e subdomínios (https), mais `OAUTH_REDIRECT_HOSTS` (lista separada por vírgula).
- Travas das caixas: `--cap-drop=ALL`, `--security-opt no-new-privileges`, `--pids-limit 256`.
- HSTS: `max-age=31536000` só com `BASE_URL` https.
- Um commit por tarefa, mensagem em português, com `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. Pedido direto (sem Caddy) com `X-Real-IP` falso não pode escolher a chave do limitador → teste em Task 1.
2. Valor guardado no formato antigo (sem cabeçalho) e cópia antiga continuam abrindo depois da Task 6 → teste em Task 6.
3. Pessoa desativada com conexão OAuth/código ativa ou Agente ligado é recusada na hora, não só no próximo login → teste em Task 4.
4. Agente com o bloco da empresa (`ZIVHUB01`) anexado ainda passa na conferência da assinatura → teste em Task 7.
5. Código do Claude criado antes desta versão continua valendo por 90 dias a partir da migração, não cai no dia do deploy → teste em Task 3.

---

### Task 1: Limite de tentativas por endereço real + IP/navegador no registro

**Files:**
- Create: `backend/internal/handler/remote.go` (move `remoteHost` de `external_page.go:131-145`)
- Modify: `backend/internal/handler/security_middleware.go:169-174` (usar `remoteHost(r)`), `auditStart` (gravar ip, user_agent)
- Create: `backend/internal/database/migrations/033_audit_origin.sql` (`ALTER TABLE security_audit ADD COLUMN IF NOT EXISTS ip TEXT NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT ''`)
- Modify: exportação em `handler/security.go` (incluir `ip`, `user_agent`)
- Test: `backend/internal/handler/remote_test.go`, teste novo em `handler_test.go` ou `auth_test.go`

**Interfaces:**
- Produces: `remoteHost(r *http.Request) string` (mesma semântica); `auditStart(ctx, actor *int64, action string, r *http.Request) (int64, error)` — todos os chamadores passam `r`.

- [ ] Teste `TestRemoteHost`: loopback + `X-Real-IP: 203.0.113.9` → `203.0.113.9`; `RemoteAddr 198.51.100.7:1234` + `X-Real-IP: 203.0.113.9` → `198.51.100.7`; loopback + `X-Real-IP: lixo` → `127.0.0.1`.
- [ ] Teste `TestLoginLimitPerAddress`: 300 POST `/api/auth/login` com `RemoteAddr 127.0.0.1` e `X-Real-IP 203.0.113.9` → o 301º é 429; o seguinte com `X-Real-IP 203.0.113.10` não é 429.
- [ ] Teste `TestAuditRecordsOrigin`: um POST autenticado grava `ip` e `user_agent` (cortado em 300 runas).
- [ ] Rodar, ver falhar; implementar; rodar `go test ./internal/handler/...`, ver passar.
- [ ] Commit `segurança: limite de tentativas por endereço real e origem no registro`.

### Task 2: Ziv só para dentro, travas nas caixas, cabeçalhos

**Files:**
- Modify: `backend/internal/config/config.go` (campo `ListenHost`, env `LISTEN_HOST`, padrão `127.0.0.1`)
- Modify: `backend/cmd/server/main.go:95` (`Addr: net.JoinHostPort(cfg.ListenHost, cfg.Port)`)
- Modify: `backend/internal/shared/engine/podman.go:291-295` (flags); `--read-only --tmpfs /tmp` só se Step de verificação passar
- Modify: `handler/security_middleware.go` `Secure` (HSTS quando `h.cfg.SecureCookies()`); CSP do site React onde o `index.html` é servido (procurar o handler de arquivos estáticos do frontend): `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; frame-src 'self' https:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'` (ajustar só o que o navegador mostrar quebrado)
- Modify: `docs/INFRASTRUCTURE.md` (`LISTEN_HOST`), `.env.example`
- Test: `podman_test.go` (args), `config_test.go`, teste de cabeçalho em `handler_test.go`

- [ ] Teste: `Start` monta args com `--cap-drop=ALL`, `--security-opt`, `no-new-privileges`, `--pids-limit`, `256`.
- [ ] Teste: config sem `LISTEN_HOST` → `127.0.0.1`; com `0.0.0.0` → `0.0.0.0`.
- [ ] Teste: `BASE_URL=https://…` → `Strict-Transport-Security: max-age=31536000`; http → ausente. `GET /` → CSP com `script-src 'self'`.
- [ ] Implementar; `go test ./...`.
- [ ] Verificação local (Podman): subir uma caixa de teste com as flags novas a partir de cada receita de `shared/recipes` usada no e2e (`e2e/compartilhados.mjs`), confirmar que sobe. De dentro da caixa: `curl host.containers.internal:8080` falha; Postgres 5433 responde. Se `--read-only` quebrar alguma receita, deixar fora e anotar em TASKS.
- [ ] Verificação no navegador (`npm run build` + servidor local): site abre sem erro de CSP no console.
- [ ] Commit `segurança: Ziv só atende de dentro, travas nas caixas, HSTS e CSP`.

### Task 3: Claude — prazo do código e lista de endereços

**Files:**
- Modify: `handler/mcp_auth.go` `createMCPCode` (`AccessExpiresAt = now+90d`), JSON da lista com `expires_at`
- Modify: query `CreateMCPConnection` (aceitar `access_expires_at`) em `queries/mcp.sql`
- Create: `migrations/034_mcp_code_expiry.sql`: `UPDATE mcp_connections SET access_expires_at = now() + interval '90 days' WHERE kind='code' AND access_expires_at IS NULL AND revoked_at IS NULL`
- Modify: `handler/mcp_oauth.go` `validRedirectURI(raw string, extra []string) bool`; config `OAuthRedirectHosts []string`
- Modify: refresh em `/oauth/token`: cliente cujo redirect não passa mais → `invalid_client`
- Modify: `frontend/src/components/ClaudeConnect.tsx` e lista no Admin: "Vale até dd/mm/aaaa"
- Test: `mcp_auth_test.go`, `mcp_oauth_test.go`, `ClaudeConnect.test.tsx`

- [ ] Testes: código novo tem expiração ≈ now+90d; código com expiração no passado → 401 no `/mcp`; migração dá 90 dias a código antigo (inserir antes, migrar, conferir).
- [ ] Testes `validRedirectURI`: aceita `http://localhost:33418/callback`, `http://127.0.0.1:5000/x`, `https://claude.ai/api/mcp/auth_callback`, `https://x.claude.com/cb`; recusa `https://evil.com/cb`, `https://claude.ai.evil.com/cb`, `http://claude.ai/cb`; aceita `https://ok.example/cb` só com `extra=["ok.example"]`.
- [ ] Teste frontend: data de validade aparece.
- [ ] Implementar; rodar Go e `npm test`.
- [ ] Commit `segurança: código do Claude vence em 90 dias e OAuth só volta para o Claude`.

### Task 4: Pessoas e sessões — desativar, inatividade, computador parado

**Files:**
- Create: `migrations/035_people_disabled.sql`: `people.disabled_at TIMESTAMPTZ`, `sessions.last_used_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Modify `queries/people.sql`: `GetSessionPerson` filtra `p.disabled_at IS NULL AND s.last_used_at > now() - interval '7 days'`; nova `TouchSession :exec` (atualiza só se `last_used_at < now() - interval '1 hour'`); `DisablePerson`, `EnablePerson`, `DeletePersonSessions`; `ListPeople` devolve `disabled_at`
- Modify `queries/agents.sql`: `RevokePersonComputers`, `RevokeIdleComputers :execrows` (`last_seen_at < now() - interval '90 days'`, ou `created_at` se nunca visto)
- Modify `queries/mcp.sql`: `RevokePersonMCPConnections`
- Modify: `handler/handler.go` `currentPerson` (chamar `TouchSession`; no caminho MCP recusar desativada), login por senha e entrada por convite (`entry.go`) recusam desativada com a mesma mensagem de senha errada; autenticação do Agente recusa desativada; `mcpConnection` recusa desativada
- Create: `handler/people_disable.go`: `POST /api/people/{id}/disable`, `POST /api/people/{id}/enable` (exige permissão de gerenciar pessoas, a mesma de `SetPersonRole`; recusa a si mesmo e o último administrador via `CountManagersExceptPerson`; grava `governance_events`/audit)
- Modify: `handler/scheduler.go` `schedulerTick`: chamar `RevokeIdleComputers` uma vez por hora
- Modify frontend: lista de pessoas do Admin com "Desativar"/"Reativar" e marca "Desativada"; `api.ts` `disablePerson(id)`, `enablePerson(id)`
- Test: `people_disable_test.go`, `auth_test.go`, `agents_test.go`, Admin test

- [ ] Testes: desativar → sessão existente dá 401; login por senha 401; conexão MCP (código e OAuth) 401; Agente do PC 401; reativar → login volta. Desativar a si mesmo → 400; último administrador → 400; pessoa sem permissão → 403.
- [ ] Teste: sessão com `last_used_at` de 8 dias → 401; de 6 dias → ok e toque atualiza.
- [ ] Teste: computador com `last_seen_at` de 91 dias → revogado por `RevokeIdleComputers`; de 89 → intacto.
- [ ] Teste frontend: botão Desativar chama a API e mostra a marca.
- [ ] Implementar; rodar tudo.
- [ ] Commit `segurança: desativar pessoa, sessão cai sem uso, computador parado perde acesso`.

### Task 5: Empresa nova escolhe o ponto de partida

**Files:**
- Modify `queries/artifacts.sql`: nova `ApplyStrictCompanyGovernance :exec` (`require_app_approval=TRUE, it_approval=TRUE, version_rule='risk', code_channel='any', mcp_restricted=TRUE`; garante `approve_it` no cargo admin)
- Modify `workspace/portal.go` `create(ctx, name, slug string, me sqlc.Person, strict bool)` e o handler de `POST /api/workspaces` (campo JSON `start: "strict" | "open"`, padrão `strict`); grava `governance_events` com a escolha
- Modify `frontend/src/pages/CompanySignupPage.tsx`: duas opções, "Começar com aprovações ligadas (recomendado)" marcada, e "Começar livre para testar"
- Test: `workspace/admin_test.go` ou novo `workspace/create_test.go`; `CompanySignupPage.test.tsx`

- [ ] Testes: `start` ausente ou `strict` → settings como acima e admin com `approve_it`; o criador sozinho aprova o próprio pedido (seguir o fluxo de `governance_test.go`); `open` → igual a hoje.
- [ ] Teste frontend: opção recomendada vem marcada e vai no corpo.
- [ ] Commit `governança: empresa nova escolhe começar com aprovações ligadas ou livre`.

### Task 6: Chave do cofre com versão e troca

**Files:**
- Modify `vault/vault.go`: `New(current string, old ...string) (*Vault, error)`; `Seal` grava `"ZK1" + keyID(4 bytes) + nonce + ct`; `Open` lê o cabeçalho, escolhe a chave pelo id, e cai no formato antigo (sem cabeçalho, tentando todas as chaves) se o cabeçalho não bater; `KeyID() [4]byte` = primeiros 4 bytes de SHA-256 da chave
- Modify `vault/stream.go`: cabeçalho do stream passa a carregar o `keyID` (versão nova do `streamMagic`); `OpenStream` aceita o antigo
- Modify config: `HUB_SECRET_KEYS_OLD` (base64 separadas por vírgula) passadas a `vault.New`
- Create `backend/cmd/trocarchave/main.go`: lê config, abre banco, recifra todas as colunas cifradas (as usadas por `Seal` em `app_keys.go`, `mcp_chaves.go`, `settings_secret.go`, `shared_config.go`, `deploy/env.go`) em todas as empresas (`workspace_registry`), e as cópias por app em disco (`shared_backups`); imprime quantos itens trocou; idempotente
- Modify `docs/INFRASTRUCTURE.md`: como trocar a chave
- Test: `vault_test.go`, `stream_test.go`, `cmd/trocarchave` teste com banco

- [ ] Testes: valor do formato antigo abre com a chave atual; valor selado com chave antiga abre quando ela está em `old`; sem ela → erro; `Seal` novo começa com `ZK1`; stream antigo abre; stream novo com chave antiga abre via `old`.
- [ ] Teste `trocarchave`: com dados selados pela chave A, rodar com atual=B e old=A → tudo abre só com B depois.
- [ ] Commit `segurança: chave do cofre com identidade e comando de troca`.

### Task 7: Agente — atualização assinada e token protegido

**Files:**
- Create `backend/internal/agent/updatekey.go`: `var UpdatePublicKey = "<base64 ed25519>"` (sobrescrevível por `-ldflags -X`)
- Create `backend/cmd/agentkey/main.go`: gera par ed25519 (`-out <dir>` grava `agente-assinatura.key` 0600 e imprime a pública) e assina (`sign -key <arq> <exe>` grava `<exe>.sig` em base64)
- Modify `agentproto/proto.go`: `UpdatePayload.Signature string \`json:"signature"\``
- Modify `handler/agent_update.go`: lê `AgentBinaryPath + ".sig"`; sem ele, não enfileira atualização e registra aviso
- Modify `agent/update.go` `download`: depois do hash, tirar o trailer (helper novo em `agentproto/trailer.go`: `StripTrailer(b []byte) []byte`) e `ed25519.Verify`; falha → `ErrBadSignature`
- Modify `agent/agent.go` `SaveToken`/`LoadToken`: em Windows, DPAPI (`windows.CryptProtectData`/`CryptUnprotectData`) em arquivo `agente.json` com campo `token_protegido` (base64); arquivo antigo com `token` em texto é lido e regravado protegido; build tags `token_windows.go`/`token_other.go`
- Modify `scripts/deploy-gcp.sh` passo 3: assinar com `~/.ziv-producao/agente-assinatura.key` (`ZIV_AGENT_SIGNING_KEY` sobrescreve); sem chave → para com aviso; envia `.sig`
- Modify `scripts/run-local.sh` e `scripts/build-agent.sh`: gerar par de teste em pasta local ignorada, compilar com `-X` da pública de teste, assinar
- Test: `agent/update_test.go`, `agentproto/trailer_test.go`, `handler/agent_update_test.go`, `agent/token_windows_test.go`

- [ ] Testes: binário assinado + trailer → aceito; byte trocado → `ErrBadSignature`; assinatura vazia → recusa; servidor sem `.sig` → não enfileira.
- [ ] Teste Windows (roda em Windows): `SaveToken`→arquivo não contém o token em texto; `LoadToken` devolve; arquivo antigo é migrado.
- [ ] Gerar a chave pública real **só depois** que o Henry gerar o par (Task 10); até lá o código usa a pública de teste e o deploy recusa sem a privada.
- [ ] Commit `segurança: Agente confere assinatura da atualização e protege o token no PC`.

### Task 8: Cópia do banco cifrada e fora da máquina

**Files:**
- Create `deploy/backup-banco.sh`: `podman exec ziv-db pg_dumpall -U <admin>` | `gzip` | `age -R ~/ziv/backup-recipient.txt` > `~/backups/ziv-AAAAMMDD-HHMM.sql.gz.age`; mantém as 7 mais novas; se `BACKUP_DEST` começa com `gs://`, `gcloud storage cp` com a conta de `GOOGLE_APPLICATION_CREDENTIALS`; sai ≠0 se qualquer passo falhar (`set -euo pipefail`); lê `~/ziv/backup.env`
- Create `deploy/ziv-backup.service`, `deploy/ziv-backup.timer` (06:30 UTC diário, `Persistent=true`)
- Create `deploy/backup.env.example`
- Modify `docs/INFRASTRUCTURE.md`: instalar, ligar, restaurar (`age -d -i chave.txt arquivo | gunzip | psql`), e remover o cron antigo
- Test: `scripts/test-backup.sh` local: roda o script contra o `ziv-db` local com `BACKUP_DEST` vazio e um par `age` temporário, decifra e confere que contém `CREATE DATABASE`

- [ ] Rodar `scripts/test-backup.sh` → passa.
- [ ] Restauração de teste: aplicar a cópia num Postgres temporário local e contar as tabelas.
- [ ] Commit `infra: cópia diária do banco cifrada e enviada para fora da máquina`.

### Task 9: Checagem automática no GitHub

**Files:**
- Create `.github/workflows/ci.yml`: jobs `backend` (Postgres 17 como serviço na 5433, mesmas variáveis dos testes; `go test ./...`; `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`) e `frontend` (Node 24; `npm ci`, `npm test`, `npm run lint`, `npm run build`, `npm audit --audit-level=high`)
- [ ] Conferir quais variáveis os testes de handler leem para achar o banco (`testdb_test.go`) e reproduzir no serviço.
- [ ] Rodar `govulncheck` e `npm audit` local; anotar o que achar em TASKS (não corrigir dependência fora do escopo, salvo alta severidade trivial).
- [ ] Commit `ci: testes, govulncheck e npm audit a cada push`.

### Task 10: ADR 048, TASKS e passo a passo do Henry

**Files:**
- Create `docs/adr/048-seguranca-fase0.md`: assinatura do Agente (chave fora do servidor), cópia fora da máquina com chave pública, chave do cofre com identidade, lista fechada do OAuth, Ziv só em 127.0.0.1
- Modify `docs/TASKS.md` "Roteiro de segurança": Fase 0 feita no branch (sem deploy), pendências do Henry, Fase 1 como próxima (decisão de 2026-11-24)
- Create `docs/seguranca-fase0-passo-a-passo.md`: o que o Henry faz (gerar chaves, bucket, conta de serviço, deploy, VM), na ordem
- [ ] Commit, push do branch.
