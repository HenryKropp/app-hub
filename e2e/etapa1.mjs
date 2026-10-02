// Visual check for UX Etapa 1 (Rodar no cartão, faixa, Minhas execuções).
// Read-only: opens the folder dialog but never confirms; the strip states come
// from a faked /api/runs/active, so nothing runs on anyone's computer.
// Usage: BASE=http://localhost:5173 OUT=<dir> node etapa1.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "/tmp"
const browser = await chromium.launch()

const at = (min) => new Date(Date.now() - min * 60000).toISOString()
const fakeRuns = (app) => [
  { id: 9001, app_id: app.id, app_name: app.name, app_folder_display: "E:\\Apps\\" + app.name, status: "running", error_message: "", created_at: at(1), finished_at: null, file_count: 0 },
  { id: 9002, app_id: app.id, app_name: app.name, app_folder_display: "E:\\Apps\\" + app.name, status: "done", error_message: "", created_at: at(9), finished_at: at(4), file_count: 4 },
  { id: 9003, app_id: app.id, app_name: app.name, app_folder_display: "E:\\Apps\\" + app.name, status: "error", error_message: "a pasta E:\\Saida não existe", created_at: at(20), finished_at: at(20), file_count: 0 },
]

for (const [theme, viewport, tag] of [
  ["light", { width: 1280, height: 720 }, "desk-light"],
  ["dark", { width: 1280, height: 720 }, "desk-dark"],
  ["light", { width: 375, height: 812 }, "mobile"],
]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  // The Hub opens light by default; dark shots pick dark explicitly.
  await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
  const page = await ctx.newPage()
  await page.request.post(`${BASE}/api/dev/login`, { data: {} })
  const apps = await (await page.request.get(`${BASE}/api/apps`)).json()
  const runnable = apps.find((a) => a.can_run) ?? apps[0]

  const shot = async (name, full = true) => {
    await page.waitForLoadState("networkidle", { timeout: 60000 }).catch(() => {})
    await page.getByText("Carregando").first().waitFor({ state: "detached", timeout: 60000 }).catch(() => {})
    await page.waitForTimeout(600)
    await page.screenshot({ path: `${OUT}/${tag}-${name}.png` })
    if (full) await page.screenshot({ path: `${OUT}/${tag}-${name}-full.png`, fullPage: true })
    console.log("saved", tag, name)
  }

  // Real data first: catalog, ficha, Minhas execuções.
  await page.goto(`${BASE}/`)
  await shot("catalogo")
  const rodar = page.getByRole("button", { name: /^Rodar / }).first()
  if (await rodar.count()) {
    await rodar.click()
    if (await page.getByRole("dialog").count()) {
      await shot("janela-pasta", false)
      await page.keyboard.press("Escape")
    }
  }
  await page.goto(`${BASE}/apps/${runnable.id}`)
  await shot("ficha")
  // Sidebar while scrolled to the bottom of a long page.
  await page.mouse.wheel(0, 20000)
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${OUT}/${tag}-ficha-rolada.png` })
  await page.goto(`${BASE}/execucoes`)
  await shot("execucoes")
  const all = page.getByRole("button", { name: "De todos" })
  if (await all.count()) {
    await all.click()
    await shot("execucoes-todos")
  }

  // Strip states, faked.
  await page.route("**/api/runs/active", (r) => r.fulfill({ json: fakeRuns(runnable) }))
  await page.goto(`${BASE}/`)
  await shot("faixa", false)
  // Ficha with a remembered folder, faked.
  await page.route(`**/api/apps/${runnable.id}`, async (r) => {
    const res = await r.fetch()
    r.fulfill({ response: res, json: { ...(await res.json()), remembered_folder: "C:\\Users\\Henry\\Documents\\Saidas" } })
  })
  await page.goto(`${BASE}/apps/${runnable.id}`)
  await shot("ficha-lembrada", false)
  await ctx.close()
}
await browser.close()
