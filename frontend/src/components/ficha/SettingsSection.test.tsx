import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api, type SettingsList } from "@/lib/api"
import { SettingsSection, keyName } from "./SettingsSection"

beforeEach(() => vi.restoreAllMocks())

const list = (over: Partial<SettingsList> = {}): SettingsList => ({
  items: [
    { name: "BTG_SENHA", label: "Senha do BTG", secret: true, required: true, has_value: false, updated_by: "", updated_at: null },
    { name: "EMAIL", label: "E-mail", secret: true, required: false, has_value: true, updated_by: "Henry", updated_at: "2026-11-09T12:00:00Z" },
    { name: "PASTA", label: "", secret: false, required: false, has_value: true, value: "C:\\Relatórios", updated_by: "Henry", updated_at: "2026-11-09T12:00:00Z" },
  ],
  can_manage: true, missing: ["Senha do BTG"], locked: false, ...over,
})

const block = (title: string) => screen.getByRole("heading", { name: title }).closest("section")!

it("shows the two blocks and says what is missing", async () => {
  vi.spyOn(api.settings, "list").mockResolvedValue(list())
  render(<SettingsSection appId={7} />)
  expect(await screen.findByText(/Falta preencher/)).toHaveTextContent("Senha do BTG")
  expect(within(block("Chaves e senhas")).getByText("E-mail")).toBeInTheDocument()
  expect(within(block("Outros ajustes")).getByLabelText("PASTA")).toHaveValue("C:\\Relatórios")
  expect(screen.queryByText(/cofre/i)).not.toBeInTheDocument()
})

it("fills an empty secret", async () => {
  const get = vi.spyOn(api.settings, "list").mockResolvedValue(list())
  const set = vi.spyOn(api.keys, "set").mockResolvedValue({ ok: true })
  const onChanged = vi.fn()
  render(<SettingsSection appId={7} onChanged={onChanged} />)
  await userEvent.type(await screen.findByLabelText("Senha do BTG"), "s3nha")
  await userEvent.click(screen.getByRole("button", { name: "Salvar Senha do BTG" }))
  expect(set).toHaveBeenCalledWith(7, "BTG_SENHA", "s3nha")
  await vi.waitFor(() => expect(get).toHaveBeenCalledTimes(2))
  expect(onChanged).toHaveBeenCalled()
})

it("shows a saved secret only after Mostrar, and hides it again", async () => {
  vi.spyOn(api.settings, "list").mockResolvedValue(list())
  const reveal = vi.spyOn(api.keys, "reveal").mockResolvedValue({ value: "henry@alis.com" })
  render(<SettingsSection appId={7} />)
  expect((await screen.findAllByText(/Preenchido por Henry/)).length).toBe(2)
  expect(screen.queryByText("henry@alis.com")).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Mostrar E-mail" }))
  expect(reveal).toHaveBeenCalledWith(7, "EMAIL")
  expect(await screen.findByText("henry@alis.com")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Esconder E-mail" }))
  expect(screen.queryByText("henry@alis.com")).not.toBeInTheDocument()
})

it("edits and clears a plain value", async () => {
  const get = vi.spyOn(api.settings, "list").mockResolvedValue(list())
  const set = vi.spyOn(api.settings, "set").mockResolvedValue({ ok: true })
  const remove = vi.spyOn(api.settings, "remove").mockResolvedValue({ ok: true })
  vi.spyOn(window, "confirm").mockReturnValue(true)
  render(<SettingsSection appId={7} />)
  const field = await screen.findByLabelText("PASTA")
  await userEvent.clear(field)
  await userEvent.type(field, "D:\\Saída")
  await userEvent.click(screen.getByRole("button", { name: "Salvar PASTA" }))
  expect(set).toHaveBeenCalledWith(7, "PASTA", "D:\\Saída")
  await userEvent.click(screen.getByRole("button", { name: "Apagar PASTA" }))
  expect(remove).toHaveBeenCalledWith(7, "PASTA")
  await vi.waitFor(() => expect(get).toHaveBeenCalledTimes(3))
})

it("lets someone who only uses the app see the state, not touch it", async () => {
  vi.spyOn(api.settings, "list").mockResolvedValue(list({ can_manage: false, items: list().items.map((i) => ({ ...i, value: undefined })) }))
  render(<SettingsSection appId={7} />)
  expect(await screen.findByText(/Só o dono do app ou um administrador/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: /Mostrar/ })).not.toBeInTheDocument()
  expect(screen.queryByLabelText("Senha do BTG")).not.toBeInTheDocument()
  expect(screen.queryByLabelText("PASTA")).not.toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Adicionar valor" })).not.toBeInTheDocument()
})

