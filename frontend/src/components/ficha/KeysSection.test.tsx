import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api, type KeysList } from "@/lib/api"
import { KeysSection, keyName } from "./KeysSection"

beforeEach(() => vi.restoreAllMocks())

const list = (over: Partial<KeysList> = {}): KeysList => ({
  keys: [
    { name: "BTG_SENHA", label: "Senha do BTG", required: true, has_value: false, updated_by: "", updated_at: null },
    { name: "EMAIL", label: "E-mail", required: false, has_value: true, updated_by: "Henry", updated_at: "2026-11-09T12:00:00Z" },
  ],
  can_manage: true, ready: false, missing: ["BTG_SENHA"], locked: false, ...over,
})

it("fills an empty key and says the script waits for it", async () => {
  const get = vi.spyOn(api.keys, "list").mockResolvedValue(list())
  const set = vi.spyOn(api.keys, "set").mockResolvedValue({ ok: true })
  render(<KeysSection appId={7} />)
  expect(await screen.findByText(/não roda até preencher/)).toHaveTextContent("Senha do BTG")
  await userEvent.type(screen.getByLabelText("Senha do BTG"), "s3nha")
  await userEvent.click(screen.getByRole("button", { name: "Salvar Senha do BTG" }))
  expect(set).toHaveBeenCalledWith(7, "BTG_SENHA", "s3nha")
  await vi.waitFor(() => expect(get).toHaveBeenCalledTimes(2))
})

it("shows a saved value only after Mostrar, and hides it again", async () => {
  vi.spyOn(api.keys, "list").mockResolvedValue(list())
  const reveal = vi.spyOn(api.keys, "reveal").mockResolvedValue({ value: "henry@alis.com" })
  render(<KeysSection appId={7} />)
  expect(await screen.findByText(/Preenchida por Henry/)).toBeInTheDocument()
  expect(screen.queryByText("henry@alis.com")).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Mostrar E-mail" }))
  expect(reveal).toHaveBeenCalledWith(7, "EMAIL")
  expect(await screen.findByText("henry@alis.com")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Esconder E-mail" }))
  expect(screen.queryByText("henry@alis.com")).not.toBeInTheDocument()
})

it("lets someone who only uses the app see the state, not touch it", async () => {
  vi.spyOn(api.keys, "list").mockResolvedValue(list({ can_manage: false }))
  render(<KeysSection appId={7} />)
  expect(await screen.findByText(/Só o dono do app ou um administrador/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: /Mostrar/ })).not.toBeInTheDocument()
  expect(screen.queryByLabelText("Senha do BTG")).not.toBeInTheDocument()
})

it("says when the vault is locked on the server", async () => {
  vi.spyOn(api.keys, "list").mockResolvedValue(list({ locked: true }))
  render(<KeysSection appId={7} />)
  expect(await screen.findByText(/cofre está fechado/)).toBeInTheDocument()
})

it("turns what a person types into the name the script reads", () => {
  expect(keyName(" api token ")).toBe("API_TOKEN")
  expect(keyName("Senha do BTG")).toBe("SENHA_DO_BTG")
  expect(keyName("usuário-é")).toBe("USUARIO_E")
})

it("adds a key from an empty vault", async () => {
  const get = vi.spyOn(api.keys, "list").mockResolvedValue(list({ keys: [], missing: [], ready: true }))
  const set = vi.spyOn(api.keys, "set").mockResolvedValue({ ok: true })
  render(<KeysSection appId={7} />)
  expect(await screen.findByText(/Nenhuma chave ainda/)).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Adicionar chave" }))
  await userEvent.type(screen.getByLabelText("Nome"), "api token")
  expect(screen.getByText(/Vai ficar/)).toHaveTextContent("API_TOKEN")
  await userEvent.type(screen.getByLabelText("Valor"), "abc123")
  await userEvent.click(screen.getByRole("button", { name: "Salvar" }))
  expect(set).toHaveBeenCalledWith(7, "API_TOKEN", "abc123")
  await vi.waitFor(() => expect(get).toHaveBeenCalledTimes(2))
  expect(screen.queryByLabelText("Valor")).not.toBeInTheDocument()
})

it("does not offer Adicionar chave to who cannot manage the vault", async () => {
  vi.spyOn(api.keys, "list").mockResolvedValue(list({ can_manage: false }))
  render(<KeysSection appId={7} />)
  expect(await screen.findByText(/Só o dono do app/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Adicionar chave" })).not.toBeInTheDocument()
})

it("speaks of the app, not a script, for a screen app, and offers to restart the 24h version", async () => {
  vi.spyOn(api.keys, "list").mockResolvedValue(list())
  vi.spyOn(api.keys, "set").mockResolvedValue({ ok: true, needs_restart: true })
  render(<KeysSection appId={7} kind="web" />)
  expect(await screen.findByText(/O app não abre até preencher/)).toBeInTheDocument()
  expect(screen.getByText(/no lugar do .env/)).toBeInTheDocument()
  await userEvent.type(screen.getByLabelText("Senha do BTG"), "s3nha")
  await userEvent.click(screen.getByRole("button", { name: "Salvar Senha do BTG" }))
  expect(await screen.findByRole("button", { name: "Religar agora" })).toBeInTheDocument()
})
