import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { ApiError, api } from "@/lib/api"
import { slugFromEntry, validCompanySlug } from "@/lib/company-entry"
import { CompanyEntryPage } from "./CompanyEntryPage"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

test("company identifiers require a lowercase letter and a 3–40 character safe slug", () => {
  expect(validCompanySlug("ziv-equipe-8")).toBe(true)
  expect(validCompanySlug("ab")).toBe(false)
  expect(validCompanySlug("Ziv-equipe")).toBe(false)
  expect(validCompanySlug("ziv/equipe")).toBe(false)
  expect(validCompanySlug(`a${"b".repeat(40)}`)).toBe(false)
})

test("company entry accepts a slug or an exact same-origin workspace URL only", () => {
  const origin = "https://hub.example"
  expect(slugFromEntry("atelier-norte", origin)).toBe("atelier-norte")
  expect(slugFromEntry("https://hub.example/w/atelier-norte", origin)).toBe("atelier-norte")
  expect(slugFromEntry("/w/atelier-norte/", origin)).toBe("atelier-norte")
  expect(slugFromEntry("https://other.example/w/atelier-norte", origin)).toBeNull()
  expect(slugFromEntry("https://hub.example/w/atelier-norte/apps", origin)).toBeNull()
  expect(slugFromEntry("Atelier-Norte", origin)).toBeNull()
})

test("unknown companies show an error and never redirect to the principal workspace", async () => {
  const lookup = vi.spyOn(api, "entryCompanyBySlug").mockRejectedValue(new ApiError(404, "Not found"))
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  render(<MemoryRouter><CompanyEntryPage /></MemoryRouter>)
  await userEvent.type(screen.getByLabelText("Identificador ou endereço"), "inexistente")
  await userEvent.click(screen.getByRole("button", { name: "Continuar" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Não encontrei uma empresa")
  expect(lookup).toHaveBeenCalledWith("inexistente")
  expect(assign).not.toHaveBeenCalled()
})

test("company creation performs a full navigation to that company's explicit sign-in", async () => {
  const create = vi.spyOn(api, "createEntryCompany").mockResolvedValue({ url: "/w/horizonte", name: "Horizonte", slug: "horizonte" })
  const login = vi.spyOn(api, "login")
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  render(<MemoryRouter><CompanyEntryPage /></MemoryRouter>)
  await userEvent.type(screen.getByLabelText("Nome da empresa"), "Horizonte")
  await userEvent.type(screen.getByLabelText("Identificador"), "horizonte")
  await userEvent.type(screen.getByLabelText("Seu nome"), "Ana")
  await userEvent.type(screen.getByLabelText("E-mail de trabalho"), "ana@example.com")
  await userEvent.type(screen.getByLabelText("Senha"), "senha-forte-123")
  await userEvent.click(screen.getByRole("button", { name: "Criar empresa" }))
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
