import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type Computer } from "@/lib/api"
import { computer } from "@/test/fixtures"
import { ComputerSection } from "./ComputerSection"

afterEach(() => vi.restoreAllMocks())
beforeEach(() => {
  vi.spyOn(api, "workspace").mockResolvedValue({ slug: "principal", name: "Empresa principal", base_path: "", can_manage: true, local_execution_is_sandboxed: false })
})
const renderPage = () => render(<MemoryRouter><ComputerSection /></MemoryRouter>)

test("no computer: says so and links to the Começar step, with no download", async () => {
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "0.1.0", agent_available: true })
  renderPage()
  expect(await screen.findByText("Nenhum computador conectado")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Conectar no Começar" })).toHaveAttribute("href", "/comecar#computador")
  expect(screen.queryByRole("link", { name: /Baixar/ })).not.toBeInTheDocument()
  expect(screen.queryByText(/Abra o arquivo baixado/)).not.toBeInTheDocument()
})

test("connected computer, outdated agent, disconnect", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [computer({ outdated: true, agent_version: "0.0.9" })], expected_version: "0.1.0", agent_available: true })
  const disconnect = vi.spyOn(api, "disconnect").mockResolvedValue(undefined)
  vi.spyOn(window, "confirm").mockReturnValue(true)
  renderPage()
  expect(await screen.findByText("NOTE-HENRY")).toBeInTheDocument()
  expect(screen.getByText("Agente ligado")).toBeInTheDocument()
  expect(screen.getByText(/Atualize o Agente/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Baixar no Começar" })).toHaveAttribute("href", "/comecar#computador")
  await user.click(screen.getByRole("button", { name: "Desconectar" }))
  expect(disconnect).toHaveBeenCalledWith(1)
})

test("update states", async () => {
  const info = (over: Partial<Computer>) => ({ computers: [computer(over)], expected_version: "0.2.0", agent_available: true })
  const spy = vi.spyOn(api, "computers").mockResolvedValue(info({ update_state: "updating", update_version: "0.2.0" }))
  const { unmount } = renderPage()
  expect(await screen.findByText("Atualizando o Agente para a versão 0.2.0…")).toBeInTheDocument()
  unmount()

  spy.mockResolvedValue(info({ update_state: "updated", update_version: "0.2.0", updated_at: "2026-09-30T14:00:00Z", agent_version: "0.2.0" }))
  renderPage()
  expect(await screen.findByText(/Agente atualizado sozinho em/)).toBeInTheDocument()
  cleanup()

  spy.mockResolvedValue(info({ update_state: "gave_up", outdated: true, agent_version: "0.1.9" }))
  renderPage()
  expect(await screen.findByText(/Não conseguimos atualizar o Agente sozinho. Baixe a versão 0.2.0 de novo./)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Baixar no Começar" })).toHaveAttribute("href", "/comecar#computador")
  expect(screen.queryByText(/Atualize o Agente/)).not.toBeInTheDocument()
})
