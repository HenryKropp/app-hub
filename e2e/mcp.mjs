// Visual check of the MCP screens in empresa Teste: "Conectar o Claude" in Configurações (1440 and 390, light and dark)
// and the consent page (Claude host and a foreign host). It creates one personal code and two pending OAuth requests,
// then cancels the code at the end.
// Usage: BASE=http://localhost:5173 OUT=/path/to/shots node mcp.mjs
import { chromium } from "playwright"
import { mkdirSync } from "node:fs"

const BASE = process.env.BASE ?? "http://localhost:5173"
const W = `${BASE}/w/teste`
const OUT = process.env.OUT ?? "shots/mcp"
mkdirSync(OUT, { recursive: true })

const sizes = { desktop: { width: 1440, height: 900 }, mobile: { width: 390, height: 844 } }
const browser = await chromium.launch()
const hdr = { Origin: BASE }

const themed = async (viewport, theme) => {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => { try { localStorage.setItem("apphub-theme", t) } catch { /* ignore */ } }, theme)
  const r = await ctx.request.post(`${W}/api/dev/login`, { headers: hdr, data: { email: "henry.kropp@alisinvestimentos.com" } })
  if (!r.ok()) throw new Error("login " + r.status())
  return ctx
}

const pending = async (ctx, redirect) => {
  const reg = await ctx.request.post(`${W}/oauth/register`, { data: { client_name: "Claude", redirect_uris: [redirect] } })
  const { client_id } = await reg.json()
  const q = new URLSearchParams({ response_type: "code", client_id, redirect_uri: redirect, code_challenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", code_challenge_method: "S256", state: "x" })
  const a = await ctx.request.get(`${W}/oauth/authorize?${q}`, { maxRedirects: 0 })
  return new URL(a.headers()["location"], BASE).searchParams.get("pedido")
}

const shoot = async (ctx, path, name) => {
  const page = await ctx.newPage()
  await page.goto(`${W}${path}`)
  await page.waitForLoadState("networkidle")
  await page.waitForTimeout(600)
  await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: true })
  await page.close()
}

const setup = await themed(sizes.desktop, "light")
const code = await (await setup.request.post(`${W}/api/mcp/codes`, { headers: hdr, data: { name: "notebook" } })).json()
const claudeReq = await pending(setup, "https://claude.ai/api/mcp/auth_callback")
const foreignReq = await pending(setup, "https://exemplo-estranho.com/cb")

for (const [sizeName, viewport] of Object.entries(sizes)) {
  for (const theme of ["light", "dark"]) {
    const ctx = await themed(viewport, theme)
    await shoot(ctx, "/configuracoes#claude", `configuracoes-${sizeName}-${theme}`)
    if (theme === "light") {
      await shoot(ctx, `/conectar-claude/autorizar?pedido=${claudeReq}`, `consentimento-${sizeName}`)
      await shoot(ctx, `/conectar-claude/autorizar?pedido=${foreignReq}`, `consentimento-estranho-${sizeName}`)
    }
    await ctx.close()
  }
}

await setup.request.delete(`${W}/api/mcp/connections/${code.id}`, { headers: hdr })
await setup.close()
await browser.close()
console.log("shots in", OUT)
