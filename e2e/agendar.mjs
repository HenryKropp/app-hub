// Visual check for Agendar (ADR 024): the app page with its Agendamentos, the Agendar
// dialog on "Em dias da semana", and a paused schedule; desktop and phone, light and dark.
// Seeds two schedules through the dev Hub API (far-off times, so the clock never fires
// them) and pauses one through `podman exec ziv-db psql`. Both are deleted at the end.
// Usage: BASE=http://localhost:5180 OUT=/path/to/shots APP=3 node agendar.mjs
import { chromium } from "playwright"
import { mkdirSync } from "node:fs"
import { execFileSync } from "node:child_process"

const BASE = process.env.BASE ?? "http://localhost:5180"
const OUT = process.env.OUT ?? "shots/agendar"
const APP = Number(process.env.APP ?? 3)
mkdirSync(OUT, { recursive: true })
const sql = (q) => execFileSync("podman", ["exec", "ziv-db", "psql", "-U", "postgres", "-d", "postgres", "-At", "-c", q]).toString()

const browser = await chromium.launch()
const ctx = await browser.newContext()
const hdr = { Origin: BASE }
const r0 = await ctx.request.post(`${BASE}/api/dev/login`, { headers: hdr, data: { email: "henry.kropp@alisinvestimentos.com" } })
if (!r0.ok()) throw new Error("login " + r0.status())
const api = ctx.request
const made = []
try {
  const comps = await (await api.get(`${BASE}/api/computers/me`)).json()
  const computer = comps.computers[0]?.id
  if (!computer) throw new Error("Henry has no computer")
  const detail = await (await api.get(`${BASE}/api/apps/${APP}`)).json()
  const inputs = Object.fromEntries((detail.manifest.inputs ?? []).filter((i) => i.required).map((i) => [i.id, i.type === "number" ? "10" : i.type === "choice" ? (i.options?.[0] ?? "") : "teste"]))
  const mk = async (name, frequency) => {
    const r = await api.post(`${BASE}/api/apps/${APP}/schedules`, { headers: hdr, data: { name, frequency, computer_id: computer, output_folder: "D:\\Saídas\\Juros", inputs, files: {} } })
    if (!r.ok()) throw new Error("schedule " + r.status() + " " + (await r.text()))
    const j = await r.json(); made.push(j.id); return j.id
  }
  await mk("Relatório do fim do mês", { kind: "monthly", at: "23:00", day: 28 })
  const paused = await mk("", { kind: "weekdays", at: "07:30", days: [1, 3, 5] })
  sql(`update public.schedules set active=false, paused_reason='Falhou 5 vezes seguidas' where id=${paused}`)

  for (const [label, viewport] of [["desktop", { width: 1366, height: 900 }], ["celular", { width: 390, height: 844 }]]) {
    for (const scheme of ["light", "dark"]) {
      const page = await ctx.newPage()
      await page.setViewportSize(viewport)
      await page.emulateMedia({ colorScheme: scheme })
      await page.goto(`${BASE}/apps/${APP}`)
      const section = page.getByRole("heading", { name: "Agendamentos" })
      await section.waitFor()
      await section.scrollIntoViewIfNeeded()
      await page.screenshot({ path: `${OUT}/${label}-${scheme}-lista.png`, fullPage: label === "celular" })
      if (scheme === "light") {
        await page.getByRole("button", { name: "Agendar" }).click()
        await page.getByLabel("Frequência").selectOption("weekdays")
        await page.getByText(/Próximas:/).waitFor()
        await page.screenshot({ path: `${OUT}/${label}-dialogo-semana.png` })
        await page.getByLabel("Frequência").selectOption("minutes")
        await page.getByLabel("A cada quantos minutos").fill("4")
        await page.screenshot({ path: `${OUT}/${label}-dialogo-erro.png` })
      }
      await page.close()
    }
  }
  console.log("ok", OUT)
} finally {
  for (const id of made) await api.delete(`${BASE}/api/schedules/${id}`, { headers: hdr })
  await browser.close()
}
