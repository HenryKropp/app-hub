import { act, render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type AppDetail, type Person, type RunSummary } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { ALL_CAN, bia, detail, henry } from "@/test/fixtures"
import { AppPage } from "./AppPage"

// Execuções in the Cuidar face (ADR 033).

const NOW = new Date("2026-11-09T12:03:12Z")

beforeEach(() => {
  localStorage.clear()
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "Date"] })
  vi.setSystemTime(NOW)
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

const summary = (over: Partial<RunSummary>): RunSummary => ({
  id: 11, status: "done", person_name: "Bia", created_at: "2026-11-09T10:00:00Z", started_at: "2026-11-09T10:00:05Z",
  finished_at: "2026-11-09T10:02:05Z", file_count: 4, ...over,
})

const mount = (d: AppDetail, runs: RunSummary[], person: Person = henry) => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person, refresh: async () => {}, setPerson: () => {} })
  vi.spyOn(api, "app").mockResolvedValue(d)
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  vi.spyOn(api.keys, "list").mockResolvedValue({ keys: [], can_manage: true, ready: true, missing: [], locked: false })
  vi.spyOn(api, "appAccess").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [] })
  const list = vi.spyOn(api, "appRuns").mockResolvedValue(runs)
  render(
    <MemoryRouter initialEntries={["/apps/3"]}>
      <RunsProvider>
        <Routes>
          <Route path="/apps/:id" element={<AppPage />} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )
  return list
}

const openRuns = async () => {
  const nav = await screen.findByRole("navigation", { name: "Seções de cuidado" })
  await userEvent.click(within(nav).getByRole("button", { name: /^Execuções/ }))
  return screen.getByRole("region", { name: "Execuções" })
}

test("the Cuidar face has Execuções: running on top with a ticking timer, then the recent ones", async () => {
  mount(detail(), [
    summary({ id: 12, status: "running", person_name: "Ana", started_at: "2026-11-09T12:00:00Z", finished_at: null, file_count: 0 }),
    summary({}),
  ])
  const region = await openRuns()
  expect(await within(region).findByRole("heading", { name: "Rodando agora" })).toBeInTheDocument()
  const live = within(region).getByRole("link", { name: /Rodando/ })
  expect(live).toHaveAttribute("href", "/execucoes/12")
  expect(live).toHaveTextContent("há 3 min 12 s")
  expect(live).toHaveTextContent("Ana")
  act(() => { vi.advanceTimersByTime(5000) })
  expect(live).toHaveTextContent("há 3 min 17 s")

  const table = within(region).getByRole("table")
  const row = within(table).getAllByRole("row")[1]
  expect(row).toHaveTextContent("Concluída")
  expect(row).toHaveTextContent("2 min")
  expect(row).toHaveTextContent("Bia")
  expect(row).toHaveTextContent("4")
  expect(within(row).getAllByRole("link")[0]).toHaveAttribute("href", "/execucoes/11")
})

test("without seeing everyone's runs the tab is about your own, with no Quem column", async () => {
  mount(detail({ can: { ...ALL_CAN, runs: false } }), [summary({ person_name: "Bia" })], bia)
  const region = await openRuns()
  expect(await within(region).findByRole("heading", { name: "Suas execuções recentes" })).toBeInTheDocument()
  expect(within(region).queryByRole("columnheader", { name: "Quem" })).toBeNull()
  expect(within(region).queryByRole("heading", { name: "Rodando agora" })).toBeNull()
})

test("the Usar face keeps its list", async () => {
  mount(detail(), [summary({})])
  await userEvent.click(await screen.findByRole("tab", { name: "Usar" }))
  expect(await screen.findByRole("heading", { name: "Execuções" })).toBeInTheDocument()
  expect(await screen.findByRole("link", { name: /4 arquivos/ })).toHaveAttribute("href", "/execucoes/11")
})
