import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { ArrowLeft, RotateCcw } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { EmptyState, KindTab, LoadError, PageHeader } from "@/components/bits"
import { api, type ArchivedApp } from "@/lib/api"
import { formatWhen } from "@/lib/runs"

// daysLeftText says when the app goes away for good.
export function daysLeftText(days: number): string {
  if (days <= 0) return "Será apagado de vez nas próximas horas"
  if (days === 1) return "Será apagado de vez em 1 dia"
  return `Será apagado de vez em ${days} dias`
}

// ArchivedAppsPage is Apps › Arquivados (ADR 031): the archived apps the
// person could bring back, each with how long it still has.
export function ArchivedAppsPage() {
  const [apps, setApps] = useState<ArchivedApp[] | null>(null)
  const [error, setError] = useState("")
  const [tries, setTries] = useState(0)
  const [busy, setBusy] = useState<number | null>(null)
  // Brought back on this visit: the line stays, with a link to the app.
  const [back, setBack] = useState<Set<number>>(() => new Set())

  useEffect(() => {
    api.archivedApps().then(setApps, (e: Error) => setError(e.message))
  }, [tries])

  const restore = async (a: ArchivedApp) => {
    setBusy(a.id)
    try {
      await api.unarchiveApp(a.id)
      setBack((s) => new Set(s).add(a.id))
      toast.success(`${a.name} voltou para a lista de apps.`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível trazer o app de volta.")
    } finally {
      setBusy(null)
    }
  }

  return (
    <>
      <Link to="/apps" className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" /> Apps
      </Link>
      <PageHeader title="Apps arquivados" />
      <p className="-mt-2 mb-6 max-w-[68ch] text-sm text-muted-foreground">
        Apps arquivados não aparecem na lista e não rodam. Cada um fica aqui por 30 dias; depois o Ziv apaga de vez. A pasta do app no computador nunca é mexida.
      </p>

      {error && <LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} />}
      {apps === null && !error && <p className="text-muted-foreground">Carregando…</p>}
      {apps?.length === 0 && <EmptyState title="Nenhum app arquivado">Quando você arquivar um app, ele aparece aqui por 30 dias.</EmptyState>}

      {apps && apps.length > 0 && (
        <ul className="flex flex-col divide-y divide-border rounded-xl border border-border bg-card">
          {apps.map((a) => {
            const restored = back.has(a.id)
            return (
              <li key={a.id} className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between sm:px-5">
                <div className="min-w-0">
                  <p className="flex flex-wrap items-center gap-2 font-medium">
                    {restored ? <Link to={`/apps/${a.id}`} className="underline underline-offset-4">{a.name}</Link> : a.name}
                    <KindTab kind={a.kind} />
                  </p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {a.area || "Sem área"} · Responsável {a.owner_name}
                  </p>
                  {restored ? (
                    <p className="mt-1 text-sm">De volta na lista. Os agendamentos e a versão no servidor continuam desligados: ligue de novo se precisar.</p>
                  ) : (
                    <p className="mt-1 text-sm text-muted-foreground">
                      Arquivado{a.archived_by_name ? ` por ${a.archived_by_name}` : ""} em {formatWhen(a.archived_at)}. <span className="text-foreground">{daysLeftText(a.days_left)}.</span>
                    </p>
                  )}
                </div>
                {!restored && (
                  <Button variant="outline" size="sm" className="shrink-0 gap-1.5 self-start sm:self-center" disabled={busy === a.id} onClick={() => restore(a)}>
                    <RotateCcw className="size-3.5" /> {busy === a.id ? "Trazendo…" : "Trazer de volta"}
                  </Button>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </>
  )
}
