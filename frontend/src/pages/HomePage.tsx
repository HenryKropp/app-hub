import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { AppWindow, PanelsTopLeft, TriangleAlert, X } from "lucide-react"
import { FavoriteStar } from "@/components/FavoriteStar"
import { LoadError, RunState } from "@/components/bits"
import { Badge } from "@/components/ui/badge"
import { api, type Home, type HomeItem, type HomeWarning, type TeamHome } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useComputerStatus } from "@/lib/computer-status"
import { has } from "@/lib/permissions"
import { relativeTime } from "@/lib/runs"
import { cn } from "@/lib/utils"

function greeting(name: string, hour = new Date().getHours()) {
  const g = hour < 12 ? "Bom dia" : hour < 18 ? "Boa tarde" : "Boa noite"
  return `${g}, ${name.split(" ")[0]}`
}

function Section({ title, count, aside, action, children, className }: { title: string; count?: number; aside?: string; action?: React.ReactNode; children: React.ReactNode; className?: string }) {
  return (
    <section className={className}>
      <div className="mb-2.5 flex min-h-6 flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <div className="flex flex-wrap items-baseline gap-x-2">
          <h2 className="font-heading text-base">{title}</h2>
          {count !== undefined && <span className="text-sm text-muted-foreground tabular-nums">{count}</span>}
          {aside && <span className="text-sm text-muted-foreground">{aside}</span>}
        </div>
        {action}
      </div>
      {children}
    </section>
  )
}

/** A bordered list with hairlines between rows: the one container every Home block uses. */
const box = "divide-y divide-border overflow-hidden rounded-xl border border-border bg-card"
const moreLink = "text-sm text-muted-foreground hover:text-foreground hover:underline"

function today() {
  const d = new Date().toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long" })
  return d.charAt(0).toUpperCase() + d.slice(1)
}

/** The same computer warning can come more than once (two Agente versions, same name); and when the
 * top banner already says this computer is not connected, the Home does not repeat it. */
