import { act, render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type Person, type RunListItem } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { activeRun, bia, detail, henry, manifest } from "@/test/fixtures"
import { MyRunsPage } from "./MyRunsPage"

beforeEach(() => vi.spyOn(api, "activeRuns").mockResolvedValue([]))
afterEach(() => vi.restoreAllMocks())

const item = (over: Partial<RunListItem> = {}): RunListItem => ({
  id: 9, app_id: 3, app_name: "Eq. Research Files Prep", status: "done", output_folder: "E:\\Velha", person_id: 1, person_name: "Henry",
  created_at: "2026-09-30T13:42:00Z", started_at: "2026-09-30T13:42:00Z", finished_at: "2026-09-30T13:43:00Z", file_count: 4, ...over,
})

const mount = (person: Person = henry) => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person, refresh: async () => {}, setPerson: () => {} })
  return render(
    <MemoryRouter>
      <RunsProvider>
        <MyRunsPage />
      </RunsProvider>
    </MemoryRouter>,
  )
}

test("lists my runs with Ver e baixar and Rodar de novo in the same folder", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "runs").mockResolvedValue({ items: [item()], more: false })
  vi.spyOn(api, "app").mockResolvedValue(detail({ manifest: manifest({ kind: "desktop" }) }))
  const start = vi.spyOn(api, "startRun").mockResolvedValue(activeRun())
  mount()
  expect(await screen.findByRole("link", { name: "Eq. Research Files Prep" })).toHaveAttribute("href", "/apps/3")
  expect(screen.getByText(/4 arquivos/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Ver e baixar" })).toHaveAttribute("href", "/execucoes/9")
  await user.click(screen.getByRole("button", { name: "Rodar de novo" }))
  await vi.waitFor(() => expect(start).toHaveBeenCalledWith(3, "E:\\Velha", undefined))
})

test("admin can switch to De todos and sees who ran", async () => {
  const user = userEvent.setup()
  const runs = vi.spyOn(api, "runs").mockResolvedValue({ items: [item({ person_name: "Ana" })], more: false })
  mount(henry)
  await screen.findByRole("link", { name: "Eq. Research Files Prep" })
  expect(runs).toHaveBeenLastCalledWith("mine")
  await user.click(screen.getByRole("button", { name: "De todos" }))
  expect(runs).toHaveBeenLastCalledWith("all")
  expect(await screen.findByText(/· Ana/)).toBeInTheDocument()
})

test("non-admin has no De todos and sees an empty state", async () => {
  vi.spyOn(api, "runs").mockResolvedValue({ items: [], more: false })
  mount(bia)
  expect(await screen.findByText(/Você ainda não rodou nenhum app/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "De todos" })).not.toBeInTheDocument()
})

test("Carregar mais asks for the next page", async () => {
  const user = userEvent.setup()
  const runs = vi
    .spyOn(api, "runs")
    .mockResolvedValueOnce({ items: [item({ id: 9 })], more: true })
    .mockResolvedValueOnce({ items: [item({ id: 5 })], more: false })
  mount()
  await user.click(await screen.findByRole("button", { name: "Carregar mais" }))
  expect(runs).toHaveBeenLastCalledWith("mine", 9)
  expect(await screen.findAllByRole("link", { name: "Eq. Research Files Prep" })).toHaveLength(2)
  expect(screen.queryByRole("button", { name: "Carregar mais" })).not.toBeInTheDocument()
})

test("live runs show their situation, no file count", async () => {
  vi.spyOn(api, "runs").mockResolvedValue({ items: [item({ status: "running", finished_at: null })], more: false })
  mount()
  expect(await screen.findByText(/Rodando/)).toBeInTheDocument()
  expect(screen.queryByText(/arquivos/)).not.toBeInTheDocument()
})

test("reloads when a run in the strip changes", async () => {
  const runs = vi.spyOn(api, "runs").mockResolvedValue({ items: [], more: false })
  vi.mocked(api.activeRuns).mockResolvedValue([activeRun({ status: "done" })])
  mount()
  await vi.waitFor(() => expect(runs).toHaveBeenCalledTimes(2))
})

test("De todos: Rodar de novo only on my own runs", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "runs").mockResolvedValue({ items: [item({ id: 9, person_id: 2, person_name: "Ana" }), item({ id: 8 })], more: false })
  mount(henry)
  await user.click(await screen.findByRole("button", { name: "De todos" }))
  await screen.findByText(/· Ana/)
  expect(screen.getAllByRole("button", { name: "Rodar de novo" })).toHaveLength(1)
  expect(screen.getAllByRole("link", { name: "Ver e baixar" })).toHaveLength(2)
})

// Rodando agora (ADR 033): live runs on top with a ticking timer and Parar.
test("Rodando agora shows each live run with a ticking timer, the computer and Parar", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "Date"] })
  vi.setSystemTime(new Date("2026-11-09T12:03:12Z"))
  try {
    const user = userEvent.setup()
    vi.mocked(api.activeRuns).mockResolvedValue([
      activeRun({ id: 7, status: "running", started_at: "2026-11-09T12:00:00Z", computer_name: "NOTE-HENRY", can_stop: true }),
    ])
    vi.spyOn(api, "runs").mockResolvedValue({ items: [item({ id: 7, status: "running", finished_at: null })], more: false })
    const stop = vi.spyOn(api, "stopRun").mockResolvedValue(undefined as never)
    mount()
    const block = await screen.findByRole("region", { name: "Rodando agora" })
    expect(within(block).getByRole("link", { name: /Eq\. Research Files Prep/ })).toHaveAttribute("href", "/execucoes/7")
    expect(block).toHaveTextContent("rodando há 3 min 12 s")
    expect(block).toHaveTextContent("NOTE-HENRY")
    act(() => { vi.advanceTimersByTime(4000) })
    expect(block).toHaveTextContent("rodando há 3 min 16 s")
    await user.click(within(block).getByRole("button", { name: "Parar" }))
    const dialog = await screen.findByRole("dialog")
    await user.click(within(dialog).getByRole("button", { name: "Parar" }))
    await vi.waitFor(() => expect(stop).toHaveBeenCalledWith(7))
  } finally {
    vi.useRealTimers()
  }
})

test("no Rodando agora block when nothing runs, and no Parar when the Agente cannot stop", async () => {
  vi.mocked(api.activeRuns).mockResolvedValue([activeRun({ status: "done" })])
  vi.spyOn(api, "runs").mockResolvedValue({ items: [item()], more: false })
  const { unmount } = mount()
  await screen.findByRole("link", { name: "Eq. Research Files Prep" })
  expect(screen.queryByRole("region", { name: "Rodando agora" })).toBeNull()
  unmount()
  vi.mocked(api.activeRuns).mockResolvedValue([activeRun({ status: "running", can_stop: false })])
  mount()
  const block = await screen.findByRole("region", { name: "Rodando agora" })
  expect(within(block).queryByRole("button", { name: "Parar" })).toBeNull()
})
