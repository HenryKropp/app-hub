import { useEffect, useMemo, useState } from "react"
import { FolderOpen, LoaderCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { api, type Input as AppInput, type Manifest } from "@/lib/api"
import { MAX_FILE_BYTES, ScriptField, readBase64, type SelectedFile } from "@/components/ScriptFields"
import type { RunTarget } from "@/lib/runs-context"

export type Submission = {
  folder: string
  remember: boolean
  inputs?: Record<string, string | string[]>
  files?: Record<string, { name: string; content_base64: string }>
}

export function RunDialog({ target, onCancel, onConfirm }: { target: RunTarget | null; onCancel: () => void; onConfirm: (submission: Submission) => void }) {
  const [folder, setFolder] = useState("")
  const [remember, setRemember] = useState(true)
  const [error, setError] = useState("")
  const [choosing, setChoosing] = useState(false)
  const [manifest, setManifest] = useState<Manifest | null>(null)
  const [loadingManifest, setLoadingManifest] = useState(false)
  const [values, setValues] = useState<Record<string, string | string[]>>({})
  const [files, setFiles] = useState<Record<string, SelectedFile>>({})
  const [submitting, setSubmitting] = useState(false)
  const web = target?.kind === "web"
  const scriptFields = useMemo(() => (manifest?.inputs ?? []).filter((item) => item.source !== "window"), [manifest])

  useEffect(() => {
    let mounted = true
    setFolder(target?.remembered_folder ?? "")
    setRemember(true)
    setError("")
    setManifest(null)
    setValues({})
    setFiles({})
    setSubmitting(false)
    if (target?.kind === "script") {
      setLoadingManifest(true)
      api.app(target.id).then((detail) => {
        if (mounted) setManifest(detail.manifest)
      }, (e: Error) => {
        if (mounted) setError(e.message)
      }).finally(() => {
        if (mounted) setLoadingManifest(false)
      })
    } else setLoadingManifest(false)
    return () => { mounted = false }
  }, [target])

  const chooseOutput = async () => {
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

  const selectFile = (field: AppInput, file?: File) => {
    setFiles((current) => {
      const next = { ...current }
      if (file) next[field.id] = { file, value: "" }
      else delete next[field.id]
      return next
    })
    setError("")
  }

  const submit = async () => {
    setError("")
    const missing = scriptFields.find((field) => {
      if (!field.required) return false
      if (field.type === "file") return !files[field.id]
      const value = values[field.id]
      return Array.isArray(value) ? value.length === 0 : !String(value ?? "").trim()
    })
    if (missing) {
      setError(`Preencha “${missing.label}” para continuar.`)
      return
    }
    setSubmitting(true)
    try {
      const selected = Object.values(files)
      if (selected.reduce((sum, item) => sum + item.file.size, 0) > MAX_FILE_BYTES) {
        throw new Error("Os arquivos selecionados passam de 10 MB no total. Remova algum arquivo e tente novamente.")
      }
      const encoded: Record<string, { name: string; content_base64: string }> = {}
      await Promise.all(Object.entries(files).map(async ([id, item]) => {
        encoded[id] = { name: item.file.name, content_base64: await readBase64(item.file) }
      }))
      const inputs = Object.fromEntries(Object.entries(values).filter(([, value]) => Array.isArray(value) ? value.length > 0 : value !== ""))
      onConfirm({ folder: folder.trim(), remember, ...(Object.keys(inputs).length ? { inputs } : {}), ...(Object.keys(encoded).length ? { files: encoded } : {}) })
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não consegui preparar os arquivos selecionados.")
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={target !== null} onOpenChange={(v) => !v && onCancel()}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{web ? "Abrir" : "Rodar"} {target?.name}</DialogTitle>
          <DialogDescription>{target?.kind === "script"
            ? "Preencha os dados pedidos pelo app e escolha onde salvar os resultados. A preparação das dependências pode levar até 5 minutos; a execução, até 15 minutos."
            : web
              ? "O app vai ligar no seu computador, com as senhas do Cofre, e abrir no navegador. Na primeira vez, preparar as bibliotecas pode levar alguns minutos. Para desligar, use Parar."
              : "O app vai abrir no seu computador. Use a janela normalmente e feche quando terminar. O Ziv mostra os arquivos que ele gerou."}</DialogDescription>
        </DialogHeader>
        {target?.kind === "script" && loadingManifest && <div className="space-y-3" aria-label="Carregando formulário"><div className="h-4 w-2/3 animate-pulse rounded bg-muted" /><div className="h-10 animate-pulse rounded bg-muted" /><div className="h-10 animate-pulse rounded bg-muted" /></div>}
        {target?.kind === "script" && !loadingManifest && manifest && scriptFields.length > 0 && (
          <div className="flex flex-col gap-4 border-y border-border py-4">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Dados para este app</p>
            {scriptFields.map((field) => <ScriptField key={field.id} field={field} value={values[field.id]} onChange={(value) => setValues((current) => ({ ...current, [field.id]: value }))} file={files[field.id]?.file} onFile={(file) => selectFile(field, file)} onError={setError} disabled={submitting} />)}
          </div>
        )}
        {target?.kind === "script" && !loadingManifest && manifest && scriptFields.length === 0 && <p className="rounded-lg bg-muted/60 p-3 text-sm text-muted-foreground">Este script não pede informações antes de rodar.</p>}
        <Field label={web ? "Onde o app guarda os dados?" : "Onde salvar os resultados?"} htmlFor="run-folder" hint={web ? "Pasta do seu computador para os arquivos que o app gravar (ele a recebe em APPHUB_DATA_DIR). O Ziv mostra o que for novo." : "O Ziv mostra para baixar os arquivos novos que o app criar nesta pasta."}>
          <div className="flex gap-2">
            <Input id="run-folder" value={folder} onChange={(e) => setFolder(e.target.value)} placeholder="Escolha uma pasta" title={folder} disabled={submitting} />
            <Button type="button" variant={folder.trim() ? "outline" : "default"} onClick={chooseOutput} disabled={choosing || submitting} className="shrink-0 gap-2">
              <FolderOpen className="size-4" /> {folder.trim() ? "Escolher outra…" : "Escolher pasta…"}
            </Button>
          </div>
          {folder.trim().length > 40 && <p className="break-all text-sm text-foreground/80">{folder.trim()}</p>}
        </Field>
        <label className="flex w-fit cursor-pointer items-center gap-2 text-sm">
          <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} className="size-4 shrink-0 accent-[var(--foreground)]" disabled={submitting} />
          Lembrar esta pasta
        </label>
        {choosing && <p className="text-sm text-muted-foreground">A janela de pastas abriu no seu computador.</p>}
        {error && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error} {target?.kind === "script" && manifest === null && !loadingManifest && <button type="button" className="underline" onClick={() => { if (target) { setLoadingManifest(true); api.app(target.id).then((detail) => { setManifest(detail.manifest); setError("") }, (e: Error) => setError(e.message)).finally(() => setLoadingManifest(false)) } }}>Tentar novamente</button>}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={submitting}>Cancelar</Button>
          <Button variant="run" className="pr-4 pl-4" onClick={() => void submit()} disabled={choosing || submitting || loadingManifest || folder.trim() === "" || (target?.kind === "script" && !manifest)}>
            {submitting && <LoaderCircle className="size-4 animate-spin" />} {submitting ? "Preparando…" : target?.kind === "script" ? "Executar script" : "Abrir app"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
