import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { ApiError, api } from "@/lib/api"
import { slugFromName } from "@/lib/company-entry"
import { CompanySignupPage } from "./CompanySignupPage"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function setup() {
  const assign = vi.fn()
  vi.stubGlobal("location", { ...window.location, assign })
  render(<MemoryRouter><CompanySignupPage /></MemoryRouter>)
  return { assign }
}

async function fill(values: Partial<Record<"company" | "name" | "email" | "password", string>> = {}) {
  const v = { company: "Horizonte", name: "Ana", email: "ana@example.com", password: "senha-forte-123", ...values }
  if (v.company) await userEvent.type(screen.getByLabelText("Nome da empresa"), v.company)
  if (v.name) await userEvent.type(screen.getByLabelText("Seu nome"), v.name)
  if (v.email) await userEvent.type(screen.getByLabelText("E-mail de trabalho"), v.email)
  if (v.password) await userEvent.type(screen.getByLabelText("Senha"), v.password)
}

const submit = () => userEvent.click(screen.getByRole("button", { name: /Criar espaço da empresa/ }))

test.each([
  ["Alis Investimentos", "alis-investimentos"],
  ["  Ação & Cia.  ", "acao-cia"],
  ["2B Logística", "b-logistica"],
  ["Zé", ""],
  ["A".repeat(60), "a".repeat(40)],
])("slugFromName(%j) is %j", (name, slug) => {
  expect(slugFromName(name)).toBe(slug)
})

test("signup creates the company, signs in to it and opens Começar", async () => {
  const create = vi.spyOn(api, "createEntryCompany").mockResolvedValue({ url: "/w/horizonte/", name: "Horizonte", slug: "horizonte" })
  const login = vi.spyOn(api, "loginAt").mockResolvedValue({} as never)
  const { assign } = setup()
  expect(document.querySelectorAll("form")).toHaveLength(1)
  await fill()
  expect(screen.getByText("ziv/w/horizonte")).toBeInTheDocument()
  await submit()
  expect(create).toHaveBeenCalledWith({
    company_name: "Horizonte",
    slug: "horizonte",
    name: "Ana",
    email: "ana@example.com",
    password: "senha-forte-123",
  })
  expect(login).toHaveBeenCalledWith("/w/horizonte", { email: "ana@example.com", password: "senha-forte-123" })
  expect(assign).toHaveBeenCalledWith("/w/horizonte/comecar")
})

test("if signing in after signup fails, the company still exists and the page opens its sign-in", async () => {
  vi.spyOn(api, "createEntryCompany").mockResolvedValue({ url: "/w/horizonte/", name: "Horizonte", slug: "horizonte" })
  vi.spyOn(api, "loginAt").mockRejectedValue(new ApiError(429, "Muitas tentativas."))
  const { assign } = setup()
  await fill()
  await submit()
  expect(assign).toHaveBeenCalledWith("/w/horizonte/entrar")
})

test("the address follows the company name until the person changes it", async () => {
  const create = vi.spyOn(api, "createEntryCompany").mockResolvedValue({ url: "/w/hzlog/", name: "Horizonte", slug: "hzlog" })
  vi.spyOn(api, "loginAt").mockResolvedValue({} as never)
  setup()
  await userEvent.type(screen.getByLabelText("Nome da empresa"), "Horizonte")
  await userEvent.click(screen.getByRole("button", { name: "trocar" }))
  const slug = screen.getByLabelText("Endereço da empresa")
  await userEvent.clear(slug)
  await userEvent.type(slug, "hzlog")
  await userEvent.type(screen.getByLabelText("Nome da empresa"), " Logística")
  expect(screen.getByLabelText("Endereço da empresa")).toHaveValue("hzlog")
  await fill({ company: "" })
  await submit()
  expect(create).toHaveBeenCalledWith(expect.objectContaining({ company_name: "Horizonte Logística", slug: "hzlog" }))
})

test("the password can be shown and hidden", async () => {
  setup()
  const password = screen.getByLabelText("Senha")
  expect(password).toHaveAttribute("type", "password")
  await userEvent.click(screen.getByRole("button", { name: "Mostrar senha" }))
  expect(password).toHaveAttribute("type", "text")
  await userEvent.click(screen.getByRole("button", { name: "Esconder senha" }))
  expect(password).toHaveAttribute("type", "password")
})

// The browser's own checks show a bubble that vanishes in seconds and stop the
// page from running; people saw "nothing happened". The page checks everything.
test("signup leaves validation to the page instead of the browser", () => {
  setup()
  expect(document.querySelector("form")!.noValidate).toBe(true)
})

test.each([
  ["a missing field", { name: "" }, "Preencha todos os campos.", "Seu nome"],
  ["an e-mail without @", { email: "ana.example.com" }, "Informe um e-mail válido.", "E-mail de trabalho"],
  ["a short password", { password: "abc123" }, "A senha precisa ter pelo menos 8 caracteres.", "Senha"],
])("signup explains %s on the page, marks the field and sends nothing", async (_, change, message, field) => {
  const create = vi.spyOn(api, "createEntryCompany")
  setup()
  await fill(change)
  await submit()
  expect(await screen.findByRole("alert")).toHaveTextContent(message)
  expect(screen.getByLabelText(field)).toHaveAttribute("aria-invalid", "true")
  expect(screen.getByLabelText(field)).toHaveFocus()
  expect(create).not.toHaveBeenCalled()
})

test("a name with no usable letters asks for an address", async () => {
  const create = vi.spyOn(api, "createEntryCompany")
  setup()
  await fill({ company: "Zé" })
  await submit()
  expect(await screen.findByRole("alert")).toHaveTextContent("O endereço deve ter")
  expect(screen.getByLabelText("Endereço da empresa")).toHaveFocus()
  expect(create).not.toHaveBeenCalled()
})

test("an address with capitals or spaces is explained", async () => {
  const create = vi.spyOn(api, "createEntryCompany")
  setup()
  await fill()
  await userEvent.click(screen.getByRole("button", { name: "trocar" }))
  const slug = screen.getByLabelText("Endereço da empresa")
  await userEvent.clear(slug)
  await userEvent.type(slug, "Horizonte Log")
  await submit()
  expect(await screen.findByRole("alert")).toHaveTextContent("O endereço deve ter")
  expect(create).not.toHaveBeenCalled()
})

test("an address already in use is explained and opens the address field", async () => {
  vi.spyOn(api, "createEntryCompany").mockRejectedValue(new ApiError(409, "Conflict"))
  setup()
  await fill()
  await submit()
  expect(await screen.findByRole("alert")).toHaveTextContent("já está em uso")
  expect(screen.getByLabelText("Endereço da empresa")).toHaveFocus()
})
