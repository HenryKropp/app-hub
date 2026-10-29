---
name: Ziv
description: A neutral, fast work tool where only what you can start, and what is running now, is colored.
colors:
  run-coral: "#FF6B4A"
  run-coral-hover: "#F0562F"
  run-ink: "#11161A"
  logo-petrol: "#0E3B43"
  logo-petrol-night: "#256A75"
  logo-sand: "#F4EFE6"
  paper-white: "#FFFFFF"
  sidebar-mist: "#FAFAFA"
  ink: "#11161A"
  quiet-gray: "#6B6F73"
  pill-gray: "#F2F2F2"
  hover-gray: "#F0F0F0"
  hairline: "#EAEAEA"
  field-stroke: "#8C9196"
  night-ground: "#11161A"
  night-sidebar: "#141A1F"
  night-surface: "#161C21"
  night-ink: "#EEF0F1"
  night-quiet: "#A3A9AE"
  night-hairline: "#252C33"
  ok-green: "oklch(0.52 0.13 150)"
  error-red: "#B4232F"
  error-red-night: "#F2707B"
  attention-amber: "oklch(0.55 0.12 85)"
typography:
  display:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "2.25rem"
    fontWeight: 600
    lineHeight: 1.25
    letterSpacing: "-0.02em"
  headline:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.75rem"
    fontWeight: 600
    lineHeight: 1.25
    letterSpacing: "-0.02em"
  title:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.05rem"
    fontWeight: 600
    lineHeight: 1.375
    letterSpacing: "-0.02em"
  body:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.625
    fontFeature: "\"tnum\""
  body-lead:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.625
  label:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 500
    lineHeight: 1.33
  mono:
    fontFamily: "Geist Mono Variable, ui-monospace, monospace"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.33
rounded:
  sm: "0.3rem"
  md: "0.4rem"
  lg: "0.5rem"
  xl: "0.7rem"
  full: "9999px"
spacing:
  hair: "2px"
  xs: "6px"
  sm: "10px"
  md: "16px"
  lg: "20px"
  xl: "32px"
  page: "48px"
components:
  button-run:
    backgroundColor: "{colors.run-coral}"
    textColor: "{colors.run-ink}"
    rounded: "{rounded.full}"
    typography: "{typography.label}"
    padding: "0 14px 0 12px"
    height: "32px"
  button-run-hover:
    backgroundColor: "{colors.run-coral-hover}"
    textColor: "{colors.run-ink}"
  button-run-hero:
    backgroundColor: "{colors.run-coral}"
    textColor: "{colors.run-ink}"
    rounded: "{rounded.full}"
    padding: "0 24px 0 20px"
    height: "44px"
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper-white}"
    rounded: "{rounded.lg}"
    padding: "0 10px"
    height: "32px"
  button-outline:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.lg}"
    padding: "0 10px"
    height: "32px"
  kind-pill:
    backgroundColor: "{colors.pill-gray}"
    textColor: "{colors.quiet-gray}"
    rounded: "{rounded.full}"
    typography: "{typography.label}"
    padding: "2px 8px"
  app-card:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.xl}"
    padding: "20px"
  input:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.lg}"
    padding: "4px 10px"
    height: "36px"
  nav-item:
    backgroundColor: "{colors.sidebar-mist}"
    textColor: "{colors.quiet-gray}"
    rounded: "{rounded.md}"
    padding: "6px 10px"
  nav-item-active:
    backgroundColor: "{colors.hover-gray}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "6px 10px"
  run-strip-card:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.xl}"
    padding: "12px 16px"
---

# Design System: Ziv

## Overview

**Creative North Star: "The Play Button in a Quiet Room"**

Ziv is the category standard played straight: the clean, neutral work tool people already know from Notion and Vercel. White ground, a faintly gray sidebar, one-pixel hairlines, near-black text, gray secondary text, and Geist everywhere. Nothing decorates. The density is that of a tool used in a hurry between other tasks: cards with three lines of meaning, a page title, a search field, and the button you came for.

Into that quiet room goes exactly one vivid color, used the way Spotify uses its green. Coral (`#FF6B4A`) marks only two things: what you can start (the Rodar pill) and what is running right now (the animated "playing" bars). Coral is always a fill, never the color of a word or an icon. Because nothing else is colored, the eye finds Rodar and the live run without searching. Identity comes from that coral, the bars and the Ziv logo (the word "ziv" drawn in round strokes, the dot of the i in coral with three thin rays, like a lighthouse), not from the layout, which is intentionally familiar.

