import { cleanup, render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type Person } from "@/lib/api"
import * as auth from "@/lib/auth"
import { activeRun, bia, henry } from "@/test/fixtures"
import { Layout } from "./Layout"

afterEach(() => vi.restoreAllMocks())

const mount = (person: Person, at = "/") => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person, refresh: async () => {}, setPerson: () => {} })
  render(
    <MemoryRouter initialEntries={[at]}>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<p>início</p>} />
          <Route path="apps/:id" element={<p>ficha</p>} />
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

test("shows Servidor only to who can manage the server", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  mount(bia)
  expect(screen.queryByRole("link", { name: /Servidor/ })).toBeNull()
  cleanup()
  mount(henry)
  expect(screen.getAllByRole("link", { name: /Servidor/ })[0]).toHaveAttribute("href", "/servidor")
})

test("shows Artifacts in the sidebar", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  mount(bia)
  expect(screen.getByRole("link", { name: /Artifacts/ })).toHaveAttribute("href", "/artifacts")
})

test("sidebar has Home and Apps, no Catálogo; Apps stays on inside an app", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  mount(bia, "/apps/12")
  expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("href", "/")
  expect(screen.getByRole("link", { name: "Apps" })).toHaveAttribute("href", "/apps")
  expect(screen.queryByRole("link", { name: "Catálogo" })).toBeNull()
  expect(screen.getByRole("link", { name: "Apps" })).toHaveAttribute("aria-current", "page")
  expect(screen.getByRole("link", { name: "Home" })).not.toHaveAttribute("aria-current")
})
