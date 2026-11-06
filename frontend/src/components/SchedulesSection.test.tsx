import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type Schedule } from "@/lib/api"
import { manifest } from "@/test/fixtures"
import { SchedulesSection } from "./SchedulesSection"

afterEach(() => vi.restoreAllMocks())

const base: Schedule = {
  id: 1, app_id: 3, app_name: "Relatório", person_id: 1, person_name: "Henry", mine: true, name: "",
  frequency: { kind: "daily", at: "07:00" }, describe: "Todo dia às 7h", computer_id: 7, computer_name: "NOTE-HENRY",
  active: true, paused_reason: "", situation: "", next_run_at: "2026-11-07T10:00:00Z", last_run_at: null, last_run_id: null,
  last_run_status: "", upcoming: ["2026-11-07T10:00:00Z"],
}

function show(list: Schedule[]) {
  vi.spyOn(api, "appSchedules").mockResolvedValue(list)
  return render(<MemoryRouter><SchedulesSection appId={3} appName="Relatório" manifest={manifest({ kind: "script" })} reloadKey={0} /></MemoryRouter>)
}

test("lists a schedule with its frequency and next run", async () => {
  show([base])
  expect(await screen.findByText("Todo dia às 7h")).toBeInTheDocument()
  expect(screen.getByText(/Próxima:/)).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Pausar" })).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Editar" })).toBeInTheDocument()
})

test("a paused schedule shows why and offers Reativar", async () => {
  const user = userEvent.setup()
  show([{ ...base, active: false, paused_reason: "Falhou 5 vezes seguidas", upcoming: [] }])
  expect(await screen.findByText("Falhou 5 vezes seguidas")).toBeInTheDocument()
  const resume = vi.spyOn(api, "resumeSchedule").mockResolvedValue({ ...base })
  await user.click(screen.getByRole("button", { name: "Reativar" }))
  expect(resume).toHaveBeenCalledWith(1)
})

test("someone else's schedule has no Editar, but whoever cares for the app can pause it", async () => {
  show([{ ...base, mine: false, person_name: "Bia" }])
  expect(await screen.findByText(/Bia/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Editar" })).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Pausar" })).toBeInTheDocument()
})

test("hides itself when there are no schedules", async () => {
  const { container } = show([])
  await vi.waitFor(() => expect(api.appSchedules).toHaveBeenCalled())
  expect(container).toBeEmptyDOMElement()
})
