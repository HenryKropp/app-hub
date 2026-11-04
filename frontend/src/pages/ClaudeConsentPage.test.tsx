import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { ClaudeConsentPage, isClaudeHost } from "./ClaudeConsentPage"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const stubAssign = () => {
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  return assign
}

const mount = (entry = "/conectar-claude/autorizar?pedido=abc") =>
  render(<MemoryRouter initialEntries={[entry]}><ClaudeConsentPage /></MemoryRouter>)

const EXPIRED = "Esse pedido de conexão venceu. Volte ao Claude e clique em Conectar de novo."
const WARNING = "Atenção: este endereço não é do Claude. Só permita se você mesmo começou esta conexão."

test("asks to allow the client, says what it may do and where the access goes", async () => {
  const get = vi.spyOn(api, "oauthRequest").mockResolvedValue({ client_name: "Claude", redirect_host: "claude.ai" })
  mount()
  const question = await screen.findByRole("heading", { level: 1 })
  expect(question).toHaveTextContent("Permitir que Claude use o Ziv como você?")
  expect(question.querySelector("strong")).toHaveTextContent("Claude")
  expect(get).toHaveBeenCalledWith("abc")
  expect(screen.getByText("Ele poderá achar apps e artifacts, rodar apps no seu computador e publicar artifacts, sempre com as suas permissões.")).toBeInTheDocument()
  expect(screen.getByText("claude.ai", { selector: "strong" })).toBeInTheDocument()
  expect(screen.queryByText(WARNING)).not.toBeInTheDocument()
})

test("warns when the access would go somewhere that is not Claude", async () => {
  vi.spyOn(api, "oauthRequest").mockResolvedValue({ client_name: "Claude", redirect_host: "claude.ai.evil.com" })
  mount()
  expect(await screen.findByText(WARNING)).toBeInTheDocument()
  expect(screen.getByText("claude.ai.evil.com", { selector: "strong" })).toBeInTheDocument()
})

test("knows which hosts are Claude's", () => {
  for (const h of ["claude.ai", "claude.com", "www.claude.ai", "app.claude.com", "localhost", "127.0.0.1", "localhost:3000", "CLAUDE.AI"]) expect(isClaudeHost(h)).toBe(true)
  for (const h of ["claude.ai.evil.com", "evilclaude.ai", "notclaude.com", "example.com", ""]) expect(isClaudeHost(h)).toBe(false)
})

test("Permitir approves, keeps the buttons off while waiting, then goes to the redirect", async () => {
  const user = userEvent.setup()
  const assign = stubAssign()
  vi.spyOn(api, "oauthRequest").mockResolvedValue({ client_name: "Claude", redirect_host: "claude.ai" })
  let finish: (v: { redirect: string }) => void = () => {}
  const approve = vi.spyOn(api, "approveOauth").mockReturnValue(new Promise((r) => { finish = r }))
  mount()
  await user.click(await screen.findByRole("button", { name: "Permitir" }))
  expect(approve).toHaveBeenCalledWith("abc")
  expect(screen.getByRole("button", { name: /Permitir|Permitindo/ })).toBeDisabled()
  expect(screen.getByRole("button", { name: "Não permitir" })).toBeDisabled()
  await user.click(screen.getByRole("button", { name: /Permitir|Permitindo/ }))
  expect(approve).toHaveBeenCalledTimes(1)
  finish({ redirect: "https://claude.ai/api/mcp/auth_callback?code=x&state=y" })
  await waitFor(() => expect(assign).toHaveBeenCalledWith("https://claude.ai/api/mcp/auth_callback?code=x&state=y"))
})

test("Não permitir denies and goes back to Claude", async () => {
  const user = userEvent.setup()
  const assign = stubAssign()
  vi.spyOn(api, "oauthRequest").mockResolvedValue({ client_name: "Claude", redirect_host: "claude.ai" })
  const deny = vi.spyOn(api, "denyOauth").mockResolvedValue({ redirect: "https://claude.ai/cb?error=access_denied" })
  const approve = vi.spyOn(api, "approveOauth")
  mount()
  await user.click(await screen.findByRole("button", { name: "Não permitir" }))
  expect(deny).toHaveBeenCalledWith("abc")
  expect(approve).not.toHaveBeenCalled()
  await waitFor(() => expect(assign).toHaveBeenCalledWith("https://claude.ai/cb?error=access_denied"))
})

test("a failure on approve says what to do next and lets the person try again", async () => {
  const user = userEvent.setup()
  stubAssign()
  vi.spyOn(api, "oauthRequest").mockResolvedValue({ client_name: "Claude", redirect_host: "claude.ai" })
  vi.spyOn(api, "approveOauth").mockRejectedValue(new ApiError(0, "Não consegui falar com o Ziv."))
  mount()
  await user.click(await screen.findByRole("button", { name: "Permitir" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Tente de novo; se continuar, volte ao Claude e clique em Conectar de novo.")
  expect(screen.getByRole("button", { name: "Permitir" })).toBeEnabled()
})

test("a 404 says the request expired", async () => {
  vi.spyOn(api, "oauthRequest").mockRejectedValue(new ApiError(404, "Este pedido de conexão expirou."))
  mount()
  expect(await screen.findByText(EXPIRED)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Permitir" })).not.toBeInTheDocument()
})

test("no pedido in the address says the same as an expired request", async () => {
  const get = vi.spyOn(api, "oauthRequest")
  mount("/conectar-claude/autorizar")
  expect(await screen.findByText(EXPIRED)).toBeInTheDocument()
  expect(get).not.toHaveBeenCalled()
})

test("another load error shows it with a way to try again", async () => {
  const user = userEvent.setup()
  const get = vi.spyOn(api, "oauthRequest").mockRejectedValueOnce(new ApiError(0, "Não consegui falar com o Ziv."))
    .mockResolvedValueOnce({ client_name: "Claude", redirect_host: "claude.ai" })
  mount()
  expect(await screen.findByText("Não consegui falar com o Ziv.")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Tentar de novo" }))
  expect(await screen.findByRole("button", { name: "Permitir" })).toBeInTheDocument()
  expect(get).toHaveBeenCalledTimes(2)
})
