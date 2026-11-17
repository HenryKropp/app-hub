import { useState } from "react"
import { Link } from "react-router-dom"
import { CircleCheck, CirclePause, CircleX, Clock } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { api, type AppControl, type ItemGovernance } from "@/lib/api"
import { cn } from "@/lib/utils"

/**
 * Whether the app may run. Everyone sees why it can't (who still has to
 * approve, or who refused and why); approving happens only in Aprovações
 * (ADR 046). Administrators suspend and reactivate it here.
 */
export function AppControlBar({ appId, control, gov, canApprove = true, onChanged, className = "mb-8" }: {
  appId: number; control?: AppControl | null; gov?: ItemGovernance | null; canApprove?: boolean; onChanged: (c: AppControl) => void; className?: string
}) {
  const [busy, setBusy] = useState(false)
  if (!control) return null
  const open = gov?.open_request ?? null
  const pending = gov ? !!open : !control.approved
  const last = gov?.history[0]
  const refusal = !pending && last?.status === "rejected" ? last.steps.find((s) => s.decision === "rejected") : undefined
  if (!control.suspended && !pending && !refusal && !control.can_manage) return null

  const set = async (next: { approved: boolean; suspended: boolean }) => {
    setBusy(true)
    try {
      onChanged(await api.setAppControl(appId, next))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível salvar.")
    } finally {
      setBusy(false)
    }
  }
  const suspend = () => {
    if (window.confirm("Suspender este app? Ninguém da empresa vai conseguir rodá-lo até você reativar.")) void set({ approved: control.approved, suspended: true })
  }

  const state = control.suspended
    ? { tone: "bad", icon: CirclePause, title: "App suspenso", text: control.can_manage ? "Ninguém da empresa consegue rodar este app." : "Um administrador suspendeu este app. Ele não pode ser rodado por enquanto." }
    : pending
      ? { tone: "warn", icon: Clock, title: "Esperando aprovação", text: open?.waiting || "Este app espera aprovação antes de rodar." }
      : refusal
        ? { tone: "bad", icon: CircleX, title: "Pedido recusado", text: `${refusal.decided_by || "Quem aprova"} recusou: ${refusal.reason}${control.approved ? " A versão aprovada antes continua no ar." : ""}` }
        : { tone: "ok", icon: CircleCheck, title: "Aprovado", text: "A equipe pode rodar este app." }
  const Icon = state.icon

  return (
    <div
      role="status"
      className={cn(
        "flex flex-col gap-3 rounded-xl border px-4 py-3 sm:flex-row sm:items-center sm:justify-between",
        className,
        state.tone === "bad" && "border-destructive/30 bg-destructive/5",
        state.tone === "warn" && "border-warn/35 bg-warn-soft",
        state.tone === "ok" && "border-border bg-card",
      )}
    >
      <p className="flex items-start gap-2.5 text-sm">
        <Icon className={cn("mt-0.5 size-4 shrink-0", state.tone === "bad" && "text-destructive", state.tone === "warn" && "text-warn", state.tone === "ok" && "text-ok")} />
        <span><span className="font-medium">{state.title}.</span> <span className="text-muted-foreground">{state.text}</span></span>
      </p>
      <div className="flex shrink-0 items-center gap-3">
        {pending && canApprove && <Link to="/aprovacoes" className="text-sm underline-offset-4 hover:underline">Ver em Aprovações</Link>}
        {control.can_manage && (control.suspended ? (
          <Button size="sm" disabled={busy} onClick={() => void set({ approved: control.approved, suspended: false })}>Reativar</Button>
        ) : (
          <Button size="sm" variant="outline" disabled={busy} onClick={suspend}>Suspender</Button>
        ))}
      </div>
    </div>
  )
}
