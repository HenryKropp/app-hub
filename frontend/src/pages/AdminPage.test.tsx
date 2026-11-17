import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError, type AdminOverview } from "@/lib/api"
import { AdminPage } from "./AdminPage"

vi.mock("@/components/ExternalLinks", () => ({ CompanyLinksPanel: () => <p>Painel de links externos</p> }))

afterEach(() => vi.restoreAllMocks())

const settings = { retention_days: 30, require_app_approval: true, block_embedded_secrets: true, local_execution_enabled: true }
const overview = (over: Partial<AdminOverview> = {}): AdminOverview => ({
  settings,
  apps: [
    { id: 1, name: "Relatório CVM", area: "Research", owner: "Ana", approved: true, suspended: false, runs_30d: 12, errors_30d: 2, last_run_at: "2026-11-02T10:00:00Z" },
    { id: 2, name: "Conciliação", area: "Operações", owner: "Bia", approved: false, suspended: false, runs_30d: 0, errors_30d: 0, last_run_at: null },
    { id: 3, name: "Robô antigo", area: "Operações", owner: "Bia", approved: true, suspended: true, runs_30d: 1, errors_30d: 0, last_run_at: null },
  ],
  top_people: [{ name: "Ana", runs_30d: 12 }],
  computers: [{ id: 1, name: "NOTE-ANA", person: "Ana", agent_version: "0.3.2", last_seen_at: "2026-11-03T10:00:00Z", connected: true, outdated: false }],
  activity: [{ kind: "app_registered", name: "Conciliação", actor: "Bia", at: "2026-11-03T09:00:00Z", app_id: 2 }],
  ...over,
})
const renderPage = () => render(<MemoryRouter><AdminPage /></MemoryRouter>)

test("the dashboard shows apps by area, what waits for approval, computers and activity", async () => {
  vi.spyOn(api, "adminOverview").mockResolvedValue(overview())
  renderPage()
  expect(await screen.findByRole("heading", { name: "Esperando sua aprovação" })).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: "Research" })).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: "Operações" })).toBeInTheDocument()
  const waiting = screen.getByRole("heading", { name: "Esperando sua aprovação" }).closest("section")!
  expect(within(waiting).getByRole("link", { name: /Conciliação/ })).toHaveAttribute("href", "/apps/2")
  expect(screen.getByText("Suspenso")).toBeInTheDocument()
  expect(screen.getByText("2 com erro")).toBeInTheDocument()
  expect(screen.getByText("NOTE-ANA")).toBeInTheDocument()
  expect(screen.getByText(/foi registrado/)).toBeInTheDocument()
})

test("pausing every app asks first and saves the company switch", async () => {
  vi.spyOn(api, "adminOverview").mockResolvedValue(overview())
  const save = vi.spyOn(api, "saveSecuritySettings").mockResolvedValue({ ...settings, local_execution_enabled: false })
  vi.spyOn(window, "confirm").mockReturnValue(true)
  renderPage()
  await userEvent.click(await screen.findByRole("button", { name: "Pausar todos" }))
  expect(save).toHaveBeenCalledWith({ ...settings, local_execution_enabled: false })
  expect(await screen.findByText("Todos os apps estão pausados.")).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Retomar apps" })).toBeInTheDocument()
})

test("someone who is not an administrator is told so", async () => {
  vi.spyOn(api, "adminOverview").mockRejectedValue(new ApiError(403, "Você não tem permissão para isso."))
  renderPage()
  expect(await screen.findByText("Só administradores veem esta página.")).toBeInTheDocument()
})

