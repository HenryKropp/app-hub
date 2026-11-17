import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type ApprovalRequest, type ItemGovernance } from "@/lib/api"
import { AppControlBar } from "./AppControl"

afterEach(() => vi.restoreAllMocks())

const base = { approved: false, suspended: false, require_approval: true, can_manage: true }
const req = (o: Partial<ApprovalRequest> = {}): ApprovalRequest => ({
  id: 7, kind: "app", item_id: 7, item_name: "Baixar", area: "Research", area_id: 1, reason: "new", risk_added: [], channel: "mcp",
  tested: true, status: "open", requested_by: "Ana", created_at: "2026-11-17T10:00:00Z", steps: [], my_steps: [],
  waiting: "Aguardando o líder de Research.", ...o,
})
const gov = (o: Partial<ItemGovernance> = {}): ItemGovernance => ({
  rule_company: "risk", rule_item: "", rule_effective: "risk", it_on: false, open_request: req(), history: [req()],
  can_loosen: true, can_change: true, ...o,
})
const mount = (ui: React.ReactElement) => render(<MemoryRouter>{ui}</MemoryRouter>)

test("the bar never approves; it says who is missing and points to Aprovações", () => {
  mount(<AppControlBar appId={7} control={base} gov={gov()} onChanged={() => {}} />)
  expect(screen.getByText("Aguardando o líder de Research.")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Aprovar" })).not.toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Ver em Aprovações" })).toHaveAttribute("href", "/aprovacoes")
})

test("a refused request shows the reason", () => {
  const refused = req({ status: "rejected", steps: [{ step: "leader", decision: "rejected", decided_by: "Paulo", reason: "site desconhecido", on_behalf: false }] })
  mount(<AppControlBar appId={7} control={base} gov={gov({ open_request: null, history: [refused] })} onChanged={() => {}} />)
  expect(screen.getByText(/Paulo recusou: site desconhecido/)).toBeInTheDocument()
})

test("suspending asks first; a suspended app can be reactivated", async () => {
  const set = vi.spyOn(api, "setAppControl").mockResolvedValue({ ...base, approved: true, suspended: true })
  vi.spyOn(window, "confirm").mockReturnValue(true)
  const { rerender } = mount(<AppControlBar appId={7} control={{ ...base, approved: true }} gov={gov({ open_request: null, history: [] })} onChanged={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Suspender" }))
  expect(set).toHaveBeenCalledWith(7, { approved: true, suspended: true })
  rerender(<MemoryRouter><AppControlBar appId={7} control={{ ...base, approved: true, suspended: true }} onChanged={() => {}} /></MemoryRouter>)
  expect(screen.getByText("App suspenso.")).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Reativar" })).toBeInTheDocument()
})

test("people who can't manage see why the app can't run, without buttons", () => {
  mount(<AppControlBar appId={7} control={{ ...base, can_manage: false }} gov={gov()} onChanged={() => {}} />)
  expect(screen.getByText("Aguardando o líder de Research.")).toBeInTheDocument()
  expect(screen.queryByRole("button")).not.toBeInTheDocument()
})

test("an approved app shows nothing to people who can't manage", () => {
  const { container } = mount(<AppControlBar appId={7} control={{ ...base, approved: true, can_manage: false }} gov={gov({ open_request: null, history: [] })} onChanged={() => {}} />)
  expect(container).toBeEmptyDOMElement()
})
