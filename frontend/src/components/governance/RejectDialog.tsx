import { useEffect, useId, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import { api, type ApprovalRequest } from "@/lib/api"

export function RejectDialog({ request, open, onOpenChange, onDone }: {
  request: ApprovalRequest; open: boolean; onOpenChange: (v: boolean) => void; onDone: () => void
}) {
  const id = useId()
  const [reason, setReason] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (open) {
      setReason("")
      setError("")
    }
  }, [open])

  const confirm = async () => {
    setBusy(true)
    setError("")
    try {
      await api.rejectRequest(request.id, reason.trim())
      onOpenChange(false)
      onDone()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível recusar.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Recusar {request.item_name}</DialogTitle>
          <DialogDescription>
            {request.requested_by ? `${request.requested_by} vai ver o motivo.` : "O motivo fica registrado."}
            {request.reason === "new" ? "" : " A versão aprovada antes continua no ar."}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-1.5 text-sm">
          <label htmlFor={id} className="font-medium">Por que recusa?</label>
          <Textarea id={id} value={reason} onChange={(e) => setReason(e.target.value)} rows={3} />
        </div>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
          <Button variant="destructive" onClick={confirm} disabled={!reason.trim() || busy}>Recusar pedido</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