it("says when the secrets are locked on the server", async () => {
  vi.spyOn(api.settings, "list").mockResolvedValue(list({ locked: true }))
  render(<SettingsSection appId={7} />)
  expect(await screen.findByText(/senhas estão trancadas/)).toBeInTheDocument()
  expect(screen.queryByLabelText("Senha do BTG")).not.toBeInTheDocument()
  expect(screen.getByLabelText("PASTA")).toBeInTheDocument()
})

it("turns what a person types into the name the app reads", () => {
  expect(keyName(" api token ")).toBe("API_TOKEN")
  expect(keyName("Senha do BTG")).toBe("SENHA_DO_BTG")
  expect(keyName("usuário-é")).toBe("USUARIO_E")
})

it("adds a secret by default, and a plain value when the box is cleared", async () => {
  const get = vi.spyOn(api.settings, "list").mockResolvedValue(list({ items: [], missing: [] }))
  const setKey = vi.spyOn(api.keys, "set").mockResolvedValue({ ok: true })
  const setPlain = vi.spyOn(api.settings, "set").mockResolvedValue({ ok: true })
  render(<SettingsSection appId={7} />)
  expect(await screen.findByText(/Nenhuma senha ainda/)).toBeInTheDocument()
  expect(screen.getByText("Nenhum ajuste ainda.")).toBeInTheDocument()

  await userEvent.click(screen.getByRole("button", { name: "Adicionar valor" }))
  await userEvent.type(screen.getByLabelText("Nome"), "api token")
  expect(screen.getByText(/Vai ficar/)).toHaveTextContent("API_TOKEN")
  await userEvent.type(screen.getByLabelText("Valor"), "abc123")
  expect(screen.getByLabelText("Valor")).toHaveAttribute("type", "password")
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(setKey).toHaveBeenCalledWith(7, "API_TOKEN", "abc123")
  await vi.waitFor(() => expect(get).toHaveBeenCalledTimes(2))

  await userEvent.click(screen.getByRole("button", { name: "Adicionar valor" }))
  await userEvent.type(screen.getByLabelText("Nome"), "pasta")
  await userEvent.click(screen.getByRole("checkbox", { name: /É uma senha/ }))
  expect(screen.getByLabelText("Valor")).toHaveAttribute("type", "text")
  await userEvent.type(screen.getByLabelText("Valor"), "C:\\Dados")
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(setPlain).toHaveBeenCalledWith(7, "PASTA", "C:\\Dados")
})

it("speaks of the app for a screen app, and offers to restart the 24h version", async () => {
  vi.spyOn(api.settings, "list").mockResolvedValue(list())
  vi.spyOn(api.keys, "set").mockResolvedValue({ ok: true, needs_restart: true })
  render(<SettingsSection appId={7} kind="web" />)
  expect(await screen.findByText(/no seu computador e na versão ligada no servidor/)).toBeInTheDocument()
  await userEvent.type(screen.getByLabelText("Senha do BTG"), "s3nha")
  await userEvent.click(screen.getByRole("button", { name: "Salvar Senha do BTG" }))
  expect(await screen.findByRole("button", { name: "Religar agora" })).toBeInTheDocument()
})

it("turns a plain value into a password after a confirmation", async () => {
  const get = vi.spyOn(api.settings, "list").mockResolvedValue(list())
  const make = vi.spyOn(api.settings, "makeSecret").mockResolvedValue({ ok: true })
  vi.spyOn(window, "confirm").mockReturnValue(true)
  render(<SettingsSection appId={7} />)
  await userEvent.click(await screen.findByRole("button", { name: "Tornar PASTA uma senha" }))
  expect(make).toHaveBeenCalledWith(7, "PASTA")
  await vi.waitFor(() => expect(get).toHaveBeenCalledTimes(2))
})
