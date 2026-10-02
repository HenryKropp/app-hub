import { useEffect, useState, type ReactNode } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, FileText, FolderOpen, KeyRound, Play, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { HistoryTimeline } from "@/components/HistoryTimeline"
import { KindTab } from "@/components/bits"
import { api, type AppDetail } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useRuns } from "@/lib/runs-context"
import { formatWhen } from "@/lib/runs"
import { INPUT_SOURCES, INPUT_TYPES, kindInfo } from "@/lib/kinds"

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="grid gap-x-10 gap-y-3 border-t border-border py-6 md:grid-cols-[12rem_1fr]">
      <h2 className="text-sm font-semibold">{title}</h2>
      <div className="min-w-0">{children}</div>
    </section>
  )
}

export function AppPage() {
  const { id } = useParams()
  const [app, setApp] = useState<AppDetail | null>(null)
  const [error, setError] = useState("")
  const { person } = useAuth()
  const canEdit = person?.role === "admin" || person?.role === "criador"
  const [readable, setReadable] = useState(true)
  const [historyKey, setHistoryKey] = useState(0)
  const { run, busy, rememberedFor } = useRuns()

  // The ficha shows what the Hub already knows; the folder check (OneDrive can
  // take seconds) runs behind it and only adds a new version or the warning.
  useEffect(() => {
    if (!id) return
    let gone = false
    api.app(id).then(
      (d) => {
        if (gone) return
        setApp(d)
        setReadable(true)
        api.checkApp(id).then(
          (c) => {
            if (gone) return
            setReadable(c.folder_readable)
            if (c.version && c.version.number !== d.version?.number) {
              setApp((a) => (a ? { ...a, version: c.version } : a))
              setHistoryKey((k) => k + 1)
            }
          },
          () => !gone && setReadable(false),
        )
      },
      (e: Error) => setError(e.message),
    )
    return () => {
      gone = true
    }
  }, [id])

  if (error) return <p className="text-destructive">{error}</p>
  if (!app) return <p className="text-muted-foreground">Carregando…</p>
  const m = app.manifest
  const target = { id: app.id, name: m.name, folder_display: app.folder_display, remembered_folder: app.remembered_folder }
  const saved = rememberedFor(target)

  return (
    <article>
      <Link to="/" className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Catálogo
      </Link>
      <header className="flex flex-col gap-5 pb-8 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0">
          <h1 className="font-heading text-3xl leading-tight md:text-4xl">{m.name}</h1>
          <p className="mt-3 max-w-[68ch] text-base leading-relaxed text-muted-foreground">{m.summary || "Sem resumo ainda."}</p>
        </div>
        {app.can_run && (
          <div className="flex shrink-0 flex-col items-start gap-2 lg:items-end">
            <Button variant="run" onClick={() => run(target)} disabled={busy.has(app.id)} className="h-11 gap-2 pr-6 pl-5 text-[0.95rem]">
              <Play className="size-4" /> Rodar
            </Button>
            {saved && (
              <span className="max-w-sm min-w-0 text-sm text-muted-foreground lg:text-right">
                salva em <code className="font-mono text-xs break-all text-foreground/80">{saved}</code> ·{" "}
                <button type="button" onClick={() => run(target, { ask: true })} className="underline underline-offset-4 hover:text-foreground">
                  trocar
                </button>
              </span>
            )}
          </div>
        )}
      </header>

      <dl className="grid grid-cols-2 overflow-hidden rounded-xl border border-border text-sm lg:grid-cols-5">
        {[
          ["Tipo", <span key="t" className="inline-flex flex-wrap items-center gap-1.5"><KindTab kind={m.kind} /><span className="font-normal text-muted-foreground">{kindInfo(m.kind).label}</span></span>],
          ["Área", m.area || "—"],
          ["Responsável", m.owner?.name ?? "—"],
          ["Reserva", m.backup?.name ?? <span key="r" className="text-warn">ninguém ainda</span>],
          ["Versão", app.version ? <span key="v"><span className="font-mono text-[0.8rem]">versão {app.version.number}</span> · <span className="font-mono text-[0.8rem]">{formatWhen(app.version.created_at)}</span></span> : "—"],
        ].map(([k, v], i) => (
          <div key={i} className="border-border px-4 py-3 max-lg:odd:border-r max-lg:not-last:border-b lg:not-last:border-r max-lg:first:col-span-2 max-lg:first:border-r-0">
            <dt className="text-xs text-muted-foreground">{k}</dt>
            <dd className="mt-0.5 font-medium">{v}</dd>
          </div>
        ))}
      </dl>
        {!readable && (
          <p role="alert" className="mt-4 flex gap-2 text-sm text-warn">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" /> Não consegui ler a pasta do app agora. O histórico continua aqui, mas mudanças novas no código não são registradas e não dá para voltar versões.
          </p>
        )}

        <div className="mt-8">
          <Section title="O que ele recebe">
            {(m.inputs ?? []).length === 0 ? (
              <p className="text-sm text-muted-foreground">Nada. É só rodar.</p>
            ) : (
              <ul className="flex flex-col gap-1.5 text-sm">
                {m.inputs!.map((i) => (
                  <li key={i.id}>
                    <span className="font-medium">{i.label}</span>{" "}
                    <span className="text-muted-foreground">
                      ({INPUT_TYPES.find((t) => t.value === i.type)?.label.toLowerCase()}, via {INPUT_SOURCES[i.source]})
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Section>
          <Section title="O que ele devolve">
            {(m.outputs ?? []).length === 0 ? (
              <p className="text-sm text-muted-foreground">Ainda não identificado.</p>
            ) : (
              <ul className="flex flex-col gap-1.5 text-sm">
                {m.outputs!.map((o) => (
                  <li key={o.pattern}>
                    {o.label} <code className="font-mono text-xs text-muted-foreground">{o.pattern}</code>
                  </li>
                ))}
              </ul>
            )}
          </Section>
          {(m.notes ?? []).length > 0 && (
            <Section title="Pontos de atenção">
              <ul className="flex flex-col gap-2 text-sm">
                {m.notes!.map((n) => (
                  <li key={n} className="flex gap-2">
                    <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warn" /> {n}
                  </li>
                ))}
              </ul>
            </Section>
          )}
          {(m.secrets ?? []).length > 0 && (
            <Section title="Chaves e senhas">
              <p className="flex gap-2 text-sm">
                <KeyRound className="mt-0.5 size-4 shrink-0 text-destructive" />
                Usa {m.secrets!.length} chave(s) ou senha(s) escritas no código. Os valores nunca aparecem no Hub.
              </p>
            </Section>
          )}
          <Section title="Histórico">
            <HistoryTimeline appId={app.id} canEdit={canEdit} reloadKey={historyKey} onChanged={() => void api.app(app.id).then(setApp, () => {})} />
          </Section>
          <Section title="Onde está">
            <p className="flex items-start gap-2 text-sm">
              <FolderOpen className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <code className="font-mono text-xs break-all">{app.folder_display}</code>
            </p>
            <p className="mt-2 flex items-start gap-2 text-sm text-muted-foreground">
              <FileText className="mt-0.5 size-4 shrink-0" />A ficha e o manifesto (app-hub.yaml) também estão salvos dentro dessa pasta.
            </p>
          </Section>
        </div>
    </article>
  )
}
