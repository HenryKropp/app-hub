import { useEffect, useState } from "react"
import { FolderOpen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { api } from "@/lib/api"
import type { RunTarget } from "@/lib/runs-context"

// RunDialog asks where to save the results. The RunsProvider opens it and runs
// the app with the answer. There is no default folder: the first time, the
// person picks one (Henry's call, 2026-10-02).
export function RunDialog({ target, onCancel, onConfirm }: { target: RunTarget | null; onCancel: () => void; onConfirm: (folder: string, remember: boolean) => void }) {
  const [folder, setFolder] = useState("")
  const [remember, setRemember] = useState(true)
  const [error, setError] = useState("")
  const [choosing, setChoosing] = useState(false)

  useEffect(() => {
    if (target) {
      setFolder(target.remembered_folder)
      setRemember(true)
      setError("")
    }
  }, [target])

  const choose = async () => {
    setError("")
    setChoosing(true)
    try {
      const r = await api.chooseFolder(folder)
      if (!r.canceled) setFolder(r.folder)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível abrir a janela de pastas.")
    } finally {
      setChoosing(false)
    }
  }

  return (
    <Dialog open={target !== null} onOpenChange={(v) => !v && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rodar {target?.name}</DialogTitle>
          <DialogDescription>O app vai abrir no seu computador. Use a janela normalmente e feche quando terminar. O Hub mostra os arquivos que ele gerou.</DialogDescription>
        </DialogHeader>
        <Field label="Onde salvar os resultados?" htmlFor="run-folder" hint="O Hub mostra para baixar os arquivos novos que o app criar nesta pasta.">
          <div className="flex gap-2">
            <Input id="run-folder" value={folder} onChange={(e) => setFolder(e.target.value)} placeholder="Escolha uma pasta" title={folder} />
            <Button type="button" variant={folder.trim() ? "outline" : "default"} onClick={choose} disabled={choosing} className="shrink-0 gap-2">
              <FolderOpen className="size-4" /> {folder.trim() ? "Escolher outra…" : "Escolher pasta…"}
            </Button>
          </div>
          {/* Long OneDrive paths don't fit the field; spell them out in full. */}
          {folder.trim().length > 40 && <p className="text-sm break-all text-foreground/80">{folder.trim()}</p>}
        </Field>
        <label className="flex w-fit cursor-pointer items-center gap-2 text-sm">
          <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} className="size-4 shrink-0 cursor-pointer accent-[var(--foreground)]" />
          Lembrar esta pasta
        </label>
        {choosing && <p className="text-sm text-muted-foreground">A janela de pastas abriu no seu computador.</p>}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>
            Cancelar
          </Button>
          <Button variant="run" className="pr-4 pl-4" onClick={() => onConfirm(folder.trim(), remember)} disabled={choosing || folder.trim() === ""}>
            Abrir app
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
