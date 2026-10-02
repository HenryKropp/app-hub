import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type HistoryItem, type VersionInfo } from "@/lib/api"
import { HistoryTimeline } from "./HistoryTimeline"

afterEach(() => vi.restoreAllMocks())

const v = (over: Partial<VersionInfo> = {}): VersionInfo => ({
  number: 2, origin: "folder", restored_from: null, author_name: "Henry Kropp", author_probable: true, description: "",
  summary: { added: 1, removed: 0, modified: 2, binaries: ["Ferramenta CVM.exe"] }, files_changed_at: "2026-10-01T12:00:00Z",
  created_at: "2026-10-01T12:05:00Z", current: true, ...over,
})

const items: HistoryItem[] = [
  { type: "version", at: "2026-10-01T12:05:00Z", version: v() },
  { type: "run", at: "2026-10-01T11:00:00Z", run: { id: 7, status: "done", person_name: "Ana", version_number: 1 } },
  { type: "ficha", at: "2026-10-01T10:00:00Z", ficha: { person_name: "Henry", changes: [{ field: "owner", label: "Responsável", before: "Ana", after: "Henry" }, { field: "summary", label: "Resumo", before: "a", after: "b", long: true }] } },
  { type: "version", at: "2026-10-01T09:00:00Z", version: v({ number: 1, origin: "original", current: false, summary: { added: 3, removed: 0, modified: 0, binaries: [] } }) },
]

const renderIt = (canEdit = true) =>
  render(
    <MemoryRouter>
      <HistoryTimeline appId={3} canEdit={canEdit} />
    </MemoryRouter>,
  )

test("shows the three kinds of item in one list", async () => {
  vi.spyOn(api, "history").mockResolvedValue({ items, more: false, next_before: "" })
  renderIt()
  const list = await screen.findByRole("list", { name: "Histórico" })
  const rows = within(list).getAllByRole("listitem")
  expect(rows).toHaveLength(4)
  expect(rows[0]).toHaveTextContent("Versão 2")
  expect(rows[0]).toHaveTextContent("Mudança na pasta")
  expect(rows[0]).toHaveTextContent("provável: Henry Kropp")
  expect(rows[0]).toHaveTextContent("2 arquivos mudaram, 1 novo · Ferramenta CVM.exe mudou")
  expect(rows[1]).toHaveTextContent("Concluída")
  expect(rows[1]).toHaveTextContent("versão 1")
  expect(within(rows[1]).getByRole("link")).toHaveAttribute("href", "/execucoes/7")
  expect(rows[2]).toHaveTextContent("Responsável: Ana → Henry")
  expect(rows[2]).toHaveTextContent("Resumo alterado")
  expect(rows[3]).toHaveTextContent("Estado original")
  // Current version has no restore button; older one has.
  expect(within(rows[0]).queryByRole("button", { name: /Voltar o código/ })).toBeNull()
  expect(within(rows[3]).getByRole("button", { name: /Voltar o código/ })).toBeInTheDocument()
  expect(within(rows[0]).getByRole("link", { name: "Ver mudanças" })).toHaveAttribute("href", "/apps/3/versoes/2")
  expect(within(rows[0]).getByRole("link", { name: "Ver como estava" })).toHaveAttribute("href", "/apps/3/versoes/2?ver=arquivos")
})

test("filter and load more", async () => {
  const user = userEvent.setup()
  const spy = vi.spyOn(api, "history")
    .mockResolvedValueOnce({ items: items.slice(0, 2), more: true, next_before: "2026-10-01T11:00:00Z" })
    .mockResolvedValueOnce({ items: items.slice(2), more: false, next_before: "" })
    .mockResolvedValueOnce({ items: [items[0], items[3]], more: false, next_before: "" })
  renderIt()
  await user.click(await screen.findByRole("button", { name: "Carregar mais" }))
  expect(spy).toHaveBeenLastCalledWith(3, "all", "2026-10-01T11:00:00Z")
  expect(await screen.findAllByRole("listitem")).toHaveLength(4)
  await user.click(screen.getByRole("button", { name: "Código" }))
  expect(spy).toHaveBeenLastCalledWith(3, "code")
})

test("people without edit rights see no Descrever or Voltar", async () => {
  vi.spyOn(api, "history").mockResolvedValue({ items, more: false, next_before: "" })
  renderIt(false)
  await screen.findByRole("list", { name: "Histórico" })
  expect(screen.queryByRole("button", { name: "Descrever" })).toBeNull()
  expect(screen.queryByRole("button", { name: /Voltar o código/ })).toBeNull()
})

test("versionLine sums up the original state and shortens long binary lists", async () => {
  const { versionLine } = await import("./HistoryTimeline")
  expect(versionLine(v({ origin: "original", summary: { added: 4, removed: 0, modified: 0, binaries: ["a/x.ico", "y.exe"] } })))
    .toBe("Guardou 4 arquivos de código e 2 outros arquivos")
  expect(versionLine(v({ summary: { added: 0, removed: 0, modified: 1, binaries: ["a/x.ico", "y.exe", "b/z.zip", "w.zip"] } })))
    .toBe("1 arquivo mudou · x.ico mudou · y.exe mudou · mais 2 arquivos que não são código mudaram")
})

test("preview shows the newest items and opens the rest on request", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "history").mockResolvedValue({ items, more: false, next_before: "" })
  render(<MemoryRouter><HistoryTimeline appId={3} canEdit preview={2} /></MemoryRouter>)
  const list = await screen.findByRole("list", { name: "Histórico" })
  expect(within(list).getAllByRole("listitem")).toHaveLength(2)
  await user.click(screen.getByRole("button", { name: "Ver histórico completo" }))
  expect(within(list).getAllByRole("listitem")).toHaveLength(4)
})
