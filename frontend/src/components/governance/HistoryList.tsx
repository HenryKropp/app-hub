import type { ApprovalRequest } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { ON_BEHALF, reasonLabel } from "./RequestCard"

const STATUS: Record<string, string> = { open: "esperando", approved: "aprovado", rejected: "recusado", auto: "foi ao ar com aviso", superseded: "substituído por um pedido mais novo" }
const STEP: Record<string, string> = { leader: "líder", it: "TI" }

/** Who asked, who approved or refused, and when — newest first (ADR 046). */
export function HistoryList({ requests }: { requests: ApprovalRequest[] }) {
  if (requests.length === 0) return <p className="text-sm text-muted-foreground">Nenhum pedido de aprovação ainda.</p>
  return (
    <ol className="divide-y divide-border rounded-xl border border-border bg-card text-sm">
      {requests.map((r) => (
        <li key={r.id} className="px-4 py-3">
          <p className="flex flex-wrap items-baseline justify-between gap-x-3">
            <span><span className="font-medium">{reasonLabel(r)[0]!.toUpperCase() + reasonLabel(r).slice(1)}</span> · {STATUS[r.status] ?? r.status}</span>
            <time className="text-xs text-muted-foreground">{formatWhen(r.created_at)}</time>
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {r.requested_by ? `Pedido por ${r.requested_by}` : "Pedido automático"}{r.channel === "mcp" ? " pelo Claude" : ""}
          </p>
          {r.steps.filter((s) => s.decision).map((s) => (
            <p key={s.step} className="mt-0.5 text-xs">
              {s.decision === "approved" ? "Aprovado" : "Recusado"} por {s.decided_by || "—"} ({STEP[s.step] ?? s.step}{s.on_behalf ? `, ${ON_BEHALF[s.on_behalf] ?? s.on_behalf}` : ""})
              {s.decided_at ? `, ${formatWhen(s.decided_at)}` : ""}{s.reason ? `: ${s.reason}` : ""}
            </p>
          ))}
          {r.risk_added.length > 0 && <p className="mt-0.5 text-xs text-muted-foreground">Risco novo: {r.risk_added.join("; ")}</p>}
        </li>
      ))}
    </ol>
  )
}