Light is the default theme; dark is a full peer, not an afterthought, with the same coral, which holds on near-black too. The system rejects the App Hub's earlier archive look (cream paper, serif, red stamp, a different color per app kind) and rejects rainbow status chips.

**Key Characteristics:**
- Neutral white, gray and near-black surfaces; one vivid coral accent, used only as a fill.
- Coral only on Rodar (and "+ Registrar apps"), the active nav marker, text selection and live runs. Focus is neutral.
- Status is always icon plus words, never a colored chip.
- App kinds are gray pills with an icon, never colored.
- Geist for all text; Geist Mono only for paths, versions, dates and durations.
- Flat surfaces with hairline borders; one soft shadow, reserved for floating things.
- Light by default, dark available.

## Colors

A grayscale world with a single coral and three small status hues. Values live as custom properties (hex, OKLCH for the status hues) in `frontend/src/index.css`; that file is normative.

### Primary
- **Run Coral** (`--brand`, `#FF6B4A`, both themes): fills only. The Rodar pill (label `--brand-foreground` `#11161A`, 6.5:1), "Abrir app" in the run dialog, the "playing" bars, the 3px marker left of the active nav item, the pulsing "Ligando" dot. Never text, icons or decoration.
- **Run Coral Hover** (`--brand-hover`, `#F0562F`): Rodar on hover (label 5.3:1).
- **Focus** (`--ring`): neutral, `#11161A` in light and `#EEF0F1` in dark. Rodar gets a 2px ring with a 2px offset so the ring never sits on the coral.
- **Logo Petrol** (`--logo-square`, `#0E3B43`, dark `#256A75`): only in the Ziv icon. Text selection uses Run Coral at 22% mixed into transparent; the caret is Ink.

### Neutral
- **Paper White**: page ground, cards, popovers and dialogs in light mode.
- **Sidebar Mist** (`--sidebar`): the left sidebar ground, a breath off white so the shell reads as a separate zone.
- **Ink** (`--foreground`, `--primary`): body text, headings, and the neutral filled button ("Ver e baixar" in the run strip, empty-state actions).
- **Quiet Gray** (`--muted-foreground`): descriptions, meta lines, labels, inactive nav text, kind-pill text.
- **Pill Gray** (`--secondary`): kind pills, avatar circles, the theme toggle track.
- **Hover Gray** (`--accent`): nav item hover and active fill, ghost hovers.
- **Hairline** (`--border`): every card, table, divider and strip border.
- **Field Stroke** (`--input`): input borders, the dashed empty-state border, card border on hover.
- **Night Ground / Night Sidebar / Night Surface / Night Ink / Night Quiet / Night Hairline**: the dark-mode counterparts of ground, sidebar, card, text, secondary text and border.

### Status
- **OK Green** (`--ok`): finished runs ("Concluída", "terminou: N arquivos novos"), "Conectado", added lines in diffs.
- **Error Red** (`--destructive`): errors and interrupted runs, error messages.
- **Attention Amber** (`--warn`): things a person must confirm or fix (no backup owner, "confirmar" notes in review, Agente disconnected).
- A run with no new files is plain Quiet Gray with an info icon.

### Named Rules
**The One Voice Rule.** Coral appears only as a fill: on Rodar and other run-starting actions, the active nav marker, selection and live runs. If something coral is neither startable, selected nor running, it is wrong, and coral is never the color of a word or an icon. One exception, Henry's call (2026-10-02): the catalog's "+ Registrar apps" is coral too, but square-cornered (8px) so it never reads as a Rodar pill.

**The No Rainbow Rule.** Kinds, areas, people and categories are never color-coded. Green, red and amber appear only as small status icons and the words beside them (soft tints are allowed only for diff lines and review notices), never as filled chips that compete with Rodar.

**The Light First Rule.** Light is the default theme on first visit. Dark is offered in the sidebar toggle (Claro, Escuro, Sistema) and must keep every rule above.

## Typography

**Body Font:** Geist Variable (with ui-sans-serif, system-ui)
**Heading Font:** Geist Variable, weight 600 with -0.02em tracking
**Mono Font:** Geist Mono Variable (with ui-monospace)

**Character:** One modern grotesk for everything, tightened for headings, with tabular figures on by default so counts and times line up. Mono is a precise instrument for machine-shaped facts, not a style.

