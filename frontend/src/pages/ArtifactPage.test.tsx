import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { ApiError, api, type ArtifactDetail } from "@/lib/api"
import * as auth from "@/lib/auth"
import { ALL_CAN, VIEW_ONLY, bia, henry } from "@/test/fixtures"
import { ARTIFACT_SANDBOX, ArtifactPage, LINK_SANDBOX } from "./ArtifactPage"

vi.mock("@/components/ficha/AccessSection", () => ({ AccessSection: () => <p>Acesso de teste</p> }))

const art = (over: Partial<ArtifactDetail> = {}): ArtifactDetail => ({
  id: 5, name: "Vendas", summary: "Por região", kind: "file", tags: ["vendas"], area_id: 1, area: "Comercial",
  owner_name: "Ana", restricted: false, version: 2, updated_at: "2026-11-03T10:00:00Z",
  favorite: false, owner_id: 2, backup_id: null, backup_name: "", origin: "", can: ALL_CAN, current: 2,
  versions: [
    { number: 2, kind: "file", url: "", file_name: "v2.html", size: 10, note: "Novo gráfico", author_name: "Ana", restored_from: null, created_at: "2026-11-03T10:00:00Z" },
    { number: 1, kind: "file", url: "", file_name: "v1.html", size: 10, note: "", author_name: "Ana", restored_from: null, created_at: "2026-11-02T10:00:00Z" },
  ],
  ...over,
})

function Where() {
  const l = useLocation()
  return <p data-testid="where">{l.pathname}</p>
}

beforeEach(() => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: henry, refresh: async () => {}, setPerson: () => {} })
})
afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/artifacts/5"]}>
      <Routes>
        <Route path="/artifacts/:id" element={<ArtifactPage />} />
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  )

test("renders file artifact in a sandboxed iframe", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art())
  mount()
  const frame = await screen.findByTitle("Vendas")
  expect(frame.getAttribute("sandbox")).toBe(ARTIFACT_SANDBOX)
  expect(frame.getAttribute("sandbox")).not.toContain("allow-same-origin")
  expect(frame.getAttribute("src")).toContain("/api/artifacts/5/content")
  expect(screen.getByRole("link", { name: "Baixar" }).getAttribute("href")).toContain("download=1")
  expect(screen.getByText("Acesso de teste")).toBeInTheDocument()
})

test("renders link artifact with Abrir in new tab", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(
    art({ kind: "link", versions: [{ number: 1, kind: "link", url: "https://claude.ai/artifact/abc", file_name: "", size: 0, note: "", author_name: "Ana", restored_from: null, created_at: "2026-11-02T10:00:00Z" }], current: 1, version: 1 }),
  )
  mount()
  const links = await screen.findAllByRole("link", { name: "Abrir" })
  expect(links[0]).toHaveAttribute("href", "https://claude.ai/artifact/abc")
  expect(links[0]).toHaveAttribute("target", "_blank")
  expect(links[0].getAttribute("rel")).toBe("noopener noreferrer")
  expect(screen.getByText("claude.ai")).toBeInTheDocument()
  expect(screen.queryByTitle("Vendas")).toBeNull()
})

const linkVersion = (url: string, embeddable: boolean) => ({ number: 1, kind: "link" as const, url, file_name: "", size: 0, note: "", author_name: "Ana", restored_from: null, created_at: "2026-11-02T10:00:00Z", embeddable })

test("an embeddable link is shown inside the page like a file", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art({ kind: "link", versions: [linkVersion("https://app.powerbi.com/view?r=abc", true)], current: 1, version: 1 }))
  mount()
  const frame = await screen.findByTitle("Vendas")
  expect(frame).toHaveAttribute("src", "https://app.powerbi.com/view?r=abc")
  expect(frame).toHaveAttribute("sandbox", LINK_SANDBOX)
  expect(frame).toHaveAttribute("referrerpolicy", "no-referrer")
  expect(screen.getByRole("link", { name: "Abrir em tela cheia" })).toHaveAttribute("href", "https://app.powerbi.com/view?r=abc")
  expect(screen.queryByText("Este artifact abre em outro site")).toBeNull()
})

test("a link to the Ziv itself is never framed, even if marked embeddable", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art({ kind: "link", versions: [linkVersion(`${window.location.origin}/apps`, true)], current: 1, version: 1 }))
  mount()
  await screen.findByText("Este artifact abre em outro site")
  expect(screen.queryByTitle("Vendas")).toBeNull()
})

