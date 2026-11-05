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
  expect(await screen.findByRole("heading", { name: "Não encontramos esse pedido de conexão" })).toBeInTheDocument()
  expect(screen.getByRole("alert")).toHaveTextContent("Abra o Agente do Ziv")
})

test("no code shows what to do next", async () => {
  vi.spyOn(api, "pairing").mockRejectedValue(new ApiError(400, "x"))
  renderAt("/conectar")
  expect(await screen.findByText("Não encontramos esse pedido de conexão")).toBeInTheDocument()
  expect(screen.getByText(/Abra o Agente do Ziv no seu computador de novo/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Ir para Configurações" })).toHaveAttribute("href", "/configuracoes")
  expect(screen.getByRole("link", { name: "Como instalar o Agente" })).toHaveAttribute("href", "/comecar#computador")
})

test("approve failure keeps its message and the two links", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "pairing").mockResolvedValue({ code: "A", computer_name: "PC", expired: false, approved: false })
  vi.spyOn(api, "approvePairing").mockRejectedValue(new Error("Falhou ao conectar."))
  renderAt("/conectar?codigo=A")
  await user.click(await screen.findByRole("button", { name: "Conectar" }))
  expect(await screen.findByText("Falhou ao conectar.")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Ir para Configurações" })).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Como instalar o Agente" })).toBeInTheDocument()
})

test("a server failure shows its own message, not the not-found text", async () => {
  vi.spyOn(api, "pairing").mockRejectedValue(new ApiError(503, "O Ziv não respondeu."))
  renderAt("/conectar?codigo=ABC")
  expect(await screen.findByText("O Ziv não respondeu.")).toBeInTheDocument()
  expect(screen.queryByText(/Não encontramos/)).not.toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Como instalar o Agente" })).toBeInTheDocument()
})
