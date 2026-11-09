import { act, fireEvent, render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { LandingPage } from "./LandingPage"

afterEach(() => vi.useRealTimers())

function renderLanding() {
  return render(<MemoryRouter><LandingPage /></MemoryRouter>)
}

test("public landing tells the product story without asking for company data", () => {
  renderLanding()
  expect(document.querySelector("form")).toBeNull()
  expect(document.querySelector("input")).toBeNull()
  expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Todos os apps da sua empresa, num lugar só.")
  expect(screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent)).toEqual([
    "Uma lista só, com busca, separada por área.",
    "Documentado em minutos, sem ninguém escrever manual.",
    "Do Claude para a empresa inteira.",
    "Comece pelos apps que já existem.",
  ])
  expect(screen.getAllByText("Exemplo ilustrativo. Os apps e as pessoas não são reais.")).toHaveLength(2)
  expect(screen.getByText("Para apps feitos com as ferramentas que o seu time já usa")).toBeInTheDocument()
  expect(screen.queryByText(/em breve/i)).toBeNull()
})

test("every way in goes to the same signup and login routes", () => {
  renderLanding()
  const create = screen.getAllByRole("link", { name: /Criar (o )?espaço/ })
  expect(create).toHaveLength(3)
  expect(create.every((link) => link.getAttribute("href") === "/cadastre-se")).toBe(true)
  const enter = screen.getAllByRole("link", { name: /Entrar/ })
  expect(enter).toHaveLength(3)
  expect(enter.every((link) => link.getAttribute("href") === "/acesso")).toBe(true)
  for (const [name, href] of [["Catálogo", "#catalogo"], ["Documentação", "#ficha"], ["Como funciona", "#como"], ["Integrações", "#integracoes"]]) {
    expect(screen.getByRole("link", { name })).toHaveAttribute("href", href)
    expect(document.getElementById(href.slice(1))).not.toBeNull()
  }
})

test("how it works advances on its own and jumps to a clicked step", () => {
  vi.useFakeTimers()
  renderLanding()
  const step = (name: RegExp) => screen.getByRole("button", { name })
  expect(step(/O time faz o app/)).toHaveAttribute("aria-current", "step")
  act(() => { vi.advanceTimersByTime(3500) })
  expect(step(/Pede para subir/)).toHaveAttribute("aria-current", "step")
  expect(step(/O time faz o app/)).not.toHaveAttribute("aria-current")
  fireEvent.click(step(/A empresa usa/))
  expect(step(/A empresa usa/)).toHaveAttribute("aria-current", "step")
  act(() => { vi.advanceTimersByTime(3500) })
  expect(step(/O time faz o app/)).toHaveAttribute("aria-current", "step")
})
