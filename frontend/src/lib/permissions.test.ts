import { describe, expect, it } from "vitest"
import { canRegister, canSeeTeam, has } from "./permissions"
import { ana, bia, henry } from "@/test/fixtures"

describe("permissions", () => {
  it("reads what the backend sent", () => {
    expect(has(henry, "manage_roles")).toBe(true)
    expect(canRegister(ana)).toBe(true)
    expect(canRegister(bia)).toBe(false)
    expect(canSeeTeam(bia)).toBe(false)
    expect(canSeeTeam({ ...bia, leads: [1] })).toBe(true)
    expect(canRegister({ ...bia, leads: [1] })).toBe(true)
  })
})
