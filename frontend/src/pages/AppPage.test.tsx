import { act, render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, it, test, vi } from "vitest"
import { api, ApiError, type AppDetail, type Person } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { VIEW_ONLY, activeRun, bia, detail, henry, manifest } from "@/test/fixtures"
import { toast } from "sonner"
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
  vi.spyOn(api.keys, "list").mockResolvedValue({ keys: [], can_manage: true, ready: true, missing: [], locked: false })
}
const base = mockApp

// The Cuidar face opens one section at a time from its menu (ADR 029).
const openCare = async (section: string) => {
  const nav = await screen.findByRole("navigation", { name: "Seções de cuidado" })
  await userEvent.click(within(nav).getByRole("button", { name: new RegExp(`^${section}`) }))
}
const openUse = async () => userEvent.click(await screen.findByRole("tab", { name: "Usar" }))

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
  let answer: (c: Awaited<ReturnType<typeof api.checkApp>>) => void = () => {}
  vi.spyOn(api, "checkApp").mockImplementation(() => new Promise((r) => { answer = r }))
  renderAt()
  expect(await screen.findByText("versão 2")).toBeInTheDocument()
  await openCare("Histórico")
  await vi.waitFor(() => expect(api.history).toHaveBeenCalledTimes(1))
  act(() => answer({ folder_readable: true, version: { number: 3, created_at: "2026-10-01T15:00:00Z" } }))
  expect(await screen.findByText("versão 3")).toBeInTheDocument()
  await vi.waitFor(() => expect(api.history).toHaveBeenCalledTimes(2))
})

test("check with same version does not reload history", async () => {
  base(detail())
  const check = vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: { number: 2, created_at: "2026-09-30T10:00:00Z" } })
  renderAt()
  await openCare("Histórico")
  await vi.waitFor(() => expect(check).toHaveBeenCalled())
  await new Promise((r) => setTimeout(r, 50))
  expect(api.history).toHaveBeenCalledTimes(1)
})

test("unreadable folder: a pendência for who takes care, a warning for who uses", async () => {
  base(detail())
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: false, version: null })
  const { unmount } = renderAt()
  expect(await screen.findByText("Não consegui ler a pasta do app")).toBeInTheDocument()
  expect(screen.getByText("versão 2")).toBeInTheDocument()
  unmount()
  base(detail({ can: VIEW_ONLY }))
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: false, version: null })
  renderPage(bia)
  expect(await screen.findByRole("alert")).toHaveTextContent("Não consegui ler a pasta do app agora.")
})

test("failed check keeps the ficha and shows the folder warning", async () => {
  base(detail())
  vi.spyOn(api, "checkApp").mockRejectedValue(new Error("boom"))
  renderAt()
  expect(await screen.findByRole("heading", { name: "Eq. Research Files Prep" })).toBeInTheDocument()
  expect(await screen.findByText("Não consegui ler a pasta do app")).toBeInTheDocument()
  expect(screen.queryByText("boom")).not.toBeInTheDocument()
})

test("Rodar with a remembered folder shows 'Salva em' and runs directly; trocar asks", async () => {
  const user = userEvent.setup()
  base({ ...detail(), remembered_folder: "D:\\Saídas" })
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  const start = vi.spyOn(api, "startRun").mockResolvedValue(activeRun({ status: "waiting" }))
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
  vi.spyOn(api, "startRun").mockResolvedValue(activeRun({ status: "waiting" }))
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

test("a script that can run offers Agendar and lists its schedules; a desktop app does not", async () => {
  const user = userEvent.setup()
  base(detail({ manifest: manifest({ kind: "script", name: "Eq. Research Files Prep" }) }))
  vi.spyOn(api, "appSchedules").mockResolvedValue([])
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "1", agent_available: true })
  const { unmount } = renderAt()
  await user.click(await screen.findByRole("button", { name: "Agendar" }))
  expect(await screen.findByRole("heading", { name: "Agendar Eq. Research Files Prep" })).toBeInTheDocument()
  unmount()
  base(detail())
  renderAt()
  expect(await screen.findByRole("button", { name: "Rodar" })).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Agendar" })).not.toBeInTheDocument()
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
  await openCare("Histórico")
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
  await openCare("Ficha")
  expect(await screen.findByText("Você preenche tudo na janela do app:")).toBeInTheDocument()
  expect(screen.getByText("Nome da empresa")).toHaveTextContent(/^Nome da empresa$/)
  await openCare("Código")
  expect(screen.queryByText("n4")).not.toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Ver mais 2" }))
  expect(screen.getByText("n5")).toBeInTheDocument()
})

