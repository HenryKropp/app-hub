import { useState } from "react"
import { Link } from "react-router-dom"
import { ChevronRight } from "lucide-react"
import { RunState } from "@/components/bits"
import type { AppLink, Input, Manifest, RunSummary } from "@/lib/api"
import { INPUT_SOURCES } from "@/lib/kinds"
import { formatWhen } from "@/lib/runs"

// Most apps take every input in their own window; then saying it once is enough.
export function InputList({ inputs }: { inputs: Input[] }) {
  if (inputs.length === 0) return <p className="text-sm text-muted-foreground">Nada. É só rodar.</p>
  const same = inputs.every((i) => i.source === inputs[0].source)
  return (
    <>
      {same && <p className="mb-2 text-sm text-muted-foreground">{inputs[0].source === "window" ? "Você preenche tudo na janela do app:" : `Tudo via ${INPUT_SOURCES[inputs[0].source]}:`}</p>}
      <ul className="flex list-disc flex-col gap-1 pl-5 text-sm marker:text-border">
        {inputs.map((i) => (
          <li key={i.id}>
            {i.label}
            {!same && <span className="text-muted-foreground"> · {INPUT_SOURCES[i.source]}</span>}
            {i.type === "choice" && (i.options ?? []).length > 0 && (
              <span className="block text-muted-foreground">
                {i.options!.join(" · ")}
                {i.multiple && " (pode marcar mais de uma)"}
              </span>
            )}
          </li>
        ))}
      </ul>
    </>
  )
}

export function OutputList({ outputs }: { outputs: Manifest["outputs"] }) {
  if (!outputs?.length) return <p className="text-sm text-muted-foreground">Ainda não identificado.</p>
  return (
    <ul className="flex list-disc flex-col gap-1 pl-5 text-sm marker:text-border">
      {outputs.map((o, i) => (
        <li key={`${o.pattern}-${i}`} title={o.pattern}>{o.label}</li>
      ))}
    </ul>
  )
}

const NOTES_SHOWN = 3

export function Notes({ notes }: { notes: string[] }) {
  const [all, setAll] = useState(false)
  if (notes.length === 0) return <p className="text-sm text-muted-foreground">Nenhum ponto de atenção.</p>
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

export function People({ owner, backup }: { owner: string; backup: string }) {
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <p>Responsável: {owner || <span className="text-warn">ninguém ainda</span>}</p>
      <p>Reserva: {backup || <span className="text-warn">ninguém ainda</span>}</p>
    </div>
  )
}

// An app the person sees gets a link; one they cannot see shows only its name;
// one the catalog does not know says so.
function AppName({ a }: { a: AppLink }) {
  if (a.id === 0) return <span className="text-muted-foreground">{a.slug} · fora do catálogo</span>
  if (!a.visible) return <span>{a.name}</span>
  return <Link to={`/apps/${a.id}`} className="underline underline-offset-4 hover:text-foreground">{a.name}</Link>
}

function AppNames({ apps }: { apps: AppLink[] }) {
  return (
    <>
      {apps.map((a, i) => (
        <span key={`${a.slug}-${i}`}>
          {i > 0 && ", "}
          <AppName a={a} />
        </span>
      ))}
    </>
  )
}

export function OtherApps({ uses, usedBy }: { uses: AppLink[]; usedBy: AppLink[] }) {
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      {uses.length ? <p>Usa: <AppNames apps={uses} /></p> : <p className="text-muted-foreground">Não usa outros apps do catálogo.</p>}
      {usedBy.length > 0 && (
        <p>
          É usado por: <AppNames apps={usedBy} />
          <span className="text-muted-foreground"> · se mudar este, confira {usedBy.length > 1 ? "esses" : "esse"}</span>
        </p>
      )}
    </div>
  )
}

export function BuildView({ build }: { build: Manifest["build"] }) {
  if (!build) return <p className="text-sm text-muted-foreground">Não informado. Se o app é um .exe gerado de código, diga onde está o código.</p>
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      {build.source && <p>Código em <code className="font-mono text-xs [overflow-wrap:anywhere]">{build.source}</code></p>}
      {build.command && <p>Para recompilar: <code className="font-mono text-xs [overflow-wrap:anywhere]">{build.command}</code></p>}
    </div>
  )
}

export function RunList({ runs, everyone }: { runs: RunSummary[] | null; everyone: boolean }) {
  if (runs === null) return <p className="text-sm text-muted-foreground">Carregando…</p>
  if (runs.length === 0) return <p className="text-sm text-muted-foreground">{everyone ? "Ninguém rodou ainda." : "Você ainda não rodou este app."}</p>
  return (
    <ul className="flex flex-col text-sm">
      {runs.map((r) => (
        <li key={r.id} className="border-b border-border last:border-b-0">
          <Link to={`/execucoes/${r.id}`} className="group flex flex-wrap items-center gap-x-4 gap-y-1 py-2 hover:text-foreground">
            <span className="w-28 shrink-0 tabular-nums">{formatWhen(r.created_at)}</span>
            {everyone && <span className="min-w-24">{r.person_name}</span>}
            <RunState status={r.status} />
            <span className="text-muted-foreground">{r.file_count === 1 ? "1 arquivo" : `${r.file_count} arquivos`}</span>
            <ChevronRight className="ml-auto size-4 text-muted-foreground group-hover:text-foreground" />
          </Link>
        </li>
      ))}
    </ul>
  )
}
