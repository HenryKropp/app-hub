import { afterEach, expect, test, vi } from "vitest"
import { api } from "./api"
import type { Onboarding, Person } from "./api"
import { landingAfterLogin } from "./landing"

afterEach(() => vi.restoreAllMocks())

const person = { id: 1, name: "A", email: "a@x.com", role_id: 1, role_name: "R", permissions: [], leads: [], areas: [], sees_others_runs: false } as Person
const done = { company_name: "X", can_manage: false, agent_available: true, computer_connected: true, claude_connected: true, has_apps: true, has_successful_run: true, has_artifacts: true, has_team: true } as Onboarding

test("goes to Começar while a visible step is missing", async () => {
  vi.spyOn(api, "onboardingAt").mockResolvedValue({ ...done, has_artifacts: false })
  expect(await landingAfterLogin("/w/a", person)).toBe("/w/a/comecar")
})
test("goes to Home when every visible step is done", async () => {
  vi.spyOn(api, "onboardingAt").mockResolvedValue({ ...done, has_team: false })
  expect(await landingAfterLogin("/w/a", person)).toBe("/w/a/")
})
test("falls back to Começar when onboarding fails", async () => {
  vi.spyOn(api, "onboardingAt").mockRejectedValue(new Error("x"))
  expect(await landingAfterLogin("/w/a", person)).toBe("/w/a/comecar")
})
