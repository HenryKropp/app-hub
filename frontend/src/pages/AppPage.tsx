import { useEffect, useState, type ReactNode } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, FolderOpen, KeyRound, Play, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { HistoryTimeline } from "@/components/HistoryTimeline"
import { KindTab, LoadError, SavedIn } from "@/components/bits"
import { api, type AppDetail, type Input } from "@/lib/api"
import { useRuns } from "@/lib/runs-context"
import { formatWhen } from "@/lib/runs"
import { INPUT_SOURCES, kindInfo } from "@/lib/kinds"

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="grid gap-x-10 gap-y-3 border-t border-border py-6 md:grid-cols-[12rem_1fr]">
      <h2 className="text-sm font-semibold">{title}</h2>
      <div className="min-w-0">{children}</div>
    </section>
  )
}

// Most apps take every input in their own window; then saying it once is enough.
function InputList({ inputs }: { inputs: Input[] }) {
  const same = inputs.every((i) => i.source === inputs[0].source)
  return (
    <>
      {same && <p className="mb-2 text-sm text-muted-foreground">{inputs[0].source === "window" ? "Você preenche tudo na janela do app:" : `Tudo via ${INPUT_SOURCES[inputs[0].source]}:`}</p>}
      <ul className="flex list-disc flex-col gap-1 pl-5 text-sm marker:text-border">
        {inputs.map((i) => (
          <li key={i.id}>
            {i.label}
            {!same && <span className="text-muted-foreground"> · {INPUT_SOURCES[i.source]}</span>}
          </li>
        ))}
      </ul>
    </>
  )
}

const NOTES_SHOWN = 3

function Notes({ notes }: { notes: string[] }) {
  const [all, setAll] = useState(false)
  const shown = all ? notes : notes.slice(0, NOTES_SHOWN)
  return (
    <>
      <ul className="flex list-disc flex-col gap-2 pl-5 text-sm marker:text-warn">
        {shown.map((n) => <li key={n}>{n}</li>)}
      </ul>
      {notes.length > NOTES_SHOWN && (
        <button type="button" onClick={() => setAll(!all)} className="mt-3 text-sm font-medium text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
          {all ? "Mostrar menos" : `Ver mais ${notes.length - NOTES_SHOWN}`}
        </button>
      )}
    </>
  )
}

export function AppPage() {
  const { id } = useParams()
  const [app, setApp] = useState<AppDetail | null>(null)
  const [error, setError] = useState("")
  const canEdit = app?.can.edit ?? false
  const canRestore = app?.can.restore ?? false
  const [readable, setReadable] = useState(true)
  const [historyKey, setHistoryKey] = useState(0)
  const [tries, setTries] = useState(0)
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
  }, [id, tries])

  if (error) return <LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} />
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
          <p className="mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
            <span title={kindInfo(m.kind).label}><KindTab kind={m.kind} /></span>
            {[
              m.area,
              m.owner && <>Responsável <span className="text-foreground">{m.owner.name}</span></>,
              m.backup ? <>Reserva <span className="text-foreground">{m.backup.name}</span></> : <span className="text-warn">Sem reserva</span>,
              app.version && <span className="inline-flex gap-1"><span>versão {app.version.number}</span><span>de {formatWhen(app.version.created_at)}</span></span>,
            ].filter(Boolean).map((part, i) => (
              <span key={i} className="inline-flex items-center gap-2"><span aria-hidden className="text-muted-foreground/50">·</span>{part}</span>
            ))}
          </p>
        </div>
        {app.can_run && (
          <div className="flex shrink-0 flex-col items-start gap-2 lg:items-end">
            <Button variant="run" onClick={() => run(target)} disabled={busy.has(app.id)} className="h-11 gap-2 pr-6 pl-5 text-[0.95rem]">
              <Play className="size-4" /> Rodar
            </Button>
            {saved && <SavedIn folder={saved} full onChange={() => run(target, { ask: true })} className="max-w-sm text-sm lg:text-right" />}
          </div>
        )}
      </header>

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
              <InputList inputs={m.inputs!} />
            )}
          </Section>
          <Section title="O que ele devolve">
            {(m.outputs ?? []).length === 0 ? (
              <p className="text-sm text-muted-foreground">Ainda não identificado.</p>
            ) : (
              <ul className="flex flex-col gap-1.5 text-sm">
                {m.outputs!.map((o) => (
                  <li key={o.pattern} title={o.pattern}>{o.label}</li>
                ))}
              </ul>
            )}
          </Section>
          {(m.notes ?? []).length > 0 && (
            <Section title="Pontos de atenção">
              <Notes notes={m.notes!} />
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
            <HistoryTimeline preview={5} appId={app.id} canEdit={canEdit} canRestore={canRestore} reloadKey={historyKey} onChanged={() => void api.app(app.id).then(setApp, () => {})} />
          </Section>
          <Section title="Onde está">
            <p className="flex items-start gap-2 text-sm">
              <FolderOpen className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <span className="[overflow-wrap:anywhere]">{app.folder_display}</span>
            </p>
            <p className="mt-1.5 pl-6 text-xs text-muted-foreground">A ficha também fica salva nessa pasta.</p>
          </Section>
        </div>
    </article>
  )
}
