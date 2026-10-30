import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { ApiError, api } from "@/lib/api"
import { CompanySignupPage } from "./CompanySignupPage"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

test("signup creates the company then navigates to its own sign-in without auto-login", async () => {
  const create = vi.spyOn(api, "createEntryCompany").mockResolvedValue({ url: "/w/horizonte", name: "Horizonte", slug: "horizonte" })
  const login = vi.spyOn(api, "login")
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  render(<MemoryRouter><CompanySignupPage /></MemoryRouter>)
  expect(document.querySelectorAll("form")).toHaveLength(1)
  await userEvent.type(screen.getByLabelText("Nome da empresa"), "Horizonte")
  await userEvent.type(screen.getByLabelText("Endereço da empresa"), "horizonte")
  await userEvent.type(screen.getByLabelText("Seu nome"), "Ana")
  await userEvent.type(screen.getByLabelText("E-mail de trabalho"), "ana@example.com")
  await userEvent.type(screen.getByLabelText("Senha"), "senha-forte-123")
  await userEvent.click(screen.getByRole("button", { name: /Criar espaço da empresa/ }))
  expect(create).toHaveBeenCalledWith({
    company_name: "Horizonte",
    slug: "horizonte",
    name: "Ana",
    email: "ana@example.com",
    password: "senha-forte-123",
  })
  expect(assign).toHaveBeenCalledWith("/w/horizonte/entrar")
  expect(login).not.toHaveBeenCalled()
})

test("signup keeps invalid addresses local and displays API conflict errors", async () => {
  const create = vi.spyOn(api, "createEntryCompany").mockRejectedValue(new ApiError(409, "Conflict"))
  render(<MemoryRouter><CompanySignupPage /></MemoryRouter>)
  await userEvent.type(screen.getByLabelText("Nome da empresa"), "Horizonte")
  await userEvent.type(screen.getByLabelText("Endereço da empresa"), "ab")
  await userEvent.type(screen.getByLabelText("Seu nome"), "Ana")
  await userEvent.type(screen.getByLabelText("E-mail de trabalho"), "ana@example.com")
  await userEvent.type(screen.getByLabelText("Senha"), "senha-forte-123")
  await userEvent.click(screen.getByRole("button", { name: /Criar espaço da empresa/ }))
  expect(await screen.findByRole("alert")).toHaveTextContent("O endereço deve ter")
  expect(create).not.toHaveBeenCalled()

  await userEvent.clear(screen.getByLabelText("Endereço da empresa"))
  await userEvent.type(screen.getByLabelText("Endereço da empresa"), "horizonte")
  await userEvent.click(screen.getByRole("button", { name: /Criar espaço da empresa/ }))
  expect(await screen.findByRole("alert")).toHaveTextContent("já está em uso")
})
