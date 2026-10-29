import { useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { ArrowRight, ExternalLink, Lock, Play, Plus, Search, X } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { SharedStatusDot } from "@/components/SharedStatus"
import { Input } from "@/components/ui/input"
import { EmptyState, KindTab, LoadError, PageHeader, Playing, SavedIn } from "@/components/bits"
import { api, openSharedUrl, type AppSummary, type Onboarding } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { canRegister } from "@/lib/permissions"
import { useRuns } from "@/lib/runs-context"
import { isLive } from "@/lib/runs"
import { cn } from "@/lib/utils"

export function CatalogPage() {
  const { person } = useAuth()
  const { run, busy, active, rememberedFor } = useRuns()
  const playing = useMemo(() => new Set(active.filter((r) => isLive(r.status)).map((r) => r.app_id)), [active])
  const [apps, setApps] = useState<AppSummary[] | null>(null)
  const [error, setError] = useState("")
  const [q, setQ] = useState("")
  const [tries, setTries] = useState(0)
  const [onboarding, setOnboarding] = useState<Onboarding | null>(null)

  useEffect(() => {
    api.apps().then(setApps, (e: Error) => setError(e.message))
    api.onboarding().then(setOnboarding, () => {})
  }, [tries])
  const retry = () => {
    setError("")
    setTries((n) => n + 1)
  }

  const shown = useMemo(() => {
    const term = q.trim().toLowerCase()
    if (!apps || !term) return apps ?? []
    return apps.filter((a) => [a.name, a.summary, a.area, a.owner_name].some((s) => s.toLowerCase().includes(term)))
  }, [apps, q])

  const canReg = canRegister(person)
  const setupDone = onboarding ? Boolean(onboarding.company_name.trim() && onboarding.computer_connected && onboarding.has_apps && onboarding.has_successful_run && onboarding.has_team) : true

  return (
    <>
      <PageHeader title="Apps da empresa">
        {apps && apps.length > 0 && (
          <div className="flex w-full items-center gap-2 md:w-auto">
            <div className="relative flex-1 md:w-72 md:flex-none">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                type="search"
                aria-label="Buscar apps"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                onKeyDown={(e) => e.key === "Escape" && setQ("")}
                placeholder="Buscar por nome, área, pessoa…"
                className="h-9 rounded-lg bg-card pr-8 pl-8 [&::-webkit-search-cancel-button]:hidden"
              />
              {q && (
                <button type="button" aria-label="Limpar busca" title="Limpar busca" onClick={() => setQ("")} className="absolute top-1/2 right-1.5 -translate-y-1/2 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground">
                  <X className="size-3.5" />
                </button>
              )}
            </div>
            {canReg && (
              <Link
                to="/registrar"
                aria-label="Registrar apps"
                title="Registrar apps"
                className={cn(buttonVariants({ variant: "run" }), "size-9 gap-1.5 rounded-lg sm:w-auto sm:px-3 [&_svg]:fill-none")}
              >
                <Plus className="size-4" />
                <span className="hidden sm:inline">Registrar apps</span>
              </Link>
            )}
          </div>
        )}
      </PageHeader>

      {onboarding?.can_manage && !setupDone && (
        <Link to="/comecar" className="mb-6 flex items-center justify-between gap-4 rounded-xl border border-border bg-muted/40 p-4 transition-colors hover:bg-muted sm:px-5">
          <span className="min-w-0"><span className="block text-sm font-medium">Faltam alguns passos para preparar o Kova para sua equipe.</span></span>
          <span className="flex shrink-0 items-center gap-2 text-sm font-medium">Continuar <ArrowRight className="size-4" /></span>
        </Link>
      )}

      {error && <LoadError message={error} onRetry={retry} />}
      {apps === null && !error && <p className="text-muted-foreground">Carregando…</p>}

      {apps?.length === 0 && (
        <EmptyState
          title="O catálogo ainda está vazio"
          action={
            canReg && (
              <Link to="/registrar" className={buttonVariants({ size: "lg", className: "mt-2 h-10 px-4" })}>
                Registrar os primeiros apps
              </Link>
            )
          }
        >
          {canReg
            ? "Aponte a pasta onde ficam os apps da empresa. O Kova analisa cada um e prepara a ficha para você revisar."
            : "Quando alguém registrar os apps da empresa, eles aparecem aqui para você rodar."}
        </EmptyState>
      )}

      {apps && apps.length > 0 && shown.length === 0 && (
        <p className="text-muted-foreground">
          Nenhum app encontrado para “{q}”.{" "}
          <button type="button" onClick={() => setQ("")} className="font-medium text-foreground underline underline-offset-4">
            Limpar busca
          </button>
        </p>
      )}

      <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {shown.map((a) => {
          const live = playing.has(a.id)
          const shared = a.shared && a.shared.status !== "draft" ? a.shared : null
          const target = { id: a.id, name: a.name, folder_display: a.folder_display, remembered_folder: a.remembered_folder, kind: a.kind }
          const saved = a.can_run && !shared ? rememberedFor(target) : ""
          return (
            <li key={a.id}>
              {/* The title link covers the card (after:inset-0); Rodar sits above it. */}
              <div className="group relative flex h-full min-h-44 flex-col rounded-xl border border-border bg-card p-5 transition-colors hover:border-input hover:bg-[color-mix(in_oklch,var(--card),var(--foreground)_1.5%)]">
                <h2 className="font-heading text-[1.05rem] leading-snug">
                  <Link to={`/apps/${a.id}`} className="after:absolute after:inset-0 after:rounded-xl focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-ring">
                    {a.name}
                  </Link>
                  {a.restricted && <Lock aria-label="Restrito" className="ml-1.5 inline size-3.5 align-[-1px] text-muted-foreground" />}
                </h2>
                <p className="mt-1.5 line-clamp-2 text-sm leading-relaxed text-muted-foreground">{a.summary || "Sem resumo ainda."}</p>
                <div className="mt-auto flex items-end justify-between gap-3 pt-5 text-xs text-muted-foreground">
                  <span className="flex min-w-0 flex-col items-start gap-2">
                    <KindTab kind={a.kind} />
                    <span className="max-w-full truncate">
                      {a.area || "Sem área"} · <span className="text-foreground/80">{a.owner_name}</span>
                    </span>
                  </span>
                  <span className="flex shrink-0 flex-col items-end gap-2">
                    {shared && <SharedStatusDot status={shared.status} />}
                    {!shared && live && (
                      <span className="inline-flex items-center gap-1.5 font-medium text-foreground">
                        <Playing /> aberto agora
                      </span>
                    )}
                    {shared && (
                      <a
                        href={openSharedUrl(shared.slug)}
                        target="_blank"
                        rel="noopener"
                        aria-label={`Abrir ${a.name}`}
                        className={cn(buttonVariants({ variant: "run" }), "relative z-10 h-8 shrink-0 gap-1.5 pr-3.5 pl-3 [&_svg]:fill-none")}
                      >
                        <ExternalLink className="size-3.5" /> Abrir
                      </a>
                    )}
                    {!shared && a.can_run && (
                      <Button
                        variant="run"
                        aria-label={`Rodar ${a.name}`}
                        disabled={busy.has(a.id)}
                        className="relative z-10 h-8 shrink-0 gap-1.5 pr-3.5 pl-3"
                        onClick={() => run(target)}
                      >
                        <Play className="size-3.5" /> Rodar
                      </Button>
                    )}
                  </span>
                </div>
                {saved && <SavedIn folder={saved} onChange={() => run(target, { ask: true })} className="mt-3 border-t border-border pt-3 text-xs" />}
              </div>
            </li>
          )
        })}
      </ul>
    </>
  )
}
