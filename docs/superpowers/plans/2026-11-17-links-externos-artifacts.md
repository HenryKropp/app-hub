# Links externos de artifacts — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Uma empresa do Ziv cria, para um artifact de arquivo HTML, um link `https://link.<SHARED_APPS_DOMAIN>/<empresa>/<código>` que abre sem login (aberto ou com código por e-mail), com prazo, cancelamento e registro de aberturas.

**Architecture:** Tabelas novas no schema de cada empresa (`external_links`, `external_link_codes`, `external_link_sessions`, `external_link_opens`). Regras puras num pacote novo `internal/extlink`. A equipe cria e cancela pela API normal da empresa. O endereço `link.` é interceptado em `gateway.Router.Root` antes dos apps compartilhados e entregue ao `Portal`, que acha a empresa pelo primeiro trecho do caminho e chama o handler dela. As páginas do lado de fora são HTML gerado no Go (`html/template`), sem o React, sem JavaScript e sem nenhum cookie da equipe.

**Tech Stack:** Go (net/http, html/template, pgx, sqlc), Postgres, React + Vitest, Resend (HTTP API) para e-mail.

**Spec:** `docs/superpowers/specs/2026-11-17-links-externos-artifacts-design.md`

## Global Constraints

- Português do Brasil em toda tela, e-mail e mensagem de erro.
- Prazos permitidos: 7, 30, 90, 365 dias; padrão 30. Não existe "sem prazo".
- Conteúdo: `latest` (padrão) ou `version` (fixa um `artifact_versions.id`).
- Tipos: `open` e `email`. Padrão `email` quando o envio de e-mail está configurado; senão `open`, e `email` aparece desabilitado com "Disponível em breve".
- Só artifacts com `kind = 'file'` ganham link.
- Permissão de cargo nova `share_external` ("Compartilhar fora da empresa"), ligada só no cargo `admin`.
- Link indisponível (não existe, vencido, cancelado, quem criou perdeu acesso, versão virou link) → sempre a mesma página, status 404: "Este link não está mais disponível".
- Código: 6 dígitos, vale 10 min, 5 tentativas erradas por código. Pedido de código: 5 por link e 20 por endereço de rede a cada 15 min (`securityLimiter`).
- Navegador liberado depois do código: 30 dias, sem passar do prazo do link.
- Abertura registrada a cada carga do conteúdo: data e hora, e o e-mail no link com código. Não guardar IP nem navegador.
- O HTML do artifact no link sai com o mesmo `artifactCSP` do ADR 018 mais `frame-ancestors 'self'`, dentro de um iframe com sandbox sem `allow-same-origin`.
- `link` vira nome reservado de empresa (`builtinReservedSlugs`) e de app compartilhado na principal.
- Nunca pôr nada da empresa em requisições para fora, exceto o e-mail do destinatário e o texto do e-mail enviado ao Resend.

## Review Focus

1. Link cancelado ou vencido enquanto alguém já está com o navegador liberado → a próxima carga do conteúdo mostra "não está mais disponível" (a liberação não sobrevive ao link). Teste em Task 4.
2. Quem criou o link perde `share_external`, perde a visão do artifact ou é apagado → o link para de abrir na hora. Teste em Task 2 (regra) e Task 4 (ponta a ponta).
3. Código de outra empresa ou caminho com empresa trocada (`/outra/<código-da-piloto>`) → indisponível, nunca o artifact da Empresa Piloto. Teste em Task 4.
4. Pedido de código para um link `open`, ou verificação com e-mail diferente do destinatário → recusado sem revelar o e-mail inteiro (só `h***@cliente.com`). Teste em Task 4.
5. `link.<domínio>` com o app "link" existente na principal ou empresa "link" → a página de link vence; o pedido de certificado do Caddy (`ask`) responde 200 para `link.<domínio>`. Teste em Task 4.

---

### Task 1: Banco, permissão e nome reservado

**Files:**
- Create: `backend/internal/database/migrations/029_external_links.sql`
- Create: `backend/internal/database/queries/external_links.sql`
- Modify: `backend/internal/access/access.go` (const `ShareExternal = "share_external"`, incluir em `RolePerms`)
- Modify: `backend/internal/config/config.go:49` (acrescentar `"link"` em `builtinReservedSlugs`)
- Modify: `frontend/src/lib/permissions.ts` (rótulo `share_external`: "Compartilhar fora da empresa", dica "Cria links de artifacts que abrem sem login, para gente de fora")
- Test: `backend/internal/handler/permissions_test.go`, `backend/internal/config/config_test.go`

