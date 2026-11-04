// Visual check of the neutral black-and-white look: public pages (logged out) and the main app screens (logged in),
// desktop 1440 and mobile 390; light and dark for /, /apps and /entrar. Strictly read-only: it only opens pages.
// Usage: BASE=http://localhost:5173 OUT=/path/to/shots node visual-neutro.mjs
import { chromium } from "playwright"
import { mkdirSync } from "node:fs"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "shots/visual-neutro"
// The dev Hub only accepts its own Origin for /api/dev/login; set LOGIN_ORIGIN when BASE is another port.
const LOGIN_ORIGIN = process.env.LOGIN_ORIGIN ?? BASE
mkdirSync(OUT, { recursive: true })

const sizes = { desktop: { width: 1440, height: 900 }, mobile: { width: 390, height: 844 } }
const browser = await chromium.launch()

// The theme preference lives in localStorage (default is light), so colorScheme alone does not switch it.
const themed = async (viewport, theme) => {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => { try { localStorage.setItem("apphub-theme", t) } catch { /* ignore */ } }, theme)
  return ctx
}

const shoot = async (ctx, path, name, tag) => {
  const page = await ctx.newPage()
  await page.goto(`${BASE}${path}`)
  await page.waitForLoadState("networkidle")
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${OUT}/${name}-${tag}.png`, fullPage: true })
  await page.close()
}

const publicPages = [["/boas-vindas", "boas-vindas"], ["/entrar", "entrar"], ["/cadastre-se", "cadastre-se"], ["/acesso", "acesso"], ["/convite", "convite"]]
const privatePages = [["/", "home"], ["/apps", "apps"], ["/artifacts/novo", "artifacts-novo"], ["/registrar", "registrar"], ["/conectar", "conectar"], ["/execucoes", "execucoes"]]
const both = new Set(["home", "apps", "entrar"])

for (const [sizeName, viewport] of Object.entries(sizes)) {
  for (const theme of ["light", "dark"]) {
    const ctx = await themed(viewport, theme)
    for (const [path, name] of publicPages) {
      if (theme === "dark" && !both.has(name)) continue
      await shoot(ctx, path, name, `${sizeName}-${theme}`)
    }
    await ctx.close()

    const lctx = await themed(viewport, theme)
    const r = await lctx.request.post(`${BASE}/api/dev/login`, { headers: { Origin: LOGIN_ORIGIN }, data: { email: "henry.kropp@alisinvestimentos.com" } })
    if (!r.ok()) throw new Error("login " + r.status())
    const apps = await (await lctx.request.get(`${BASE}/api/apps`)).json()
    const list = [...privatePages]
    if (Array.isArray(apps) && apps[0]) list.splice(2, 0, [`/apps/${apps[0].id}`, "app-ficha"])
    for (const [path, name] of list) {
      if (theme === "dark" && !both.has(name)) continue
      await shoot(lctx, path, name, `${sizeName}-${theme}`)
    }
    await lctx.close()
  }
}
await browser.close()
console.log("done", OUT)
