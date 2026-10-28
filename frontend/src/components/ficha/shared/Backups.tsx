import { useCallback, useEffect, useState } from "react"
import { Download } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, backupDownloadUrl, type SharedBackup } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { formatBytes } from "@/lib/shared"
import { errText } from "./settings"

function backupKind(b: SharedBackup): string {
  switch (b.kind) {
    case "daily":
      return "diária"
    case "manual":
      return b.created_by_name ? `feita por ${b.created_by_name}` : "feita à mão"
    case "pre_deploy":
      return b.version_number != null ? `antes da versão ${b.version_number}` : "antes de uma versão nova"
    case "pre_restore":
      return "antes de restaurar"
  }
}

/** The app database's copies: make one now, download, restore (with a copy of now kept first). */
export function Backups({ appId, onRestored }: { appId: number; onRestored?: () => void }) {
  const [list, setList] = useState<SharedBackup[] | null>(null)
  const [error, setError] = useState("")
  const [making, setMaking] = useState(false)
  const [restoring, setRestoring] = useState<SharedBackup | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(() => {
    api.shared.backups(appId).then(
      (b) => {
        setList(b)
        setError("")
      },
      (e) => setError(errText(e, "Não consegui carregar as cópias.")),
    )
  }, [appId])
  useEffect(load, [load])

  const make = async () => {
    setMaking(true)
    try {
      await api.shared.backup(appId)
      toast.success("Cópia feita.")
      load()
    } catch (e) {
      toast.error(errText(e))
    } finally {
      setMaking(false)
    }
  }
  const restore = async () => {
    if (!restoring) return
    setBusy(true)
    try {
      await api.shared.restore(appId, restoring.id)
      setRestoring(null)
      toast.success("Dados restaurados.")
      load()
      onRestored?.()
    } catch (e) {
      toast.error(errText(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">O Hub faz uma cópia dos dados todo dia e antes de cada versão nova.</p>
      {error && !list && <p className="text-sm text-destructive" role="alert">{error}</p>}
      {list && list.length === 0 && <p className="text-sm text-muted-foreground">Ainda não há cópias.</p>}
      {list && list.length > 0 && (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {list.map((b) => (
            <li key={b.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 text-sm">
              <span className="min-w-0 flex-1">
                {formatWhen(b.created_at)} <span className="text-muted-foreground">· {backupKind(b)} · {formatBytes(b.size_bytes)}</span>
              </span>
              <a href={backupDownloadUrl(appId, b.id)} download className="inline-flex items-center gap-1 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline" aria-label={`Baixar a cópia de ${formatWhen(b.created_at)}`}>
                <Download className="size-3.5" /> Baixar
              </a>
              <Button size="sm" variant="ghost" onClick={() => setRestoring(b)} aria-label={`Restaurar a cópia de ${formatWhen(b.created_at)}`}>Restaurar</Button>
            </li>
          ))}
        </ul>
      )}
      <div>
        <Button size="sm" variant="outline" onClick={make} disabled={making}>{making ? "Fazendo a cópia…" : "Fazer cópia agora"}</Button>
      </div>

      <Dialog open={!!restoring} onOpenChange={(o) => !o && setRestoring(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Restaurar a cópia de {restoring && formatWhen(restoring.created_at)}?</DialogTitle>
            <DialogDescription>Os dados de agora são trocados pelos desta cópia. Antes disso o Hub guarda uma cópia de agora, para dar para desfazer.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRestoring(null)}>Cancelar</Button>
            <Button onClick={restore} disabled={busy}>Sim, restaurar</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
