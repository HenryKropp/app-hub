import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError, backupDownloadUrl, fileUrl, openSharedUrl, OFFLINE, workspaceBasePath, workspaceUrl } from "./api"
import { manifest as manifestFixture } from "@/test/fixtures"

afterEach(() => vi.unstubAllGlobals())

test("workspace requests and app URLs use the company path prefix", async () => {
  const original = window.location.pathname
  window.history.replaceState({}, "", "/w/atelier-norte/seguranca")
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ slug: "atelier-norte", name: "Ateliê Norte", base_path: "/w/atelier-norte", can_manage: true, local_execution_is_sandboxed: false }), { status: 200 }))
  vi.stubGlobal("fetch", fetchMock)
  expect(workspaceBasePath()).toBe("/w/atelier-norte")
  expect(workspaceUrl("/api/agent/download")).toBe("/w/atelier-norte/api/agent/download")
  expect(fileUrl(8, 4)).toBe("/w/atelier-norte/api/runs/8/files/4")
  await api.workspace()
  expect(fetchMock).toHaveBeenCalledWith("/w/atelier-norte/api/workspace", expect.anything())
  window.history.replaceState({}, "", original)
})

test("workspace creation uses the provisioning contract", async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ slug: "filial", name: "Filial", url: "/w/filial" }), { status: 201 }))
  vi.stubGlobal("fetch", fetchMock)
  const payload = { name: "Filial", slug: "filial", password: "senha-atual" }
  await expect(api.createWorkspace(payload)).resolves.toMatchObject({ url: "/w/filial" })
  expect(fetchMock).toHaveBeenCalledWith("/api/workspaces", expect.objectContaining({ method: "POST", body: JSON.stringify(payload) }))
})

test("company entry creates at the root and confirms an explicit workspace path", async () => {
  const original = window.location.pathname
  try {
    window.history.replaceState({}, "", "/w/current-space/entrar")
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ url: "/w/new-space", name: "Nova", slug: "new-space" }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ name: "Outra", slug: "outra", base_path: "/w/outra" }), { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)
    await api.createEntryCompany({ company_name: "Nova", slug: "new-space", name: "Ana", email: "ana@example.com", password: "senha-123" })
    await api.entryCompanyBySlug("outra")
    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/entry/companies", expect.objectContaining({
      method: "POST",
      credentials: "same-origin",
      body: JSON.stringify({ company_name: "Nova", slug: "new-space", name: "Ana", email: "ana@example.com", password: "senha-123" }),
    }))
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/w/outra/api/entry/company", expect.objectContaining({ method: "GET" }))
  } finally {
    window.history.replaceState({}, "", original)
  }
})

test("membership management uses workspace-scoped endpoints and the stated payloads", async () => {
  const original = window.location.pathname
  try {
    window.history.replaceState({}, "", "/w/atelier/equipe")
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)
    await api.requestCompanyAccess({ name: "Lia", email: "lia@example.com" })
    await api.membership()
    await api.createMembershipInvitation({ email: "lia@example.com", role_id: 2 })
    await api.approveMembershipRequest(7, 2)
    await api.rejectMembershipRequest(8)
    await api.revokeMembershipInvitation(9)
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "/w/atelier/api/entry/requests",
      "/w/atelier/api/membership",
      "/w/atelier/api/membership/invitations",
      "/w/atelier/api/membership/requests/7/approve",
      "/w/atelier/api/membership/requests/8/reject",
      "/w/atelier/api/membership/invitations/9/revoke",
    ])
    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({ name: "Lia", email: "lia@example.com" })
    expect(JSON.parse(fetchMock.mock.calls[3][1]?.body as string)).toEqual({ role_id: 2 })
    expect(JSON.parse(fetchMock.mock.calls[4][1]?.body as string)).toEqual({})
  } finally {
    window.history.replaceState({}, "", original)
  }
})

