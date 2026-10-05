import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, it, test, vi } from "vitest"
import { api, ApiError, type AppDetail, type Person } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { VIEW_ONLY, bia, detail, henry, run } from "@/test/fixtures"
import { AppPage } from "./AppPage"

beforeEach(() => localStorage.clear())
afterEach(() => vi.restoreAllMocks())

const renderPage = (person: Person = henry) => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person, refresh: async () => {}, setPerson: () => {} })
  return render(
    <MemoryRouter initialEntries={["/apps/3"]}>
      <RunsProvider>
        <Routes>
          <Route path="/apps/:id" element={<AppPage />} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )
}
const renderAt = () => renderPage()

const mockApp = (d: AppDetail) => {
  vi.spyOn(api, "app").mockResolvedValue(d)
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "appRuns").mockResolvedValue([])
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  vi.spyOn(api, "areas").mockResolvedValue([])
  vi.spyOn(api, "people").mockResolvedValue([])
  vi.spyOn(api, "apps").mockResolvedValue([])
  vi.spyOn(api, "appAccess").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [] })
}
const base = mockApp

test("ficha shows before the folder check answers", async () => {
  base(detail())
  vi.spyOn(api, "checkApp").mockReturnValue(new Promise(() => {})) // nunca responde
  renderAt()
  expect(await screen.findByRole("heading", { name: "Eq. Research Files Prep" })).toBeInTheDocument()
  expect(screen.getByText("versão 2")).toBeInTheDocument()
  expect(screen.queryByRole("alert")).not.toBeInTheDocument()
})

test("check finding a new version updates the field and reloads history", async () => {
  base(detail())
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
  base(detail())
  const check = vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: { number: 2, created_at: "2026-09-30T10:00:00Z" } })
  renderAt()
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  await vi.waitFor(() => expect(check).toHaveBeenCalled())
  await new Promise((r) => setTimeout(r, 50))
  expect(api.history).toHaveBeenCalledTimes(1)
})

test("unreadable folder warning appears after the check", async () => {
  base(detail())
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: false, version: null })
  renderAt()
  expect(await screen.findByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora.")
  expect(screen.getByText("versão 2")).toBeInTheDocument()
})

