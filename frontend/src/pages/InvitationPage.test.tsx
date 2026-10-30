import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api } from "@/lib/api"
import { InvitationPage } from "./InvitationPage"

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

test("invitation token stays in the fragment and acceptance sends only name and password", async () => {
  const original = window.location.pathname
  try {
    window.history.replaceState({}, "", "/w/atelier/convite#token=frag-chave")
    const preview = vi.spyOn(api, "previewInvitation").mockResolvedValue({ email: "bia@example.com", company_name: "Ateliê Norte" })
    const accept = vi.spyOn(api, "acceptInvitation").mockResolvedValue({ url: "/w/atelier" })
    const assign = vi.fn()
    vi.stubGlobal("location", { ...window.location, assign })
    render(<MemoryRouter><InvitationPage /></MemoryRouter>)
    expect(await screen.findByText("bia@example.com")).toBeInTheDocument()
    expect(preview).toHaveBeenCalledWith("frag-chave")
    await userEvent.type(screen.getByLabelText("Seu nome"), "Bia")
    await userEvent.type(screen.getByLabelText("Crie uma senha"), "senha-forte-123")
    await userEvent.click(screen.getByRole("button", { name: "Aceitar convite" }))
    expect(accept).toHaveBeenCalledWith({ token: "frag-chave", name: "Bia", password: "senha-forte-123" })
    expect(assign).toHaveBeenCalledWith("/w/atelier")
  } finally {
    window.history.replaceState({}, "", original)
  }
})
