import { useEffect, useState } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"
import { ArrowLeft, PencilLine, RotateCcw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DescribeDialog } from "@/components/DescribeDialog"
import { DiffView } from "@/components/DiffView"
import { RestoreDialog } from "@/components/RestoreDialog"
import { versionLine, versionOrigin } from "@/components/HistoryTimeline"
import { api, type AppDetail, type FileChange, type VersionFile, type VersionInfo } from "@/lib/api"
import { LoadError } from "@/components/bits"
import { formatSize, formatWhen } from "@/lib/runs"

const STATUS_LABEL: Record<FileChange["status"], string> = { added: "novo", removed: "removido", modified: "alterado" }

function Changes({ files }: { files: FileChange[] }) {
  if (files.length === 0) return <p className="text-sm text-muted-foreground">Nenhum arquivo mudou.</p>
  return (
    <div className="flex flex-col gap-5">
      {files.map((f) => (
        <section key={f.path}>
          <p className="mb-2 text-sm break-all">
            <code className="font-mono text-xs">{f.path}</code> <span className="text-muted-foreground">· {STATUS_LABEL[f.status]}</span>
          </p>
          {f.kind === "binary" ? (
            <p className="text-sm text-muted-foreground">
              {f.status === "modified" ? `mudou (${formatSize(f.old_size ?? 0)} → ${formatSize(f.new_size ?? 0)})` : formatSize((f.new_size ?? f.old_size) ?? 0)}
            </p>
          ) : (
            <DiffView hunks={f.hunks} />
          )}
        </section>
      ))}
    </div>
  )
}

function AsItWas({ appId, n, files }: { appId: string; n: string; files: VersionFile[] }) {
  const [open, setOpen] = useState<{ path: string; text: string } | null>(null)
  const [error, setError] = useState("")
  const show = (path: string) => {
    setError("")
    api.versionContent(appId, n, path).then(setOpen, (e: Error) => setError(e.message))
  }
  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4 md:grid-cols-[minmax(0,16rem)_minmax(0,1fr)]">
      <ul className="flex min-w-0 flex-col gap-1 text-sm">
        {files.map((f) => (
          <li key={f.path}>
            <button type="button" disabled={f.kind === "binary"} onClick={() => show(f.path)}
              className={`w-full truncate text-left font-mono text-xs ${open?.path === f.path ? "font-medium text-foreground" : ""} disabled:text-muted-foreground`}
              title={f.kind === "binary" ? "Programa pronto: o Ziv guarda só o tamanho e a data" : f.path}>
              {f.path}
            </button>
          </li>
        ))}
      </ul>
      <div className="min-w-0">
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        {open ? (
          <pre className="overflow-x-auto rounded-md border border-border p-3 font-mono text-xs whitespace-pre">{open.text}</pre>
        ) : (
          <p className="text-sm text-muted-foreground">Escolha um arquivo para ver como ele estava.</p>
        )}
      </div>
    </div>
  )
}

export function VersionPage() {
  const { id = "", n = "" } = useParams()
  const [params, setParams] = useSearchParams()
  const view = params.get("ver") === "arquivos" ? "arquivos" : "mudancas"
  const [app, setApp] = useState<AppDetail | null>(null)
  const canEdit = app?.can.edit ?? false
  const canRestore = app?.can.restore ?? false
  const [tries, setTries] = useState(0)
  const [info, setInfo] = useState<VersionInfo | null>(null)
  const [changes, setChanges] = useState<FileChange[] | null>(null)
  const [files, setFiles] = useState<VersionFile[] | null>(null)
  const [error, setError] = useState("")
  const [describe, setDescribe] = useState(false)
  const [restore, setRestore] = useState(false)
  const [reload, setReload] = useState(0)

  useEffect(() => {
    setError("")
    api.app(id).then(setApp, (e: Error) => setError(e.message))
    if (view === "mudancas") {
      api.versionChanges(id, n).then((r) => { setInfo(r.version); setChanges(r.files) }, (e: Error) => setError(e.message))
    } else {
      api.version(id, n).then(setInfo, (e: Error) => setError(e.message))
      api.versionFiles(id, n).then(setFiles, (e: Error) => setError(e.message))
    }
  }, [id, n, view, reload, tries])

  if (error) return <LoadError message={error} onRetry={() => setTries((x) => x + 1)} />
  if (!info) return <p className="text-muted-foreground">Carregando…</p>

  return (
    <article>
      <Link to={`/apps/${id}`} className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Ficha do app
      </Link>
      <div className="rounded-xl border border-border bg-card p-8">
        <h1 className="font-heading text-3xl">Versão {info.number}{info.current && <span className="ml-2 text-base text-muted-foreground">(atual)</span>}</h1>
        <p className="mt-2 text-muted-foreground">
          {versionOrigin(info)} · {formatWhen(info.created_at)} · {info.author_probable ? `provável: ${info.author_name}` : info.author_name || "autor não informado"}
        </p>
        {info.description && <p className="mt-2">{info.description}</p>}
        <p className="mt-1 text-sm text-muted-foreground">{versionLine(info)}</p>
        {(canEdit || canRestore) && (
          <div className="mt-4 flex flex-wrap gap-2">
            {canEdit && <Button variant="outline" className="gap-2" onClick={() => setDescribe(true)}><PencilLine className="size-4" /> Descrever</Button>}
            {canRestore && !info.current && (
              <Button className="gap-2" onClick={() => setRestore(true)}><RotateCcw className="size-4" /> Voltar o código para esta versão</Button>
            )}
            <DescribeDialog appId={Number(id)} version={info} open={describe} onOpenChange={setDescribe} onSaved={() => setReload((x) => x + 1)} />
            <RestoreDialog appId={Number(id)} number={info.number} open={restore} onOpenChange={setRestore} onDone={() => setReload((x) => x + 1)} />
          </div>
        )}
        <div className="mt-6 mb-4 flex gap-2 border-t border-border pt-5" role="tablist">
          <Button role="tab" aria-selected={view === "mudancas"} size="sm" variant={view === "mudancas" ? "default" : "outline"} onClick={() => setParams({})}>O que mudou</Button>
          <Button role="tab" aria-selected={view === "arquivos"} size="sm" variant={view === "arquivos" ? "default" : "outline"} onClick={() => setParams({ ver: "arquivos" })}>Como estava</Button>
        </div>
        {view === "mudancas" && info.origin === "original" ? (
          <p className="text-sm text-muted-foreground">
            Esta é a primeira versão: como a pasta estava quando o Ziv começou a acompanhar o app. Para ver os arquivos, abra{" "}
            <button type="button" className="underline underline-offset-4 hover:text-foreground" onClick={() => setParams({ ver: "arquivos" })}>Como estava</button>.
          </p>
        ) : view === "mudancas" ? changes && <Changes files={changes} /> : files && <AsItWas appId={id} n={n} files={files} />}
      </div>
    </article>
  )
}
