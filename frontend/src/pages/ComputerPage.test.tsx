import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { computer } from "@/test/fixtures"
import { ComputerPage } from "./ComputerPage"

afterEach(() => vi.restoreAllMocks())
const renderPage = () => render(<MemoryRouter><ComputerPage /></MemoryRouter>)

test("no computer: shows how to install", async () => {
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "0.1.0", agent_available: true })
  renderPage()
  expect(await screen.findByText("Nenhum computador conectado")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /Instalar o Agente/ })).toHaveAttribute("href", "/api/agent/download")
})

test("connected computer, outdated agent, disconnect", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [computer({ outdated: true, agent_version: "0.0.9" })], expected_version: "0.1.0", agent_available: true })
  const disconnect = vi.spyOn(api, "disconnect").mockResolvedValue(undefined)
  vi.spyOn(window, "confirm").mockReturnValue(true)
  renderPage()
  expect(await screen.findByText("NOTE-HENRY")).toBeInTheDocument()
  expect(screen.getByText("Conectado")).toBeInTheDocument()
  expect(screen.getByText(/Atualize o Agente/)).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Desconectar" }))
  expect(disconnect).toHaveBeenCalledWith(1)
})

test("agent not built on this Hub", async () => {
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "0.1.0", agent_available: false })
  renderPage()
  expect(await screen.findByText(/O Agente ainda não foi preparado neste Hub/)).toBeInTheDocument()
})
