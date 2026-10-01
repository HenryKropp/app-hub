import { useState, type ReactNode } from "react"
import { Link, useNavigate } from "react-router-dom"
import { Check, LoaderCircle, TriangleAlert, X } from "lucide-react"
import type { ActiveRun } from "@/lib/api"
import { useRuns } from "@/lib/runs-context"
import { isLive } from "@/lib/runs"
import { cn } from "@/lib/utils"

const MAX = 3

function text(r: ActiveRun): ReactNode {
  const n = r.file_count
  const app = <b className="font-semibold">{r.app_name}</b>
  switch (r.status) {
    case "waiting":
      return <>Abrindo {app} no seu computador…</>
    case "running":
      return <>{app} aberto no seu computador. Use a janela e feche quando terminar.</>
    case "done":
      return <>{app} terminou: {n} {n === 1 ? "arquivo novo" : "arquivos novos"}</>
    case "no_files":
      return <>{app} terminou sem arquivos novos</>
    case "interrupted":
      return <>{app} foi interrompido: o computador parou de responder</>
    default:
      return <>Não deu para abrir o {app}: {r.error_message}</>
  }
}

const ACTION = "rounded-md bg-ink-foreground/12 px-2.5 py-1 text-xs font-medium text-ink-foreground transition-colors hover:bg-ink-foreground/20"

function Card({ tone, children, actions }: { tone: "live" | "ok" | "bad"; children: ReactNode; actions: ReactNode }) {
  return (
    <div role="status" className="pointer-events-auto flex flex-wrap items-center justify-between gap-x-4 gap-y-2 rounded-lg bg-ink px-4 py-3 text-sm text-ink-foreground shadow-lg ring-1 ring-ink-foreground/10">
      <span className="flex min-w-0 items-start gap-2.5">
        {tone === "live" && <LoaderCircle className="mt-0.5 size-4 shrink-0 animate-spin text-ink-foreground/70" />}
        {tone === "ok" && <Check className="mt-0.5 size-4 shrink-0 text-ok" />}
        {tone === "bad" && <TriangleAlert className="mt-0.5 size-4 shrink-0 text-stamp" />}
        <span className="min-w-0 break-words">{children}</span>
      </span>
      <span className="ml-auto flex shrink-0 items-center gap-1.5">{actions}</span>
    </div>
  )
}

function Dismiss({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" aria-label="Dispensar" onClick={onClick} className="rounded p-1 text-ink-foreground/60 hover:bg-ink-foreground/10 hover:text-ink-foreground">
      <X className="size-4" />
    </button>
  )
}

// RunStrip is the bottom strip: one card per run not dismissed yet, plus the
// "Abrindo…" and error cards of requests this tab sent.
export function RunStrip() {
  const { active, notices, dismiss, dropNotice, run } = useRuns()
  const [hidden, setHidden] = useState<Set<number>>(new Set())
  const navigate = useNavigate()
  const open = (id: number) => {
    dismiss(id)
    navigate(`/execucoes/${id}`)
  }

  const items: ReactNode[] = [
    ...notices.map((n) =>
      n.kind === "starting" ? (
        <Card key={n.key} tone="live" actions={null}>
          Abrindo <b className="font-semibold">{n.app_name}</b> no seu computador…
        </Card>
      ) : (
        <Card
          key={n.key}
          tone="bad"
          actions={
            <>
              {n.offline ? (
                <button type="button" className={ACTION} onClick={() => { dropNotice(n.key); navigate("/meu-computador") }}>
                  Ver Meu computador
                </button>
              ) : (
                n.target && (
                  <button type="button" className={ACTION} onClick={() => { dropNotice(n.key); run(n.target!, { ask: true }) }}>
                    trocar pasta
                  </button>
                )
              )}
              <Dismiss onClick={() => dropNotice(n.key)} />
            </>
          }
        >
          Não deu para abrir o <b className="font-semibold">{n.app_name}</b>: {n.message}
        </Card>
      ),
    ),
    ...active
      .filter((r) => !(isLive(r.status) && hidden.has(r.id)))
      .map((r) => {
        const live = isLive(r.status)
        return (
          <Card
            key={r.id}
            tone={live ? "live" : r.status === "done" ? "ok" : "bad"}
            actions={
              live ? (
                <button type="button" className={ACTION} onClick={() => setHidden((h) => new Set(h).add(r.id))}>
                  ocultar
                </button>
              ) : (
                <>
                  {r.status === "error" && (
                    <button type="button" className={ACTION}
                      onClick={() => { dismiss(r.id); run({ id: r.app_id, name: r.app_name, folder_display: r.app_folder_display, remembered_folder: "" }, { ask: true }) }}>
                      trocar pasta
                    </button>
                  )}
                  <button type="button" className={cn(ACTION, r.status === "done" && "bg-ok/90 text-white hover:bg-ok")} onClick={() => open(r.id)}>
                    {r.status === "done" ? "Ver e baixar" : "Ver detalhes"}
                  </button>
                  <Dismiss onClick={() => dismiss(r.id)} />
                </>
              )
            }
          >
            {text(r)}
          </Card>
        )
      }),
  ]
  if (items.length === 0) return null
  const extra = items.length - MAX
  return (
    <div className="pointer-events-none fixed inset-x-4 bottom-4 z-40 mx-auto flex max-w-3xl flex-col gap-2 md:left-[calc(16rem+3rem)] md:right-12">
      {items.slice(0, MAX)}
      {extra > 0 && (
        <Link to="/execucoes" className="pointer-events-auto self-end rounded-md bg-ink px-3 py-1.5 text-xs text-ink-foreground shadow-lg underline-offset-4 hover:underline">
          +{extra} em Minhas execuções
        </Link>
      )}
    </div>
  )
}