**Interfaces — Produces (tabelas):**
```sql
external_links(id BIGSERIAL PK, token TEXT UNIQUE NOT NULL, artifact_id BIGINT NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
  version_id BIGINT REFERENCES artifact_versions(id) ON DELETE CASCADE,  -- NULL = sempre a mais recente
  kind TEXT NOT NULL CHECK (kind IN ('open','email')), recipient_email TEXT,  -- obrigatório sse kind='email'
  created_by BIGINT REFERENCES people(id) ON DELETE SET NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL, cancelled_at TIMESTAMPTZ, cancelled_by BIGINT REFERENCES people(id) ON DELETE SET NULL)
external_link_codes(id BIGSERIAL PK, link_id → external_links ON DELETE CASCADE, code_hash TEXT NOT NULL, expires_at TIMESTAMPTZ NOT NULL, wrong_tries INT NOT NULL DEFAULT 0, used_at TIMESTAMPTZ)
external_link_sessions(id BIGSERIAL PK, link_id → external_links ON DELETE CASCADE, secret_hash TEXT UNIQUE NOT NULL, email TEXT NOT NULL, expires_at TIMESTAMPTZ NOT NULL)
external_link_opens(id BIGSERIAL PK, link_id → external_links ON DELETE CASCADE, opened_at TIMESTAMPTZ NOT NULL DEFAULT now(), email TEXT)
```
A migração também faz `UPDATE roles SET permissions = array_append(permissions,'share_external') WHERE key='admin' AND NOT ('share_external' = ANY(permissions))` (mesmo padrão de `008_shared.sql`). O token fica em texto: quem lê o banco já lê o próprio HTML; anotar no ADR.

**Queries (sqlc, nomes fixos):** `CreateExternalLink`, `GetExternalLinkByToken`, `GetExternalLink`, `ListArtifactExternalLinks` (com nome de quem criou, contagem e última abertura), `ListCompanyExternalLinks` (idem + nome do artifact), `CancelExternalLink` (só se `cancelled_at IS NULL`), `CreateExternalLinkCode`, `GetLatestExternalLinkCode`, `AddExternalLinkCodeWrongTry`, `UseExternalLinkCode`, `CreateExternalLinkSession`, `GetExternalLinkSession` (por `secret_hash` e `link_id`, `expires_at > now()`), `RecordExternalLinkOpen`, `LatestArtifactVersion` (se ainda não existir equivalente — reusar o que existir).

- [ ] Step 1: Testes que falham: `TestAdminRoleHasShareExternal` (cargo admin de empresa nova tem `share_external`; cargo `member` não) e `TestLinkIsReservedSlug` (`cfg.ReservedSlugs["link"]`).
- [ ] Step 2: Rodar e ver falhar.
- [ ] Step 3: Escrever migração, queries, `mise x -- sqlc generate`, constantes e rótulo.
- [ ] Step 4: `cd backend && mise x -- go test ./internal/handler -run 'TestAdminRoleHasShareExternal' ./internal/config` → PASS.
- [ ] Step 5: Commit.

### Task 2: Regras puras `internal/extlink`

**Files:**
- Create: `backend/internal/extlink/extlink.go`, `backend/internal/extlink/extlink_test.go`

**Interfaces — Produces:**
```go
var ExpiryDays = []int{7, 30, 90, 365}; const DefaultExpiryDays = 30
const CodeTTL = 10 * time.Minute; const MaxWrongTries = 5; const SessionTTL = 30 * 24 * time.Hour
func NewToken() string                       // 32 bytes aleatórios, base64url sem '='
func NewCode() string                        // 6 dígitos, crypto/rand, com zeros à esquerda
func Hash(s string) string                   // sha256 hex
func ValidExpiry(days int) bool
func MaskEmail(email string) string          // "henry@cliente.com" → "h***@cliente.com"
type State struct { Now, ExpiresAt time.Time; Cancelled, CreatorGone, CreatorCanShare, CreatorSeesArtifact, ContentIsFile bool }
func Open(s State) bool                      // true só se nada fecha o link
func SessionExpiry(now, linkExpires time.Time) time.Time // min(now+SessionTTL, linkExpires)
```

