// Visual check for Etapa 1 (ADR 043): "não testada" in the history and the
// "vai ao ar sem teste" confirmation. Creates a throwaway script app in the
// local dev database. Usage: BASE=http://localhost:5199 OUT=<dir> node rascunho.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "/tmp"
const b64 = (s) => Buffer.from(s).toString("base64")
const browser = await chromium.launch()
const ctx = await browser.newContext({ viewport: { width: 1366, height: 800 }, colorScheme: "light" })
await ctx.addInitScript(() => localStorage.setItem("apphub-theme", "light"))
const page = await ctx.newPage()
await page.request.post(`${BASE}/api/dev/login`, { data: {} })
const manifest = { name: "Soma rascunho", summary: "Soma dois números", kind: "script", entrypoint: "main.py", area: "Research",
  inputs: [{ id: "amount", label: "Valor", type: "number", source: "argument", required: true }] }
const imp = await page.request.post(`${BASE}/api/scripts/import`, { data: { manifest, files: { "main.py": { content_base64: b64("print('v1')") } } } })
const { id } = await imp.json()
console.log("app", id, imp.status())
const sent = await page.request.post(`${BASE}/api/apps/${id}/sources`, { data: { files: { "main.py": { content_base64: b64("print('v2')") } }, note: "Corrige a soma" } })
console.log("sources", sent.status())
await page.goto(`${BASE}/apps/${id}`)
await page.getByRole("tab", { name: "Histórico" }).click()
await page.getByText("não testada").first().waitFor()
await page.screenshot({ path: `${OUT}/historico.png` })
await browser.close()
