import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { api, type CredentialMeta, type SecurityApp } from "@/lib/api"
import { CredentialVault } from "./CredentialVault"

const apps: SecurityApp[] = [
  { id: 12, name: "Painel de compras", classification: "internal", purpose: "", external_domains: [], approved: true, suspended: false },
]
const credential: CredentialMeta = {
  id: "cred-7", app_id: 12, environment: "staging", role_id: 4, revision: 3, revoked: false, updated_at: "2025-01-15T10:00:00Z",
}

function mockVault(available = true, entries: CredentialMeta[] = []) {
  vi.spyOn(api, "credentials").mockResolvedValue({ available, credentials: entries })
  vi.spyOn(api, "roles").mockResolvedValue([{ id: 4, key: "operator", name: "Operação", permissions: [], people_count: 1 }])
  vi.spyOn(api, "credentialAudit").mockResolvedValue([])
}

afterEach(() => vi.restoreAllMocks())

test("submits a write-only bearer token and clears the input", async () => {
  const user = userEvent.setup()
  mockVault()
  const create = vi.spyOn(api, "createCredential").mockResolvedValue(credential)
  render(<CredentialVault apps={apps} />)

  await screen.findByRole("heading", { name: "Cofre de credenciais" })
  await user.selectOptions(screen.getByLabelText("Aplicativo"), "12")
  await user.selectOptions(screen.getByLabelText("Papel"), "4")
  await user.type(screen.getByLabelText("Endpoint HTTPS"), "https://api.example.test/health")
  const secret = "token-confidencial-312"
  const secretInput = screen.getByLabelText("Token bearer")
  await user.type(secretInput, secret)
  await user.click(screen.getByRole("button", { name: "Armazenar credencial" }))

  await waitFor(() => expect(create).toHaveBeenCalledWith({
    app_id: 12, environment: "development", role_id: 4, endpoint: "https://api.example.test/health", value: secret,
  }))
  await waitFor(() => expect(secretInput).toHaveValue(""))
  expect(document.body).not.toHaveTextContent(secret)
})

test("clears the token after failed storage and does not render a secret echoed by the API", async () => {
  const user = userEvent.setup()
  mockVault()
  vi.spyOn(api, "createCredential").mockRejectedValue(new Error("Falha para token-confidencial-312"))
  render(<CredentialVault apps={apps} />)
  await screen.findByRole("heading", { name: "Cofre de credenciais" })
  await user.selectOptions(screen.getByLabelText("Aplicativo"), "12")
  await user.selectOptions(screen.getByLabelText("Papel"), "4")
  await user.type(screen.getByLabelText("Endpoint HTTPS"), "https://api.example.test")
  const secretInput = screen.getByLabelText("Token bearer")
  await user.type(secretInput, "token-confidencial-312")
  await user.click(screen.getByRole("button", { name: "Armazenar credencial" }))

  expect(await screen.findByRole("alert")).toHaveTextContent("[redigido]")
  await waitFor(() => expect(secretInput).toHaveValue(""))
  expect(document.body).not.toHaveTextContent("token-confidencial-312")
})

test("keyring lock keeps safe metadata and permits revocation", async () => {
  const user = userEvent.setup()
  mockVault(false, [credential])
  const revoke = vi.spyOn(api, "revokeCredential").mockResolvedValue({ revoked: true })
  vi.spyOn(window, "confirm").mockReturnValue(true)
  render(<CredentialVault apps={apps} />)

  expect(await screen.findByText(/Cofre bloqueado/)).toBeInTheDocument()
  expect(screen.getAllByText("Painel de compras").length).toBeGreaterThan(0)
  expect(screen.getByRole("button", { name: "Rotacionar" })).toBeDisabled()
  expect(screen.getByRole("button", { name: "Testar HEAD" })).toBeDisabled()
  await user.click(screen.getByRole("button", { name: "Revogar" }))
  await waitFor(() => expect(revoke).toHaveBeenCalledWith(credential.id))
})

test("rotation keeps app and environment immutable", async () => {
  const user = userEvent.setup()
  mockVault(true, [credential])
  const replace = vi.spyOn(api, "replaceCredential").mockResolvedValue({ ...credential, revision: 4 })
  render(<CredentialVault apps={apps} />)
  await user.click(await screen.findByRole("button", { name: "Rotacionar" }))
  const form = screen.getByRole("heading", { name: "Rotacionar credencial" }).closest("form")
  expect(form).not.toBeNull()
  expect(within(form as HTMLFormElement).getByLabelText("Aplicativo")).toBeDisabled()
  expect(within(form as HTMLFormElement).getByLabelText("Ambiente")).toBeDisabled()
  expect(screen.getByText(/não podem ser alterados durante a rotação/)).toBeInTheDocument()
  await user.type(within(form as HTMLFormElement).getByLabelText("Endpoint HTTPS"), "https://api.example.test/health")
  await user.type(within(form as HTMLFormElement).getByLabelText("Token bearer"), "fresh-token")
  await user.click(within(form as HTMLFormElement).getByRole("button", { name: "Salvar rotação" }))
  await waitFor(() => expect(replace).toHaveBeenCalledWith(credential.id, expect.objectContaining({
    app_id: 12, environment: "staging", role_id: 4, value: "fresh-token",
  })))
})

test("HEAD check and rewrap display only status and metadata receipts", async () => {
  const user = userEvent.setup()
  mockVault(true, [credential])
  const check = vi.spyOn(api, "checkCredential").mockResolvedValue({ upstream_status: 401, credential_revision: 3 })
  const rewrap = vi.spyOn(api, "rewrapCredentials").mockResolvedValue({ rewrapped: 1 })
  render(<CredentialVault apps={apps} />)
  await user.click(await screen.findByRole("button", { name: "Testar HEAD" }))
  expect(check).toHaveBeenCalledWith(credential)
  expect(await screen.findByText(/Status HTTP recebido do destino: 401/)).toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Reproteger com chave ativa" }))
  expect(rewrap).toHaveBeenCalledOnce()
  expect(await screen.findByText(/1 credencial\(is\) reprotegida/)).toBeInTheDocument()
})

test("load failure offers retry without exposing stale management controls", async () => {
  const user = userEvent.setup()
  mockVault()
  vi.mocked(api.credentials).mockRejectedValueOnce(new Error("Cofre indisponível"))
  render(<CredentialVault apps={apps} />)
  expect(await screen.findByRole("alert")).toHaveTextContent("Cofre indisponível")
  expect(screen.queryByRole("button", { name: "Armazenar credencial" })).not.toBeInTheDocument()
  await user.click(screen.getByRole("button", { name: "Tentar novamente" }))
  expect(await screen.findByRole("button", { name: "Armazenar credencial" })).toBeInTheDocument()
})

test("unapproved apps cannot be checked and revoked credentials cannot be rotated", async () => {
  mockVault(true, [credential, { ...credential, id: "revoked", revoked: true }])
  render(<CredentialVault apps={[{ ...apps[0], approved: false }]} />)
  expect(await screen.findByText("Revogada")).toBeInTheDocument()
  expect(screen.getAllByRole("button", { name: "Rotacionar" })).toHaveLength(1)
  expect(screen.getByRole("button", { name: "Testar HEAD" })).toBeDisabled()
})