test("the principal company turns screen apps on for another company, after confirming", async () => {
  vi.spyOn(api, "adminOverview").mockResolvedValue(overview())
  vi.spyOn(api, "adminCompanies").mockResolvedValue([{ slug: "outra", name: "Outra", shared_apps: false }])
  const set = vi.spyOn(api, "setCompanyShared").mockResolvedValue({ slug: "outra", name: "Outra", shared_apps: true })
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(true)
  renderPage()
  expect(await screen.findByRole("heading", { name: "Empresas" })).toBeInTheDocument()
  const sw = screen.getByRole("switch", { name: "Apps com tela de Outra" })
  expect(sw).not.toBeChecked()
  await userEvent.click(sw)
  expect(confirm).toHaveBeenCalledWith("Ligar apps com tela para Outra? Pessoas dessa empresa poderão rodar programas no servidor do Ziv.")
  expect(set).toHaveBeenCalledWith("outra", true)
  expect(await screen.findByRole("switch", { name: "Apps com tela de Outra" })).toBeChecked()
})

test("no Empresas section outside the principal company", async () => {
  vi.spyOn(api, "adminOverview").mockResolvedValue(overview())
  vi.spyOn(api, "adminCompanies").mockRejectedValue(new ApiError(404, "Página não encontrada."))
  renderPage()
  expect(await screen.findByRole("heading", { name: "Apps por área" })).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Empresas" })).not.toBeInTheDocument()
})

test("Admin shows the company's external links panel", async () => {
  vi.spyOn(api, "adminOverview").mockResolvedValue(overview())
  render(<MemoryRouter><AdminPage /></MemoryRouter>)
  expect(await screen.findByText("Painel de links externos")).toBeInTheDocument()
})

test("Governança saves the four company choices", async () => {
  vi.spyOn(api, "adminOverview").mockResolvedValue(overview())
  vi.spyOn(api, "governanceSettings").mockResolvedValue({ it_approval: false, version_rule: "risk", code_channel: "any", mcp_restricted: false })
  vi.spyOn(api, "companyMCP").mockResolvedValue([])
  vi.spyOn(api, "governanceHistory").mockResolvedValue([])
  const save = vi.spyOn(api, "saveGovernanceSettings").mockImplementation(async (b) => b)
  renderPage()
  const section = (await screen.findByRole("heading", { name: "Governança" })).closest("section")!
  expect(within(section).getByText("O líder da área sempre aprova.")).toBeInTheDocument()
  expect(within(section).queryByText(/Aprovar apps novos antes de rodarem/)).not.toBeInTheDocument()
  await userEvent.click(within(section).getByRole("checkbox", { name: /A TI também aprova/ }))
  expect(save).toHaveBeenLastCalledWith({ it_approval: true, version_rule: "risk", code_channel: "any", mcp_restricted: false })
  await userEvent.click(within(section).getByRole("radio", { name: "Toda versão" }))
  expect(save).toHaveBeenLastCalledWith({ it_approval: true, version_rule: "every", code_channel: "any", mcp_restricted: false })
  await userEvent.click(within(section).getByRole("radio", { name: "Só pelo Claude" }))
  expect(save).toHaveBeenLastCalledWith({ it_approval: true, version_rule: "every", code_channel: "mcp", mcp_restricted: false })
  await userEvent.click(within(section).getByRole("checkbox", { name: /Conectar o Claude/ }))
  expect(save).toHaveBeenLastCalledWith({ it_approval: true, version_rule: "every", code_channel: "mcp", mcp_restricted: true })
})

test("the admin cancels an MCP key of the company", async () => {
  vi.spyOn(api, "adminOverview").mockResolvedValue(overview())
  vi.spyOn(api, "governanceSettings").mockResolvedValue({ it_approval: false, version_rule: "risk", code_channel: "any", mcp_restricted: false })
  vi.spyOn(api, "governanceHistory").mockResolvedValue([])
  vi.spyOn(api, "companyMCP").mockResolvedValue([{ id: 5, kind: "code", name: "NOTE-ANA", person_id: 2, person_name: "Ana", created_at: "2026-11-10T10:00:00Z", last_used_at: null, revoked: false }])
  vi.spyOn(window, "confirm").mockReturnValue(true)
  const revoke = vi.spyOn(api, "revokeCompanyMCP").mockResolvedValue(undefined)
  renderPage()
  await userEvent.click(await screen.findByRole("button", { name: "Cancelar a chave NOTE-ANA de Ana" }))
  expect(revoke).toHaveBeenCalledWith(5)
})
