// Visual check for the Home tab (Meu dia, Meu time, celular, estado vazio, catálogo em /apps) in light and dark.
// Seeds through the dev Hub API (favorites, artifacts, one opened artifact) and, for a failed run, through
// `podman exec ziv-db psql` (the dev company schema). Everything is removed at the end.
// Usage: BASE=http://localhost:5173 OUT=/path/to/shots node home.mjs
import { chromium } from "playwright"
import { mkdirSync } from "node:fs"
import { execFileSync } from "node:child_process"

const BASE = process.env.BASE ?? "http://localhost:5173"
const OUT = process.env.OUT ?? "shots/home"
const SCHEMA = process.env.SCHEMA ?? "public"
const EMPTY_EMAIL = "home-vazia-e2e@example.com"
mkdirSync(OUT, { recursive: true })

const sql = (q) => execFileSync("podman", ["exec", "ziv-db", "psql", "-U", "postgres", "-d", "postgres", "-At", "-c", `set search_path=${SCHEMA}; ${q}`]).toString()

const browser = await chromium.launch()
const login = async (ctx, email) => {
  const r = await ctx.request.post(`${BASE}/api/dev/login`, { headers: { Origin: BASE }, data: { email } })
  if (!r.ok()) throw new Error("login " + r.status())
  return r.json()
}
const hdr = { Origin: BASE }
const seedCtx = await browser.newContext()
const me = await login(seedCtx, "henry.kropp@alisinvestimentos.com")
const api = seedCtx.request
const artifactIds = []
const favorites = []
let runId = null
let emptyId = null
try {
  const areas = await (await api.get(`${BASE}/api/areas`)).json()
  const area = areas[0].id
  const mk = async (data) => {
    const r = await api.post(`${BASE}/api/artifacts`, { headers: hdr, data })
    if (!r.ok()) throw new Error("create " + r.status() + " " + (await r.text()))
    const j = await r.json(); artifactIds.push(j.id); return j.id
  }
  const painel = await mk({ name: "Painel de ações (exemplo)", summary: "Link para o painel do time de investimentos.", tags: ["painel"], area_id: area, kind: "link", url: "https://example.com/painel" })
  const recente = await mk({ name: "Receita por trimestre", summary: "Gráfico de barras da receita.", tags: ["receita"], area_id: area, kind: "file", file_name: "r.html", content_base64: Buffer.from("<h1>Receita</h1>").toString("base64") })
  const apps = await (await api.get(`${BASE}/api/apps`)).json()
  const app = apps[0]
  const star = async (kind, id) => {
    const r = await api.put(`${BASE}/api/favorites/${kind}/${id}`, { headers: hdr })
    if (!r.ok()) throw new Error("favorite " + r.status())
    favorites.push([kind, id])
  }
  await star("app", app.id)
  await star("artifact", painel)
  // opening the artifact detail records an open ("Usados recentemente")
  await api.get(`${BASE}/api/artifacts/${recente}`)
  // a failed run by me: "Atenção" + "Minhas últimas execuções"
  const errApp = apps.find((a) => /erro/i.test(a.name)) ?? app
  const comp = sql(`select id from computers where person_id = ${me.id} order by id limit 1`).split("\n").find((l) => /^\d+$/.test(l))
  if (comp) {
    const out = sql(`insert into runs (app_id, person_id, computer_id, output_folder, status, exit_code, error_message) values (${errApp.id}, ${me.id}, ${comp}, 'x', 'error', 1, 'falhou de propósito (e2e)') returning id`)
    runId = out.split("\n").find((l) => /^\d+$/.test(l))
  } else console.log("no computer in dev DB: skipping failed run")

  // a fresh person with nothing, not an admin
  const emptyCtx = await browser.newContext({ viewport: { width: 1280, height: 800 } })
  const emp = await login(emptyCtx, EMPTY_EMAIL)
  emptyId = emp.id
  sql(`update people set role_id = (select id from roles where key = 'usuario'), name = 'Bia Nova' where id = ${emptyId}`)

  const shoot = async (ctx, name, path, wait, tag) => {
    const page = await ctx.newPage()
    await page.goto(`${BASE}${path}`)
    await wait(page)
    await page.waitForTimeout(1200)
    await page.screenshot({ path: `${OUT}/${name}-${tag}.png`, fullPage: true })
    const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
    console.log("saved", name, tag, over ? "OVERFLOW-X" : "ok")
    return page
  }
  const homeReady = (p) => p.getByRole("heading", { name: "Favoritos", exact: true }).waitFor()

  for (const [theme, viewport, tag] of [["light", { width: 1280, height: 900 }, "desk"], ["dark", { width: 1280, height: 900 }, "dark"], ["light", { width: 390, height: 844 }, "celular"]]) {
    const ctx = await browser.newContext({ viewport, colorScheme: theme })
    await ctx.addInitScript((t) => localStorage.setItem("apphub-theme", t), theme)
    await login(ctx, "henry.kropp@alisinvestimentos.com")
    await shoot(ctx, "home-admin", "/", homeReady, tag)
    await shoot(ctx, "apps-catalogo", "/apps", (p) => p.getByText(app.name).first().waitFor(), tag)
    if (tag === "desk") {
      // the star on a card must not open the card
      const page = await ctx.newPage()
      await page.goto(`${BASE}/apps`)
      await page.getByText(app.name).first().waitFor()
      const btn = page.getByRole("button", { name: /favorit/i }).first()
      await btn.click()
      await page.waitForTimeout(600)
      const stayed = new URL(page.url()).pathname === "/apps"
      console.log("star click keeps catalog:", stayed ? "ok" : "FAIL " + page.url())
      await btn.click() // toggle back
      await page.waitForTimeout(400)
      await page.close()
    }
    await ctx.close()
  }
  const vctx = await browser.newContext({ viewport: { width: 1280, height: 900 } })
  await login(vctx, EMPTY_EMAIL)
  await shoot(vctx, "home-vazia", "/", homeReady, "desk")
  const vm = await browser.newContext({ viewport: { width: 390, height: 844 } })
  await login(vm, EMPTY_EMAIL)
  await shoot(vm, "home-vazia", "/", homeReady, "celular")
} finally {
  for (const [k, id] of favorites) await api.delete(`${BASE}/api/favorites/${k}/${id}`, { headers: hdr })
  for (const id of artifactIds) await api.delete(`${BASE}/api/artifacts/${id}`, { headers: hdr })
  try {
    if (runId) sql(`delete from runs where id = ${runId}`)
    if (emptyId) sql(`delete from people where id = ${emptyId}`)
  } catch (e) { console.log("cleanup sql failed", e.message) }
  console.log("cleaned", artifactIds.length, "artifacts,", favorites.length, "favorites")
  await browser.close()
}