### Hierarchy
- **Display** (600, 2.25rem at md and up, 1.875rem below, line-height 1.25): the app name at the top of an app's page (ficha).
- **Headline** (600, 1.75rem at md and up, 1.5rem below, line-height 1.25): page titles through the shared page header ("Apps da empresa", "Minhas execuções").
- **Title** (600, 1.05rem, line-height 1.375): the app name on catalog cards; section headings at 1.25rem in empty states and dialogs.
- **Body** (400, 0.875rem, line-height 1.625): descriptions, list rows, form text. Card descriptions clamp to two lines.
- **Body Lead** (400, 1rem, line-height 1.625, max 68ch): the summary under an app's name on its page.
- **Label** (500, 0.75rem): field hints, meta lines, kind pills, strip actions, field captions in the app facts row.
- **Mono** (400, 0.75rem to 0.8rem): folder paths ("salva em"), file names and patterns, versions, dates and durations.

### Named Rules
**The Mono Is Data Rule.** Geist Mono is used only for paths, versions, dates and durations. Separators and the words around them (the " · " between facts, "salva em", "trocar") stay in Geist sans: wrap only the value in mono, as the run lists do.

**The No Kicker Rule.** No eyebrow or kicker labels above titles. A page opens with its h1; small gray labels exist only as field captions directly above their value.

## Layout

A two-zone shell. On desktop (md, 768px and up) a 240px sticky sidebar runs full height on the left: the wordmark at top, nav, then a Configurações link and the account row pinned to the bottom, the account behind a hairline. The theme choice (Claro, Escuro, Igual ao sistema) lives on the Configurações page, below Meu computador; on phones, Configurações is in the avatar menu. Main content sits right with 48px side padding (32px at sm, 16px on phones), 40px top padding, and a centered max width of 72rem. Bottom padding is generous (160px) so the run strip never covers the last row.

On mobile the sidebar becomes a top bar: the wordmark and an avatar menu (theme and Sair live in that menu), with the nav as a horizontally scrolling row that fades out at the right edge.

The catalog is a grid of cards: one column on phones, two from sm, three from xl, 16px gaps. Page headers put the h1 left and the search (288px) right on desktop, stacked on mobile, with 32px below. The app page puts the name and summary left and the large Rodar right, then a bordered facts row (five columns on lg, two below), then label-and-value sections split by hairlines.

The run strip floats at the bottom of the viewport, aligned to the content column (max 48rem), stacking up to three cards (one on phones) with 8px gaps and a "+N em Minhas execuções" link beyond that.

Spacing rhythm is small and even: 2px, 6px and 10px inside controls; 16px and 20px for card and grid spacing; 32px between page header and content; 48px page gutters.

## Elevation & Depth

The system is flat. Depth comes from tonal layering (Sidebar Mist beside Paper White, Night Sidebar and Night Surface on Night Ground) and one-pixel hairlines. Cards at rest have no shadow; on hover their border darkens to Field Stroke and the ground tints by 1.5% of the text color.

### Shadow Vocabulary
- **Pop** (`--shadow-pop`; light `0 1px 2px oklch(0 0 0 / 0.04), 0 8px 24px -6px oklch(0 0 0 / 0.12)`, dark `0 1px 2px oklch(0 0 0 / 0.4), 0 12px 32px -8px oklch(0 0 0 / 0.7)`): only on things that float above the page, which means run-strip cards and the overflow link.
- **Extra small** (Tailwind `shadow-xs`): the selected segment of the theme toggle and native selects.
- Dialogs use a 1px ring at 10% of the text color over a 25% black scrim, no blur.

### Named Rules
**The Float Only Rule.** A shadow means "this is floating over the page". Cards, tables and panels that sit in the flow get a hairline, never a shadow.

**The No Glass Rule.** No decorative blur, glass, gradients or glows. Scrims are plain translucent black.

## Shapes

Soft, small radii from one 0.5rem base. Nav items and small controls use the medium step (6.4px), buttons and inputs the large step (8px), cards, the facts row, dialogs and strip cards the extra-large step (11.2px). Pills are fully round: the Rodar button, kind pills and avatar circles. Rodar is the only fully round button, which is part of how it is found. Empty states use a dashed Field Stroke border; everything else is a solid hairline. The logo is the Ziv wordmark: "ziv" drawn in round strokes (stroke 5 of a 60 by 50 box) in the text color, the dot of the i a coral circle (r 4) with three thin coral rays above it (straight up and at 45°). It sits alone at the top of the sidebar and on the sign-in page, with no separate text name. The icon (browser tab, small places) is a Logo Petrol square (rx 12 of 48) holding only the i: a Logo Sand stem, the coral dot and the rays; the square turns Logo Petrol Night in dark mode.

