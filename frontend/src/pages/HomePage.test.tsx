import { act, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type Home, type Person, type TeamHome } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { activeRun, bia, henry } from "@/test/fixtures"
import { HomePage } from "./HomePage"

const empty: Home = { attention: [], favorites: [], recent: [], runs: [] }
const now = () => new Date().toISOString()
const team = (o: Partial<TeamHome> = {}): TeamHome => ({ scope: "areas", areas: ["Investimentos"], usage: [], never_used: [], creating: [], pending: null, ...o })

const asPerson = (p: Person) =>
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: p, refresh: async () => {}, setPerson: () => {} })

beforeEach(() => {
  asPerson(bia)
  vi.spyOn(api, "homeTeam").mockRejectedValue(new Error("404"))
})
afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter>
      <HomePage />
    </MemoryRouter>,
  )

test("shows warnings with links", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "home").mockResolvedValue({ ...empty, attention: [{ kind: "run_error", at: now(), app_id: 4, app_name: "Rentabilidade", run_id: 9 }] })
  const dismiss = vi.spyOn(api, "dismissRun").mockResolvedValue(undefined)
  mount()
  const link = await screen.findByRole("link", { name: "Rentabilidade deu erro" })
  expect(link).toHaveAttribute("href", "/execucoes/9")
  await user.click(screen.getByRole("button", { name: /^Dispensar/ }))
  expect(dismiss).toHaveBeenCalledWith(9)
  await waitFor(() => expect(screen.queryByRole("link", { name: "Rentabilidade deu erro" })).not.toBeInTheDocument())
})

test("shows the interrupted run warning", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "home").mockResolvedValue({ ...empty, attention: [{ kind: "run_interrupted", at: now(), app_id: 4, app_name: "Rentabilidade", run_id: 11 }] })
  const dismiss = vi.spyOn(api, "dismissRun").mockResolvedValue(undefined)
  mount()
  const link = await screen.findByRole("link", { name: "Rentabilidade não rodou: seu computador não respondeu" })
  expect(link).toHaveAttribute("href", "/execucoes/11")
  await user.click(screen.getByRole("button", { name: /^Dispensar/ }))
  expect(dismiss).toHaveBeenCalledWith(11)
})

test("warnings without a time show none", async () => {
  vi.spyOn(api, "home").mockResolvedValue({ ...empty, attention: [{ kind: "approval_pending", app_id: 4, app_name: "Rentabilidade" }] })
  mount()
  await screen.findByRole("link", { name: /esperando aprovação/ })
  expect(screen.queryByText("agora")).not.toBeInTheDocument()
})

test("a paused schedule says which app and why", async () => {
  vi.spyOn(api, "home").mockResolvedValue({ ...empty, attention: [{ kind: "schedule_paused", app_id: 4, app_name: "Rentabilidade", reason: "Falhou 5 vezes seguidas" }] })
  mount()
  const link = await screen.findByRole("link", { name: "Rentabilidade: agendamento pausado — Falhou 5 vezes seguidas" })
  expect(link).toHaveAttribute("href", "/apps/4")
})

test("hides empty parts", async () => {
  vi.spyOn(api, "home").mockResolvedValue(empty)
  mount()
  expect(await screen.findByText(/Marque apps e artifacts com a estrela/)).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: "Favoritos" })).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Atenção" })).not.toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Usados recentemente" })).not.toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Minhas últimas execuções" })).not.toBeInTheDocument()
})

test("hides Meu time", async () => {
  vi.spyOn(api, "home").mockResolvedValue(empty)
  mount()
  await screen.findByRole("heading", { name: "Favoritos" })
  expect(api.homeTeam).not.toHaveBeenCalled()
  expect(screen.queryByRole("heading", { name: "Meu time" })).not.toBeInTheDocument()
})

test("shows Meu time for a leader", async () => {
  asPerson({ ...bia, leads: [2] })
  vi.spyOn(api, "home").mockResolvedValue(empty)
  vi.spyOn(api, "homeTeam").mockResolvedValue(
    team({
      usage: [{ person_id: 5, name: "Caio", runs_7d: 3, opens_7d: 4 }],
      creating: [{ kind: "app", id: 7, name: "Painel", person_name: "Ana", event: "new_version", version: 2, at: now() }],
    }),
  )
  mount()
  expect(await screen.findByRole("heading", { name: "Meu time" })).toBeInTheDocument()
  expect(screen.getByText("Áreas: Investimentos")).toBeInTheDocument()
  expect(within(screen.getByRole("row", { name: /Caio/ })).getAllByRole("cell").map((c) => c.textContent)).toEqual(["Caio", "3", "4"])
  expect(screen.getByRole("link", { name: "Painel" })).toHaveAttribute("href", "/apps/7")
  expect(screen.getByText(/Ana subiu a versão 2 de/)).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Pendências de gestão" })).not.toBeInTheDocument()
})

