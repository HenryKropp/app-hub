import type { ReadinessItem, SharedState, SharedStatus } from "./api"
import { formatSize } from "./runs"

export const SHARED_STATUS: Record<SharedStatus, { label: string; tone: "ok" | "live" | "muted" | "bad" | "draft" }> = {
  draft: { label: "Ainda não publicado", tone: "draft" },
  starting: { label: "Ligando", tone: "live" },
  running: { label: "No ar", tone: "ok" },
  sleeping: { label: "Descansando", tone: "muted" },
  stopped: { label: "Parado", tone: "muted" },
  crashed: { label: "Caiu", tone: "bad" },
}

export function sharedLine(s: SharedState, liveVersion?: number): string {
  switch (s.status) {
    case "running":
      return liveVersion ? `No ar · versão ${liveVersion}` : "No ar"
    case "sleeping":
      return "Descansando · acorda em segundos quando alguém abre"
    case "stopped":
      return s.stopped_by ? `Parado por ${s.stopped_by}` : "Parado"
    case "crashed":
      return "Caiu e não voltou sozinho"
    case "starting":
      return "Ligando"
    default:
      return "Ainda não publicado"
  }
}

const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" })
export const formatCents = (cents: number) => brl.format(cents / 100).replace(/ /g, " ")
export const formatBytes = formatSize

export function claudeTextFor(items: ReadinessItem[]): string {
  const todo = items.filter((i) => (i.status === "missing" || i.status === "warn") && i.claude_text)
  return [
    "Estou preparando este app para rodar compartilhado no Kova. Faça estas mudanças sem quebrar o jeito de rodar no computador:",
    ...todo.map((i, n) => `${n + 1}. ${i.claude_text}`),
    "Depois, atualize o manifesto do app (kova.yaml, ou app-hub.yaml nos apps mais antigos) se precisar e me diga o que mudou.",
  ].join("\n")
}
