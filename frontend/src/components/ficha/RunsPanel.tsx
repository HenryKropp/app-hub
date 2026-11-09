import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { ChevronRight } from "lucide-react"
import { RunState } from "@/components/bits"
import { Elapsed } from "@/components/Elapsed"
import { api, type RunSummary } from "@/lib/api"
import { formatDuration, formatWhen, isLive } from "@/lib/runs"

/** While something runs the list asks again this often, so it moves to the recent ones when it ends. */
const LIVE_POLL_MS = 3000

/**
 * Execuções in the Cuidar face (ADR 033): what is running now, with a ticking
 * timer, and then the recent runs. everyone: the person sees everyone's runs
 * of this app (can.runs); otherwise only their own, as on the Usar face.
 * reloadKey changes when the page learns that a run changed.
 */
export function RunsPanel({ appId, everyone, reloadKey }: { appId: number; everyone: boolean; reloadKey?: string }) {
  const [runs, setRuns] = useState<RunSummary[] | null>(null)
  const [tick, setTick] = useState(0)
  const live = (runs ?? []).filter((r) => isLive(r.status))
  const anyLive = live.length > 0

  useEffect(() => {
    let gone = false
    api.appRuns(appId).then(
      (r) => !gone && setRuns(r),
      () => !gone && setRuns((cur) => cur ?? []),
    )
    return () => { gone = true }
  }, [appId, reloadKey, tick])

  useEffect(() => {
    if (!anyLive) return
    const t = setInterval(() => setTick((n) => n + 1), LIVE_POLL_MS)
    return () => clearInterval(t)
  }, [anyLive])

  if (runs === null) return <p className="text-sm text-muted-foreground">Carregando…</p>
  if (runs.length === 0) return <p className="text-sm text-muted-foreground">{everyone ? "Ninguém rodou este app ainda." : "Você ainda não rodou este app."}</p>
  const recent = runs.filter((r) => !isLive(r.status))

  return (
    <div className="flex flex-col gap-8">
      {anyLive && (
        <section aria-labelledby="execucoes-agora">
          <h2 id="execucoes-agora" className="mb-2.5 font-heading text-base">Rodando agora</h2>
          <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card text-sm">
            {live.map((r) => (
              <li key={r.id}>
                <Link to={`/execucoes/${r.id}`} className="group flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2.5 hover:bg-accent/60">
                  <RunState status={r.status} />
                  <span className="text-muted-foreground">
                    há <Elapsed from={r.started_at ?? r.created_at} className="text-[0.8rem] text-foreground" />
                  </span>
                  {everyone && <span>{r.person_name}</span>}
                  <ChevronRight className="ml-auto size-4 text-muted-foreground group-hover:text-foreground" />
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}
      {recent.length > 0 && (
        <section aria-labelledby="execucoes-recentes">
          <h2 id="execucoes-recentes" className="mb-2.5 font-heading text-base">{everyone ? "Recentes" : "Suas execuções recentes"}</h2>
          <div className="overflow-x-auto rounded-xl border border-border bg-card">
            <table className="w-full text-sm">
              <thead className="text-left text-xs text-muted-foreground">
                <tr className="border-b border-border">
                  <th className="px-4 py-2 font-normal">Situação</th>
                  <th className="px-4 py-2 font-normal">Começou</th>
                  <th className="px-4 py-2 font-normal">Duração</th>
                  {everyone && <th className="px-4 py-2 font-normal">Quem</th>}
                  <th className="px-4 py-2 text-right font-normal">Arquivos</th>
                  <th className="w-8 px-2 py-2"><span className="sr-only">Abrir</span></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {recent.map((r) => (
                  <tr key={r.id} className="hover:bg-accent/60">
                    <td className="px-4 py-2.5 whitespace-nowrap"><RunState status={r.status} /></td>
                    <td className="px-4 py-2.5 font-mono text-[0.8rem] whitespace-nowrap">
                      <Link to={`/execucoes/${r.id}`} className="underline-offset-4 hover:underline">{formatWhen(r.started_at ?? r.created_at)}</Link>
                    </td>
                    <td className="px-4 py-2.5 font-mono text-[0.8rem] whitespace-nowrap">{formatDuration(r.started_at, r.finished_at)}</td>
                    {everyone && <td className="px-4 py-2.5">{r.person_name}</td>}
                    <td className="px-4 py-2.5 text-right tabular-nums">{r.file_count}</td>
                    <td className="px-2 py-2.5 text-right">
                      <Link to={`/execucoes/${r.id}`} tabIndex={-1} aria-hidden className="inline-flex text-muted-foreground hover:text-foreground">
                        <ChevronRight className="size-4" />
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  )
}