test("a Claude link explains how to show it inside, only to who can edit", async () => {
  const claude = art({ kind: "link", versions: [linkVersion("https://claude.ai/public/artifacts/abc", false)], current: 1, version: 1 })
  vi.spyOn(api, "artifact").mockResolvedValue(claude)
  const { unmount } = mount()
  expect(await screen.findByText(/Links do Claude não aparecem dentro do Ziv/)).toBeInTheDocument()
  expect(screen.queryByTitle("Vendas")).toBeNull()
  unmount()
  vi.spyOn(api, "artifact").mockResolvedValue({ ...claude, can: VIEW_ONLY })
  mount()
  await screen.findByText("Este artifact abre em outro site")
  expect(screen.queryByText(/Links do Claude não aparecem dentro do Ziv/)).toBeNull()
})

test("a link that is not from Claude and not embeddable shows no hint", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art({ kind: "link", versions: [linkVersion("https://lookerstudio.google.com/x", false)], current: 1, version: 1 }))
  mount()
  await screen.findByText("Este artifact abre em outro site")
  expect(screen.queryByText(/Links do Claude/)).toBeNull()
})

test("hides edit, new version and restore without permission", async () => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: bia, refresh: async () => {}, setPerson: () => {} })
  vi.spyOn(api, "artifact").mockResolvedValue(art({ can: VIEW_ONLY }))
  mount()
  await screen.findByTitle("Vendas")
  expect(screen.queryByRole("button", { name: /Editar/ })).toBeNull()
  expect(screen.queryByRole("button", { name: "Enviar versão nova" })).toBeNull()
  expect(screen.queryByRole("button", { name: "Voltar para esta" })).toBeNull()
  expect(screen.queryByRole("button", { name: "Apagar artifact" })).toBeNull()
  expect(screen.queryByText("Acesso de teste")).toBeNull()
})

test("restores an older version", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art())
  const restore = vi.spyOn(api, "restoreArtifact").mockResolvedValue(art({ current: 3 }))
  const user = userEvent.setup()
  mount()
  await screen.findByTitle("Vendas")
  const buttons = screen.getAllByRole("button", { name: "Voltar para esta" })
  expect(buttons).toHaveLength(1)
  await user.click(buttons[0])
  await waitFor(() => expect(restore).toHaveBeenCalledWith(5, 1))
})

test("sends a new version with a note", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art())
  const send = vi.spyOn(api, "artifactVersion").mockResolvedValue(art())
  const user = userEvent.setup()
  mount()
  await screen.findByTitle("Vendas")
  await user.click(screen.getByRole("button", { name: "Enviar versão nova" }))
  await user.click(screen.getByRole("tab", { name: "Colar link" }))
  await user.type(screen.getByLabelText("Link"), "https://x.com/novo")
  await user.type(screen.getByLabelText("O que mudou?"), "Troquei as cores")
  await user.click(screen.getByRole("button", { name: "Enviar" }))
  await waitFor(() => expect(send).toHaveBeenCalledWith(5, { kind: "link", url: "https://x.com/novo", note: "Troquei as cores" }))
})

test("asks to confirm when secrets are found", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art())
  const send = vi
    .spyOn(api, "artifactVersion")
    .mockRejectedValueOnce(new ApiError(409, "x", { code: "artifact_secrets_confirmation_required", secret_count: 2 }))
    .mockResolvedValueOnce(art())
  const user = userEvent.setup()
  mount()
  await screen.findByTitle("Vendas")
  await user.click(screen.getByRole("button", { name: "Enviar versão nova" }))
  await user.click(screen.getByRole("tab", { name: "Colar link" }))
  await user.type(screen.getByLabelText("Link"), "https://x.com/novo")
  await user.click(screen.getByRole("button", { name: "Enviar" }))
  const dialog = await screen.findByRole("alertdialog")
  await user.click(within(dialog).getByRole("button", { name: "Enviar mesmo assim" }))
  await waitFor(() => expect(send).toHaveBeenLastCalledWith(5, expect.objectContaining({ confirm_secrets: true })))
})

test("deletes after confirmation and goes back to the list", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art())
  const del = vi.spyOn(api, "deleteArtifact").mockResolvedValue(undefined)
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(true)
  const user = userEvent.setup()
  mount()
  await screen.findByTitle("Vendas")
  await user.click(screen.getByRole("button", { name: "Apagar artifact" }))
  expect(confirm).toHaveBeenCalledWith('Apagar "Vendas"? Isso não pode ser desfeito.')
  await waitFor(() => expect(del).toHaveBeenCalledWith(5))
  expect(await screen.findByTestId("where")).toHaveTextContent("/artifacts")
})

