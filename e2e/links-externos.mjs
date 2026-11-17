// Visual check for external links (ADR 045): the artifact section and form,
// the Admin list, and the pages people outside the company see.
// Run against an isolated DEV_MODE Ziv (never the one on 8080):
//   BASE=http://localhost:8090 OUT=shots/links LOG=<server log> node links-externos.mjs
// With LOG (the DEV_MODE server log, where codes are printed) it also types
// the code in the browser and checks the content opens.
import { chromium } from "playwright"
import { mkdirSync, readFileSync } from "node:fs"

const BASE = process.env.BASE ?? "http://localhost:8090"
const OUT = process.env.OUT ?? "shots/links"
mkdirSync(OUT, { recursive: true })

const html = `<!doctype html><html><head><meta charset="utf-8"><style>body{font-family:sans-serif;margin:24px}</style></head>
<body><h2>Carta aos cotistas — outubro</h2><p>O fundo rendeu 2,3% no mês, contra 1,1% do índice.</p></body></html>`

const browser = await chromium.launch()
const login = async (ctx) => {
  const r = await ctx.request.post(`${BASE}/api/dev/login`, { headers: { Origin: BASE }, data: { email: "admin@exemplo.com" } })
  if (!r.ok()) throw new Error("login " + r.status())
}
const seed = await browser.newContext()
await login(seed)
const api = seed.request
const hdr = { Origin: BASE }
const art = await (await api.post(`${BASE}/api/artifacts`, { headers: hdr, data: { name: "Carta aos cotistas", summary: "Carta mensal", tags: [], area_id: 1, kind: "file", file_name: "carta.html", content_base64: Buffer.from(html).toString("base64") } })).json()
const mk = async (data) => (await api.post(`${BASE}/api/artifacts/${art.id}/links`, { headers: hdr, data })).json()
const open = await mk({ kind: "open", content: "latest", version: 1, expires_days: 30 })
const email = await mk({ kind: "email", email: "cliente@fora.com", content: "version", version: 1, expires_days: 7 })
const gone = await mk({ kind: "open", content: "latest", version: 1, expires_days: 90 })
await api.post(`${BASE}/api/links/${gone.id}/cancel`, { headers: hdr })

const check = async (page, name) => {
  const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
  console.log("saved", name, over ? "OVERFLOW-X" : "ok")
}

for (const [theme, viewport, tag] of [["light", { width: 1280, height: 800 }, "desk"], ["dark", { width: 1280, height: 800 }, "dark"], ["light", { width: 390, height: 844 }, "mobile"]]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
  await login(ctx)
  const page = await ctx.newPage()
  await page.goto(`${BASE}/artifacts/${art.id}`)
  await page.getByRole("button", { name: "Criar link para fora" }).waitFor()
  await page.locator("#links-externos").scrollIntoViewIfNeeded()
  await page.waitForTimeout(800)
  await page.screenshot({ path: `${OUT}/secao-${tag}.png`, fullPage: true })
  await check(page, `secao-${tag}`)
  await page.getByRole("button", { name: "Compartilhar" }).click()
  await page.getByRole("dialog", { name: "Compartilhar fora da empresa" }).waitFor()
  await page.waitForTimeout(300)
  await page.screenshot({ path: `${OUT}/janela-${tag}.png` })
  await check(page, `janela-${tag}`)
  await page.getByRole("dialog").getByRole("radio", { name: /Link aberto/ }).check()
  await page.getByRole("dialog").getByRole("button", { name: "Criar link" }).click()
  await page.getByRole("button", { name: "Pronto" }).waitFor()
  await page.screenshot({ path: `${OUT}/janela-criado-${tag}.png` })
  await page.getByRole("button", { name: "Pronto" }).click()
  await page.getByRole("button", { name: "Criar link para fora" }).click()
  await page.waitForTimeout(300)
  await page.screenshot({ path: `${OUT}/form-${tag}.png`, fullPage: true })
  await check(page, `form-${tag}`)
  await page.goto(`${BASE}/admin`)
  await page.getByRole("heading", { name: "Links externos" }).waitFor()
  await page.getByRole("heading", { name: "Links externos" }).scrollIntoViewIfNeeded()
  await page.waitForTimeout(800)
  await page.screenshot({ path: `${OUT}/admin-${tag}.png`, fullPage: true })
  await check(page, `admin-${tag}`)

  // Outside the company: a fresh browser with no Ziv login.
  const out = await browser.newContext({ viewport, colorScheme: theme })
  const op = await out.newPage()
  for (const [name, url] of [["fora-aberto", open.url], ["fora-codigo", email.url], ["fora-indisponivel", gone.url]]) {
    await op.goto(url)
    await op.waitForTimeout(800)
    await op.screenshot({ path: `${OUT}/${name}-${tag}.png` })
    await check(op, `${name}-${tag}`)
  }
  await op.goto(email.url)
  await op.getByRole("button", { name: "Enviar código" }).click()
  await op.getByLabel("Código").waitFor()
  await op.screenshot({ path: `${OUT}/fora-digite-${tag}.png` })
  await check(op, `fora-digite-${tag}`)
  if (process.env.LOG) {
    const codes = [...readFileSync(process.env.LOG, "utf8").matchAll(/Seu código: (\d{6})/g)]
    await op.getByLabel("Código").fill(codes.at(-1)[1])
    await op.getByRole("button", { name: "Abrir" }).click()
    await op.frameLocator("iframe").getByText("O fundo rendeu").waitFor()
    await op.screenshot({ path: `${OUT}/fora-aberto-email-${tag}.png` })
    await check(op, `fora-aberto-email-${tag}`)
  }
  await out.close()
  await ctx.close()
}
await browser.close()
