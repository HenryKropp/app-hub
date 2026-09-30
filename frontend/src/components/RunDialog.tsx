import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { FolderOpen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { api, ApiError, type AppDetail } from "@/lib/api"

export function RunDialog({ app, open, onOpenChange }: { app: AppDetail; open: boolean; onOpenChange: (v: boolean) => void }) {
  const navigate = useNavigate()
  const [folder, setFolder] = useState(app.folder_display)
  const [error, setError] = useState<{ msg: string; offline: boolean } | null>(null)
  const [busy, setBusy] = useState<"" | "folder" | "run">("")

  const choose = async () => {
    setError(null)
    setBusy("folder")
    try {
      const r = await api.chooseFolder(folder)
      if (!r.canceled) setFolder(r.folder)
    } catch (e) {
      setError({ msg: e instanceof Error ? e.message : "Não foi possível abrir a janela de pastas.", offline: e instanceof ApiError && e.status === 409 })
    } finally {
      setBusy("")
    }
  }

  const start = async () => {
    setError(null)
    setBusy("run")
    try {
      const r = await api.startRun(app.id, folder)
      navigate(`/execucoes/${r.id}`)
    } catch (e) {
      setError({ msg: e instanceof Error ? e.message : "Não foi possível abrir o app.", offline: e instanceof ApiError && e.status === 409 })
      setBusy("")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rodar {app.manifest.name}</DialogTitle>
          <DialogDescription>O app vai abrir no seu computador. Use a janela normalmente e feche quando terminar. O Hub mostra os arquivos que ele gerou.</DialogDescription>
        </DialogHeader>
        <Field label="Onde salvar os resultados?" htmlFor="run-folder" hint="Por padrão, a própria pasta do app.">
          <div className="flex gap-2">
            <Input id="run-folder" value={folder} onChange={(e) => setFolder(e.target.value)} className="font-mono text-xs" />
            <Button type="button" variant="outline" onClick={choose} disabled={busy !== ""} className="shrink-0 gap-2">
              <FolderOpen className="size-4" /> Escolher outra…
            </Button>
          </div>
        </Field>
        {busy === "folder" && <p className="text-sm text-muted-foreground">A janela de pastas abriu no seu computador.</p>}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error.msg}{" "}
            {error.offline && <Link to="/meu-computador" className="underline">Ver Meu computador</Link>}
          </p>
        )}
        <DialogFooter>
          <Button onClick={start} disabled={busy !== "" || folder.trim() === ""}>Abrir app</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
