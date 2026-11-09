import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, expect, it, vi } from "vitest"
import { api, type KeysList } from "@/lib/api"
import { KeysSection } from "./KeysSection"

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
