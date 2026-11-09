import { act, render, screen, within } from "@testing-library/react"
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
  // The dialog leaves after its closing animation.
  await vi.waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument())
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
  // The dialog leaves after its closing animation.
  await vi.waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument())
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

test("a run started by a schedule says agendada", async () => {
  vi.spyOn(api, "run").mockResolvedValueOnce(run({ origin: "schedule" }))
  renderAt()
  expect(await screen.findByText("agendada")).toBeInTheDocument()
})

test("a run started by Claude says pelo Claude; a manual one does not", async () => {
  vi.spyOn(api, "run").mockResolvedValueOnce(run({ origin: "mcp" })).mockResolvedValueOnce(run({ origin: "manual" }))
  const { unmount } = renderAt()
  const label = await screen.findByText("pelo Claude")
  expect(label.className).not.toMatch(/brand|coral/)
  unmount()
  renderAt()
  expect(await screen.findByText("Concluída")).toBeInTheDocument()
  expect(screen.queryByText("pelo Claude")).not.toBeInTheDocument()
})

// O que foi preenchido, the live timer and the app's log (ADR 033).
test("shows what was filled: labels, a file by its name, a masked secret and a date as people read it", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ inputs_recorded: true, inputs: [
    { id: "mes", label: "Mês", type: "text", values: ["outubro"], masked: false },
    { id: "dia", label: "Dia", type: "date", values: ["2026-11-09"], masked: false },
    { id: "docs", label: "Documentos", type: "choice", values: ["FRE", "ITR"], masked: false },
    { id: "base", label: "Base", type: "file", values: [], file: "base.csv", masked: false },
    { id: "senha", label: "Senha do portal", type: "text", values: [], masked: true },
  ] }))
  renderAt()
  const filled = await screen.findByRole("region", { name: "O que foi preenchido" })
  const value = (label: string) => within(filled).getByText(label).nextElementSibling
  expect(value("Mês")).toHaveTextContent("outubro")
  expect(value("Dia")).toHaveTextContent("09/11/2026")
  expect(value("Documentos")).toHaveTextContent("FRE, ITR")
  expect(value("Base")).toHaveTextContent("base.csv")
  expect(value("Senha do portal")).toHaveTextContent("escondido")
})

test("says when nothing was filled, and when the run is older than the record", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ inputs_recorded: true, inputs: [] }))
  const { unmount } = renderAt()
  expect(await screen.findByText("Nenhum valor preenchido.")).toBeInTheDocument()
  unmount()
  vi.mocked(api.run).mockResolvedValue(run({ inputs_recorded: false, inputs: [] }))
  renderAt()
  expect(await screen.findByText(/antes de o Ziv guardar o que foi preenchido/)).toBeInTheDocument()
})

test("a live run shows how long it has been running, ticking", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "Date"] })
  vi.setSystemTime(new Date("2026-11-09T12:00:45Z"))
  try {
    vi.spyOn(api, "run").mockResolvedValue(run({ status: "running", started_at: "2026-11-09T12:00:00Z", finished_at: null, groups: [] }))
    renderAt()
    const line = (await screen.findByText(/rodando há/)).closest("span")!
    expect(line).toHaveTextContent("rodando há 45 s")
    act(() => { vi.advanceTimersByTime(20_000) })
    expect(line).toHaveTextContent("rodando há 1 min 05 s")
  } finally {
    vi.useRealTimers()
  }
})

test("a finished run with a console log shows it in a readable block, and links to download it", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ console: { file_id: 9, size: 30 } }))
  const text = vi.spyOn(api, "runFileText").mockResolvedValue("Baixando PETROBRAS...\nPronto.\n")
  renderAt()
  const log = await screen.findByLabelText("Registro do app", { selector: "pre" })
  expect(log).toHaveTextContent("Baixando PETROBRAS... Pronto.")
  expect(text).toHaveBeenCalledWith(7, 9)
  expect(screen.getByRole("link", { name: /Baixar o registro/ })).toHaveAttribute("href", "/api/runs/7/files/9")
})

test("a long log shows only its end; a huge one is only downloaded; a failure says why", async () => {
  vi.spyOn(api, "run").mockResolvedValue(run({ console: { file_id: 9, size: 30_000 } }))
  vi.spyOn(api, "runFileText").mockResolvedValue("a".repeat(25_000) + "FIM")
  const first = renderAt()
  expect(await screen.findByText(/Mostrando só o final/)).toBeInTheDocument()
  expect(screen.getByLabelText("Registro do app", { selector: "pre" }).textContent).toHaveLength(20_000)
  first.unmount()

  vi.mocked(api.run).mockResolvedValue(run({ console: { file_id: 9, size: 5 * 1024 * 1024 } }))
  vi.mocked(api.runFileText).mockClear()
  const second = renderAt()
  expect(await screen.findByText(/grande demais para mostrar aqui/)).toBeInTheDocument()
  expect(api.runFileText).not.toHaveBeenCalled()
  second.unmount()

  vi.mocked(api.run).mockResolvedValue(run({ console: { file_id: 9, size: 30 } }))
  vi.mocked(api.runFileText).mockRejectedValue(new Error("O computador onde o app rodou (NOTE-HENRY) não está conectado."))
  renderAt()
  expect(await screen.findByText(/NOTE-HENRY\) não está conectado/)).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeInTheDocument()
})
