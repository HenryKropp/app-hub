import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, Download, LoaderCircle, X } from "lucide-react"
import { buttonVariants } from "@/components/ui/button"
import { api, fileUrl, groupZipUrl, selectedUrl, type Run, type RunGroup } from "@/lib/api"
import { formatDuration, formatSize, formatWhen, isLive, STATUS } from "@/lib/runs"
import { cn } from "@/lib/utils"

const SHOWN = 10

const CHECK = "size-4 shrink-0 cursor-pointer accent-[var(--stamp)]"

function Group({ run, g, selected, toggle }: { run: Run; g: RunGroup; selected: Set<number>; toggle: (ids: number[], on: boolean) => void }) {
  const [all, setAll] = useState(false)
  const files = all ? g.files : g.files.slice(0, SHOWN)
  return (
    <section className="border-t border-border py-5">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="flex items-center gap-3">
          {g.files.length > 0 && (
            <input
              type="checkbox"
              aria-label={`Marcar todos: ${g.label}`}
              className={CHECK}
              checked={g.files.every((f) => selected.has(f.id))}
              onChange={(e) => toggle(g.files.map((f) => f.id), e.target.checked)}
            />
          )}
          <h2 className="font-mono text-xs tracking-[0.16em] text-muted-foreground uppercase">{g.label} ({g.files.length})</h2>
        </div>
        {g.files.length > 0 && (
          <a href={groupZipUrl(run.id, g.index)} className={cn(buttonVariants({ variant: "outline", size: "sm" }), "gap-2")}>
            <Download className="size-4" /> Baixar tudo
          </a>
        )}
      </div>
      {g.files.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nada desta vez.</p>
      ) : (
        <ul className="flex flex-col gap-2 text-sm">
          {files.map((f) => (
            <li key={f.id} className="flex items-baseline gap-3">
              <input
                type="checkbox"
                aria-label={f.path.split("/").pop()}
                className={cn(CHECK, "translate-y-0.5")}
                checked={selected.has(f.id)}
                onChange={(e) => toggle([f.id], e.target.checked)}
              />
              <a href={fileUrl(run.id, f.id)} className="group min-w-0 flex-1">
                <span className="block truncate font-mono text-xs underline-offset-4 group-hover:underline">{f.path.split("/").pop()}</span>
                {f.path.includes("/") && (
                  <span className="block truncate text-[0.7rem] text-muted-foreground">{f.path.split("/").slice(0, -1).join(" / ")}</span>
                )}
              </a>
              <span className="shrink-0 text-xs text-muted-foreground">{f.change === "changed" ? "alterado · " : ""}{formatSize(f.size)}</span>
            </li>
          ))}
          {!all && g.files.length > SHOWN && (
            <li>
              <button type="button" onClick={() => setAll(true)} className="text-sm text-stamp underline-offset-4 hover:underline">
                … mais {g.files.length - SHOWN}
              </button>
            </li>
          )}
        </ul>
      )}
    </section>
  )
}

export function RunPage() {
  const { id } = useParams()
  const [run, setRun] = useState<Run | null>(null)
  const [error, setError] = useState("")
  const [, setNow] = useState(() => Date.now())
  const [selected, setSelected] = useState<Set<number>>(() => new Set())
  const toggle = (ids: number[], on: boolean) =>
    setSelected((cur) => {
      const next = new Set(cur)
      for (const i of ids) {
        if (on) next.add(i)
        else next.delete(i)
      }
      return next
    })

  useEffect(() => {
    if (!id) return
    let stop = false
    let live = true
    const load = () =>
      api.run(id).then(
        (r) => {
          if (stop) return
          live = isLive(r.status)
          setRun(r)
        },
        (e: Error) => !stop && setError(e.message),
      )
    load()
    const t = setInterval(() => {
      setNow(Date.now())
      if (live) load()
    }, 2000)
    return () => {
      stop = true
      clearInterval(t)
    }
  }, [id])

  if (error) return <p role="alert" className="text-destructive">{error}</p>
  if (!run) return <p className="text-muted-foreground">Carregando…</p>
  const st = STATUS[run.status]

  return (
    <article className="rise">
      <Link to={`/apps/${run.app_id}`} className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> {run.app_name}
      </Link>
      <div className="paper rounded-lg border border-border bg-card py-8 pr-8 pl-20 shadow-card">
        <p className="font-mono text-xs tracking-[0.16em] text-stamp uppercase">Execução</p>
        <h1 className="mt-2 font-heading text-3xl">{formatWhen(run.created_at)} · {formatDuration(run.started_at, run.finished_at)} · {run.person_name}</h1>
        <p className={cn("mt-3 inline-flex items-center gap-2 font-medium", st.tone === "bad" && "text-destructive", st.tone === "ok" && "text-ok", st.tone === "live" && "text-stamp")}>
          {isLive(run.status) && <LoaderCircle className="size-4 animate-spin" />}
          {st.label}
        </p>
        {run.status === "running" && <p className="mt-2 text-muted-foreground">Rodando: a janela está aberta no seu computador. Quando você fechar, os arquivos aparecem aqui.</p>}
        {run.status === "waiting" && <p className="mt-2 text-muted-foreground">Esperando o computador {run.computer_name} abrir o app…</p>}
        {(run.status === "error" || run.status === "interrupted") && run.error_message && (
          <p role="alert" className="mt-2 text-destructive">{run.error_message}</p>
        )}
        <dl className="mt-6 grid gap-x-8 gap-y-3 text-sm sm:grid-cols-3">
          <div><dt className="text-muted-foreground">Pasta</dt><dd className="font-mono text-xs break-all">{run.output_folder}</dd></div>
          <div><dt className="text-muted-foreground">Computador</dt><dd className="font-medium">{run.computer_name}</dd></div>
          <div><dt className="text-muted-foreground">Versão do app</dt><dd className="font-medium">{run.app_version || "—"}</dd></div>
        </dl>
        {!isLive(run.status) && run.groups.length > 0 && (
          <div className="mt-6">
            <p className="text-sm text-muted-foreground">Clique num arquivo para baixar só ele, ou marque os que quiser e baixe de uma vez.</p>
            {run.groups.map((g) => (
              <Group key={`${g.index}-${g.label}`} run={run} g={g} selected={selected} toggle={toggle} />
            ))}
          </div>
        )}
        {selected.size > 0 && (
          <div className="rise sticky bottom-4 z-10 mt-4 flex flex-wrap items-center justify-end gap-3 rounded-lg border border-border bg-card/95 p-3 shadow-card backdrop-blur">
            <button type="button" onClick={() => setSelected(new Set())} className="inline-flex cursor-pointer items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
              <X className="size-4" /> Limpar
            </button>
            <a href={selectedUrl(run.id, [...selected].sort((a, b) => a - b))} className={cn(buttonVariants(), "gap-2")}>
              <Download className="size-4" /> Baixar selecionados ({selected.size})
            </a>
          </div>
        )}
      </div>
    </article>
  )
}
