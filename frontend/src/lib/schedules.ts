import type { Frequency } from "./api"

// The same rules as backend/internal/schedule (ADR 024), so the form can show
// the next runs and refuse bad choices before saving. Schedules live in
// Brasília time, which has had no daylight saving since 2019: a fixed UTC-3.
const OFFSET_MS = 3 * 60 * 60 * 1000
const MIN_MINUTES = 5
export const DAY_NAMES = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"]

/** A "wall clock" date: its UTC fields read as Brasília local time. */
const toLocal = (d: Date) => new Date(d.getTime() - OFFSET_MS)
const fromLocal = (d: Date) => new Date(d.getTime() + OFFSET_MS)

function parseAt(at = ""): [number, number] | null {
  const m = /^(\d{2}):(\d{2})$/.exec(at)
  if (!m) return null
  const h = Number(m[1]), min = Number(m[2])
  return h < 24 && min < 60 ? [h, min] : null
}

/** Why a frequency cannot be saved, in the server's words; "" when it can. */
export function frequencyProblem(f: Frequency): string {
  switch (f.kind) {
    case "minutes":
      if (!Number.isInteger(f.every) || (f.every ?? 0) < MIN_MINUTES) return "No mínimo a cada 5 minutos."
      if ((f.every ?? 0) > 1440) return "No máximo a cada 1440 minutos (um dia)."
      return ""
    case "hours":
      return Number.isInteger(f.every) && (f.every ?? 0) >= 1 && (f.every ?? 0) <= 24 ? "" : "Escolha de 1 a 24 horas."
    case "daily":
    case "weekdays":
    case "monthly":
      if (!parseAt(f.at)) return "Informe o horário no formato HH:MM, por exemplo 07:00."
      if (f.kind === "weekdays" && !(f.days ?? []).length) return "Escolha pelo menos um dia da semana."
      if (f.kind === "monthly" && !(Number.isInteger(f.day) && (f.day ?? 0) >= 1 && (f.day ?? 0) <= 28)) return "Escolha um dia do mês de 1 a 28."
      return ""
  }
  return "Escolha com que frequência o app roda."
}

function next(f: Frequency, after: Date): Date | null {
  const t = toLocal(after)
  const midnight = Date.UTC(t.getUTCFullYear(), t.getUTCMonth(), t.getUTCDate())
  if (f.kind === "minutes" || f.kind === "hours") {
    const step = (f.every ?? 1) * (f.kind === "hours" ? 3_600_000 : 60_000)
    const n = Math.floor((t.getTime() - midnight) / step) + 1
    return fromLocal(new Date(Math.min(midnight + n * step, midnight + 86_400_000)))
  }
  const at = parseAt(f.at)
  if (!at) return null
  for (let d = 0; d < 400; d++) {
    const day = new Date(midnight + d * 86_400_000)
    const when = new Date(Date.UTC(day.getUTCFullYear(), day.getUTCMonth(), day.getUTCDate(), at[0], at[1]))
    if (when.getTime() <= t.getTime()) continue
    if (f.kind === "daily") return fromLocal(when)
    if (f.kind === "weekdays" && (f.days ?? []).includes(when.getUTCDay())) return fromLocal(when)
    if (f.kind === "monthly" && when.getUTCDate() === f.day) return fromLocal(when)
  }
  return null
}

/** The next n run times after a moment (empty when the frequency is invalid). */
export function upcoming(f: Frequency, after: Date, n: number): Date[] {
  if (frequencyProblem(f)) return []
  const out: Date[] = []
  for (let cur: Date | null = after; out.length < n; ) {
    cur = next(f, cur)
    if (!cur) break
    out.push(cur)
  }
  return out
}

function hour(at = "") {
  const [h, m] = parseAt(at) ?? [0, 0]
  return m === 0 ? `${h}h` : `${h}h${String(m).padStart(2, "0")}`
}

/** The frequency in plain Portuguese ("Todo dia às 7h"), as the server says it. */
export function describeFrequency(f: Frequency): string {
  switch (f.kind) {
    case "minutes": return `A cada ${f.every} minutos`
    case "hours": return f.every === 1 ? "A cada hora" : `A cada ${f.every} horas`
    case "daily": return `Todo dia às ${hour(f.at)}`
    case "weekdays": {
      const names = [...new Set(f.days ?? [])].sort((a, b) => a - b).map((d, i) => (i ? DAY_NAMES[d].toLowerCase() : DAY_NAMES[d]))
      const list = names.length === 1 ? names[0] : `${names.slice(0, -1).join(", ")} e ${names[names.length - 1]}`
      return `${list} às ${hour(f.at)}`
    }
    case "monthly": return `Todo mês no dia ${f.day} às ${hour(f.at)}`
  }
  return ""
}

/** "sex 07/11 às 7h" in Brasília time. */
export function formatRunTime(iso: string | Date): string {
  const d = toLocal(typeof iso === "string" ? new Date(iso) : iso)
  const hh = d.getUTCHours(), mm = d.getUTCMinutes()
  const time = mm === 0 ? `${hh}h` : `${hh}h${String(mm).padStart(2, "0")}`
  return `${DAY_NAMES[d.getUTCDay()].toLowerCase()} ${String(d.getUTCDate()).padStart(2, "0")}/${String(d.getUTCMonth() + 1).padStart(2, "0")} às ${time}`
}
