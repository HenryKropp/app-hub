// Visual check for Parar, against a throwaway Hub (never the real one): the
// strip with an open app and a stopped one, the confirmation, and the run page.
// Usage: BASE=http://localhost:8091 EMAIL=… PASSWORD=… OUT=shots/parar node parar.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:8091"
const OUT = process.env.OUT ?? "shots/parar"
const browser = await chromium.launch()

for (const [theme, viewport, tag] of [
  ["light", { width: 1280, height: 800 }, "desk-light"],
  ["dark", { width: 1280, height: 800 }, "desk-dark"],
  ["light", { width: 390, height: 844 }, "mobile"],
]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
  const page = await ctx.newPage()
  await page.request.post(`${BASE}/api/auth/login`, { data: { email: process.env.EMAIL, password: process.env.PASSWORD } })
  const shot = async (name) => {
    await page.waitForTimeout(1200)
    await page.screenshot({ path: `${OUT}/${tag}-${name}.png` })
    const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
    console.log("saved", tag, name, over ? "OVERFLOW-X" : "ok")
  }
  await page.goto(`${BASE}/`)
  await shot("faixa")
  // On a phone the strip shows one card, so the open app may sit in "+1".
  if (await page.getByRole("button", { name: "Parar" }).count()) {
    await page.getByRole("button", { name: "Parar" }).first().click()
    await shot("confirmar")
    await page.getByRole("button", { name: "Continuar rodando" }).click()
  }
  await page.goto(`${BASE}/execucoes/1`)
  await shot("execucao-aberta")
  await page.goto(`${BASE}/execucoes/2`)
  await shot("execucao-parada")
  await ctx.close()
}
await browser.close()
