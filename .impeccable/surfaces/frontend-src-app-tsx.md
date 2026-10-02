---
version: 1
slug: "frontend-src-app-tsx"
primary_target: "frontend/src/App.tsx"
related_targets: ["frontend/src/components/Layout.tsx"]
---

# App Hub: whole app (shell, catalog, ficha, runs, register, people, computer, login)

Scope: every screen of the Hub web app, one shared shell. Visitor mode: Operate.
Audience: analysts and managers in a hurry on Windows office PCs; creators who register apps; admins.
Job: find the app, press Rodar, get the files; know which version ran and where files went.
Constraints: PRODUCT.md copy and functions stay; pt-BR plain language; light by default with dark; no per-app screens.
Chosen: the category standard played straight (Notion, Vercel), plus one vivid violet used the way Spotify uses its green. User asked: "cleanzinho, fácil de ver, não muito colorido, uma cor de destaque e o resto neutro".
Memorable moment: the violet Rodar pill and the violet "tocando" bars on whatever is open right now.

## Direction contract

THESIS: A neutral, fast work tool where only one thing is ever colored: what you can start, and what is running now. Refuses the old archive look (cream paper, serif, red stamp, colored tabs per kind) and refuses rainbow status chips.

OWN-WORLD: White ground, #FAFAFA sidebar, 1px #EAEAEA hairlines, near-black #171717 text, gray secondary. Geist for everything, Geist Mono only for paths, versions and times. One vivid violet (oklch ~0.54 0.24 293): Rodar pill, active nav icon, focus ring, live run. Kinds are gray pills with a small icon, never colored. Green/red/amber only as small status icons and text. Dark: #0A0A0A ground, #111 surfaces, lighter violet.

STORY: The visitor sees the list of apps, reads name and what it does, presses the violet Rodar, watches the run in the strip, clicks Ver e baixar. Trust comes from version and "salva em" always shown in mono.

FIRST VIEWPORT: Left 240px light sidebar: wordmark with violet mark, nav, user and theme at bottom. Main: h1 "Apps da empresa" left (no eyebrow), search right, then a grid of white bordered app cards: name, two-line description, área · responsável, gray kind pill, violet Rodar pill bottom right. Run strip stacks bottom right.

FORM: The category standard (canon, safer register, steered), chosen after two re-rolls; seed key fb71f91e. Signature interaction: live runs show animated violet equalizer bars (static under reduced motion) in the strip, the card and the nav count.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