- [ ] Step 1: Testes: `TestOpenClosesOnEachReason` (tabela: vencido em `Now == ExpiresAt` fecha; cancelado; criador apagado; sem permissão; sem visão; conteúdo não-arquivo; tudo ok abre), `TestNewCodeIsSixDigits` (1000 amostras casam `^\d{6}$`), `TestMaskEmail` (`a@b.com`→`a***@b.com`, `henry@cliente.com`→`h***@cliente.com`, e-mail sem `@` → `***`), `TestSessionExpiryCapsAtLink`, `TestValidExpiry` (7/30/90/365 sim; 0, 1, 366 não).
- [ ] Step 2: Rodar e ver falhar.
- [ ] Step 3: Implementar.
- [ ] Step 4: `mise x -- go test ./internal/extlink` → PASS.
- [ ] Step 5: Commit.

### Task 3: API da equipe e envio de e-mail

**Files:**
- Create: `backend/internal/mail/mail.go`, `backend/internal/mail/mail_test.go`
- Create: `backend/internal/handler/external_links.go`, `backend/internal/handler/external_links_test.go`
- Modify: `backend/internal/handler/handler.go` (rotas, campo `mail mail.Sender`, `SetMail`)
- Modify: `backend/internal/config/config.go` (`ResendAPIKey` de `RESEND_API_KEY`, `MailFrom` de `MAIL_FROM`; ambos fora do log de config)
- Modify: `backend/internal/workspace/portal.go` (`attach` chama `h.SetMail(...)`)

**Interfaces — Produces:**
```go
// package mail
type Sender interface { Send(ctx context.Context, to, subject, text string) error }
func NewResend(apiKey, from string) Sender   // POST https://api.resend.com/emails, Bearer, timeout 10s
type Log struct{}                            // DEV_MODE: escreve destino e texto no slog
func FromConfig(cfg config.Config) Sender    // Resend se chave e remetente; Log se DevMode; senão nil
```
Rotas (dentro de `h.Secure`, com sessão normal):
- `GET /api/artifacts/{id}/links` → `{"email_available": bool, "can_share": bool, "links": [linkJSON]}`
- `POST /api/artifacts/{id}/links` body `{"kind":"open"|"email","email":"…","content":"latest"|"version","version":N,"expires_days":30}` → 201 `linkJSON`
- `POST /api/links/{id}/cancel` → 204
- `GET /api/links` (só `manage_roles`) → `{"links":[linkJSON + "artifact_id","artifact_name"]}`

`linkJSON`: `id, url, kind, recipient (mascarado só na página de fora; aqui inteiro), content ("latest"|"version"), version_number, created_by_name, created_at, expires_at, status ("ativo"|"vencido"|"cancelado"|"fechado"), opens, last_opened_at`. `url = scheme + "://link." + SharedDomain + "/" + slug + "/" + token` (scheme `https` quando `cfg.SecureCookies()`).

Regras: criar exige `can.View` no artifact **e** `share_external`; artifact `kind='file'`; `email` exige `mail != nil` e e-mail válido; `version` exige versão do próprio artifact com `kind='file'`; prazo em `ExpiryDays`. Listar e cancelar no artifact: `can.View` + `share_external`. Status "fechado" quando `extlink.Open` dá falso por motivo de criador.

- [ ] Step 1: Testes que falham: `TestCreateLinkNeedsShareExternal` (403 "Você não tem permissão para compartilhar fora da empresa."), `TestCreateLinkRejectsLinkArtifact` (400), `TestCreateEmailLinkWithoutMailIs400` ("O envio de e-mail ainda não está ligado neste Ziv."), `TestCreateLinkBadExpiry` (400 para 0 e 366), `TestCreateAndListLink` (201, `url` termina com `/<slug>/<token>`, aparece na lista do artifact e em `GET /api/links`), `TestCancelLink` (204; depois status "cancelado"), `TestCompanyLinksOnlyForAdmin` (403 sem `manage_roles`), `TestResendSendsJSON` (servidor `httptest` recebe `from`, `to`, `subject`, `text` e `Authorization: Bearer k`).
- [ ] Step 2: Rodar e ver falhar (lembrar: testes de handler do Go não rodam em paralelo — `-p 1`).
- [ ] Step 3: Implementar.
- [ ] Step 4: `mise x -- go test -p 1 ./internal/handler -run 'Link' ./internal/mail` → PASS.
- [ ] Step 5: Commit.

