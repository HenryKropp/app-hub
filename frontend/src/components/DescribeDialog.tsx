import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import { Field, NativeSelect } from "@/components/bits"
import { api, type Person, type VersionInfo } from "@/lib/api"

export function DescribeDialog({ appId, version, open, onOpenChange, onSaved }: {
  appId: number; version: VersionInfo; open: boolean; onOpenChange: (v: boolean) => void; onSaved: () => void
}) {
  const [text, setText] = useState(version.description)
  const [people, setPeople] = useState<Person[]>([])
  const [author, setAuthor] = useState<number | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setText(version.description)
    api.people().then((ps) => {
      setPeople(ps)
      setAuthor(ps.find((p) => p.name === version.author_name)?.id ?? null)
    }, () => setPeople([]))
  }, [open, version])

  const save = async () => {
    setBusy(true)
    setError("")
    try {
      await api.describeVersion(appId, version.number, { description: text, author_id: author })
      onOpenChange(false)
      onSaved()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Descrever a versão {version.number}</DialogTitle>
          <DialogDescription>Conte o que mudou e quem fez. É opcional e pode ser mudado depois.</DialogDescription>
        </DialogHeader>
        <Field label="O que mudou?" htmlFor="version-desc">
          <Textarea id="version-desc" value={text} onChange={(e) => setText(e.target.value)} rows={4} maxLength={2000} />
        </Field>
        <Field label="Quem fez a mudança?" htmlFor="version-author">
          <NativeSelect id="version-author" value={author ?? ""} onChange={(e) => setAuthor(e.target.value ? Number(e.target.value) : null)}>
            <option value="">Não sei</option>
            {people.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </NativeSelect>
        </Field>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancelar</Button>
          <Button onClick={save} disabled={busy}>Salvar</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
