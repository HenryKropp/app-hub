import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { appSummary, bia, henry, run } from "@/test/fixtures"
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
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("")
  expect(screen.queryByText(/Salva em/)).not.toBeInTheDocument()
})

test("card shows where the files go, and trocar asks again", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "apps").mockResolvedValue([appSummary({ remembered_folder: "D:\\Saídas\\CVM" })])
  const start = vi.spyOn(api, "startRun")
  mount()
  expect(await screen.findByText("CVM")).toHaveAttribute("title", "D:\\Saídas\\CVM")
  await user.click(screen.getByRole("button", { name: "trocar" }))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("D:\\Saídas\\CVM")
  expect(start).not.toHaveBeenCalled()
  expect(screen.queryByText("ficha")).not.toBeInTheDocument()
})

test("clicking the card name still opens the ficha", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "apps").mockResolvedValue([appSummary()])
  mount()
  await user.click(await screen.findByRole("link", { name: "Eq. Research Files Prep" }))
  expect(await screen.findByText("ficha")).toBeInTheDocument()
})

test("the + Registrar apps sits in the catalog for creators, not for usuários", async () => {
  vi.spyOn(api, "apps").mockResolvedValue([appSummary()])
  const { unmount } = mount()
  expect(await screen.findByRole("link", { name: "Registrar apps" })).toHaveAttribute("href", "/registrar")
  unmount()
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: bia, refresh: async () => {}, setPerson: () => {} })
  mount()
  await screen.findByText("Eq. Research Files Prep")
  expect(screen.queryByRole("link", { name: "Registrar apps" })).not.toBeInTheDocument()
})

test("a catalog that didn't load says why and tries again", async () => {
  const user = userEvent.setup()
  const apps = vi.spyOn(api, "apps").mockRejectedValueOnce(new Error("O Hub não respondeu. Espere um pouco e tente de novo.")).mockResolvedValue([appSummary()])
  mount()
  expect(await screen.findByRole("alert")).toHaveTextContent("O Hub não respondeu")
  await user.click(screen.getByRole("button", { name: "Tentar de novo" }))
  expect(await screen.findByText("Eq. Research Files Prep")).toBeInTheDocument()
  expect(apps).toHaveBeenCalledTimes(2)
})

test("search has a name and can be cleared", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "apps").mockResolvedValue([appSummary()])
  mount()
  await user.type(await screen.findByRole("searchbox", { name: "Buscar apps" }), "xyz")
  expect(screen.getByText(/Nenhum app encontrado/)).toBeInTheDocument()
  await user.click(screen.getAllByRole("button", { name: "Limpar busca" })[0]!)
  expect(screen.getByRole("searchbox", { name: "Buscar apps" })).toHaveValue("")
  expect(screen.getByText("Eq. Research Files Prep")).toBeInTheDocument()
})

test("the run window has Cancelar", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "apps").mockResolvedValue([appSummary()])
  const start = vi.spyOn(api, "startRun")
  mount()
  await user.click(await screen.findByRole("button", { name: "Rodar Eq. Research Files Prep" }))
  await user.click(screen.getByRole("button", { name: "Cancelar" }))
  await vi.waitFor(() => expect(screen.queryByLabelText("Onde salvar os resultados?")).not.toBeInTheDocument())
  expect(start).not.toHaveBeenCalled()
})

test("an empty catalog tells a usuário what to expect, not how to register", async () => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: bia, refresh: async () => {}, setPerson: () => {} })
  vi.spyOn(api, "apps").mockResolvedValue([])
  mount()
  expect(await screen.findByText(/Quando alguém registrar os apps/)).toBeInTheDocument()
  expect(screen.queryByRole("link", { name: /Registrar/ })).not.toBeInTheDocument()
})

test("shows Abrir for a shared app instead of Rodar", async () => {
  vi.spyOn(api, "apps").mockResolvedValue([appSummary({ name: "Notas", shared: { status: "running", url: "http://notas.localhost:8080", slug: "notas" } })])
  mount()
  const link = await screen.findByRole("link", { name: "Abrir Notas" })
  expect(link).toHaveAttribute("href", "/api/shared/authorize?app=notas&volta=%2F")
  expect(link).toHaveAttribute("target", "_blank")
  expect(screen.queryByRole("button", { name: "Rodar Notas" })).not.toBeInTheDocument()
  expect(screen.getByText("No ar")).toBeInTheDocument()
})

test("does not offer Abrir for a draft shared app", async () => {
  vi.spyOn(api, "apps").mockResolvedValue([appSummary({ name: "Notas", shared: { status: "draft", url: "http://notas.localhost:8080", slug: "notas" } })])
  mount()
  await screen.findByText("Notas")
  expect(screen.queryByRole("link", { name: "Abrir Notas" })).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Rodar Notas" })).toBeInTheDocument()
})
