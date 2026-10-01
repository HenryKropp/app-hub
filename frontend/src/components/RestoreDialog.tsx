import { useEffect, useState } from "react"
import { TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, ApiError, type RestorePlan, type RestoreResult } from "@/lib/api"

function Files({ title, list }: { title: string; list: string[] }) {
  if (list.length === 0) return null
  return (
    <div>
      <p className="font-medium">{title}</p>
      <ul className="mt-1 font-mono text-xs text-muted-foreground">{list.map((p) => <li key={p}>{p}</li>)}</ul>
    </div>
  )
}

export function RestoreDialog({ appId, number, open, onOpenChange, onDone }: {
  appId: number; number: number; open: boolean; onOpenChange: (v: boolean) => void; onDone: () => void
}) {
  const [plan, setPlan] = useState<RestorePlan | null>(null)
  const [result, setResult] = useState<RestoreResult | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setPlan(null)
    setResult(null)
    setError("")
    api.restorePreview(appId, number).then((p) => setPlan(p.plan), (e: Error) => setError(e.message))
  }, [open, appId, number])

  const confirm = async () => {
    setBusy(true)
    setError("")
    try {
      setResult(await api.restore(appId, number))
      onDone()
    } catch (e) {
      setError(e instanceof ApiError || e instanceof Error ? e.message : "Não foi possível voltar o código.")
    } finally {
      setBusy(false)
    }
  }

  const nothing = plan && plan.rewrite.length + plan.recreate.length + plan.to_trash.length === 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Voltar o código para a versão {number}</DialogTitle>
          <DialogDescription>
            Antes de mexer, o Hub guarda o estado atual como uma versão. Nada é apagado: arquivos a mais vão para a pasta .app-hub-lixeira dentro do app.
          </DialogDescription>
        </DialogHeader>
        {!plan && !error && <p className="text-sm text-muted-foreground">Conferindo a pasta…</p>}
        {plan && !result && (
          <div className="flex flex-col gap-3 text-sm">
            {nothing && <p>O código já está igual ao da versão {number}.</p>}
            <Files title="Voltam ao que eram" list={plan.rewrite} />
            <Files title="Voltam a existir" list={plan.recreate} />
            <Files title="Vão para a lixeira" list={plan.to_trash} />
            {plan.binaries.length > 0 && (
              <p className="flex gap-2 text-warn">
                <TriangleAlert className="mt-0.5 size-4 shrink-0" />
                {plan.binaries.join(", ")} não volta. O Hub guarda só a impressão digital de programas prontos. Recompile depois de voltar o código.
              </p>
            )}
          </div>
        )}
        {result && (
          <div className="flex flex-col gap-2 text-sm" role="status">
            {result.failed.length === 0 ? (
              <p>Pronto. O código voltou para a versão {number} e foi registrado como versão {result.number}.</p>
            ) : (
              <>
                <p className="text-destructive">Parei no meio. A versão {result.number} registra como a pasta ficou.</p>
                <Files title="Voltaram" list={result.done} />
                <Files title="Não voltaram" list={result.failed.map((f) => `${f.path} (${f.error})`)} />
              </>
            )}
          </div>
        )}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          {result ? (
            <Button onClick={() => onOpenChange(false)}>Fechar</Button>
          ) : (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
              <Button onClick={confirm} disabled={!plan || busy || !!nothing}>Voltar o código</Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
