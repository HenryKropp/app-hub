import { render, screen } from "@testing-library/react"
import { expect, test } from "vitest"
import { Logo } from "./bits"

test("Logo desenha o símbolo oficial do Kova", () => {
  render(<Logo />)
  const svg = screen.getByRole("img", { name: "Kova" })
  expect(svg.querySelector("rect")?.getAttribute("fill")).toBe("var(--logo-square)")
  expect(svg.querySelector("path")?.getAttribute("d")).toBe("M33 13.5A14 14 0 1 0 33 34.5")
  expect(svg.querySelector("circle")?.getAttribute("fill")).toBe("#FF6B4A")
})
