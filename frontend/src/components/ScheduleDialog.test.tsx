import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { manifest } from "@/test/fixtures"
import { ScheduleDialog } from "./ScheduleDialog"

afterEach(() => vi.restoreAllMocks())

const computers = {
  computers: [
    { id: 4, name: "PC antigo", agent_version: "1", last_seen_at: null, connected: false, outdated: false, update_state: "" as const },
    { id: 7, name: "NOTE-HENRY", agent_version: "1", last_seen_at: null, connected: true, outdated: false, update_state: "" as const },
  ],
  expected_version: "1", agent_available: true,
}
const m = manifest({ kind: "script", inputs: [{ id: "mes", label: "Mês", type: "text", source: "argument", required: true }] })

function open() {
  vi.spyOn(api, "computers").mockResolvedValue(computers)
  const save = vi.spyOn(api, "createSchedule").mockResolvedValue({} as never)
  const saved = vi.fn()
  render(<ScheduleDialog appId={3} appName="Relatório" manifest={m} onClose={vi.fn()} onSaved={saved} />)
  return { save, saved }
}

test("saves a daily schedule with the app's fields, folder and the connected computer", async () => {
  const user = userEvent.setup()
  const { save, saved } = open()
  await user.type(await screen.findByLabelText("Mês"), "outubro")
  await user.type(screen.getByLabelText("Onde salvar os resultados?"), "D:\\Saídas")
  await user.selectOptions(screen.getByLabelText("Frequência"), "daily")
  await user.clear(screen.getByLabelText("Às"))
  await user.type(screen.getByLabelText("Às"), "07:00")
  expect(screen.getByText(/Próximas:/)).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Salvar agendamento" }))
  expect(save).toHaveBeenCalledWith(3, {
    name: "", frequency: { kind: "daily", at: "07:00" }, computer_id: 7, output_folder: "D:\\Saídas",
    inputs: { mes: "outubro" }, files: {},
  })
  expect(saved).toHaveBeenCalled()
})

test("refuses fewer than 5 minutes", async () => {
  const user = userEvent.setup()
  const { save } = open()
  await user.type(await screen.findByLabelText("Mês"), "outubro")
  await user.type(screen.getByLabelText("Onde salvar os resultados?"), "D:\\Saídas")
  await user.selectOptions(screen.getByLabelText("Frequência"), "minutes")
  await user.clear(screen.getByLabelText("A cada quantos minutos"))
  await user.type(screen.getByLabelText("A cada quantos minutos"), "4")
  expect(screen.getByText("No mínimo a cada 5 minutos.")).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Salvar agendamento" })).toBeDisabled()
  expect(save).not.toHaveBeenCalled()
})

test("weekdays without a day cannot be saved", async () => {
  const user = userEvent.setup()
  open()
  await user.type(await screen.findByLabelText("Mês"), "outubro")
  await user.type(screen.getByLabelText("Onde salvar os resultados?"), "D:\\Saídas")
  await user.selectOptions(screen.getByLabelText("Frequência"), "weekdays")
  for (const d of ["Seg", "Ter", "Qua", "Qui", "Sex"]) {
    const box = screen.getByRole("checkbox", { name: d })
    if ((box as HTMLInputElement).checked) await user.click(box)
  }
  expect(screen.getByRole("button", { name: "Salvar agendamento" })).toBeDisabled()
})

test("editing leaves out values of fields the app no longer has, so the schedule can be fixed", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "computers").mockResolvedValue(computers)
  const update = vi.spyOn(api, "updateSchedule").mockResolvedValue({} as never)
  const schedule = {
    id: 9, app_id: 3, app_name: "Relatório", person_id: 1, person_name: "Henry", mine: true, name: "",
    frequency: { kind: "daily" as const, at: "07:00" }, describe: "Todo dia às 7h", computer_id: 7, computer_name: "NOTE-HENRY",
    active: false, paused_reason: "O app mudou", situation: "", next_run_at: null, last_run_at: null, last_run_id: null,
    last_run_status: "" as const, upcoming: [], output_folder: "D:\\Saídas",
    inputs: { mes: ["outubro"], velho: ["x"] }, files: { antigo: { name: "a.csv" } },
  }
  render(<ScheduleDialog appId={3} appName="Relatório" manifest={m} schedule={schedule} onClose={vi.fn()} onSaved={vi.fn()} />)
  await screen.findByRole("option", { name: "NOTE-HENRY" })
  await user.click(screen.getByRole("button", { name: "Salvar agendamento" }))
  expect(update).toHaveBeenCalledWith(9, expect.objectContaining({ inputs: { mes: "outubro" }, files: {} }))
})
