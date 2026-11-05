import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { ApiError, api } from "@/lib/api"
import * as landing from "@/lib/landing"
import { CompanyEntryPage } from "./CompanyEntryPage"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const person = { id: 1 } as never

function setup() {
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  render(<MemoryRouter><CompanyEntryPage /></MemoryRouter>)
  return { assign }
}

async function fillAndSubmit() {
  await userEvent.type(screen.getByLabelText("E-mail"), "ana@example.com")
  await userEvent.type(screen.getByLabelText("Senha"), "senha-forte-123")
  await userEvent.click(screen.getByRole("button", { name: /Entrar/ }))
}

test("the form asks only for e-mail and password, with no company address or principal link", () => {
  setup()
  expect(document.querySelectorAll("form")).toHaveLength(1)
  expect(screen.getByRole("heading", { name: "Entre no espaço da sua equipe" })).toBeInTheDocument()
  expect(screen.queryByLabelText(/Endereço da empresa/)).not.toBeInTheDocument()
  expect(document.body).not.toHaveTextContent("espaço principal")
  expect(screen.getByRole("link", { name: /Cadastre-se/ })).toHaveAttribute("href", "/cadastre-se")
})

test("one company signs in directly and goes where landingAfterLogin says", async () => {
  vi.spyOn(api, "findLogins").mockResolvedValue({ companies: [{ slug: "a", name: "A", url: "/w/a/" }] })
  const login = vi.spyOn(api, "loginAt").mockResolvedValue(person)
  vi.spyOn(landing, "landingAfterLogin").mockResolvedValue("/w/a/comecar")
  const { assign } = setup()
  await fillAndSubmit()
  await vi.waitFor(() => expect(assign).toHaveBeenCalledWith("/w/a/comecar"))
  expect(login).toHaveBeenCalledWith("/w/a", { email: "ana@example.com", password: "senha-forte-123" })
})

test("several companies ask which one, then sign in to the chosen one", async () => {
  vi.spyOn(api, "findLogins").mockResolvedValue({ companies: [{ slug: "a", name: "Alfa", url: "/w/a/" }, { slug: "b", name: "Beta", url: "/w/b/" }] })
  const login = vi.spyOn(api, "loginAt").mockResolvedValue(person)
  vi.spyOn(landing, "landingAfterLogin").mockResolvedValue("/w/b/")
  const { assign } = setup()
  await fillAndSubmit()
  expect(await screen.findByText("Em qual empresa?")).toBeInTheDocument()
  expect(screen.getByRole("button", { name: /Alfa/ })).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: /Beta/ }))
  await vi.waitFor(() => expect(assign).toHaveBeenCalledWith("/w/b/"))
  expect(login).toHaveBeenCalledWith("/w/b", { email: "ana@example.com", password: "senha-forte-123" })
})

test("wrong credentials show the server message and do not navigate", async () => {
  vi.spyOn(api, "findLogins").mockRejectedValue(new ApiError(401, "E-mail ou senha não conferem."))
  const { assign } = setup()
  await fillAndSubmit()
  expect(await screen.findByRole("alert")).toHaveTextContent("E-mail ou senha não conferem.")
  expect(assign).not.toHaveBeenCalled()
})