test("shared app APIs and navigation never fall back to the original company", async () => {
  const original = window.location.pathname
  try {
    window.history.replaceState({}, "", "/w/filial/apps/1")
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)
    await api.server.get()
    await api.shared.publish(1)
    expect(fetchMock).toHaveBeenNthCalledWith(1, "/w/filial/api/server", expect.anything())
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/w/filial/api/apps/1/shared/publish", expect.anything())
    expect(backupDownloadUrl(1, 2)).toBe("/w/filial/api/apps/1/shared/backups/2/download")
    expect(openSharedUrl("notas")).toBe("/w/filial/api/shared/authorize?app=notas&volta=%2F")
    expect(openSharedUrl("notas", true)).toContain("/w/filial/api/shared/authorize?app=notas&teste=1")
  } finally {
    window.history.replaceState({}, "", original)
  }
})

test("import retains structured confirmation errors and sends explicit consent", async () => {
  const body = {
    code: "script_secrets_confirmation_required",
    error: "Possíveis senhas ou chaves no código.",
    secret_count: 2,
  }
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status: 409 }))
  vi.stubGlobal("fetch", fetchMock)
  const payload = { manifest: manifestFixture(), files: {}, confirm_secrets: true }
  await expect(api.importScript(payload)).rejects.toMatchObject({
    status: 409, body, message: body.error,
  } satisfies Partial<ApiError>)
  expect(fetchMock).toHaveBeenCalledWith("/api/scripts/import", expect.objectContaining({
    method: "POST", credentials: "same-origin", body: JSON.stringify(payload),
  }))
})

test("a request that never reaches the Hub says so in Portuguese", async () => {
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")))
  await expect(api.apps()).rejects.toThrow(OFFLINE)
})

test("a Hub that answers 5xx without a message reads as not responding", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("Bad Gateway", { status: 502 })))
  await expect(api.apps()).rejects.toThrow("O Ziv não respondeu. Espere um pouco e tente de novo.")
})

test("the Hub's own message wins", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "App não encontrado." }), { status: 404 })))
  await expect(api.apps()).rejects.toThrow("App não encontrado.")
})

test("onboarding company name uses the authenticated setup endpoint", async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ company_name: "Alis", can_manage: true }), { status: 200 }))
  vi.stubGlobal("fetch", fetchMock)
  await api.saveCompanyName("Alis")
  expect(fetchMock).toHaveBeenCalledWith("/api/onboarding", expect.objectContaining({
    method: "PUT",
    credentials: "same-origin",
    body: JSON.stringify({ company_name: "Alis" }),
  }))
})

test("script run sends manifest values and uploaded file metadata only when provided", async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 2 }), { status: 200 }))
  vi.stubGlobal("fetch", fetchMock)
  await api.startRun(4, "D:\\Saídas", true, { periodo: "2026-10" }, { entrada: { name: "dados.csv", content_base64: "YQ==" } })
  expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({
    output_folder: "D:\\Saídas",
    remember: true,
    inputs: { periodo: "2026-10" },
    files: { entrada: { name: "dados.csv", content_base64: "YQ==" } },
  })
})

test("desktop run keeps its legacy request body", async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 2 }), { status: 200 }))
  vi.stubGlobal("fetch", fetchMock)
  await api.startRun(4, "D:\\Saídas")
  expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({ output_folder: "D:\\Saídas" })
})

test("trusted script registration posts its manifest and relative file map", async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 12 }), { status: 200 }))
  vi.stubGlobal("fetch", fetchMock)
  const manifest = manifestFixture({ id: "", name: "Ferramenta" })
  const files = { "src/main.py": { name: "main.py", content_base64: "cHJpbnQoMSk=" } }
  await api.importScript({ manifest, files })
  expect(fetchMock).toHaveBeenCalledWith("/api/scripts/import", expect.objectContaining({
    method: "POST",
    credentials: "same-origin",
    body: JSON.stringify({ manifest, files }),
  }))
})
