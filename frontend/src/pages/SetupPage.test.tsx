import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type Onboarding } from "@/lib/api"
import { SetupPage } from "./SetupPage"

afterEach(() => vi.restoreAllMocks())

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

test("shows confirmed progress and saves the company name through onboarding", async () => {
  const user = userEvent.setup()
  const get = vi.spyOn(api, "onboarding").mockResolvedValue(state())
  const save = vi.spyOn(api, "saveCompanyName").mockResolvedValue(state({ company_name: "Alis" }))
  mount()

  expect(await screen.findByText("Prepare o App Hub para sua equipe")).toBeInTheDocument()
  expect(screen.getByText("0")).toBeInTheDocument()
  expect(screen.queryByRole("link", { name: "Baixar Agente" })).toBeInTheDocument()
  await user.type(screen.getByLabelText("Nome da empresa"), "Alis")
  await user.click(screen.getByRole("button", { name: "Salvar nome" }))

  expect(save).toHaveBeenCalledWith("Alis")
  expect(await screen.findByText("1")).toBeInTheDocument()
  expect(get).toHaveBeenCalledTimes(1)
  expect(screen.getByRole("link", { name: "Configurar computador" })).toHaveAttribute("href", "/configuracoes#computador")
  expect(screen.getByRole("link", { name: "Registrar app" })).toHaveAttribute("href", "/registrar")
  expect(screen.getByRole("link", { name: "Abrir equipe" })).toHaveAttribute("href", "/equipe?aba=pessoas")
})

test("does not claim setup is complete before the Hub reports it", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ company_name: "Alis", can_manage: false, agent_available: false }))
  mount()
  expect(await screen.findByText("1")).toBeInTheDocument()
  expect(screen.getByText(/só uma pessoa com permissão de gestão/i)).toBeInTheDocument()
  expect(screen.queryByRole("link", { name: "Baixar Agente" })).not.toBeInTheDocument()
  expect(screen.queryByText(/Tudo pronto/)).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Salvar nome" })).toBeDisabled()
})
