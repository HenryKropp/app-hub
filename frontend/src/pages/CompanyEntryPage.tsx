import { useState, type FormEvent } from "react"
import { ArrowLeft, ArrowRight } from "lucide-react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { AuthLayout, authBack, authError, authLink } from "@/components/AuthLayout"
import { api } from "@/lib/api"
import { landingAfterLogin } from "@/lib/landing"

type Company = { slug: string; name: string; url: string }

export function CompanyEntryPage() {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [companies, setCompanies] = useState<Company[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  const signInTo = async (company: Company) => {
    setBusy(true)
    setError("")
    try {
      const basePath = `/w/${company.slug}`
      const person = await api.loginAt(basePath, { email: email.trim(), password })
      window.location.assign(await landingAfterLogin(basePath, person))
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Não foi possível entrar. Tente de novo.")
      setBusy(false)
    }
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setError("")
    setBusy(true)
    try {
      const found = (await api.findLogins({ email: email.trim(), password })).companies
      if (found.length === 1) {
        await signInTo(found[0])
        return
      }
      if (found.length === 0) {
        setError("E-mail ou senha não conferem.")
      } else {
        setCompanies(found)
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Não foi possível entrar. Tente de novo.")
    }
    setBusy(false)
  }

  return (
    <AuthLayout
      back={<Link className={authBack} to="/boas-vindas"><ArrowLeft size={15} /> Início</Link>}
      logo={<Link to="/boas-vindas" aria-label="Ziv, início" className="rounded-md"><Wordmark className="h-10 w-auto" /></Link>}
      below={<p>Precisa de acesso? Peça um convite à sua empresa.</p>}
    >
      <h1 className="font-heading text-2xl leading-tight text-balance">Entre no espaço da sua equipe</h1>
      {companies ? (
        <div className="mt-6 flex flex-col gap-3">
          <p className="text-sm font-medium">Em qual empresa?</p>
          {companies.map((company) => (
            <Button key={company.slug} type="button" variant="outline" size="lg" className="w-full justify-between" disabled={busy} onClick={() => signInTo(company)}>
              {company.name} <ArrowRight data-icon="inline-end" />
            </Button>
          ))}
          {error && <p role="alert" className={authError}>{error}</p>}
          <button type="button" className={`${authLink} self-start text-sm`} onClick={() => { setCompanies(null); setError("") }}>Usar outro e-mail</button>
        </div>
      ) : (
        <form onSubmit={submit} className="mt-6 flex flex-col gap-4">
          <Field label="E-mail" htmlFor="login-email">
            <Input id="login-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" required className="h-9" />
          </Field>
          <Field label="Senha" htmlFor="login-password">
            <Input id="login-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required className="h-9" />
          </Field>
          {error && <p role="alert" className={authError}>{error}</p>}
          <Button type="submit" size="lg" className="w-full" disabled={busy}>{busy ? "Entrando…" : "Entrar"} {!busy && <ArrowRight data-icon="inline-end" />}</Button>
        </form>
      )}
      <div className="mt-6 border-t border-border pt-5 text-sm text-muted-foreground">
        <p>Ainda não tem um espaço? <Link to="/cadastre-se" className={authLink}>Cadastre-se</Link></p>
      </div>
    </AuthLayout>
  )
}
