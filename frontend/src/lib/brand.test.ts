import { describe, expect, it } from "vitest"
import { AGENT_NAME, PRODUCT_NAME } from "./brand"

describe("brand", () => {
  it("nome do produto e do agente", () => {
    expect(PRODUCT_NAME).toBe("Ziv")
    expect(AGENT_NAME).toBe("Agente do Ziv")
  })
})
