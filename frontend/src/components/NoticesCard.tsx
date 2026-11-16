import { useEffect, useState } from "react"
import { CircleCheck } from "lucide-react"
import { Link } from "react-router-dom"
import { api, type Notice } from "@/lib/api"
import { relativeTime } from "@/lib/runs"

/** Home card with the "versão N no ar — testada" notices not read yet (ADR 044). Hidden when there are none. */
export function NoticesCard() {
  const [notices, setNotices] = useState<Notice[]>([])
  useEffect(() => {
    let alive = true
    api.notices().then((list) => alive && setNotices(list.filter((n) => !n.read_at)), () => {})
    return () => { alive = false }
  }, [])
  if (notices.length === 0) return null
  const read = (n: Notice) =>
    api.readNotice(n.id).then(() => setNotices((list) => list.filter((x) => x.id !== n.id)), () => {})
  return (
    <section className="mt-8">
      <div className="mb-2.5 flex items-baseline gap-x-2">
        <h2 className="font-heading text-base">Avisos</h2>
        <span className="text-sm text-muted-foreground tabular-nums">{notices.length}</span>
      </div>
      <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
        {notices.map((n) => (
          <li key={n.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
            <CircleCheck className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <Link to={`/apps/${n.app_id}`} className="min-w-0 flex-1 truncate font-medium hover:underline">{n.text}</Link>
            <span className="shrink-0 text-xs text-muted-foreground">{relativeTime(n.created_at)}</span>
            <button
              type="button"
              onClick={() => read(n)}
              aria-label={`Marcar como lido: ${n.text}`}
              className="-my-1 shrink-0 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              Marcar como lido
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}
