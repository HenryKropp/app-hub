import type { AppDetail, SharedSettings, SharedState } from "@/lib/api"

// The values "Tornar compartilhado" starts with.
export const SHARED_DEFAULTS = { memory_mb: 512, cpus: 0.5, idle_minutes: 30 }

// settingsOf is the whole shared block as PUT /shared wants it: what the
// manifest says, then what the Hub already runs with, then the defaults.
// Every save sends it whole, with the ficha's updated_at as the stale guard.
export function settingsOf(app: AppDetail, state?: SharedState | null, patch: Partial<SharedSettings> = {}): SharedSettings {
  const m = app.manifest.shared ?? null
  return {
    enabled: true,
    always_on: m?.always_on ?? state?.always_on ?? false,
    idle_minutes: m?.idle_minutes || state?.idle_minutes || SHARED_DEFAULTS.idle_minutes,
    memory_mb: m?.memory_mb || state?.memory_mb || SHARED_DEFAULTS.memory_mb,
    cpus: m?.cpus || state?.cpus || SHARED_DEFAULTS.cpus,
    port: m?.port ?? 0,
    health_path: m?.health_path ?? "",
    database: m?.database ?? "",
    command: m?.command ?? "",
    env: m?.env ?? [],
    acks: state?.acks ?? [],
    updated_at: app.updated_at,
    ...patch,
  }
}

export const errText = (e: unknown, fallback = "Não foi possível completar a ação.") => (e instanceof Error && e.message ? e.message : fallback)
