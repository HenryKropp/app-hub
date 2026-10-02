import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { ThemeProvider } from "@/lib/theme"
import { SettingsPage } from "./SettingsPage"

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.classList.remove("dark")
  localStorage.clear()
})

test("settings holds Meu computador and the theme choice", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "computers").mockResolvedValue({ computers: [], expected_version: "0.1.0", agent_available: true })
  render(<ThemeProvider><MemoryRouter><SettingsPage /></MemoryRouter></ThemeProvider>)
  expect(screen.getByRole("heading", { name: "Configurações" })).toBeInTheDocument()
  expect(await screen.findByRole("heading", { name: "Meu computador" })).toBeInTheDocument()
  expect(screen.getByRole("heading", { name: "Aparência" })).toBeInTheDocument()
  await user.click(screen.getByRole("radio", { name: /Escuro/ }))
  expect(screen.getByRole("radio", { name: /Escuro/ })).toHaveAttribute("aria-checked", "true")
  expect(document.documentElement).toHaveClass("dark")
})
