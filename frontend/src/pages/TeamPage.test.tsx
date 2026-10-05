import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { ApiError, api, type Person } from "@/lib/api"
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
  })
  afterEach(() => vi.restoreAllMocks())

  it("shows the three tabs to an admin", async () => {
    renderAt("/equipe", henry)
    for (const t of ["Pessoas", "Áreas", "Cargos"]) expect(await screen.findByRole("tab", { name: t })).toBeInTheDocument()
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
    vi.spyOn(api, "roles").mockRejectedValue(new ApiError(500, "O Hub não respondeu."))
    renderAt("/equipe", henry)
    expect(await screen.findByRole("alert")).toHaveTextContent("O Hub não respondeu.")
    expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeInTheDocument()
  })

  it("sends someone who cannot see the team back to the catalog", async () => {
    renderAt("/equipe", bia)
    expect(await screen.findByText("Catálogo")).toBeInTheDocument()
    expect(screen.queryByRole("tab")).toBeNull()
  })
})
