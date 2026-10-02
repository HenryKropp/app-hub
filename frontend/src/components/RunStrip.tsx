import { useState, type ReactNode } from "react"
import { Link, useNavigate } from "react-router-dom"
import { CircleCheck, Info, TriangleAlert, X } from "lucide-react"
import { Playing } from "./bits"
import type { ActiveRun } from "@/lib/api"
import { useRuns } from "@/lib/runs-context"
import { isLive } from "@/lib/runs"

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

// Folder problems (from the Hub or the Agente) mention "pasta"; only those get "trocar pasta".
const aboutFolder = (msg = "") => /pasta/i.test(msg)

const ACTION = "rounded-md border border-border bg-background px-2.5 py-1 text-xs font-medium text-foreground transition-colors hover:bg-accent"
const PRIMARY = "rounded-md bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/85"

function Card({ tone, children, actions }: { tone: "live" | "ok" | "muted" | "bad"; children: ReactNode; actions: ReactNode }) {
  return (
    <div role="status" className="shadow-pop pointer-events-auto flex flex-wrap items-center justify-between gap-x-4 gap-y-2 md:flex-nowrap rounded-xl border border-border bg-popover px-4 py-3 text-sm text-popover-foreground">
      <span className="flex min-w-0 items-start gap-2.5">
        <span className="mt-0.5 flex size-4 shrink-0 items-center justify-center">
          {tone === "live" && <Playing />}
          {tone === "ok" && <CircleCheck className="size-4 text-ok" />}
          {tone === "muted" && <Info className="size-4 text-muted-foreground" />}
          {tone === "bad" && <TriangleAlert className="size-4 text-destructive" />}
        </span>
        <span className="min-w-0 break-words">{children}</span>
      </span>
      <span className="ml-auto flex shrink-0 items-center gap-1.5">{actions}</span>
    </div>
  )
}

function Dismiss({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" aria-label="Dispensar" onClick={onClick} className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground">
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
                n.target && aboutFolder(n.message) && (
                  <button type="button" className={ACTION} onClick={() => { dropNotice(n.key); run(n.target!, { ask: true }) }}>
                    Trocar pasta
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
            tone={live ? "live" : r.status === "done" ? "ok" : r.status === "no_files" ? "muted" : "bad"}
            actions={
              live ? (
                <button type="button" className={ACTION} onClick={() => setHidden((h) => new Set(h).add(r.id))}>
                  Ocultar
                </button>
              ) : (
                <>
                  {r.status === "interrupted" && (
                    <button type="button" className={ACTION} onClick={() => { dismiss(r.id); navigate("/meu-computador") }}>
                      Ver Meu computador
                    </button>
                  )}
                  {r.status === "error" && aboutFolder(r.error_message) && (
                    <button type="button" className={ACTION}
                      onClick={() => { dismiss(r.id); run({ id: r.app_id, name: r.app_name, folder_display: r.app_folder_display, remembered_folder: "" }, { ask: true }) }}>
                      Trocar pasta
                    </button>
                  )}
                  <button type="button" className={r.status === "done" ? PRIMARY : ACTION} onClick={() => open(r.id)}>
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
    <div className="pointer-events-none fixed inset-x-4 bottom-4 z-40 mx-auto flex max-w-3xl flex-col gap-2 md:left-[calc(15rem+3rem)] md:right-12">
      {items.slice(0, MAX)}
      {extra > 0 && (
        <Link to="/execucoes" className="shadow-pop pointer-events-auto self-end rounded-md border border-border bg-popover px-3 py-1.5 text-xs font-medium underline-offset-4 hover:underline">
          +{extra} em Minhas execuções
        </Link>
      )}
    </div>
  )
}