test("shows pending management items for the admin", async () => {
  asPerson(henry)
  vi.spyOn(api, "home").mockResolvedValue(empty)
  vi.spyOn(api, "homeTeam").mockResolvedValue(
    team({ scope: "company", areas: [], pending: { invitations: [{ id: 1, email: "x@y.com", created_at: now(), expired: true }], requests: [], without_area: [] } }),
  )
  mount()
  expect(await screen.findByRole("heading", { name: "Pendências de gestão" })).toBeInTheDocument()
  expect(screen.getByText("Empresa toda")).toBeInTheDocument()
  expect(screen.getByText("venceu")).toBeInTheDocument()
})

test("unstar removes the favorite card", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "home").mockResolvedValue({ ...empty, favorites: [{ kind: "artifact", id: 3, name: "Guia de RH", summary: "Políticas", area: "Gente" }] })
  vi.spyOn(api, "setFavorite").mockResolvedValue(undefined as never)
  mount()
  expect(await screen.findByRole("link", { name: /Guia de RH/ })).toHaveAttribute("href", "/artifacts/3")
  await user.click(screen.getByRole("button", { name: "Tirar Guia de RH dos favoritos" }))
  await waitFor(() => expect(screen.queryByRole("link", { name: /Guia de RH/ })).not.toBeInTheDocument())
  expect(screen.getByText(/Marque apps e artifacts com a estrela/)).toBeInTheDocument()
})

test("shows each computer warning once", async () => {
  const outdated = { kind: "agent_outdated" as const, computer_name: "PC-1", at: now() }
  vi.spyOn(api, "home").mockResolvedValue({ ...empty, attention: [outdated, { ...outdated }] })
  mount()
  expect(await screen.findAllByRole("link", { name: /O Agente do computador PC-1 está desatualizado/ })).toHaveLength(1)
})

// Rodando agora (ADR 033): one compact line per live run while something runs.
test("Rodando agora: one line per live run, linking to it, with a ticking timer", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "Date"] })
  vi.setSystemTime(new Date("2026-11-09T12:03:00Z"))
  try {
    vi.spyOn(api, "home").mockResolvedValue(empty)
    vi.spyOn(api, "activeRuns").mockResolvedValue([
      activeRun({ id: 7, app_name: "Rentabilidade", status: "running", started_at: "2026-11-09T12:00:00Z" }),
      activeRun({ id: 8, app_name: "Fechamento", status: "waiting", started_at: null }),
      activeRun({ id: 9, app_name: "Antigo", status: "done", started_at: "2026-11-09T10:00:00Z", finished_at: "2026-11-09T10:01:00Z" }),
    ])
    render(
      <MemoryRouter>
        <RunsProvider>
          <HomePage />
        </RunsProvider>
      </MemoryRouter>,
    )
    const list = await screen.findByRole("list", { name: "Rodando agora" })
    const lines = within(list).getAllByRole("listitem")
    expect(lines).toHaveLength(2)
    expect(within(lines[0]).getByRole("link", { name: "Rentabilidade" })).toHaveAttribute("href", "/execucoes/7")
    expect(lines[0]).toHaveTextContent("Rodando agora: Rentabilidade há 3 min 00 s")
    expect(lines[1]).toHaveTextContent("Rodando agora: Fechamento esperando o computador")
    act(() => { vi.advanceTimersByTime(2000) })
    expect(lines[0]).toHaveTextContent("há 3 min 02 s")
  } finally {
    vi.useRealTimers()
  }
})

test("no Rodando agora line when nothing runs", async () => {
  vi.spyOn(api, "home").mockResolvedValue(empty)
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "done" })])
  render(
    <MemoryRouter>
      <RunsProvider>
        <HomePage />
      </RunsProvider>
    </MemoryRouter>,
  )
  await screen.findByText(/Marque apps e artifacts/)
  expect(screen.queryByRole("list", { name: "Rodando agora" })).toBeNull()
})
