# Visual preto e branco + correções da crítica — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Put the Ziv back on the neutral black-and-white look that DESIGN.md describes, rebuild the public pages in that look, keep coral only on what starts or is running, and fix the critique's copy and dead-end-error findings.

**Architecture:** Frontend only. Theme tokens in `frontend/src/index.css`; public pages are `LandingPage`, `CompanyEntryPage`, `CompanySignupPage`, `LoginPage`, `InvitationPage` with their CSS block in `index.css` (from line ~175, the `.entry-*`, `.landing*`, `.auth-*`, `.login-*`, `.signup-*` rules). No backend change.

**Tech Stack:** React + Vite + TS, Tailwind v4, shadcn, Vitest, Playwright (e2e/).

**Spec:** `DESIGN.md` (binding visual rules) and `PRODUCT.md`; critique snapshot `.impeccable/critique/2026-11-04T14-11-03Z__frontend-src-app-tsx.md`. Henry's rulings on 2026-11-04: the neutral look is official; keep the name "Artifacts"; scope = visual + coral discipline + plain words + errors with a next step.

## Global Constraints

- Colors exactly as DESIGN.md: light `--background #FFFFFF`, `--foreground #11161A`, `--sidebar #FAFAFA`, `--accent #F0F0F0`, `--border #EAEAEA`, `--muted-foreground #6B6F73`, `--brand #FF6B4A`, `--brand-hover #F0562F`, `--brand-foreground #11161A`; dark `--background #11161A`, `--sidebar #141A1F`, `--card #161C21`, `--foreground #EEF0F1`, `--muted-foreground #A3A9AE`, `--border #252C33`, `--brand #FF6B4A`. The full previous blocks are in `git show 942c2d9^:frontend/src/index.css` (`:root` and `.dark`): restore those values verbatim.
- Geist Variable for all text, Geist Mono only for paths, versions, dates, durations. No serif anywhere (no Georgia), no italic display words, no uppercase mono "kickers" above titles, no gradients, no decorative shadows beyond `--shadow-pop` for floating things.
- Coral (`brand`) only as a fill, only on: Rodar / Abrir / Rodar de novo (the `run` button variant), "+ Registrar apps", the 3px active-nav marker, text selection, and live/running indicators (playing bars, "Ligando" dot). Never the color of text, links, icons, checkboxes or radios.
- All copy in simple Brazilian Portuguese; the product is "o Ziv" (never "Hub"). Keep the word "Artifacts".
- Every error a person can see says what to do next.
- Do not change backend code, routes, or test IDs other tests rely on. Keep existing Vitest tests passing; update copy assertions where copy changes.

## Review Focus

1. Dark theme: after the token swap, every screen (sidebar active item, cards, banners, the run strip) must stay readable — check `.dark` too, not just light.
2. Public pages at 390px width: no horizontal scroll, CTA visible without hunting.
3. Login flows still work end to end: /entrar (email+senha and "Entrar com Google" if present), /cadastre-se, /acesso, /convite?token= — only the look changes, not the behavior or field names.
4. No leftover coral text/links: grep for `text-brand`, `--coral`, `accent-[var(--brand)]`, `fill-brand` after the change.
5. The landing page must not show invented customers, numbers or a fake product name ("Aurora Studio"); an illustration of the real Ziv catalog, labeled as an example, is fine.

---

### Task 1: Neutral tokens and coral discipline inside the app

**Files:**
- Modify: `frontend/src/index.css` (`:root` lines ~52–88, `.dark` ~89–125, header comment line 9, the `body`/app-level rules before line ~175 only)
- Modify: `frontend/src/pages/NewArtifactPage.tsx:124,150`, `frontend/src/pages/ArtifactsPage.tsx:118` (and its "+ Novo artifact" button), `frontend/src/components/FavoriteStar.tsx:32`, `frontend/src/pages/AdminPage.tsx:226`
- Test: existing `*.test.tsx` of the touched files

**Interfaces:** Produces the neutral token set Task 2 builds on. Consumes nothing.

- [ ] Step 1: Restore the `:root` and `.dark` token values from `942c2d9^` (Global Constraints). Change the line-9 comment to describe the neutral look. Leave the public-page CSS (from ~line 175) for Task 2.
- [ ] Step 2: "Criar artifact" (`NewArtifactPage.tsx:150`) and "+ Novo artifact" use the default (ink) button, not `run`. Radio (`:124`) and Admin checkbox (`:226`) use `accent-foreground` (ink), not brand. ArtifactsPage icon at `:118` uses `text-muted-foreground`. FavoriteStar filled state: `fill-foreground text-foreground`.
- [ ] Step 3: `grep -rn "text-brand\|fill-brand\|accent-\[var(--brand)\]" frontend/src --include=*.tsx` returns nothing outside `components/ui/button.tsx`.
- [ ] Step 4: Update any test asserting the old classes; run `cd frontend && mise x -- npx vitest run` and `mise x -- npx tsc -b` — all pass.
- [ ] Step 5: Commit `Visual preto e branco: cores neutras e coral só no Rodar`.

### Task 2: Public pages rebuilt in the neutral look

**Files:**
- Modify: `frontend/src/pages/LandingPage.tsx`, `CompanyEntryPage.tsx`, `CompanySignupPage.tsx`, `LoginPage.tsx`, `InvitationPage.tsx`
- Modify: `frontend/src/index.css` public-page block (~line 175 to end): rewrite or delete; prefer Tailwind classes in the components and keep only what Tailwind can't express (the `.playing` bars and `.entry-rise` motion with its reduced-motion rule stay).
- Test: the five pages' `*.test.tsx`

