import { useState } from "react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import type { ApprovalRequest, ApprovalStep } from "@/lib/api"
import { relativeTime } from "@/lib/runs"
import { ApproveDialog } from "./ApproveDialog"
import { RejectDialog } from "./RejectDialog"

const REASON: Record<string, string> = { new: "novo", version: "versão nova", ficha: "ficha mudou", area: "mudou de área" }
export const reasonLabel = (r: ApprovalRequest) => REASON[r.reason] ?? r.reason
const channelLabel = (c: string) => (c === "mcp" ? "pelo Claude" : c === "folder" ? "da pasta do PC" : "pela tela")
const STEP: Record<string, string> = { leader: "Líder", it: "TI" }
/** Why someone other than the step's owner decided it (ADR 046). */
export const ON_BEHALF: Record<string, string> = { admin_no_leader: "administrador, a área não tem líder", self: "quem pediu é o líder" }

function StepState({ s }: { s: ApprovalStep }) {
  const who = s.decided_by ? ` ${s.decided_by}` : ""
  const text = s.decision === "approved" ? `✓${who}${s.on_behalf ? ` (${ON_BEHALF[s.on_behalf] ?? s.on_behalf})` : ""}` : s.decision === "rejected" ? `recusou${who}` : "— esperando"
  return (
    <span className="whitespace-nowrap">
      <span className="font-medium">{STEP[s.step] ?? s.step}</span>{" "}
      <span className={s.decision ? "" : "text-muted-foreground"}>{text}</span>
    </span>
  )
}

export const itemLink = (r: ApprovalRequest) => (r.kind === "app" ? `/apps/${r.item_id}` : `/artifacts/${r.item_id}`)

export function RequestCard({ request: r, onChanged }: { request: ApprovalRequest; onChanged: (msg: string) => void }) {
  const [approving, setApproving] = useState(false)
  const [rejecting, setRejecting] = useState(false)
  const canDecide = r.status === "open" && r.my_steps.length > 0
  return (
    <li className="px-4 py-3.5 text-sm">
      <div className="flex flex-wrap items-baseline gap-x-2">
        <Link to={itemLink(r)} className="font-medium hover:underline">{r.item_name}</Link>
        <span className="text-muted-foreground">· {reasonLabel(r)} · {r.area}</span>
      </div>
      <p className="mt-0.5 text-xs text-muted-foreground">
        {r.requested_by ? `Pedido por ${r.requested_by}, ` : ""}{relativeTime(r.created_at)} · {channelLabel(r.channel)}
        {r.kind === "app" && (r.tested ? " · testada ✓" : " · sem teste")}
      </p>
      {r.risk_added.length > 0 && (
        <div className="mt-2">
          <p className="text-xs font-medium">O que muda no risco:</p>
          <ul className="mt-0.5 list-disc pl-5 text-xs">{r.risk_added.map((x) => <li key={x}>{x}</li>)}</ul>
        </div>
      )}
      {r.steps.length > 0 && (
        <p className="mt-2 flex flex-wrap gap-x-5 text-xs">{r.steps.map((s) => <StepState key={s.step} s={s} />)}</p>
      )}
      {r.waiting && r.status !== "auto" && <p className="mt-1 text-xs text-muted-foreground">{r.waiting}</p>}
      {canDecide && (
        <div className="mt-3 flex gap-2">
          <Button size="sm" onClick={() => setApproving(true)}>Aprovar</Button>
          <Button size="sm" variant="outline" onClick={() => setRejecting(true)}>Recusar</Button>
          <ApproveDialog request={r} open={approving} onOpenChange={setApproving}
            onDone={(waiting) => onChanged(waiting ? `${r.item_name}: aprovado. ${waiting}` : `${r.item_name} foi aprovado.`)} />
          <RejectDialog request={r} open={rejecting} onOpenChange={setRejecting} onDone={() => onChanged(`${r.item_name}: pedido recusado.`)} />
        </div>
      )}
    </li>
  )
}
