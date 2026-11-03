import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { FavoriteStar } from "./FavoriteStar"

afterEach(() => vi.restoreAllMocks())

test("a click stars at once and calls setFavorite", async () => {
  const user = userEvent.setup()
  const set = vi.spyOn(api, "setFavorite").mockResolvedValue(undefined)
  render(<FavoriteStar kind="app" id={3} on={false} name="Files Prep" />)
  const btn = screen.getByRole("button", { name: "Marcar Files Prep como favorito" })
  expect(btn).toHaveAttribute("aria-pressed", "false")
  await user.click(btn)
  expect(set).toHaveBeenCalledWith("app", 3, true)
  expect(screen.getByRole("button", { name: "Tirar Files Prep dos favoritos" })).toHaveAttribute("aria-pressed", "true")
})

test("goes back when the call fails", async () => {
  const user = userEvent.setup()
  vi.spyOn(api, "setFavorite").mockRejectedValue(new Error("x"))
  render(<FavoriteStar kind="artifact" id={4} on={false} name="Guia" />)
  await user.click(screen.getByRole("button", { name: "Marcar Guia como favorito" }))
  await waitFor(() => expect(screen.getByRole("button", { name: "Marcar Guia como favorito" })).toHaveAttribute("aria-pressed", "false"))
})
