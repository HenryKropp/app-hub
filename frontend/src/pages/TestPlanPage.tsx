import { useEffect, useState } from "react"
import { ArrowLeft } from "lucide-react"
import { Link, useParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Field, LoadError, NativeSelect, PageHeader } from "@/components/bits"
import { api, type TestPlan } from "@/lib/api"

const ddmm = (iso: string) => {
  const d = new Date(iso)
  return `${String(d.getDate()).padStart(2, "0")}/${String(d.getMonth() + 1).padStart(2, "0")}`
}

/** Plano de teste (ADR 044): what the automatic test of this app touches, confirmed once by the owner or an admin. */
export function TestPlanPage() {
  const { id = "" } = useParams()
  const [plan, setPlan] = useState<TestPlan | null>(null)
  const [error, setError] = useState("")
  const [actionError, setActionError] = useState("")
  const [tries, setTries] = useState(0)
  const [pc, setPc] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let alive = true
    api.testPlan.get(id).then(
      (p) => {
        if (!alive) return
        setPlan(p)
        const chosen = p.confirmed?.computer_id ?? p.computers.find((c) => c.connected)?.id ?? p.computers[0]?.id
        setPc(chosen ? String(chosen) : "")
      },
      (e: Error) => alive && setError(e.message),
    )
    return () => { alive = false }
  }, [id, tries])

  const act = (call: Promise<TestPlan>) => {
    setBusy(true)
    setActionError("")
    call.then(setPlan, (e: Error) => setActionError(e.message)).finally(() => setBusy(false))
  }

  const back = (
    <Link to={`/apps/${id}`} className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
      <ArrowLeft className="size-4" /> Voltar ao app
    </Link>
  )
  if (error) return <>{back}<LoadError message={error} onRetry={() => { setError(""); setTries((n) => n + 1) }} /></>
  if (!plan) return <>{back}<p className="text-sm text-muted-foreground">Carregando…</p></>

  const confirmed = plan.confirmed && plan.matches ? plan.confirmed : null
  const p = plan.plan
  return (
    <article className="max-w-2xl">
      {back}
      <PageHeader title="Plano de teste" />
      {!p ? (
        <p className="text-sm text-muted-foreground">Este app ainda não tem plano de teste. Ele chega com a próxima versão que o Claude enviar.</p>
      ) : (
        <div className="space-y-6">
          <dl className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card text-sm">
            <div className="px-4 py-3">
              <dt className="text-muted-foreground">O teste acessa</dt>
              <dd className="mt-1">
                <ul className="list-disc space-y-0.5 pl-5">
                  {p.acessa.map((a) => <li key={a}>{a}</li>)}
                </ul>
              </dd>
            </div>
            <div className="px-4 py-3">
              <dt className="text-muted-foreground">O teste pula</dt>
              <dd className="mt-1">{p.pula}</dd>
            </div>
            <div className="px-4 py-3">
              <dt className="text-muted-foreground">Chaves usadas</dt>
              <dd className="mt-1 flex flex-wrap gap-1.5">
                {p.keys.length === 0 ? <span>nenhuma</span> : p.keys.map((k) => (
                  <code key={k} className="rounded bg-secondary px-1.5 py-0.5 font-mono text-xs">{k}</code>
                ))}
              </dd>
            </div>
          </dl>

          {confirmed && (
            <p className="text-sm">Plano confirmado por {confirmed.by || "alguém da equipe"}{confirmed.via === "claude_code" ? " pelo Claude Code" : ""} em {ddmm(confirmed.at)}.</p>
          )}

          {!plan.can_confirm ? (
            <p className="text-sm text-muted-foreground">Só o dono do app ou um administrador confirma este plano.</p>
          ) : (
            <div className="space-y-5">
              {plan.computers.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Onde roda: no PC. Ligue o Agente num computador seu para escolher onde o teste roda.{" "}
                  <Link to="/comecar#computador" className="underline underline-offset-4">Ligar o Agente</Link>
                </p>
              ) : (
                <Field label="Onde roda: no PC" htmlFor="pc-teste" hint="O teste roda neste computador, com o Agente ligado.">
                  <NativeSelect id="pc-teste" value={pc} onChange={(e) => setPc(e.target.value)} className="max-w-sm">
                    {plan.computers.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}{c.connected ? "" : " (desligado)"}{c.outdated ? " (Agente desatualizado)" : ""}
                      </option>
                    ))}
                  </NativeSelect>
                </Field>
              )}
              {confirmed ? (
                <Button variant="outline" disabled={busy || !pc || Number(pc) === confirmed.computer_id} onClick={() => act(api.testPlan.setComputer(Number(id), Number(pc)))}>
                  Trocar o PC de teste
                </Button>
              ) : (
                <div className="space-y-4">
                  <p className="rounded-xl border border-foreground/80 bg-card px-4 py-3 text-base font-medium leading-relaxed">{plan.confirm_text}</p>
                  <Button disabled={busy || !pc} onClick={() => act(api.testPlan.confirm(Number(id), plan.fingerprint, Number(pc)))}>
                    Confirmar plano de teste
                  </Button>
                </div>
              )}
              {actionError && <p role="alert" className="text-sm text-destructive">{actionError}</p>}
            </div>
          )}
        </div>
      )}
    </article>
  )
}