**Interfaces:** Consumes Task 1 tokens (`bg-background`, `text-foreground`, `text-muted-foreground`, `border-border`, `bg-sidebar`, `Button` variants). Keeps every field `id`/label, handler, route and API call unchanged.

- [ ] Step 1: Read DESIGN.md fully and the impeccable craft floor (`/home/henrykroppalisinvest/.claude/plugins/cache/impeccable/impeccable/4.5.0/skills/impeccable/reference/craft-floor.md`). Mode: /boas-vindas is Persuade, the four auth pages are Operate.
- [ ] Step 2: /boas-vindas: Geist only, white ground, ink text, hairlines. Hero = one clear sentence about what the Ziv does (keep the existing Portuguese message, minus the italic word), primary CTA "Cadastre-se" as the ink default button and "Entrar" as outline. Replace the "Aurora Studio" window and the orbit diagram with an honest mini illustration of the real Ziv catalog (2–3 app cards with gray kind pill and a coral Rodar pill), captioned "Exemplo ilustrativo". Keep the sections' meaning (menos procura; encontrar/usar/rodar no computador; um espaço por empresa; chamada final) but no numbered kickers, no dark petrol band, no serif.
- [ ] Step 3: /entrar, /cadastre-se, /acesso, /convite: one centered card (max-w ~sm/md) on white, Ziv logo above, Geist title, the same fields and buttons. Remove the petrol story panel, kickers, coral rings and coral links (links = ink with underline). Primary submit = default ink button.
- [ ] Step 4: Delete now-unused CSS; `grep -n "Georgia\|serif\|--coral\|kicker\|eyebrow" frontend/src/index.css` returns nothing.
- [ ] Step 5: Vitest + tsc pass (update copy assertions only where copy changed). Commit `Páginas públicas no visual preto e branco`.

### Task 3: Plain words and errors with a next step

**Files:**
- Modify: `frontend/src/pages/RegisterPage.tsx:107,111,158`, `frontend/src/components/ScriptImportForm.tsx:184,208`, `frontend/src/pages/CompanyEntryPage.tsx:20,27,33,47,55`, `frontend/src/pages/ConnectPage.tsx`, `frontend/src/lib/api.ts:326`, `frontend/src/components/ficha/shared/settings.ts:28`
- Test: the matching `*.test.tsx`; add `ConnectPage.test.tsx` cases

**Interfaces:** None new. If Task 2 already restyled CompanyEntryPage, only change its words.

- [ ] Step 1: RegisterPage: every "Hub" → "Ziv"; the label "Alternativa legada · somente Hub no computador" → "Outra forma: analisar uma pasta (só quando o Ziv roda no seu computador)", as normal-case text, not an uppercase kicker.
- [ ] Step 2: ScriptImportForm `:184` → "Envie só scripts em que você confia. O Ziv roda o script numa pasta separada, mas ele ainda consegue acessar o que a sua conta do computador acessa." Replace the native file input `:208` with a visible outline button "Escolher arquivos" (the input becomes `sr-only`, the button triggers it) and a line showing "Nenhum arquivo escolhido" or the chosen count/names, in Portuguese.
- [ ] Step 3: CompanyEntryPage: label "Endereço da empresa no Ziv", hint "É a última parte do link da sua empresa. Ex.: em ziv.exemplo.com/w/horizonte, digite horizonte." Error texts: replace "identificador" with "endereço" and end with "Se não souber, pergunte a quem administra o Ziv na sua empresa."
- [ ] Step 4: ConnectPage: when loading the pairing fails (no code, bad or unknown code), show a title "Não encontramos esse pedido de conexão", the line "Abra o Agente do Ziv no seu computador de novo; ele abre esta página com o código certo.", and two actions: link "Ir para Configurações" (`/configuracoes`) and link "Como instalar o Agente" (`/instalar-agente`). Approve failure keeps its message plus the same two actions. Test: render with no code → those texts and links.
- [ ] Step 5: Generic fallback in `api.ts:326` and `settings.ts:28`: "Algo deu errado. Tente de novo; se continuar, avise quem administra o Ziv." Update tests asserting the old string.
- [ ] Step 6: Vitest + tsc pass. Commit `Palavras simples e erros com próximo passo`.

### Task 4: Visual check and documents

**Files:**
- Create: `e2e/visual-neutro.mjs`; `docs/adr/020-visual-preto-e-branco.md`
- Modify: `DESIGN.md` (fix line ~296: Abrir/Testar/Publicar use the coral `run` button — there is no violet; add a short "Public pages" note: same neutral system, Geist, ink CTAs), `docs/TASKS.md` (Done rows), `docs/PRD.md` only if a requirement changed

- [ ] Step 1: Screenshot script (login via `/api/dev/login` like `e2e/home.mjs`, read-only): /boas-vindas, /entrar, /cadastre-se, /acesso, /convite (logged out); /, /apps, /apps/:first-id, /artifacts/novo, /registrar, /conectar, /execucoes (logged in); desktop 1440 and mobile 390; light and dark for /, /apps, /entrar.
- [ ] Step 2: Look at every screenshot; fix what is off; one more round at most.
- [ ] Step 3: ADR 020: context (942c2d9 drift, critique 22/40), decision (neutral official, Artifacts name kept), alternatives (keep cream/petrol; mixed).
- [ ] Step 4: Commit `Visual preto e branco: conferência visual e documentos`.
