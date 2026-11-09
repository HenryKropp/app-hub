import { useState } from "react"
import { CircleCheck, CirclePause, Clock } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { api, type AppControl } from "@/lib/api"
import { cn } from "@/lib/utils"

/**
 * Whether the company lets this app run. Everyone sees why it can't run;
 * administrators approve, suspend and reactivate it right here.
 */
export function AppControlBar({ appId, control, onChanged, className = "mb-8" }: { appId: number; control?: AppControl | null; onChanged: (c: AppControl) => void; className?: string }) {
  const [busy, setBusy] = useState(false)
  if (!control) return null
  const pending = control.require_approval && !control.approved
  if (!control.suspended && !pending && !control.can_manage) return null

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
      ? { tone: "warn", icon: Clock, title: "Esperando aprovação", text: control.can_manage ? "Revise o app e aprove para a equipe poder rodar. Se o código mudar, ele volta para aprovação." : "Um administrador precisa aprovar este app antes de ele poder rodar." }
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
      {control.can_manage && (
        <div className="flex shrink-0 gap-2">
          {control.suspended ? (
            <Button size="sm" disabled={busy} onClick={() => void set({ approved: control.approved, suspended: false })}>Reativar</Button>
          ) : (
            <>
              {pending && <Button size="sm" disabled={busy} onClick={() => void set({ approved: true, suspended: false })}>Aprovar</Button>}
              <Button size="sm" variant="outline" disabled={busy} onClick={suspend}>Suspender</Button>
            </>
          )}
        </div>
      )}
    </div>
  )
}
