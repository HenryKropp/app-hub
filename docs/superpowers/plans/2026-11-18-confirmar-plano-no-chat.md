# Confirmar o plano de teste no chat — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `enviar_versao`/`ver_versao` reach "esperando confirmação", the Ziv asks the owner/admin directly in Claude Code (MCP elicitation) and, on Confirmar, confirms the plan and starts the test; otherwise the site link stays.

**Architecture:** The MCP endpoint becomes stateful (sessions bound to the token's person) so the server can send `elicitation/create` mid tool call. Confirmation logic is pulled out of `confirmTestPlan` into one function used by the site route and by the MCP tools. A new column records where the plan was confirmed.

**Tech Stack:** Go, modelcontextprotocol/go-sdk v1.8.0 (`ServerSession.Elicit`, client `ElicitationHandler`), Postgres/sqlc, React + Vitest.

**Spec:** `docs/superpowers/specs/2026-11-18-confirmar-plano-no-chat-design.md`

## Global Constraints
- The model can never confirm: no new tool, no route added to `mcpRoutes`; `POST /api/apps/{id}/test-plan/confirm` keeps refusing MCP (`msgPlanSiteOnly`).
- Only owner or admin (`ownerOrAdmin`) gets the box; everyone else gets today's message with the link.
- Client without elicitation capability, decline, cancel or any elicit error → today's link message, nothing confirmed.
- PC choice limited to the confirmer's own non-revoked computers (same as `planChanger`). One PC → pre-selected.
- Fingerprint is computed when the box is built and checked again on accept (`msgPlanChanged` if it moved).
- All user text in Portuguese, plain; never write the forbidden names (see the project memory on names).

## Review Focus
- Session hijack: a session ID used with another person's token must be refused.
- Box open while the Ziv restarts or the person takes long: the call ends without confirming; the link still works.
- Person with no PC (or only revoked/old-agent PCs): no box, the message says to install/connect the Agente, as today.
- Plan changed between box and click: refused with `msgPlanChanged`, nothing stored.
- Many PCs with the same name: the enum must still map to the right ID.

---

### Task 1: Stateful MCP sessions bound to the person

**Files:**
- Modify: `backend/internal/handler/mcp_server.go` (`mcpHTTPFor`)
- Test: `backend/internal/handler/mcp_server_test.go`

**Interfaces:**
- Produces: tool handlers receive a live `req.Session` that can `Elicit`.

- [ ] Step 1: test `TestMCPSessionBoundToPerson`: open a session with person A's token, then send a request with A's `Mcp-Session-Id` and person B's token → not 200 (SDK answers 403/404); A's own follow-up call works.
- [ ] Step 2: run, expect FAIL (stateless ignores the session).
- [ ] Step 3: `Stateless: false`, `SessionTimeout: 30 * time.Minute`; after the bearer check put `auth.TokenInfo{UserID: <person id as string>, Expiration: <far>}` into the request context the SDK reads (wrap with `auth.RequireBearerToken` and a verifier that calls `h.mcpConnection`, keeping today's 401 text/headers).
- [ ] Step 4: run the new test and every `TestMCP*`/`TestEnviar*`/`TestVer*` test → PASS.
- [ ] Step 5: commit.

### Task 2: One confirmation function, with "where"

**Files:**
- Create: `backend/internal/database/migrations/032_test_plan_via.sql` (`ALTER TABLE app_test_plans ADD COLUMN IF NOT EXISTS confirmed_via TEXT NOT NULL DEFAULT 'site'`)
- Modify: `backend/internal/database/queries/*` (UpsertTestPlan gets `confirmed_via`), `backend/internal/handler/test_plans.go`
- Test: `backend/internal/handler/test_plans_test.go`

**Interfaces:**
- Produces: `func (h *Handler) confirmPlan(ctx context.Context, a sqlc.App, me sqlc.Person, pc int64, fingerprint, via string) (code, msg string, err error)` — code "" on success, else `test_plan_missing` / `plan_changed`; stores and calls `advanceWaiting`. `planConfirmed` gains `Via string json:"via"`.

- [ ] Step 1: test: site confirm → `via == "site"` in GET test-plan.
- [ ] Step 2: FAIL; Step 3: migration, sqlc generate, `confirmTestPlan` calls `confirmPlan(..., "site")`.
- [ ] Step 4: `go test ./internal/handler -run 'TestPlan|TestTestPlan|TestConfirm'` PASS; commit.

### Task 3: The box in enviar_versao and ver_versao

**Files:**
- Create: `backend/internal/handler/mcp_confirmar.go`
- Modify: `backend/internal/handler/mcp_versao.go` (`startVersaoTest`, `toolVerVersao`: where `d.ConfirmURL != ""`)
- Test: `backend/internal/handler/mcp_confirmar_test.go` (go-sdk client with `ElicitationHandler`)

**Interfaces:**
- Consumes: `confirmPlan` (Task 2), `req.Session` (Task 1), `testPlanView`.
- Produces: `func (h *Handler) askPlanInChat(ctx context.Context, ss *mcp.ServerSession, appID int64) (confirmed bool, note string)`.

Box: message = "Confirmar o plano de teste de <app>?" + lines "Usa as chaves: …", "Acessa: …", "Pula: …", "Roda no seu PC." Schema: one property `computador` (string enum of "<nome> (nº <id>)", default the connected one) only when there are 2+ PCs; with one PC no properties. Accept → `confirmPlan(..., "claude_code")`, then the tool reports as today minus the link ("Plano confirmado por você no Claude Code. O teste começou.").

Tests:
- [ ] `TestChatConfirmStartsTest`: owner, client accepts → plan stored with via `claude_code`, draft left `esperando_confirmacao`, text has no link.
- [ ] `TestChatConfirmDeclinedKeepsLink`: decline → nothing stored, text has the link.
- [ ] `TestChatConfirmNoCapabilityKeepsLink`: client without handler → no elicit, link.
- [ ] `TestChatConfirmOnlyOwnerOrAdmin`: another creator in the area → handler never called, link.
- [ ] `TestChatConfirmPlanChangedRefused`: fingerprint changed inside the handler before accepting → nothing stored, `msgPlanChanged` in text.
- [ ] `TestChatConfirmNoPC`: owner without PC → no box, today's text.
- [ ] Run each RED first, implement, `go test ./internal/handler -run 'TestChatConfirm|TestEnviar|TestVerVersao'` PASS; commit.

### Task 4: Site shows where it was confirmed

**Files:** `frontend/src/pages/TestPlanPage.tsx`, its test, `frontend/src/lib/api.ts` (type gets `via`).

- [ ] Vitest: confirmed with `via: "claude_code"` shows "Plano confirmado por Henry pelo Claude Code em dd/mm."; FAIL → implement → PASS; commit.

### Task 5: Docs, full suite, ship

- [ ] ADR 047 (exception to "plano só pelo site": the box answer comes from the program, not the model), PRD 3.x line, TASKS, MCP instructions text if it mentions the link.
- [ ] Full Go suite (`go test -p 1 ./...`, grep the log for FAIL) and `npm test`.
- [ ] Final review (fresh reviewer, most capable model) → fix pass → merge.
- [ ] Deploy only with Henry's explicit OK; hands-on: Henry asks his Claude Code for a new app version that needs confirmation and sees the box.
