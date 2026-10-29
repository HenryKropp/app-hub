import { Check, CircleAlert, X } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Textarea } from "@/components/ui/textarea"
import type { Readiness, ReadinessItem } from "@/lib/api"
import { claudeTextFor } from "@/lib/shared"
import { cn } from "@/lib/utils"

function ItemIcon({ status }: { status: ReadinessItem["status"] }) {
  if (status === "missing") return <X aria-label="Falta" className="mt-0.5 size-4 shrink-0 text-destructive" />
  if (status === "warn") return <CircleAlert aria-label="Atenção" className="mt-0.5 size-4 shrink-0 text-warn" />
  return <Check aria-label="Certo" className="mt-0.5 size-4 shrink-0 text-ok" />
}

/** The readiness checklist: one line per item, the text to paste into Claude, and "estou ciente" on warnings. */
export function ReadinessList({ report, acks, onAck, onRecheck, busy }: {
  report: Readiness; acks: string[]; onAck: (acks: string[]) => void; onRecheck?: () => void; busy?: boolean
}) {
  const items = report.items ?? []
  const todo = items.filter((i) => (i.status === "missing" || i.status === "warn") && i.claude_text)
  const text = todo.length > 0 ? claudeTextFor(items) : ""
  const marked = (i: ReadinessItem) => i.acked || acks.includes(i.key)
  const pending = items.some((i) => i.status === "missing" || (i.status === "warn" && !marked(i)))
  const toggle = (i: ReadinessItem, on: boolean) => {
    const rest = acks.filter((k) => k !== i.key)
    onAck(on ? [...rest, i.key] : rest)
  }
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success("Copiado. Cole no Claude, na pasta do app.")
    } catch {
      toast.error("Não consegui copiar. Selecione o texto e copie com Ctrl+C.")
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {items.length > 0 && (
        <ul className="flex flex-col gap-3">
          {items.map((i) => (
            <li key={i.key} className="flex gap-2.5 text-sm">
              <ItemIcon status={i.status} />
              <div className="min-w-0 flex-1">
                <p className="font-medium">
                  {i.title}
                  {i.status === "auto" && <span className="ml-2 font-normal text-muted-foreground">o Ziv resolve</span>}
                </p>
                {i.detail && <p className={cn("mt-0.5 text-muted-foreground", i.status === "missing" && "text-foreground/80")}>{i.detail}</p>}
                {i.status === "warn" && (
                  <label className="mt-1.5 inline-flex cursor-pointer items-center gap-2 text-sm">
                    <Checkbox checked={marked(i)} disabled={busy} onCheckedChange={(c) => toggle(i, c === true)} />
                    <span>Estou ciente<span className="sr-only">: {i.title}</span></span>
                  </label>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
      {report.ready && !pending && <p className="text-sm text-ok">Tudo certo para publicar.</p>}
      {text && (
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between gap-3">
            <span className="text-sm font-medium">Texto para o Claude</span>
            <Button size="sm" variant="outline" onClick={copy}>Copiar</Button>
          </div>
          <Textarea readOnly aria-label="Texto para o Claude" value={text} rows={Math.min(10, todo.length + 3)} className="font-mono text-xs" />
          <p className="text-xs text-muted-foreground">Depois que o Claude mudar o app, clique em Conferir de novo.</p>
        </div>
      )}
      {onRecheck && (text || !report.ready) && (
        <div>
          <Button size="sm" variant="outline" onClick={onRecheck} disabled={busy}>Conferir de novo</Button>
        </div>
      )}
    </div>
  )
}
