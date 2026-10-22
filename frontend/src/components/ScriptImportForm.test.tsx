import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { ScriptImportForm } from "./ScriptImportForm"

afterEach(() => vi.restoreAllMocks())

test("uploads a trusted script package with its selected entrypoint and navigates after registration", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "areas").mockResolvedValue([{ id: 6, name: "Pesquisa", app_count: 0, leaders: [], members: [] }])
  const register = vi.spyOn(api, "importScript").mockResolvedValue({ id: 42 })
  render(<MemoryRouter><ScriptImportForm /></MemoryRouter>)

  await user.type(screen.getByLabelText("Nome do app"), "Resumo mensal")
  await user.type(screen.getByLabelText("Resumo"), "Gera um resumo mensal.")
  const source = new File(["print('ok')"], "main.py", { type: "text/x-python" })
  await user.upload(screen.getByLabelText("Arquivos do script"), source)
  await user.selectOptions(screen.getByLabelText("Arquivo que inicia o app"), "main.py")
  await user.click(screen.getByRole("button", { name: "Registrar script" }))

  await waitFor(() => expect(register).toHaveBeenCalledTimes(1))
  expect(register).toHaveBeenCalledTimes(1)
  const request = register.mock.calls[0][0]
  expect(request.manifest).toMatchObject({
    name: "Resumo mensal",
    summary: "Gera um resumo mensal.",
    kind: "script",
    language: "python",
    entrypoint: "main.py",
    area: "Pesquisa",
  })
  expect(request.files).toEqual({ "main.py": { name: "main.py", content_base64: "cHJpbnQoJ29rJyk=" } })
})
