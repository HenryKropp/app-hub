import { act, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type Onboarding } from "@/lib/api"
import * as auth from "@/lib/auth"
import { ana, henry } from "@/test/fixtures"
import { SetupPage } from "./SetupPage"

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })
beforeEach(() => {
  vi.spyOn(api, "workspace").mockResolvedValue({ slug: "acme", name: "Acme", base_path: "/w/acme", can_manage: true, local_execution_is_sandboxed: false })
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: henry, refresh: async () => {}, setPerson: vi.fn() })
})

const state = (over: Partial<Onboarding> = {}): Onboarding => ({
  company_name: "Acme",
  can_manage: true,
  agent_available: true,
  computer_connected: false,
  has_apps: false,
  has_successful_run: false,
  has_team: false,
  claude_connected: false,
  has_artifacts: false,
  ...over,
})

const mount = (entry = "/comecar") => render(<MemoryRouter initialEntries={[entry]}><SetupPage /></MemoryRouter>)
const header = (name: RegExp | string) => screen.getByRole("button", { name })

test("the first step not done is the only one open; done steps show a check and a summary", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ computer_connected: true }))
  mount()
  expect(await screen.findByText("1 de 6 feitos")).toBeInTheDocument()
  expect(header(/Conectar o Claude/)).toHaveAttribute("aria-expanded", "true")
  expect(header(/Conectar seu computador/)).toHaveAttribute("aria-expanded", "false")
  expect(header(/Conectar seu computador/)).toHaveTextContent("✓")
  expect(header(/Conectar seu computador/)).toHaveTextContent("Agente ligado")
  expect(document.querySelectorAll('[aria-expanded="true"]')).toHaveLength(1)
  expect(screen.getByRole("button", { name: "Gerar código" })).toBeInTheDocument()
  expect(screen.getByText(/Fica ✓ quando o Claude falar com o Ziv pela primeira vez/)).toBeInTheDocument()
})

test("clicking a header opens that step and closes the other", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ computer_connected: true }))
  mount()
  await screen.findByText("1 de 6 feitos")
  await user.click(header(/Rodar o app/))
  expect(header(/Rodar o app/)).toHaveAttribute("aria-expanded", "true")
  expect(header(/Conectar o Claude/)).toHaveAttribute("aria-expanded", "false")
  expect(screen.getByRole("link", { name: "Abrir o catálogo" })).toHaveAttribute("href", "/apps")
})

test("the hash opens its step", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ computer_connected: true }))
  mount("/comecar#computador")
  await screen.findByText("1 de 6 feitos")
  expect(header(/Conectar seu computador/)).toHaveAttribute("aria-expanded", "true")
  expect(screen.getByRole("link", { name: /Baixar Agente/ })).toBeInTheDocument()
  expect(screen.getByText(/Executar assim mesmo/)).toBeInTheDocument()
})

test("without a prepared Agente the computer step says so", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ agent_available: false }))
  mount()
  expect(await screen.findByText("O Agente ainda não foi preparado neste Ziv. Peça a quem administra o Ziv.")).toBeInTheDocument()
  expect(screen.queryByRole("link", { name: /Baixar Agente/ })).not.toBeInTheDocument()
})

test("step links", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "onboarding").mockResolvedValue(state())
  mount()
  await screen.findByText("0 de 6 feitos")
  await user.click(header(/Subir seu primeiro app/))
  expect(screen.getByRole("link", { name: "Registrar app" })).toHaveAttribute("href", "/registrar")
  await user.click(header(/Publicar um artifact/))
  expect(screen.getByRole("link", { name: "Publicar pelo site" })).toHaveAttribute("href", "/artifacts/novo")
  await user.click(header(/Convidar a equipe/))
  expect(screen.getByRole("link", { name: "Abrir equipe" })).toHaveAttribute("href", "/equipe?aba=pessoas")
})

test("a plain member sees only four steps", async () => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: { ...ana, permissions: [] }, refresh: async () => {}, setPerson: vi.fn() })
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ can_manage: false }))
  mount()
  expect(await screen.findByText("0 de 4 feitos")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: /Convidar a equipe/ })).not.toBeInTheDocument()
  expect(screen.queryByRole("button", { name: /Subir seu primeiro app/ })).not.toBeInTheDocument()
})

test("everything done shows Tudo pronto", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state({ computer_connected: true, claude_connected: true, has_apps: true, has_successful_run: true, has_artifacts: true, has_team: true }))
  mount()
  expect(await screen.findByText("6 de 6 feitos")).toBeInTheDocument()
  expect(screen.getByText(/Tudo pronto/)).toBeInTheDocument()
})

test("does not claim completion before the Ziv reports it", async () => {
  vi.spyOn(api, "onboarding").mockResolvedValue(state())
  mount()
  expect(await screen.findByText("0 de 6 feitos")).toBeInTheDocument()
  expect(screen.queryByText(/Tudo pronto/)).not.toBeInTheDocument()
  expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1)
})

test("polls every 5 seconds so checks appear by themselves", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  const spy = vi.spyOn(api, "onboarding").mockResolvedValue(state())
  mount()
  expect(await screen.findByText("0 de 6 feitos")).toBeInTheDocument()
  spy.mockResolvedValue(state({ computer_connected: true }))
  await act(async () => { await vi.advanceTimersByTimeAsync(5100) })
  expect(await screen.findByText("1 de 6 feitos")).toBeInTheDocument()
})
