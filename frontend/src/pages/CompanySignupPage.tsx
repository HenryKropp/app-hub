import { useEffect, useState, type FormEvent } from "react"
import { ArrowLeft, ArrowRight, Eye, EyeOff } from "lucide-react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { api, ApiError } from "@/lib/api"

function localPath(url: string) {
  try {
    const parsed = new URL(url, window.location.origin)
    if (parsed.origin !== window.location.origin) return null
    return parsed.pathname
  } catch {
    return null
  }
}

export function CompanySignupPage() {
  const [companyName, setCompanyName] = useState("")
  const [ownerName, setOwnerName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [invalid, setInvalid] = useState<string | null>(null)
  // A new object each time, so asking for the same field twice still moves focus.
  const [focusField, setFocusField] = useState<{ id: string } | null>(null)

  useEffect(() => {
    if (focusField) document.getElementById(focusField.id)?.focus()
  }, [focusField])

  const fail = (id: string | null, message: string) => {
    setError(message)
    setInvalid(id)
    if (id) setFocusField({ id })
  }

  const createCompany = async (event: FormEvent) => {
    event.preventDefault()
    setError("")
    setInvalid(null)
    const missing = [
      ["company-name", companyName.trim()], ["owner-name", ownerName.trim()],
      ["owner-email", email.trim()], ["owner-password", password],
    ].find(([, value]) => !value)
    if (missing) return fail(missing[0], "Preencha todos os campos.")
    if (!/^[^\s@]+@[^\s@]+$/.test(email.trim())) return fail("owner-email", "Informe um e-mail válido.")
    const byteLength = new TextEncoder().encode(password).length
    if (byteLength < 8) return fail("owner-password", "A senha precisa ter pelo menos 8 caracteres.")
    if (byteLength > 72) return fail("owner-password", "A senha é longa demais. Use até 72 caracteres sem acento.")
    setBusy(true)
    try {
      const result = await api.createEntryCompany({
        company_name: companyName.trim(),
        name: ownerName.trim(),
        email: email.trim(),
        password,
      })
      const path = localPath(result.url)
      if (!path) throw new Error("O endereço retornado pela empresa não é válido.")
      const basePath = path.replace(/\/$/, "")
      // The company exists now; signing in is a convenience and may fail on its own.
      try {
        await api.loginAt(basePath, { email: email.trim(), password })
        window.location.assign(`${basePath}/comecar`)
      } catch {
        window.location.assign(`${basePath}/entrar`)
      }
    } catch (reason) {
      const status = reason instanceof ApiError ? reason.status : 0
      if (status === 409) fail("company-name", reason instanceof Error ? reason.message : "Use um nome de empresa mais específico.")
      else fail(null, status === 400
        ? (reason instanceof Error ? reason.message : "Confira os dados informados.")
        : status === 503
          ? "Novas empresas estão temporariamente indisponíveis. Tente novamente mais tarde."
          : reason instanceof Error ? reason.message : "Não foi possível criar a empresa.")
    } finally {
      setBusy(false)
    }
  }

  const isInvalid = (id: string) => invalid === id || undefined

  return (
    <main className="auth-shell auth-signup">
      <header className="auth-header"><Link to="/boas-vindas" aria-label="Ziv, início"><Wordmark className="h-9 w-auto" /></Link><Link className="auth-back" to="/acesso"><ArrowLeft size={15} /> Já tenho acesso</Link></header>
      <div className="signup-content signup-single">
        <section className="auth-card signup-card">
          <h1>Cadastre sua empresa</h1>
          <p className="auth-card-description">Você começa como administrador e já entra no espaço da empresa.</p>
          {/* noValidate: the page explains every problem in text that stays; the browser's bubble vanishes. */}
          <form onSubmit={createCompany} className="auth-form signup-form" noValidate>
            <Field label="Nome da empresa" htmlFor="company-name">
              <Input id="company-name" value={companyName} onChange={(event) => setCompanyName(event.target.value)} placeholder="Ex.: Horizonte Logística" autoComplete="organization" aria-invalid={isInvalid("company-name")} required />
            </Field>
            <div className="signup-divider"><span>ADMINISTRADOR DO ESPAÇO</span></div>
            <div className="signup-pair">
              <Field label="Seu nome" htmlFor="owner-name"><Input id="owner-name" value={ownerName} onChange={(event) => setOwnerName(event.target.value)} autoComplete="name" aria-invalid={isInvalid("owner-name")} required /></Field>
              <Field label="E-mail de trabalho" htmlFor="owner-email"><Input id="owner-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" aria-invalid={isInvalid("owner-email")} required /></Field>
            </div>
            <Field label="Senha" htmlFor="owner-password" hint="Pelo menos 8 caracteres.">
              <div className="password-field">
                <Input id="owner-password" type={showPassword ? "text" : "password"} value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" aria-invalid={isInvalid("owner-password")} required />
                <button type="button" className="password-toggle" onClick={() => setShowPassword((shown) => !shown)} aria-label={showPassword ? "Esconder senha" : "Mostrar senha"}>
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </Field>
            {error && <p role="alert" className="auth-error">{error}</p>}
            <Button type="submit" size="lg" className="auth-submit" disabled={busy}>{busy ? "Criando espaço…" : "Criar espaço da empresa"} {!busy && <ArrowRight size={17} />}</Button>
          </form>
        </section>
      </div>
      <footer className="auth-footer"><span>Ziv · um lugar compartilhado para as ferramentas da equipe</span><Link to="/acesso">Já tem um espaço? Login</Link></footer>
    </main>
  )
}
