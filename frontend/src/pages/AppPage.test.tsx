import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type AppDetail } from "@/lib/api"
import { RunsProvider } from "@/lib/runs-context"
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
      <RunsProvider>
        <Routes>
          <Route path="/apps/:id" element={<AppPage />} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )

const base = (d: AppDetail) => {
  vi.spyOn(api, "app").mockResolvedValue(d)
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
}

test("ficha shows before the folder check answers", async () => {
  base(detail(true))
  vi.spyOn(api, "checkApp").mockReturnValue(new Promise(() => {})) // nunca responde
  renderAt()
  expect(await screen.findByRole("heading", { name: "Eq. Research Files Prep" })).toBeInTheDocument()
  expect(screen.getByText("versão 2")).toBeInTheDocument()
  expect(screen.queryByRole("alert")).not.toBeInTheDocument()
})

test("check finding a new version updates the field and reloads history", async () => {
  base(detail(true))
  // The check answers after the ficha is on screen (OneDrive is slow).
  vi.spyOn(api, "checkApp").mockImplementation(
    () => new Promise((r) => setTimeout(() => r({ folder_readable: true, version: { number: 3, created_at: "2026-10-01T15:00:00Z" } }), 50)),
  )
  renderAt()
  expect(await screen.findByText("versão 2")).toBeInTheDocument()
  expect(await screen.findByText("versão 3")).toBeInTheDocument()
  await vi.waitFor(() => expect(api.history).toHaveBeenCalledTimes(2))
})

test("check with same version does not reload history", async () => {
  base(detail(true))
  const check = vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: { number: 2, created_at: "2026-09-30T10:00:00Z" } })
  renderAt()
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  await vi.waitFor(() => expect(check).toHaveBeenCalled())
  await new Promise((r) => setTimeout(r, 50))
  expect(api.history).toHaveBeenCalledTimes(1)
})

test("unreadable folder warning appears after the check", async () => {
  base(detail(true))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: false, version: null })
  renderAt()
  expect(await screen.findByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora.")
  expect(screen.getByText("versão 2")).toBeInTheDocument()
})

test("failed check keeps the ficha and shows the folder warning", async () => {
  base(detail(true))
  vi.spyOn(api, "checkApp").mockRejectedValue(new Error("boom"))
  renderAt()
  expect(await screen.findByRole("heading", { name: "Eq. Research Files Prep" })).toBeInTheDocument()
  expect(await screen.findByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora.")
  expect(screen.queryByText("boom")).not.toBeInTheDocument()
})

test("Rodar with a remembered folder shows 'Salva em' and runs directly; trocar asks", async () => {
  const user = userEvent.setup()
  base({ ...detail(true), remembered_folder: "D:\\Saídas" })
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  const start = vi.spyOn(api, "startRun").mockResolvedValue(run({ status: "waiting" }))
  renderAt()
  expect(await screen.findByText("D:\\Saídas")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Rodar" }))
  expect(start).toHaveBeenCalledWith(3, "D:\\Saídas", undefined)
  await user.click(screen.getByRole("button", { name: "trocar" }))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("D:\\Saídas")
})

test("Rodar without a remembered folder asks, and then shows 'Salva em'", async () => {
  const user = userEvent.setup()
  base(detail(true))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  vi.spyOn(api, "startRun").mockResolvedValue(run({ status: "waiting" }))
  renderAt()
  await user.click(await screen.findByRole("button", { name: "Rodar" }))
  expect(screen.queryByText(/Salva em/)).not.toBeInTheDocument()
  await user.type(screen.getByLabelText("Onde salvar os resultados?"), "E:\\Resultados")
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(await screen.findByText(/Salva em/)).toHaveTextContent("Salva em E:\\Resultados · trocar")
})

test("apps that cannot run have no Rodar button", async () => {
  base(detail(false))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  renderAt()
  expect(await screen.findByText("Eq. Research Files Prep")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Rodar" })).not.toBeInTheDocument()
})

test("shows the history timeline", async () => {
  vi.spyOn(api, "app").mockResolvedValue(detail(true))
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  vi.spyOn(api, "history").mockResolvedValue({
    items: [{ type: "run", at: "2026-09-30T14:10:00Z", run: { id: 7, status: "done", person_name: "Henry", version_number: 2 } }],
    more: false, next_before: "",
  })
  renderAt()
  expect(await screen.findByRole("link", { name: /Concluída/ })).toHaveAttribute("href", "/execucoes/7")
})
