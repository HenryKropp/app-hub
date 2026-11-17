import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, expect, test, vi } from "vitest"
import { api, type ExternalLink } from "@/lib/api"
import { ArtifactLinksSection, CompanyLinksPanel } from "./ExternalLinks"

afterEach(() => vi.restoreAllMocks())

const link = (over: Partial<ExternalLink> = {}): ExternalLink => ({
  id: 7, url: "https://link.ziv.test/piloto/abc", kind: "open", recipient: "", content: "latest", version_number: 0,
  created_by_name: "Ana", created_at: "2026-11-17T10:00:00Z", expires_at: "2026-12-17T10:00:00Z", status: "ativo",
  opens: 2, last_opened_at: "2026-11-17T12:00:00Z", artifact_id: 5, ...over,
})

function showSection(data: { email_available: boolean; can_share: boolean; links: ExternalLink[] }) {
  vi.spyOn(api, "artifactLinks").mockResolvedValue(data)
  return render(<MemoryRouter><ArtifactLinksSection artifactId={5} currentVersion={2} /></MemoryRouter>)
}

test("cria link aberto e mostra Copiar link", async () => {
  showSection({ email_available: false, can_share: true, links: [] })
  const create = vi.spyOn(api, "createArtifactLink").mockResolvedValue(link())
  const user = userEvent.setup()
  await user.click(await screen.findByRole("button", { name: "Criar link para fora" }))
  expect(screen.getByText("Qualquer pessoa com este link consegue abrir.")).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Criar link" }))
  expect(create).toHaveBeenCalledWith(5, { kind: "open", email: "", content: "latest", version: 2, expires_days: 30 })
  expect(await screen.findByText("https://link.ziv.test/piloto/abc")).toBeInTheDocument()
  expect(screen.getAllByRole("button", { name: "Copiar link" }).length).toBeGreaterThan(0)
})

test("e-mail desabilitado quando email_available é falso", async () => {
  showSection({ email_available: false, can_share: true, links: [] })
  await userEvent.setup().click(await screen.findByRole("button", { name: "Criar link para fora" }))
  expect(screen.getByRole("radio", { name: /Código por e-mail/ })).toBeDisabled()
  expect(screen.getByText("Disponível em breve")).toBeInTheDocument()
  expect(screen.getByRole("radio", { name: /Link aberto/ })).toBeChecked()
})

test("com e-mail ligado, o padrão é código por e-mail e pede o e-mail", async () => {
  showSection({ email_available: true, can_share: true, links: [] })
  const create = vi.spyOn(api, "createArtifactLink").mockResolvedValue(link({ kind: "email", recipient: "cli@fora.com" }))
  const user = userEvent.setup()
  await user.click(await screen.findByRole("button", { name: "Criar link para fora" }))
  expect(screen.getByRole("radio", { name: /Código por e-mail/ })).toBeChecked()
  await user.type(screen.getByLabelText("E-mail de quem vai receber"), "cli@fora.com")
  await user.click(screen.getByRole("radio", { name: /Só a versão 2/ }))
  await user.selectOptions(screen.getByLabelText("Prazo"), "7")
  await user.click(screen.getByRole("button", { name: "Criar link" }))
  expect(create).toHaveBeenCalledWith(5, { kind: "email", email: "cli@fora.com", content: "version", version: 2, expires_days: 7 })
})

test("Cancelar pede confirmação e chama a API", async () => {
  showSection({ email_available: false, can_share: true, links: [link()] })
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(true)
  const cancel = vi.spyOn(api, "cancelLink").mockResolvedValue(undefined)
  await userEvent.setup().click(await screen.findByRole("button", { name: "Cancelar" }))
  expect(confirm).toHaveBeenCalledWith("Cancelar este link? Quem tiver o link não vai mais conseguir abrir.")
  await waitFor(() => expect(cancel).toHaveBeenCalledWith(7))
})

test("link cancelado não mostra Cancelar nem Copiar", async () => {
  showSection({ email_available: false, can_share: true, links: [link({ status: "cancelado" })] })
  expect(await screen.findByText("Cancelado")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Cancelar" })).not.toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "Copiar link" })).not.toBeInTheDocument()
})

test("data do prazo conforme o estado do link", async () => {
  showSection({ email_available: false, can_share: true, links: [
    link({ id: 1, status: "ativo" }), link({ id: 2, status: "vencido" }), link({ id: 3, status: "cancelado" }),
  ] })
  expect(await screen.findByText(/vence em/)).toBeInTheDocument()
  expect(screen.getByText(/venceu em/)).toBeInTheDocument()
  expect(screen.getByText(/prazo era/)).toBeInTheDocument()
})

test("seção não aparece sem can_share", async () => {
  const { container } = showSection({ email_available: false, can_share: false, links: [] })
  await waitFor(() => expect(api.artifactLinks).toHaveBeenCalled())
  expect(container).toBeEmptyDOMElement()
})

test("Admin mostra links da empresa com o nome do artifact", async () => {
  vi.spyOn(api, "companyLinks").mockResolvedValue({ links: [link({ artifact_name: "Relatório mensal", kind: "email", recipient: "cli@fora.com" })] })
  render(<MemoryRouter><CompanyLinksPanel /></MemoryRouter>)
  const a = await screen.findByRole("link", { name: "Relatório mensal" })
  expect(a).toHaveAttribute("href", "/artifacts/5")
  expect(screen.getByText("cli@fora.com")).toBeInTheDocument()
  expect(screen.getByText(/2 aberturas/)).toBeInTheDocument()
})
