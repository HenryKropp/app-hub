import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { Play, Square } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { EmptyState, LoadError, PageHeader, Playing, RunState } from "@/components/bits"
import { Elapsed } from "@/components/Elapsed"
import { api, type ActiveRun, type RunListItem } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useRuns } from "@/lib/runs-context"
import { formatDuration, formatWhen, isLive } from "@/lib/runs"
import { cn } from "@/lib/utils"

/** Rodando agora: the person's live runs, with a ticking timer and Parar. */
function LiveNow({ runs }: { runs: ActiveRun[] }) {
  const { askStop, stopping } = useRuns()
  return (
    <section aria-labelledby="rodando-agora" className="mb-8">
      <h2 id="rodando-agora" className="mb-2.5 font-heading text-base">Rodando agora</h2>
      <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
        {runs.map((r) => (
          <li key={r.id} className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 px-5 py-3">
            <div className="min-w-0">
              <Link to={`/execucoes/${r.id}`} className="inline-flex items-center gap-2 font-medium leading-snug hover:underline">
                <Playing /> {r.app_name}
              </Link>
              <p className="mt-0.5 text-sm text-muted-foreground">
                {r.status === "waiting" ? (
                  <>esperando o computador abrir há <Elapsed from={r.created_at} className="text-[0.8rem]" /></>
                ) : (
                  <>rodando há <Elapsed from={r.started_at ?? r.created_at} className="text-[0.8rem] text-foreground" /></>
                )}
                {r.computer_name && <> · {r.computer_name}</>}
              </p>
            </div>
            {stopping.has(r.id) ? (
              <span className="text-sm text-muted-foreground">Parando…</span>
            ) : r.can_stop && (
              <Button size="sm" variant="destructive" className="gap-1.5" onClick={() => askStop({ id: r.id, app_name: r.app_name, status: r.status })}>
                <Square className="size-3.5" /> Parar
              </Button>
            )}
          </li>
        ))}
      </ul>
    </section>
  )
}

export function MyRunsPage() {
  const { person } = useAuth()
  const { run, active, busy } = useRuns()
  const [scope, setScope] = useState<"mine" | "all">("mine")
  const [items, setItems] = useState<RunListItem[] | null>(null)
  const [more, setMore] = useState(false)
  const [error, setError] = useState("")
  const [tries, setTries] = useState(0)

  // Reload when the filter changes and when a run in the strip changes situation.
  const signal = active.map((r) => `${r.id}:${r.status}`).join(",")
  const live = active.filter((r) => isLive(r.status))
  // A run already in "Rodando agora" is not repeated in the list below.
  const listed = items?.filter((r) => !live.some((l) => l.id === r.id)) ?? null
  useEffect(() => {
    let gone = false
    api.runs(scope).then(
      (l) => {
        if (gone) return
        setItems(l.items)
        setMore(l.more)
      },
      (e: Error) => !gone && setError(e.message),
    )
    return () => {
      gone = true
    }
  }, [scope, signal, tries])

  const loadMore = () => {
    const last = items?.[items.length - 1]
    if (!last) return
    api.runs(scope, last.id).then(
      (l) => {
        setItems((xs) => [...(xs ?? []), ...l.items])
        setMore(l.more)
      },
      (e: Error) => setError(e.message),
    )
  }

  return (
    <>
      <PageHeader title={scope === "all" ? "O que todos rodaram" : "O que você rodou"}>
        {person?.sees_others_runs && (
          <div className="flex gap-0.5 rounded-lg bg-secondary p-0.5 text-sm">
            {(["mine", "all"] as const).map((s) => (
              <button
                key={s}
                type="button"
                aria-pressed={scope === s}
                onClick={() => setScope(s)}
                className={cn("rounded-md px-3 py-1", scope === s ? "bg-background font-medium text-foreground shadow-xs" : "text-muted-foreground hover:text-foreground")}
              >
                {s === "mine" ? "Minhas" : "De todos"}
              </button>
            ))}
          </div>
        )}
      </PageHeader>
      {live.length > 0 && <LiveNow runs={live} />}
      {error && <LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} />}
      {items === null && !error && <p className="text-muted-foreground">Carregando…</p>}
      {items?.length === 0 && (
        <EmptyState title={scope === "all" ? "Ninguém rodou nenhum app ainda" : "Você ainda não rodou nenhum app"}>
          No catálogo, clique em Rodar no cartão do app. O resultado aparece aqui.
        </EmptyState>
      )}
      {listed && listed.length > 0 && (
        <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
          {listed.map((r) => {
            return (
              <li key={r.id} className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 px-5 py-3.5 transition-colors hover:bg-muted/50">
                <div className="min-w-0">
                  <Link to={`/apps/${r.app_id}`} className="font-medium leading-snug hover:underline">
                    {r.app_name}
                  </Link>
                  <p className="mt-1 flex flex-wrap items-center gap-x-1.5 text-sm text-muted-foreground">
                    <RunState status={r.status} />
                    <span>· <span className="font-mono text-[0.8rem]">{formatWhen(r.created_at)}</span> · <span className="font-mono text-[0.8rem]">{formatDuration(r.started_at, r.finished_at)}</span></span>
                    {!isLive(r.status) && ` · ${r.file_count} ${r.file_count === 1 ? "arquivo" : "arquivos"}`}
                    {scope === "all" && ` · ${r.person_name}`}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Link to={`/execucoes/${r.id}`} className={buttonVariants({ variant: "outline", size: "sm" })}>
                    Ver e baixar
                  </Link>
                  {/* Someone else's folder is on their computer, not on mine. */}
                  {r.person_id === person?.id && (
                    <Button
                      size="sm"
                      variant="outline"
                      className="gap-1.5 [&_svg]:fill-current"
                      disabled={busy.has(r.app_id)}
                      onClick={() => run({ id: r.app_id, name: r.app_name, folder_display: r.output_folder, remembered_folder: "" }, { folder: r.output_folder })}
                    >
                      <Play className="size-3.5" /> Rodar de novo
                    </Button>
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      )}
      {more && (
        <Button variant="outline" className="mt-4" onClick={loadMore}>
          Carregar mais
        </Button>
      )}
    </>
  )
}