it("a colleague sees use and runs, no Editar and no technical block", async () => {
  mockApp(detail({ can: VIEW_ONLY }))
  renderPage(bia)
  expect(await screen.findByText("O que ele pede")).toBeInTheDocument()
  expect(screen.queryByText("Editar")).toBeNull()
  expect(screen.queryByRole("tab", { name: /Cuidar/ })).toBeNull()
  expect(screen.queryByRole("navigation", { name: "Seções de cuidado" })).toBeNull()
  expect(screen.getByText("Suas execuções")).toBeInTheDocument()
})

it("saves one part and shows the new ficha", async () => {
  const d = detail()
  mockApp(d)
  const update = vi.spyOn(api, "updateApp").mockResolvedValue({ ...d, manifest: { ...d.manifest, outputs: [{ label: "Relatório", pattern: "*.pdf" }] }, updated_at: "2026-10-05T12:00:00Z" })
  renderPage(henry)
  await openCare("Ficha")
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
  await openCare("Ficha")
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Recarregue")
  expect(screen.getByRole("button", { name: "Recarregar" })).toBeInTheDocument()
})

it("asks before discarding changes", async () => {
  mockApp(detail())
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false)
  renderPage(henry)
  await openCare("Ficha")
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
  await openCare("Código")
  expect(screen.getByRole("link", { name: "Files Prep" })).toHaveAttribute("href", "/apps/7")
  expect(screen.getByText(/sumiu · fora do catálogo/)).toBeInTheDocument()
})

it("asks before opening another part over unsaved changes", async () => {
  mockApp(detail())
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false)
  renderPage(henry)
  await openCare("Ficha")
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
  await openUse()
  expect(await screen.findByText(/FRE · ITR/)).toHaveTextContent("FRE · ITR (pode marcar mais de uma)")
})

it("lists the last runs with a link to each", async () => {
  mockApp(detail())
  vi.mocked(api.appRuns).mockResolvedValue([
    { id: 11, status: "done", person_name: "Bia", created_at: "2026-10-01T10:00:00Z", started_at: null, finished_at: null, file_count: 4 },
  ])
  renderPage(henry)
  await openUse()
  expect(await screen.findByRole("heading", { name: "Execuções" })).toBeInTheDocument()
  expect(await screen.findByRole("link", { name: /4 arquivos/ })).toHaveAttribute("href", "/execucoes/11")
  expect(screen.getByText(/Bia/)).toBeInTheDocument()
})

it("who takes care starts on Cuidar with the pendências and can switch to Usar", async () => {
  mockApp(detail())
  renderPage(henry)
  expect(await screen.findByRole("tab", { name: /Cuidar/ })).toHaveAttribute("aria-selected", "true")
  expect(screen.getByRole("list", { name: "Etapas do app" })).toHaveTextContent("Com reserva")
  expect(screen.getByText("Escolher uma reserva")).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Execuções" })).toBeNull()
  await openUse()
  expect(screen.getByRole("heading", { name: "O que ele entrega" })).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: /^Editar O que/ })).toBeNull()
})

it("Escolher in the pendências opens Quem cuida for editing in the Ficha", async () => {
  mockApp(detail())
  renderPage(henry)
  await userEvent.click(await screen.findByRole("button", { name: "Escolher" }))
  expect(within(screen.getByRole("region", { name: "Ficha" })).getByRole("button", { name: "Salvar" })).toBeInTheDocument()
})