test("shows not found", async () => {
  vi.spyOn(api, "artifact").mockRejectedValue(new ApiError(404, "nope"))
  mount()
  expect(await screen.findByText("Artifact não encontrado.")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /artifacts/i })).toHaveAttribute("href", "/artifacts")
})

test("the preview reloads after a new version or a restore", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art())
  vi.spyOn(api, "artifactVersion").mockResolvedValue(art({ current: 3 }))
  vi.spyOn(api, "restoreArtifact").mockResolvedValue(art({ current: 4 }))
  const user = userEvent.setup()
  mount()
  expect((await screen.findByTitle("Vendas")).getAttribute("src")).toContain("v=2")
  await user.click(screen.getByRole("button", { name: "Enviar versão nova" }))
  await user.click(screen.getByRole("tab", { name: "Colar link" }))
  await user.type(screen.getByLabelText("Link"), "https://x.com/novo")
  await user.click(screen.getByRole("button", { name: "Enviar" }))
  await waitFor(() => expect(screen.getByTitle("Vendas").getAttribute("src")).toContain("v=3"))
  await user.click(screen.getAllByRole("button", { name: "Voltar para esta" })[0])
  await waitFor(() => expect(screen.getByTitle("Vendas").getAttribute("src")).toContain("v=4"))
})

test("whoever takes care changes owner, backup and area", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art())
  vi.spyOn(api, "people").mockResolvedValue([
    { id: 2, name: "Ana", email: "ana@alis.com", role_id: 2, role_name: "Criador", area_ids: [] },
    { id: 3, name: "Bia", email: "bia@alis.com", role_id: 3, role_name: "Usuário", area_ids: [] },
  ])
  vi.spyOn(api, "areas").mockResolvedValue([
    { id: 1, name: "Comercial", app_count: 0, leaders: [], members: [] },
    { id: 4, name: "Investimentos", app_count: 0, leaders: [], members: [] },
  ])
  const save = vi.spyOn(api, "updateArtifact").mockResolvedValue(art({ owner_id: 3, owner_name: "Bia", backup_id: 2, backup_name: "Ana", area_id: 4, area: "Investimentos" }))
  const user = userEvent.setup()
  mount()
  await screen.findByTitle("Vendas")
  await user.click(screen.getByRole("button", { name: "Editar quem cuida" }))
  await waitFor(() => expect(screen.getByLabelText("Responsável")).toHaveValue("2"))
  await user.selectOptions(screen.getByLabelText("Responsável"), "3")
  await user.selectOptions(screen.getByLabelText("Reserva"), "2")
  await user.selectOptions(screen.getByLabelText("Área"), "4")
  await user.click(screen.getByRole("button", { name: "Salvar" }))
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith(5, { name: "Vendas", summary: "Por região", tags: ["vendas"], owner_id: 3, backup_id: 2, area_id: 4 }),
  )
  expect(await screen.findByText("Bia")).toBeInTheDocument()
})

test("removing the backup sends 0", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art({ backup_id: 3, backup_name: "Bia" }))
  vi.spyOn(api, "people").mockResolvedValue([
    { id: 2, name: "Ana", email: "ana@alis.com", role_id: 2, role_name: "Criador", area_ids: [] },
    { id: 3, name: "Bia", email: "bia@alis.com", role_id: 3, role_name: "Usuário", area_ids: [] },
  ])
  vi.spyOn(api, "areas").mockResolvedValue([{ id: 1, name: "Comercial", app_count: 0, leaders: [], members: [] }])
  const save = vi.spyOn(api, "updateArtifact").mockResolvedValue(art())
  const user = userEvent.setup()
  mount()
  await screen.findByTitle("Vendas")
  await user.click(screen.getByRole("button", { name: "Editar quem cuida" }))
  await waitFor(() => expect(screen.getByLabelText("Reserva")).toHaveValue("3"))
  await user.selectOptions(screen.getByLabelText("Reserva"), "0")
  await user.click(screen.getByRole("button", { name: "Salvar" }))
  await waitFor(() => expect(save).toHaveBeenCalledWith(5, expect.objectContaining({ owner_id: 2, backup_id: 0, area_id: 1 })))
})

test("without access the care editor is hidden, but the ficha editor stays", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art({ can: { ...ALL_CAN, access: false, grants: false } }))
  mount()
  await screen.findByTitle("Vendas")
  expect(screen.getByRole("button", { name: "Editar o artifact" })).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Editar quem cuida" })).toBeNull()
})

test("the title has a star", async () => {
  vi.spyOn(api, "artifact").mockResolvedValue(art({ favorite: true }))
  mount()
  expect(await screen.findByRole("button", { name: "Tirar Vendas dos favoritos" })).toHaveAttribute("aria-pressed", "true")
})
