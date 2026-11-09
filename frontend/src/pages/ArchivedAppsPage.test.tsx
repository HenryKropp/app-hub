import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type ArchivedApp } from "@/lib/api"
import { ArchivedAppsPage, daysLeftText } from "./ArchivedAppsPage"

afterEach(() => vi.restoreAllMocks())

const archived = (over: Partial<ArchivedApp> = {}): ArchivedApp => ({
  id: 7, name: "Notas", summary: "", kind: "web", area: "Research", owner_name: "Ana",
  archived_at: "2026-11-02T12:00:00Z", archived_by_name: "Henry", purge_at: "2026-12-02T12:00:00Z", days_left: 23, ...over,
})

const mount = () => render(<MemoryRouter><ArchivedAppsPage /></MemoryRouter>)

test("days left in plain words", () => {
  expect(daysLeftText(23)).toBe("Será apagado de vez em 23 dias")
  expect(daysLeftText(1)).toBe("Será apagado de vez em 1 dia")
  expect(daysLeftText(0)).toBe("Será apagado de vez nas próximas horas")
})

test("lists the archived apps with who archived them and the days left", async () => {
  vi.spyOn(api, "archivedApps").mockResolvedValue([archived()])
  mount()
  expect(await screen.findByText("Notas")).toBeInTheDocument()
  expect(screen.getByText(/Arquivado por Henry em/)).toBeInTheDocument()
  expect(screen.getByText("Será apagado de vez em 23 dias.")).toBeInTheDocument()
  expect(screen.getByText(/Responsável Ana/)).toBeInTheDocument()
})

test("Trazer de volta restores it and says schedules and the server stay off", async () => {
  vi.spyOn(api, "archivedApps").mockResolvedValue([archived()])
  const restore = vi.spyOn(api, "unarchiveApp").mockResolvedValue({ id: 7, name: "Notas" })
  mount()
  await userEvent.click(await screen.findByRole("button", { name: "Trazer de volta" }))
  expect(restore).toHaveBeenCalledWith(7)
  expect(await screen.findByText(/ligue de novo se precisar/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Notas" })).toHaveAttribute("href", "/apps/7")
  expect(screen.queryByRole("button", { name: "Trazer de volta" })).not.toBeInTheDocument()
})

test("nothing archived says so", async () => {
  vi.spyOn(api, "archivedApps").mockResolvedValue([])
  mount()
  expect(await screen.findByText("Nenhum app arquivado")).toBeInTheDocument()
})
