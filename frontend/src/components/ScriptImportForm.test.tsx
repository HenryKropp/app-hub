import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
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
  expect(request.confirm_secrets).toBe(false)
})

async function prepareWarning() {
  const user = userEvent.setup()
  vi.spyOn(api, "areas").mockResolvedValue([{ id: 6, name: "Pesquisa", app_count: 0, leaders: [], members: [] }])
  const register = vi.spyOn(api, "importScript").mockRejectedValueOnce(new ApiError(409, "Possíveis senhas ou chaves no código.", {
    code: "script_secrets_confirmation_required", secret_count: 2,
  })).mockResolvedValue({ id: 42 })
  render(<MemoryRouter><ScriptImportForm /></MemoryRouter>)
  await user.type(screen.getByLabelText("Nome do app"), "Resumo mensal")
  const source = new File(["password = 'FictitiousValue987'"], "main.py", { type: "text/x-python" })
  await user.upload(screen.getByLabelText("Arquivos do script"), source)
  await user.selectOptions(screen.getByLabelText("Arquivo que inicia o app"), "main.py")
  await user.click(screen.getByRole("button", { name: "Registrar script" }))
  await screen.findByText("Possíveis senhas ou chaves no código: 2 ocorrência(s).")
  return { user, register, source }
}

test("requires explicit consent, shows no source values and resends unchanged code only after confirmation", async () => {
  const { user, register } = await prepareWarning()
  expect(register).toHaveBeenCalledTimes(1)
  expect(screen.getByRole("button", { name: "Registrar script" })).toBeDisabled()
  expect(screen.getByText(/Nada foi registrado/)).toHaveTextContent("falsos positivos")
  expect(document.body.textContent).not.toContain("FictitiousValue987")
  await user.click(screen.getByLabelText(/Revisei os arquivos e aceito/))
  await user.click(screen.getByRole("button", { name: "Registrar script" }))
  await waitFor(() => expect(register).toHaveBeenCalledTimes(2))
  expect(register.mock.calls[0][0].confirm_secrets).toBe(false)
  expect(register.mock.calls[1][0].confirm_secrets).toBe(true)
  expect(register.mock.calls[1][0].files).toEqual(register.mock.calls[0][0].files)
})

test("cancelling confirmation never registers and clears consent for the next attempt", async () => {
  const { user, register } = await prepareWarning()
  await user.click(screen.getByLabelText(/Revisei os arquivos e aceito/))
  await user.click(screen.getByRole("button", { name: "Cancelar confirmação" }))
  expect(register).toHaveBeenCalledTimes(1)
  expect(screen.queryByLabelText(/Revisei os arquivos e aceito/)).not.toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Registrar script" }))
  await waitFor(() => expect(register).toHaveBeenCalledTimes(2))
  expect(register.mock.calls[1][0].confirm_secrets).toBe(false)
})

test("replacing files invalidates consent even if their names are unchanged", async () => {
  const { user, register } = await prepareWarning()
  await user.click(screen.getByLabelText(/Revisei os arquivos e aceito/))
  await user.upload(screen.getByLabelText("Arquivos do script"), new File(["print('fixed')"], "main.py"))
  expect(screen.queryByLabelText(/Revisei os arquivos e aceito/)).not.toBeInTheDocument()
  await user.selectOptions(screen.getByLabelText("Arquivo que inicia o app"), "main.py")
  await user.click(screen.getByRole("button", { name: "Registrar script" }))
  await waitFor(() => expect(register).toHaveBeenCalledTimes(2))
  expect(register.mock.calls[1][0].confirm_secrets).toBe(false)
  expect(register.mock.calls[1][0].files).not.toEqual(register.mock.calls[0][0].files)
})

test("removing a file clears consent", async () => {
  const { user, register } = await prepareWarning()
  await user.click(screen.getByLabelText(/Revisei os arquivos e aceito/))
  await user.click(screen.getByRole("button", { name: "Remover main.py" }))
  expect(screen.queryByLabelText(/Revisei os arquivos e aceito/)).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Registrar script" })).toBeDisabled()
  expect(register).toHaveBeenCalledTimes(1)
})

test("a non-confirmable error cannot be accepted as a secrets warning", async () => {
  const { user, register } = await prepareWarning()
  register.mockRejectedValueOnce(new ApiError(400, "Não envie arquivos de credenciais (.env, .pem, .key)."))
  await user.click(screen.getByLabelText(/Revisei os arquivos e aceito/))
  await user.click(screen.getByRole("button", { name: "Registrar script" }))
  expect(await screen.findByText("Não envie arquivos de credenciais (.env, .pem, .key).")).toBeInTheDocument()
})
