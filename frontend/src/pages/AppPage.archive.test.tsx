import { render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type AppDetail } from "@/lib/api"
import * as auth from "@/lib/auth"
import { RunsProvider } from "@/lib/runs-context"
import { VIEW_ONLY, bia, detail, henry } from "@/test/fixtures"
import { AppPage } from "./AppPage"

// Arquivar on the app page (ADR 031): only for the owner or an administrator (can.access).

afterEach(() => vi.restoreAllMocks())

const mount = (d: AppDetail, person = henry) => {
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person, refresh: async () => {}, setPerson: () => {} })
  vi.spyOn(api, "app").mockResolvedValue(d)
  vi.spyOn(api, "history").mockResolvedValue({ items: [], more: false, next_before: "" })
  vi.spyOn(api, "activeRuns").mockResolvedValue([])
  vi.spyOn(api, "appRuns").mockResolvedValue([])
  vi.spyOn(api, "checkApp").mockResolvedValue({ folder_readable: true, version: null })
  vi.spyOn(api, "appAccess").mockResolvedValue({ restricted: false, areas: [], people: [], grants: [], area_grants: [] })
  vi.spyOn(api.settings, "list").mockResolvedValue({ items: [], can_manage: true, missing: [], locked: false })
  return render(
    <MemoryRouter initialEntries={["/apps/3"]}>
      <RunsProvider>
        <Routes>
          <Route path="/apps/:id" element={<AppPage />} />
        </Routes>
      </RunsProvider>
    </MemoryRouter>,
  )
}

test("the owner or an administrator sees Arquivar", async () => {
  mount(detail())
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  expect(screen.getByRole("button", { name: "Arquivar" })).toBeInTheDocument()
})

test("someone who only uses the app does not", async () => {
  mount(detail({ can: VIEW_ONLY }), bia)
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  expect(screen.queryByRole("button", { name: "Arquivar" })).not.toBeInTheDocument()
})

test("an editor without control of the access does not", async () => {
  mount(detail({ can: { ...VIEW_ONLY, edit: true, restore: true, tech: true } }))
  await screen.findByRole("heading", { name: "Eq. Research Files Prep" })
  expect(screen.queryByRole("button", { name: "Arquivar" })).not.toBeInTheDocument()
})
