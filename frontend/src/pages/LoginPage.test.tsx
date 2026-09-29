import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import * as auth from "@/lib/auth"
import { henry } from "@/test/fixtures"
import { LoginPage } from "./LoginPage"

afterEach(() => vi.restoreAllMocks())

function mockAuth(needsSetup: boolean) {
  const setPerson = vi.fn()
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup, person: null, refresh: async () => {}, setPerson })
  return setPerson
}

const renderPage = () =>
  render(
    <MemoryRouter>
      <LoginPage />
    </MemoryRouter>,
  )

test("first access creates the administrator", async () => {
  const user = userEvent.setup()
  const setPerson = mockAuth(true)
  const setup = vi.spyOn(api, "setup").mockResolvedValue(henry)
  renderPage()
  expect(screen.getByText("Crie a conta de administrador")).toBeInTheDocument()
  await user.type(screen.getByLabelText("Seu nome"), "Henry")
  await user.type(screen.getByLabelText("E-mail"), "henry@alis.com")
  await user.type(screen.getByLabelText("Senha"), "senha-forte-1")
  await user.click(screen.getByRole("button", { name: "Criar conta e entrar" }))
  expect(setup).toHaveBeenCalledWith({ name: "Henry", email: "henry@alis.com", password: "senha-forte-1" })
  expect(setPerson).toHaveBeenCalledWith(henry)
})

test("login shows the error from the server", async () => {
  const user = userEvent.setup()
  mockAuth(false)
  vi.spyOn(api, "login").mockRejectedValue(new ApiError(401, "E-mail ou senha não conferem."))
  renderPage()
  expect(screen.queryByLabelText("Seu nome")).not.toBeInTheDocument()
  await user.type(screen.getByLabelText("E-mail"), "henry@alis.com")
  await user.type(screen.getByLabelText("Senha"), "errada")
  await user.click(screen.getByRole("button", { name: "Entrar" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("E-mail ou senha não conferem.")
})
