import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, it, vi } from "vitest"
import { api } from "@/lib/api"
import { ALL_CAN, VIEW_ONLY, ana, bia, detail, henry } from "@/test/fixtures"
import { AccessSection } from "./AccessSection"

afterEach(() => vi.restoreAllMocks())

const open = { restricted: false, areas: [], people: [], grants: [], area_grants: [] }

it("the owner restricts but does not see the grants form", async () => {
  vi.spyOn(api, "appAccess").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [] })
  vi.spyOn(api, "areas").mockResolvedValue([{ id: 2, name: "Investimentos", app_count: 1, leaders: [], members: [] }])
  vi.spyOn(api, "people").mockResolvedValue([])
  const set = vi.spyOn(api, "setAppAccess").mockResolvedValue({ restricted: true, areas: [{ id: 2, name: "Investimentos" }], people: [], grants: [], area_grants: [] })
  render(<AccessSection app={detail({ can: { ...ALL_CAN, grants: false } })} me={ana} />)
  expect(await screen.findByText(/Quem vê: Todos/)).toBeInTheDocument()
  expect(screen.queryByText("Dar acesso extra")).toBeNull()
  await userEvent.click(screen.getByRole("button", { name: "Editar Quem vê" }))
  await userEvent.click(screen.getByLabelText("Só algumas áreas e pessoas"))
  await userEvent.click(screen.getByLabelText("Investimentos"))
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(set).toHaveBeenCalledWith(expect.any(Number), { restricted: true, area_ids: [2], person_ids: [] })
  expect(await screen.findByText(/só Investimentos/)).toBeInTheDocument()
})

it("the leader gives an extra access for the whole area", async () => {
  vi.spyOn(api, "appAccess").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [] })
  vi.spyOn(api, "areas").mockResolvedValue([])
  vi.spyOn(api, "people").mockResolvedValue([{ id: 3, name: "Bia", email: "bia@alis.com", role_id: 3, role_name: "Usuário", area_ids: [] }])
  const set = vi.spyOn(api, "setGrants").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [{ person_id: 3, name: "Bia", permissions: ["runs"] }] })
  render(<AccessSection app={detail()} me={henry} />)
  await userEvent.selectOptions(await screen.findByLabelText("Pessoa"), "3")
  await userEvent.click(screen.getByLabelText("Ver execuções de todos"))
  await userEvent.click(screen.getByLabelText(/Em todos os apps da área/))
  await userEvent.click(screen.getByRole("button", { name: "Dar acesso" }))
  expect(set).toHaveBeenCalledWith(expect.any(Number), { person_id: 3, permissions: ["runs"], whole_area: true })
  expect(await screen.findByText(/todos os apps da área/)).toBeInTheDocument()
})

it("lists extra accesses with their labels and removes one", async () => {
  vi.spyOn(api, "appAccess").mockResolvedValue({ ...open, restricted: true, areas: [{ id: 2, name: "Investimentos" }], people: [{ id: 3, name: "Bia" }],
    grants: [{ person_id: 3, name: "Bia", permissions: ["edit", "runs"] }] })
  vi.spyOn(api, "areas").mockResolvedValue([])
  vi.spyOn(api, "people").mockResolvedValue([])
  const set = vi.spyOn(api, "setGrants").mockResolvedValue(open)
  render(<AccessSection app={detail()} me={henry} />)
  expect(await screen.findByText(/Quem vê: só Investimentos e Bia/)).toBeInTheDocument()
  expect(screen.getByText("Bia: Editar a ficha, Ver execuções de todos")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Tirar Bia" }))
  expect(set).toHaveBeenCalledWith(expect.any(Number), { person_id: 3, permissions: [], whole_area: false })
  expect(await screen.findByText("Ninguém tem acesso extra.")).toBeInTheDocument()
})

it("someone who only sees the technical part cannot change access", async () => {
  vi.spyOn(api, "appAccess").mockResolvedValue(open)
  render(<AccessSection app={detail({ can: { ...VIEW_ONLY, tech: true } })} me={bia} />)
  expect(await screen.findByText(/Quem vê: Todos/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Editar Quem vê" })).toBeNull()
  expect(screen.queryByText("Dar acesso extra")).toBeNull()
})
