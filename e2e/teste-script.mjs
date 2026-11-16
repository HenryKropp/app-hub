// Visual check for Etapa 2 (ADR 044): the test plan page before and after
// confirming, the "em teste" band on the app page and the notices card on
// the Home. Creates a throwaway script app in the local dev database.
// Usage: BASE=http://localhost:5199 OUT=<dir> node teste-script.mjs
import { chromium } from "playwright"

const BASE = process.env.BASE ?? "http://localhost:5199"
const OUT = process.env.OUT ?? "/tmp"
const b64 = (s) => Buffer.from(s).toString("base64")
const browser = await chromium.launch()
const ctx = await browser.newContext({ viewport: { width: 1366, height: 800 }, colorScheme: "light" })
await ctx.addInitScript(() => localStorage.setItem("apphub-theme", "light"))
const page = await ctx.newPage()
await page.request.post(`${BASE}/api/dev/login`, { data: {} })

const manifest = {
  name: "Relatórios do BTG", summary: "Baixa os relatórios do dia e monta um PDF.", kind: "script", entrypoint: "main.py", area: "Research",
  keys: [{ name: "BTG_SENHA", label: "Senha do BTG", required: false }],
  teste: {
    acessa: ["BTG (login com a chave BTG_SENHA)", "Pasta de teste do Agente"],
    pula: "mover os relatórios para a pasta oficial do OneDrive",
    espera: [{ arquivos: "*.pdf", no_minimo: 1 }],
  },
}
const imp = await page.request.post(`${BASE}/api/scripts/import`, {
  data: { manifest, files: { "main.py": { content_base64: b64("print('v0')") } }, in_test: true },
})
const { id } = await imp.json()
const created = await page.request.post(`${BASE}/api/apps/${id}/drafts`, {
  data: { files: { "main.py": { content_base64: b64("print('v1')") } }, note: "Primeira versão" },
})
const { draft_id } = await created.json()
const started = await page.request.post(`${BASE}/api/apps/${id}/drafts/${draft_id}/test`)
console.log("app", id, "draft", draft_id, imp.status(), created.status(), started.status())

await page.goto(`${BASE}/apps/${id}`)
await page.getByText("Versão nova em teste").waitFor()
await page.screenshot({ path: `${OUT}/app-faixa.png` })

await page.goto(`${BASE}/apps/${id}/plano-de-teste`)
await page.getByRole("button", { name: "Confirmar plano de teste" }).waitFor()
await page.screenshot({ path: `${OUT}/plano-antes.png`, fullPage: true })
const text = await page.getByText("Os próximos testes deste app serão automáticos.").textContent()
console.log("confirm text:", text)
await page.getByRole("button", { name: "Confirmar plano de teste" }).click()
await page.getByText(/Plano confirmado por/).waitFor()
await page.screenshot({ path: `${OUT}/plano-depois.png`, fullPage: true })

await page.goto(`${BASE}/`)
await page.getByRole("button", { name: "Marcar como lido" }).first().waitFor()
await page.screenshot({ path: `${OUT}/home-avisos.png` })

await ctx.close()
const mobile = await browser.newContext({ viewport: { width: 390, height: 844 }, colorScheme: "light" })
await mobile.addInitScript(() => localStorage.setItem("apphub-theme", "light"))
const m = await mobile.newPage()
await m.request.post(`${BASE}/api/dev/login`, { data: {} })
await m.goto(`${BASE}/apps/${id}/plano-de-teste`)
await m.getByText(/Plano confirmado por/).waitFor()
await m.screenshot({ path: `${OUT}/plano-celular.png`, fullPage: true })
await browser.close()
