---
name: App Hub
description: A neutral, fast work tool where only what you can start, and what is running now, is colored.
colors:
  run-violet: "oklch(0.53 0.25 293)"
  run-violet-night: "oklch(0.62 0.22 293)"
  violet-wash: "oklch(0.955 0.03 293)"
  violet-wash-night: "oklch(0.27 0.07 293)"
  focus-violet: "oklch(0.56 0.24 293)"
  focus-violet-night: "oklch(0.7 0.18 293)"
  paper-white: "oklch(1 0 0)"
  sidebar-mist: "oklch(0.982 0 0)"
  ink: "oklch(0.205 0 0)"
  quiet-gray: "oklch(0.5 0 0)"
  pill-gray: "oklch(0.965 0 0)"
  hover-gray: "oklch(0.955 0 0)"
  hairline: "oklch(0.925 0 0)"
  field-stroke: "oklch(0.88 0 0)"
  night-ground: "oklch(0.145 0 0)"
  night-sidebar: "oklch(0.165 0 0)"
  night-surface: "oklch(0.18 0 0)"
  night-ink: "oklch(0.94 0 0)"
  night-quiet: "oklch(0.7 0 0)"
  night-hairline: "oklch(0.27 0 0)"
  ok-green: "oklch(0.52 0.13 150)"
  error-red: "oklch(0.55 0.21 27)"
  attention-amber: "oklch(0.55 0.13 65)"
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
    backgroundColor: "{colors.run-violet}"
    textColor: "{colors.paper-white}"
    rounded: "{rounded.full}"
    typography: "{typography.label}"
    padding: "0 14px 0 12px"
    height: "32px"
  button-run-hover:
    backgroundColor: "color-mix(in oklch, oklch(0.53 0.25 293), black 12%)"
    textColor: "{colors.paper-white}"
  button-run-hero:
    backgroundColor: "{colors.run-violet}"
    textColor: "{colors.paper-white}"
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

# Design System: App Hub

## Overview

**Creative North Star: "The Play Button in a Quiet Room"**

App Hub is the category standard played straight: the clean, neutral work tool people already know from Notion and Vercel. White ground, a faintly gray sidebar, one-pixel hairlines, near-black text, gray secondary text, and Geist everywhere. Nothing decorates. The density is that of a tool used in a hurry between other tasks: cards with three lines of meaning, a page title, a search field, and the button you came for.

Into that quiet room goes exactly one vivid color, used the way Spotify uses its green. Violet marks only two things: what you can start (the Rodar pill) and what is running right now (the animated "playing" bars). Because nothing else is colored, the eye finds Rodar and the live run without searching. Identity comes from that violet and from the bars, not from the layout, which is intentionally familiar.

Light is the default theme; dark is a full peer, not an afterthought, with a lighter violet so the accent keeps its punch on near-black. The system rejects the Hub's previous archive look (cream paper, serif, red stamp, a different color per app kind) and rejects rainbow status chips.

**Key Characteristics:**
- Neutral white, gray and near-black surfaces; one vivid violet accent.
- Violet only on Rodar, the active nav icon, focus and selection, and live runs.
- Status is always icon plus words, never a colored chip.
- App kinds are gray pills with an icon, never colored.
- Geist for all text; Geist Mono only for paths, versions, dates and durations.
- Flat surfaces with hairline borders; one soft shadow, reserved for floating things.
- Light by default, dark available.

## Colors

A grayscale world with a single saturated violet and three small status hues. All values are OKLCH and live as custom properties in `frontend/src/index.css`; that file is normative.

### Primary
- **Run Violet** (`--brand`, light): the Rodar pill, "Abrir app" in the run dialog, the active nav icon, the "playing" bars, "aberto agora" and the live-run count in the nav, the violet play icon on "Rodar de novo", and the selected file in a version's file list.
- **Run Violet Night** (`--brand`, dark): the same roles on dark ground; lighter so it holds contrast against near-black.
- **Focus Violet** (`--ring`, light and dark): focus rings and focused field borders, always at reduced opacity (40 to 50%).
- **Violet Wash** (`--brand-soft`, light and dark): a pale violet tint available for the accent's soft surface. Text selection uses Run Violet at 22% mixed into transparent; the text caret in inputs is Run Violet.

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
**The One Voice Rule.** Violet appears only on Rodar and other run-starting actions, the active nav icon, focus and selection, and live runs. If something violet is neither startable, focused, selected nor running, it is wrong.

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

A two-zone shell. On desktop (md, 768px and up) a 240px sticky sidebar runs full height on the left: logo and wordmark at top, nav, then account and theme toggle pinned to the bottom behind a hairline. Main content sits right with 48px side padding (32px at sm, 16px on phones), 40px top padding, and a centered max width of 72rem. Bottom padding is generous (160px) so the run strip never covers the last row.

On mobile the sidebar becomes a top bar: logo, wordmark and an avatar menu (theme and Sair live in that menu), with the nav as a horizontally scrolling row that fades out at the right edge.

The catalog is a grid of cards: one column on phones, two from sm, three from xl, 16px gaps. Page headers put the h1 left and the search (288px) right on desktop, stacked on mobile, with 32px below. The app page puts the name and summary left and the large Rodar right, then a bordered facts row (five columns on lg, two below), then label-and-value sections split by hairlines.

The run strip floats at the bottom of the viewport, aligned to the content column (max 48rem), stacking up to three cards with 8px gaps and a "+N em Minhas execuções" link beyond that.

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