function cleanWarnings(list: HomeWarning[], bannerShown: boolean): HomeWarning[] {
  const seen = new Set<string>()
  return list.filter((w) => {
    if (w.kind === "computer_offline" && bannerShown) return false
    if (w.kind !== "computer_offline" && w.kind !== "agent_outdated") return true
    const key = `${w.kind}:${w.computer_name}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

function warningLine(w: HomeWarning, admin: boolean): { text: string; to: string } {
  const app = w.app_name ?? "O app"
  const runTo = `/execucoes/${w.run_id}`
  switch (w.kind) {
    case "run_error": return { text: `${app} deu erro`, to: runTo }
    case "run_waiting": return { text: `${app} está esperando seu computador`, to: runTo }
    case "run_interrupted": return { text: `${app} não rodou: seu computador não respondeu`, to: runTo }
    case "computer_offline": return { text: `Seu computador ${w.computer_name} está desconectado`, to: "/configuracoes" }
    case "agent_outdated": return { text: `O Agente do computador ${w.computer_name} está desatualizado`, to: "/configuracoes" }
    case "shared_crashed": return { text: `${app} caiu e não voltou sozinho`, to: `/apps/${w.app_id}` }
    case "app_errors": return { text: `${app} teve ${w.count} ${w.count === 1 ? "execução com erro" : "execuções com erro"} nesta semana`, to: `/apps/${w.app_id}` }
    case "approval_pending": return { text: `${app} tem versão nova esperando aprovação`, to: admin ? "/admin" : `/apps/${w.app_id}` }
  }
}

function Items({ items, onUnstar }: { items: HomeItem[]; onUnstar?: (it: HomeItem) => void }) {
  return (
    <ul className={box}>
      {items.map((it) => {
        const Icon = it.kind === "app" ? AppWindow : PanelsTopLeft
        return (
          <li key={`${it.kind}-${it.id}`} className="group relative flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-accent/60">
            <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <div className="min-w-0 flex-1">
              <Link
                to={`/${it.kind === "app" ? "apps" : "artifacts"}/${it.id}`}
                className="block truncate font-medium after:absolute after:inset-0 focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-inset focus-visible:after:ring-ring"
              >
                {it.name}
              </Link>
              {it.summary && <p className="truncate text-sm text-muted-foreground">{it.summary}</p>}
            </div>
            <span className="sr-only shrink-0 text-xs text-muted-foreground sm:not-sr-only">
              {it.kind === "app" ? "App" : "Artifact"}{it.area && ` · ${it.area}`}
            </span>
            <span className="relative z-10 -mr-1.5 shrink-0">
              <FavoriteStar kind={it.kind} id={it.id} on={onUnstar !== undefined} name={it.name} onChange={(on) => !on && onUnstar?.(it)} />
            </span>
          </li>
        )
      })}
    </ul>
  )
}

const CREATING_SHOWN = 5
const PENDING_SHOWN = 5

function Team({ t }: { t: TeamHome }) {
  const [allCreating, setAllCreating] = useState(false)
  const p = t.pending
  const pending = p
    ? [
        ...p.invitations.map((i) => ({ key: `i${i.id}`, text: `Convite para ${i.email} ainda não aceito`, expired: i.expired, at: i.created_at as string | undefined })),
        ...p.requests.map((r) => ({ key: `r${r.id}`, text: `${r.name} pediu acesso`, expired: false, at: r.created_at as string | undefined })),
        ...p.without_area.map((a) => ({ key: `a${a.id}`, text: `Sem área: ${a.name}`, expired: false, at: undefined })),
      ]
    : []
  const creating = allCreating ? t.creating : t.creating.slice(0, CREATING_SHOWN)
  return (
    <section className="mt-12 border-t border-border pt-8">
      <div className="mb-5 flex flex-wrap items-baseline gap-x-2">
        <h2 className="font-heading text-lg">Meu time</h2>
        <span className="text-sm text-muted-foreground">{t.scope === "company" ? "Empresa toda" : `Áreas: ${t.areas.join(", ")}`}</span>
      </div>
      <div className="grid grid-cols-1 gap-x-8 gap-y-8 lg:grid-cols-2">
        {(t.usage.length > 0 || t.never_used.length > 0) && (
          <Section title="Quem está usando" aside="últimos 7 dias">
            {t.usage.length > 0 && (
              <div className="overflow-x-auto rounded-xl border border-border bg-card">
                <table className="w-full text-sm">
                  <thead className="text-left text-xs text-muted-foreground">
                    <tr className="border-b border-border">
                      <th className="px-4 py-2 font-normal">Pessoa</th>
                      <th className="px-4 py-2 text-right font-normal">Execuções</th>
                      <th className="px-4 py-2 text-right font-normal">Artifacts abertos</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {t.usage.map((u) => (
                      <tr key={u.person_id}>
                        <td className="px-4 py-2.5 font-medium">{u.name}</td>
                        <td className="px-4 py-2.5 text-right tabular-nums">{u.runs_7d}</td>
                        <td className="px-4 py-2.5 text-right tabular-nums">{u.opens_7d}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {t.never_used.length > 0 && (
              <p className={t.usage.length > 0 ? "mt-3 text-sm text-muted-foreground" : "text-sm text-muted-foreground"}>
                <span className="text-foreground">Ainda não usaram:</span> {t.never_used.map((n) => n.name).join(", ")}
              </p>
            )}
          </Section>
        )}
        {t.creating.length > 0 && (
          <Section
            title="O que estão criando"
            action={t.creating.length > CREATING_SHOWN && (
              <button type="button" onClick={() => setAllCreating((v) => !v)} className={moreLink}>
                {allCreating ? "Mostrar menos" : `Ver tudo (${t.creating.length})`}
              </button>
            )}
          >
            <ul className={box}>
              {creating.map((c) => (
                <li key={`${c.kind}-${c.id}-${c.event}-${c.version}-${c.at}`} className="flex items-baseline justify-between gap-4 px-4 py-2.5 text-sm">
                  <span className="min-w-0 truncate">
                    <span className="text-muted-foreground">{c.person_name} {c.event === "created" ? "criou" : `subiu a versão ${c.version} de`}</span>{" "}
                    <Link to={`/${c.kind === "app" ? "apps" : "artifacts"}/${c.id}`} className="font-medium hover:underline">{c.name}</Link>
                  </span>
                  <span className="shrink-0 text-xs text-muted-foreground">{relativeTime(c.at)}</span>
                </li>
              ))}
            </ul>
          </Section>
        )}
        {pending.length > 0 && (
          <Section
            title="Pendências de gestão"
            count={pending.length}
            action={<Link to="/equipe" className={moreLink}>Abrir Equipe</Link>}
          >
            <ul className={box}>
              {pending.slice(0, PENDING_SHOWN).map((x) => (
                <li key={x.key} className="flex items-center gap-2 px-4 py-2.5 text-sm">
                  <Link to="/equipe" className="min-w-0 truncate hover:underline">{x.text}</Link>
                  {x.expired && <Badge variant="destructive">venceu</Badge>}
                  {x.at && <span className="ml-auto shrink-0 text-xs text-muted-foreground">{relativeTime(x.at)}</span>}
                </li>
              ))}
              {pending.length > PENDING_SHOWN && (
                <li className="px-4 py-2.5 text-sm">
                  <Link to="/equipe" className={moreLink}>e mais {pending.length - PENDING_SHOWN} na Equipe</Link>
                </li>
              )}
            </ul>
          </Section>
        )}
      </div>
    </section>
  )
}

export function HomePage() {
  const { person } = useAuth()
  const { connected } = useComputerStatus()
  const [home, setHome] = useState<Home | null>(null)
  const [team, setTeam] = useState<TeamHome | null>(null)
  const [error, setError] = useState("")
  const [tries, setTries] = useState(0)
  const admin = has(person, "manage_roles")
  const hasTeam = admin || (person?.leads.length ?? 0) > 0

  useEffect(() => {
    let gone = false
    api.home().then(
      (h) => !gone && setHome(h),
      (e: Error) => !gone && setError(e.message),
    )
    return () => { gone = true }
  }, [tries])

  useEffect(() => {
    if (!hasTeam) return
    let gone = false
    api.homeTeam().then(
      (t) => !gone && setTeam(t),
      () => {}, // 404: the section just stays hidden
    )
    return () => { gone = true }
  }, [hasTeam, tries])

  const dismiss = (w: HomeWarning) => {
    if (!w.run_id) return
    api.dismissRun(w.run_id).then(
      () => setHome((h) => h && { ...h, attention: h.attention.filter((x) => x !== w) }),
      (e: Error) => setError(e.message),
    )
  }
  const unstar = (it: HomeItem) => setHome((h) => h && { ...h, favorites: h.favorites.filter((f) => !(f.kind === it.kind && f.id === it.id)) })
  const warnings = home ? cleanWarnings(home.attention, connected === false) : []

  return (
    <>
      <header>
        <h1 className="font-heading text-2xl">{greeting(person?.name ?? "")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{today()}</p>
      </header>
      {error && <div className="mt-6"><LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} /></div>}
      {home === null && !error && <p className="mt-6 text-sm text-muted-foreground">Carregando…</p>}

      {home && (
        <>
          {warnings.length > 0 && (
            <Section title="Atenção" count={warnings.length} className="mt-8">
              <ul className={box}>
                {warnings.map((w, i) => {
                  const line = warningLine(w, admin)
                  const canDismiss = !!w.run_id && (w.kind === "run_error" || w.kind === "run_waiting" || w.kind === "run_interrupted")
                  return (
                    <li key={`${w.kind}-${w.run_id ?? w.app_id ?? w.computer_name}-${i}`} className="flex items-center gap-3 px-4 py-2.5 text-sm">
                      <TriangleAlert className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                      <Link to={line.to} className="min-w-0 flex-1 truncate font-medium hover:underline">{line.text}</Link>
                      {w.at && <span className="shrink-0 text-xs text-muted-foreground">{relativeTime(w.at)}</span>}
                      {canDismiss && (
                        <button type="button" onClick={() => dismiss(w)} aria-label={`Dispensar: ${line.text}`} className="-my-1 inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground">
                          <X className="size-3.5" /> Dispensar
                        </button>
                      )}
                    </li>
                  )
                })}
              </ul>
            </Section>
          )}

          <div className={cn("mt-8 grid grid-cols-1 gap-x-8 gap-y-8", home.runs.length > 0 && "lg:grid-cols-[minmax(0,1fr)_minmax(0,22rem)]")}>
            <div className="space-y-8">
              <Section title="Favoritos">
                {home.favorites.length === 0 ? (
                  <p className="rounded-xl border border-dashed border-border px-4 py-5 text-sm text-muted-foreground">
                    Marque apps e artifacts com a estrela ☆ para eles aparecerem aqui.
                  </p>
                ) : (
                  <Items items={home.favorites} onUnstar={unstar} />
                )}
              </Section>
              {home.recent.length > 0 && (
                <Section title="Usados recentemente">
                  <Items items={home.recent} />
                </Section>
              )}
            </div>

            {home.runs.length > 0 && (
              <Section title="Minhas últimas execuções" action={<Link to="/execucoes" className={moreLink}>Ver todas</Link>}>
                <ul className={box}>
                  {home.runs.map((r) => (
                    <li key={r.id} className="px-4 py-2.5">
                      <Link to={`/execucoes/${r.id}`} className="block truncate text-sm font-medium hover:underline">{r.app_name}</Link>
                      <span className="mt-0.5 flex items-center justify-between gap-3 text-xs">
                        <RunState status={r.status} className="[&_svg]:size-3.5" />
                        <span className="text-muted-foreground">{relativeTime(r.created_at)}</span>
                      </span>
                    </li>
                  ))}
                </ul>
              </Section>
            )}
          </div>
        </>
      )}

      {team && <Team t={team} />}
    </>
  )
}