it("an app waiting approval counts as a pendência and is approved there", async () => {
  const d = detail({ backup_id: 2, manifest: { ...detail().manifest, backup: { id: 2, name: "Bia", email: "bia@x.com" }, summary: "Faz algo." } as AppDetail["manifest"], control: { require_approval: true, approved: false, suspended: false, can_manage: true } as AppDetail["control"] })
  mockApp(d)
  const set = vi.spyOn(api, "setAppControl").mockResolvedValue({ ...d.control!, approved: true })
  renderPage(henry)
  expect(await screen.findByRole("tab", { name: /^Cuidar.*1 pendência$/ })).toBeInTheDocument()
  expect(screen.getByRole("list", { name: "Etapas do app" })).toHaveTextContent("esperando você")
  await userEvent.click(screen.getByRole("button", { name: "Aprovar" }))
  expect(set).toHaveBeenCalledWith(3, { approved: true, suspended: false })
  expect(await screen.findByText("Tudo em dia. Nada pendente neste app.")).toBeInTheDocument()
})

it("saves with the version the edit started from, so a change in between gets the conflict", async () => {
  const d = detail()
  mockApp(d)
  vi.mocked(api.app).mockResolvedValueOnce(d).mockResolvedValue({ ...d, updated_at: "2026-10-05T09:00:00Z" })
  vi.mocked(api.history).mockResolvedValue({
    items: [{ type: "version", at: "2026-09-30T10:00:00Z", version: {
      number: 2, origin: "folder", channel: "manual", restored_from: null, author_name: "Ana", author_probable: false, description: "",
      summary: { added: 0, removed: 0, modified: 1, binaries: [] }, files_changed_at: null, created_at: "2026-09-30T10:00:00Z", current: true,
    } }],
    more: false, next_before: "",
  })
  vi.spyOn(api, "describeVersion").mockResolvedValue({} as never)
  const update = vi.spyOn(api, "updateApp").mockRejectedValue(new ApiError(409, "Alguém mudou esta ficha agora há pouco. Recarregue para ver a versão nova."))
  renderPage(henry)
  await openCare("Ficha")
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  // While the part is open, the version gets a description in the history: the app reloads.
  await openCare("Histórico")
  await userEvent.click(await screen.findByRole("button", { name: "Descrever" }))
  await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Salvar" }))
  await vi.waitFor(() => expect(api.app).toHaveBeenCalledTimes(2))
  await vi.waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
  await openCare("Ficha")
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(update).toHaveBeenCalledWith(d.id, expect.objectContaining({ updated_at: d.updated_at }))
  expect(await screen.findByRole("button", { name: "Recarregar" })).toBeInTheDocument()
})

it("shows Recarregar only for a conflict or a lost app, not for something missing", async () => {
  const d = detail()
  d.manifest = { ...d.manifest, inputs: [{ id: "docs", label: "Documentos", type: "choice", source: "window", options: [], required: true }] }
  mockApp(d)
  const update = vi.spyOn(api, "updateApp")
  renderPage(henry)
  await openCare("Ficha")
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele pede" }))
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(await screen.findByRole("alert")).toHaveTextContent('Falta definir as opções de "Documentos".')
  expect(screen.queryByRole("button", { name: "Recarregar" })).toBeNull()
  expect(update).not.toHaveBeenCalled()
})

it("an app lost while open: save says not found, Recarregar shows it and offers to try again", async () => {
  mockApp(detail())
  vi.spyOn(api, "updateApp").mockRejectedValue(new ApiError(404, "App não encontrado."))
  renderPage(bia)
  await openCare("Ficha")
  await userEvent.click(await screen.findByRole("button", { name: "Editar O que ele entrega" }))
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("App não encontrado.")
  vi.mocked(api.app).mockRejectedValue(new ApiError(404, "App não encontrado."))
  await userEvent.click(screen.getByRole("button", { name: "Recarregar" }))
  expect(await screen.findByText("App não encontrado.")).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Eq. Research Files Prep" })).toBeNull()
})

it("tries the editor lists again after they failed", async () => {
  mockApp(detail())
  vi.mocked(api.people).mockRejectedValueOnce(new ApiError(0, "offline")).mockResolvedValue([])
  renderPage(henry)
  await openCare("Ficha")
  await userEvent.click(await screen.findByRole("button", { name: "Editar Quem cuida" }))
  await vi.waitFor(() => expect(api.people).toHaveBeenCalledTimes(1))
  await userEvent.click(screen.getByRole("button", { name: "Cancelar" }))
  await userEvent.click(screen.getByRole("button", { name: "Editar Quem cuida" }))
  await vi.waitFor(() => expect(api.people).toHaveBeenCalledTimes(2))
  expect(api.areas).toHaveBeenCalledTimes(2)
})

