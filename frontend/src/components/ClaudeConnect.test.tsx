import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { ClaudeConnect, isLocalHost } from "./ClaudeConnect"

afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

const mount = (entry = "/comecar") => render(<MemoryRouter initialEntries={[entry]}><ClaudeConnect /></MemoryRouter>)

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
