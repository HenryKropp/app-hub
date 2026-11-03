import { useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { FileCode2, Link2, Lock, Plus, Search, X } from "lucide-react"
import { buttonVariants } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { EmptyState, LoadError, NativeSelect, PageHeader } from "@/components/bits"
import { api, type ArtifactSummary } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { canRegister } from "@/lib/permissions"
import { cn } from "@/lib/utils"

function relativeDate(iso: string): string {
  const days = Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000)
  if (days <= 0) return "hoje"
  if (days === 1) return "ontem"
  if (days < 30) return `há ${days} dias`
  const months = Math.floor(days / 30)
  if (months < 12) return months === 1 ? "há 1 mês" : `há ${months} meses`
  const years = Math.floor(months / 12)
  return years === 1 ? "há 1 ano" : `há ${years} anos`
}

export function ArtifactsPage() {
  const { person } = useAuth()
  const [items, setItems] = useState<ArtifactSummary[] | null>(null)
  const [error, setError] = useState("")
  const [tries, setTries] = useState(0)
  const [q, setQ] = useState("")
  const [tag, setTag] = useState("")
  const [areaId, setAreaId] = useState("")

  useEffect(() => {
    api.artifacts().then(setItems, (e: Error) => setError(e.message))
  }, [tries])
  const retry = () => {
    setError("")
    setTries((n) => n + 1)
  }

  const areas = useMemo(() => {
    const m = new Map<number, string>()
    items?.forEach((a) => m.set(a.area_id, a.area))
    return [...m].sort((a, b) => a[1].localeCompare(b[1]))
  }, [items])

  const shown = useMemo(() => {
    const term = q.trim().toLowerCase()
    return (items ?? []).filter((a) => {
      if (tag && !a.tags.includes(tag)) return false
      if (areaId && String(a.area_id) !== areaId) return false
      return !term || [a.name, a.summary, a.area, ...a.tags].some((s) => s.toLowerCase().includes(term))
    })
  }, [items, q, tag, areaId])

  const canReg = canRegister(person)
  const newButton = (className?: string) => (
    <Link to="/artifacts/novo" className={cn(buttonVariants({ variant: "run" }), "h-9 gap-1.5 rounded-lg px-3 [&_svg]:fill-none", className)}>
      <Plus className="size-4" />
      Novo artifact
    </Link>
  )

  return (
    <>
      <PageHeader title="Artifacts">
        {items && items.length > 0 && (
          <div className="flex w-full flex-wrap items-center gap-2 md:w-auto">
            <div className="relative w-full md:w-64 md:flex-none">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                type="search"
                aria-label="Buscar artifacts"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                onKeyDown={(e) => e.key === "Escape" && setQ("")}
                placeholder="Buscar artifacts…"
                className="h-9 rounded-lg bg-card pr-8 pl-8 [&::-webkit-search-cancel-button]:hidden"
              />
              {q && (
                <button type="button" aria-label="Limpar busca" title="Limpar busca" onClick={() => setQ("")} className="absolute top-1/2 right-1.5 -translate-y-1/2 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground">
                  <X className="size-3.5" />
                </button>
              )}
            </div>
            <NativeSelect aria-label="Área" value={areaId} onChange={(e) => setAreaId(e.target.value)} className="w-auto max-w-44">
              <option value="">Todas as áreas</option>
              {areas.map(([id, name]) => (
                <option key={id} value={id}>
                  {name}
                </option>
              ))}
            </NativeSelect>
            {tag && (
              <button
                type="button"
                aria-label={`Remover filtro ${tag}`}
                onClick={() => setTag("")}
                className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-border bg-accent px-2.5 text-sm font-medium hover:bg-accent/70"
              >
                <X className="size-3.5" />
                {tag}
              </button>
            )}
            {canReg && newButton()}
          </div>
        )}
      </PageHeader>

      {error && <LoadError message={error} onRetry={retry} />}
      {items === null && !error && <p className="text-muted-foreground">Carregando…</p>}

      {items?.length === 0 && (
        <EmptyState title="Nenhum artifact ainda" action={canReg && newButton("mt-2 h-10 px-4")}>
          Guarde aqui os dashboards e páginas que o time cria no Claude.
        </EmptyState>
      )}

      {items && items.length > 0 && shown.length === 0 && <p className="text-muted-foreground">Nada encontrado para essa busca.</p>}

      <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {shown.map((a) => {
          const Icon = a.kind === "link" ? Link2 : FileCode2
          return (
            <li key={a.id}>
              <div className="group relative flex h-full min-h-40 flex-col rounded-xl border border-border bg-card p-5 transition-colors hover:border-input hover:bg-[color-mix(in_oklch,var(--card),var(--foreground)_1.5%)]">
                <h2 className="flex items-start gap-2 font-heading text-[1.05rem] leading-snug">
                  <Icon aria-hidden className="mt-0.5 size-4 shrink-0 text-brand" />
                  <Link to={`/artifacts/${a.id}`} className="min-w-0 after:absolute after:inset-0 after:rounded-xl focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-ring">
                    {a.name}
                  </Link>
                  {a.restricted && <Lock aria-label="Restrito" className="mt-1 size-3.5 shrink-0 text-muted-foreground" />}
                </h2>
                <p className="mt-1.5 line-clamp-1 text-sm text-muted-foreground">{a.summary || "Sem resumo ainda."}</p>
                {a.tags.length > 0 && (
                  <div className="relative z-10 mt-3 flex flex-wrap gap-1.5">
                    {a.tags.map((t) => (
                      <button
                        key={t}
                        type="button"
                        onClick={() => setTag(t)}
                        className={cn("rounded-full border px-2 py-0.5 text-xs transition-colors", t === tag ? "border-transparent bg-accent text-foreground" : "border-border text-muted-foreground hover:bg-accent hover:text-foreground")}
                      >
                        {t}
                      </button>
                    ))}
                  </div>
                )}
                <p className="mt-auto flex items-center justify-between gap-3 pt-4 text-xs text-muted-foreground">
                  <span className="truncate">{a.area || "Sem área"}</span>
                  <span className="shrink-0">
                    versão {a.version} · {relativeDate(a.updated_at)}
                  </span>
                </p>
              </div>
            </li>
          )
        })}
      </ul>
    </>
  )
}
