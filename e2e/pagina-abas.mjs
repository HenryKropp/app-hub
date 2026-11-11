// Visual check for the app page in tabs (ADR 039). Read-only: starting a run is blocked.
// Usage: BASE=http://localhost:5199 OUT=<dir> node pagina-abas.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "/tmp"
const browser = await chromium.launch()
for (const [viewport, tag] of [[{ width: 1366, height: 800 }, "desk"], [{ width: 390, height: 844 }, "mobile"]]) {
  const ctx = await browser.newContext({ viewport, colorScheme: "light" })
  await ctx.addInitScript(() => localStorage.setItem("apphub-theme", "light"))
  const page = await ctx.newPage()
  await page.route("**/api/apps/*/runs", (r) => (r.request().method() === "POST" ? r.abort() : r.continue()))
  await page.request.post(`${BASE}/api/dev/login`, { data: {} })
  const apps = await (await page.request.get(`${BASE}/api/apps`)).json()
  console.log(apps.map((a) => `${a.id}:${a.manifest?.kind ?? a.kind}:${a.shared?.status ?? "-"}:${a.manifest?.name ?? a.name}`).join("\n"))
  const pick = [apps.find((a) => a.shared), apps.find((a) => (a.manifest?.kind ?? a.kind) === "script")].filter(Boolean)
  for (const a of pick) {
    await page.goto(`${BASE}/apps/${a.id}`)
    await page.getByRole("tablist", { name: "Partes do app" }).waitFor()
    const tabs = await page.getByRole("tab").allTextContents()
    for (const [i, t] of tabs.entries()) {
      await page.getByRole("tab").nth(i).click()
      await page.waitForTimeout(600)
      await page.screenshot({ path: `${OUT}/${tag}-${a.id}-${i}-${t.replace(/\W+/g, "")}.png`, fullPage: tag === "desk" && i === 0 })
      if (tag === "mobile" && i > 0) break
    }
    if (tag === "desk" && a.shared) {
      await page.getByRole("tab").first().click()
      const more = page.getByRole("button", { name: /^Mais/ })
      if (await more.count()) { await more.click(); await page.waitForTimeout(400); await page.screenshot({ path: `${OUT}/${tag}-${a.id}-mais.png` }); await page.keyboard.press("Escape") }
    }
  }
  await ctx.close()
}
await browser.close()
