import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { Field, NativeSelect } from "@/components/bits"
import { Section } from "@/components/ficha/Part"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, type AppDetail, type Deployment, type SharedInfo, type UsagePoint } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { errText, settingsOf } from "./settings"

const MEMORY = [256, 512, 1024, 2048]
const CPUS = [0.25, 0.5, 1, 2]
const IDLE = [15, 30, 60, 120]
const cpuText = (n: number) => String(n).replace(".", ",")

const DEP_STATUS: Record<Deployment["status"], string> = {
  building: "montando",
  failed: "não deu certo",
  ready: "teste pronto",
  live: "no ar",
  retired: "encerrada",
}

function Limits({ app, info, onChanged }: { app: AppDetail; info: SharedInfo; onChanged: (next?: AppDetail) => void }) {
  const base = settingsOf(app, info.shared)
  const [mem, setMem] = useState(base.memory_mb)
  const [cpus, setCpus] = useState(base.cpus)
  const [always, setAlways] = useState(base.always_on)
  const [idle, setIdle] = useState(base.idle_minutes)
  const [busy, setBusy] = useState(false)
  const editable = app.can.restore
  const dirty = mem !== base.memory_mb || cpus !== base.cpus || always !== base.always_on || idle !== base.idle_minutes

  const save = async () => {
    setBusy(true)
    try {
      const next = await api.shared.save(app.id, { ...base, memory_mb: mem, cpus, always_on: always, idle_minutes: idle })
      toast.success(info.shared?.status === "running" ? "Salvo. Religue o app para valer." : "Salvo.")
      onChanged(next)
    } catch (e) {
      toast.error(errText(e, "Não consegui salvar."))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex max-w-xl flex-col gap-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <Field label="Memória" htmlFor="lim-mem">
          <NativeSelect id="lim-mem" value={mem} disabled={!editable} onChange={(e) => setMem(Number(e.target.value))}>
            {MEMORY.map((m) => <option key={m} value={m}>{m} MB</option>)}
          </NativeSelect>
        </Field>
        <Field label="Processador" htmlFor="lim-cpu">
          <NativeSelect id="lim-cpu" value={cpus} disabled={!editable} onChange={(e) => setCpus(Number(e.target.value))}>
            {CPUS.map((c) => <option key={c} value={c}>{cpuText(c)}</option>)}
          </NativeSelect>
        </Field>
        {!always && (
          <Field label="Descansar depois de" htmlFor="lim-idle">
            <NativeSelect id="lim-idle" value={idle} disabled={!editable} onChange={(e) => setIdle(Number(e.target.value))}>
              {IDLE.map((m) => <option key={m} value={m}>{m} min sem uso</option>)}
            </NativeSelect>
          </Field>
        )}
      </div>
      <label className="flex items-start gap-2 text-sm">
        <Checkbox className="mt-0.5" checked={always} disabled={!editable} onCheckedChange={(c) => setAlways(c === true)} />
        <span>
          Sempre ligado
          <span className="block text-muted-foreground">Gasta mais: fica ligado mesmo sem ninguém usando.</span>
        </span>
      </label>
      {editable && (
        <div>
          <Button size="sm" onClick={save} disabled={busy || !dirty}>{busy ? "Salvando…" : "Salvar limites"}</Button>
        </div>
      )}
    </div>
  )
}

/** Memory over the last 24 hours as one plain line. */
export function UsageChart({ points }: { points: UsagePoint[] }) {
  if (points.length === 0) return <p className="text-sm text-muted-foreground">Ainda sem medições nas últimas 24 h.</p>
  const W = 600
  const H = 80
  const max = Math.max(...points.map((p) => p.mem_mb))
  const top = max > 0 ? max * 1.1 : 1
  const t0 = new Date(points[0].at).getTime()
  const span = Math.max(1, new Date(points[points.length - 1].at).getTime() - t0)
  const d = points
    .map((p, i) => {
      const x = points.length === 1 ? W / 2 : ((new Date(p.at).getTime() - t0) / span) * W
      const y = H - (p.mem_mb / top) * H
      return `${i ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(" ")
  return (
    <figure className="flex max-w-2xl flex-col gap-1.5">
      <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className="h-20 w-full text-foreground" role="img" aria-label={`Memória usada nas últimas 24 h, no máximo ${Math.round(max)} MB`}>
        <line x1="0" y1={H - 0.5} x2={W} y2={H - 0.5} className="stroke-border" strokeWidth="1" vectorEffect="non-scaling-stroke" />
        <path d={d} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" strokeLinejoin="round" />
      </svg>
      <figcaption className="flex justify-between text-xs text-muted-foreground">
        <span>Memória usada (MB)</span>
        <span>máximo {Math.round(max)} MB</span>
      </figcaption>
    </figure>
  )
}

function Logs({ appId }: { appId: number }) {
  const [text, setText] = useState<string | null>(null)
  const [error, setError] = useState("")
  const load = useCallback(() => {
    setError("")
    api.shared.logs(appId).then((l) => setText(l.text), (e) => setError(errText(e, "Não consegui ler as mensagens do app.")))
  }, [appId])
  useEffect(load, [load])
  return (
    <div className="flex flex-col gap-2">
      {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
      {text !== null && (text ? (
        <pre className="max-h-80 overflow-auto rounded-md bg-secondary p-3 font-mono text-xs whitespace-pre-wrap [overflow-wrap:anywhere]">{text}</pre>
      ) : (
        <p className="text-sm text-muted-foreground">O app ainda não escreveu nada.</p>
      ))}
      <div>
        <Button size="sm" variant="outline" onClick={load}>Atualizar</Button>
      </div>
    </div>
  )
}

function History({ deployments }: { deployments: Deployment[] }) {
  const [log, setLog] = useState<Deployment | null>(null)
  if (deployments.length === 0) return <p className="text-sm text-muted-foreground">Ainda não houve publicações.</p>
  return (
    <>
      <ul className="flex flex-col gap-2 text-sm">
        {deployments.map((d) => (
          <li key={d.id} className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span>{formatWhen(d.created_at)}</span>
            <span className="text-muted-foreground">
              · {d.kind === "test" ? "teste" : "publicação"}
              {d.version_number != null && ` da versão ${d.version_number}`}
              {d.created_by_name && ` por ${d.created_by_name}`} · {DEP_STATUS[d.status]}
            </span>
            {d.build_log && (
              <button type="button" className="text-muted-foreground underline underline-offset-4 hover:text-foreground" onClick={() => setLog(d)}>
                Ver registro
              </button>
            )}
          </li>
        ))}
      </ul>
      <Dialog open={!!log} onOpenChange={(o) => !o && setLog(null)}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Registro da montagem</DialogTitle>
            <DialogDescription>{log && formatWhen(log.created_at)}</DialogDescription>
          </DialogHeader>
          <pre className="max-h-[60vh] overflow-auto rounded-md bg-secondary p-3 font-mono text-xs whitespace-pre-wrap [overflow-wrap:anywhere]">{log?.build_log}</pre>
        </DialogContent>
      </Dialog>
    </>
  )
}

/** The shared app's technical parts, inside "Para quem cuida do app". */
export function Tech({ app, onChanged }: { app: AppDetail; onChanged: (next?: AppDetail) => void }) {
  const [info, setInfo] = useState<SharedInfo | null>(null)
  const [usage, setUsage] = useState<UsagePoint[] | null>(null)
  useEffect(() => {
    let gone = false
    api.shared.get(app.id).then((i) => !gone && setInfo(i), () => {})
    api.shared.usage(app.id).then((u) => !gone && setUsage(u), () => !gone && setUsage([]))
    return () => {
      gone = true
    }
  }, [app.id, app.updated_at])

  if (!info?.shared) return null
  const deps = info.deployments
  return (
    <>
      <Section title="Limites do app">
        <Limits key={app.updated_at} app={app} info={info} onChanged={onChanged} />
      </Section>
      <Section title="Uso nas últimas 24 h">
        {usage ? <UsageChart points={usage} /> : <p className="text-sm text-muted-foreground">Carregando…</p>}
      </Section>
      <Section title="Mensagens do app">
        <Logs appId={app.id} />
      </Section>
      {deps && (
        <Section title="Publicações">
          <History deployments={deps} />
        </Section>
      )}
    </>
  )
}
