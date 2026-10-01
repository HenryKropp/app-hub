import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { Play } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { EmptyState, PageHeader } from "@/components/bits"
import { api, type RunListItem } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useRuns } from "@/lib/runs-context"
import { formatDuration, formatWhen, isLive, STATUS } from "@/lib/runs"
import { cn } from "@/lib/utils"

export function MyRunsPage() {
  const { person } = useAuth()
  const { run, active, busy } = useRuns()
  const [scope, setScope] = useState<"mine" | "all">("mine")
  const [items, setItems] = useState<RunListItem[] | null>(null)
  const [more, setMore] = useState(false)
  const [error, setError] = useState("")

  // Reload when the filter changes and when a run in the strip changes situation.
  const signal = active.map((r) => `${r.id}:${r.status}`).join(",")
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
  }, [scope, signal])

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
      <PageHeader eyebrow="Minhas execuções" title={scope === "all" ? "O que todos rodaram" : "O que você rodou"}>
        {person?.role === "admin" && (
          <div className="flex gap-1 rounded-md border border-border bg-card p-1 text-sm">
            {(["mine", "all"] as const).map((s) => (
              <button
                key={s}
                type="button"
                aria-pressed={scope === s}
                onClick={() => setScope(s)}
                className={cn("rounded px-3 py-1", scope === s ? "bg-ink text-ink-foreground" : "text-muted-foreground hover:text-foreground")}
              >
                {s === "mine" ? "Minhas" : "De todos"}
              </button>
            ))}
          </div>
        )}
      </PageHeader>
      {error && <p className="text-destructive">{error}</p>}
      {items === null && !error && <p className="text-muted-foreground">Carregando…</p>}
      {items?.length === 0 && (
        <EmptyState title={scope === "all" ? "Ninguém rodou nenhum app ainda" : "Você ainda não rodou nenhum app"}>
          No catálogo, clique em Rodar no cartão do app. O resultado aparece aqui.
        </EmptyState>
      )}
      {items && items.length > 0 && (
        <ul className="rise divide-y divide-border rounded-lg border border-border bg-card shadow-card">
          {items.map((r) => {
            const st = STATUS[r.status]
            return (
              <li key={r.id} className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 px-5 py-4">
                <div className="min-w-0">
                  <Link to={`/apps/${r.app_id}`} className="font-heading text-lg leading-snug hover:text-stamp">
                    {r.app_name}
                  </Link>
                  <p className="mt-0.5 text-sm text-muted-foreground">
                    {formatWhen(r.created_at)} · {formatDuration(r.started_at, r.finished_at)} ·{" "}
                    <span className={cn("font-medium", st.tone === "bad" && "text-destructive", st.tone === "ok" && "text-ok", st.tone === "live" && "text-stamp")}>
                      {st.label}
                    </span>
                    {!isLive(r.status) && ` · ${r.file_count} ${r.file_count === 1 ? "arquivo" : "arquivos"}`}
                    {scope === "all" && ` · ${r.person_name}`}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Link to={`/execucoes/${r.id}`} className={buttonVariants({ variant: "outline", size: "sm" })}>
                    Ver e baixar
                  </Link>
                  <Button
                    size="sm"
                    className="gap-1.5"
                    disabled={busy.has(r.app_id)}
                    onClick={() => run({ id: r.app_id, name: r.app_name, folder_display: r.output_folder, remembered_folder: "" }, { folder: r.output_folder })}
                  >
                    <Play className="size-3.5" /> Rodar de novo
                  </Button>
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
