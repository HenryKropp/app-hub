import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { expect, test } from "vitest"
import { LandingPage } from "./LandingPage"

test("public landing tells the product story without asking for company data", () => {
  render(<MemoryRouter><LandingPage /></MemoryRouter>)
  expect(document.querySelector("form")).toBeNull()
  expect(document.querySelector("input")).toBeNull()
  expect(screen.getAllByRole("link", { name: "Entrar" }).every((link) => link.getAttribute("href") === "/acesso")).toBe(true)
  const create = screen.getAllByRole("link", { name: /Criar (o )?espaço/ })
  expect(create).toHaveLength(3)
  expect(create.every((link) => link.getAttribute("href") === "/cadastre-se")).toBe(true)
  expect(screen.getByText(/Recebeu um convite\? Use/)).toBeInTheDocument()
  expect(screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual([
    "Sobe em minutos, já documentado.",
    "Qualquer pessoa roda, sem chamar quem fez.",
    "Quebrou? Volte para a versão que funcionava.",
    "Os painéis que a IA gera também ficam guardados.",
  ])
  expect(screen.getByText(/Exemplo ilustrativo/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Como funciona" })).toHaveAttribute("href", "#como-funciona")
  expect(screen.queryByText(/Aurora Studio/)).toBeNull()
})
