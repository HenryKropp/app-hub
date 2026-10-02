---
target: catálogo (tela principal)
total_score: 25
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/henrykroppalisinvest/app-hub/frontend/src/pages/CatalogPage.tsx"
target_fingerprint: "sha256:407abb8e59fae2d1682bfa57264f5bc48ffb4740136f71d0048ed2dcce89f451"
target_path: /home/henrykroppalisinvest/app-hub/frontend/src/pages/CatalogPage.tsx
timestamp: 2026-10-02T15-00-49Z
slug: frontend-src-pages-catalogpage-tsx
---
# Critique: catálogo (CatalogPage + RunDialog + RunStrip + Layout)
Method: dual-agent (A: design review · B: detector + browser overlay)

## Heuristics (25/40, Acceptable)
| # | Heuristic | Score | Key issue |
|---|---|---|---|
| 1 | Visibility of status | 3 | Remembered folder: Rodar starts at once, destination never shown on card/strip |
| 2 | Match real world | 3 | "Por padrão, a própria pasta do app" opaque; tail-truncated mono paths |
| 3 | User control | 2 | No Cancelar in dialog; no way to stop a run; "ocultar" one-way |
| 4 | Consistency | 3 | Mixed case on strip actions; dialog close sr-only "Close" in English |
| 5 | Error prevention | 2 | Default output = app folder (shared OneDrive); free-text folder unchecked |
| 6 | Recognition | 3 | Remembered folder invisible; card doesn't show last run |
| 7 | Flexibility | 2 | No "rodar em outra pasta" on card, no recents |
| 8 | Minimalist | 3 | "Em breve" items noise for usuário; 3 strip cards cover mobile |
| 9 | Error recovery | 2 | Raw error_message in strip; catalog load error bare e.message, no next step |
| 10 | Help | 2 | Nothing on what the app produces or what "sem arquivos novos" means |

## Specificity
Distinct paper/ink/stamp/folder-tab world, both themes. Card content generic (name, summary, area, owner); says nothing about output, version, last run.
Detector: CLI 0 findings in target TSX; 1 in frontend/src (bounce-easing index.css:202, stamp-in, deliberate). Browser overlay: undersized-ui-text (Janela badge 10.9px, sidebar 9.9–10.4px), low-contrast kicker #c83d22 on #f4f0e6 4.45:1, kicker-above-heading / cream-palette (deliberate style, false positive), gpt-thin-border-wide-shadow on card, layout-transition height, repeating-stripes-gradient on /execucoes.

## Priority issues
1. [P1] Output folder invisible at the moment of trust (card, strip, dialog). Fix: "Salva em: …" + trocar on card; echo in strip; readable path in dialog; plain default. → shape/clarify
2. [P1] Strip: no sort, so errors/done can fall into "+N" behind live runs (RunStrip.tsx:142); role=status on cards inserted with container (not announced); dark mode "Ver e baixar" white on ok 0.76 ≈2:1. Fix: sort error>done>no_files>live, persistent aria-live, role=alert for errors, dark text on green. → harden/audit
3. [P2] no_files shown with red warning, no next step; raw server errors; catalog load error without "Tentar de novo". → clarify
4. [P2] No escape hatches: Cancelar in dialog, Parar/Ver on live card, chip after ocultar. → harden
5. [P3] Labels: search aria-label, "Fechar", consistent case, hide "Em breve" for usuário, empty-catalog copy per role, Limpar busca, Janela badge size, kicker contrast. → polish

## Personas
Jordan: "JANELA" meaningless; pre-checked Lembrar makes a guess permanent; no_files reads as broken.
Sam: no skip link (8 tabs to search); strip not announced; "Close" in English; search unlabeled.
Analista com pressa: can't confirm where files went; Hub tab hidden behind app window when run ends; no tab-title/notification.

## Minor
Running badge in stamp red reads as alert; mobile nav cut off, logout/theme hidden; RunPage trust facts (version, folder) small under H1; Rodar de novo gives no destination feedback.

## Questions
Card shows last output ("Último: 12/09, 4 arquivos — Baixar")? Default folder per person instead of app folder? Finish signal via tab title/notification?
