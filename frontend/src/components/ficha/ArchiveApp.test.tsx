import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { ArchiveApp } from "./ArchiveApp"

afterEach(() => vi.restoreAllMocks())

const mount = (shared = false) =>
  render(
    <MemoryRouter initialEntries={["/apps/7"]}>
      <Routes>
        <Route path="/apps/:id" element={<ArchiveApp appId={7} name="Notas" shared={shared} />} />
        <Route path="/apps" element={<p>catálogo</p>} />
      </Routes>
    </MemoryRouter>,
  )

test("Arquivar asks first and explains what happens", async () => {
  const archive = vi.spyOn(api, "archiveApp")
  mount(true)
  await userEvent.click(screen.getByRole("button", { name: "Arquivar" }))
  const dialog = await screen.findByRole("dialog")
  expect(dialog).toHaveTextContent("Arquivar o Notas?")
  expect(dialog).toHaveTextContent("agendamentos dele ficam desligados")
  expect(dialog).toHaveTextContent("versão no servidor é desligada")
  expect(dialog).toHaveTextContent("30 dias")
  expect(dialog).toHaveTextContent("A pasta do app no computador não é mexida")
  await userEvent.click(screen.getByRole("button", { name: "Cancelar" }))
  expect(archive).not.toHaveBeenCalled()
})

test("an app without a server version does not mention it", async () => {
  mount(false)
  await userEvent.click(screen.getByRole("button", { name: "Arquivar" }))
  expect(await screen.findByRole("dialog")).not.toHaveTextContent("servidor")
})

test("confirming archives and goes back to the catalog", async () => {
  const archive = vi.spyOn(api, "archiveApp").mockResolvedValue({
    id: 7, name: "Notas", summary: "", kind: "web", area: "Research", owner_name: "Henry",
    archived_at: "2026-11-09T12:00:00Z", archived_by_name: "Henry", purge_at: "2026-12-09T12:00:00Z", days_left: 30,
  })
  mount()
  await userEvent.click(screen.getByRole("button", { name: "Arquivar" }))
  const buttons = await screen.findAllByRole("button", { name: "Arquivar" })
  await userEvent.click(buttons[buttons.length - 1])
  expect(archive).toHaveBeenCalledWith(7)
  expect(await screen.findByText("catálogo")).toBeInTheDocument()
})

test("a refusal stays in the dialog", async () => {
  vi.spyOn(api, "archiveApp").mockRejectedValue(new ApiError(403, "Só o responsável pelo app ou um administrador pode arquivar ou trazer de volta."))
  mount()
  await userEvent.click(screen.getByRole("button", { name: "Arquivar" }))
  const buttons = await screen.findAllByRole("button", { name: "Arquivar" })
  await userEvent.click(buttons[buttons.length - 1])
  expect(await screen.findByRole("alert")).toHaveTextContent("Só o responsável")
  expect(screen.queryByText("catálogo")).not.toBeInTheDocument()
})
