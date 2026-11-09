import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, Download, FileText, Play, Square, X } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { ByClaude, LoadError, RunState, Scheduled } from "@/components/bits"
import { Elapsed } from "@/components/Elapsed"
import { api, fileUrl, groupZipUrl, selectedUrl, type Run, type RunGroup, type RunInput } from "@/lib/api"
import { useRuns } from "@/lib/runs-context"
import { formatDuration, formatSize, formatWhen, isLive } from "@/lib/runs"
import { cn } from "@/lib/utils"

const SHOWN = 10

const CHECK = "size-4 shrink-0 cursor-pointer accent-[var(--foreground)]"

function Group({ run, g, selected, toggle }: { run: Run; g: RunGroup; selected: Set<number>; toggle: (ids: number[], on: boolean) => void }) {
  const [all, setAll] = useState(false)
  const files = all ? g.files : g.files.slice(0, SHOWN)
  return (
    <section className="border-t border-border py-5">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="flex items-center gap-3">
          {g.files.length > 0 && (
            <input
              type="checkbox"
              aria-label={`Marcar todos: ${g.label}`}
              className={CHECK}
              checked={g.files.every((f) => selected.has(f.id))}
              onChange={(e) => toggle(g.files.map((f) => f.id), e.target.checked)}
            />
          )}
          <h2 className="text-sm font-semibold">{g.label} <span className="font-normal text-muted-foreground">({g.files.length})</span></h2>
        </div>
        {g.files.length > 0 && (
          <a href={groupZipUrl(run.id, g.index)} className={cn(buttonVariants({ variant: "outline", size: "sm" }), "gap-2")}>
            <Download className="size-4" /> Baixar tudo
          </a>
        )}
      </div>
      {g.files.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nada desta vez.</p>
      ) : (
        <ul className="flex flex-col gap-2 text-sm">
          {files.map((f) => (
            <li key={f.id} className="flex items-baseline gap-3">
              <input
                type="checkbox"
                aria-label={f.path.split("/").pop()}
                className={cn(CHECK, "translate-y-0.5")}
                checked={selected.has(f.id)}
                onChange={(e) => toggle([f.id], e.target.checked)}
              />
              <a href={fileUrl(run.id, f.id)} className="group min-w-0 flex-1">
                <span className="block truncate font-mono text-xs underline-offset-4 group-hover:underline">{f.path.split("/").pop()}</span>
                {f.path.includes("/") && (
                  <span className="block truncate text-[0.7rem] text-muted-foreground">{f.path.split("/").slice(0, -1).join(" / ")}</span>
                )}
              </a>
              <span className="shrink-0 text-xs text-muted-foreground">{f.change === "changed" ? "alterado · " : ""}{formatSize(f.size)}</span>
            </li>
          ))}
          {!all && g.files.length > SHOWN && (
            <li>
              <button type="button" onClick={() => setAll(true)} className="text-sm font-medium underline underline-offset-4">
                … mais {g.files.length - SHOWN}
              </button>
            </li>
          )}
        </ul>
      )}
    </section>
  )
}

/** A date input comes as 2026-11-09; people read 09/11/2026. */
function inputValue(i: RunInput): string {
  if (i.type === "date") return i.values.map((v) => (/^\d{4}-\d{2}-\d{2}$/.test(v) ? v.split("-").reverse().join("/") : v)).join(", ")
  return i.values.join(", ")
}

