import { afterEach, expect, test, vi } from "vitest"
import { api, OFFLINE } from "./api"

afterEach(() => vi.unstubAllGlobals())

test("a request that never reaches the Hub says so in Portuguese", async () => {
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")))
  await expect(api.apps()).rejects.toThrow(OFFLINE)
})

test("a Hub that answers 5xx without a message reads as not responding", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("Bad Gateway", { status: 502 })))
  await expect(api.apps()).rejects.toThrow("O Hub não respondeu. Espere um pouco e tente de novo.")
})

test("the Hub's own message wins", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "App não encontrado." }), { status: 404 })))
  await expect(api.apps()).rejects.toThrow("App não encontrado.")
})
