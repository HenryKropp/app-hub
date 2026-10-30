import { useState, type FormEvent } from "react"
import { ArrowLeft, ArrowRight, Check, LockKeyhole } from "lucide-react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { api, ApiError } from "@/lib/api"
import { validCompanySlug } from "@/lib/company-entry"

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
  const [slug, setSlug] = useState("")
  const [ownerName, setOwnerName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  const createCompany = async (event: FormEvent) => {
    event.preventDefault()
    setError("")
    if (!validCompanySlug(slug)) {
      setError("O endereço deve ter de 3 a 40 caracteres, começar com uma letra minúscula e conter apenas letras, números ou hífens.")
      return
    }
    const byteLength = new TextEncoder().encode(password).length
    if (byteLength < 8 || byteLength > 72) {
      setError("Escolha uma senha com pelo menos 8 caracteres e que não seja muito longa.")
      return
    }
    setBusy(true)
    try {
      const result = await api.createEntryCompany({
        company_name: companyName.trim(),
        slug,
        name: ownerName.trim(),
        email: email.trim(),
        password,
      })
      const path = localPath(result.url)
      if (!path) throw new Error("O endereço retornado pela empresa não é válido.")
      const basePath = path.replace(/\/$/, "")
      window.location.assign(basePath ? `${basePath}/entrar` : "/entrar")
    } catch (reason) {
      const status = reason instanceof ApiError ? reason.status : 0
      setError(status === 409
        ? "Esse identificador já está em uso. Escolha outro."
        : status === 400
          ? (reason instanceof Error ? reason.message : "Confira os dados informados.")
          : status === 503
            ? "Novas empresas estão temporariamente indisponíveis. Tente novamente mais tarde."
            : reason instanceof Error ? reason.message : "Não foi possível criar a empresa.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="auth-shell auth-signup">
      <header className="auth-header"><Link to="/boas-vindas" aria-label="Ziv, início"><Wordmark className="h-9 w-auto" /></Link><Link className="auth-back" to="/acesso"><ArrowLeft size={15} /> Já tenho acesso</Link></header>
      <div className="signup-content">
        <section className="signup-heading">
          <div className="auth-kicker"><span /> SEU ESPAÇO COMEÇA AQUI</div>
          <h1>Abra uma casa<br />para as ferramentas<br /><em>da sua equipe.</em></h1>
          <p>Crie o espaço da empresa e defina quem vai administrá-lo. Depois, entre para organizar as ferramentas do time.</p>
          <div className="signup-points"><span><Check size={14} /> Um espaço próprio para sua empresa</span><span><Check size={14} /> Você começa como administrador</span><span><Check size={14} /> Sem entrada automática após criar</span></div>
        </section>
        <section className="auth-card signup-card">
          <div className="auth-card-icon"><LockKeyhole size={18} /></div>
          <h2>Cadastre sua empresa</h2>
          <p className="auth-card-description">Defina o endereço do espaço e os dados de quem vai administrá-lo.</p>
          <form onSubmit={createCompany} className="auth-form signup-form">
            <Field label="Nome da empresa" htmlFor="company-name"><Input id="company-name" value={companyName} onChange={(event) => setCompanyName(event.target.value)} placeholder="Ex.: Horizonte Logística" autoComplete="organization" required /></Field>
            <Field label="Endereço da empresa" htmlFor="company-slug" hint="3–40 caracteres; letras minúsculas, números e hífens.">
              <div className="slug-field"><span>ziv /</span><Input id="company-slug" className="slug-input" value={slug} onChange={(event) => setSlug(event.target.value)} placeholder="horizonte" minLength={3} maxLength={40} pattern="[a-z][a-z0-9-]{2,39}" autoCapitalize="none" spellCheck={false} required /></div>
            </Field>
            <div className="signup-divider"><span>ADMINISTRADOR DO ESPAÇO</span></div>
            <Field label="Seu nome" htmlFor="owner-name"><Input id="owner-name" value={ownerName} onChange={(event) => setOwnerName(event.target.value)} autoComplete="name" required /></Field>
            <Field label="E-mail de trabalho" htmlFor="owner-email"><Input id="owner-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" required /></Field>
            <Field label="Senha" htmlFor="owner-password" hint="Pelo menos 8 caracteres.">
              <Input id="owner-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" minLength={8} required />
            </Field>
            {error && <p role="alert" className="auth-error">{error}</p>}
            <Button type="submit" size="lg" className="auth-submit" disabled={busy}>{busy ? "Criando espaço…" : "Criar espaço da empresa"} {!busy && <ArrowRight size={17} />}</Button>
          </form>
          <p className="signup-disclaimer">Após a criação, você seguirá para a tela de login da empresa.</p>
        </section>
      </div>
      <footer className="auth-footer"><span>Ziv · um lugar compartilhado para as ferramentas da equipe</span><Link to="/acesso">Já tem um espaço? Login</Link></footer>
    </main>
  )
}
