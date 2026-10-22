import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { detail, manifest } from "@/test/fixtures"
import { RunDialog } from "./RunDialog"

afterEach(() => vi.restoreAllMocks())

test("loads script inputs, excludes window-owned fields, validates required values, and submits manifest values", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail({ manifest: manifest({
    inputs: [
      { id: "periodo", label: "Período", type: "text", source: "argument", flag: "--periodo", required: true },
      { id: "janela", label: "Janela própria", type: "text", source: "window", required: false },
    ],
  }) }))
  const confirm = vi.fn()
  render(<RunDialog target={{ id: 3, name: "Relatório", folder_display: "C:\\apps", remembered_folder: "", kind: "script" }} onCancel={vi.fn()} onConfirm={confirm} />)

  expect(await screen.findByLabelText("Período")).toBeInTheDocument()
  expect(screen.queryByLabelText("Janela própria")).not.toBeInTheDocument()
  await user.type(screen.getByLabelText("Onde salvar os resultados?"), "D:\\Saídas")
  await user.click(screen.getByRole("button", { name: "Executar script" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("Preencha “Período”")
  expect(confirm).not.toHaveBeenCalled()

  await user.type(screen.getByLabelText("Período"), "2026-10")
  await user.click(screen.getByRole("button", { name: "Executar script" }))
  expect(confirm).toHaveBeenCalledWith({
    folder: "D:\\Saídas",
    remember: true,
    inputs: { periodo: "2026-10" },
  })
})

test("limits the total selected file size and keeps the form open on errors", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "app").mockResolvedValue(detail({ manifest: manifest({
    inputs: [{ id: "entrada", label: "Arquivo de entrada", type: "file", source: "fixed_file", path: "dados.csv", required: true }],
  }) }))
  const confirm = vi.fn()
  render(<RunDialog target={{ id: 3, name: "Relatório", folder_display: "C:\\apps", remembered_folder: "", kind: "script" }} onCancel={vi.fn()} onConfirm={confirm} />)
  const input = await screen.findByLabelText("Arquivo de entrada")
  const largeFile = new File([new Uint8Array(10 * 1024 * 1024 + 1)], "dados.csv", { type: "text/csv" })
  await user.upload(input, largeFile)
  await user.type(screen.getByLabelText("Onde salvar os resultados?"), "D:\\Saídas")
  await user.click(screen.getByRole("button", { name: "Executar script" }))
  expect(await screen.findByRole("alert")).toHaveTextContent("passam de 10 MB")
  expect(confirm).not.toHaveBeenCalled()
})
