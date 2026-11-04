import { useEffect, useState, type FormEvent } from "react"
import { ShieldCheck } from "lucide-react"
import { Navigate, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { Button, buttonVariants } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field } from "@/components/bits"
import { AuthLayout, authError, authLink } from "@/components/AuthLayout"
import { api, OFFLINE, workspaceUrl } from "@/lib/api"
import { useAuth } from "@/lib/auth"

// sharedVolta keeps only a way back to a shared app's sign-in handoff
// (a server address, so it is opened with a full page load).
function sharedVolta(volta: string | null): string {
  if (!volta) return ""
  const authorize = workspaceUrl("/api/shared/authorize")
  return volta === authorize || volta.startsWith(authorize + "?") ? volta : ""
}

export function LoginPage() {
  const { needsSetup, person, setPerson, statusError, refresh } = useAuth()
  const navigate = useNavigate()
  const from = (useLocation().state as { from?: string } | null)?.from ?? "/"
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [workspaceName, setWorkspaceName] = useState("")
  const [requestOpen, setRequestOpen] = useState(false)
  const [requestName, setRequestName] = useState("")
  const [requestEmail, setRequestEmail] = useState("")
  const [requestError, setRequestError] = useState("")
  const [requestMessage, setRequestMessage] = useState("")
  const [requestBusy, setRequestBusy] = useState(false)
  const canRequestAccess = !needsSetup && !statusError
  useEffect(() => {
    let active = true
    api.workspace().then((workspace) => { if (active) setWorkspaceName(workspace.name) }).catch(() => {})
    return () => { active = false }
  }, [])
  const [params] = useSearchParams()
  const toApp = sharedVolta(params.get("volta"))

  useEffect(() => {
    if (person && toApp) window.location.assign(toApp)
  }, [person, toApp])

  if (person) return toApp ? <p className="p-6 text-sm text-muted-foreground">Abrindo o app…</p> : <Navigate to={from} replace />

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setBusy(true)
    try {
      const p = needsSetup ? await api.setup({ name, email, password }) : await api.login({ email, password })
      if (toApp) {
        window.location.assign(toApp)
        return
      }
      setPerson(p)
      navigate(from, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : "Não foi possível entrar.")
    } finally {
      setBusy(false)
    }
  }

  const requestAccess = async (e: FormEvent) => {
    e.preventDefault()
    setRequestError("")
    setRequestMessage("")
    setRequestBusy(true)
    try {
      const result = await api.requestCompanyAccess({ name: requestName.trim(), email: requestEmail.trim() })
      setRequestMessage(result.message || "Sua solicitação foi registrada. A administração da empresa poderá analisá-la.")
      setRequestName("")
      setRequestEmail("")
    } catch (err) {
      setRequestError(err instanceof Error ? err.message : "Não foi possível enviar sua solicitação.")
    } finally {
      setRequestBusy(false)
    }
  }

  return (
    <AuthLayout
      below={!statusError && <p className="inline-flex items-center gap-1.5"><ShieldCheck className="size-4 shrink-0" /> Este acesso pertence somente a esta empresa.</p>}
    >
      <form id="login-form" onSubmit={submit}>
        {statusError ? (
          <section className="rounded-lg border border-warn/30 bg-warn-soft p-4" aria-labelledby="status-error">
            <h1 id="status-error" className="font-heading text-lg leading-snug">Não consegui confirmar o estado do Ziv</h1>
            <p role="alert" className="mt-2 text-sm">{statusError}</p>
            <p className="mt-2 text-sm text-muted-foreground">Confira o endereço da página e tente de novo. Não é possível saber se este Ziv já foi configurado até ele responder.</p>
            <div className="mt-4 flex flex-wrap gap-2">
              <Button type="button" onClick={() => void refresh()}>Tentar novamente</Button>
              <a className={buttonVariants({ variant: "outline" })} href={window.location.href} target="_blank" rel="noreferrer">Abrir esta página em outra aba</a>
            </div>
          </section>
        ) : <>
          <h1 className="font-heading text-2xl leading-tight">{needsSetup ? "Configure o primeiro acesso" : "Bom ter você de volta."}</h1>
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
            {needsSetup
              ? "Crie a conta de administrador para começar. Depois, convide as pessoas da empresa."
              : workspaceName ? <>Entre no espaço <span className="font-medium text-foreground">{workspaceName}</span>.</> : "Entre no espaço da sua empresa."}
          </p>

          <div className="mt-6 flex flex-col gap-4">
            {needsSetup && (
              <Field label="Seu nome" htmlFor="name">
                <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required autoComplete="name" className="h-9" />
              </Field>
            )}
            <Field label="E-mail" htmlFor="email">
              <Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" className="h-9" />
            </Field>
            <Field label="Senha" htmlFor="password" hint={needsSetup ? "Pelo menos 8 caracteres." : undefined}>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete={needsSetup ? "new-password" : "current-password"}
                className="h-9"
              />
            </Field>
            {error && (
              <div>
                <p role="alert" className={authError}>{error}</p>
                {error === OFFLINE && <div className="mt-2 flex flex-wrap gap-3 text-xs">
                  <button type="button" className={authLink} onClick={() => (document.getElementById("login-form") as HTMLFormElement | null)?.requestSubmit()}>Tentar novamente</button>
                  <a href={window.location.href} target="_blank" rel="noreferrer" className="text-muted-foreground underline underline-offset-4 hover:text-foreground">Abrir esta página em outra aba</a>
                </div>}
              </div>
            )}
            <Button type="submit" size="lg" disabled={busy} className="mt-1 w-full">
              {busy ? "Aguarde…" : needsSetup ? "Criar conta e entrar" : "Entrar"}
            </Button>
          </div>
        </>}
      </form>
      <div className="mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-border pt-5 text-sm">
        <a href="/acesso" className="text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline">Trocar de empresa</a>
        {canRequestAccess && <button type="button" aria-expanded={requestOpen} onClick={() => { setRequestOpen((open) => !open); setRequestError(""); setRequestMessage("") }} className={authLink}>
          {requestOpen ? "Fechar solicitação" : "Solicitar acesso"}
        </button>}
      </div>
      {canRequestAccess && requestOpen && (
        <form onSubmit={requestAccess} className="mt-5 border-t border-border pt-5">
          <h2 className="font-heading text-base">Peça acesso a {workspaceName || "esta empresa"}</h2>
          <p className="mt-1 text-sm leading-relaxed text-muted-foreground">A administração receberá seu nome e e-mail para decidir sobre o acesso.</p>
          <div className="mt-4 flex flex-col gap-3">
            <Field label="Seu nome" htmlFor="request-name">
              <Input id="request-name" value={requestName} onChange={(event) => setRequestName(event.target.value)} autoComplete="name" required className="h-9" />
            </Field>
            <Field label="E-mail de trabalho" htmlFor="request-email">
              <Input id="request-email" type="email" value={requestEmail} onChange={(event) => setRequestEmail(event.target.value)} autoComplete="email" required className="h-9" />
            </Field>
            {requestError && <p role="alert" className={authError}>{requestError}</p>}
            {requestMessage && <p role="status" className="rounded-lg bg-ok-soft px-3 py-2 text-sm text-ok">{requestMessage}</p>}
            <Button type="submit" variant="outline" size="lg" className="w-full" disabled={requestBusy}>{requestBusy ? "Enviando…" : "Enviar solicitação"}</Button>
          </div>
        </form>
      )}
    </AuthLayout>
  )
}
