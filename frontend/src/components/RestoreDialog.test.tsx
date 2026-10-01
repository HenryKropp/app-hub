import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { RestoreDialog } from "./RestoreDialog"

afterEach(() => vi.restoreAllMocks())

const plan = { rewrite: ["a.py"], recreate: [], to_trash: ["b.py"], binaries: [] }

async function setup() {
  vi.spyOn(api, "restorePreview").mockResolvedValue({ plan, current: 3 })
  const onDone = vi.fn()
  const onOpenChange = vi.fn()
  render(<RestoreDialog appId={3} number={1} open onOpenChange={onOpenChange} onDone={onDone} />)
  const user = userEvent.setup()
  await user.click(await screen.findByRole("button", { name: "Voltar o código" }))
  return { user, onDone, onOpenChange }
}

test("success stays visible and onDone fires only on Fechar", async () => {
  vi.spyOn(api, "restore").mockResolvedValue({ number: 4, done: ["a.py"], failed: [] })
  const { user, onDone } = await setup()
  expect(await screen.findByText(/Pronto\. O código voltou para a versão 1/)).toBeInTheDocument()
  expect(onDone).not.toHaveBeenCalled()
  await user.click(screen.getByRole("button", { name: "Fechar" }))
  expect(onDone).toHaveBeenCalledTimes(1)
})

test("partial failure shows Parei no meio with lists", async () => {
  vi.spyOn(api, "restore").mockResolvedValue({ number: 4, done: ["a.py"], failed: [{ path: "b.py", error: "sem permissão" }] })
  await setup()
  expect(await screen.findByText(/Parei no meio/)).toBeInTheDocument()
  expect(screen.getByText("a.py")).toBeInTheDocument()
  expect(screen.getByText("b.py (sem permissão)")).toBeInTheDocument()
})

test("409 with body shows the error and the lists", async () => {
  vi.spyOn(api, "restore").mockRejectedValue(
    new ApiError(409, "Voltei parte do código, mas não consegui conferir a pasta depois.", {
      number: 0, done: ["a.py"], failed: [{ path: "b.py", error: "travado" }], error: "Voltei parte do código, mas não consegui conferir a pasta depois.",
    }),
  )
  await setup()
  expect(await screen.findByText(/não consegui conferir a pasta depois/)).toBeInTheDocument()
  expect(screen.getByText("a.py")).toBeInTheDocument()
  expect(screen.getByText("b.py (travado)")).toBeInTheDocument()
})
