import { render, screen } from "@testing-library/react"
import { expect, test } from "vitest"
import { Button } from "./button"

test("run é preenchido de coral, com texto escuro e foco neutro", () => {
  render(<Button variant="run">Rodar</Button>)
  const cls = screen.getByRole("button", { name: "Rodar" }).className
  expect(cls).toContain("bg-brand")
  expect(cls).toContain("text-brand-foreground")
  expect(cls).toContain("hover:bg-brand-hover")
  expect(cls).toContain("focus-visible:ring-offset-2")
  expect(cls).not.toContain("ring-brand")
})
