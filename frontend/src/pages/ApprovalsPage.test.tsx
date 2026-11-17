import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type ApprovalRequest } from "@/lib/api"
import { ApprovalsPage } from "./ApprovalsPage"

const req = (o: Partial<ApprovalRequest> = {}): ApprovalRequest => ({
  id: 7, kind: "app", item_id: 12, item_name: "Baixar Relatórios", area: "Research", area_id: 1,
  reason: "version", risk_added: [], channel: "mcp", tested: true, status: "open", requested_by: "Ana",
  created_at: new Date(Date.now() - 2 * 3600_000).toISOString(),
  steps: [{ step: "leader", decision: "pending", decided_by: "", reason: "", on_behalf: false }],
  my_steps: ["leader"], ...o,
})

beforeEach(() => {
  vi.spyOn(api, "areas").mockResolvedValue([
    { id: 1, name: "Research", app_count: 0, leaders: [], members: [] },
    { id: 2, name: "Gestão", app_count: 0, leaders: [], members: [] },
  ])
  vi.spyOn(api, "people").mockResolvedValue([])
})
afterEach(() => vi.restoreAllMocks())

const mount = () => render(<MemoryRouter><ApprovalsPage /></MemoryRouter>)

test("the card says what changes in the risk", async () => {
  vi.spyOn(api, "approvals").mockResolvedValue([req({ risk_added: ["acessa site novo: api.exemplo.com"] })])
  mount()
  expect(await screen.findByText("Baixar Relatórios")).toBeInTheDocument()
  expect(screen.getByText(/versão nova · Research/)).toBeInTheDocument()
  expect(screen.getByText(/pelo Claude/)).toBeInTheDocument()
  expect(screen.getByText("acessa site novo: api.exemplo.com")).toBeInTheDocument()
})

test("approving a new app needs who uses it", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "approvals").mockResolvedValue([req({ reason: "new" })])
  const approve = vi.spyOn(api, "approveRequest").mockResolvedValue({ status: "approved", waiting: "" })
  mount()
  await user.click(await screen.findByRole("button", { name: "Aprovar" }))
  const confirm = screen.getByRole("button", { name: "Aprovar e liberar" })
  expect(confirm).toBeDisabled()
  await user.click(screen.getByRole("radio", { name: "Empresa toda" }))
  expect(confirm).toBeEnabled()
  await user.click(confirm)
  await waitFor(() => expect(approve).toHaveBeenCalledWith(7, { mode: "company", area_ids: [], person_ids: [] }))
})

test("a version approves without choosing who uses it", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "approvals").mockResolvedValue([req()])
  const approve = vi.spyOn(api, "approveRequest").mockResolvedValue({ status: "approved", waiting: "" })
  mount()
  await user.click(await screen.findByRole("button", { name: "Aprovar" }))
  await user.click(screen.getByRole("button", { name: "Aprovar e liberar" }))
  await waitFor(() => expect(approve).toHaveBeenCalledWith(7, undefined))
})

test("rejecting needs a reason", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "approvals").mockResolvedValue([req()])
  const reject = vi.spyOn(api, "rejectRequest").mockResolvedValue({ status: "rejected" })
  mount()
  await user.click(await screen.findByRole("button", { name: "Recusar" }))
  const confirm = screen.getByRole("button", { name: "Recusar pedido" })
  expect(confirm).toBeDisabled()
  await user.type(screen.getByLabelText("Por que recusa?"), "site desconhecido")
  await user.click(confirm)
  await waitFor(() => expect(reject).toHaveBeenCalledWith(7, "site desconhecido"))
})

test("what went live with a warning stays folded", async () => {
  vi.spyOn(api, "approvals").mockResolvedValue([req({ id: 9, status: "auto", item_name: "Painel", my_steps: [] })])
  mount()
  expect(await screen.findByText(/Foram ao ar com aviso/)).toBeInTheDocument()
  expect(screen.getByText("Nada esperando sua aprovação.")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Aprovar" })).not.toBeInTheDocument()
})
