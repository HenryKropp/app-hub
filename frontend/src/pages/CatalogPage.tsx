import { useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { Play, Search } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { EmptyState, KindTab, PageHeader } from "@/components/bits"
import { api, type AppSummary } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useRuns } from "@/lib/runs-context"

export function CatalogPage() {
  const { person } = useAuth()
  const { run, busy } = useRuns()
  const [apps, setApps] = useState<AppSummary[] | null>(null)
  const [error, setError] = useState("")
  const [q, setQ] = useState("")

  useEffect(() => {
    api.apps().then(setApps, (e: Error) => setError(e.message))
  }, [])

  const shown = useMemo(() => {
    const term = q.trim().toLowerCase()
    if (!apps || !term) return apps ?? []
    return apps.filter((a) => [a.name, a.summary, a.area, a.owner_name].some((s) => s.toLowerCase().includes(term)))
  }, [apps, q])

  const canRegister = person?.role === "admin" || person?.role === "criador"

  return (
    <>
      <PageHeader eyebrow="Catálogo" title="Apps da empresa">
        {apps && apps.length > 0 && (
          <div className="relative w-full md:w-72">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Buscar por nome, área, pessoa…" className="h-9 bg-card pl-8" />
          </div>
        )}
      </PageHeader>

      {error && <p className="text-destructive">{error}</p>}
      {apps === null && !error && <p className="text-muted-foreground">Carregando…</p>}

      {apps?.length === 0 && (
        <EmptyState
          title="O fichário ainda está vazio"
          action={
            canRegister && (
              <Link to="/registrar" className={buttonVariants({ size: "lg", className: "mt-2 h-10 px-4" })}>
                Registrar os primeiros apps
              </Link>
            )
          }
        >
          Aponte a pasta onde ficam os apps da empresa. O Hub analisa cada um e prepara a ficha para você revisar.
        </EmptyState>
      )}

      {apps && apps.length > 0 && shown.length === 0 && <p className="text-muted-foreground">Nenhum app encontrado para “{q}”.</p>}

      <ul className="grid gap-x-5 gap-y-8 sm:grid-cols-2 xl:grid-cols-3">
        {shown.map((a, i) => (
          <li key={a.id} className="rise" style={{ animationDelay: `${Math.min(i, 12) * 40}ms` }}>
            {/* The title link covers the card (after:inset-0); Rodar sits above it. */}
            <div className="group">
              <KindTab kind={a.kind} className="ml-4" />
              <div className="relative flex min-h-40 flex-col rounded-lg border border-border bg-card p-5 shadow-card transition-transform group-hover:-translate-y-0.5">
                <h2 className="font-heading text-xl leading-snug group-hover:text-stamp">
                  <Link to={`/apps/${a.id}`} className="after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-ring">
                    {a.name}
                  </Link>
                </h2>
                <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">{a.summary || "Sem resumo ainda."}</p>
                <div className="mt-auto flex items-center justify-between gap-3 border-t border-dashed border-border pt-3 text-xs text-muted-foreground">
                  <span className="min-w-0 truncate">
                    {a.area || "Sem área"} · <span className="font-medium text-foreground/80">{a.owner_name}</span>
                  </span>
                  {a.can_run && (
                    <Button
                      size="sm"
                      aria-label={`Rodar ${a.name}`}
                      disabled={busy.has(a.id)}
                      className="relative z-10 h-7 shrink-0 gap-1.5 px-2.5"
                      onClick={() => run({ id: a.id, name: a.name, folder_display: a.folder_display, remembered_folder: a.remembered_folder })}
                    >
                      <Play className="size-3.5" /> Rodar
                    </Button>
                  )}
                </div>
              </div>
            </div>
          </li>
        ))}
      </ul>
    </>
  )
}
