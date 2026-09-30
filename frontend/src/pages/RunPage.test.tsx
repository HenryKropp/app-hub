import { render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { run } from "@/test/fixtures"
import { RunPage } from "./RunPage"

afterEach(() => vi.restoreAllMocks())
const renderAt = () =>
  render(
    <MemoryRouter initialEntries={["/execucoes/7"]}>
      <Routes><Route path="/execucoes/:id" element={<RunPage />} /></Routes>
    </MemoryRouter>,
  )

test("finished run shows groups with downloads", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run())
  renderAt()
  expect(await screen.findByText("Concluída")).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: /Data Pack/ })).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: /FRE cortado/ })).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /PETROBRAS_DataPack_Consolidado\.md/ })).toHaveAttribute("href", "/api/runs/7/files/2")
  expect(screen.getAllByRole("link", { name: /Baixar tudo/ })[0]).toHaveAttribute("href", "/api/runs/7/groups/0/zip")
})

test("running run tells the window is open", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "running", finished_at: null, groups: [] }))
  renderAt()
  expect(await screen.findByText(/a janela está aberta no seu computador/)).toBeInTheDocument()
})

test("error shows the message", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "error", error_message: "Já existe um Ferramenta CVM.exe nessa pasta.", groups: [] }))
  renderAt()
  expect(await screen.findByRole("alert")).toHaveTextContent("Já existe um Ferramenta CVM.exe nessa pasta.")
})
