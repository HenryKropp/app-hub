import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type ArtifactSummary } from "@/lib/api"
import * as auth from "@/lib/auth"
import { bia, henry } from "@/test/fixtures"
import { ArtifactsPage } from "./ArtifactsPage"

const art = (o: Partial<ArtifactSummary> = {}): ArtifactSummary => ({
  id: 1, name: "Dashboard de vendas", summary: "Vendas por região", kind: "file", tags: ["vendas", "mensal"],
  area_id: 1, area: "Comercial", owner_name: "Henry", restricted: false, version: 2, updated_at: new Date().toISOString(), ...o,
})
const list = () => [art(), art({ id: 2, name: "Guia de RH", summary: "Políticas", kind: "link", tags: ["pessoas"], area_id: 2, area: "Gente", restricted: true })]

beforeEach(() => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: henry, refresh: async () => {}, setPerson: () => {} })
})
afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/artifacts"]}>
      <Routes>
        <Route path="/artifacts" element={<ArtifactsPage />} />
      </Routes>
    </MemoryRouter>,
  )

test("lists artifacts and filters by search", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "artifacts").mockResolvedValue(list())
  mount()
  expect(await screen.findByText("Dashboard de vendas")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /Guia de RH/ })).toHaveAttribute("href", "/artifacts/2")
  await user.type(screen.getByRole("searchbox", { name: "Buscar artifacts" }), "GENTE")
  expect(screen.queryByText("Dashboard de vendas")).not.toBeInTheDocument()
  expect(screen.getByText("Guia de RH")).toBeInTheDocument()
  await user.clear(screen.getByRole("searchbox", { name: "Buscar artifacts" }))
  await user.type(screen.getByRole("searchbox", { name: "Buscar artifacts" }), "zzz")
  expect(screen.getByText("Nada encontrado para essa busca.")).toBeInTheDocument()
})

test("clicking a tag filters and the chip clears it", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "artifacts").mockResolvedValue(list())
  mount()
  await screen.findByText("Dashboard de vendas")
  await user.click(screen.getByRole("button", { name: "vendas" }))
  expect(screen.queryByText("Guia de RH")).not.toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Remover filtro vendas" }))
  expect(screen.getByText("Guia de RH")).toBeInTheDocument()
})

test("filters by area", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "artifacts").mockResolvedValue(list())
  mount()
  await screen.findByText("Dashboard de vendas")
  await user.selectOptions(screen.getByRole("combobox", { name: "Área" }), "Gente")
  expect(screen.queryByText("Dashboard de vendas")).not.toBeInTheDocument()
  expect(screen.getByText("Guia de RH")).toBeInTheDocument()
})

test("hides Novo artifact for a usuário", async () => {
  vi.spyOn(api, "artifacts").mockResolvedValue(list())
  mount()
  expect(await screen.findByRole("link", { name: /Novo artifact/ })).toHaveAttribute("href", "/artifacts/novo")
  document.body.innerHTML = ""
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: bia, refresh: async () => {}, setPerson: () => {} })
  mount()
  await screen.findByText("Guia de RH")
  expect(screen.queryByRole("link", { name: /Novo artifact/ })).not.toBeInTheDocument()
})

test("shows empty state", async () => {
  vi.spyOn(api, "artifacts").mockResolvedValue([])
  mount()
  expect(await screen.findByText("Nenhum artifact ainda")).toBeInTheDocument()
  expect(screen.getByText("Guarde aqui os dashboards e páginas que o time cria no Claude.")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /Novo artifact/ })).toBeInTheDocument()
})
