import type { RunStatus } from "./api"

export const STATUS: Record<RunStatus, { label: string; tone: "live" | "ok" | "muted" | "bad" }> = {
  waiting: { label: "Aguardando o computador", tone: "live" },
  running: { label: "Rodando", tone: "live" },
  done: { label: "Concluída", tone: "ok" },
  no_files: { label: "Sem arquivos novos", tone: "muted" },
  error: { label: "Erro", tone: "bad" },
  interrupted: { label: "Interrompida", tone: "bad" },
  stopped: { label: "Parada", tone: "muted" },
  missed: { label: "Perdida", tone: "bad" },
}

export const isLive = (s: RunStatus) => s === "waiting" || s === "running"

export function formatDuration(startISO: string | null, endISO: string | null, nowMs = Date.now()): string {
  if (!startISO) return "—"
  const secs = Math.max(0, Math.round(((endISO ? Date.parse(endISO) : nowMs) - Date.parse(startISO)) / 1000))
  if (secs < 60) return `${secs} s`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins} min`
  return `${Math.floor(mins / 60)} h ${String(mins % 60).padStart(2, "0")} min`
}

/** How long something has been going, to the second while under an hour: "45 s", "3 min 12 s", "1 h 03 min". */
export function formatElapsed(fromISO: string, nowMs = Date.now()): string {
  const secs = Math.max(0, Math.floor((nowMs - Date.parse(fromISO)) / 1000))
  if (secs < 60) return `${secs} s`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins} min ${String(secs % 60).padStart(2, "0")} s`
  return `${Math.floor(mins / 60)} h ${String(mins % 60).padStart(2, "0")} min`
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  if (bytes < 1024 ** 3) return `${(bytes / (1024 * 1024)).toFixed(1).replace(".", ",")} MB`
  return `${(bytes / 1024 ** 3).toFixed(1).replace(".", ",")} GB`
}

export function formatWhen(iso: string): string {
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, "0")
  return `${p(d.getDate())}/${p(d.getMonth() + 1)} às ${p(d.getHours())}:${p(d.getMinutes())}`
}

export function relativeDate(iso: string, nowMs = Date.now()): string {
  // Calendar days in local time: last night is "ontem" even when it was only hours ago.
  const dayStart = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const days = Math.round((dayStart(new Date(nowMs)) - dayStart(new Date(iso))) / 86_400_000)
  if (days <= 0) return "hoje"
  if (days === 1) return "ontem"
  if (days < 30) return `há ${days} dias`
  const months = Math.floor(days / 30)
  if (months < 12) return months === 1 ? "há 1 mês" : `há ${months} meses`
  const years = Math.floor(months / 12)
  return years === 1 ? "há 1 ano" : `há ${years} anos`
}

/** Like relativeDate, but with minutes and hours for what happened today. */
export function relativeTime(iso: string, nowMs = Date.now()): string {
  const mins = Math.floor((nowMs - new Date(iso).getTime()) / 60_000)
  if (mins < 1) return "agora"
  if (mins < 60) return `há ${mins} min`
  if (mins < 60 * 24 && new Date(iso).getDate() === new Date(nowMs).getDate()) return `há ${Math.floor(mins / 60)} h`
  return relativeDate(iso, nowMs)
}
