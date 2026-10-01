import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError, type AppDetail } from "@/lib/api"
import { manifest, run } from "@/test/fixtures"
import { AppPage } from "./AppPage"

afterEach(() => vi.restoreAllMocks())

const detail = (can_run: boolean): AppDetail => ({
  id: 3, folder_path: "/mnt/c/apps/fp", folder_display: "C:\\apps\\fp", owner_id: 1, backup_id: null, updated_at: "2026-09-30T10:00:00Z",
  manifest: manifest({ kind: "desktop", entrypoint: "Ferramenta CVM.exe", name: "Eq. Research Files Prep" }), can_run,
  version: { number: 2, created_at: "2026-09-30T10:00:00Z" }, folder_readable: true, remembered_folder: "",
})

const renderAt = () =>
  render(
    <MemoryRouter initialEntries={["/apps/3"]}>
      <Routes>
        <Route path="/apps/:id" element={<AppPage />} />
        <Route path="/execucoes/:id" element={<p>página da execução</p>} />
      </Routes>
    </MemoryRouter>,
  )

test("Rodar opens the dialog with the app folder and starts the run", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run({ status: "waiting" }))
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar" }))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("C:\\apps\\fp")
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(start).toHaveBeenCalledWith(3, "C:\\apps\\fp")
  expect(await screen.findByText("página da execução")).toBeInTheDocument()
})

test("choose another folder fills the field", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  vi.spyOn(api, "chooseFolder").mockResolvedValue({ folder: "D:\\Saídas", canceled: false })
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar" }))
  await user.click(screen.getByRole("button", { name: "Escolher outra…" }))
  expect(await screen.findByDisplayValue("D:\\Saídas")).toBeInTheDocument()
})

test("computer not connected points to Meu computador", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  vi.spyOn(api, "startRun").mockRejectedValue(new ApiError(409, "Seu computador não está conectado."))
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar" }))
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Seu computador não está conectado.")
  expect(screen.getByRole("link", { name: /Meu computador/ })).toHaveAttribute("href", "/meu-computador")
})

test("apps that cannot run have no Rodar button", async () => {
  vi.spyOn(api, "app").mockResolvedValue(detail(false))
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  renderAt()
  expect(await screen.findByText("Eq. Research Files Prep")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Rodar" })).not.toBeInTheDocument()
})

test("shows the version and warns when the folder cannot be read", async () => {
  vi.spyOn(api, "app").mockResolvedValue({ ...detail(true), folder_readable: false })
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  renderAt()
  expect(await screen.findByText(/versão 2 ·/)).toBeInTheDocument()
  expect(screen.getByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora.")
})

test("shows the history timeline", async () => {
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "history").mockResolvedValue({
    items: [{ type: "run", at: "2026-09-30T14:10:00Z", run: { id: 7, status: "done", person_name: "Henry", version_number: 2 } }],
    more: false, next_before: "",
  })
  renderAt()
  expect(await screen.findByRole("link", { name: /Concluída/ })).toHaveAttribute("href", "/execucoes/7")
})