/** O que foi preenchido: the values this run used, in the order of the ficha. */
function Filled({ run }: { run: Run }) {
  const inputs = run.inputs ?? []
  return (
    <section className="mt-8" aria-labelledby="preenchido">
      <h2 id="preenchido" className="mb-3 text-sm font-semibold">O que foi preenchido</h2>
      {run.inputs_recorded === false ? (
        <p className="text-sm text-muted-foreground">Esta execução é de antes de o Ziv guardar o que foi preenchido.</p>
      ) : inputs.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhum valor preenchido.</p>
      ) : (
        <dl className="divide-y divide-border overflow-hidden rounded-xl border border-border text-sm">
          {inputs.map((i) => (
            <div key={i.id} className="grid gap-x-6 gap-y-0.5 px-4 py-2.5 sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)]">
              <dt className="text-muted-foreground">{i.label}</dt>
              <dd className="min-w-0 [overflow-wrap:anywhere]">
                {i.masked ? (
                  <span className="text-muted-foreground"><span aria-hidden className="font-mono">••••••</span> escondido</span>
                ) : i.file ? (
                  <span className="inline-flex items-center gap-1.5"><FileText className="size-4 shrink-0 text-muted-foreground" aria-hidden /><span className="font-mono text-xs">{i.file}</span></span>
                ) : (
                  inputValue(i)
                )}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </section>
  )
}

/** Above this the log is only downloaded: bringing it from the computer to show would be slow. */
const CONSOLE_SHOW_MAX = 1024 * 1024
/** How much of the end of the log the page shows. */
const CONSOLE_TAIL = 20_000

/** Registro do app: what a script printed, brought from the computer where it ran. */
function ConsoleLog({ run, file }: { run: Run; file: { file_id: number; size: number } }) {
  const [text, setText] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [tries, setTries] = useState(0)
  const big = file.size > CONSOLE_SHOW_MAX
  useEffect(() => {
    if (big) return
    let gone = false
    api.runFileText(run.id, file.file_id).then(
      (t) => !gone && setText(t),
      (e: Error) => !gone && setError(e.message),
    )
    return () => { gone = true }
  }, [run.id, file.file_id, big, tries])
  const cut = text !== null && text.length > CONSOLE_TAIL
  return (
    <section className="mt-8" aria-labelledby="registro">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h2 id="registro" className="text-sm font-semibold">Registro do app <span className="font-normal text-muted-foreground">(o que ele escreveu enquanto rodava)</span></h2>
        <a href={fileUrl(run.id, file.file_id)} className={cn(buttonVariants({ variant: "outline", size: "sm" }), "gap-2")}>
          <Download className="size-4" /> Baixar o registro
        </a>
      </div>
      {big ? (
        <p className="text-sm text-muted-foreground">O registro tem {formatSize(file.size)}: grande demais para mostrar aqui. Baixe para ver.</p>
      ) : error ? (
        <p className="flex flex-wrap items-center gap-x-3 text-sm text-muted-foreground">
          {error}
          <button type="button" onClick={() => { setError(""); setTries((n) => n + 1) }} className="font-medium text-foreground underline underline-offset-4">Tentar de novo</button>
        </p>
      ) : text === null ? (
        <p className="text-sm text-muted-foreground">Trazendo o registro do computador {run.computer_name}…</p>
      ) : text.trim() === "" ? (
        <p className="text-sm text-muted-foreground">O app não escreveu nada.</p>
      ) : (
        <>
          {cut && <p className="mb-2 text-xs text-muted-foreground">Mostrando só o final. Baixe o registro para ver tudo.</p>}
          <pre aria-label="Registro do app" className="max-h-96 overflow-auto rounded-xl border border-border bg-muted/40 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">
            {cut ? text.slice(-CONSOLE_TAIL) : text}
          </pre>
        </>
      )}
    </section>
  )
}

export function RunPage() {
  const { id } = useParams()
  const [run, setRun] = useState<Run | null>(null)
  const [error, setError] = useState("")
  const [tries, setTries] = useState(0)
  const [, setNow] = useState(() => Date.now())
  const [selected, setSelected] = useState<Set<number>>(() => new Set())
  const { run: runApp, busy, askStop, stopping } = useRuns()
  const toggle = (ids: number[], on: boolean) =>
    setSelected((cur) => {
      const next = new Set(cur)
      for (const i of ids) {
        if (on) next.add(i)
        else next.delete(i)
      }
      return next
    })

  useEffect(() => {
    if (!id) return
    let stop = false
    let live = true
    const load = () =>
      api.run(id).then(
        (r) => {
          if (stop) return
          live = isLive(r.status)
          setRun(r)
        },
        (e: Error) => !stop && setError(e.message),
      )
    load()
    const t = setInterval(() => {
      setNow(Date.now())
      if (live) load()
    }, 2000)
    return () => {
      stop = true
      clearInterval(t)
    }
  }, [id, tries])

  if (error) return <LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} />
  if (!run) return <p className="text-muted-foreground">Carregando…</p>

  return (
    <article>
      <Link to="/execucoes" className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Minhas execuções
      </Link>
      <header className="flex flex-col gap-4 pb-6 md:flex-row md:items-end md:justify-between">
        <div className="min-w-0">
          <h1 className="font-heading text-2xl leading-tight md:text-3xl">
            <Link to={`/apps/${run.app_id}`} className="underline-offset-4 hover:underline">
              {run.app_name}
            </Link>
          </h1>
          <p className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
            <RunState status={run.status} />
            <span>· <span className="font-mono text-[0.8rem]">{formatWhen(run.created_at)}</span> · {isLive(run.status) && run.started_at ? (
              <span>rodando há <Elapsed from={run.started_at} className="text-[0.8rem] text-foreground" /></span>
            ) : (
              <span className="font-mono text-[0.8rem]">{formatDuration(run.started_at, run.finished_at)}</span>
            )}</span>
            <span className="whitespace-nowrap">· {run.person_name}</span>
            {run.origin === "mcp" && <ByClaude />}
            {run.origin === "schedule" && <Scheduled />}
          </p>
        </div>
        {isLive(run.status) && (stopping.has(run.id) ? (
          <span className="text-sm text-muted-foreground">Parando…</span>
        ) : run.can_stop && (
          <Button variant="destructive" className="h-9 shrink-0 gap-1.5 pr-4 pl-3.5" onClick={() => askStop({ id: run.id, app_name: run.app_name, status: run.status })}>
            <Square className="size-3.5" /> Parar
          </Button>
        ))}
        {!isLive(run.status) && (
          <Button
            variant="run"
            className="h-9 shrink-0 gap-1.5 pr-4 pl-3.5"
            disabled={busy.has(run.app_id)}
            onClick={() => runApp({ id: run.app_id, name: run.app_name, folder_display: run.output_folder, remembered_folder: "" }, { folder: run.output_folder })}
          >
            <Play className="size-3.5" /> Rodar de novo
          </Button>
        )}
      </header>
      <div>
        {run.status === "running" && <p className="mt-2 text-muted-foreground">Rodando: a janela está aberta no seu computador. Quando você fechar, os arquivos aparecem aqui.</p>}
        {run.status === "waiting" && <p className="mt-2 text-muted-foreground">Esperando o computador {run.computer_name} abrir o app…</p>}
        {run.status === "stopped" && (
          <p className="mt-2 max-w-prose text-muted-foreground">
            Você parou o app pelo Ziv. {run.groups.some((g) => g.files.length > 0) ? "Abaixo estão os arquivos que ele criou ou mudou até ali." : "Ele não tinha criado nem mudado arquivos na pasta abaixo."}
          </p>
        )}
        {run.status === "no_files" && (
          <p className="mt-2 max-w-prose text-muted-foreground">
            O app fechou sem criar nem mudar arquivos na pasta abaixo. Se você esperava algum, confira se o app salvou nesta pasta: o Ziv só vê o que fica nela.
          </p>
        )}
        {(run.status === "error" || run.status === "interrupted") && run.error_message && (
          <p role="alert" className="mt-2 text-destructive">{run.error_message}</p>
        )}
        <dl className="mt-5 grid overflow-hidden rounded-xl border border-border text-sm sm:grid-cols-[2fr_1fr_1fr]">
          <div className="px-4 py-3 max-sm:border-b sm:border-r"><dt className="text-xs text-muted-foreground">Salva em</dt><dd className="mt-0.5 font-mono text-xs break-all">{run.output_folder}</dd></div>
          <div className="px-4 py-3 max-sm:border-b sm:border-r"><dt className="text-xs text-muted-foreground">Computador</dt><dd className="mt-0.5 font-medium">{run.computer_name}</dd></div>
          <div className="px-4 py-3"><dt className="text-xs text-muted-foreground">Versão do app</dt><dd className="mt-0.5 font-mono text-[0.8rem]">{run.version_number ? `versão ${run.version_number}` : run.app_version || "—"}</dd></div>
        </dl>
        <Filled run={run} />
        {!isLive(run.status) && run.groups.length > 0 && (
          <div className="mt-6">
            <p className="mb-2 text-sm text-muted-foreground">Clique num arquivo para baixar só ele, ou marque os que quiser e baixe de uma vez.</p>
            {run.groups.map((g) => (
              <Group key={`${g.index}-${g.label}`} run={run} g={g} selected={selected} toggle={toggle} />
            ))}
          </div>
        )}
        {!isLive(run.status) && run.console && <ConsoleLog run={run} file={run.console} />}
        {selected.size > 0 && (
          <div className="sticky bottom-4 z-10 mt-4 flex flex-wrap items-center justify-end gap-3 shadow-pop rounded-xl border border-border bg-popover p-3">
            <button type="button" onClick={() => setSelected(new Set())} className="inline-flex cursor-pointer items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
              <X className="size-4" /> Limpar
            </button>
            <a href={selectedUrl(run.id, [...selected].sort((a, b) => a - b))} className={cn(buttonVariants(), "gap-2")}>
              <Download className="size-4" /> Baixar selecionados ({selected.size})
            </a>
          </div>
        )}
      </div>
    </article>
  )
}
