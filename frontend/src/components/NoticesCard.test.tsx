import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type Notice } from "@/lib/api"
import { NoticesCard } from "./NoticesCard"

afterEach(() => vi.restoreAllMocks())

const notice = (o: Partial<Notice> = {}): Notice => ({
  id: 1, app_id: 4, app_name: "Relatórios", version: 3, text: "Relatórios versão 3 no ar — testada: 2 arquivos salvos",
  created_at: new Date().toISOString(), read_at: null, ...o,
})

const mount = () => render(<MemoryRouter><NoticesCard /></MemoryRouter>)

test("hidden without unread notices", async () => {
  const list = vi.spyOn(api, "notices").mockResolvedValue([notice({ read_at: new Date().toISOString() })])
  const { container } = mount()
  await waitFor(() => expect(list).toHaveBeenCalled())
  expect(container).toBeEmptyDOMElement()
})

test("shows unread notices and marks them as read", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "notices").mockResolvedValue([notice()])
  const read = vi.spyOn(api, "readNotice").mockResolvedValue(undefined)
  mount()
  expect(await screen.findByText("Relatórios versão 3 no ar — testada: 2 arquivos salvos")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: /Marcar como lido/ }))
  expect(read).toHaveBeenCalledWith(1)
  await waitFor(() => expect(screen.queryByText(/versão 3 no ar/)).not.toBeInTheDocument())
})