it("following another app's link loads that app fresh", async () => {
  const other = detail({ id: 7, manifest: { ...detail().manifest, name: "Files Prep" } })
  mockApp(detail({ depends_on_apps: [{ id: 7, slug: "files-prep", name: "Files Prep", visible: true }] }))
  let answer: (d: AppDetail) => void = () => {}
  renderPage(henry)
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  vi.mocked(api.app).mockImplementation(() => new Promise((r) => { answer = r }))
  await openCare("Código")
  await userEvent.click(screen.getByRole("link", { name: "Files Prep" }))
  expect(await screen.findByText("Carregando…")).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "Eq. Research Files Prep" })).toBeNull()
  act(() => answer(other))
  expect(await screen.findByRole("heading", { name: "Files Prep" })).toBeInTheDocument()
})

it("an editor without access control cannot change who cares for the app nor its area", async () => {
  mockApp(detail({ can: { ...VIEW_ONLY, edit: true, tech: true } }))
  vi.mocked(api.areas).mockResolvedValue([{ id: 1, name: "Research", app_count: 1, leaders: [], members: [] }])
  renderPage(bia)
  await openCare("Ficha")
  expect(screen.getByRole("heading", { name: "Quem cuida" })).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Editar Quem cuida" })).toBeNull()
  await userEvent.click(screen.getByRole("button", { name: "Editar Sobre" }))
  const sobre = screen.getByRole("group", { name: "Sobre" })
  expect(within(sobre).queryByLabelText(/Área/)).toBeNull()
  expect(within(sobre).getByText("Só quem cuida do acesso do app troca a área.")).toBeInTheDocument()
})

it("someone who cares for the app's access changes who cares and the area", async () => {
  mockApp(detail())
  renderPage(henry)
  await openCare("Ficha")
  expect(await screen.findByRole("button", { name: "Editar Quem cuida" })).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Editar Sobre" }))
  expect(within(screen.getByRole("group", { name: "Sobre" })).getByLabelText(/Área/)).toBeInTheDocument()
})

it("offers Tornar compartilhado on a web app to who takes care, not to others", async () => {
  const web = detail({ manifest: { ...detail().manifest, kind: "web" } })
  base(web)
  const { unmount } = renderPage()
  await openCare("Servidor")
  expect(await screen.findByRole("button", { name: "Tornar compartilhado" })).toBeInTheDocument()
  expect(screen.getByText("Fica num endereço só, para várias pessoas usarem ao mesmo tempo, com os dados guardados no Ziv.")).toBeInTheDocument()
  unmount()
  base({ ...web, can: { ...web.can, restore: false } })
  renderPage(bia)
  expect(await screen.findByRole("heading", { name: "Eq. Research Files Prep" })).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Tornar compartilhado" })).not.toBeInTheDocument()
})

it("Tornar compartilhado saves the defaults and shows the shared part", async () => {
  const user = userEvent.setup()
  const web = detail({ manifest: { ...detail().manifest, kind: "web" } })
  base(web)
  const shared = { ...web, shared: { status: "draft" as const, url: "", slug: "fp" } }
  const save = vi.spyOn(api.shared, "save").mockResolvedValue(shared)
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: { status: "draft", status_message: "", stopped_by: "", always_on: false, idle_minutes: 30, memory_mb: 512, cpus: 1, wants_db: false, url: "", acks: [], has_previous: false }, deployments: [] })
  const check = vi.spyOn(api.shared, "check").mockResolvedValue({ ready: true, recipe: { kind: "flask", command: "", dockerfile: "", port: 0, needs_confirm: false, requirements: null }, wants_db: false, suggest_env: [], items: [] })
  vi.spyOn(api.shared, "config").mockResolvedValue([])
  renderPage()
  await openCare("Servidor")
  await user.click(await screen.findByRole("button", { name: "Tornar compartilhado" }))
  expect(save).toHaveBeenCalledWith(3, expect.objectContaining({ enabled: true, memory_mb: 512, cpus: 1, idle_minutes: 30, database: "", updated_at: web.updated_at }))
  expect(await screen.findByText("Tudo certo para publicar.")).toBeInTheDocument()
  expect(check).toHaveBeenCalled()
})

