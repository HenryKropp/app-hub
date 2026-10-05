// Visual check for Etapa 2 (ficha design B, Equipe). STRICTLY read-only on the
// real Hub: it only opens pages, opens/closes <details>, and clicks "Editar"
// then "Cancelar". It never saves, submits or changes anything.
// Usage: BASE=http://localhost:5173 OUT=e2e/shots/etapa2 node etapa2.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "shots/etapa2"
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
  const app = apps.find((a) => /files prep/i.test(a.name)) ?? apps[0]

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

  await page.goto(`${BASE}/apps/${app.id}`)
  await shot("ficha-fechada")
  const details = page.locator("summary", { hasText: "Para quem cuida do app" })
  if (await details.count()) {
    const open = await details.evaluate((s) => s.parentElement.open)
    if (!open) await details.click()
    await shot("ficha-tecnico-aberto")
    await details.click() // close again (persisted preference)
  }
  const editar = page.getByRole("button", { name: "Editar O que ele pede" })
  if (await editar.count()) {
    await editar.click()
    await shot("ficha-editando-pede")
    await page.getByRole("button", { name: "Cancelar" }).first().click()
  }

  for (const aba of ["pessoas", "areas", "cargos"]) {
    await page.goto(`${BASE}/equipe?aba=${aba}`)
    await shot(`equipe-${aba}`)
  }
  await ctx.close()
}
await browser.close()
