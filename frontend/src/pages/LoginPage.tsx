import { useState, type FormEvent } from "react"
import { Navigate, useLocation, useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Logo } from "@/components/bits"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"

export function LoginPage() {
  const { needsSetup, person, setPerson } = useAuth()
  const navigate = useNavigate()
  const from = (useLocation().state as { from?: string } | null)?.from ?? "/"
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  if (person) return <Navigate to={from} replace />

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setBusy(true)
    try {
      const p = needsSetup ? await api.setup({ name, email, password }) : await api.login({ email, password })
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
      <section className="grain relative hidden overflow-hidden bg-ink p-12 text-ink-foreground md:flex md:flex-col">
        <div className="flex items-center gap-3">
          <Logo className="size-10" />
          <span className="font-heading text-2xl">App Hub</span>
        </div>
        <div className="relative mt-auto max-w-md">
          {/* Stacked index cards, each tab one kind of app. */}
          <div className="relative mb-12 h-44">
            {["var(--kind-reference)", "var(--kind-desktop)", "var(--kind-web)", "var(--kind-script)"].map((c, i) => (
              <div
                key={c}
                className="rise absolute inset-x-0 h-32 rounded-md border border-ink-foreground/10 bg-ink-foreground/[0.06] backdrop-blur-sm"
                style={{ top: i * 14, left: i * 18, right: -i * 6, animationDelay: `${i * 90}ms` }}
              >
                <span className="absolute -top-3 left-6 h-3 w-16 rounded-t-md" style={{ left: 24 + i * 58, background: c }} />
              </div>
            ))}
          </div>
          <h1 className="font-heading text-4xl leading-tight">Todo app da empresa, com ficha, dono e botão de usar.</h1>
          <p className="mt-4 text-ink-foreground/65">
            Registre os scripts e ferramentas que estão espalhados em pastas. O Hub descobre como cada um funciona e deixa pronto para qualquer
            pessoa usar.
          </p>
        </div>
      </section>

      <section className="paper flex items-center justify-center px-4 py-12">
        <form onSubmit={submit} className="rise w-full max-w-sm rounded-lg border border-border bg-card p-8 shadow-card">
          <div className="mb-6 flex items-center gap-2 md:hidden">
            <Logo className="size-8" />
            <span className="font-heading text-xl">App Hub</span>
          </div>
          <p className="font-mono text-xs tracking-[0.18em] text-stamp uppercase">{needsSetup ? "Primeiro acesso" : "Entrar"}</p>
          <h2 className="mt-2 font-heading text-2xl">{needsSetup ? "Crie a conta de administrador" : "Bem-vindo de volta"}</h2>
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
              <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error}
              </p>
            )}
            <Button type="submit" size="lg" disabled={busy} className="mt-2 h-10">
              {busy ? "Aguarde…" : needsSetup ? "Criar conta e entrar" : "Entrar"}
            </Button>
          </div>
        </form>
      </section>
    </div>
  )
}
