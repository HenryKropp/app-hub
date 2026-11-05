import type { Onboarding, Person } from "./api"
import { canRegister } from "./permissions"

export type StepId = "computador" | "claude" | "app" | "rodar" | "artifact" | "equipe"

export function visibleSteps(o: Onboarding, person: Person): { id: StepId; done: boolean }[] {
  const steps: { id: StepId; done: boolean }[] = [
    { id: "computador", done: o.computer_connected },
    { id: "claude", done: o.claude_connected },
  ]
  if (canRegister(person)) steps.push({ id: "app", done: o.has_apps })
  steps.push({ id: "rodar", done: o.has_successful_run }, { id: "artifact", done: o.has_artifacts })
  if (o.can_manage) steps.push({ id: "equipe", done: o.has_team })
  return steps
}