Soft, small radii from one 0.5rem base. Nav items and small controls use the medium step (6.4px), buttons and inputs the large step (8px), cards, the facts row, dialogs and strip cards the extra-large step (11.2px). Pills are fully round: the Rodar button, kind pills and avatar circles. Rodar is the only fully round button, which is part of how it is found. Empty states use a dashed Field Stroke border; everything else is a solid hairline. The logo is a violet rounded square holding three white tiles and a play triangle.

## Components

### Buttons
Quiet and compact, with one exception that is loud on purpose.
- **Rodar (run):** fully round, Run Violet fill, white semibold label, play icon filled with current color. Catalog size is 32px tall; the app page uses a 44px hero size. Hover darkens the violet by mixing in 12% black; focus shows a violet ring at 40%; pressing nudges it down 1px. "Abrir app" in the run dialog uses the same variant because it starts the run.
- **Rodar de novo:** the outline button with a violet filled play icon, used in run lists where a full violet pill per row would be too loud.
- **Primary (neutral):** Ink fill, white text, 8px radius, hover at 80%. For non-run main actions ("Ver e baixar" on a finished run, "Registrar os primeiros apps").
- **Outline / Ghost / Secondary:** hairline or no border on the ground, hover fills Hover Gray. Destructive is a 10% red tint with red text.
- **Hover / Focus:** color transitions only; focus is a 3px ring of Focus Violet at 50% plus a violet border.

### Kind Pills
- **Style:** Pill Gray fill, Quiet Gray 12px medium text, a 14px Lucide icon per kind (script, web, desktop, scheduled, executable, reference, system), fully round, 2px by 8px padding.
- **State:** none. Kinds are never colored and never interactive.

### Status (RunState)
Icon plus words, always. Live: violet playing bars and violet text. Concluída: green check. Sem arquivos novos: gray info icon. Erro and Interrompida: red warning triangle. Text is medium weight in the same color as its icon.

### Cards / Containers
- **Corner Style:** extra-large (11.2px).
- **Background:** Paper White (Night Surface in dark).
- **Shadow Strategy:** none at rest; see Elevation.
- **Border:** Hairline; Field Stroke on hover.
- **Internal Padding:** 20px. The catalog card holds the name (a link stretched over the whole card), a two-line description, then a bottom row: kind pill and "área · responsável" left, "aberto agora" and Rodar right. Rodar sits above the card link so both work.

### Inputs / Fields
- **Style:** 1px Field Stroke, 8px radius, transparent ground (Paper White in search), 32 to 36px tall, 14px text; the caret is violet.
- **Focus:** border turns Focus Violet with a 3px violet ring at 50%.
- **Error / Disabled:** red border and 20% red ring; disabled at 50% opacity.
- **Field:** a 14px medium label above, an optional 12px gray hint below. Folder and command inputs are set in mono.

### Navigation
Sidebar rows are 14px text with a 16px icon, 6px by 10px padding, medium radius. Inactive rows are Quiet Gray and fill Hover Gray at 70% on hover; the active row fills Hover Gray, turns Ink and medium weight, and its icon alone turns violet. "Minhas execuções" shows violet playing bars and a count when runs are live. On mobile the nav is a scrolling row in the top bar.

### Run Strip (signature)
The bottom-of-screen stack that narrates each run: a floating card per run (Paper White or Night popover, hairline, extra-large radius, Pop shadow, 12px by 16px padding). Left is the status mark (playing bars, green check, gray info, red triangle), then a plain Portuguese sentence with the app name in semibold. Right are small 6px-radius actions: "Ocultar" while live; "Ver e baixar" as the neutral primary when done; "Ver detalhes", "Trocar pasta" or "Ver Meu computador" as outline actions; and a dismiss X.

### Playing Bars (signature)
Three 3px violet bars, 2px apart and 0.85em tall, scaling between 30% and 100% height over 1s with staggered delays, like a music player's equalizer. They appear only on what is open on someone's computer right now: the strip, the catalog card ("aberto agora"), the nav count and the live status. With reduced motion they freeze at three different static heights.

### Theme Toggle
A Pill Gray segmented control in the sidebar footer (Claro, Escuro, Sistema, each with an icon); the selected segment lifts to the ground color with an extra-small shadow.

### Empty State
A dashed Field Stroke box, extra-large radius, 48px by 32px padding, left-aligned: a 1.25rem title, a gray paragraph (max 65ch) saying what to do next, and one action.

## Do's and Don'ts

### Do:
- **Do** keep every surface white, gray or near-black, and spend violet only on Rodar, the active nav icon, focus and selection, and live runs.
- **Do** make Rodar the only fully round, violet-filled button on a screen.
- **Do** show status as an icon plus words: green check for ok, red triangle for errors, gray info for no files, violet bars for live.
- **Do** show app kinds as gray pills with their icon.
- **Do** set paths, versions, dates and durations in Geist Mono, and keep the separators and words around them in Geist sans.
- **Do** always show which version ran and where the files were saved ("salva em").
- **Do** open the app in light mode by default and keep dark mode as a full peer with the lighter violet.
- **Do** write all UI copy in plain Brazilian Portuguese, in short, direct sentences.
- **Do** keep the playing bars static under reduced motion.

### Don't:
- **Don't** color app kinds, areas or people, or use colored status chips.
- **Don't** use violet for decoration, headings, links, illustrations or backgrounds.
- **Don't** put eyebrow or kicker labels above titles.
- **Don't** use decorative blur, glass, gradients or glows.
- **Don't** ship placeholder, sample or fake content. Show a real empty state that says what to do.
- **Don't** add shadows to cards or panels that sit in the page flow.
- **Don't** bring back the archive look: cream paper, serif type, stamps or per-kind colored tabs.
