// The whole approval flow (ADR 046) in empresa Teste, through the screens and
// the API, with screenshots at 1280×720 and 375×812.
// Run against an isolated DEV_MODE Ziv (never the one on 8080), e.g.:
//   ZIV_API=http://localhost:8094 npx vite --port 5194   (frontend)
//   BASE=http://localhost:5194 OUT=<dir> node governanca.mjs
// It creates Ana (Criador, Research), Paulo (leader of Research) and Bia
// (Investimentos) if missing, and puts the company's approval settings back
// as they were at the end.
import { chromium } from "playwright"
import { mkdirSync } from "node:fs"

const BASE = process.env.BASE ?? "http://localhost:5194"
const W = `${BASE}/w/teste`
const OUT = process.env.OUT ?? "shots/governanca"
mkdirSync(OUT, { recursive: true })
const hdr = { Origin: BASE }
const b64 = (s) => Buffer.from(s).toString("base64")
const stamp = Date.now().toString(36)

const fail = (msg) => { throw new Error(msg) }
const expect = (ok, msg) => { if (!ok) fail(msg); console.log("ok:", msg) }

const browser = await chromium.launch()
const as = async (email, viewport = { width: 1280, height: 720 }) => {
  const ctx = await browser.newContext({ viewport, colorScheme: "light" })
  await ctx.addInitScript(() => { try { localStorage.setItem("apphub-theme", "light") } catch { /* ignore */ } })
  const r = await ctx.request.post(`${W}/api/dev/login`, { headers: hdr, data: { email } })
  if (!r.ok()) fail(`login ${email}: ${r.status()}`)
  return ctx
}
const call = async (ctx, method, path, data) => {
  const r = await ctx.request.fetch(`${W}${path}`, { method, headers: hdr, data })
  const text = await r.text()
  let body
  try { body = text ? JSON.parse(text) : undefined } catch { body = text }
  return { status: r.status(), body }
}
const ok = async (ctx, method, path, data) => {
  const r = await call(ctx, method, path, data)
  if (r.status >= 400) fail(`${method} ${path} → ${r.status} ${JSON.stringify(r.body)}`)
  return r.body
}

// ---- people and areas ----
const admin = await as("admin@exemplo.com")
const roles = await ok(admin, "GET", "/api/roles")
const role = (key) => roles.find((r) => r.key === key)?.id ?? fail(`role ${key}`)
const person = async (name, email, roleKey) => {
  const list = await ok(admin, "GET", "/api/people")
  const found = list.find((p) => p.email === email)
  if (found) {
    if (found.role_id !== role(roleKey)) await ok(admin, "PUT", `/api/people/${found.id}`, { role_id: role(roleKey) })
    return found.id
  }
  return (await ok(admin, "POST", "/api/people", { name, email, password: "senha-de-teste-123", role_id: role(roleKey) })).id
}
const ana = await person("Ana", "ana.gov@teste.local", "criador")
const paulo = await person("Paulo", "paulo.gov@teste.local", "usuario")
const bia = await person("Bia", "bia.gov@teste.local", "usuario")
const areas = await ok(admin, "GET", "/api/areas")
const area = (name) => areas.find((a) => a.name === name) ?? fail(`area ${name}`)
const research = area("Research")
const invest = area("Investimentos")
await ok(admin, "PUT", `/api/areas/${research.id}/leaders`, { person_ids: [...new Set([...research.leaders.map((p) => p.id), paulo])] })
await ok(admin, "PUT", `/api/areas/${research.id}/members`, { person_ids: [...new Set([...research.members.map((p) => p.id), ana])] })
await ok(admin, "PUT", `/api/areas/${invest.id}/members`, { person_ids: [...new Set([...invest.members.map((p) => p.id), bia].filter((id) => id !== ana))] })

const before = await ok(admin, "GET", "/api/governance/settings")
const settings = (o) => ok(admin, "PUT", "/api/governance/settings", { ...before, ...o })