it("a published shared app is opened, not run", async () => {
  base(detail({ manifest: { ...detail().manifest, kind: "web" }, shared: { status: "running", url: "http://fp.localhost", slug: "fp" }, can: VIEW_ONLY }))
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: { status: "running", status_message: "", stopped_by: "", always_on: false, idle_minutes: 30, memory_mb: 512, cpus: 1, wants_db: false, url: "", acks: [], has_previous: false }, live_version: 2 })
  renderPage(bia)
  expect(await screen.findByText("No ar · versão 2")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Rodar" })).not.toBeInTheDocument()
  expect(screen.getAllByRole("link", { name: /Abrir/ })[0]).toHaveAttribute("href", "/api/shared/authorize?app=fp&volta=%2F")
})

it("says what to do when the ficha does not reload after a shared change", async () => {
  const user = userEvent.setup()
  const shared = detail({ manifest: { ...detail().manifest, kind: "web" }, shared: { status: "running", url: "", slug: "fp" } })
  base(shared)
  vi.spyOn(api, "app").mockResolvedValueOnce(shared).mockRejectedValue(new Error("boom"))
  vi.spyOn(api.shared, "get").mockResolvedValue({ shared: { status: "running", status_message: "", stopped_by: "", always_on: false, idle_minutes: 30, memory_mb: 512, cpus: 1, wants_db: false, url: "", acks: [], has_previous: false }, live_version: 2, latest_version: 2, deployments: [] })
  vi.spyOn(api.shared, "check").mockResolvedValue({ ready: true, recipe: { kind: "flask", command: "", dockerfile: "", port: 0, needs_confirm: false, requirements: null }, wants_db: false, suggest_env: [], items: [] })
  vi.spyOn(api.shared, "config").mockResolvedValue([])
  vi.spyOn(api.shared, "stop").mockResolvedValue({})
  const err = vi.spyOn(toast, "error")
  renderPage()
  await openCare("Servidor")
  await user.click(await screen.findByRole("button", { name: "Parar" }))
  await user.click(screen.getByRole("button", { name: "Sim, parar" }))
  await vi.waitFor(() => expect(err).toHaveBeenCalledWith("Não consegui atualizar a ficha. Recarregue a página para ver o estado novo."))
})

test("the back link goes to /apps and the title has a star", async () => {
  base(detail({ favorite: true }))
  renderAt()
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  expect(screen.getByRole("link", { name: /Apps/ })).toHaveAttribute("href", "/apps")
  expect(screen.getByRole("button", { name: "Tirar Eq. Research Files Prep dos favoritos" })).toHaveAttribute("aria-pressed", "true")
})

test("a screen app has the Cofre and opens on the computer", async () => {
  base(detail({ manifest: manifest({ kind: "web", name: "Notícias" }) }))
  renderAt()
  expect(await screen.findByRole("button", { name: "Abrir no computador" })).toBeInTheDocument()
  await openCare("Cofre")
  expect(await screen.findByRole("heading", { name: "Cofre" })).toBeInTheDocument()
  expect(screen.getByText(/no lugar do .env/)).toBeInTheDocument()
})

test("a screen app on the server opens there, and also on the computer", async () => {
  base(detail({ manifest: manifest({ kind: "web", name: "Notícias" }), shared: { status: "running", url: "http://noticias.localhost", slug: "noticias" } as AppDetail["shared"] }))
  vi.spyOn(api.shared, "get").mockReturnValue(new Promise(() => {})) // the Compartilhar part is tested elsewhere
  renderAt()
  expect((await screen.findAllByRole("link", { name: /Abrir/ })).length).toBeGreaterThan(0)
  expect(screen.getByRole("button", { name: "No meu computador" })).toBeInTheDocument()
})