### Task 4: O endereço `link.` e as páginas de fora

**Files:**
- Modify: `backend/internal/shared/gateway/router.go` (`Root` e `ask`)
- Modify: `backend/internal/workspace/portal.go` (método `ServeLink`)
- Create: `backend/internal/handler/external_page.go`, `backend/internal/handler/external_page.html`, `backend/internal/handler/external_page_test.go`
- Test: `backend/internal/shared/gateway/router_test.go`

**Interfaces:**
- `gateway.Router.SetLinks(h http.Handler)`; em `Root`, antes de `route`, se `strings.EqualFold(host, "link."+r.domain)` → `h`. `ask` responde 200 para `domain == "link."+r.domain`.
- `func (p *Portal) ServeLink(w, r)`: caminho `/<slug>/<token>[/<ação>]`; carrega a empresa com `p.load`; sem empresa → página indisponível; senão `e.handler.ServeExternal(w, r, token, ação)`. Guardar `*handler.Handler` em `entry` (hoje só guarda `http`). Em `main.go`: `router.SetLinks(http.HandlerFunc(portal.ServeLink))`.
- `func (h *Handler) ServeExternal(w http.ResponseWriter, r *http.Request, token, action string)` com ações:
  - `""` (GET): página. Link `open` → cabeçalho + iframe `src="<token>/conteudo"`. Link `email` sem sessão válida → "Este conteúdo foi enviado para h***@cliente.com." + botão "Enviar código" (form POST `codigo`). Com sessão → cabeçalho + iframe.
  - `codigo` (POST): limites; gera código, guarda hash, manda e-mail (assunto "Seu código para abrir <artifact>", texto com o código e "vale por 10 minutos"); mostra campo de 6 dígitos (form POST `confirmar`).
  - `confirmar` (POST): confere hash do código mais recente não usado e não vencido; errado soma tentativa (5 → código morto, "Peça um código novo"); certo marca usado, cria sessão (`secret` aleatório, guarda hash), cookie `ziv_link` = secret, `Path=/<slug>/<token>`, `HttpOnly`, `Secure` quando https, `SameSite=Lax`, `Expires` = `SessionExpiry`; redireciona 303 para a página.
  - `conteudo` (GET): recalcula `extlink.Open` (carrega quem criou com `GetPerson` + `loadAccess` + `ForArtifact`), exige sessão válida se `email`, grava abertura, serve o HTML da versão (fixa ou a mais recente) com os mesmos cabeçalhos de `artifactContent`.
- Cabeçalho da página: logo do Ziv (SVG de `frontend/src/components/bits.tsx` `Logo`, copiado para o template) dentro de `<a href="<origem de BASE_URL>/">`; "<Nome da empresa> compartilhou com você"; nome do artifact. CSP da página: `default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; frame-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'`. Sem JavaScript. Visual preto e branco do `DESIGN.md`.
- POSTs conferem `Origin`/`Sec-Fetch-Site` iguais ao próprio `link.` (mesmo critério de `security_middleware.go`).

- [ ] Step 1: Testes que falham (com `httptest` passando pelo `Router.Root` de verdade):
  - `TestLinkHostWinsOverSharedApp` e `TestAskAcceptsLinkHost`.
  - `TestOpenLinkServesPageAndContent` (200; página tem nome da empresa, nome do artifact e `href` para a origem do `BASE_URL`; conteúdo tem `Content-Security-Policy` com `sandbox` sem `allow-same-origin`; abertura gravada sem e-mail).
  - `TestLatestFollowsNewVersion` e `TestPinnedVersionStays`.
  - `TestClosedLinkReasonsAllLookTheSame` (cancelado, vencido, criador sem `share_external`, criador apagado, token inexistente, empresa trocada no caminho: mesmo corpo e 404).
  - `TestEmailLinkCodeFlow` (com `mail` falso: sem sessão, `conteudo` dá 403 e a página mostra "Enviar código"; pede código; e-mail falso recebeu 6 dígitos; código errado 5× mata; código certo dá cookie com `Path=/<slug>/<token>` e o conteúdo abre; abertura com o e-mail).
  - `TestSessionDiesWithLink` (sessão válida, depois cancelar → conteúdo indisponível).
  - `TestCodeRequestRateLimited` (6º pedido no mesmo link em 15 min → 429 com "Muitas tentativas. Espere alguns minutos.").
  - `TestPageMasksEmail` (corpo tem `h***@cliente.com` e não tem `henry@`).