## Components

### Buttons
Quiet and compact, with one exception that is loud on purpose.
- **Rodar (run):** fully round, Run Coral fill, Ink semibold label, play icon filled with current color. Catalog size is 32px tall; the app page uses a 44px hero size. Hover switches to Run Coral Hover; focus shows a solid neutral ring with a 2px offset; pressing nudges it down 1px. "Abrir app" in the run dialog uses the same variant because it starts the run.
- **Rodar de novo:** the outline button with a filled Ink play icon, used in run lists where a full coral pill per row would be too loud.
- **Primary (neutral):** Ink fill, white text, 8px radius, hover at 80%. For non-run main actions ("Ver e baixar" on a finished run, "Registrar os primeiros apps").
- **Outline / Ghost / Secondary:** hairline or no border on the ground, hover fills Hover Gray. Destructive is a 10% red tint with red text.
- **Hover / Focus:** color transitions only; focus is a 3px neutral ring at 50% plus a neutral border; Rodar uses a solid 2px neutral ring with a 2px offset.

### Kind Pills
- **Style:** Pill Gray fill, Quiet Gray 12px medium text, a 14px Lucide icon per kind (script, web, desktop, scheduled, executable, reference, system), fully round, 2px by 8px padding.
- **State:** none. Kinds are never colored and never interactive.

### Status (RunState)
Icon plus words, always. Live: coral playing bars and Ink text. Concluída: green check. Sem arquivos novos: gray info icon. Erro and Interrompida: red warning triangle. Text is medium weight in the same color as its icon.

### Cards / Containers
- **Corner Style:** extra-large (11.2px).
- **Background:** Paper White (Night Surface in dark).
- **Shadow Strategy:** none at rest; see Elevation.
- **Border:** Hairline; Field Stroke on hover.
- **Internal Padding:** 20px. The catalog card holds the name (a link stretched over the whole card), a two-line description, then a bottom row: kind pill and "área · responsável" left, "aberto agora" and Rodar right. Rodar sits above the card link so both work.

### Inputs / Fields
- **Style:** 1px Field Stroke, 8px radius, transparent ground (Paper White in search), 32 to 36px tall, 14px text; the caret is Ink.
- **Focus:** border turns Ink (Night Ink in dark) with a 3px neutral ring at 50%.
- **Error / Disabled:** red border and 20% red ring; disabled at 50% opacity.
- **Field:** a 14px medium label above, an optional 12px gray hint below. Folder and command inputs are set in mono.

### Navigation
Sidebar rows are 14px text with a 16px icon, 6px by 10px padding, medium radius. Inactive rows are Quiet Gray and fill Hover Gray at 70% on hover; the active row fills Hover Gray, turns Ink and medium weight, with a 3px coral marker on its left edge; its icon turns Ink. "Minhas execuções" shows coral playing bars and an Ink count when runs are live. On mobile the nav is a scrolling row in the top bar.

### Run Strip (signature)
The bottom-of-screen stack that narrates each run: a floating card per run (Paper White or Night popover, hairline, extra-large radius, Pop shadow, 12px by 16px padding). Left is the status mark (playing bars, green check, gray info, red triangle), then a plain Portuguese sentence with the app name in semibold. Right are small 6px-radius actions: "Ver" and "Ocultar" while live; "Ver e baixar" as the neutral primary when done; "Ver detalhes", "Trocar pasta" or "Ver Meu computador" as outline actions; and a dismiss X. Cards are sorted problems first, then results, then live runs. Desktop shows up to three, phones one; the rest go into a "+N em Minhas execuções" pill, and hidden live runs into a "N aberto oculto · Mostrar" pill beside it.

### App Page (ficha, design B)
One page, parts in this order: header (name, summary with a quiet "Editar", kind pill · area · owner · backup · version, Rodar), O que ele pede, O que ele entrega, Execuções, Quem cuida, then a folded `<details>` "Para quem cuida do app" (only for people who may see the technical side). Each part is a two-column row: a semibold label on the left with "Editar" under it, the content on the right; on phones the columns stack. **Editar** is gray text, no border and no fill, underlined only on hover; it never competes with Rodar, which stays the only coral pill. A part in edit mode swaps its content for a hairline-bordered panel with Salvar (neutral primary) and Cancelar (ghost). The details summary starts with a "›" marker that turns when open, the title in semibold and, in 12px gray, "N pontos de atenção · código, outros apps, histórico, acesso". Inside, the same label-and-content rows: Pontos de atenção, Outros apps, Código-fonte, Bibliotecas, Como roda, Histórico, Onde está, Acesso.

