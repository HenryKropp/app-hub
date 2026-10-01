import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import * as auth from "@/lib/auth"
import { ana, henry, manifest, scanItem } from "@/test/fixtures"
import { RegisterPage } from "./RegisterPage"

vi.mock("sonner", () => ({ toast: { success: vi.fn() } }))

beforeEach(() => {
  localStorage.clear()
  vi.spyOn(auth, "useAuth").mockReturnValue({ loading: false, needsSetup: false, person: henry, refresh: async () => {}, setPerson: () => {} })
  vi.spyOn(api, "people").mockResolvedValue([henry, ana])
})
afterEach(() => vi.restoreAllMocks())

const renderPage = () =>
  render(
    <MemoryRouter>
      <RegisterPage />
    </MemoryRouter>,
  )

test("scans a folder, lists apps and moves to the next after publishing", async () => {
  const user = userEvent.setup()
  const scan = vi.spyOn(api, "scan").mockResolvedValue({
    root_display: "C:\\apps",
    items: [
      scanItem(),
      scanItem({ folder_path: "/mnt/c/apps/Old", folder_name: "0. Old", suggest_skip: true, manifest: manifest({ name: "0. Old", kind: "reference" }) }),
      scanItem({ folder_path: "/mnt/c/apps/Sistema", folder_name: "Sistema", manifest: manifest({ name: "Sistema", kind: "web" }), similar: [{ name: "Projeto Sistema" }] }),
    ],
  })
  vi.spyOn(api, "saveApp").mockResolvedValue({ id: 5, folder_path: "/mnt/c/apps/Rentabilidade", folder_display: "", owner_id: 1, backup_id: null, manifest: manifest(), updated_at: "", can_run: false, version: null, folder_readable: true, remembered_folder: "" })

  renderPage()
  await user.type(screen.getByLabelText("Onde estão os apps?"), "C:\\apps")
  await user.click(screen.getByRole("button", { name: /Analisar/ }))

  expect(scan).toHaveBeenCalledWith("C:\\apps", "batch")
  expect(await screen.findByText("3 apps encontrados")).toBeInTheDocument()
  expect(screen.getByText("0 no catálogo · 2 para revisar · 1 pulados")).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: "Rentabilidade fundos" })).toBeInTheDocument()

  await user.click(screen.getByRole("button", { name: "Publicar no catálogo" }))
  expect(await screen.findByText("1 no catálogo · 1 para revisar · 1 pulados")).toBeInTheDocument()
  // The archive folder was skipped automatically, so the next pending app opens.
  expect(screen.getByRole("heading", { name: "Sistema" })).toBeInTheDocument()
  expect(screen.getByText(/Parece com/)).toBeInTheDocument()
})

test("single-app mode and scan errors", async () => {
  const user = userEvent.setup()
  const { ApiError } = await import("@/lib/api")
  const scan = vi.spyOn(api, "scan").mockRejectedValue(new ApiError(400, "Não encontrei essa pasta."))
  renderPage()
  await user.type(screen.getByLabelText("Onde estão os apps?"), "C:\\nada")
  await user.click(screen.getByRole("radio", { name: "Um app só" }))
  await user.click(screen.getByRole("button", { name: /Analisar/ }))
  expect(scan).toHaveBeenCalledWith("C:\\nada", "single")
  expect(await screen.findByRole("alert")).toHaveTextContent("Não encontrei essa pasta.")
})