- [ ] Step 2: Rodar e ver falhar.
- [ ] Step 3: Implementar.
- [ ] Step 4: `mise x -- go test -p 1 ./internal/handler -run 'Link|Email|Session|Code' ./internal/shared/gateway ./internal/workspace` → PASS.
- [ ] Step 5: Commit.

### Task 5: Telas da equipe

**Files:**
- Create: `frontend/src/components/ExternalLinks.tsx`, `frontend/src/components/ExternalLinks.test.tsx`
- Modify: `frontend/src/pages/ArtifactPage.tsx` (seção "Links externos" para artifact de arquivo, quando `can_share`)
- Modify: `frontend/src/pages/AdminPage.tsx` (seção "Links externos" com `GET /api/links`)
- Modify: `frontend/src/lib/api.ts` (ou onde ficam as chamadas) com `listArtifactLinks`, `createArtifactLink`, `cancelLink`, `listCompanyLinks`

**Comportamento:**
- Botão "Criar link para fora" abre formulário: Tipo (rádio "Código por e-mail" / "Link aberto"; e-mail desabilitado com "Disponível em breve" quando `email_available` falso, e aí "Link aberto" vem marcado), E-mail (só no tipo e-mail), Conteúdo ("Sempre a versão mais recente" / "Só a versão N" — N = versão atual), Prazo (7 dias, 30 dias, 90 dias, 1 ano; 30 marcado). Aviso fixo no link aberto: "Qualquer pessoa com este link consegue abrir."
- Depois de criar: o link aparece com botão "Copiar link".
- Lista: destinatário (ou "Link aberto"), conteúdo, quem criou, vence em, aberturas e última, status, "Copiar link" e "Cancelar" (confirmação: "Cancelar este link? Quem tiver o link não vai mais conseguir abrir."). Admin: mesma lista com coluna Artifact (link para a página dele).

- [ ] Step 1: Testes que falham: `cria link aberto e mostra Copiar link`, `e-mail desabilitado quando email_available é falso`, `Cancelar pede confirmação e chama a API`, `Admin mostra links da empresa com o nome do artifact`, `seção não aparece sem can_share`.
- [ ] Step 2: Rodar e ver falhar: `cd frontend && mise x -- npx vitest run src/components/ExternalLinks.test.tsx src/pages/AdminPage.test.tsx src/pages/ArtifactPage.test.tsx`.
- [ ] Step 3: Implementar no padrão visual preto e branco (ADR 020).
- [ ] Step 4: Mesmo comando → PASS; depois a suíte inteira do frontend.
- [ ] Step 5: Commit.

### Task 6: Fluxo real, fotos e documentos

**Files:**
- Create: `docs/adr/045-links-externos.md`; `e2e/links-externos.mjs`
- Modify: `docs/PRD.md` (seção de artifacts), `docs/TASKS.md`, `docs/INFRASTRUCTURE.md` (`RESEND_API_KEY`, `MAIL_FROM`, endereço `link.`; e-mail desligado em produção até ter domínio)

- [ ] Step 1: Subir o Ziv local (com `DEV_MODE`, nunca um Hub de teste na 8080) e fazer o fluxo inteiro: criar artifact HTML, criar link aberto, abrir `http://link.localhost:<porta>/<slug>/<token>` numa janela sem login, ver o conteúdo; subir versão nova e ver o link "mais recente" mudar; cancelar e ver "não está mais disponível"; criar link com código, pegar o código no log, abrir.
- [ ] Step 2: Fotos Playwright (`e2e/links-externos.mjs`): formulário de criar, lista no artifact, lista no Admin, página de fora aberta, página do código, página indisponível. Conferir alinhamento e contraste.
- [ ] Step 3: Escrever ADR 045 (decisões do spec + token em texto + páginas sem JS + e-mail desligado em produção até domínio próprio) e atualizar PRD, TASKS, INFRASTRUCTURE.
- [ ] Step 4: Suítes completas: `cd backend && mise x -- go test -p 1 ./...` e `cd frontend && mise x -- npx vitest run` → tudo PASS.
- [ ] Step 5: Commit e push.