test("failed check keeps the ficha and shows the folder warning", async () => {
  base(detail())
  vi.spyOn(api, "checkApp").mockRejectedValue(new Error("boom"))
  renderAt()
  expect(await screen.findByRole("heading", { name: "Eq. Research Files Prep" })).toBeInTheDocument()
  expect(await screen.findByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora.")
  expect(screen.queryByText("boom")).not.toBeInTheDocument()
})

test("Rodar with a remembered folder shows 'Salva em' and runs directly; trocar asks", async () => {
  const user = userEvent.setup()
  base({ ...detail(), remembered_folder: "D:\\Saídas" })
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
  base(detail())
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
  base(detail({ can_run: false }))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  renderAt()
  expect(await screen.findByText("Eq. Research Files Prep")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Rodar" })).not.toBeInTheDocument()
})

test("shows the history timeline", async () => {
  vi.spyOn(api, "app").mockResolvedValue(detail())
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  vi.spyOn(api, "history").mockResolvedValue({
    items: [{ type: "run", at: "2026-09-30T14:10:00Z", run: { id: 7, status: "done", person_name: "Henry", version_number: 2 } }],
    more: false, next_before: "",
  })
  renderAt()
  expect(await screen.findByRole("link", { name: /Concluída/ })).toHaveAttribute("href", "/execucoes/7")
})

test("ficha says once where inputs come from, shows 3 notes and the rest on request", async () => {
  const user = userEvent.setup()
  const d = detail()
  d.manifest = { ...d.manifest, inputs: [
    { id: "a", label: "Nome da empresa", type: "text", source: "window" },
    { id: "b", label: "Anos", type: "number", source: "window" },
  ], notes: ["n1", "n2", "n3", "n4", "n5"] } as AppDetail["manifest"]
  base(d)
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  renderAt()
  expect(await screen.findByText("Você preenche tudo na janela do app:")).toBeInTheDocument()
  expect(screen.getByText("Nome da empresa")).toHaveTextContent(/^Nome da empresa$/)
  expect(screen.queryByText("n4")).not.toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Ver mais 2" }))
  expect(screen.getByText("n5")).toBeInTheDocument()
  expect(screen.getByText("Sem reserva")).toBeInTheDocument()
})

it("a colleague sees use and runs, no Editar and no technical block", async () => {
  mockApp(detail({ can: VIEW_ONLY }))
  renderPage(bia)
  expect(await screen.findByText("O que ele pede")).toBeInTheDocument()
  expect(screen.queryByText("Editar")).toBeNull()
  expect(screen.queryByText("Para quem cuida do app")).toBeNull()
  expect(screen.getByText("Suas execuções")).toBeInTheDocument()
})

it("saves one part and shows the new ficha", async () => {
  const d = detail()
  mockApp(d)
  const update = vi.spyOn(api, "updateApp").mockResolvedValue({ ...d, manifest: { ...d.manifest, outputs: [{ label: "Relatório", pattern: "*.pdf" }] }, updated_at: "2026-10-05T12:00:00Z" })
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  const label = screen.getByLabelText("Descrição do resultado")
  await userEvent.clear(label)
  await userEvent.type(label, "Relatório")
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(update).toHaveBeenCalledWith(d.id, expect.objectContaining({ updated_at: d.updated_at }))
  expect(await screen.findByText("Relatório")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Salvar" })).toBeNull()
})

it("keeps the part open and offers reload on a conflict", async () => {
  mockApp(detail())
  vi.spyOn(api, "updateApp").mockRejectedValue(new ApiError(409, "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova."))
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Recarregue")
  expect(screen.getByRole("button", { name: "Recarregar" })).toBeInTheDocument()
})

it("asks before discarding changes", async () => {
  mockApp(detail())
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false)
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  await userEvent.type(screen.getByLabelText("Descrição do resultado"), "x")
  await userEvent.click(screen.getByRole("button", { name: "Cancelar" }))
  expect(confirm).toHaveBeenCalled()
  expect(screen.getByRole("button", { name: "Salvar" })).toBeInTheDocument()
})

it("shows a lost app as not found", async () => {
  vi.spyOn(api, "app").mockRejectedValue(new ApiError(404, "App não encontrado."))
  renderPage(bia)
  expect(await screen.findByText("App não encontrado.")).toBeInTheDocument()
})

it("links other apps and marks the unknown ones", async () => {
  mockApp(detail({ depends_on_apps: [{ id: 7, slug: "files-prep", name: "Files Prep", visible: true }, { id: 0, slug: "sumiu", name: "sumiu", visible: false }], used_by: [] }))
  renderPage(henry)
  await userEvent.click(await screen.findByText("Para quem cuida do app"))
  expect(screen.getByRole("link", { name: "Files Prep" })).toHaveAttribute("href", "/apps/7")
  expect(screen.getByText(/sumiu · fora do catálogo/)).toBeInTheDocument()
})

it("asks before opening another part over unsaved changes", async () => {
  mockApp(detail())
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false)
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  await userEvent.type(screen.getByLabelText("Descrição do resultado"), "x")
  await userEvent.click(screen.getByRole("button", { name: "Editar O que ele pede" }))
  expect(confirm).toHaveBeenCalled()
  expect(screen.getByLabelText("Descrição do resultado")).toBeInTheDocument()
})

it("shows choice options in one line", async () => {
  const d = detail()
  d.manifest = { ...d.manifest, inputs: [{ id: "docs", label: "Documentos", type: "choice", source: "window", options: ["FRE", "ITR"], multiple: true, required: true }] }
  mockApp(d)
  renderPage(henry)
  expect(await screen.findByText(/FRE · ITR/)).toHaveTextContent("FRE · ITR (pode marcar mais de uma)")
})

it("lists the last runs with a link to each", async () => {
  mockApp(detail())
  vi.mocked(api.appRuns).mockResolvedValue([
    { id: 11, status: "done", person_name: "Bia", created_at: "2026-10-01T10:00:00Z", started_at: null, finished_at: null, file_count: 4 },
  ])
  renderPage(henry)
  expect(await screen.findByRole("heading", { name: "Execuções" })).toBeInTheDocument()
  expect(await screen.findByRole("link", { name: /4 arquivos/ })).toHaveAttribute("href", "/execucoes/11")
  expect(screen.getByText(/Bia/)).toBeInTheDocument()
})

it("remembers the technical block open per app", async () => {
  mockApp(detail())
  renderPage(henry)
  await userEvent.click(await screen.findByText("Para quem cuida do app"))
  await vi.waitFor(() => expect(localStorage.getItem("apphub-tech-open-3")).toBe("1"))
})
