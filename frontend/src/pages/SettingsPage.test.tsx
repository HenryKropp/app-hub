import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { ThemeProvider } from "@/lib/theme"
import { SettingsPage } from "./SettingsPage"

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.classList.remove("dark")
  localStorage.clear()
})

test("settings lists connections with status, links to Começar for what is missing, and keeps Aparência", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "0.1.0", agent_available: true })
  vi.spyOn(api, "mcpConnections").mockResolvedValue([])
  render(<ThemeProvider><MemoryRouter><SettingsPage /></MemoryRouter></ThemeProvider>)
  expect(screen.getByRole("heading", { name: "Configurações" })).toBeInTheDocument()
  expect(await screen.findByText("Nenhum computador conectado")).toBeInTheDocument()
  expect(await screen.findByText("Nenhuma conexão do Claude")).toBeInTheDocument()
  const links = screen.getAllByRole("link", { name: "Conectar no Começar" })
  expect(links.map((l) => l.getAttribute("href"))).toEqual(["/comecar#computador", "/comecar#claude"])
  expect(screen.queryByRole("link", { name: /Baixar/ })).not.toBeInTheDocument()
  expect(screen.queryByRole("button", { name: /Gerar código/ })).not.toBeInTheDocument()
  expect(screen.queryByText(/Endereço do conector/)).not.toBeInTheDocument()
  expect(screen.getByRole("heading", { name: "Aparência" })).toBeInTheDocument()
  await user.click(screen.getByRole("radio", { name: /Escuro/ }))
  expect(screen.getByRole("radio", { name: /Escuro/ })).toHaveAttribute("aria-checked", "true")
  expect(document.documentElement).toHaveClass("dark")
})
