import { useEffect, useState } from "react"
import { ChevronRight } from "lucide-react"
import { LoadError, PageHeader } from "@/components/bits"
import { RequestCard } from "@/components/governance/RequestCard"
import { api, type ApprovalRequest } from "@/lib/api"

const box = "divide-y divide-border overflow-hidden rounded-xl border border-border bg-card"

/** Aprovações: what waits for me to decide, and what went live with a warning (ADR 046). */
export function ApprovalsPage() {
  const [list, setList] = useState<ApprovalRequest[] | null>(null)
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")
  const [tries, setTries] = useState(0)

  useEffect(() => {
    let gone = false
    api.approvals().then((l) => !gone && setList(l), (e: Error) => !gone && setError(e.message))
    return () => { gone = true }
  }, [tries])

  const changed = (msg: string) => {
    setNotice(msg)
    setTries((n) => n + 1)
  }
  const open = list?.filter((r) => r.status === "open") ?? []
  const autos = list?.filter((r) => r.status === "auto") ?? []

  return (
    <>
      <PageHeader title="Aprovações" />
      <p className="mt-1 text-sm text-muted-foreground">Apps e artifacts que só vão ao ar depois que você aprovar.</p>
      {notice && <p role="status" className="mt-4 rounded-lg border border-border px-4 py-2.5 text-sm">{notice}</p>}
      {error && <div className="mt-6"><LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} /></div>}
      {list === null && !error && <p className="mt-6 text-sm text-muted-foreground">Carregando…</p>}
      {list && (
        <>
          <section className="mt-6">
            <h2 className="mb-2.5 font-heading text-base">
              Esperando você <span className="text-sm text-muted-foreground tabular-nums">{open.length}</span>
            </h2>
            {open.length === 0 ? (
              <p className="rounded-xl border border-dashed border-border px-4 py-5 text-sm text-muted-foreground">Nada esperando sua aprovação.</p>
            ) : (
              <ul className={box}>{open.map((r) => <RequestCard key={r.id} request={r} onChanged={changed} />)}</ul>
            )}
          </section>
          {autos.length > 0 && (
            <details className="group mt-8">
              <summary className="flex cursor-pointer list-none items-center gap-1.5 text-sm font-medium [&::-webkit-details-marker]:hidden">
                <ChevronRight className="size-4 transition-transform group-open:rotate-90" aria-hidden />
                Foram ao ar com aviso <span className="text-muted-foreground tabular-nums">{autos.length}</span>
                <span className="font-normal text-muted-foreground">· últimos 30 dias</span>
              </summary>
              <ul className={`${box} mt-2.5`}>{autos.map((r) => <RequestCard key={r.id} request={r} onChanged={changed} />)}</ul>
            </details>
          )}
        </>
      )}
    </>
  )
}
