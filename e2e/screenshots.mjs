// Visual check: takes screenshots of the key pages. Usage:
//   BASE=http://localhost:5173 SCAN=<pasta real, só leitura> DEMO=<pasta de demonstração> OUT=<dir> node screenshots.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "/tmp"
const SCAN = process.env.SCAN
const DEMO = process.env.DEMO

const browser = await chromium.launch()

async function shot(page, name) {
  await page.waitForTimeout(700)
  await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: true })
  console.log("saved", name)
}

async function context(theme = "light", viewport = { width: 1440, height: 900 }) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  return ctx
}

// 1. Login / first access, before anyone signs in.
{
  const ctx = await context()
  const page = await ctx.newPage()
  await page.goto(`${BASE}/entrar`)
  await shot(page, "01-entrar")
  await ctx.close()
}

for (const theme of ["light", "dark"]) {
  const ctx = await context(theme)
  const res = await ctx.request.post(`${BASE}/api/dev/login`, { data: { email: "henry.demo@alis.local" } })
  if (!res.ok()) throw new Error("dev login failed")
  const page = await ctx.newPage()

  if (theme === "light") {
    await page.goto(`${BASE}/`)
    await shot(page, "02-catalogo-vazio")
  }

  if (DEMO && theme === "light") {
    // Publish the demo apps through the API so the catalog has content.
    const me = await (await ctx.request.get(`${BASE}/api/auth/status`)).json()
    const scan = await (await ctx.request.post(`${BASE}/api/scan`, { data: { path: DEMO, mode: "batch" } })).json()
    for (const it of scan.items) {
      const m = { ...it.manifest, area: it.manifest.kind === "reference" ? "Administrativo" : "Investimentos" }
      await ctx.request.post(`${BASE}/api/apps`, { data: { folder_path: it.folder_path, owner_id: me.person.id, backup_id: null, manifest: m } })
    }
  }

  await page.goto(`${BASE}/`)
  await shot(page, `03-catalogo-${theme}`)

  if (SCAN) {
    await page.goto(`${BASE}/registrar`)
    await page.getByLabel("Onde estão os apps?").fill(SCAN)
    await page.getByRole("button", { name: /Analisar/ }).click()
    await page.getByText(/apps encontrados/).waitFor({ timeout: 60000 })
    await shot(page, `04-registrar-${theme}`)
    if (theme === "light") {
      for (const name of ["Rentabilidade fundos", "Research Platform", "News Scrapper"]) {
        await page.getByRole("button", { name: new RegExp(name) }).first().click()
        await shot(page, `05-revisar-${name.replace(/\s+/g, "-").toLowerCase()}`)
      }
    }
  }

  if (theme === "light") {
    const apps = await (await ctx.request.get(`${BASE}/api/apps`)).json()
    if (apps.length) {
      await page.goto(`${BASE}/apps/${apps[0].id}`)
      await shot(page, "06-ficha")
    }
    await page.goto(`${BASE}/pessoas`)
    await shot(page, "07-pessoas")
  }
  await ctx.close()
}

// Phone width.
{
  const ctx = await context("light", { width: 390, height: 844 })
  await ctx.request.post(`${BASE}/api/dev/login`, { data: { email: "henry.demo@alis.local" } })
  const page = await ctx.newPage()
  await page.goto(`${BASE}/`)
  await shot(page, "08-catalogo-celular")
  await ctx.close()
}

await browser.close()
