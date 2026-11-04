import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { henry } from "@/test/fixtures"
import App from "./App"

afterEach(() => {
  vi.restoreAllMocks()
  window.history.replaceState(null, "", "/")
})

test("after signing in, the Claude consent page comes back with its pedido", async () => {
  const user = userEvent.setup()
  window.history.replaceState(null, "", "/conectar-claude/autorizar?pedido=abc")
  vi.spyOn(api, "status").mockResolvedValue({ needs_setup: false, person: null })
  vi.spyOn(api, "workspace").mockResolvedValue({ slug: "principal", name: "Empresa principal", base_path: "", can_manage: true, local_execution_is_sandboxed: false })
  vi.spyOn(api, "login").mockResolvedValue(henry)
  const get = vi.spyOn(api, "oauthRequest").mockResolvedValue({ client_name: "Claude", redirect_host: "claude.ai" })
  render(<App />)

  await user.type(await screen.findByLabelText("E-mail"), "henry@alis.com")
  await user.type(screen.getByLabelText("Senha"), "senha-forte-1")
  await user.click(screen.getByRole("button", { name: "Entrar" }))

  expect(await screen.findByRole("button", { name: "Permitir" })).toBeInTheDocument()
  expect(get).toHaveBeenCalledWith("abc")
  await waitFor(() => expect(window.location.pathname + window.location.search).toBe("/conectar-claude/autorizar?pedido=abc"))
})
