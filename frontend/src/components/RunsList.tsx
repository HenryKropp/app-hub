import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { api, type RunSummary } from "@/lib/api"
import { formatDuration, formatWhen, STATUS } from "@/lib/runs"

export function RunsList({ appId }: { appId: number }) {
  const [runs, setRuns] = useState<RunSummary[] | null>(null)
  useEffect(() => {
    api.appRuns(appId).then(setRuns, () => setRuns([]))
  }, [appId])
  if (!runs) return null
  if (runs.length === 0) return <p className="text-sm text-muted-foreground">Nenhuma execução ainda.</p>
  return (
    <ul className="flex flex-col divide-y divide-border text-sm">
      {runs.map((r) => (
        <li key={r.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 py-2">
          <Link to={`/execucoes/${r.id}`} className="font-medium underline-offset-4 hover:underline">{STATUS[r.status].label}</Link>
          <span className="text-muted-foreground">{formatWhen(r.created_at)} · {r.person_name} · {formatDuration(r.started_at, r.finished_at)}</span>
          <span className="ml-auto font-mono text-xs text-muted-foreground">{r.file_count} arquivos</span>
        </li>
      ))}
    </ul>
  )
}
