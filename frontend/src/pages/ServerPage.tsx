import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react"
import { Link } from "react-router-dom"
import { toast } from "sonner"
import { TriangleAlert } from "lucide-react"
import { Field, LoadError, NativeSelect, PageHeader } from "@/components/bits"
import { CPU_CHOICES, IDLE_CHOICES, MEMORY_CHOICES, cpuText, errText } from "@/components/ficha/shared/settings"
import { SharedStatusDot } from "@/components/SharedStatus"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { api, ApiError, type ServerApp, type ServerInfo } from "@/lib/api"
import { formatBytes, formatCents } from "@/lib/shared"
import { cn } from "@/lib/utils"

const COST_ERROR = "Digite um valor em reais, por exemplo 300 ou 300,50."
const centsText = (c: number) => (c / 100).toFixed(2).replace(".", ",")
const ROW = "md:grid md:grid-cols-[minmax(0,1.4fr)_minmax(0,0.9fr)_0.6fr_minmax(0,1.4fr)_minmax(0,1.1fr)_0.7fr_auto] md:items-center md:gap-4"
const CELL_LABEL = "text-xs text-muted-foreground md:hidden"

/** "300" or "300,50" (also "1.300,50") in reais to cents; null when it makes no sense. */
export function parseReais(v: string): number | null {
  const n = parseFloat(v.trim().replace(/\./g, "").replace(",", "."))
  if (!Number.isFinite(n) || n < 0) return null
  return Math.round(n * 100)
}

function Meter({ label, used, total, warn }: { label: string; used: number; total: number; warn?: boolean }) {
  const pct = total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="text-xl font-semibold tabular-nums">{formatBytes(used)} <span className="text-sm font-normal text-muted-foreground">de {formatBytes(total)}</span></p>
      <div role="meter" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} className="h-2 overflow-hidden rounded-full bg-secondary">
        <div className={cn("h-full rounded-full", warn ? "bg-warn" : "bg-foreground/60")} style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}

