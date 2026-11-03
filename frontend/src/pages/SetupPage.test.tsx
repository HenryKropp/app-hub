import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type Onboarding } from "@/lib/api"
import { SetupPage } from "./SetupPage"

afterEach(() => vi.restoreAllMocks())
beforeEach(() => {
  vi.spyOn(api, "workspace").mockResolvedValue({ slug: "principal", name: "Empresa principal", base_path: "", can_manage: true, local_execution_is_sandboxed: false })
})

const state = (over: Partial<Onboarding> = {}): Onboarding => ({
  company_name: "",
  can_manage: true,
  agent_available: true,
  computer_connected: false,
  has_apps: false,
  has_successful_run: false,
  has_team: false,
  ...over,
})

const mount = () => render(<MemoryRouter><SetupPage /></MemoryRouter>)

test("shows confirmed progress without a company-name step", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ company_name: "Alis", computer_connected: true }))
  mount()

  expect(await screen.findByText("Prepare o Ziv para sua equipe")).toBeInTheDocument()
  // The name comes from signup: no step, no field for it.
  expect(screen.queryByLabelText("Nome da empresa")).not.toBeInTheDocument()
  expect(screen.queryByText("Dê um nome à empresa")).not.toBeInTheDocument()
  expect(screen.getByText("1")).toBeInTheDocument()
  expect(screen.getByText("/4")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Baixar Agente" })).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Passo a passo" })).toHaveAttribute("href", "/instalar-agente")
  expect(screen.getByRole("link", { name: "Registrar app" })).toHaveAttribute("href", "/registrar")
  expect(screen.getByRole("link", { name: "Abrir equipe" })).toHaveAttribute("href", "/equipe?aba=pessoas")
})

test("does not claim setup is complete before the Hub reports it", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ company_name: "Alis", can_manage: false, agent_available: false }))
  mount()
  expect(await screen.findByText("0")).toBeInTheDocument()
  expect(screen.queryByRole("link", { name: "Baixar Agente" })).not.toBeInTheDocument()
  expect(screen.queryByText(/Tudo pronto/)).not.toBeInTheDocument()
})

test("has a single h1 and a neutral Abrir catálogo", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ company_name: "Acme", computer_connected: true, has_apps: true }))
  mount()
  const link = await screen.findByRole("link", { name: "Abrir catálogo" })
  expect(link.className).not.toContain("bg-brand")
  expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1)
})
