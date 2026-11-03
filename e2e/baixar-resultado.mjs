// Visual check for the download button on the run strip, with mocked finished
// runs (one with several files, one with a single file). Uses the dev login.
// Usage: BASE=http://localhost:5173 node baixar-resultado.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = "shots/baixar"
const now = new Date().toISOString()
const run = (id, name, status, n) => ({ id, app_id: id, app_name: name, app_folder_display: "C:\\Apps", status, error_message: "",
  output_folder: "C:\\Users\\Henry\\Documentos\\Resultados", file_count: n, created_at: now, finished_at: now, can_stop: false })
const browser = await chromium.launch()
for (const [theme, viewport, tag] of [["light", { width: 1280, height: 800 }, "desk"], ["dark", { width: 1280, height: 800 }, "dark"], ["light", { width: 390, height: 844 }, "mobile"]]) {
  const ctx = await browser.newContext({ viewport, colorScheme: theme })
  await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
  const page = await ctx.newPage()
  await page.request.post(`${BASE}/api/dev/login`, { headers: { Origin: BASE }, data: { email: "henry.kropp@alisinvestimentos.com" } })
  await page.route("**/api/runs/active", (route) => route.fulfill({ json: [run(1, "Contador de palavras", "done", 3), run(2, "Olá Ziv", "done", 1)] }))
  await page.goto(`${BASE}/`)
  await page.getByText("Baixar tudo").waitFor()
  await page.waitForTimeout(600)
  await page.screenshot({ path: `${OUT}/${tag}.png` })
  const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
  console.log("saved", tag, over ? "OVERFLOW-X" : "ok")
  await ctx.close()
}
await browser.close()
