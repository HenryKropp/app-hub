#!/usr/bin/env node
// Confere o contraste WCAG dos tokens de cor do frontend/src/index.css,
// nos temas claro e escuro. Sai com código 1 se algum par ficar abaixo do mínimo.
// Uso: node scripts/contrast-check.mjs
import { readFileSync } from "node:fs"

const css = readFileSync(new URL("../frontend/src/index.css", import.meta.url), "utf8")

function block(selector) {
  const start = css.indexOf(`${selector} {`)
  if (start < 0) throw new Error(`bloco ${selector} não encontrado`)
  const body = css.slice(start, css.indexOf("\n}", start))
  return Object.fromEntries([...body.matchAll(/--([\w-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]))
}

// Converte "#RRGGBB", "#RRGGBBAA" ou "oklch(L C H [/ A])" em [r, g, b, a] (sRGB gama, 0..1).
function parse(value) {
  if (value.startsWith("#")) {
    const h = value.slice(1)
    const n = (i) => parseInt(h.slice(i, i + 2), 16) / 255
    return [n(0), n(2), n(4), h.length === 8 ? n(6) : 1]
  }
  const m = value.match(/oklch\(\s*([\d.]+)(%?)\s+([\d.]+)\s+([\d.]+)\s*(?:\/\s*([\d.]+)(%?))?\s*\)/)
  if (!m) throw new Error(`cor não reconhecida: ${value}`)
  const L = m[2] ? +m[1] / 100 : +m[1]
  const C = +m[3]
  const H = (+m[4] * Math.PI) / 180
  const alpha = m[5] === undefined ? 1 : m[6] ? +m[5] / 100 : +m[5]
  const a = C * Math.cos(H)
  const b = C * Math.sin(H)
  const l_ = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3
  const m_ = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3
  const s_ = (L - 0.0894841775 * a - 1.291485548 * b) ** 3
  const lin = [
    4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_,
    -1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_,
    -0.0041960863 * l_ - 0.7034186147 * m_ + 1.707614701 * s_,
  ]
  const gamma = (x) => {
    x = Math.min(1, Math.max(0, x))
    return x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055
  }
  return [...lin.map(gamma), alpha]
}

// Mistura alfa no espaço sRGB gama, como o navegador faz ao pintar.
function over(fg, bg) {
  const a = fg[3]
  return [0, 1, 2].map((i) => fg[i] * a + bg[i] * (1 - a))
}

function luminance([r, g, b]) {
  const lin = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}

function ratio(fg, bg) {
  const solidBg = over(bg, [1, 1, 1, 1])
  const [l1, l2] = [luminance(over(fg, solidBg)), luminance(solidBg)].sort((x, y) => y - x)
  return (l1 + 0.05) / (l2 + 0.05)
}

const TEXT = 4.5
const UI = 3
const pairs = [
  ...["foreground", "muted-foreground", "destructive", "warn", "ok"].flatMap((fg) =>
    ["background", "card"].map((bg) => [fg, bg, TEXT]),
  ),
  ["muted-foreground", "sidebar", TEXT],
  ["muted-foreground", "muted", TEXT],
  ["foreground", "accent", TEXT],
  ["brand-foreground", "brand", TEXT],
  ["brand-foreground", "brand-hover", TEXT],
  ["input", "background", UI],
  ["input", "card", UI],
  ["ring", "background", UI],
]

let failed = 0
for (const [theme, selector] of [["claro", ":root"], ["escuro", ".dark"]]) {
  const tokens = { ...block(":root"), ...(selector === ":root" ? {} : block(selector)) }
  for (const [fg, bg, min] of pairs) {
    const r = ratio(parse(tokens[fg]), parse(tokens[bg]))
    const ok = r >= min
    if (!ok) failed++
    console.log(`${ok ? "ok  " : "FALHA"} ${theme.padEnd(6)} ${fg} sobre ${bg}: ${r.toFixed(2)}:1 (mínimo ${min})`)
  }
}
console.log(failed ? `\n${failed} par(es) abaixo do mínimo.` : "\nTodos os pares passam.")
process.exit(failed ? 1 : 0)
