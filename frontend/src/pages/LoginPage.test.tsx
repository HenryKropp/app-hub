import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import * as auth from "@/lib/auth"
import { henry } from "@/test/fixtures"
import { LoginPage } from "./LoginPage"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function mockAuth(needsSetup: boolean, person: typeof henry | null = null) {
  const setPerson = vi.fn()
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup, person, refresh: async () => {}, setPerson })
  return setPerson
}

const renderPage = (path = "/entrar") =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <LoginPage />
    </MemoryRouter>,
  )

function stubAssign() {
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  return assign
}

async function signIn() {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText("E-mail"), "henry@alis.com")
  await user.type(screen.getByLabelText("Senha"), "senha-forte-1")
  await user.click(screen.getByRole("button", { name: "Entrar" }))
}

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

test("goes back to the shared app after signing in", async () => {
  const assign = stubAssign()
  mockAuth(false)
  const login = vi.spyOn(api, "login").mockResolvedValue(henry)
  renderPage("/entrar?volta=%2Fapi%2Fshared%2Fauthorize%3Fapp%3Dnotas")
  await signIn()
  await waitFor(() => expect(assign).toHaveBeenCalledWith("/api/shared/authorize?app=notas"))
  expect(login).toHaveBeenCalled()
})

test("ignores a volta that is not a shared app", async () => {
  for (const volta of ["https%3A%2F%2Fevil.com", "%2Fapi%2Fshared%2Fauthorizer", "%2F%2Fevil.com%2Fapi%2Fshared%2Fauthorize"]) {
    const assign = stubAssign()
    const setPerson = mockAuth(false)
    vi.spyOn(api, "login").mockResolvedValue(henry)
    const { unmount } = renderPage("/entrar?volta=" + volta)
    await signIn()
    await waitFor(() => expect(setPerson).toHaveBeenCalledWith(henry))
    expect(assign).not.toHaveBeenCalled()
    unmount()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  }
})

test("already signed in goes straight to the shared app", async () => {
  const assign = stubAssign()
  mockAuth(false, henry)
  renderPage("/entrar?volta=%2Fapi%2Fshared%2Fauthorize%3Fapp%3Dnotas%26teste%3D1")
  await waitFor(() => expect(assign).toHaveBeenCalledWith("/api/shared/authorize?app=notas&teste=1"))
})
