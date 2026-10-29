import { render, screen } from "@testing-library/react"
import { expect, test } from "vitest"
import { Logo, Wordmark } from "./bits"

test("Logo desenha o símbolo oficial do Ziv", () => {
  render(<Logo />)
  const svg = screen.getByRole("img", { name: "Ziv" })
  expect(svg.querySelector("rect")?.getAttribute("fill")).toBe("var(--logo-square)")
  expect(svg.querySelector('path[d="M24 22V36"]')).not.toBeNull()
  expect(svg.querySelector("circle")?.getAttribute("fill")).toBe("#FF6B4A")
  expect(svg.querySelectorAll('g[stroke="#FF6B4A"] path')).toHaveLength(3)
})

test("Wordmark escreve ziv com o pingo coral e as letras na cor do texto", () => {
  render(<Wordmark />)
  const svg = screen.getByRole("img", { name: "Ziv" })
  expect(svg.querySelector('g[stroke="currentColor"]')?.querySelectorAll("path")).toHaveLength(3)
  expect(svg.querySelector("circle")?.getAttribute("fill")).toBe("#FF6B4A")
})
