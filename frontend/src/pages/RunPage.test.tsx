import { render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { RunsProvider } from "@/lib/runs-context"
import { activeRun, detail, manifest, run } from "@/test/fixtures"
import { RunPage } from "./RunPage"

beforeEach(() => vi.spyOn(api, "activeRuns").mockResolvedValue([]))
afterEach(() => vi.restoreAllMocks())
const renderAt = () =>
  render(
    <MemoryRouter initialEntries={["/execucoes/7"]}>
      <RunsProvider>
        <Routes><Route path="/execucoes/:id" element={<RunPage />} /></Routes>
      </RunsProvider>
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

test("choose files and download the selection", async () => {
  const user = (await import("@testing-library/user-event")).default.setup()
  vi.spyOn(api, "run").mockResolvedValue(run())
  renderAt()
  await screen.findByText("Concluída")
  expect(screen.queryByRole("link", { name: /Baixar selecionados/ })).not.toBeInTheDocument()
  await user.click(screen.getByRole("checkbox", { name: "FRE_2025.pdf" }))
  await user.click(screen.getByRole("checkbox", { name: "log.txt" }))
  expect(screen.getByRole("link", { name: /Baixar selecionados \(2\)/ })).toHaveAttribute("href", "/api/runs/7/selected?files=1,3")
  await user.click(screen.getByRole("checkbox", { name: "log.txt" }))
  expect(screen.getByRole("link", { name: /Baixar selecionados \(1\)/ })).toHaveAttribute("href", "/api/runs/7/selected?files=1")
})

test("mark all in a group", async () => {
  const user = (await import("@testing-library/user-event")).default.setup()
  vi.spyOn(api, "run").mockResolvedValue(run())
  renderAt()
  await screen.findByText("Concluída")
  await user.click(screen.getByRole("checkbox", { name: "Marcar todos: Data Pack" }))
  expect(screen.getByRole("link", { name: /Baixar selecionados \(1\)/ })).toHaveAttribute("href", "/api/runs/7/selected?files=2")
})

test("Rodar de novo for a desktop app uses the same folder; back goes to Minhas execuções; app name links to the ficha", async () => {
  const user = (await import("@testing-library/user-event")).default.setup()
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "done", output_folder: "E:\\Velha", app_id: 3 }))
  vi.spyOn(api, "app").mockResolvedValue(detail())
  const start = vi.spyOn(api, "startRun").mockResolvedValue(activeRun())
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar de novo" }))
  await vi.waitFor(() => expect(start).toHaveBeenCalledWith(3, "E:\\Velha", undefined))
  expect(screen.getByRole("link", { name: /Minhas execuções/ })).toHaveAttribute("href", "/execucoes")
  expect(screen.getByRole("link", { name: "Eq. Research Files Prep" })).toHaveAttribute("href", "/apps/3")
})

test("Rodar de novo for a script opens its form with the previous folder and sends data only after confirmation", async () => {
  const user = (await import("@testing-library/user-event")).default.setup()
  vi.spyOn(api, "run").mockResolvedValue(run({ output_folder: "E:\\Velha", app_id: 3 }))
  const app = vi.spyOn(api, "app").mockResolvedValue(detail({ remembered_folder: "D:\\Outra", manifest: manifest({
    inputs: [
      { id: "cnpjs", label: "Arquivo cnpjs.csv", type: "file", source: "fixed_file", path: "cnpjs.csv", required: true },
      { id: "periodo", label: "Período", type: "text", source: "argument", required: true },
    ],
  }) }))
  const start = vi.spyOn(api, "startRun").mockResolvedValue(activeRun())
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar de novo" }))

  const fileInput = await screen.findByLabelText("Arquivo cnpjs.csv")
  expect(app).toHaveBeenCalledWith(3)
  expect(screen.getByRole("dialog", { name: "Rodar Eq. Research Files Prep" })).toBeInTheDocument()
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("E:\\Velha")
  expect(start).not.toHaveBeenCalled()

  await user.click(screen.getByRole("button", { name: "Executar script" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Preencha “Arquivo cnpjs.csv”")
  expect(start).not.toHaveBeenCalled()

  await user.upload(fileInput, new File(["123"], "cnpjs.csv", { type: "text/csv" }))
  await user.type(screen.getByLabelText("Período"), "2026-10")
  expect(start).not.toHaveBeenCalled()
  await user.click(screen.getByRole("button", { name: "Executar script" }))
  await vi.waitFor(() => expect(start).toHaveBeenCalledExactlyOnceWith(
    3, "E:\\Velha", true,
    { periodo: "2026-10" },
    { cnpjs: { name: "cnpjs.csv", content_base64: "MTIz" } },
  ))
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  expect(screen.getByRole("link", { name: /Minhas execuções/ })).toHaveAttribute("href", "/execucoes")
  expect(screen.getByRole("link", { name: "Eq. Research Files Prep" })).toHaveAttribute("href", "/apps/3")
})

test("canceling Rodar de novo for a script does not start a run", async () => {
  const user = (await import("@testing-library/user-event")).default.setup()
  vi.spyOn(api, "run").mockResolvedValue(run({ output_folder: "E:\\Velha" }))
  vi.spyOn(api, "app").mockResolvedValue(detail({ manifest: manifest() }))
  const start = vi.spyOn(api, "startRun").mockResolvedValue(activeRun())
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar de novo" }))
  await screen.findByLabelText("Arquivo cnpjs.csv")
  await user.click(screen.getByRole("button", { name: "Cancelar" }))
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  expect(start).not.toHaveBeenCalled()
})

test("live run has no Rodar de novo", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "running", finished_at: null, groups: [] }))
  renderAt()
  await screen.findByText(/a janela está aberta/)
  expect(screen.queryByRole("button", { name: "Rodar de novo" })).not.toBeInTheDocument()
})

test("open run of mine offers Parar", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "running", finished_at: null, groups: [], can_stop: true }))
  renderAt()
  expect(await screen.findByRole("button", { name: /Parar/ })).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: /Rodar de novo/ })).not.toBeInTheDocument()
})

test("stopped run explains and lists its files", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ status: "stopped" }))
  renderAt()
  expect(await screen.findByText("Parada")).toBeInTheDocument()
  expect(screen.getByText(/Você parou o app pelo Ziv\. Abaixo estão/)).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: /Data Pack/ })).toBeInTheDocument()
})
