import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type McpConnection } from "@/lib/api"
import { ClaudeConnections } from "./ClaudeConnections"

const conn = (over: Partial<McpConnection> = {}): McpConnection => ({
  id: 1, kind: "oauth", name: "Claude", created_at: "2026-11-01T10:00:00Z", last_used_at: "2026-11-03T14:30:00Z", ...over,
})

beforeEach(() => {
  vi.spyOn(api, "mcpConnections").mockResolvedValue([])
})
afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

const mount = (entry = "/configuracoes") => render(<MemoryRouter initialEntries={[entry]}><ClaudeConnections /></MemoryRouter>)

test("the list shows claude.ai and código rows; Desconectar asks and removes the row", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "mcpConnections").mockResolvedValue([
    conn(),
    conn({ id: 2, kind: "code", name: "Notebook", last_used_at: null }),
  ])
  const revoke = vi.spyOn(api, "revokeMcpConnection").mockResolvedValue(undefined)
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(true)
  mount()
  const list = await screen.findByRole("list", { name: "Conexões do Claude" })
  const rows = within(list).getAllByRole("listitem")
  expect(rows).toHaveLength(2)
  expect(within(rows[0]).getByText("claude.ai")).toBeInTheDocument()
  expect(within(rows[0]).getByText(/Último uso: 03\/11/)).toBeInTheDocument()
  expect(within(rows[1]).getByText("código")).toBeInTheDocument()
  expect(within(rows[1]).getByText("Notebook")).toBeInTheDocument()
  expect(within(rows[1]).getByText(/Nunca usado/)).toBeInTheDocument()

  await user.click(within(rows[1]).getByRole("button", { name: "Desconectar" }))
  expect(confirm).toHaveBeenCalled()
  expect(revoke).toHaveBeenCalledWith(2)
  await waitFor(() => expect(within(list).getAllByRole("listitem")).toHaveLength(1))
  expect(within(list).queryByText("Notebook")).not.toBeInTheDocument()
})

test("Desconectar does nothing when the person says no, and shows an error on failure", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "mcpConnections").mockResolvedValue([conn()])
  const revoke = vi.spyOn(api, "revokeMcpConnection").mockRejectedValue(new Error("Conexão não encontrada."))
  const confirm = vi.spyOn(window, "confirm").mockReturnValueOnce(false).mockReturnValueOnce(true)
  mount()
  const list = await screen.findByRole("list", { name: "Conexões do Claude" })
  await user.click(within(list).getByRole("button", { name: "Desconectar" }))
  expect(revoke).not.toHaveBeenCalled()
  await user.click(within(list).getByRole("button", { name: "Desconectar" }))
  expect(confirm).toHaveBeenCalledTimes(2)
  expect(await screen.findByText(/Não consegui desconectar: Conexão não encontrada./)).toBeInTheDocument()
  expect(within(list).getAllByRole("listitem")).toHaveLength(1)
})

test("no Claude connection: says so and links to the Começar step", async () => {
  mount()
  expect(await screen.findByText("Nenhuma conexão do Claude")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Conectar no Começar" })).toHaveAttribute("href", "/comecar#claude")
})

test("settings does not offer the connector address or Gerar código", async () => {
  vi.spyOn(api, "mcpConnections").mockResolvedValue([conn()])
  mount()
  await screen.findByRole("list", { name: "Conexões do Claude" })
  expect(screen.queryByRole("button", { name: /Gerar código|Copiar/ })).not.toBeInTheDocument()
})
