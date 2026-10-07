import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api, type AppDetail, type SharedState } from "@/lib/api"
import { detail as makeDetail, manifest } from "@/test/fixtures"
import { Tech } from "./Tech"

const state: SharedState = { status: "running", status_message: "", stopped_by: "", always_on: false, idle_minutes: 30,
  memory_mb: 512, cpus: 0.5, wants_db: false, url: "", acks: [], has_previous: false }
const app: AppDetail = makeDetail({ manifest: manifest({ kind: "web", shared: { enabled: true } }), shared: { status: "running", url: "", slug: "notas" } })

beforeEach(() => {
  vi.restoreAllMocks()
  vi.spyOn(api.shared, "usage").mockResolvedValue([])
  vi.spyOn(api.shared, "logs").mockResolvedValue({ text: "" })
})

it("saving the limits carries the 'estou ciente' marks saved meanwhile", async () => {
  vi.spyOn(api.shared, "get")
    .mockResolvedValueOnce({ shared: state, deployments: [] })
    .mockResolvedValueOnce({ shared: { ...state, acks: ["file_data"] }, deployments: [] })
  const save = vi.spyOn(api.shared, "save").mockResolvedValue(app)
  render(<Tech app={app} onChanged={() => {}} />)
  await userEvent.selectOptions(await screen.findByLabelText("Memória"), "1024")
  await userEvent.click(screen.getByRole("button", { name: "Salvar limites" }))
  await vi.waitFor(() => expect(save).toHaveBeenCalled())
  expect(save.mock.calls[0][1]).toMatchObject({ memory_mb: 1024, acks: ["file_data"] })
})

it("says so when it does not load, and tries again", async () => {
  vi.spyOn(api.shared, "get")
    .mockRejectedValueOnce(new Error("O Hub não respondeu. Espere um pouco e tente de novo."))
    .mockResolvedValueOnce({ shared: state, deployments: [] })
  render(<Tech app={app} onChanged={() => {}} />)
  expect(await screen.findByText("O Hub não respondeu. Espere um pouco e tente de novo.")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Tentar de novo" }))
  expect(await screen.findByLabelText("Memória")).toBeInTheDocument()
})
