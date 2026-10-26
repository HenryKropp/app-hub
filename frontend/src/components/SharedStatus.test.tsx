import { render, screen } from "@testing-library/react"
import { it, expect } from "vitest"
import { SharedStatusDot } from "./SharedStatus"

it("shows the status in words for screen readers", () => {
  render(<SharedStatusDot status="sleeping" />)
  expect(screen.getByText("Descansando")).toBeInTheDocument()
})
