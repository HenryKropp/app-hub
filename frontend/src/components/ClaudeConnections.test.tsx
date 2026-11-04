import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type McpConnection } from "@/lib/api"
import { ClaudeConnections, isLocalHost } from "./ClaudeConnections"

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

test("shows the connector address with Copiar and a Copiado feedback", async () => {
  const user = userEvent.setup()
  mount()
  const address = `${window.location.origin}/mcp`
  expect(await screen.findByText(address)).toBeInTheDocument()
  const block = screen.getByRole("region", { name: "No site ou no app do Claude" })
  await user.click(within(block).getByRole("button", { name: "Copiar endereço" }))
  expect(await navigator.clipboard.readText()).toBe(address)
  expect(within(block).getByText("Copiado")).toBeInTheDocument()
  expect(screen.getByText("No Claude, abra Configurações → Conectores → Adicionar conector personalizado.")).toBeInTheDocument()
  expect(screen.getByText("Cole o endereço acima e salve.")).toBeInTheDocument()
  expect(screen.getByText("Clique em Conectar e entre com a sua conta do Ziv.")).toBeInTheDocument()
  expect(screen.getByText(/Se a sua empresa usa o Claude em plano de equipe/)).toBeInTheDocument()
})

test("the section has id claude so /configuracoes#claude reaches it, and scrolls there", async () => {
  const scroll = vi.fn()
  Element.prototype.scrollIntoView = scroll
  const { container } = mount("/configuracoes#claude")
  expect(container.querySelector("section#claude")).not.toBeNull()
  await waitFor(() => expect(scroll).toHaveBeenCalled())
})

test("on localhost warns that the site way only works once the Ziv is on the internet", async () => {
  mount()
  expect(await screen.findByText("Esta forma só funciona depois que o Ziv estiver publicado na internet.")).toBeInTheDocument()
  expect(isLocalHost("localhost")).toBe(true)
  expect(isLocalHost("127.0.0.1")).toBe(true)
  expect(isLocalHost("ziv.app")).toBe(false)
})

test("Gerar código shows the command once, with Copiar, and clears it on the next code", async () => {
  const user = userEvent.setup()
  const create = vi.spyOn(api, "createMcpCode")
    .mockResolvedValueOnce({ id: 7, name: "Notebook", code: "ziv_um", command: "claude mcp add --transport http ziv http://x/mcp --header \"Authorization: Bearer ziv_um\"" })
    .mockResolvedValueOnce({ id: 8, name: "Casa", code: "ziv_dois", command: "claude mcp add --transport http ziv http://x/mcp --header \"Authorization: Bearer ziv_dois\"" })
  mount()
  const block = await screen.findByRole("region", { name: "No Claude Code" })
  const generate = within(block).getByRole("button", { name: "Gerar código" })
  expect(generate).toBeDisabled()
  await user.type(within(block).getByLabelText("Nome deste código"), "Notebook")
  await user.click(generate)
  expect(create).toHaveBeenCalledWith("Notebook")
  expect(await within(block).findByText(/Bearer ziv_um/)).toBeInTheDocument()
  expect(within(block).getByText("Guarde agora: o código não aparece de novo.")).toBeInTheDocument()
  await user.click(within(block).getByRole("button", { name: "Copiar comando" }))
  expect(await navigator.clipboard.readText()).toContain("ziv_um")
  expect(within(block).getByText("Copiado")).toBeInTheDocument()
  expect(JSON.stringify({ ...localStorage })).not.toContain("ziv_um")

  await user.type(within(block).getByLabelText("Nome deste código"), "Casa")
  await user.click(within(block).getByRole("button", { name: "Gerar código" }))
  expect(await within(block).findByText(/Bearer ziv_dois/)).toBeInTheDocument()
  expect(within(block).queryByText(/Bearer ziv_um/)).not.toBeInTheDocument()
})

test("a failed code shows the error", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "createMcpCode").mockRejectedValue(new Error("O nome pode ter até 80 letras."))
  mount()
  const block = await screen.findByRole("region", { name: "No Claude Code" })
  await user.type(within(block).getByLabelText("Nome deste código"), "x")
  await user.click(within(block).getByRole("button", { name: "Gerar código" }))
  expect(await within(block).findByRole("alert")).toHaveTextContent("O nome pode ter até 80 letras.")
})

test("the list shows claude.ai and código rows; Cancelar asks and removes the row", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "mcpConnections").mockResolvedValue([
    conn(),
    conn({ id: 2, kind: "code", name: "Notebook", last_used_at: null }),
  ])
  const revoke = vi.spyOn(api, "revokeMcpConnection").mockResolvedValue(undefined)
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(true)
  mount()
  const list = await screen.findByRole("list", { name: "Conexões ativas" })
  const rows = within(list).getAllByRole("listitem")
  expect(rows).toHaveLength(2)
  expect(within(rows[0]).getByText("claude.ai")).toBeInTheDocument()
  expect(within(rows[0]).getByText(/Último uso: 03\/11/)).toBeInTheDocument()
  expect(within(rows[1]).getByText("código")).toBeInTheDocument()
  expect(within(rows[1]).getByText("Notebook")).toBeInTheDocument()
  expect(within(rows[1]).getByText(/Nunca usado/)).toBeInTheDocument()

  await user.click(within(rows[1]).getByRole("button", { name: "Cancelar" }))
  expect(confirm).toHaveBeenCalled()
  expect(revoke).toHaveBeenCalledWith(2)
  await waitFor(() => expect(within(list).getAllByRole("listitem")).toHaveLength(1))
  expect(within(list).queryByText("Notebook")).not.toBeInTheDocument()
})

test("Cancelar does nothing when the person says no, and shows an error on failure", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "mcpConnections").mockResolvedValue([conn()])
  const revoke = vi.spyOn(api, "revokeMcpConnection").mockRejectedValue(new Error("Conexão não encontrada."))
  const confirm = vi.spyOn(window, "confirm").mockReturnValueOnce(false).mockReturnValueOnce(true)
  mount()
  const list = await screen.findByRole("list", { name: "Conexões ativas" })
  await user.click(within(list).getByRole("button", { name: "Cancelar" }))
  expect(revoke).not.toHaveBeenCalled()
  await user.click(within(list).getByRole("button", { name: "Cancelar" }))
  expect(confirm).toHaveBeenCalledTimes(2)
  expect(await screen.findByText(/Não consegui cancelar: Conexão não encontrada./)).toBeInTheDocument()
  expect(within(list).getAllByRole("listitem")).toHaveLength(1)
})

test("empty list says Nenhuma conexão ativa.", async () => {
  mount()
  expect(await screen.findByText("Nenhuma conexão ativa.")).toBeInTheDocument()
})
