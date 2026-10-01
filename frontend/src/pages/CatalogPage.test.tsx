import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { appSummary, henry, run } from "@/test/fixtures"
import { CatalogPage } from "./CatalogPage"

beforeEach(() => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: henry, refresh: async () => {}, setPerson: () => {} })
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
})
afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/"]}>
      <RunsProvider>
        <Routes>
          <Route path="/" element={<CatalogPage />} />
          <Route path="/apps/:id" element={<p>ficha</p>} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )

test("card with can_run has Rodar; a document has none", async () => {
  vi.spyOn(api, "apps").mockResolvedValue([appSummary(), appSummary({ id: 4, name: "Expandir Projeção", kind: "reference", can_run: false })])
  mount()
  await screen.findByText("Expandir Projeção")
  expect(screen.getAllByRole("button", { name: "Rodar Eq. Research Files Prep" })).toHaveLength(1)
  expect(screen.queryByRole("button", { name: "Rodar Expandir Projeção" })).not.toBeInTheDocument()
})

test("Rodar on the card runs directly with the remembered folder and stays on the catalog", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "apps").mockResolvedValue([appSummary({ remembered_folder: "D:\\Saídas" })])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run())
  mount()
  await user.click(await screen.findByRole("button", { name: "Rodar Eq. Research Files Prep" }))
  expect(start).toHaveBeenCalledWith(3, "D:\\Saídas", undefined)
  expect(screen.queryByText("ficha")).not.toBeInTheDocument()
})

test("Rodar on the card the first time asks where to save", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "apps").mockResolvedValue([appSummary()])
  mount()
  await user.click(await screen.findByRole("button", { name: "Rodar Eq. Research Files Prep" }))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("C:\\apps\\fp")
})

test("clicking the card name still opens the ficha", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "apps").mockResolvedValue([appSummary()])
  mount()
  await user.click(await screen.findByRole("link", { name: "Eq. Research Files Prep" }))
  expect(await screen.findByText("ficha")).toBeInTheDocument()
})
