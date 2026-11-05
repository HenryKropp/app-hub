import { api } from "./api"
import type { Person } from "./api"
import { visibleSteps } from "./steps"

// Where to send someone right after signing in: Começar while steps remain, Home otherwise.
export async function landingAfterLogin(basePath: string, person: Person): Promise<string> {
  try {
    const onboarding = await api.onboardingAt(basePath)
    return visibleSteps(onboarding, person).some((s) => !s.done) ? `${basePath}/comecar` : `${basePath}/`
  } catch {
    return `${basePath}/comecar`
  }
}
