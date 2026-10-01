import { useCallback, useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { FileCode2, PencilLine, Play, RotateCcw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DescribeDialog } from "@/components/DescribeDialog"
import { RestoreDialog } from "@/components/RestoreDialog"
import { api, type HistoryFilter, type HistoryItem, type VersionInfo } from "@/lib/api"
import { formatWhen, STATUS } from "@/lib/runs"

const FILTERS: { value: HistoryFilter; label: string }[] = [
  { value: "all", label: "Tudo" },
  { value: "code", label: "Código" },
  { value: "runs", label: "Execuções" },
  { value: "ficha", label: "Ficha" },
]

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

// eslint-disable-next-line react-refresh/only-export-components
export function versionLine(v: VersionInfo): string {
  const parts: string[] = []
  if (v.summary.modified) parts.push(`${plural(v.summary.modified, "arquivo mudou", "arquivos mudaram")}`)
  if (v.summary.added) parts.push(plural(v.summary.added, "novo", "novos"))
  if (v.summary.removed) parts.push(plural(v.summary.removed, "removido", "removidos"))
  const code = parts.join(", ")
  const bins = (v.summary.binaries ?? []).map((b) => `${b} mudou`)
  return [code, ...bins].filter(Boolean).join(" · ") || "Sem mudança no código"
}

// eslint-disable-next-line react-refresh/only-export-components
export function versionOrigin(v: VersionInfo): string {
  if (v.origin === "original") return "Estado original"
  if (v.origin === "restore") return `Restaurada da versão ${v.restored_from}`
  return "Mudança na pasta"
}

function VersionRow({ appId, v, canEdit, onChanged }: { appId: number; v: VersionInfo; canEdit: boolean; onChanged: () => void }) {
  const [describe, setDescribe] = useState(false)
  const [restore, setRestore] = useState(false)
  return (
    <div className="flex gap-3">
      <FileCode2 className="mt-0.5 size-4 shrink-0 text-stamp" />
      <div className="min-w-0 flex-1">
        <p className="font-medium">
          Versão {v.number} <span className="font-normal text-muted-foreground">· {versionOrigin(v)}{v.current && " · atual"}</span>
        </p>
        <p className="text-muted-foreground">
          {formatWhen(v.created_at)} · {v.author_probable ? `provável: ${v.author_name}` : v.author_name || "autor não informado"}
        </p>
        {v.description && <p className="mt-1">{v.description}</p>}
        <p className="mt-1 text-muted-foreground">{versionLine(v)}</p>
        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm">
          <Link to={`/apps/${appId}/versoes/${v.number}`} className="underline underline-offset-4">Ver mudanças</Link>
          <Link to={`/apps/${appId}/versoes/${v.number}?ver=arquivos`} className="underline underline-offset-4">Ver como estava</Link>
          {canEdit && (
            <button type="button" onClick={() => setDescribe(true)} className="inline-flex items-center gap-1 underline underline-offset-4">
              <PencilLine className="size-3.5" /> Descrever
            </button>
          )}
          {canEdit && !v.current && (
            <button type="button" onClick={() => setRestore(true)} className="inline-flex items-center gap-1 text-stamp underline underline-offset-4">
              <RotateCcw className="size-3.5" /> Voltar o código para esta versão
            </button>
          )}
        </div>
        {canEdit && <DescribeDialog appId={appId} version={v} open={describe} onOpenChange={setDescribe} onSaved={onChanged} />}
        {canEdit && <RestoreDialog appId={appId} number={v.number} open={restore} onOpenChange={setRestore} onDone={onChanged} />}
      </div>
    </div>
  )
}

function Row({ appId, item, canEdit, onChanged }: { appId: number; item: HistoryItem; canEdit: boolean; onChanged: () => void }) {
  if (item.version) return <VersionRow appId={appId} v={item.version} canEdit={canEdit} onChanged={onChanged} />
  if (item.run) {
    const r = item.run
    return (
      <div className="flex gap-3">
        <Play className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
        <p>
          <Link to={`/execucoes/${r.id}`} className="font-medium underline-offset-4 hover:underline">{STATUS[r.status].label}</Link>{" "}
          <span className="text-muted-foreground">
            · {formatWhen(item.at)} · {r.person_name}{r.version_number ? ` · versão ${r.version_number}` : ""}
          </span>
        </p>
      </div>
    )
  }
  const f = item.ficha!
  return (
    <div className="flex gap-3">
      <PencilLine className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div>
        <p className="text-muted-foreground">Ficha · {formatWhen(item.at)} · {f.person_name}</p>
        <div className="mt-1">
          {f.changes.map((c) => (
            <p key={c.field}>
              {c.field === "created" ? c.label : c.long ? `${c.label} alterado` : `${c.label}: ${c.before || "—"} → ${c.after || "—"}`}
            </p>
          ))}
        </div>
      </div>
    </div>
  )
}

export function HistoryTimeline({ appId, canEdit }: { appId: number; canEdit: boolean }) {
  const [filter, setFilter] = useState<HistoryFilter>("all")
  const [items, setItems] = useState<HistoryItem[] | null>(null)
  const [more, setMore] = useState(false)
  const [next, setNext] = useState("")
  const [error, setError] = useState("")

  const load = useCallback(
    (f: HistoryFilter) => {
      setError("")
      api.history(appId, f).then(
        (h) => {
          setItems(h.items)
          setMore(h.more)
          setNext(h.next_before)
        },
        (e: Error) => setError(e.message),
      )
    },
    [appId],
  )

  useEffect(() => load(filter), [load, filter])

  const loadMore = () =>
    api.history(appId, filter, next).then(
      (h) => {
        setItems((cur) => [...(cur ?? []), ...h.items])
        setMore(h.more)
        setNext(h.next_before)
      },
      (e: Error) => setError(e.message),
    )

  return (
    <div>
      <div className="mb-4 flex flex-wrap gap-2" role="group" aria-label="Filtrar histórico">
        {FILTERS.map((f) => (
          <Button key={f.value} size="sm" variant={filter === f.value ? "default" : "outline"} aria-pressed={filter === f.value} onClick={() => setFilter(f.value)}>
            {f.label}
          </Button>
        ))}
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {items && items.length === 0 && <p className="text-sm text-muted-foreground">Nada por aqui ainda.</p>}
      {items && items.length > 0 && (
        <ul aria-label="Histórico" className="flex flex-col divide-y divide-border text-sm">
          {items.map((it, i) => (
            <li key={`${it.type}-${it.at}-${i}`} className="py-3">
              <Row appId={appId} item={it} canEdit={canEdit} onChanged={() => load(filter)} />
            </li>
          ))}
        </ul>
      )}
      {more && (
        <Button variant="outline" size="sm" className="mt-3" onClick={loadMore}>Carregar mais</Button>
      )}
    </div>
  )
}
