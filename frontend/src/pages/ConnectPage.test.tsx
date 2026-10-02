import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError } from "@/lib/api"
import { ConnectPage } from "./ConnectPage"

afterEach(() => vi.restoreAllMocks())
const renderAt = (url: string) =>
  render(
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path="/conectar" element={<ConnectPage />} />
        <Route path="/configuracoes" element={<p>página do computador</p>} />
      </Routes>
    </MemoryRouter>,
  )

test("approves the computer", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "pairing").mockResolvedValue({ code: "ABCD2345", computer_name: "NOTE-HENRY", expired: false, approved: false })
  const approve = vi.spyOn(api, "approvePairing").mockResolvedValue(undefined)
  renderAt("/conectar?codigo=ABCD2345")
  expect(await screen.findByText("NOTE-HENRY")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Conectar" }))
  expect(approve).toHaveBeenCalledWith("ABCD2345")
  expect(await screen.findByText("página do computador")).toBeInTheDocument()
})

test("expired code", async () => {
  vi.spyOn(api, "pairing").mockResolvedValue({ code: "X", computer_name: "PC", expired: true, approved: false })
  renderAt("/conectar?codigo=X")
  expect(await screen.findByText(/venceu/)).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Conectar" })).not.toBeInTheDocument()
})

test("unknown code", async () => {
  vi.spyOn(api, "pairing").mockRejectedValue(new ApiError(404, "Não encontrei esse pedido de conexão. Abra o Agente de novo."))
  renderAt("/conectar?codigo=Z")
  expect(await screen.findByRole("alert")).toHaveTextContent("Não encontrei esse pedido")
})
