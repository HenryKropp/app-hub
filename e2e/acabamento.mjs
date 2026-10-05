// Visual check for the Etapa 1 finishing (mensagens, saídas, acabamento).
// Read-only: starting a run is blocked, strip states and the catalog error are faked.
// Usage: BASE=http://localhost:5173 OUT=<dir> node acabamento.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "/tmp"
const browser = await chromium.launch()
const at = (min) => new Date(Date.now() - min * 60000).toISOString()

for (const [theme, viewport, tag] of [
  ["light", { width: 1280, height: 720 }, "desk-light"],
  ["dark", { width: 1280, height: 720 }, "desk-dark"],
  ["light", { width: 375, height: 812 }, "mobile"],
]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
  const page = await ctx.newPage()
  // Never start anything on a real computer.
  await page.route("**/api/apps/*/runs", (r) => (r.request().method() === "POST" ? r.abort() : r.continue()))
  await page.request.post(`${BASE}/api/dev/login`, { data: {} })
  const apps = await (await page.request.get(`${BASE}/api/apps`)).json()
  const app = apps.find((a) => a.can_run) ?? apps[0]
  const shot = async (name) => {
    await page.waitForTimeout(700)
    await page.screenshot({ path: `${OUT}/${tag}-${name}.png` })
    console.log("saved", tag, name)
  }

  // Run window with Cancelar: catalog with no remembered folders so Rodar asks.
  await page.route("**/api/apps", async (r) => {
    const res = await r.fetch()
    r.fulfill({ response: res, json: (await res.json()).map((a) => ({ ...a, remembered_folder: "" })) })
  })
  await page.goto(`${BASE}/`)
  await page.getByRole("button", { name: /^Rodar / }).first().click()
  await page.getByRole("dialog").waitFor()
  await shot("janela-cancelar")
  await page.getByRole("button", { name: "Cancelar" }).click()

  // Search with no result.
  await page.getByRole("searchbox", { name: "Buscar apps" }).fill("xyz")
  await shot("busca-vazia")

  // Strip: no files, an open run with Ver, an error, then one open run hidden.
  const r = (o) => ({ app_id: app.id, app_name: app.name, app_folder_display: "E:\\Apps", output_folder: "C:\\Users\\Henry\\Documents\\Saidas", error_message: "", created_at: at(3), finished_at: null, file_count: 0, ...o })
  await page.route("**/api/runs/active", (x) => x.fulfill({ json: [
    r({ id: 9001, status: "running" }),
    r({ id: 9002, status: "running", app_name: "Outro app" }),
    r({ id: 9003, status: "no_files", finished_at: at(1) }),
    r({ id: 9004, status: "error", error_message: "O Windows não deixou abrir o app. Confira se o arquivo do app ainda existe e se o antivírus não o bloqueou, e tente de novo." }),
  ] }))
  await page.goto(`${BASE}/`)
  await shot("faixa")
  // On a phone only the first card shows, so there may be no Ocultar to press.
  const hide = page.getByRole("button", { name: "Ocultar" }).first()
  if (await hide.count()) {
    await hide.click()
    await shot("faixa-oculto")
  }

  // Catalog that doesn't load.
  await page.unroute("**/api/apps")
  await page.route("**/api/apps", (x) => x.fulfill({ status: 502, body: "Bad Gateway" }))
  await page.goto(`${BASE}/`)
  await shot("erro-catalogo")
  await ctx.close()
}
await browser.close()
