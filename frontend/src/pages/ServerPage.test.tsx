import { act, render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { beforeEach, expect, it, vi } from "vitest"
import { toast } from "sonner"
import { api, ApiError, type ServerInfo } from "@/lib/api"
import { parseReais, ServerPage } from "./ServerPage"

const info: ServerInfo = {
  memory_total: 8 * 2 ** 30, memory_used: 3 * 2 ** 30, disk_total: 100 * 2 ** 30, disk_used: 95 * 2 ** 30, disk_warning: true,
  engine_ok: true, monthly_cost_cents: 30000, hub_cents: 9000, period_start: "2026-10-01T00:00:00-03:00",
  apps: [{ app_id: 1, name: "Sistema Alis", slug: "sistema-alis", status: "running", always_on: true, memory_mb: 1024, cpus: 1,
    idle_minutes: 30, hours_on: 120, avg_mem_mb: 400, max_mem_mb: 700, avg_cpu: 4, db_bytes: 50 * 2 ** 20, backup_bytes: 200 * 2 ** 20, cost_cents: 1234 }],
}

beforeEach(() => { vi.restoreAllMocks(); vi.spyOn(api.server, "get").mockResolvedValue(info) })
const renderPage = () => render(<MemoryRouter><ServerPage /></MemoryRouter>)

it("shows usage, the disk warning and the cost per app", async () => {
  renderPage()
  expect(await screen.findByText("Sistema Alis")).toBeInTheDocument()
  expect(screen.getByText("R$ 12,34")).toBeInTheDocument()
  expect(screen.getByText("R$ 90,00")).toBeInTheDocument()
  expect(screen.getByText(/O disco passou de 90%/)).toBeInTheDocument()
})

it("saves the monthly cost in cents", async () => {
  const set = vi.spyOn(api.server, "setCost").mockResolvedValue({})
  renderPage()
  const input = await screen.findByLabelText("Quanto o servidor custa por mês")
  await userEvent.clear(input)
  await userEvent.type(input, "450,50")
  await userEvent.click(screen.getByRole("button", { name: "Salvar custo" }))
  expect(set).toHaveBeenCalledWith(45050)
})

it("refuses an invalid cost without calling the API", async () => {
  const set = vi.spyOn(api.server, "setCost").mockResolvedValue({})
  renderPage()
  const input = await screen.findByLabelText("Quanto o servidor custa por mês")
  await userEvent.clear(input)
  await userEvent.type(input, "abc")
  await userEvent.click(screen.getByRole("button", { name: "Salvar custo" }))
  expect(set).not.toHaveBeenCalled()
  expect(screen.getByText("Digite um valor em reais, por exemplo 300 ou 300,50.")).toBeInTheDocument()
})

it("stops an app", async () => {
  const stop = vi.spyOn(api.server, "stop").mockResolvedValue({})
  renderPage()
  await userEvent.click(await screen.findByRole("button", { name: "Parar Sistema Alis" }))
  await userEvent.click(screen.getByRole("button", { name: "Sim, parar" }))
  expect(stop).toHaveBeenCalledWith(1)
})

it("shows the server's message as-is when stopping fails", async () => {
  const err = vi.spyOn(toast, "error").mockImplementation(() => "")
  vi.spyOn(api.server, "stop").mockRejectedValue(new ApiError(409, "Já tem uma ação em andamento. Espere um pouco e veja de novo."))
  renderPage()
  await userEvent.click(await screen.findByRole("button", { name: "Parar Sistema Alis" }))
  await userEvent.click(screen.getByRole("button", { name: "Sim, parar" }))
  expect(err).toHaveBeenCalledWith("Já tem uma ação em andamento. Espere um pouco e veja de novo.")
})

it("saves limits and says when they wait for the next start", async () => {
  const ok = vi.spyOn(toast, "success").mockImplementation(() => "")
  const set = vi.spyOn(api.server, "setApp").mockResolvedValue({ needs_restart: true })
  renderPage()
  await userEvent.click(await screen.findByRole("button", { name: "Limites de Sistema Alis" }))
  await userEvent.selectOptions(within(screen.getByRole("dialog")).getByLabelText("Memória"), "2048")
  await userEvent.click(screen.getByRole("button", { name: "Salvar limites" }))
  expect(set).toHaveBeenCalledWith(1, { memory_mb: 2048, cpus: 1, always_on: true, idle_minutes: 30 })
  expect(ok).toHaveBeenCalledWith("Salvo. Vale na próxima vez que ligar.")
})

it("warns when Podman is down", async () => {
  vi.spyOn(api.server, "get").mockResolvedValue({ ...info, engine_ok: false })
  renderPage()
  expect(await screen.findByText(/O Podman não está respondendo/)).toBeInTheDocument()
})

it("explains when cost is zero", async () => {
  vi.spyOn(api.server, "get").mockResolvedValue({ ...info, monthly_cost_cents: 0 })
  renderPage()
  expect(await screen.findByText("Informe o custo mensal para ver quanto cada app gasta.")).toBeInTheDocument()
})

it("says there is no access on 403", async () => {
  vi.spyOn(api.server, "get").mockRejectedValue(new ApiError(403, "negado"))
  renderPage()
  expect(await screen.findByText("Você não tem acesso a esta página.")).toBeInTheDocument()
})

it("offers a retry on other load errors", async () => {
  vi.spyOn(api.server, "get").mockRejectedValueOnce(new ApiError(500, "O Ziv não respondeu. Tente de novo.")).mockResolvedValue(info)
  renderPage()
  expect(await screen.findByText("O Ziv não respondeu. Tente de novo.")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Tentar de novo" }))
  expect(await screen.findByText("Sistema Alis")).toBeInTheDocument()
})

it("reads reais strictly", () => {
  expect(parseReais("300")).toBe(30000)
  expect(parseReais("300,50")).toBe(30050)
  expect(parseReais("1.234,56")).toBe(123456)
  expect(parseReais("1234,5")).toBe(123450)
  expect(parseReais(" 300 ")).toBe(30000)
  for (const bad of ["300abc", "1e3", "300.50", "", "abc", "-5", "1.23,4", "300,505", "12.34"]) expect(parseReais(bad)).toBeNull()
})

it("the minute reload does not overwrite the cost being typed, and refills it otherwise", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  try {
    const get = vi.spyOn(api.server, "get").mockResolvedValue(info)
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    renderPage()
    const input = await screen.findByLabelText("Quanto o servidor custa por mês")
    expect(input).toHaveValue("300,00")
    get.mockResolvedValue({ ...info, monthly_cost_cents: 40000 })
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000) })
    expect(input).toHaveValue("400,00") // nobody was editing: shows the new value
    await user.clear(input)
    await user.type(input, "45")
    get.mockResolvedValue({ ...info, monthly_cost_cents: 50000 })
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000) })
    expect(input).toHaveValue("45")
    await user.clear(input)
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000) })
    expect(input).toHaveValue("") // emptied on purpose: still editing
  } finally {
    vi.useRealTimers()
  }
})
