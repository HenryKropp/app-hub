import { useCallback, useEffect, useState, type ReactNode } from "react"
import { Link } from "react-router-dom"
import { toast } from "sonner"
import { CircleCheck, CirclePause, Clock, Monitor, TriangleAlert } from "lucide-react"
import { LoadError, PageHeader } from "@/components/bits"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api, ApiError, type AdminApp, type AdminEvent, type AdminOverview, type CompanyShared, type SecuritySettings } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { cn } from "@/lib/utils"

const pending = (a: AdminApp) => !a.suspended && !a.approved

export function AdminPage() {
  const [data, setData] = useState<AdminOverview | null>(null)
  const [error, setError] = useState("")
  const load = useCallback(async () => {
    setError("")
    try {
      setData(await api.adminOverview())
    } catch (e) {
      setError(e instanceof ApiError && e.status === 403 ? "Só administradores veem esta página." : e instanceof Error ? e.message : "Não foi possível carregar o Admin.")
    }
  }, [])
  useEffect(() => { void load() }, [load])

  if (error) return <><PageHeader title="Admin" /><LoadError message={error} onRetry={() => void load()} /></>
  if (!data) return <div role="status" aria-label="Carregando Admin" className="space-y-5"><div className="h-8 w-40 animate-pulse rounded bg-muted" /><div className="h-24 animate-pulse rounded-xl bg-muted" /><div className="h-64 animate-pulse rounded-xl bg-muted" /></div>

  const { apps, computers, settings } = data
  const waiting = settings.require_app_approval ? apps.filter(pending) : []
  const runs = apps.reduce((n, a) => n + a.runs_30d, 0)
  const errors = apps.reduce((n, a) => n + a.errors_30d, 0)
  const areas = [...new Set(apps.map((a) => a.area || "Sem área"))]

  return (
    <>
      <PageHeader title="Admin" />
      {!settings.local_execution_enabled && (
        <p role="alert" className="mb-6 flex items-start gap-2.5 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm">
          <CirclePause className="mt-0.5 size-4 shrink-0 text-destructive" />
          <span><span className="font-medium">Todos os apps estão pausados.</span> <span className="text-muted-foreground">Ninguém da empresa consegue rodar apps até você retomar, no fim desta página.</span></span>
        </p>
      )}

      <dl className="mb-10 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Apps no catálogo" value={apps.length} />
        <Stat label="Esperando aprovação" value={waiting.length} tone={waiting.length > 0 ? "warn" : undefined} />
        <Stat label="Execuções em 30 dias" value={runs} detail={errors > 0 ? `${errors} com erro` : undefined} />
        <Stat label="Computadores conectados" value={`${computers.filter((c) => c.connected).length} de ${computers.length}`} />
      </dl>

      {waiting.length > 0 && (
        <section aria-labelledby="precisam" className="mb-10 rounded-xl border border-warn/35 bg-warn-soft p-5">
          <h2 id="precisam" className="font-heading text-lg">Esperando sua aprovação</h2>
          <p className="mt-1 text-sm text-muted-foreground">Ninguém consegue rodar estes apps até você aprovar. Abra o app para revisar.</p>
          <ul className="mt-3 flex flex-wrap gap-2">
            {waiting.map((a) => (
              <li key={a.id}><Link to={`/apps/${a.id}`} className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-accent"><Clock className="size-3.5 text-warn" />{a.name}</Link></li>
            ))}
          </ul>
        </section>
      )}

      <section aria-labelledby="apps-area" className="mb-10">
        <h2 id="apps-area" className="font-heading text-xl">Apps por área</h2>
        <p className="mt-1 mb-4 text-sm text-muted-foreground">Uso nos últimos 30 dias. Clique num app para aprovar, suspender ou mudar quem tem acesso.</p>
        {apps.length === 0 ? (
          <p className="rounded-xl border border-dashed border-input p-6 text-sm text-muted-foreground">Nenhum app registrado ainda.</p>
        ) : (
          <div className="space-y-4">
            {areas.map((area) => {
              const list = apps.filter((a) => (a.area || "Sem área") === area)
              return (
                <div key={area} className="overflow-hidden rounded-xl border border-border bg-card">
                  <div className="flex items-baseline justify-between gap-3 border-b border-border px-4 py-2.5">
                    <h3 className="font-medium">{area}</h3>
                    <span className="text-xs text-muted-foreground">{list.length} {list.length === 1 ? "app" : "apps"} · {list.reduce((n, a) => n + a.runs_30d, 0)} execuções</span>
                  </div>
                  <div className="overflow-x-auto">
                    <table className="w-full min-w-[560px] text-sm">
                      <thead className="text-left text-xs text-muted-foreground">
                        <tr><th className="px-4 py-2 font-normal">App</th><th className="px-4 py-2 font-normal">Situação</th><th className="px-4 py-2 text-right font-normal">Execuções</th><th className="px-4 py-2 text-right font-normal">Erros</th><th className="px-4 py-2 font-normal">Última vez</th></tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {list.map((a) => (
                          <tr key={a.id}>
                            <td className="px-4 py-2.5"><Link to={`/apps/${a.id}`} className="font-medium hover:underline">{a.name}</Link><span className="block text-xs text-muted-foreground">{a.owner}</span></td>
                            <td className="px-4 py-2.5"><AppStatus app={a} requireApproval={settings.require_app_approval} /></td>
                            <td className="px-4 py-2.5 text-right">{a.runs_30d}</td>
                            <td className={cn("px-4 py-2.5 text-right", a.errors_30d > 0 ? "text-destructive" : "text-muted-foreground")}>{a.errors_30d}</td>
                            <td className="px-4 py-2.5 text-muted-foreground">{a.last_run_at ? formatWhen(a.last_run_at) : "Nunca"}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </section>

      <div className="mb-10 grid gap-8 lg:grid-cols-2">
        <Panel title="Computadores da equipe">
          {computers.length === 0 ? <Empty>Ninguém conectou um computador ainda.</Empty> : (
            <ul className="divide-y divide-border">
              {computers.map((c) => (
                <li key={c.id} className="flex items-center justify-between gap-3 py-2.5 text-sm">
                  <span className="flex min-w-0 items-center gap-2.5"><Monitor className="size-4 shrink-0 text-muted-foreground" /><span className="min-w-0"><span className="block truncate font-medium">{c.name}</span><span className="block text-xs text-muted-foreground">{c.person}{c.outdated && <span className="text-warn"> · Agente desatualizado</span>}</span></span></span>
                  <span className={cn("inline-flex shrink-0 items-center gap-1.5 text-xs", c.connected ? "text-ok" : "text-muted-foreground")}><span className={cn("size-2 rounded-full", c.connected ? "bg-ok" : "bg-muted-foreground/40")} />{c.connected ? "Conectado" : c.last_seen_at ? `Visto ${formatWhen(c.last_seen_at)}` : "Nunca conectou"}</span>
                </li>
              ))}
            </ul>
          )}
        </Panel>
        <div className="space-y-8">
          <Panel title="Quem mais usa (30 dias)">
            {data.top_people.length === 0 ? <Empty>Ninguém rodou apps nos últimos 30 dias.</Empty> : (
              <ol className="divide-y divide-border">
                {data.top_people.map((p) => <li key={p.name} className="flex justify-between py-2 text-sm"><span>{p.name}</span><span className="text-muted-foreground">{p.runs_30d} {p.runs_30d === 1 ? "execução" : "execuções"}</span></li>)}
              </ol>
            )}
          </Panel>
          <Panel title="Atividade recente">
            {data.activity.length === 0 ? <Empty>Nada por aqui ainda.</Empty> : (
              <ol className="divide-y divide-border">
                {data.activity.map((e, i) => <li key={i} className="flex justify-between gap-3 py-2 text-sm"><span>{describe(e)}</span><time className="shrink-0 text-xs text-muted-foreground">{formatWhen(e.at)}</time></li>)}
              </ol>
            )}
          </Panel>
        </div>
      </div>

      <CompanyControls settings={settings} onSaved={(s) => setData({ ...data, settings: s })} />
      <Companies />
    </>
  )
}

function describe(e: AdminEvent): ReactNode {
  const app = e.app_id ? <Link to={`/apps/${e.app_id}`} className="font-medium hover:underline">{e.name}</Link> : <span className="font-medium">{e.name}</span>
  const by = e.actor ? <span className="text-muted-foreground"> por {e.actor}</span> : null
  switch (e.kind) {
    case "app_registered": return <>{app} foi registrado{by}</>
    case "app_updated": return <>{app} ganhou versão nova{by}</>
    case "app_approved": return <>{app} foi aprovado{by}</>
    case "app_suspended": return <>{app} foi suspenso{by}</>
    case "app_unapproved": return <>{app} voltou para aprovação{by}</>
    case "person_joined": return <>{app} entrou na empresa</>
  }
}

function AppStatus({ app, requireApproval }: { app: AdminApp; requireApproval: boolean }) {
  if (app.suspended) return <span className="inline-flex items-center gap-1.5 text-destructive"><CirclePause className="size-3.5" />Suspenso</span>
  if (requireApproval && !app.approved) return <span className="inline-flex items-center gap-1.5 text-warn"><Clock className="size-3.5" />Esperando aprovação</span>
  return <span className="inline-flex items-center gap-1.5 text-ok"><CircleCheck className="size-3.5" />Liberado</span>
}

function Stat({ label, value, detail, tone }: { label: string; value: ReactNode; detail?: string; tone?: "warn" }) {
  return (
    <div className={cn("rounded-xl border border-border bg-card px-4 py-3", tone === "warn" && "border-warn/40 bg-warn-soft")}>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-1 font-heading text-2xl">{value}</dd>
      {detail && <dd className="text-xs text-destructive">{detail}</dd>}
    </div>
  )
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return <section><h2 className="mb-2 font-heading text-lg">{title}</h2>{children}</section>
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="text-sm text-muted-foreground">{children}</p>
}

/** Company-wide switches: pause every app, require approval, clean old history. */
function CompanyControls({ settings, onSaved }: { settings: SecuritySettings; onSaved: (s: SecuritySettings) => void }) {
  const [busy, setBusy] = useState(false)
  const [days, setDays] = useState(String(settings.retention_days))

  const save = async (next: SecuritySettings, done?: string) => {
    setBusy(true)
    try {
      onSaved(await api.saveSecuritySettings(next))
      if (done) toast.success(done)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível salvar.")
    } finally {
      setBusy(false)
    }
  }
  const togglePause = () => {
    const pausing = settings.local_execution_enabled
    if (pausing && !window.confirm("Pausar todos os apps? Ninguém da empresa vai conseguir rodar nada até você retomar. Execuções que já começaram continuam até o fim.")) return
    void save({ ...settings, local_execution_enabled: !pausing }, pausing ? "Todos os apps foram pausados." : "Apps liberados de novo.")
  }
  const purge = async () => {
    const n = Number(days)
    if (!Number.isInteger(n) || n < 1 || n > 365) return toast.error("Use um número de 1 a 365 dias.")
    if (!window.confirm(`Apagar o histórico das execuções terminadas há mais de ${n} dias? Isso não pode ser desfeito. Os arquivos que os apps salvaram no computador de cada pessoa continuam lá.`)) return
    setBusy(true)
    try {
      if (n !== settings.retention_days) onSaved(await api.saveSecuritySettings({ ...settings, retention_days: n }))
      const r = await api.purgeRuns()
      toast.success(r.deleted_runs === 1 ? "1 execução antiga apagada." : `${r.deleted_runs} execuções antigas apagadas.`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível apagar o histórico.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <section aria-labelledby="controles" className="border-t border-border pt-8">
      <h2 id="controles" className="font-heading text-xl">Controles da empresa</h2>
      <div className="mt-4 divide-y divide-border rounded-xl border border-border bg-card">
        <Control title="Pausar todos os apps" text="Para uma emergência: ninguém da empresa consegue rodar apps até você retomar.">
          <Button size="sm" variant={settings.local_execution_enabled ? "outline" : "default"} disabled={busy} onClick={togglePause}>
            {settings.local_execution_enabled ? "Pausar todos" : "Retomar apps"}
          </Button>
        </Control>
        <Control title="Aprovar apps novos antes de rodarem" text="Com isto ligado, um app novo ou com código alterado só roda depois que um administrador aprova.">
          <label className="inline-flex cursor-pointer items-center gap-2 text-sm">
            <input type="checkbox" className="size-4 accent-foreground" checked={settings.require_app_approval} disabled={busy} onChange={(e) => void save({ ...settings, require_app_approval: e.target.checked })} />
            {settings.require_app_approval ? "Ligado" : "Desligado"}
          </label>
        </Control>
        <Control title="Apagar histórico antigo" text="Apaga o registro das execuções terminadas há mais dias do que o número ao lado.">
          <div className="flex items-center gap-2">
            <Input aria-label="Dias de histórico" type="number" min={1} max={365} value={days} onChange={(e) => setDays(e.target.value)} className="h-8 w-20" />
            <span className="text-sm text-muted-foreground">dias</span>
            <Button size="sm" variant="outline" disabled={busy} onClick={() => void purge()}><TriangleAlert className="size-3.5" />Apagar</Button>
          </div>
        </Control>
      </div>
    </section>
  )
}

/** Only in the principal company: turns screen apps (shared apps that run on
 * the Ziv's server) on or off for each other company. Hidden elsewhere. */
function Companies() {
  const [list, setList] = useState<CompanyShared[] | null>(null)
  const [busy, setBusy] = useState("")
  useEffect(() => {
    let live = true
    api.adminCompanies().then((l) => { if (live) setList(l) }, () => { if (live) setList(null) })
    return () => { live = false }
  }, [])
  if (!list) return null

  const toggle = async (c: CompanyShared) => {
    const on = !c.shared_apps
    if (on && !window.confirm(`Ligar apps com tela para ${c.name}? Pessoas dessa empresa poderão rodar programas no servidor do Ziv.`)) return
    setBusy(c.slug)
    try {
      const saved = await api.setCompanyShared(c.slug, on)
      setList((l) => l && l.map((x) => (x.slug === saved.slug ? saved : x)))
      toast.success(on ? `Apps com tela ligados para ${c.name}.` : `Apps com tela desligados para ${c.name}. Os que estavam abertos vão dormir em até 1 minuto.`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível salvar.")
    } finally {
      setBusy("")
    }
  }

  return (
    <section aria-labelledby="empresas" className="mt-10 border-t border-border pt-8">
      <h2 id="empresas" className="font-heading text-xl">Empresas</h2>
      <p className="mt-1 mb-4 text-sm text-muted-foreground">Apps com tela rodam no servidor do Ziv. Cada empresa só usa depois que você ligar.</p>
      {list.length === 0 ? <Empty>Nenhuma outra empresa ainda.</Empty> : (
        <ul className="divide-y divide-border rounded-xl border border-border bg-card">
          {list.map((c) => (
            <li key={c.slug} className="flex items-center justify-between gap-3 p-4">
              <span className="min-w-0"><span className="block truncate text-sm font-medium">{c.name}</span><span className="block text-xs text-muted-foreground">/w/{c.slug}</span></span>
              <label className="inline-flex shrink-0 cursor-pointer items-center gap-2 text-sm">
                <input type="checkbox" role="switch" aria-label={`Apps com tela de ${c.name}`} className="size-4 accent-foreground"
                  checked={c.shared_apps} disabled={busy !== ""} onChange={() => void toggle(c)} />
                {c.shared_apps ? "Apps com tela ligados" : "Apps com tela desligados"}
              </label>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function Control({ title, text, children }: { title: string; text: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
      <div><p className="text-sm font-medium">{title}</p><p className="mt-0.5 text-sm text-muted-foreground">{text}</p></div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}