try {
  // 1. A light company: only the leader approves, versions only when the risk changes.
  await settings({ it_approval: false, version_rule: "risk", code_channel: "any" })

  // 2. Ana registers an app: it waits for the leader of Research.
  const anaCtx = await as("ana.gov@teste.local")
  const manifest = { name: `Baixar Relatórios ${stamp}`, summary: "Baixa os relatórios do dia", kind: "script", entrypoint: "main.py", area: "Research", inputs: [] }
  const app = await ok(anaCtx, "POST", "/api/scripts/import", { manifest, files: { "main.py": { content_base64: b64("print('v1')") } } })
  const id = app.id
  let gov = await ok(anaCtx, "GET", `/api/apps/${id}/governance`)
  expect(gov.open_request?.waiting === "Aguardando o líder de Research.", "Ana vê 'Aguardando o líder de Research.'")
  const anaPage = await anaCtx.newPage()
  await anaPage.goto(`${W}/apps/${id}`)
  await anaPage.getByText("Aguardando o líder de Research.").first().waitFor()
  await anaPage.screenshot({ path: `${OUT}/app-ana-esperando-1280.png`, fullPage: true })

  // 3. Paulo sees it on Início and in Aprovações, chooses "Só Research" and approves.
  for (const [tag, viewport] of [["375", { width: 375, height: 812 }], ["1280", { width: 1280, height: 720 }]]) {
    const ctx = await as("paulo.gov@teste.local", viewport)
    const page = await ctx.newPage()
    await page.goto(`${W}/`)
    await page.getByRole("heading", { name: "Esperando sua aprovação" }).waitFor()
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${OUT}/inicio-paulo-${tag}.png`, fullPage: true })
    await page.goto(`${W}/aprovacoes`)
    await page.getByText(manifest.name).first().waitFor()
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${OUT}/aprovacoes-${tag}.png`, fullPage: true })
    if (tag === "1280") {
      const card = page.locator("li", { hasText: manifest.name })
      await card.getByRole("button", { name: "Aprovar" }).click()
      await page.getByRole("radio", { name: "Só Research" }).check()
      await page.waitForTimeout(300)
      await page.screenshot({ path: `${OUT}/aprovar-publico-${tag}.png` })
      await page.getByRole("button", { name: "Aprovar e liberar" }).click()
      await page.getByText(`${manifest.name} foi aprovado.`).waitFor()
    }
    await ctx.close()
  }
  gov = await ok(anaCtx, "GET", `/api/apps/${id}/governance`)
  expect(!gov.open_request, "depois do líder, nada mais espera")
  const detail = await ok(anaCtx, "GET", `/api/apps/${id}`)
  expect(detail.control?.approved === true, "o app está aprovado e Ana pode rodar")

  // 4. Bia (Investimentos) does not see an app only for Research.
  const biaCtx = await as("bia.gov@teste.local")
  const biaApps = await ok(biaCtx, "GET", "/api/apps")
  expect(!biaApps.some((a) => a.id === id), "Bia não vê o app (só Research)")
  const biaDetail = await call(biaCtx, "GET", `/api/apps/${id}`)
  expect(biaDetail.status === 404 || biaDetail.status === 403, "Bia não abre a página do app")

  // 5. TI also approves, every version.
  await settings({ it_approval: true, version_rule: "every", code_channel: "any" })
  await ok(anaCtx, "POST", `/api/apps/${id}/sources`, { files: { "main.py": { content_base64: b64("print('v2')") } }, note: "Versão 2" })
  gov = await ok(anaCtx, "GET", `/api/apps/${id}/governance`)
  expect(gov.open_request?.reason === "version", "a versão nova espera aprovação")
  expect(gov.open_request.steps.map((s) => s.step).sort().join(",") === "it,leader", "espera o líder e a TI")
  const run = await call(anaCtx, "POST", `/api/apps/${id}/runs`, { inputs: {} })
  expect(!/Aguardando|aprova/i.test(JSON.stringify(run.body ?? "")), `a versão aprovada continua rodando (a execução não para por aprovação: ${run.status})`)

  const pauloCtx = await as("paulo.gov@teste.local")
  const pauloList = await ok(pauloCtx, "GET", "/api/approvals")
  const req = pauloList.find((r) => r.item_id === id && r.status === "open") ?? fail("Paulo não vê o pedido da versão")
  await ok(pauloCtx, "POST", `/api/approvals/${req.id}/approve`, {})
  gov = await ok(anaCtx, "GET", `/api/apps/${id}/governance`)
  expect(gov.open_request?.waiting === "Aguardando a TI.", "depois do líder, espera a TI")
  await ok(admin, "POST", `/api/approvals/${req.id}/approve`, {})
  gov = await ok(anaCtx, "GET", `/api/apps/${id}/governance`)
  expect(!gov.open_request, "com líder e TI, a versão nova está no ar")

  // 6. Only through Claude: the site refuses code.
  await settings({ it_approval: true, version_rule: "every", code_channel: "mcp" })
  const refused = await call(anaCtx, "POST", `/api/apps/${id}/sources`, { files: { "main.py": { content_base64: b64("print('v3')") } }, note: "Pelo site" })
  expect(refused.status === 403 && /Claude/.test(refused.body?.error ?? ""), `o site recusa código: ${refused.body?.error}`)

  // 7. The history says who approved what.
  const hist = await ok(admin, "GET", `/api/governance/history?item=app:${id}`)
  const deciders = hist.filter((e) => e.type === "request").flatMap((e) => e.request.steps.filter((s) => s.decision === "approved").map((s) => s.decided_by))
  expect(deciders.includes("Paulo") && deciders.length >= 3, `histórico mostra quem aprovou: ${deciders.join(", ")}`)

  // 8. Screens: the app (Configurações, Histórico) and Admin → Governança.
  for (const [tag, viewport] of [["1280", { width: 1280, height: 720 }], ["375", { width: 375, height: 812 }]]) {
    const ctx = await as("admin@exemplo.com", viewport)
    const page = await ctx.newPage()
    await page.goto(`${W}/apps/${id}`)
    await page.getByRole("tab", { name: "Configurações" }).click()
    await page.getByRole("heading", { name: "Aprovação de versões" }).waitFor()
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${OUT}/app-configuracoes-${tag}.png`, fullPage: true })
    await page.getByRole("tab", { name: "Histórico" }).click()
    await page.getByRole("heading", { name: "Aprovações" }).waitFor()
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${OUT}/app-historico-${tag}.png`, fullPage: true })
    await page.goto(`${W}/admin`)
    await page.getByRole("heading", { name: "Governança", exact: true }).waitFor()
    await page.getByRole("heading", { name: "Governança", exact: true }).scrollIntoViewIfNeeded()
    await page.waitForTimeout(600)
    await page.screenshot({ path: `${OUT}/admin-governanca-${tag}.png` })
    const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
    expect(!over, `admin ${tag} sem rolagem lateral`)
    await ctx.close()
  }
  console.log("FLUXO OK")
} finally {
  await settings({})
  await browser.close()
}
