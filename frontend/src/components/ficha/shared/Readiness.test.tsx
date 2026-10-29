import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { expect, it, vi } from "vitest"
import { ReadinessList } from "./Readiness"
import type { Readiness } from "@/lib/api"

const recipe = (kind: string): Readiness["recipe"] => ({ kind, command: "", dockerfile: "", port: 0, needs_confirm: false, requirements: null })

const rep: Readiness = { ready: false, recipe: recipe("streamlit"), wants_db: false, suggest_env: [], items: [
  { key: "recipe", status: "auto", title: "Jeito de rodar", detail: "Streamlit, o Ziv monta sozinho.", claude_text: "", acked: false },
  { key: "port", status: "missing", title: "Porta", detail: "O app não lê a porta que o Ziv dá.", claude_text: "Faça o app ler PORT.", acked: false },
  { key: "file_data", status: "warn", title: "Dados em arquivo", detail: "Grava notas.json na pasta.", claude_text: "Use APPHUB_DATA_DIR.", acked: false },
]}

it("lists items in plain words with the Claude text to copy", async () => {
  const onAck = vi.fn()
  render(<ReadinessList report={rep} acks={[]} onAck={onAck} />)
  expect(screen.getByText("Porta")).toBeInTheDocument()
  expect(screen.getByText("Streamlit, o Ziv monta sozinho.")).toBeInTheDocument()
  const box = screen.getByRole("textbox", { name: "Texto para o Claude" })
  expect((box as HTMLTextAreaElement).value).toContain("Faça o app ler PORT.")
  await userEvent.click(screen.getByRole("checkbox", { name: /estou ciente/i }))
  expect(onAck).toHaveBeenCalledWith(["file_data"])
})

it("unmarks an item the person had marked", async () => {
  const onAck = vi.fn()
  const acked = { ...rep, items: rep.items.map((i) => (i.key === "file_data" ? { ...i, acked: true } : i)) }
  render(<ReadinessList report={acked} acks={["file_data"]} onAck={onAck} />)
  await userEvent.click(screen.getByRole("checkbox", { name: /estou ciente/i }))
  expect(onAck).toHaveBeenCalledWith([])
})

it("says it is ready when nothing is missing", () => {
  render(<ReadinessList report={{ ready: true, recipe: recipe("flask"), wants_db: false, suggest_env: [], items: [] }} acks={[]} onAck={() => {}} />)
  expect(screen.getByText("Tudo certo para publicar.")).toBeInTheDocument()
  expect(screen.queryByRole("textbox", { name: "Texto para o Claude" })).not.toBeInTheDocument()
})

it("checks again on request", async () => {
  const onRecheck = vi.fn()
  render(<ReadinessList report={rep} acks={[]} onAck={() => {}} onRecheck={onRecheck} />)
  await userEvent.click(screen.getByRole("button", { name: "Conferir de novo" }))
  expect(onRecheck).toHaveBeenCalled()
})
