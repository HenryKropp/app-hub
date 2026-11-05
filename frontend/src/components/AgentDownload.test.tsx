import type { ReactNode } from "react"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type ComputersInfo } from "@/lib/api"
import { ComputerStatusProvider } from "@/lib/computer-status"
import { RunsProvider, useRuns } from "@/lib/runs-context"
import { AgentBanner } from "./AgentDownload"

afterEach(() => vi.restoreAllMocks())

const computers = (connected: boolean | null): ComputersInfo => ({
  computers: connected === null ? [] : [{ id: 1, name: "PC", connected, last_seen_at: null, agent_version: "0.3.2" } as never],
  expected_version: "0.3.2",
  agent_available: true,
})

const withStatus = (ui: ReactNode) => render(<MemoryRouter><ComputerStatusProvider>{ui}</ComputerStatusProvider></MemoryRouter>)

test("the strip warns while the computer is not connected", async () => {
  vi.spyOn(api, "computers").mockResolvedValue(computers(null))
  withStatus(<AgentBanner />)
  expect(await screen.findByText("Seu computador não está conectado ao Ziv.")).toBeInTheDocument()
  expect(screen.getByText(/não será possível rodar os apps do catálogo/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Baixar Agente" })).toBeInTheDocument()
})

test("the strip stays away when connected", async () => {
  const get = vi.spyOn(api, "computers").mockResolvedValue(computers(true))
  withStatus(<AgentBanner />)
  await vi.waitFor(() => expect(get).toHaveBeenCalled())
  expect(screen.queryByText(/não está conectado/)).not.toBeInTheDocument()
})

function RunButton() {
  const { run } = useRuns()
  return <button onClick={() => run({ id: 3, name: "Relatório", folder_display: "", remembered_folder: "C:\\saida", kind: "desktop" })}>Rodar</button>
}

test("Rodar without a connected computer explains instead of starting", async () => {
  vi.spyOn(api, "computers").mockResolvedValue(computers(false))
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  const start = vi.spyOn(api, "startRun")
  withStatus(<RunsProvider><RunButton /></RunsProvider>)
  await vi.waitFor(() => expect(api.computers).toHaveBeenCalled())
  await userEvent.click(screen.getByRole("button", { name: "Rodar" }))
  expect(await screen.findByRole("dialog", { name: "Conecte seu computador para rodar" })).toBeInTheDocument()
  expect(start).not.toHaveBeenCalled()
})
