import { cleanup, render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type Person } from "@/lib/api"
import * as auth from "@/lib/auth"
import { activeRun, bia, henry } from "@/test/fixtures"
import { Layout } from "./Layout"

afterEach(() => vi.restoreAllMocks())

const mount = (person: Person) => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person, refresh: async () => {}, setPerson: () => {} })
  render(
    <MemoryRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<p>catálogo</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

test("Minhas execuções is in the sidebar for everyone and counts open apps", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ id: 1, status: "running" }), activeRun({ id: 2, status: "waiting" }), activeRun({ id: 3, status: "done" })])
  mount(bia)
  expect(screen.getByRole("link", { name: /Minhas execuções/ })).toHaveAttribute("href", "/execucoes")
  expect(await screen.findByLabelText("2 abertos agora")).toHaveTextContent("2")
})

test("no counter when nothing is open", async () => {
  const active = vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "done" })])
  mount(henry)
  await vi.waitFor(() => expect(active).toHaveBeenCalled())
  expect(screen.queryByLabelText(/abertos? agora/)).not.toBeInTheDocument()
})

test("Configurações sits at the foot of the sidebar; Meu computador moved into it", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  mount(bia)
  expect(screen.getByRole("link", { name: "Configurações" })).toHaveAttribute("href", "/configuracoes")
  expect(screen.queryByRole("link", { name: "Meu computador" })).not.toBeInTheDocument()
  expect(screen.queryByRole("link", { name: "Registrar apps" })).not.toBeInTheDocument()
  expect(screen.queryByRole("radiogroup", { name: "Tema" })).not.toBeInTheDocument()
})

test("Em breve is only for who registers apps; a skip link leads to the content", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  mount(bia)
  expect(screen.queryByText("Em breve")).not.toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Pular para o conteúdo" })).toHaveAttribute("href", "#conteudo")
})

test("shows Equipe only to who takes care of the team", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  mount(bia)
  expect(screen.queryByRole("link", { name: /Equipe/ })).toBeNull()
  cleanup()
  mount(henry)
  expect(screen.getAllByRole("link", { name: /Equipe/ }).length).toBeGreaterThan(0)
})
