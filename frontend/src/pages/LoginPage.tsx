import { useEffect, useState, type FormEvent } from "react"
import { Navigate, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
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
    <div className="grid min-h-svh md:grid-cols-[1.1fr_1fr]">
      <section className="relative hidden overflow-hidden border-r border-sidebar-border bg-sidebar p-12 md:flex md:flex-col">
        <div className="flex items-center gap-2.5">
          <Wordmark className="h-11 w-auto" />
        </div>
        <div className="mt-auto max-w-md">
          <h1 className="font-heading text-[2.5rem] leading-[1.1] text-balance">Todo app da empresa, com ficha, dono e botão de usar.</h1>
          <p className="mt-4 text-muted-foreground">
            Registre os scripts e ferramentas que estão espalhados em pastas. O Ziv descobre como cada um funciona e deixa pronto para qualquer
            pessoa usar.
          </p>
        </div>
      </section>

      <section className="flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm">
          <form id="login-form" onSubmit={submit}>
          <div className="mb-6 flex items-center gap-2 md:hidden">
            <Wordmark className="h-11 w-auto" />
          </div>
          {statusError ? (
            <section className="mt-5 rounded-xl border border-warn/30 bg-warn-soft p-4" aria-labelledby="status-error">
              <h2 id="status-error" className="font-heading text-lg">Não consegui confirmar o estado do Ziv</h2>
              <p role="alert" className="mt-2 text-sm">{statusError}</p>
              <p className="mt-2 text-sm text-muted-foreground">Confira o endereço da página e tente de novo. Não é possível saber se este Ziv já foi configurado até ele responder.</p>
              <div className="mt-4 flex flex-wrap gap-2">
                <Button type="button" onClick={() => void refresh()}>Tentar novamente</Button>
                <a className="inline-flex h-9 items-center rounded-md border border-border px-3 text-sm font-medium hover:bg-accent" href={window.location.href} target="_blank" rel="noreferrer">Abrir esta página em outra aba</a>
              </div>
            </section>
          ) : <>
          {workspaceName && <p className="mb-2 text-sm font-medium text-brand">{workspaceName}</p>}
          <h2 className="font-heading text-2xl">{needsSetup ? "Crie a conta de administrador" : "Bem-vindo de volta"}</h2>
          {needsSetup && <p className="mt-2 text-sm text-muted-foreground">Você será o administrador. Depois pode cadastrar as outras pessoas da empresa.</p>}

          <div className="mt-6 flex flex-col gap-4">
            {needsSetup && (
              <Field label="Seu nome" htmlFor="name">
                <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required autoComplete="name" />
              </Field>
            )}
            <Field label="E-mail" htmlFor="email">
              <Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" />
            </Field>
            <Field label="Senha" htmlFor="password" hint={needsSetup ? "Pelo menos 8 caracteres." : undefined}>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete={needsSetup ? "new-password" : "current-password"}
              />
            </Field>
            {error && (
              <div>
                <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
                {error === OFFLINE && <div className="mt-2 flex flex-wrap gap-3 text-xs">
                  <button type="button" className="font-medium underline underline-offset-4" onClick={() => (document.getElementById("login-form") as HTMLFormElement | null)?.requestSubmit()}>Tentar novamente</button>
                  <a href={window.location.href} target="_blank" rel="noreferrer" className="text-muted-foreground underline underline-offset-4">Abrir esta página em outra aba</a>
                </div>}
              </div>
            )}
            <Button type="submit" size="lg" disabled={busy} className="mt-2 h-10">
              {busy ? "Aguarde…" : needsSetup ? "Criar conta e entrar" : "Entrar"}
            </Button>
          </div>
          </>}
          </form>
          <div className="mt-6 flex items-center justify-between border-t border-border pt-4 text-sm">
            <a href="/boas-vindas" className="text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">Trocar de empresa</a>
            {canRequestAccess && <button type="button" onClick={() => { setRequestOpen((open) => !open); setRequestError(""); setRequestMessage("") }} className="font-medium text-logo-square underline-offset-4 hover:underline">
              {requestOpen ? "Fechar solicitação" : "Solicitar acesso"}
            </button>}
          </div>
          {canRequestAccess && requestOpen && (
            <form onSubmit={requestAccess} className="mt-4 rounded-xl border border-border bg-card p-4">
              <h3 className="font-heading text-base">Peça acesso a {workspaceName || "esta empresa"}</h3>
              <p className="mt-1 text-sm text-muted-foreground">A administração receberá seu nome e e-mail para decidir sobre o acesso.</p>
              <div className="mt-4 flex flex-col gap-3">
                <Field label="Seu nome" htmlFor="request-name">
                  <Input id="request-name" value={requestName} onChange={(event) => setRequestName(event.target.value)} autoComplete="name" required />
                </Field>
                <Field label="E-mail de trabalho" htmlFor="request-email">
                  <Input id="request-email" type="email" value={requestEmail} onChange={(event) => setRequestEmail(event.target.value)} autoComplete="email" required />
                </Field>
                {requestError && <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{requestError}</p>}
                {requestMessage && <p role="status" className="rounded-md bg-ok-soft px-3 py-2 text-sm text-ok">{requestMessage}</p>}
                <Button type="submit" variant="outline" disabled={requestBusy}>{requestBusy ? "Enviando…" : "Enviar solicitação"}</Button>
              </div>
            </form>
          )}
        </div>
      </section>
    </div>
  )
}
