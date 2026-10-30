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

test("access page has one company form and confirms the company before full navigation", async () => {
  const lookup = vi.spyOn(api, "entryCompanyBySlug").mockResolvedValue({ slug: "horizonte", name: "Horizonte", base_path: "/w/horizonte" })
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  render(<MemoryRouter><CompanyEntryPage /></MemoryRouter>)
  expect(document.querySelectorAll("form")).toHaveLength(1)
  await userEvent.type(screen.getByLabelText("Identificador ou endereço"), "horizonte")
  await userEvent.click(screen.getByRole("button", { name: "Continuar" }))
  expect(lookup).toHaveBeenCalledWith("horizonte")
  expect(assign).toHaveBeenCalledWith("/w/horizonte/entrar")
  expect(screen.getByRole("link", { name: /Cadastre-se/ })).toHaveAttribute("href", "/cadastre-se")
})

test("the principal company with an empty base path goes to the root sign-in", async () => {
  const lookup = vi.spyOn(api, "entryCompanyBySlug").mockResolvedValue({ slug: "principal", name: "Principal", base_path: "" })
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  render(<MemoryRouter><CompanyEntryPage /></MemoryRouter>)
  await userEvent.type(screen.getByLabelText("Identificador ou endereço"), "principal")
  await userEvent.click(screen.getByRole("button", { name: "Continuar" }))
  expect(lookup).toHaveBeenCalledWith("principal")
  expect(assign).toHaveBeenCalledWith("/entrar")
})
