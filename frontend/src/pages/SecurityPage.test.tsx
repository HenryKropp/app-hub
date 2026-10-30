import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, test, vi } from "vitest"
import { ApiError, api, type SecurityInfo } from "@/lib/api"
import { SecurityPage } from "./SecurityPage"

const fixture: SecurityInfo = {
  can_manage: true,
  settings: { retention_days: 45, require_app_approval: true, block_embedded_secrets: true, local_execution_enabled: true },
  limits: { sandboxed: false, vault_available: false, external_marketplace_enabled: false },
  apps: [{ id: 7, name: "Conferência mensal", classification: "internal", purpose: "Conciliação local", external_domains: ["relatorios.interno"], approved: false, suspended: false }],
}

function mockManager() {
  vi.spyOn(api, "security").mockResolvedValue(fixture)
  vi.spyOn(api, "securityAudit").mockResolvedValue({ events: [] })
  vi.spyOn(api, "workspace").mockResolvedValue({ slug: "atelier", name: "Ateliê Norte", base_path: "/w/atelier", can_manage: true, local_execution_is_sandboxed: false })
  vi.spyOn(api, "credentials").mockResolvedValue({ available: false, credentials: [] })
  vi.spyOn(api, "roles").mockResolvedValue([])
  vi.spyOn(api, "credentialAudit").mockResolvedValue([])
}

afterEach(() => vi.restoreAllMocks())

test("security page states the actual execution and vault boundaries and saves settings", async () => {
  const user = userEvent.setup()
  mockManager()
  const save = vi.spyOn(api, "saveSecuritySettings").mockImplementation(async (settings) => settings)
  render(<SecurityPage />)
  expect(await screen.findByText("Não é isolada")).toBeInTheDocument()
  expect(screen.getByText("Indisponível")).toBeInTheDocument()
  expect(screen.getByText(/Domínios externos declarados/)).toBeInTheDocument()
  const retention = screen.getByLabelText("Retenção de execuções finalizadas (dias)")
  await user.clear(retention)
  await user.type(retention, "62")
  await user.click(screen.getByRole("button", { name: "Salvar política" }))
  expect(save).toHaveBeenCalledWith(expect.objectContaining({ retention_days: 62 }))
  expect(await screen.findByRole("status")).toHaveTextContent("Configurações salvas.")
})

test("company creation presents credential reauthentication and no company directory", async () => {
  const user = userEvent.setup()
  mockManager()
  render(<SecurityPage />)
  await user.click(await screen.findByRole("button", { name: "Criar espaço de empresa" }))
  expect(screen.getByLabelText("Nome da empresa")).toBeInTheDocument()
  expect(screen.getByLabelText("Confirme sua senha atual")).toBeInTheDocument()
  expect(screen.getByText(/Não existe diretório ou listagem de outras empresas/)).toBeInTheDocument()
})

test("non-managers receive a clear access-denied state from the API permission boundary", async () => {
  vi.spyOn(api, "security").mockRejectedValue(new ApiError(403, "Proibido"))
  vi.spyOn(api, "securityAudit").mockRejectedValue(new ApiError(403, "Proibido"))
  vi.spyOn(api, "workspace").mockResolvedValue({ slug: "atelier", name: "Ateliê Norte", base_path: "/w/atelier", can_manage: false, local_execution_is_sandboxed: false })
  render(<SecurityPage />)
  expect(await screen.findByText("Acesso restrito")).toBeInTheDocument()
  expect(screen.getByText(/exige a permissão manage_roles/)).toBeInTheDocument()
})
