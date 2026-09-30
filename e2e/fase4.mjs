// Visual check for Phase 4 (agent and runs). Needs a dev Hub with a paired
// computer, app 1 and run 1. Usage: BASE=http://localhost:5173 OUT=<dir> node fase4.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "/tmp"
const browser = await chromium.launch()

for (const [theme, viewport, tag] of [
  ["light", { width: 1440, height: 900 }, "desk-light"],
  ["dark", { width: 1440, height: 900 }, "desk-dark"],
  ["light", { width: 375, height: 812 }, "mobile"],
]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  const page = await ctx.newPage()
  await page.request.post(`${BASE}/api/dev/login`, { data: {} })

  const shot = async (name) => {
    await page.waitForTimeout(800)
    await page.screenshot({ path: `${OUT}/${tag}-${name}.png`, fullPage: true })
    console.log("saved", tag, name)
  }

  await page.goto(`${BASE}/meu-computador`)
  await shot("computador")
  await page.goto(`${BASE}/apps/1`)
  await shot("app")
  await page.getByRole("button", { name: "Rodar" }).click()
  await shot("rodar")
  await page.goto(`${BASE}/execucoes/1`)
  await shot("execucao")
  await ctx.close()
}
await browser.close()
