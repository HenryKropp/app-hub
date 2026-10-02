import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { activeRun, run as runFx } from "@/test/fixtures"
import { RunsProvider, useRuns, type RunTarget } from "./runs-context"

afterEach(() => vi.restoreAllMocks())

const target = (remembered = ""): RunTarget => ({ id: 3, name: "Files Prep", folder_display: "C:\\apps\\fp", remembered_folder: remembered })

function Probe({ t, folder, ask }: { t: RunTarget; folder?: string; ask?: boolean }) {
  const { run, notices, busy } = useRuns()
  return (
    <>
      <button onClick={() => run(t, { folder, ask })} disabled={busy.has(t.id)}>go</button>
      {notices.map((n) => (
        <p key={n.key}>
          {n.kind}:{n.message}
        </p>
      ))}
    </>
  )
}

const mount = (ui: React.ReactNode) => render(<RunsProvider>{ui}</RunsProvider>)

test("first time asks with Lembrar checked and sends remember", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx({ status: "waiting" }))
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("C:\\apps\\fp")
  expect(screen.getByLabelText("Lembrar esta pasta")).toBeChecked()
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(start).toHaveBeenCalledWith(3, "C:\\apps\\fp", true)
})

test("unchecking Lembrar sends remember false", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  await user.click(screen.getByLabelText("Lembrar esta pasta"))
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  expect(start).toHaveBeenCalledWith(3, "C:\\apps\\fp", false)
})

test("remembered folder runs in one click without remember", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target("D:\\Saídas")} />)
  await user.click(screen.getByText("go"))
  expect(screen.queryByLabelText("Onde salvar os resultados?")).not.toBeInTheDocument()
  expect(start).toHaveBeenCalledWith(3, "D:\\Saídas", undefined)
})

test("Rodar de novo uses the given folder", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target("D:\\Saídas")} folder={"E:\\Velha"} />)
  await user.click(screen.getByText("go"))
  expect(start).toHaveBeenCalledWith(3, "E:\\Velha", undefined)
})

test("ask opens the dialog with the remembered folder", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  mount(<Probe t={target("D:\\Saídas")} ask />)
  await user.click(screen.getByText("go"))
  expect(screen.getByLabelText("Onde salvar os resultados?")).toHaveValue("D:\\Saídas")
})

test("after remembering, the next click runs directly", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockResolvedValue(runFx())
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  await user.click(screen.getByRole("button", { name: "Abrir app" }))
  await user.click(await screen.findByText("go"))
  expect(start).toHaveBeenLastCalledWith(3, "C:\\apps\\fp", undefined)
  expect(screen.queryByLabelText("Onde salvar os resultados?")).not.toBeInTheDocument()
})

test("error while asking to run becomes an error notice", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "startRun").mockRejectedValue(new ApiError(409, "Seu computador não está conectado."))
  mount(<Probe t={target("D:\\Saídas")} />)
  await user.click(screen.getByText("go"))
  expect(await screen.findByText("error:Seu computador não está conectado.")).toBeInTheDocument()
})

test("button is disabled while the request is going out", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun").mockReturnValue(new Promise(() => {}))
  mount(<Probe t={target("D:\\Saídas")} />)
  await user.click(screen.getByText("go"))
  expect(screen.getByText("go")).toBeDisabled()
  expect(screen.getByText("starting:")).toBeInTheDocument()
  await user.click(screen.getByText("go"))
  expect(start).toHaveBeenCalledTimes(1)
})

test("Escolher outra fills the field", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "chooseFolder").mockResolvedValue({ folder: "D:\\Saídas", canceled: false })
  mount(<Probe t={target()} />)
  await user.click(screen.getByText("go"))
  await user.click(screen.getByRole("button", { name: "Escolher outra…" }))
  expect(await screen.findByDisplayValue("D:\\Saídas")).toBeInTheDocument()
})

function Cards() {
  const { active, notices, dismiss, refresh } = useRuns()
  return (
    <>
      <button onClick={refresh}>refresh</button>
      {notices.map((n) => <p key={n.key}>notice:{n.kind}</p>)}
      {active.map((r) => (
        <button key={r.id} onClick={() => dismiss(r.id)}>run:{r.id}:{r.status}</button>
      ))}
    </>
  )
}

test("the starting notice stays until the server lists the run, then the run replaces it", async () => {
  const user = userEvent.setup()
  const pending: ((v: ReturnType<typeof activeRun>[]) => void)[] = []
  const active = vi.spyOn(api, "activeRuns").mockResolvedValueOnce([])
  vi.spyOn(api, "startRun").mockResolvedValue(runFx({ id: 7, status: "waiting" }))
  mount(<><Probe t={target("D:\\Saídas")} /><Cards /></>)
  await vi.waitFor(() => expect(active).toHaveBeenCalledTimes(1))
  active.mockImplementation(() => new Promise((r) => void pending.push(r)))
  await user.click(screen.getByText("go"))
  // Request done, refresh in flight: the card must not disappear.
  expect(await screen.findByText("notice:starting")).toBeInTheDocument()
  await vi.waitFor(() => expect(pending.length).toBeGreaterThan(0))
  for (const r of pending) r([activeRun({ id: 7, status: "waiting" })])
  expect(await screen.findByText("run:7:waiting")).toBeInTheDocument()
  await vi.waitFor(() => expect(screen.queryByText("notice:starting")).not.toBeInTheDocument())
})

test("a poll in flight does not bring back a dismissed run", async () => {
  const user = userEvent.setup()
  let resolveActive: (v: ReturnType<typeof activeRun>[]) => void = () => {}
  const active = vi.spyOn(api, "activeRuns").mockResolvedValueOnce([activeRun({ id: 7, status: "done" })])
  vi.spyOn(api, "dismissRun").mockResolvedValue()
  mount(<Cards />)
  await screen.findByText("run:7:done")
  active.mockImplementation(() => new Promise((r) => (resolveActive = r)))
  await user.click(screen.getByText("refresh")) // poll leaves before the dismiss
  await user.click(screen.getByText("run:7:done"))
  resolveActive([activeRun({ id: 7, status: "done" })])
  await new Promise((r) => setTimeout(r, 20))
  expect(screen.queryByText("run:7:done")).not.toBeInTheDocument()
})
