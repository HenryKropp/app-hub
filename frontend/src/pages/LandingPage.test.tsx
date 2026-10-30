import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { expect, test } from "vitest"
import { LandingPage } from "./LandingPage"

test("public landing tells the product story without asking for company data", () => {
  render(<MemoryRouter><LandingPage /></MemoryRouter>)
  expect(document.querySelector("form")).toBeNull()
  expect(document.querySelector("input")).toBeNull()
  expect(screen.getAllByRole("link", { name: "Login" })[0]).toHaveAttribute("href", "/acesso")
  expect(screen.getAllByRole("link", { name: "Login / Cadastre-se" }).every((link) => link.getAttribute("href") === "/acesso")).toBe(true)
  expect(screen.getAllByRole("link", { name: "Cadastre-se" }).every((link) => link.getAttribute("href") === "/cadastre-se")).toBe(true)
  expect(screen.getByText(/Exemplo ilustrativo da biblioteca de ferramentas/)).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Como funciona" })).toHaveAttribute("href", "#como-funciona")
})
