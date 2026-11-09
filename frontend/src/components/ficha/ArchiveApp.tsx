import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { Archive } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api } from "@/lib/api"
import { cn } from "@/lib/utils"

// ArchiveApp is the quiet "Arquivar" at the foot of the app page (ADR 031),
// shown only to the owner or an administrator. It asks first, then goes back
// to the catalog, where the app no longer appears.
export function ArchiveApp({ appId, name, shared, className }: { appId: number; name: string; shared: boolean; className?: string }) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const navigate = useNavigate()

  const confirm = async () => {
    setBusy(true)
    setError("")
    try {
      await api.archiveApp(appId)
      toast.success(`${name} foi arquivado. Para trazer de volta, abra Apps › Arquivados.`)
      navigate("/apps")
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível arquivar o app.")
      setBusy(false)
    }
  }

  return (
    <div className={cn("flex flex-wrap items-center justify-between gap-3 border-t border-border pt-6", className)}>
      <p className="text-sm text-muted-foreground">Ninguém usa mais este app? Arquive: ele sai da lista e pode voltar por 30 dias.</p>
      <Button variant="outline" size="sm" className="gap-1.5" onClick={() => { setError(""); setOpen(true) }}>
        <Archive className="size-3.5" /> Arquivar
      </Button>
      <Dialog open={open} onOpenChange={(v) => !v && !busy && setOpen(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Arquivar o {name}?</DialogTitle>
            <DialogDescription>
              O app some da lista de apps, da tela inicial, da busca e do Claude, e ninguém consegue mais rodá-lo.
            </DialogDescription>
          </DialogHeader>
          <ul className="flex list-disc flex-col gap-1.5 pl-5 text-sm text-muted-foreground">
            <li>Os agendamentos dele ficam desligados.</li>
            {shared && <li>A versão no servidor é desligada. Os dados dela ficam guardados.</li>}
            <li>Você pode trazer de volta em Apps › Arquivados por 30 dias. Depois disso, o Ziv apaga o app de vez.</li>
            <li>A pasta do app no computador não é mexida.</li>
          </ul>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={busy}>Cancelar</Button>
            <Button variant="destructive" onClick={confirm} disabled={busy}>{busy ? "Arquivando…" : "Arquivar"}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
