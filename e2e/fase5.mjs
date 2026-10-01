// Visual check for Phase 5 (history and restore). Needs a dev Hub with app 1.
// Opens the restore dialog but never confirms it.
// Usage: BASE=http://localhost:5173 OUT=<dir> node fase5.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "/tmp"
const browser = await chromium.launch()

for (const [theme, viewport, tag] of [
  ["light", { width: 1280, height: 720 }, "desk-light"],
  ["dark", { width: 1280, height: 720 }, "desk-dark"],
  ["light", { width: 375, height: 812 }, "mobile"],
]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  const page = await ctx.newPage()
  await page.request.post(`${BASE}/api/dev/login`, { data: {} })

  const shot = async (name) => {
    await page.waitForLoadState("networkidle", { timeout: 60000 })
    await page.getByText("Carregando").first().waitFor({ state: "detached", timeout: 60000 }).catch(() => {})
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${OUT}/${tag}-${name}.png` })
    await page.screenshot({ path: `${OUT}/${tag}-${name}-full.png`, fullPage: true })
    console.log("saved", tag, name)
  }

  await page.goto(`${BASE}/apps/1`)
  await shot("ficha")
  await page.goto(`${BASE}/apps/1/versoes/1`)
  await shot("versao-mudou")
  await page.goto(`${BASE}/apps/1/versoes/1?ver=arquivos`)
  await page.waitForLoadState("networkidle", { timeout: 60000 })
  const first = page.locator("main button:not([disabled])").filter({ hasText: ".py" }).first()
  if (await first.count()) await first.click()
  await shot("versao-como-estava")
  const restore = page.getByRole("button", { name: /Voltar o código/ })
  if (await restore.count()) {
    await restore.first().click()
    await shot("restaurar")
  }
  await ctx.close()
}
await browser.close()
