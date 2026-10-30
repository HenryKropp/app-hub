import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { ApiError, api, type Person, type PersonRow } from "@/lib/api"
import * as auth from "@/lib/auth"
import { PERMISSIONS } from "@/lib/permissions"
import { bia, henry } from "@/test/fixtures"
import { TeamPage } from "./TeamPage"

vi.mock("sonner", () => ({ toast: { success: vi.fn() } }))

const renderAt = (path: string, person: Person, refresh: () => Promise<void> = async () => {}) => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person, refresh, setPerson: () => {} })
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/equipe" element={<TeamPage />} />
        <Route path="/" element={<p>Catálogo</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe("TeamPage", () => {
  beforeEach(() => {
    vi.spyOn(api, "people").mockResolvedValue([
      { id: 1, name: "Henry", email: "henry@alis.com", role_id: 1, role_name: "Administrador", area_ids: [] },
      { id: 3, name: "Bia", email: "bia@alis.com", role_id: 3, role_name: "Usuário", area_ids: [2] },
    ])
    vi.spyOn(api, "roles").mockResolvedValue([
      { id: 1, key: "admin", name: "Administrador", permissions: PERMISSIONS.map((p) => p.key), people_count: 1 },
      { id: 3, key: "usuario", name: "Usuário", permissions: [], people_count: 1 },
      { id: 9, key: "", name: "Sub-admin", permissions: ["see_all_runs"], people_count: 0 },
    ])
    vi.spyOn(api, "areas").mockResolvedValue([{ id: 2, name: "Investimentos", app_count: 0, leaders: [], members: [{ id: 3, name: "Bia" }] }])
    vi.spyOn(api, "membership").mockResolvedValue({ invitations: [], requests: [] })
  })
  afterEach(() => vi.restoreAllMocks())

  it("shows the three tabs to an admin", async () => {
    renderAt("/equipe", henry)
    for (const t of ["Pessoas", "Áreas", "Cargos", "Acessos"]) expect(await screen.findByRole("tab", { name: t })).toBeInTheDocument()
  })

  it("approving an access request creates a one-use invite link instead of a membership", async () => {
    vi.spyOn(api, "membership").mockResolvedValueOnce({
      invitations: [],
      requests: [{ id: 12, name: "Celia", email: "celia@example.com", created_at: "2026-05-01T12:00:00Z" }],
    }).mockResolvedValue({
      invitations: [{ id: 7, email: "celia@example.com", role_id: 3, role_name: "Usuário", created_at: "2026-05-01T12:00:00Z", expires_at: "2026-05-08T12:00:00Z" }],
      requests: [],
    })
    const approve = vi.spyOn(api, "approveMembershipRequest").mockResolvedValue({ url: "/w/alis/convite#token=one-use" })
    renderAt("/equipe?aba=acessos", henry)
    await userEvent.click(await screen.findByRole("button", { name: "Aprovar" }))
    expect(approve).toHaveBeenCalledWith(12, 3)
    expect(await screen.findByLabelText("Endereço do convite")).toHaveValue(`${window.location.origin}/w/alis/convite#token=one-use`)
    expect(await screen.findByText("Nenhuma solicitação aguardando análise")).toBeInTheDocument()
    expect(await screen.findByRole("button", { name: "Revogar convite de celia@example.com" })).toBeInTheDocument()
  })

  it("creates an invitation for the selected role without choosing a person's password", async () => {
    const create = vi.spyOn(api, "createMembershipInvitation").mockResolvedValue({ url: "/convite#token=invite-key" })
    renderAt("/equipe?aba=acessos", henry)
    await userEvent.type(await screen.findByLabelText("E-mail da pessoa"), "new@example.com")
    await userEvent.click(screen.getByRole("button", { name: "Criar convite" }))
    expect(create).toHaveBeenCalledWith({ email: "new@example.com", role_id: 3 })
    expect(await screen.findByLabelText("Endereço do convite")).toHaveValue(`${window.location.origin}/convite#token=invite-key`)
    expect(screen.queryByLabelText("Senha inicial")).not.toBeInTheDocument()
  })

  it("revokes an invitation and removes it from the active list", async () => {
    vi.spyOn(api, "membership").mockResolvedValue({
      invitations: [{ id: 7, email: "celia@example.com", role_id: 3, role_name: "Usuário", created_at: "2026-05-01T12:00:00Z", expires_at: "2026-05-08T12:00:00Z" }],
      requests: [],
    })
    vi.spyOn(window, "confirm").mockReturnValue(true)
    const revoke = vi.spyOn(api, "revokeMembershipInvitation").mockResolvedValue({ ok: true })
    renderAt("/equipe?aba=acessos", henry)
    await userEvent.click(await screen.findByRole("button", { name: "Revogar convite de celia@example.com" }))
    expect(revoke).toHaveBeenCalledWith(7)
    expect(await screen.findByText("Sem convites ativos")).toBeInTheDocument()
  })

  it("rejects an access request without issuing an invitation", async () => {
    vi.spyOn(api, "membership").mockResolvedValue({
      invitations: [],
      requests: [{ id: 12, name: "Celia", email: "celia@example.com", created_at: "2026-05-01T12:00:00Z" }],
    })
    vi.spyOn(window, "confirm").mockReturnValue(true)
    const reject = vi.spyOn(api, "rejectMembershipRequest").mockResolvedValue({ ok: true })
    const invite = vi.spyOn(api, "createMembershipInvitation")
    renderAt("/equipe?aba=acessos", henry)
    await userEvent.click(await screen.findByRole("button", { name: "Recusar solicitação de Celia" }))
    expect(reject).toHaveBeenCalledWith(12)
    expect(invite).not.toHaveBeenCalled()
    expect(await screen.findByText("Nenhuma solicitação aguardando análise")).toBeInTheDocument()
  })

  it("a leader sees only Áreas and can set members of their area", async () => {
    const set = vi.spyOn(api, "setAreaMembers").mockResolvedValue()
    renderAt("/equipe", { ...bia, leads: [2] })
    expect(await screen.findByRole("tab", { name: "Áreas" })).toBeInTheDocument()
    expect(screen.queryByRole("tab", { name: "Cargos" })).toBeNull()
    expect(screen.queryByRole("button", { name: "Renomear Investimentos" })).toBeNull()
    await userEvent.click(await screen.findByRole("button", { name: "Membros de Investimentos" }))
    await userEvent.click(screen.getByLabelText("Henry"))
    await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
    expect(set).toHaveBeenCalledWith(2, [3, 1])
  })

  it("puts the role back when the Hub refuses the change", async () => {
    vi.spyOn(api, "setPersonRole").mockRejectedValue(new ApiError(409, "A empresa precisa de ao menos uma pessoa que cuide das pessoas e dos cargos."))
    renderAt("/equipe", henry)
    const select = await screen.findByLabelText("Cargo de Henry")
    await userEvent.selectOptions(select, "3")
    expect(await screen.findByRole("alert")).toHaveTextContent("ao menos uma pessoa")
    expect(select).toHaveValue("1")
  })

  it("refreshes the session after changing one's own role", async () => {
    const refresh = vi.fn(async () => {})
    vi.spyOn(api, "setPersonRole").mockResolvedValue({ id: 1, name: "Henry", email: "henry@alis.com", role_id: 3, role_name: "Usuário", area_ids: [] })
    renderAt("/equipe", henry, refresh)
    await userEvent.selectOptions(await screen.findByLabelText("Cargo de Henry"), "3")
    await vi.waitFor(() => expect(refresh).toHaveBeenCalled())
  })

  it("edits a role's permissions with plain names", async () => {
    const upd = vi.spyOn(api, "updateRole").mockResolvedValue({ id: 9, key: "", name: "Sub-admin", permissions: ["see_all_runs", "edit_any_app"], people_count: 0 })
    renderAt("/equipe?aba=cargos", henry)
    await userEvent.click(await screen.findByRole("button", { name: "Editar Sub-admin" }))
    await userEvent.click(screen.getByLabelText("Editar qualquer app"))
    await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
    expect(upd).toHaveBeenCalledWith(9, { name: "Sub-admin", permissions: ["see_all_runs", "edit_any_app"] })
    expect(screen.getByRole("button", { name: "Apagar Sub-admin" })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Apagar Administrador" })).toBeNull()
  })

  it("shows the error and a retry when roles fail to load", async () => {
    vi.spyOn(api, "roles").mockRejectedValue(new ApiError(500, "O Ziv não respondeu."))
    renderAt("/equipe", henry)
    expect(await screen.findByRole("alert")).toHaveTextContent("O Ziv não respondeu.")
    expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeInTheDocument()
  })

  it("sends someone who cannot see the team back to the catalog", async () => {
    renderAt("/equipe", bia)
    expect(await screen.findByText("Catálogo")).toBeInTheDocument()
    expect(screen.queryByRole("tab")).toBeNull()
  })

  it("a people manager with a smaller role sees bigger roles as text and offers only roles within theirs", async () => {
    const rh: Person = { ...bia, role_id: 9, role_name: "RH", permissions: ["manage_people"] }
    vi.spyOn(api, "roles").mockResolvedValue([
      { id: 1, key: "admin", name: "Administrador", permissions: PERMISSIONS.map((p) => p.key), people_count: 1 },
      { id: 3, key: "usuario", name: "Usuário", permissions: [], people_count: 1 },
      { id: 9, key: "", name: "RH", permissions: ["manage_people"], people_count: 1 },
    ])
    renderAt("/equipe", rh)
    const biaSelect = await screen.findByLabelText("Cargo de Bia")
    expect([...(biaSelect as HTMLSelectElement).options].map((o) => o.text)).toEqual(["Usuário", "RH"])
    expect(screen.queryByLabelText("Cargo de Henry")).toBeNull()
    expect(screen.getByText("Administrador")).toBeInTheDocument()
    const formSelect = screen.getByLabelText("Cargo") as HTMLSelectElement
    expect([...formSelect.options].map((o) => o.text)).toEqual(["Usuário", "RH"])
  })

  it("disables the role while saving, then reloads the role counts", async () => {
    let done: (v: PersonRow) => void = () => {}
    vi.spyOn(api, "setPersonRole").mockReturnValue(new Promise<PersonRow>((r) => (done = r)))
    renderAt("/equipe", henry)
    const select = await screen.findByLabelText("Cargo de Bia")
    await userEvent.selectOptions(select, "9")
    expect(select).toBeDisabled()
    const roles = vi.mocked(api.roles)
    const before = roles.mock.calls.length
    done({ id: 3, name: "Bia", email: "bia@alis.com", role_id: 9, role_name: "Sub-admin", area_ids: [2] })
    await vi.waitFor(() => expect(select).not.toBeDisabled())
    expect(roles.mock.calls.length).toBe(before + 1)
  })

  it("does not call a saved role change failed when the refresh fails", async () => {
    vi.spyOn(api, "setPersonRole").mockResolvedValue({ id: 1, name: "Henry", email: "henry@alis.com", role_id: 3, role_name: "Usuário", area_ids: [] })
    renderAt("/equipe", henry, async () => { throw new Error("rede") })
    const select = await screen.findByLabelText("Cargo de Henry")
    await userEvent.selectOptions(select, "3")
    await vi.waitFor(() => expect(select).toHaveValue("3"))
    expect(screen.queryByRole("alert")).toBeNull()
  })

  it("reloads the roles after registering someone", async () => {
    vi.spyOn(api, "createPerson").mockResolvedValue({ id: 4, name: "Caio", email: "caio@alis.com", role_id: 3, role_name: "Usuário", area_ids: [] })
    renderAt("/equipe", henry)
    await userEvent.type(await screen.findByLabelText("Nome"), "Caio")
    await userEvent.type(screen.getByLabelText("E-mail"), "caio@alis.com")
    await userEvent.type(screen.getByLabelText(/Senha inicial/), "senha-forte-1")
    const before = vi.mocked(api.roles).mock.calls.length
    await userEvent.click(screen.getByRole("button", { name: "Cadastrar" }))
    await vi.waitFor(() => expect(vi.mocked(api.roles).mock.calls.length).toBe(before + 1))
  })

  it("refreshes the session after editing one's own role", async () => {
    const refresh = vi.fn(async () => {})
    vi.spyOn(api, "updateRole").mockResolvedValue({ id: 1, key: "admin", name: "Administrador", permissions: PERMISSIONS.map((p) => p.key), people_count: 1 })
    renderAt("/equipe?aba=cargos", henry, refresh)
    await userEvent.click(await screen.findByRole("button", { name: "Editar Administrador" }))
    await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
    await vi.waitFor(() => expect(refresh).toHaveBeenCalled())
  })

  it("uses singular counts", async () => {
    vi.spyOn(api, "areas").mockResolvedValue([{ id: 2, name: "Investimentos", app_count: 1, leaders: [], members: [] }])
    renderAt("/equipe?aba=areas", henry)
    expect(await screen.findByText("1 app")).toBeInTheDocument()
    await userEvent.click(screen.getByRole("tab", { name: "Cargos" }))
    expect((await screen.findAllByText("1 pessoa")).length).toBeGreaterThan(0)
  })

  it("stacks the role under the name on a phone", async () => {
    renderAt("/equipe", henry)
    const select = await screen.findByLabelText("Cargo de Bia")
    // The row is a column below sm; the select is not squeezed beside the name.
    expect(select.parentElement).toHaveClass("flex-col", "sm:flex-row")
    expect(select).not.toHaveClass("w-44")
  })
})
