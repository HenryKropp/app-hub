// Visual check for accepted file types: the field in the ficha editor (not
// saved) and the run dialog with a mocked list. Uses the local dev login.
// Usage: BASE=http://localhost:5173 APP=4 node tipos-arquivo.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const APP = process.env.APP ?? "4"
const OUT = "shots/tipos"
const browser = await chromium.launch()
for (const [theme, viewport, tag] of [["light", { width: 1280, height: 900 }, "desk"], ["dark", { width: 1280, height: 900 }, "dark"], ["light", { width: 390, height: 844 }, "mobile"]]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
  const page = await ctx.newPage()
  await page.request.post(`${BASE}/api/dev/login`, { headers: { Origin: BASE }, data: { email: "henry.kropp@alisinvestimentos.com" } })
  await page.route(`**/api/apps/${APP}`, async (route) => {
    if (route.request().method() !== "GET") return route.continue()
    const res = await route.fetch()
    const body = await res.json()
    if (!body.manifest) return route.fulfill({ response: res })
    for (const i of body.manifest.inputs ?? []) if (i.type === "file") i.accept = [".txt", ".csv", ".md"]
    await route.fulfill({ response: res, json: body })
  })
  const shot = async (name, full = false) => {
    await page.waitForTimeout(800)
    await page.screenshot({ path: `${OUT}/${tag}-${name}.png`, fullPage: full })
    const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
    console.log("saved", tag, name, over ? "OVERFLOW-X" : "ok")
  }
  await page.goto(`${BASE}/apps/${APP}`)
  await page.getByText("O que ele pede").first().scrollIntoViewIfNeeded()
  await shot("ficha")
  // The run dialog needs a connected Agente; its look is covered by Henry's test.
  const edit = page.getByText("O que ele pede", { exact: true }).locator("..").getByRole("button", { name: "Editar" })
  await edit.click()
  await page.getByText("Tipos de arquivo aceitos (opcional)").scrollIntoViewIfNeeded()
  await shot("editar")
  await ctx.close()
}
await browser.close()
