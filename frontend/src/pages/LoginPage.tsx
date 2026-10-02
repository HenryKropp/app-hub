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
      <section className="relative hidden overflow-hidden border-r border-sidebar-border bg-sidebar p-12 md:flex md:flex-col">
        <div className="flex items-center gap-2.5">
          <Logo className="size-8" />
          <span className="text-lg font-semibold tracking-[-0.01em]">App Hub</span>
        </div>
        <div className="mt-auto max-w-md">
          <h1 className="font-heading text-[2.5rem] leading-[1.1] text-balance">Todo app da empresa, com ficha, dono e botão de usar.</h1>
          <p className="mt-4 text-muted-foreground">
            Registre os scripts e ferramentas que estão espalhados em pastas. O Hub descobre como cada um funciona e deixa pronto para qualquer
            pessoa usar.
          </p>
        </div>
      </section>

      <section className="flex items-center justify-center px-4 py-12">
        <form onSubmit={submit} className="w-full max-w-sm">
          <div className="mb-6 flex items-center gap-2 md:hidden">
            <Logo className="size-8" />
            <span className="text-lg font-semibold">App Hub</span>
          </div>
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
