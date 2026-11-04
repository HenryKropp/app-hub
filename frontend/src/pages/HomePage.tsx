import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { X } from "lucide-react"
import { FavoriteStar } from "@/components/FavoriteStar"
import { LoadError, RunState } from "@/components/bits"
import { Badge } from "@/components/ui/badge"
import { api, type Home, type HomeItem, type HomeWarning, type TeamHome } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { has } from "@/lib/permissions"
import { relativeTime } from "@/lib/runs"

function greeting(name: string, hour = new Date().getHours()) {
  const g = hour < 12 ? "Bom dia" : hour < 18 ? "Boa tarde" : "Boa noite"
  return `${g}, ${name.split(" ")[0]}`
}

function Section({ title, subtitle, action, children }: { title: string; subtitle?: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="mt-8">
      <div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <div className="flex flex-wrap items-baseline gap-x-3">
          <h2 className="font-heading text-xl">{title}</h2>
          {subtitle && <span className="text-sm text-muted-foreground">{subtitle}</span>}
        </div>
        {action}
      </div>
      {children}
    </section>
  )
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

function Cards({ items, onUnstar }: { items: HomeItem[]; onUnstar?: (it: HomeItem) => void }) {
  return (
    <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      {items.map((it) => (
        <li key={`${it.kind}-${it.id}`}>
          <div className="group relative flex h-full min-h-28 flex-col rounded-xl border border-border bg-card p-5 transition-colors hover:border-input hover:bg-[color-mix(in_oklch,var(--card),var(--foreground)_1.5%)]">
            <h3 className="flex items-start gap-2 font-heading text-[1.05rem] leading-snug">
              <Link to={`/${it.kind === "app" ? "apps" : "artifacts"}/${it.id}`} className="min-w-0 break-words after:absolute after:inset-0 after:rounded-xl focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-ring">
                {it.name}
              </Link>
              <span className="ml-auto -mt-0.5 -mr-1.5">
                <FavoriteStar kind={it.kind} id={it.id} on={onUnstar !== undefined} name={it.name} onChange={(on) => !on && onUnstar?.(it)} />
              </span>
            </h3>
            {it.summary && <p className="mt-1.5 line-clamp-2 text-sm text-muted-foreground">{it.summary}</p>}
            <p className="mt-auto flex flex-wrap items-center gap-2 pt-3 text-xs text-muted-foreground">
              <Badge variant="secondary">{it.kind === "app" ? "App" : "Artifact"}</Badge>
              {it.area && <span>{it.area}</span>}
            </p>
          </div>
        </li>
      ))}
    </ul>
  )
}

function Team({ t }: { t: TeamHome }) {
  const p = t.pending
  const showPending = !!p && (p.invitations.length > 0 || p.requests.length > 0 || p.without_area.length > 0)
  return (
    <Section title="Meu time" subtitle={t.scope === "company" ? "Empresa toda" : `Áreas: ${t.areas.join(", ")}`}>
      <div className="space-y-6">
        {t.usage.length > 0 && (
          <div>
            <h3 className="mb-2 font-medium">Quem está usando</h3>
            <div className="overflow-x-auto rounded-xl border border-border bg-card">
              <table className="w-full text-sm">
                <thead className="text-left text-xs text-muted-foreground">
                  <tr>
                    <th className="px-4 py-2 font-normal">Pessoa</th>
                    <th className="px-4 py-2 text-right font-normal">Execuções (7 dias)</th>
                    <th className="px-4 py-2 text-right font-normal">Artifacts abertos (7 dias)</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {t.usage.map((u) => (
                    <tr key={u.person_id}>
                      <td className="px-4 py-2.5 font-medium">{u.name}</td>
                      <td className="px-4 py-2.5 text-right">{u.runs_7d}</td>
                      <td className="px-4 py-2.5 text-right">{u.opens_7d}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
        {t.never_used.length > 0 && (
          <div>
            <h3 className="mb-1 font-medium">Ainda não usaram</h3>
            <p className="text-sm text-muted-foreground">{t.never_used.map((n) => n.name).join(", ")}</p>
          </div>
        )}
        {t.creating.length > 0 && (
          <div>
            <h3 className="mb-2 font-medium">O que estão criando</h3>
            <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
              {t.creating.map((c) => (
                <li key={`${c.kind}-${c.id}-${c.event}-${c.version}-${c.at}`} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 px-4 py-2.5 text-sm">
                  <span className="min-w-0">
                    {c.person_name} {c.event === "created" ? "criou" : `subiu a versão ${c.version} de`}{" "}
                    <Link to={`/${c.kind === "app" ? "apps" : "artifacts"}/${c.id}`} className="font-medium hover:underline">{c.name}</Link>
                  </span>
                  <span className="text-xs text-muted-foreground">{relativeTime(c.at)}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
        {showPending && p && (
          <div>
            <h3 className="mb-2 font-medium">Pendências de gestão</h3>
            <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card text-sm">
              {p.invitations.map((i) => (
                <li key={`i${i.id}`} className="flex flex-wrap items-center gap-2 px-4 py-2.5">
                  <Link to="/equipe" className="hover:underline">Convite para {i.email} ainda não aceito</Link>
                  {i.expired && <Badge variant="destructive">venceu</Badge>}
                  <span className="ml-auto text-xs text-muted-foreground">{relativeTime(i.created_at)}</span>
                </li>
              ))}
              {p.requests.map((r) => (
                <li key={`r${r.id}`} className="flex flex-wrap items-center gap-2 px-4 py-2.5">
                  <Link to="/equipe" className="hover:underline">{r.name} pediu acesso</Link>
                  <span className="ml-auto text-xs text-muted-foreground">{relativeTime(r.created_at)}</span>
                </li>
              ))}
              {p.without_area.map((a) => (
                <li key={`a${a.id}`} className="px-4 py-2.5">
                  <Link to="/equipe" className="hover:underline">Sem área: {a.name}</Link>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Section>
  )
}

export function HomePage() {
  const { person } = useAuth()
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

  return (
    <>
      <h1 className="font-heading text-3xl">{greeting(person?.name ?? "")}</h1>
      {error && <div className="mt-4"><LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} /></div>}
      {home === null && !error && <p className="mt-4 text-muted-foreground">Carregando…</p>}

      {home && (
        <>
          {home.attention.length > 0 && (
            <Section title="Atenção">
              <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
                {home.attention.map((w, i) => {
                  const line = warningLine(w, admin)
                  return (
                    <li key={`${w.kind}-${w.run_id ?? w.app_id ?? w.computer_name}-${i}`} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
                      <Link to={line.to} className="min-w-0 font-medium hover:underline">{line.text}</Link>
                      <span className="text-xs text-muted-foreground">{w.at && <>{relativeTime(w.at)}</>}</span>
                      {w.run_id && (w.kind === "run_error" || w.kind === "run_waiting" || w.kind === "run_interrupted") && (
                        <button type="button" onClick={() => dismiss(w)} className="ml-auto inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-accent hover:text-foreground">
                          <X className="size-3.5" /> Dispensar
                        </button>
                      )}
                    </li>
                  )
                })}
              </ul>
            </Section>
          )}

          <Section title="Favoritos">
            {home.favorites.length === 0 ? (
              <p className="text-muted-foreground">Marque apps e artifacts com a estrela ☆ para eles aparecerem aqui.</p>
            ) : (
              <Cards items={home.favorites} onUnstar={unstar} />
            )}
          </Section>

          {home.recent.length > 0 && (
            <Section title="Usados recentemente">
              <Cards items={home.recent} />
            </Section>
          )}

          {home.runs.length > 0 && (
            <Section title="Minhas últimas execuções" action={<Link to="/execucoes" className="text-sm text-muted-foreground hover:text-foreground hover:underline">Ver todas</Link>}>
              <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
                {home.runs.map((r) => (
                  <li key={r.id} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 py-2.5 text-sm">
                    <Link to={`/execucoes/${r.id}`} className="min-w-0 font-medium hover:underline">{r.app_name}</Link>
                    <span className="flex items-center gap-3">
                      <RunState status={r.status} />
                      <span className="text-xs text-muted-foreground">{relativeTime(r.created_at)}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </Section>
          )}
        </>
      )}

      {team && <Team t={team} />}
    </>
  )
}
