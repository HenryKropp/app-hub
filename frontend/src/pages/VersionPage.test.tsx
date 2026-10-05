import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { api, type AppDetail, type VersionInfo } from "@/lib/api"
import { ALL_CAN, manifest } from "@/test/fixtures"
import { VersionPage } from "./VersionPage"

afterEach(() => vi.restoreAllMocks())

const detail: AppDetail = {
  id: 3, folder_path: "/mnt/c/apps/fp", folder_display: "C:\\apps\\fp", owner_id: 1, backup_id: null, updated_at: "2026-09-30T10:00:00Z",
  manifest: manifest(), can_run: true, version: { number: 2, created_at: "2026-09-30T10:00:00Z" }, folder_readable: true, remembered_folder: "",
  area_id: 1, restricted: false, can: ALL_CAN,
}
beforeEach(() => { vi.spyOn(api, "app").mockResolvedValue(detail) })

const info: VersionInfo = {
  number: 2, origin: "folder", restored_from: null, author_name: "Henry Kropp", author_probable: true, description: "Troca do título",
  summary: { added: 0, removed: 0, modified: 1, binaries: ["Ferramenta CVM.exe"] }, files_changed_at: null, created_at: "2026-10-01T12:00:00Z", current: false,
}

const renderAt = (url: string) =>
  render(
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path="/apps/:id/versoes/:n" element={<VersionPage />} />
      </Routes>
    </MemoryRouter>,
  )

test("O que mudou shows red and green lines and binary sizes", async () => {
  vi.spyOn(api, "versionChanges").mockResolvedValue({
    version: info,
    files: [
      { path: "Código-fonte/Files Prep.py", status: "modified", kind: "code", old_size: 10, new_size: 12,
        hunks: [{ lines: [{ op: " ", text: "import os", old: 1, new: 1 }, { op: "-", text: "TITLE = 'Pipeline'", old: 2, new: 0 }, { op: "+", text: "TITLE = 'Files Prep'", old: 0, new: 2 }] }] },
      { path: "Ferramenta CVM.exe", status: "modified", kind: "binary", old_size: 79291437, new_size: 79500000, hunks: [] },
    ],
  })
  renderAt("/apps/3/versoes/2")
  expect(await screen.findByRole("heading", { name: /Versão 2/ })).toBeInTheDocument()
  expect(screen.getByText("Troca do título")).toBeInTheDocument()
  expect(screen.getByText("TITLE = 'Pipeline'").closest("[data-op]")).toHaveAttribute("data-op", "-")
  expect(screen.getByText("TITLE = 'Files Prep'").closest("[data-op]")).toHaveAttribute("data-op", "+")
  expect(screen.getByText("mudou (75,6 MB → 75,8 MB)")).toBeInTheDocument()
})

test("Como estava lists files and opens one", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "version").mockResolvedValue(info)
  vi.spyOn(api, "versionFiles").mockResolvedValue([
    { path: "Código-fonte/Files Prep.py", size: 12, kind: "code", modified: "2026-10-01T12:00:00Z" },
    { path: "Ferramenta CVM.exe", size: 79291437, kind: "binary", modified: "2026-08-13T17:54:00Z" },
  ])
  const content = vi.spyOn(api, "versionContent").mockResolvedValue({ path: "Código-fonte/Files Prep.py", text: "API = \"••••\"\nprint(1)\n" })
  renderAt("/apps/3/versoes/2?ver=arquivos")
  await user.click(await screen.findByRole("button", { name: "Código-fonte/Files Prep.py" }))
  expect(content).toHaveBeenCalledWith("3", "2", "Código-fonte/Files Prep.py")
  expect(await screen.findByText(/print\(1\)/)).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Ferramenta CVM.exe" })).toBeDisabled()
})

test("the original version explains itself instead of showing every line as new", async () => {
  vi.spyOn(api, "versionChanges").mockResolvedValue({
    version: { ...info, number: 1, origin: "original", summary: { added: 1, removed: 0, modified: 0, binaries: [] } },
    files: [{ path: "a.py", status: "added", kind: "code", old_size: 0, new_size: 5, hunks: [{ lines: [{ op: "+", text: "print(1)", old: 0, new: 1 }] }] }],
  })
  renderAt("/apps/3/versoes/1")
  expect(await screen.findByText(/Esta é a primeira versão/)).toBeInTheDocument()
  expect(screen.queryByText("print(1)")).not.toBeInTheDocument()
})
