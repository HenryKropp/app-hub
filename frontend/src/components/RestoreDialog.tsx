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
    } catch (e) {
      const b = e instanceof ApiError && e.status === 409 ? (e.body as Partial<RestoreResult> | undefined) : undefined
      if (b && Array.isArray(b.done) && Array.isArray(b.failed)) {
        setResult({ number: b.number ?? 0, done: b.done, failed: b.failed, error: e instanceof Error ? e.message : "" })
      } else {
        setError(e instanceof Error ? e.message : "Não foi possível voltar o código.")
      }
    } finally {
      setBusy(false)
    }
  }

  // The timeline reloads only after the person has read the outcome.
  const changeOpen = (v: boolean) => {
    if (!v && result) onDone()
    onOpenChange(v)
  }

  const nothing = plan && plan.rewrite.length + plan.recreate.length + plan.to_trash.length === 0
  // A path in the trash list and in a "voltam" list has no copy in the Hub yet.
  const trashedFirst = plan ? plan.to_trash.some((p) => plan.rewrite.includes(p) || plan.recreate.includes(p)) : false

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Voltar o código para a versão {number}</DialogTitle>
          <DialogDescription>
            Antes de mexer, o Ziv guarda o estado atual como uma versão. Nada é apagado: arquivos a mais vão para a pasta .ziv-lixeira dentro do app.
          </DialogDescription>
        </DialogHeader>
        {!plan && !error && <p className="text-sm text-muted-foreground">Conferindo a pasta…</p>}
        {plan && !result && (
          <div className="flex flex-col gap-3 text-sm">
            {nothing && <p>O código já está igual ao da versão {number}.</p>}
            <Files title="Voltam ao que eram" list={plan.rewrite} />
            <Files title="Voltam a existir" list={plan.recreate} />
            <Files title="Vão para a lixeira" list={plan.to_trash} />
            {trashedFirst && (
              <p className="text-muted-foreground">Arquivos que ainda não têm cópia no Ziv vão para a lixeira antes de voltar.</p>
            )}
            {plan.binaries.length > 0 && (
              <p className="flex gap-2 text-warn">
                <TriangleAlert className="mt-0.5 size-4 shrink-0" />
                {plan.binaries.join(", ")} não volta. O Ziv guarda só a impressão digital de programas prontos. Recompile depois de voltar o código.
              </p>
            )}
          </div>
        )}
        {result && (
          <div className="flex flex-col gap-2 text-sm" role="status">
            {result.error ? (
              <p className="text-destructive">{result.error}</p>
            ) : result.failed.length === 0 ? (
              <p>Pronto. O código voltou para a versão {number} e foi registrado como versão {result.number}.</p>
            ) : (
              <>
                <p className="text-destructive">Parei no meio. A versão {result.number} registra como a pasta ficou.</p>
              </>
            )}
            {(result.error || result.failed.length > 0) && (
              <>
                <Files title="Voltaram" list={result.done} />
                <Files title="Não voltaram" list={result.failed.map((f) => `${f.path} (${f.error})`)} />
              </>
            )}
          </div>
        )}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          {result ? (
            <Button onClick={() => changeOpen(false)}>Fechar</Button>
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
