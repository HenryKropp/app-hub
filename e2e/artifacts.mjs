// Visual check for Artifacts (list, file artifact page, care editor, new page) in light, dark and mobile.
// Seeds real artifacts through the dev Hub API and deletes them at the end.
// Usage: BASE=http://localhost:5173 OUT=/path/to/shots node artifacts.mjs
import { chromium } from "playwright"
import { mkdirSync } from "node:fs"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "shots/artifacts"
mkdirSync(OUT, { recursive: true })

const html = `<!doctype html><html><head><meta charset="utf-8"><title>Vendas</title>
<style>body{font-family:sans-serif;margin:16px}canvas{max-width:100%}</style></head><body>
<h2>Receita por trimestre</h2><canvas id="c" height="160"></canvas>
<script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/4.4.1/chart.umd.min.js"></script>
<script>
const d=[12,19,8,15], l=["1T","2T","3T","4T"], cv=document.getElementById("c");
if (window.Chart) { new Chart(cv,{type:"bar",data:{labels:l,datasets:[{label:"R$ mi",data:d,backgroundColor:"#7c3aed"}]},options:{animation:false}}) }
else { const x=cv.getContext("2d"); d.forEach((v,i)=>{x.fillStyle="#7c3aed";x.fillRect(20+i*70,150-v*6,50,v*6);x.fillStyle="#000";x.fillText(l[i],35+i*70,148)}) ; document.body.append("(sem CDN: barras de reserva)") }
</script></body></html>`

const browser = await chromium.launch()
const login = async (ctx) => {
  const r = await ctx.request.post(`${BASE}/api/dev/login`, { headers: { Origin: BASE }, data: { email: "henry.kropp@alisinvestimentos.com" } })
  if (!r.ok()) throw new Error("login " + r.status())
}
const seedCtx = await browser.newContext()
await login(seedCtx)
const api = seedCtx.request
const hdr = { Origin: BASE }
const areas = await (await api.get(`${BASE}/api/areas`)).json()
const area = areas[0].id
const ids = []
try {
  const mk = async (data) => {
    const r = await api.post(`${BASE}/api/artifacts`, { headers: hdr, data })
    if (!r.ok()) throw new Error("create " + r.status() + " " + (await r.text()))
    const j = await r.json(); ids.push(j.id); return j.id
  }
  await mk({ name: "Painel de ações (exemplo)", summary: "Link para o painel do time de investimentos.", tags: ["painel", "investimentos"], area_id: area, kind: "link", url: "https://example.com/painel" })
  const fileId = await mk({ name: "Receita por trimestre", summary: "Gráfico de barras da receita, gerado pelo Claude.", tags: ["gráfico", "receita"], area_id: area, kind: "file", file_name: "receita.html", content_base64: Buffer.from(html).toString("base64") })

  for (const [theme, viewport, tag] of [["light", { width: 1280, height: 800 }, "desk"], ["dark", { width: 1280, height: 800 }, "dark"], ["light", { width: 390, height: 844 }, "mobile"], ["dark", { width: 390, height: 844 }, "mobile-dark"]]) {
    const ctx = await browser.newContext({ viewport, colorScheme: theme })
    await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
    await login(ctx)
    const page = await ctx.newPage()
    const shot = async (name, path, wait) => {
      await page.goto(`${BASE}${path}`)
      await wait(page)
      await page.waitForTimeout(1500)
      await page.screenshot({ path: `${OUT}/${name}-${tag}.png`, fullPage: true })
      const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
      console.log("saved", name, tag, over ? "OVERFLOW-X" : "ok")
    }
    await shot("lista", "/artifacts", (p) => p.getByText("Receita por trimestre").first().waitFor())
    await shot("artifact", `/artifacts/${fileId}`, (p) => p.locator("iframe").waitFor())
    await shot("cuida", `/artifacts/${fileId}`, async (p) => {
      await p.locator("iframe").waitFor()
      await p.getByRole("button", { name: "Editar quem cuida" }).click()
      await p.getByLabel("Área").last().waitFor()
    })
    await shot("novo", "/artifacts/novo", (p) => p.waitForLoadState("networkidle"))
    await ctx.close()
  }
} finally {
  for (const id of ids) await api.delete(`${BASE}/api/artifacts/${id}`, { headers: hdr })
  console.log("deleted", ids.length)
  await browser.close()
}
