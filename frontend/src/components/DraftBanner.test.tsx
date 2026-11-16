import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { expect, test } from "vitest"
import { DraftBanner } from "./DraftBanner"

const mount = (stage: string, label: string) =>
  render(
    <MemoryRouter>
      <DraftBanner appId={4} draft={{ id: 9, stage, stage_label: label }} />
    </MemoryRouter>,
  )

test("waiting confirmation links to the plan", () => {
  mount("esperando_confirmacao", "esperando confirmação do plano de teste")
  expect(screen.getByText("Versão nova em teste: esperando confirmação do plano de teste")).toBeInTheDocument()
  expect(screen.getByRole("link", { name: "Confirmar plano de teste" })).toHaveAttribute("href", "/apps/4/plano-de-teste")
})

test("other stages show only the label", () => {
  mount("rodando", "testando no PC — 2 min")
  expect(screen.getByText("Versão nova em teste: testando no PC — 2 min")).toBeInTheDocument()
  expect(screen.queryByRole("link", { name: "Confirmar plano de teste" })).not.toBeInTheDocument()
})