function LimitsDialog({ app, onClose, onSaved }: { app: ServerApp; onClose: () => void; onSaved: () => void }) {
  const [mem, setMem] = useState(app.memory_mb)
  const [cpus, setCpus] = useState(app.cpus)
  const [always, setAlways] = useState(app.always_on)
  const [idle, setIdle] = useState(app.idle_minutes)
  const [busy, setBusy] = useState(false)
  const save = async () => {
    setBusy(true)
    try {
      const r = await api.server.setApp(app.app_id, { memory_mb: mem, cpus, always_on: always, idle_minutes: idle })
      toast.success(r.needs_restart ? "Salvo. Vale na próxima vez que ligar." : "Salvo.")
      onSaved()
      onClose()
    } catch (e) {
      toast.error(errText(e, "Não consegui salvar. Tente de novo em alguns segundos."))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Limites de {app.name}</DialogTitle>
          <DialogDescription>Quanto o app pode usar do servidor.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <Field label="Memória" htmlFor="srv-mem">
              <NativeSelect id="srv-mem" value={mem} onChange={(e) => setMem(Number(e.target.value))}>
                {(MEMORY_CHOICES.includes(mem) ? MEMORY_CHOICES : [...MEMORY_CHOICES, mem].sort((a, b) => a - b)).map((m) => <option key={m} value={m}>{m} MB</option>)}
              </NativeSelect>
            </Field>
            <Field label="Processador" htmlFor="srv-cpu">
              <NativeSelect id="srv-cpu" value={cpus} onChange={(e) => setCpus(Number(e.target.value))}>
                {(CPU_CHOICES.includes(cpus) ? CPU_CHOICES : [...CPU_CHOICES, cpus].sort((a, b) => a - b)).map((c) => <option key={c} value={c}>{cpuText(c)}</option>)}
              </NativeSelect>
            </Field>
            {!always && (
              <Field label="Descansar depois de" htmlFor="srv-idle">
                <NativeSelect id="srv-idle" value={idle} onChange={(e) => setIdle(Number(e.target.value))}>
                  {(IDLE_CHOICES.includes(idle) ? IDLE_CHOICES : [...IDLE_CHOICES, idle].sort((a, b) => a - b)).map((m) => <option key={m} value={m}>{m} min sem uso</option>)}
                </NativeSelect>
              </Field>
            )}
          </div>
          <label className="flex items-start gap-2 text-sm">
            <Checkbox className="mt-0.5" checked={always} onCheckedChange={(c) => setAlways(c === true)} />
            <span>
              Sempre ligado
              <span className="block text-muted-foreground">Gasta mais: fica ligado mesmo sem ninguém usando.</span>
            </span>
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancelar</Button>
          <Button onClick={save} disabled={busy}>{busy ? "Salvando…" : "Salvar limites"}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function Cell({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  return (
    <div className={cn("flex items-baseline justify-between gap-3 text-sm md:block", className)}>
      <span className={CELL_LABEL}>{label}</span>
      <span className="text-right md:text-left">{children}</span>
    </div>
  )
}

export function ServerPage() {
  const [info, setInfo] = useState<ServerInfo | null>(null)
  const [denied, setDenied] = useState(false)
  const [loadError, setLoadError] = useState("")
  const [cost, setCost] = useState("")
  const [costError, setCostError] = useState("")
  const [costBusy, setCostBusy] = useState(false)
  const [stopping, setStopping] = useState<ServerApp | null>(null)
  const [limits, setLimits] = useState<ServerApp | null>(null)
  const [busy, setBusy] = useState(false)
  const [costTouched, setCostTouched] = useState(false)

  const load = useCallback(() => {
    api.server.get().then(
      (i) => {
        setInfo(i)
        setLoadError("")
        setCost((c) => (c === "" ? centsText(i.monthly_cost_cents) : c))
      },
      (e) => {
        if (e instanceof ApiError && e.status === 403) setDenied(true)
        else setLoadError(errText(e, "O Hub não respondeu. Tente de novo em alguns segundos."))
      },
    )
  }, [])
  useEffect(() => {
    load()
    const t = setInterval(load, 60_000)
    return () => clearInterval(t)
  }, [load])

  const saveCost = async (e: FormEvent) => {
    e.preventDefault()
    const cents = parseReais(cost)
    if (cents === null) {
      setCostError(COST_ERROR)
      return
    }
    setCostError("")
    setCostBusy(true)
    try {
      await api.server.setCost(cents)
      toast.success("Custo salvo.")
      setCost(centsText(cents))
      setCostTouched(false)
      load()
    } catch (err) {
      toast.error(errText(err, "Não consegui salvar o custo. Tente de novo em alguns segundos."))
    } finally {
      setCostBusy(false)
    }
  }

  const act = async (app: ServerApp, what: "stop" | "start") => {
    setBusy(true)
    try {
      await api.server[what](app.app_id)
      toast.success(what === "stop" ? `${app.name} parado.` : `${app.name} ligando.`)
      setStopping(null)
      load()
    } catch (e) {
      toast.error(errText(e, "Não deu certo. Tente de novo em alguns segundos."))
      setStopping(null)
    } finally {
      setBusy(false)
    }
  }

  if (denied) return <p>Você não tem acesso a esta página.</p>
  if (!info) {
    return loadError ? <LoadError message={loadError} onRetry={load} /> : <p className="text-muted-foreground">Carregando…</p>
  }

  const spent = info.apps.reduce((s, a) => s + a.cost_cents, 0) + info.hub_cents
  const hasCost = info.monthly_cost_cents > 0

  return (
    <div className="flex flex-col gap-8">
      <div>
        <PageHeader title="Servidor e gastos" />
        <p className="-mt-6 text-muted-foreground">Quanto o servidor usa e quanto cada app compartilhado custa, aproximadamente.</p>
      </div>

      {!info.engine_ok && (
        <p role="alert" className="flex items-start gap-2 rounded-lg border border-warn/40 bg-warn-soft/60 p-3 text-sm">
          <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warn" />
          O Podman não está respondendo. Os apps compartilhados não abrem até ele voltar.
        </p>
      )}
      {loadError && <p role="alert" className="text-sm text-destructive">{loadError}</p>}

      <section className="grid gap-4 md:grid-cols-3" aria-label="Uso do servidor">
        <Meter label="Memória" used={info.memory_used} total={info.memory_total} />
        <div className="flex flex-col gap-2">
          <Meter label="Disco" used={info.disk_used} total={info.disk_total} warn={info.disk_warning} />
          {info.disk_warning && <p className="text-sm text-warn">O disco passou de 90%. Apague cópias antigas ou aumente o disco.</p>}
        </div>
        <div className="flex flex-col gap-2 rounded-lg border border-border p-4 md:self-start">
          <p className="text-sm text-muted-foreground">Gasto do mês até agora</p>
          <p className="text-xl font-semibold tabular-nums">{hasCost ? formatCents(spent) : "—"}</p>
        </div>
      </section>

      <form onSubmit={saveCost} className="flex max-w-md flex-col gap-2" noValidate>
        <Field label="Quanto o servidor custa por mês" htmlFor="srv-cost" hint="Em reais, por exemplo 300 ou 300,50.">
          <div className="flex gap-2">
            <Input id="srv-cost" inputMode="decimal" value={cost} aria-invalid={!!costError} onChange={(e) => { setCost(e.target.value); setCostTouched(true) }} />
            <Button type="submit" disabled={costBusy || (!costTouched && cost !== "")} variant="outline">{costBusy ? "Salvando…" : "Salvar custo"}</Button>
          </div>
        </Field>
        {costError && <p role="alert" className="text-sm text-destructive">{costError}</p>}
      </form>

      {!hasCost && <p className="text-sm text-muted-foreground">Informe o custo mensal para ver quanto cada app gasta.</p>}

      <section aria-label="Apps compartilhados" className="flex flex-col">
        <div className={cn("hidden border-b border-border pb-2 text-xs text-muted-foreground", ROW)} aria-hidden>
          <span>App</span><span>Estado</span><span>Ligado no mês</span><span>Memória (média / máxima / reservada)</span><span>Banco e cópias</span><span>Gasto</span><span className="w-40" />
        </div>
        {info.apps.length === 0 && <p className="py-4 text-sm text-muted-foreground">Nenhum app compartilhado ainda.</p>}
        <ul className="flex flex-col gap-3 md:gap-0">
          {info.apps.map((a) => {
            const running = a.status === "running" || a.status === "starting" || a.status === "sleeping"
            const canStart = a.status === "stopped" || a.status === "crashed"
            return (
              <li key={a.app_id} className={cn("flex flex-col gap-2 rounded-lg border border-border p-4 md:rounded-none md:border-0 md:border-b md:px-0 md:py-3", ROW)}>
                <Link to={`/apps/${a.app_id}`} className="font-medium underline-offset-4 hover:underline">{a.name}</Link>
                <Cell label="Estado"><SharedStatusDot status={a.status} /></Cell>
                <Cell label="Ligado no mês">{Math.round(a.hours_on)} h</Cell>
                <Cell label="Memória">{Math.round(a.avg_mem_mb)} / {Math.round(a.max_mem_mb)} MB de {a.memory_mb} MB</Cell>
                <Cell label="Banco e cópias">{formatBytes(a.db_bytes)} · cópias {formatBytes(a.backup_bytes)}</Cell>
                <Cell label="Gasto" className="tabular-nums">{hasCost ? formatCents(a.cost_cents) : "—"}</Cell>
                <div className="flex gap-2 md:justify-end">
                  {running && <Button size="sm" variant="outline" aria-label={`Parar ${a.name}`} onClick={() => setStopping(a)}>Parar</Button>}
                  {canStart && <Button size="sm" variant="outline" aria-label={`Ligar ${a.name}`} disabled={busy} onClick={() => act(a, "start")}>Ligar</Button>}
                  <Button size="sm" variant="ghost" aria-label={`Limites de ${a.name}`} onClick={() => setLimits(a)}>Limites</Button>
                </div>
              </li>
            )
          })}
        </ul>
        <div className="mt-3 flex items-baseline justify-between gap-3 px-4 text-sm md:px-0">
          <span>Hub e espaço livre</span>
          <span className="tabular-nums">{hasCost ? formatCents(info.hub_cents) : "—"}</span>
        </div>
      </section>

      <p className="text-xs text-muted-foreground">A conta divide o custo do servidor pela memória reservada × tempo ligado de cada app. É uma estimativa.</p>

      <Dialog open={stopping !== null} onOpenChange={(o) => !o && setStopping(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Parar {stopping?.name}?</DialogTitle>
            <DialogDescription>Ninguém consegue abrir até alguém ligar de novo.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setStopping(null)}>Cancelar</Button>
            <Button variant="destructive" disabled={busy} onClick={() => stopping && act(stopping, "stop")}>Sim, parar</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      {limits && <LimitsDialog app={limits} onClose={() => setLimits(null)} onSaved={load} />}
    </div>
  )
}
