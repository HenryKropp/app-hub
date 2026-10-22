import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { RunsProvider, useRuns } from "@/lib/runs-context"
import { activeRun } from "@/test/fixtures"
import { RunStrip } from "./RunStrip"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function Starter() {
  const { run } = useRuns()
  return <button onClick={() => run({ id: 3, name: "Files Prep", folder_display: "C:\\fp", remembered_folder: "D:\\Saídas", kind: "desktop" })}>go</button>
}

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/"]}>
      <RunsProvider>
        <Routes>
          <Route path="/" element={<><Starter /><RunStrip /></>} />
          <Route path="/execucoes/:id" element={<p>página da execução</p>} />
          <Route path="/execucoes" element={<p>minhas execuções</p>} />
          <Route path="/configuracoes" element={<p>meu computador</p>} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )

test("open app shows the hint and can be hidden", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "running" })])
  mount()
  expect(await screen.findByText(/aberto no seu computador/)).toBeInTheDocument()
  expect(screen.getByText("CVM")).toHaveAttribute("title", "D:\\Saídas\\CVM")
  await user.click(screen.getByRole("button", { name: "Ocultar" }))
  expect(screen.queryByText(/aberto no seu computador/)).not.toBeInTheDocument()
  // A reminder stays, and brings it back.
  await user.click(screen.getByRole("button", { name: /1 aberto oculto · Mostrar/ }))
  expect(screen.getByText(/aberto no seu computador/)).toBeInTheDocument()
})

test("Ver on an open app goes to its page and keeps it in the strip", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "running" })])
  const dismiss = vi.spyOn(api, "dismissRun").mockResolvedValue()
  mount()
  await user.click(await screen.findByRole("button", { name: "Ver" }))
  expect(await screen.findByText("página da execução")).toBeInTheDocument()
  expect(dismiss).not.toHaveBeenCalled()
})

test("waiting run says it is opening", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "waiting" })])
  mount()
  expect(await screen.findByText(/no seu computador…/)).toHaveTextContent("Abrindo Eq. Research Files Prep no seu computador…")
})

test("done run offers Ver e baixar, which dismisses and opens the run", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "done", file_count: 4 })])
  const dismiss = vi.spyOn(api, "dismissRun").mockResolvedValue()
  mount()
  expect(await screen.findByText(/terminou: 4 arquivos novos/)).toBeInTheDocument()
  expect(screen.getByText("Salva em")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Ver e baixar" }))
  expect(dismiss).toHaveBeenCalledWith(7)
  expect(await screen.findByText("página da execução")).toBeInTheDocument()
})

test("each ending has its own text", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([
    activeRun({ id: 1, status: "no_files" }),
    activeRun({ id: 2, status: "interrupted" }),
    activeRun({ id: 3, status: "error", error_message: "O Agente não conseguiu abrir o app." }),
  ])
  mount()
  expect(await screen.findByText(/terminou sem arquivos novos/)).toBeInTheDocument()
  expect(screen.getByText(/foi interrompido/)).toBeInTheDocument()
  expect(screen.getByText(/O Agente não conseguiu abrir o app./)).toBeInTheDocument()
  expect(screen.getAllByRole("button", { name: "Ver detalhes" })).toHaveLength(3)
})

test("✕ dismisses a finished run", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "no_files" })])
  const dismiss = vi.spyOn(api, "dismissRun").mockResolvedValue()
  mount()
  await user.click(await screen.findByRole("button", { name: "Dispensar" }))
  expect(dismiss).toHaveBeenCalledWith(7)
  expect(screen.queryByText(/terminou sem arquivos novos/)).not.toBeInTheDocument()
})

test("more than 3 shows +N linking to Minhas execuções", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([1, 2, 3, 4, 5].map((id) => activeRun({ id, status: "running" })))
  mount()
  await user.click(await screen.findByRole("link", { name: "+2 em Minhas execuções" }))
  expect(await screen.findByText("minhas execuções")).toBeInTheDocument()
})

test("offline error offers Ver Meu computador", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "startRun").mockRejectedValue(new ApiError(409, "Seu computador não está conectado."))
  mount()
  await user.click(screen.getByText("go"))
  expect(await screen.findByText(/Seu computador não está conectado/)).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Ver Meu computador" }))
  expect(await screen.findByText("meu computador")).toBeInTheDocument()
})

