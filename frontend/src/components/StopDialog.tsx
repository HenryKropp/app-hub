import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api } from "@/lib/api"
import type { RunStatus } from "@/lib/api"

export type StopTarget = { id: number; app_name: string; status: RunStatus }

// StopDialog confirms Parar: the app closes at once, with what it opened.
export function StopDialog({ target, onCancel, onStopped }: { target: StopTarget | null; onCancel: () => void; onStopped: (id: number) => void }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    setBusy(false)
    setError("")
  }, [target])

  const confirm = async () => {
    if (!target) return
    setBusy(true)
    setError("")
    try {
      await api.stopRun(target.id)
      onStopped(target.id)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível parar o app.")
      setBusy(false)
    }
  }

  const waiting = target?.status === "waiting"
  return (
    <Dialog open={!!target} onOpenChange={(v) => !v && !busy && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Parar o {target?.app_name}?</DialogTitle>
          <DialogDescription>
            {waiting
              ? "O app ainda não abriu. O Kova cancela o pedido e ele não abre mais."
              : "O app fecha agora no seu computador, junto com as janelas que ele abriu. O que não foi salvo se perde; os arquivos que ele já criou continuam na pasta."}
          </DialogDescription>
        </DialogHeader>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={busy}>{waiting ? "Deixar abrir" : "Continuar rodando"}</Button>
          <Button variant="destructive" onClick={confirm} disabled={busy}>{busy ? "Parando…" : "Parar"}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