### Team Page (Equipe)
A page title, then small pill tabs (Pessoas, Áreas, Cargos); the selected tab is the neutral Ink pill, the others outline. Tabs show only what the person may use. Each tab is one hairline card of rows: name in medium weight with gray facts beside it (apps, leaders, members; permissions written in Portuguese for roles) and quiet gray text actions below (Renomear, Apagar, Editar). Creating things uses a small field and a neutral button under the card. No color besides the active nav icon.

### Shared Status Dot (SharedStatusDot)
A 8px dot plus a short Portuguese word, 12px, for a shared app's state: green for "No ar", a pulsing coral dot for "Ligando", gray for "Descansando" or "Parado", red for "Caiu", and an empty outlined dot for "Ainda não publicado". These are the exact words the app shows (`frontend/src/lib/shared.ts`). It appears on the catalog card beside "Abrir" and in the ficha line. Coral is kept only for what is starting or can be started: **Abrir**, **Testar** and **Publicar** use the violet button; Parar, Ligar and Limites stay outline.

### Shared Part of the Ficha (Compartilhado)
A part in the same label-and-content row as the others. It holds the readiness checklist ("Checagem antes de publicar"): one row per item with a status icon (green check, amber triangle, gray info), a plain sentence, an "estou ciente" mark for warnings and a "Texto para o Claude" copy action. Below it come Testar and Publicar, then the settings, backups and, for people who see the technical side, logs and usage inside "Para quem cuida do app".

### Server Page (Servidor e gastos)
A page title, then a three-column row on desktop (stacked on phones, 16px gutters): memory meter, disk meter (turns amber with a sentence past 90%) and a hairline card with the month's spending in a tabular 20px figure. Meters are 8px tall rounded bars on the Pill Gray track. Under them: the monthly cost field with an outline "Salvar custo", then one row per shared app (name link, state dot, hours on, memory average / peak / reserved, database and backups, spend, and outline Parar / Ligar / Limites). On phones each row becomes a hairline card with labelled cells.

### Load Error
A page that didn't load shows a red warning triangle with the reason in plain Portuguese and an outline "Tentar de novo" button below. The browser's own English errors never reach the screen.

### Playing Bars (signature)
Three 3px coral bars, 2px apart and 0.85em tall, scaling between 30% and 100% height over 1s with staggered delays, like a music player's equalizer. They appear only on what is open on someone's computer right now: the strip, the catalog card ("aberto agora"), the nav count and the live status. With reduced motion they freeze at three different static heights.

### Theme Toggle
A Pill Gray segmented control in the sidebar footer (Claro, Escuro, Sistema, each with an icon); the selected segment lifts to the ground color with an extra-small shadow.

### Empty State
A dashed Field Stroke box, extra-large radius, 48px by 32px padding, left-aligned: a 1.25rem title, a gray paragraph (max 65ch) saying what to do next, and one action.

## Do's and Don'ts

### Do:
- **Do** keep every surface white, gray or near-black, and spend coral only as a fill: Rodar, the active nav marker, selection and live runs.
- **Do** make Rodar the only fully round, coral-filled button on a screen.
- **Do** show status as an icon plus words: green check for ok, red triangle for errors, gray info for no files, coral bars for live.
- **Do** show app kinds as gray pills with their icon.
- **Do** set paths, versions, dates and durations in Geist Mono, and keep the separators and words around them in Geist sans.
- **Do** always show which version ran and where the files were saved ("salva em").
- **Do** open the app in light mode by default and keep dark mode as a full peer with the same coral.
- **Do** write all UI copy in plain Brazilian Portuguese, in short, direct sentences.
- **Do** keep the playing bars static under reduced motion.

### Don't:
- **Don't** color app kinds, areas or people, or use colored status chips.
- **Don't** use coral for text, icons, decoration, headings, links, illustrations, backgrounds or focus rings.
- **Don't** put eyebrow or kicker labels above titles.
- **Don't** use decorative blur, glass, gradients or glows.
- **Don't** ship placeholder, sample or fake content. Show a real empty state that says what to do.
- **Don't** add shadows to cards or panels that sit in the page flow.
- **Don't** bring back the archive look: cream paper, serif type, stamps or per-kind colored tabs.
