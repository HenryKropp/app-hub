import { useEffect, useMemo, useState, type ChangeEvent } from "react"
import { FolderOpen, LoaderCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { api, type Input as AppInput, type Manifest } from "@/lib/api"
import type { RunTarget } from "@/lib/runs-context"

type SelectedFile = { file: File; value: string }
export type Submission = {
  folder: string
  remember: boolean
  inputs?: Record<string, string | string[]>
  files?: Record<string, { name: string; content_base64: string }>
}
const MAX_FILE_BYTES = 10 * 1024 * 1024

function readBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error(`Não consegui ler o arquivo ${file.name}.`))
    reader.onload = () => {
      const result = String(reader.result ?? "")
      resolve(result.slice(result.indexOf(",") + 1))
    }
    reader.readAsDataURL(file)
  })
}

function ScriptField({ field, value, onChange, file, onFile, onError, disabled }: {
  field: AppInput
  value: string | string[] | undefined
  onChange: (value: string | string[]) => void
  file?: File
  onFile: (file?: File) => void
  onError: (message: string) => void
  disabled: boolean
}) {
  const id = `run-input-${field.id}`
  const hint = field.source === "argument"
    ? `O Ziv envia este valor como argumento${field.flag === "" ? " posicional" : ` ${field.flag || `--${field.id}`}`}.`
    : field.source === "prompt"
      ? "O Ziv envia este valor ao app quando ele solicitar uma resposta."
      : field.source === "dialog"
        ? `O Ziv disponibiliza o valor para o app em APP_HUB_INPUT_${field.id.toUpperCase()}; o app precisa ler essa variável.`
        : field.source === "fixed_file"
          ? `O arquivo será copiado para ${field.path || "o caminho definido no manifesto"}.`
          : undefined
  if (field.type === "file") {
    return <Field label={field.label} htmlFor={id} hint={`${hint ? `${hint} ` : ""}Até 10 MB no total por execução.`}>
      <Input id={id} type="file" className="h-auto min-h-10 bg-background py-2 file:mr-3 file:rounded file:border-0 file:bg-secondary file:px-2 file:py-1 file:text-xs file:font-medium" disabled={disabled} required={field.required} onChange={(e) => onFile(e.target.files?.[0])} />
      {file && <p className="text-xs text-muted-foreground">{file.name}</p>}
      {file && <button type="button" className="mt-1 text-xs text-muted-foreground underline" onClick={() => onFile(undefined)}>Remover arquivo</button>}
    </Field>
  }
  if (field.type === "folder") {
    return <Field label={field.label} htmlFor={id} hint={hint || "Escolha uma pasta no computador."}>
      <div className="flex gap-2">
        <Input id={id} value={typeof value === "string" ? value : ""} onChange={(e) => onChange(e.target.value)} placeholder="Caminho da pasta" required={field.required} disabled={disabled} />
        <Button type="button" variant="outline" className="shrink-0 gap-2" disabled={disabled} onClick={async () => {
          try {
            const result = await api.chooseFolder(typeof value === "string" ? value : "")
            if (!result.canceled) onChange(result.folder)
          } catch (e) {
            onError(e instanceof Error ? e.message : "Não consegui abrir a janela de pastas.")
          }
        }}><FolderOpen className="size-4" /> Escolher</Button>
      </div>
    </Field>
  }
  if (field.type === "choice") {
    const options = field.options ?? []
    if (field.multiple) {
      const selected = Array.isArray(value) ? value : []
      return <fieldset className="min-w-0">
        <legend className="mb-1.5 text-sm font-medium">{field.label}{field.required && <span aria-hidden="true"> *</span>}</legend>
        <div className="flex flex-wrap gap-2">{options.map((option) => <label key={option} className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm">
          <input type="checkbox" disabled={disabled} checked={selected.includes(option)} onChange={(e) => onChange(e.target.checked ? [...selected, option] : selected.filter((item) => item !== option))} />
          {option}
        </label>)}</div>
        {field.required && selected.length === 0 && <span className="sr-only">Selecione pelo menos uma opção.</span>}
      </fieldset>
    }
    return <Field label={field.label} htmlFor={id} hint={hint}>
      <select id={id} value={typeof value === "string" ? value : ""} onChange={(e) => onChange(e.target.value)} required={field.required} disabled={disabled} className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm">
        <option value="">Selecione</option>{options.map((option) => <option key={option} value={option}>{option}</option>)}
      </select>
    </Field>
  }
  return <Field label={field.label} htmlFor={id} hint={hint}>
    <Input id={id} type={field.type} value={typeof value === "string" ? value : ""} onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)} required={field.required} disabled={disabled} />
  </Field>
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
          <DialogTitle>Rodar {target?.name}</DialogTitle>
          <DialogDescription>{target?.kind === "script" ? "Preencha os dados pedidos pelo app e escolha onde salvar os resultados. A preparação das dependências pode levar até 5 minutos; a execução, até 15 minutos." : "O app vai abrir no seu computador. Use a janela normalmente e feche quando terminar. O Ziv mostra os arquivos que ele gerou."}</DialogDescription>
        </DialogHeader>
        {target?.kind === "script" && loadingManifest && <div className="space-y-3" aria-label="Carregando formulário"><div className="h-4 w-2/3 animate-pulse rounded bg-muted" /><div className="h-10 animate-pulse rounded bg-muted" /><div className="h-10 animate-pulse rounded bg-muted" /></div>}
        {target?.kind === "script" && !loadingManifest && manifest && scriptFields.length > 0 && (
          <div className="flex flex-col gap-4 border-y border-border py-4">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Dados para este app</p>
            {scriptFields.map((field) => <ScriptField key={field.id} field={field} value={values[field.id]} onChange={(value) => setValues((current) => ({ ...current, [field.id]: value }))} file={files[field.id]?.file} onFile={(file) => selectFile(field, file)} onError={setError} disabled={submitting} />)}
          </div>
        )}
        {target?.kind === "script" && !loadingManifest && manifest && scriptFields.length === 0 && <p className="rounded-lg bg-muted/60 p-3 text-sm text-muted-foreground">Este script não pede informações antes de rodar.</p>}
        <Field label="Onde salvar os resultados?" htmlFor="run-folder" hint="O Ziv mostra para baixar os arquivos novos que o app criar nesta pasta.">
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
