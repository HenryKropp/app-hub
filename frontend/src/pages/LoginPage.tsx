import { useEffect, useState, type FormEvent } from "react"
import { Navigate, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Logo } from "@/components/bits"
import { api, OFFLINE } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { PRODUCT_NAME } from "@/lib/brand"

// sharedVolta keeps only a way back to a shared app's sign-in handoff
// (a server address, so it is opened with a full page load).
function sharedVolta(volta: string | null): string {
  if (!volta) return ""
  return volta === "/api/shared/authorize" || volta.startsWith("/api/shared/authorize?") ? volta : ""
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

  return (
    <div className="grid min-h-svh md:grid-cols-[1.1fr_1fr]">
      <section className="relative hidden overflow-hidden border-r border-sidebar-border bg-sidebar p-12 md:flex md:flex-col">
        <div className="flex items-center gap-2.5">
          <Logo className="size-8" />
          <span className="text-lg font-semibold tracking-[-0.01em]">{PRODUCT_NAME}</span>
        </div>
        <div className="mt-auto max-w-md">
          <h1 className="font-heading text-[2.5rem] leading-[1.1] text-balance">Todo app da empresa, com ficha, dono e botão de usar.</h1>
          <p className="mt-4 text-muted-foreground">
            Registre os scripts e ferramentas que estão espalhados em pastas. O Kova descobre como cada um funciona e deixa pronto para qualquer
            pessoa usar.
          </p>
        </div>
      </section>

      <section className="flex items-center justify-center px-4 py-12">
          <form id="login-form" onSubmit={submit} className="w-full max-w-sm">
          <div className="mb-6 flex items-center gap-2 md:hidden">
            <Logo className="size-8" />
            <span className="text-lg font-semibold">{PRODUCT_NAME}</span>
          </div>
          {statusError ? (
            <section className="mt-5 rounded-xl border border-warn/30 bg-warn-soft p-4" aria-labelledby="status-error">
              <h2 id="status-error" className="font-heading text-lg">Não consegui confirmar o estado do Kova</h2>
              <p role="alert" className="mt-2 text-sm">{statusError}</p>
              <p className="mt-2 text-sm text-muted-foreground">Confira o endereço da página e tente de novo. Não é possível saber se este Kova já foi configurado até ele responder.</p>
              <div className="mt-4 flex flex-wrap gap-2">
                <Button type="button" onClick={() => void refresh()}>Tentar novamente</Button>
                <a className="inline-flex h-9 items-center rounded-md border border-border px-3 text-sm font-medium hover:bg-accent" href={window.location.href} target="_blank" rel="noreferrer">Abrir esta página em outra aba</a>
              </div>
            </section>
          ) : <>
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
      </section>
    </div>
  )
}
