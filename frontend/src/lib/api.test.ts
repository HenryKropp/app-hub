import { afterEach, expect, test, vi } from "vitest"
import { api, ApiError, OFFLINE } from "./api"
import { manifest as manifestFixture } from "@/test/fixtures"

afterEach(() => vi.unstubAllGlobals())

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
