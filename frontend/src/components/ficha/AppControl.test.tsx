import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { AppControlBar } from "./AppControl"

afterEach(() => vi.restoreAllMocks())

const base = { approved: false, suspended: false, require_approval: true, can_manage: true }

test("an administrator approves a waiting app on its page", async () => {
  const set = vi.spyOn(api, "setAppControl").mockResolvedValue({ ...base, approved: true })
  const changed = vi.fn()
  render(<AppControlBar appId={7} control={base} onChanged={changed} />)
  expect(screen.getByText("Esperando aprovação.")).toBeInTheDocument()
  await userEvent.click(screen.getByRole("button", { name: "Aprovar" }))
  expect(set).toHaveBeenCalledWith(7, { approved: true, suspended: false })
  expect(changed).toHaveBeenCalledWith({ ...base, approved: true })
})

test("suspending asks first; a suspended app can be reactivated", async () => {
  const set = vi.spyOn(api, "setAppControl").mockResolvedValue({ ...base, approved: true, suspended: true })
  vi.spyOn(window, "confirm").mockReturnValue(true)
  const { rerender } = render(<AppControlBar appId={7} control={{ ...base, approved: true }} onChanged={() => {}} />)
  await userEvent.click(screen.getByRole("button", { name: "Suspender" }))
  expect(set).toHaveBeenCalledWith(7, { approved: true, suspended: true })
  rerender(<AppControlBar appId={7} control={{ ...base, approved: true, suspended: true }} onChanged={() => {}} />)
  expect(screen.getByText("App suspenso.")).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Reativar" })).toBeInTheDocument()
})

test("people who can't manage see why the app can't run, without buttons", () => {
  render(<AppControlBar appId={7} control={{ ...base, can_manage: false }} onChanged={() => {}} />)
  expect(screen.getByText(/precisa aprovar este app/)).toBeInTheDocument()
  expect(screen.queryByRole("button")).not.toBeInTheDocument()
})

test("an approved app shows nothing to people who can't manage", () => {
  const { container } = render(<AppControlBar appId={7} control={{ ...base, approved: true, can_manage: false }} onChanged={() => {}} />)
  expect(container).toBeEmptyDOMElement()
})
