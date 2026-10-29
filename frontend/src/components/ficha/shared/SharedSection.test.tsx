import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api, type AppDetail, type Can, type SharedState } from "@/lib/api"
import { detail as makeDetail, manifest } from "@/test/fixtures"
import { MakeShared, SharedSection } from "./SharedSection"

const running: SharedState = { status: "running", status_message: "", stopped_by: "", always_on: false, idle_minutes: 30,
  memory_mb: 512, cpus: 1, wants_db: false, url: "http://notas.localhost:8080", acks: [], has_previous: false }
const detail: AppDetail = makeDetail({ manifest: manifest({ kind: "web", name: "Notas", shared: { enabled: true } }), shared: { status: "running", url: "http://notas.localhost:8080", slug: "notas" } })
const can: Can = { view: true, edit: false, restore: false, tech: false, runs: false, access: false, grants: false }

const mocks = () => {
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: running, live_version: 2, latest_version: 2, deployments: [] })
  vi.spyOn(api.shared, "check").mockResolvedValue({ ready: true, recipe: { kind: "streamlit", command: "", dockerfile: "", port: 0, needs_confirm: false, requirements: null }, wants_db: false, suggest_env: [], items: [] })
  vi.spyOn(api.shared, "config").mockResolvedValue([])
  vi.spyOn(api.shared, "backups").mockResolvedValue([])
}

beforeEach(() => {
  vi.restoreAllMocks()
  mocks()
})

it("shows the whole part to who takes care, only the line to others", async () => {
  const { rerender } = render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  expect(await screen.findByRole("button", { name: "Parar" })).toBeInTheDocument()
  rerender(<SharedSection app={detail} can={{ ...can, restore: false }} onChanged={() => {}} />)
  expect(screen.queryByRole("button", { name: "Parar" })).not.toBeInTheDocument()
  expect(screen.getByText("No ar · versão 2")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /Abrir/ })).toHaveAttribute("href", "/api/shared/authorize?app=notas&volta=%2F")
})

it("does not run the check for who only opens the app", async () => {
  render(<SharedSection app={detail} can={can} onChanged={() => {}} />)
  expect(await screen.findByText("No ar · versão 2")).toBeInTheDocument()
  expect(api.shared.check).not.toHaveBeenCalled()
})

it("asks for confirmation before unsharing", async () => {
  const un = vi.spyOn(api.shared, "unshare").mockResolvedValue({})
  const onChanged = vi.fn()
  render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={onChanged} />)
  await userEvent.click(await screen.findByRole("button", { name: "Deixar de compartilhar" }))
  expect(screen.getByText(/os dados ficam guardados por 30 dias/i)).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Sim, deixar de compartilhar" }))
  expect(un).toHaveBeenCalledWith(detail.id)
  await vi.waitFor(() => expect(onChanged).toHaveBeenCalled())
})

it("asks before stopping", async () => {
  const stop = vi.spyOn(api.shared, "stop").mockResolvedValue({})
  render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  await userEvent.click(await screen.findByRole("button", { name: "Parar" }))
  expect(screen.getByText("Ninguém consegue abrir até alguém ligar de novo.")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Sim, parar" }))
  expect(stop).toHaveBeenCalledWith(detail.id)
})

it("offers Religar when the app fell", async () => {
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: { ...running, status: "crashed" }, live_version: 2, latest_version: 2, deployments: [] })
  const start = vi.spyOn(api.shared, "start").mockResolvedValue({})
  render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  expect(await screen.findByText("Caiu e não voltou sozinho")).toBeInTheDocument()
  await userEvent.click(screen.getAllByRole("button", { name: "Religar" })[0])
  expect(start).toHaveBeenCalledWith(detail.id)
})

it("marking 'estou ciente' saves the marks and checks again", async () => {
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: { ...running, status: "draft" }, deployments: [] })
  const check = vi.spyOn(api.shared, "check").mockResolvedValue({ ready: false, recipe: { kind: "flask", command: "", dockerfile: "", port: 0, needs_confirm: false, requirements: null }, wants_db: false, suggest_env: [], items: [
    { key: "file_data", status: "warn", title: "Dados em arquivo", detail: "Grava notas.json.", claude_text: "Use APPHUB_DATA_DIR.", acked: false },
  ] })
  const save = vi.spyOn(api.shared, "save").mockResolvedValue(detail)
  render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  await userEvent.click(await screen.findByRole("checkbox", { name: /estou ciente/i }))
  expect(save).toHaveBeenCalledWith(detail.id, expect.objectContaining({ enabled: true, acks: ["file_data"], updated_at: detail.updated_at }))
  await vi.waitFor(() => expect(check).toHaveBeenCalledTimes(2))
})

it("a setting added after 'estou ciente' keeps the mark", async () => {
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: { ...running, status: "draft" }, deployments: [] })
  vi.spyOn(api.shared, "check").mockResolvedValue({ ready: false, recipe: { kind: "flask", command: "", dockerfile: "", port: 0, needs_confirm: false, requirements: null }, wants_db: false, suggest_env: [], items: [
    { key: "file_data", status: "warn", title: "Dados em arquivo", detail: "Grava notas.json.", claude_text: "Use APPHUB_DATA_DIR.", acked: false },
  ] })
  const save = vi.spyOn(api.shared, "save").mockResolvedValue(detail)
  render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  await userEvent.click(await screen.findByRole("checkbox", { name: /estou ciente/i }))
  await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(1))
  await userEvent.click(await screen.findByRole("button", { name: "Adicionar configuração" }))
  await userEvent.type(screen.getByLabelText("Nome no código"), "API_KEY")
  await userEvent.click(screen.getByRole("button", { name: "Adicionar" }))
  await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(2))
  expect(save.mock.calls[1][1]).toMatchObject({ acks: ["file_data"], env: [{ name: "API_KEY" }] })
})

it("says so when the shared part does not load, and tries again", async () => {
  const get = vi.spyOn(api.shared, "get").mockRejectedValueOnce(new Error("O Kova não respondeu. Espere um pouco e tente de novo."))
  render(<SharedSection app={detail} can={{ ...can, restore: true }} onChanged={() => {}} />)
  expect(await screen.findByText("O Kova não respondeu. Espere um pouco e tente de novo.")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Tentar de novo" }))
  expect(await screen.findByRole("button", { name: "Parar" })).toBeInTheDocument()
  expect(get).toHaveBeenCalledTimes(2)
})

it("Tornar compartilhado keeps the database the manifest already asks for", async () => {
  const save = vi.spyOn(api.shared, "save").mockResolvedValue(detail)
  const app = makeDetail({ manifest: manifest({ kind: "web", name: "Notas", shared: { enabled: false, database: "postgres" } }) })
  render(<MakeShared app={app} onChanged={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Tornar compartilhado" }))
  expect(save).toHaveBeenCalledWith(app.id, expect.objectContaining({ enabled: true, database: "postgres", cpus: 1 }))
})
