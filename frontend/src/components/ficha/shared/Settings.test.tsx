import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api } from "@/lib/api"
import { SharedSettingsForm } from "./Settings"

beforeEach(() => vi.restoreAllMocks())

it("never shows a saved secret and can replace it", async () => {
  vi.spyOn(api.shared, "config").mockResolvedValue([
    { name: "SMTP_PASSWORD", label: "Senha do e-mail", secret: true, required: true, default: "", has_value: true, value: "" },
    { name: "ADMIN_EMAILS", label: "E-mails dos administradores", secret: false, required: true, default: "", has_value: true, value: "henry@alis.com" },
  ])
  const set = vi.spyOn(api.shared, "setConfig").mockResolvedValue({ needs_restart: true })
  render(<SharedSettingsForm appId={1} />)
  expect(await screen.findByText("Guardada")).toBeInTheDocument()
  expect(screen.getByLabelText("E-mails dos administradores")).toHaveValue("henry@alis.com")
  await userEvent.click(screen.getByRole("button", { name: "Trocar Senha do e-mail" }))
  await userEvent.type(screen.getByLabelText("Senha do e-mail"), "nova")
  await userEvent.click(screen.getByRole("button", { name: "Salvar Senha do e-mail" }))
  expect(set).toHaveBeenCalledWith(1, "SMTP_PASSWORD", "nova")
  expect(await screen.findByText("Salvo. Religue o app para valer.")).toBeInTheDocument()
})

it("says when the app asks for nothing", async () => {
  vi.spyOn(api.shared, "config").mockResolvedValue([])
  render(<SharedSettingsForm appId={1} />)
  expect(await screen.findByText("Este app não pede configurações.")).toBeInTheDocument()
})

it("declares a new setting through the given save", async () => {
  const cfg = vi.spyOn(api.shared, "config").mockResolvedValue([])
  const onAdd = vi.fn().mockResolvedValue(undefined)
  render(<SharedSettingsForm appId={1} onAdd={onAdd} />)
  await userEvent.click(await screen.findByRole("button", { name: "Adicionar configuração" }))
  await userEvent.type(screen.getByLabelText("Nome no código"), "API_KEY")
  await userEvent.type(screen.getByLabelText("Nome para as pessoas"), "Chave da API")
  await userEvent.click(screen.getByRole("checkbox", { name: "É uma senha" }))
  await userEvent.click(screen.getByRole("button", { name: "Adicionar" }))
  expect(onAdd).toHaveBeenCalledWith({ name: "API_KEY", label: "Chave da API", secret: true, required: false })
  await vi.waitFor(() => expect(cfg).toHaveBeenCalledTimes(2))
})
