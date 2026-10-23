import { useEffect, useMemo, useState, type ComponentProps, type FormEvent } from "react"
import { AlertTriangle, FileCode2, FilePlus2, X } from "lucide-react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Field } from "@/components/bits"
import { InputsEditor, choiceProblems } from "@/components/ficha/editors"
import { api, ApiError, type Area, type Input as AppInput, type Manifest } from "@/lib/api"

const MAX_BYTES = 10 * 1024 * 1024
const MAX_FILES = 100
type UploadFile = File & { webkitRelativePath?: string }

function encodeFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error(`Não consegui ler ${file.name}.`))
    reader.onload = () => {
      const result = String(reader.result ?? "")
      resolve(result.slice(result.indexOf(",") + 1))
    }
    reader.readAsDataURL(file)
  })
}

function sourcePath(file: UploadFile) {
  return (file.webkitRelativePath || file.name).replaceAll("\\", "/")
}

export function ScriptImportForm() {
  const navigate = useNavigate()
  const [areas, setAreas] = useState<Area[]>([])
  const [area, setArea] = useState("")
  const [areasError, setAreasError] = useState("")
  const [loadingAreas, setLoadingAreas] = useState(true)
  const [areaTry, setAreaTry] = useState(0)
  const [name, setName] = useState("")
  const [summary, setSummary] = useState("")
  const [language, setLanguage] = useState<"python" | "node">("python")
  const [entrypoint, setEntrypoint] = useState("")
  const [dependenciesFile, setDependenciesFile] = useState("")
  const [files, setFiles] = useState<UploadFile[]>([])
  const [inputs, setInputs] = useState<AppInput[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [secretCount, setSecretCount] = useState(0)
  const [confirmSecrets, setConfirmSecrets] = useState(false)

  const resetSecretConfirmation = () => {
    setSecretCount(0)
    setConfirmSecrets(false)
  }

  const entrypoints = useMemo(() => files.filter((file) => {
    const path = sourcePath(file).toLowerCase()
    return language === "python" ? path.endsWith(".py") : path.endsWith(".js") || path.endsWith(".mjs") || path.endsWith(".cjs")
  }), [files, language])
  const totalBytes = useMemo(() => files.reduce((sum, file) => sum + file.size, 0), [files])

  useEffect(() => {
    let mounted = true
    setLoadingAreas(true)
    setAreasError("")
    api.areas().then((result) => {
      if (!mounted) return
      setAreas(result)
      setArea((current) => result.some((item) => item.name === current) ? current : result[0]?.name ?? "")
    }, (e: Error) => {
      if (mounted) setAreasError(e.message)
    }).finally(() => { if (mounted) setLoadingAreas(false) })
    return () => { mounted = false }
  }, [areaTry])

  const setLanguageValue = (next: "python" | "node") => {
    setLanguage(next)
    setEntrypoint("")
  }

  const chooseFiles = (list: FileList | null, append = false) => {
    resetSecretConfirmation()
    const next = Array.from(list ?? []) as UploadFile[]
    setFiles((current) => {
      if (!append) return next
      const all = [...current]
      next.forEach((file) => {
        const existing = all.findIndex((item) => sourcePath(item) === sourcePath(file))
        if (existing >= 0) all.splice(existing, 1, file)
        else all.push(file)
      })
      return all
    })
    setEntrypoint("")
    setDependenciesFile("")
    setError("")
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setError("")
    if (secretCount > 0 && !confirmSecrets) return
    if (totalBytes > MAX_BYTES) {
      setError("Os arquivos passam de 10 MB no total. Remova algum arquivo e tente novamente.")
      return
    }
    if (files.length > MAX_FILES) {
      setError("Selecione no máximo 100 arquivos.")
      return
    }
    const paths = files.map(sourcePath)
    if (new Set(paths).size !== paths.length) {
      setError("Há arquivos com o mesmo caminho. Escolha uma seleção sem nomes repetidos.")
      return
    }
    if (!entrypoints.some((file) => sourcePath(file) === entrypoint)) {
      setError("Escolha um arquivo de entrada compatível com a linguagem selecionada.")
      return
    }
    if (dependenciesFile && !paths.includes(dependenciesFile)) {
      setError("O arquivo de dependências precisa estar entre os arquivos enviados.")
      return
    }
    if (inputs.some((input) => !input.label.trim())) {
      setError("Dê um nome a cada campo que o script recebe.")
      return
    }
    const invalidChoice = choiceProblems(inputs)[0]
    if (invalidChoice) {
      setError(`Defina ${invalidChoice} antes de registrar.`)
      return
    }
    setBusy(true)
    try {
      const upload: Record<string, { name: string; content_base64: string }> = {}
      await Promise.all(files.map(async (file) => {
        upload[sourcePath(file)] = { name: file.name, content_base64: await encodeFile(file) }
      }))
      const manifest: Manifest = {
        manifest_version: 1,
        id: "",
        name: name.trim(),
        summary: summary.trim(),
        kind: "script",
        language,
        entrypoint,
        run: { command: "", url: "" },
        runtime: { language, dependencies_file: dependenciesFile, dependencies: null },
        inputs,
        outputs: [],
        secrets: null,
        schedule: "",
        area,
        owner: null,
        backup: null,
        tags: [],
        notes: ["Registrado pelo envio de arquivos do navegador."],
      }
      const created = await api.importScript({ manifest, files: upload, confirm_secrets: confirmSecrets })
      toast.success(`${name.trim()} está no catálogo`)
      navigate(`/apps/${created.id}`)
    } catch (e) {
      if (e instanceof ApiError && e.status === 409 && e.body && typeof e.body === "object" && "code" in e.body && e.body.code === "script_secrets_confirmation_required") {
        setSecretCount("secret_count" in e.body && typeof e.body.secret_count === "number" ? Math.max(1, e.body.secret_count) : 1)
        setConfirmSecrets(false)
      } else {
        setError(e instanceof Error ? e.message : "Não consegui registrar este script. Confira os arquivos e tente novamente.")
      }
    } finally {
      setBusy(false)
    }
  }

  return <section aria-labelledby="script-import-title" className="mb-8 rounded-xl border border-brand/20 bg-brand-soft/25 p-5 md:p-6">
    <div className="flex items-start gap-3">
      <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-brand-soft text-brand"><FileCode2 className="size-5" /></span>
      <div className="min-w-0">
        <h2 id="script-import-title" className="font-heading text-lg">Enviar um script do computador</h2>
        <p className="mt-1 max-w-3xl text-sm text-muted-foreground">Use este caminho quando a pasta do script não está acessível ao Hub. Envie o arquivo principal e os arquivos de que ele depende.</p>
      </div>
    </div>
    <div className="mt-4 flex gap-2 rounded-lg border border-warn/30 bg-warn-soft/60 p-3 text-sm">
      <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warn" />
      <p>Envie apenas scripts confiáveis. O Hub prepara uma pasta de trabalho separada, mas isso <strong>não é um sandbox do sistema operacional</strong>; o script ainda pode acessar recursos disponíveis para a conta que o executa.</p>
    </div>
    <form onSubmit={submit} onChange={(event) => { if ((event.target as HTMLElement).id !== "script-confirm-secrets") resetSecretConfirmation() }} className="mt-5 flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Nome do app" htmlFor="script-name">
          <Input id="script-name" value={name} onChange={(e) => setName(e.target.value)} required disabled={busy} placeholder="Ex.: Organizar relatórios" />
        </Field>
        <Field label="Linguagem" htmlFor="script-language">
          <select id="script-language" value={language} onChange={(e) => setLanguageValue(e.target.value as "python" | "node")} disabled={busy} className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm">
            <option value="python">Python (.py)</option><option value="node">Node.js (.js, .mjs ou .cjs)</option>
          </select>
        </Field>
        <Field label="Área" htmlFor="script-area" hint="O script será associado a uma área existente.">
          <select id="script-area" value={area} onChange={(e) => setArea(e.target.value)} disabled={busy || loadingAreas || areas.length === 0} required className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm">
            <option value="">{loadingAreas ? "Carregando áreas…" : "Escolha uma área"}</option>
            {areas.map((item) => <option key={item.id} value={item.name}>{item.name}</option>)}
          </select>
        </Field>
        <Field label="Resumo" htmlFor="script-summary" className="sm:col-span-2" hint="Explique em uma frase o que o script entrega.">
          <Textarea id="script-summary" value={summary} onChange={(e) => setSummary(e.target.value)} rows={2} disabled={busy} placeholder="O que a pessoa vai receber?" />
        </Field>
      </div>
      <Field label="Arquivos do script" htmlFor="script-files" hint="Selecione o arquivo de entrada e os arquivos locais que ele importa. Até 10 MB no total.">
        <div className="flex flex-wrap items-center gap-2">
          <Input id="script-files" type="file" multiple disabled={busy} className="h-auto min-h-10 max-w-lg bg-background py-2 file:mr-3 file:rounded file:border-0 file:bg-secondary file:px-2 file:py-1 file:text-xs file:font-medium" onChange={(e) => chooseFiles(e.target.files)} />
          <label htmlFor="script-folder-files" className="inline-flex h-9 items-center rounded-md border border-border px-3 text-sm font-medium hover:bg-accent">Adicionar pasta do script</label>
          <input id="script-folder-files" type="file" multiple disabled={busy} className="sr-only" {...({ webkitdirectory: "" } as ComponentProps<"input">)} onChange={(e) => chooseFiles(e.target.files, true)} />
        </div>
        {files.length > 0 && <ul className="mt-2 flex max-h-32 flex-col gap-1 overflow-y-auto text-xs text-muted-foreground">
          {files.map((file) => <li key={sourcePath(file)} className="flex items-center gap-2"><FilePlus2 className="size-3.5 shrink-0" /><span className="min-w-0 flex-1 truncate font-mono">{sourcePath(file)}</span><span className="shrink-0">{(file.size / 1024).toFixed(0)} KB</span><button type="button" aria-label={`Remover ${file.name}`} className="rounded p-1 hover:bg-accent" disabled={busy} onClick={() => { resetSecretConfirmation(); setFiles((current) => current.filter((item) => item !== file)); if (entrypoint === sourcePath(file)) setEntrypoint("") }}><X className="size-3.5" /></button></li>)}
        </ul>}
        <p className={totalBytes > MAX_BYTES ? "text-xs font-medium text-destructive" : "text-xs text-muted-foreground"} aria-live="polite">
          {files.length} / {MAX_FILES} arquivos · {(totalBytes / (1024 * 1024)).toFixed(1)} / 10 MB
          {totalBytes > MAX_BYTES && " — remova arquivos para continuar."}
        </p>
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Arquivo que inicia o app" htmlFor="script-entrypoint" hint="Python usa .py; Node.js usa .js, .mjs ou .cjs.">
          <select id="script-entrypoint" value={entrypoint} onChange={(e) => setEntrypoint(e.target.value)} required disabled={busy || entrypoints.length === 0} className="h-10 w-full rounded-md border border-input bg-background px-3 font-mono text-xs">
            <option value="">{entrypoints.length ? "Escolha um arquivo" : "Envie um arquivo compatível"}</option>
            {entrypoints.map((file) => <option key={sourcePath(file)} value={sourcePath(file)}>{sourcePath(file)}</option>)}
          </select>
        </Field>
        <Field label="Arquivo de dependências (opcional)" htmlFor="script-dependencies" hint={language === "python" ? "Ex.: requirements.txt. O Hub instala as bibliotecas em uma pasta isolada." : "Ex.: package.json; envie também o lockfile, se houver. O Hub usa npm sem scripts de instalação e sem dependências de desenvolvimento (até 5 min)."}>
          <select id="script-dependencies" value={dependenciesFile} onChange={(e) => setDependenciesFile(e.target.value)} disabled={busy} className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm">
            <option value="">Nenhum</option>
            {files.map((file) => <option key={sourcePath(file)} value={sourcePath(file)}>{sourcePath(file)}</option>)}
          </select>
        </Field>
      </div>
      {areasError && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{areasError} <button type="button" className="font-medium underline" onClick={() => setAreaTry((n) => n + 1)}>Tentar carregar áreas novamente</button></p>}
      {files.length > MAX_FILES && <p role="alert" className="text-sm text-destructive">Selecione no máximo 100 arquivos. Agora há {files.length}.</p>}
      <div className="border-t border-border pt-4">
        <h3 className="mb-1 text-sm font-semibold">O que o script recebe</h3>
        <p className="mb-3 text-xs text-muted-foreground">Crie campos para arquivos, pastas, texto, números, datas ou opções. O Hub entrega os valores conforme a origem escolhida.</p>
        <InputsEditor value={inputs} onChange={setInputs} />
      </div>
      {error && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}
      {secretCount > 0 && <div role="alert" className="rounded-lg border border-warn/30 bg-warn-soft/60 p-3 text-sm">
        <p className="font-semibold">Possíveis senhas ou chaves no código: {secretCount} ocorrência(s).</p>
        <p className="mt-1">Nada foi registrado. A detecção pode gerar falsos positivos e não garante encontrar todos os segredos. Recomendamos remover as credenciais dos arquivos originais e selecioná-los novamente. Os valores não são exibidos neste aviso.</p>
        <p className="mt-1">Se continuar, o código será salvo sem alterações no Hub e enviado ao computador que executar o script. Não há remoção automática de senhas nem cofre de credenciais.</p>
        <label htmlFor="script-confirm-secrets" className="mt-3 flex items-start gap-2">
          <input id="script-confirm-secrets" type="checkbox" checked={confirmSecrets} disabled={busy} onChange={(event) => setConfirmSecrets(event.target.checked)} />
          Revisei os arquivos e aceito enviar o código sem alterações, mesmo com este aviso.
        </label>
        <Button type="button" variant="outline" disabled={busy} className="mt-3" onClick={resetSecretConfirmation}>Cancelar confirmação</Button>
      </div>}
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-4">
        <p className="text-xs text-muted-foreground">Os arquivos selecionados serão enviados ao Hub para registrar este app.</p>
        <Button type="submit" disabled={busy || loadingAreas || !area || areas.length === 0 || !name.trim() || !entrypoint || files.length === 0 || (secretCount > 0 && !confirmSecrets)} className="gap-2">
          {busy ? "Enviando e registrando…" : "Registrar script"}
        </Button>
      </div>
    </form>
  </section>
}
