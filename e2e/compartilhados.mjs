// Visual check for Apps compartilhados (catálogo, ficha com a parte
// "Compartilhado" e "Para quem cuida do app" abertos, página /servidor).
// STRICTLY read-only on the real Hub: it only opens pages and opens/closes
// <details>. It never saves, publishes, tests, stops or changes anything.
// Run only after the Hub was restarted with the new code.
// Usage: BASE=http://localhost:5173 OUT=e2e/shots/compartilhados node compartilhados.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "shots/compartilhados"
const browser = await chromium.launch()

for (const [theme, viewport, tag] of [
  ["light", { width: 1280, height: 800 }, "desk-light"],
  ["dark", { width: 1280, height: 800 }, "desk-dark"],
  ["light", { width: 390, height: 844 }, "mobile"],
]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
  const page = await ctx.newPage()
  await page.request.post(`${BASE}/api/dev/login`, { data: {} })
  const apps = await (await page.request.get(`${BASE}/api/apps`)).json()
  // Prefer an app that is already shared; otherwise fall back to the first one.
  const app = apps.find((a) => a.shared) ?? apps[0]

  const shot = async (name) => {
    await page.waitForLoadState("networkidle", { timeout: 60000 }).catch(() => {})
    await page.getByText("Carregando").first().waitFor({ state: "detached", timeout: 60000 }).catch(() => {})
    await page.waitForTimeout(600)
    await page.screenshot({ path: `${OUT}/${tag}-${name}.png`, fullPage: true })
    const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
    console.log("saved", tag, name, over ? "OVERFLOW-X" : "ok")
  }

  await page.goto(`${BASE}/`)
  await shot("catalogo")

  if (app) {
    await page.goto(`${BASE}/apps/${app.id}`)
    await shot("ficha-compartilhado")
    const details = page.locator("summary", { hasText: "Para quem cuida do app" })
    if (await details.count()) {
      const open = await details.evaluate((s) => s.parentElement.open)
      if (!open) await details.click()
      await shot("ficha-tecnico-aberto")
      await details.click() // close again (persisted preference)
    }
  } else {
    console.log("no apps found; skipping ficha")
  }

  await page.goto(`${BASE}/servidor`)
  await shot("servidor")
  await ctx.close()
}
await browser.close()
