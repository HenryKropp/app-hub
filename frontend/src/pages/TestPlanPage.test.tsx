import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type TestPlan } from "@/lib/api"
import { TestPlanPage } from "./TestPlanPage"

const CONFIRM = "Os próximos testes deste app serão automáticos. Só pergunto de novo se o plano de teste mudar."

const plan = (o: Partial<TestPlan> = {}): TestPlan => ({
  plan: { acessa: ["o site do BTG (login com BTG_SENHA)"], pula: "mover para a pasta oficial", keys: ["BTG_SENHA"], place: "pc" },
  fingerprint: "abc",
  confirmed: null,
  matches: false,
  computers: [{ id: 7, name: "PC do escritório", connected: true, outdated: false }, { id: 8, name: "Notebook", connected: false, outdated: false }],
  can_confirm: true,
  confirm_text: CONFIRM,
  ...o,
})

afterEach(() => vi.restoreAllMocks())

const mount = () =>
  render(
    <MemoryRouter initialEntries={["/apps/4/plano-de-teste"]}>
      <Routes>
        <Route path="/apps/:id/plano-de-teste" element={<TestPlanPage />} />
      </Routes>
    </MemoryRouter>,
  )

test("shows the plan and confirms with the fingerprint and the chosen PC", async () => {
  const user = userEvent.setup()
  vi.spyOn(api.testPlan, "get").mockResolvedValue(plan())
  const confirm = vi.spyOn(api.testPlan, "confirm").mockResolvedValue(plan({ confirmed: { by: "Henry", at: "2026-11-16T12:00:00Z", computer_id: 8 }, matches: true }))
  mount()
  expect(await screen.findByRole("heading", { name: "Plano de teste" })).toBeInTheDocument()
  expect(screen.getByText("o site do BTG (login com BTG_SENHA)")).toBeInTheDocument()
  expect(screen.getByText("mover para a pasta oficial")).toBeInTheDocument()
  expect(screen.getByText("BTG_SENHA")).toBeInTheDocument()
  expect(screen.getByText(CONFIRM)).toBeInTheDocument()
  await user.selectOptions(screen.getByLabelText(/Onde roda/), "8")
  await user.click(screen.getByRole("button", { name: "Confirmar plano de teste" }))
  expect(confirm).toHaveBeenCalledWith(4, "abc", 8)
  expect(await screen.findByText(/Plano confirmado por Henry em 16\/11/)).toBeInTheDocument()
})

test("without permission shows the message and hides the button", async () => {
  vi.spyOn(api.testPlan, "get").mockResolvedValue(plan({ can_confirm: false }))
  mount()
  expect(await screen.findByText("Só o dono do app ou um administrador confirma este plano.")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Confirmar plano de teste" })).not.toBeInTheDocument()
})

test("already confirmed changes only the PC", async () => {
  const user = userEvent.setup()
  vi.spyOn(api.testPlan, "get").mockResolvedValue(plan({ confirmed: { by: "Henry", at: "2026-11-10T12:00:00Z", computer_id: 7 }, matches: true }))
  const setPC = vi.spyOn(api.testPlan, "setComputer").mockResolvedValue(plan({ confirmed: { by: "Henry", at: "2026-11-10T12:00:00Z", computer_id: 8 }, matches: true }))
  mount()
  expect(await screen.findByText(/Plano confirmado por Henry em 10\/11/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Confirmar plano de teste" })).not.toBeInTheDocument()
  await user.selectOptions(screen.getByLabelText(/Onde roda/), "8")
  await user.click(screen.getByRole("button", { name: "Trocar o PC de teste" }))
  expect(setPC).toHaveBeenCalledWith(4, 8)
})

test("a plan confirmed in the Claude Code chat says so", async () => {
  vi.spyOn(api.testPlan, "get").mockResolvedValue(plan({ confirmed: { by: "Henry", at: "2026-11-18T12:00:00Z", computer_id: 7, via: "claude_code" }, matches: true }))
  mount()
  expect(await screen.findByText(/Plano confirmado por Henry pelo Claude Code em 18\/11/)).toBeInTheDocument()
})
