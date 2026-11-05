import { expect, test } from "vitest"
import type { Onboarding, Person } from "./api"
import { visibleSteps } from "./steps"

const onboarding = (over: Partial<Onboarding> = {}): Onboarding => ({
  company_name: "X", can_manage: false, agent_available: true, computer_connected: false,
  claude_connected: false, has_apps: false, has_successful_run: false, has_artifacts: false, has_team: false, ...over,
})
const person = (permissions: string[] = []): Person => ({
  id: 1, name: "A", email: "a@x.com", role_id: 1, role_name: "R", permissions, leads: [], areas: [], sees_others_runs: false,
})

test("an admin sees the six steps in order", () => {
  const steps = visibleSteps(onboarding({ can_manage: true, has_team: true }), person(["register_apps"]))
  expect(steps.map((s) => s.id)).toEqual(["computador", "claude", "app", "rodar", "artifact", "equipe"])
  expect(steps.find((s) => s.id === "equipe")?.done).toBe(true)
})

test("a plain member sees four steps", () => {
  expect(visibleSteps(onboarding(), person()).map((s) => s.id)).toEqual(["computador", "claude", "rodar", "artifact"])
})

test("done follows the onboarding flags", () => {
  const steps = visibleSteps(onboarding({ computer_connected: true, has_successful_run: true }), person())
  expect(steps.map((s) => s.done)).toEqual([true, false, true, false])
})
