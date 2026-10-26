import type { RunStatus } from "./api"

export const STATUS: Record<RunStatus, { label: string; tone: "live" | "ok" | "muted" | "bad" }> = {
  waiting: { label: "Aguardando o computador", tone: "live" },
  running: { label: "Rodando", tone: "live" },
  done: { label: "Concluída", tone: "ok" },
  no_files: { label: "Sem arquivos novos", tone: "muted" },
  error: { label: "Erro", tone: "bad" },
  interrupted: { label: "Interrompida", tone: "bad" },
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
