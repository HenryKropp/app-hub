import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type Person, type RunListItem } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { activeRun, henry, run } from "@/test/fixtures"
import { MyRunsPage } from "./MyRunsPage"

beforeEach(() => vi.spyOn(api, "activeRuns").mockResolvedValue([]))
afterEach(() => vi.restoreAllMocks())

const item = (over: Partial<RunListItem> = {}): RunListItem => ({
  id: 9, app_id: 3, app_name: "Eq. Research Files Prep", status: "done", output_folder: "E:\\Velha", person_name: "Henry",
  created_at: "2026-09-30T13:42:00Z", started_at: "2026-09-30T13:42:00Z", finished_at: "2026-09-30T13:43:00Z", file_count: 4, ...over,
})

const bia: Person = { id: 3, name: "Bia", email: "bia@alis.com", role: "usuario" }

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
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run())
  mount()
  expect(await screen.findByRole("link", { name: "Eq. Research Files Prep" })).toHaveAttribute("href", "/apps/3")
  expect(screen.getByText(/4 arquivos/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Ver e baixar" })).toHaveAttribute("href", "/execucoes/9")
  await user.click(screen.getByRole("button", { name: "Rodar de novo" }))
  expect(start).toHaveBeenCalledWith(3, "E:\\Velha", undefined)
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
