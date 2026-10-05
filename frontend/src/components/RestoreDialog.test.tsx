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
  // The footer button; the corner X is also named Fechar.
  await user.click(screen.getAllByRole("button", { name: "Fechar" }).at(-1)!)
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

test("explains files without a copy that go to the trash before coming back", async () => {
  vi.spyOn(api, "restorePreview").mockResolvedValue({
    plan: { rewrite: ["dados.json"], recreate: [], to_trash: ["dados.json"], binaries: [] }, current: 3,
  })
  render(<RestoreDialog appId={3} number={1} open onOpenChange={vi.fn()} onDone={vi.fn()} />)
  expect(await screen.findByText("Arquivos que ainda não têm cópia no Hub vão para a lixeira antes de voltar.")).toBeInTheDocument()
  expect(screen.getAllByText("dados.json")).toHaveLength(2)
})

test("no explanation when the trash only has extra files", async () => {
  vi.spyOn(api, "restorePreview").mockResolvedValue({ plan, current: 3 })
  render(<RestoreDialog appId={3} number={1} open onOpenChange={vi.fn()} onDone={vi.fn()} />)
  await screen.findByText("Vão para a lixeira")
  expect(screen.queryByText(/ainda não têm cópia/)).not.toBeInTheDocument()
})