test("other start error offers trocar pasta, which opens the dialog", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "startRun").mockRejectedValue(new ApiError(400, "Informe a pasta completa, começando pela letra do disco."))
  mount()
  await user.click(screen.getByText("go"))
  await user.click(await screen.findByRole("button", { name: "Trocar pasta" }))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("D:\\Saídas")
})

test("interrupted run points to Meu computador", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "interrupted" })])
  vi.spyOn(api, "dismissRun").mockResolvedValue()
  mount()
  await user.click(await screen.findByRole("button", { name: "Ver Meu computador" }))
  expect(await screen.findByText("meu computador")).toBeInTheDocument()
})

test("trocar pasta only for folder errors", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([
    activeRun({ id: 1, status: "error", error_message: "Não consegui usar a pasta D:\\X: ela não existe ou não aceita gravação." }),
    activeRun({ id: 2, status: "error", error_message: "O Windows não abriu o app." }),
  ])
  mount()
  await screen.findByText(/O Windows não abriu o app/)
  expect(screen.getAllByRole("button", { name: "Trocar pasta" })).toHaveLength(1)
})

test("request error not about the folder has no trocar pasta", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "startRun").mockRejectedValue(new ApiError(400, "Este app ainda não pode rodar pelo Hub."))
  mount()
  await user.click(screen.getByText("go"))
  await screen.findByText(/ainda não pode rodar/)
  expect(screen.queryByRole("button", { name: "Trocar pasta" })).not.toBeInTheDocument()
})

test("the Abrindo notice already says where the files go", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "startRun").mockReturnValue(new Promise(() => {}))
  mount()
  await user.click(screen.getByText("go"))
  expect(await screen.findByText("Saídas")).toHaveAttribute("title", "D:\\Saídas")
})

test("no Salva em on an error card", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ status: "error", error_message: "deu ruim" })])
  mount()
  expect(await screen.findByText(/deu ruim/)).toBeInTheDocument()
  expect(screen.queryByText("Salva em")).not.toBeInTheDocument()
})

test("problems and results come first, so live runs are the ones cut into +N", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([
    activeRun({ id: 1, status: "running" }),
    activeRun({ id: 2, status: "running" }),
    activeRun({ id: 3, status: "no_files" }),
    activeRun({ id: 4, status: "done", file_count: 2 }),
    activeRun({ id: 5, status: "error", error_message: "deu ruim" }),
  ])
  mount()
  const err = await screen.findByText(/deu ruim/)
  const done = screen.getByText(/terminou: 2 arquivos/)
  const none = screen.getByText(/sem arquivos novos/)
  expect(err.compareDocumentPosition(done) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(done.compareDocumentPosition(none) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(screen.queryByText(/aberto no seu computador/)).not.toBeInTheDocument()
  expect(screen.getByRole("link", { name: "+2 em Minhas execuções" })).toBeInTheDocument()
})

test("the strip is a live region even before any card", async () => {
  const active = vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const { container } = mount()
  await vi.waitFor(() => expect(active).toHaveBeenCalled())
  expect(container.querySelector('[aria-live="polite"]')).toBeInTheDocument()
})

test("error cards are alerts, the others are not", async () => {
  vi.spyOn(api, "activeRuns").mockResolvedValue([
    activeRun({ id: 1, status: "done" }),
    activeRun({ id: 2, status: "error", error_message: "deu ruim" }),
  ])
  mount()
  expect(await screen.findByRole("alert")).toHaveTextContent("deu ruim")
  expect(screen.getAllByRole("alert")).toHaveLength(1)
})

test("on a phone the strip shows one card and counts the rest", async () => {
  vi.stubGlobal("matchMedia", (q: string) => ({ matches: q.includes("max-width"), media: q, addEventListener: () => {}, removeEventListener: () => {} }))
  vi.spyOn(api, "activeRuns").mockResolvedValue([activeRun({ id: 1, status: "running" }), activeRun({ id: 2, status: "no_files" })])
  mount()
  expect(await screen.findByText(/terminou sem arquivos novos/)).toBeInTheDocument()
  expect(screen.queryByText(/aberto no seu computador/)).not.toBeInTheDocument()
  expect(screen.getByRole("link", { name: "+1 em Minhas execuções" })).toBeInTheDocument()
})
