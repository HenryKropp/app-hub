import { useState, type FormEvent } from "react"
import { ArrowLeft, ArrowRight, Building2, ShieldCheck } from "lucide-react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { api, ApiError } from "@/lib/api"
import { slugFromEntry } from "@/lib/company-entry"

export function CompanyEntryPage() {
  const [companyInput, setCompanyInput] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  const enterCompany = async (event: FormEvent) => {
    event.preventDefault()
    setError("")
    const companySlug = slugFromEntry(companyInput, window.location.origin)
    if (!companySlug) {
      setError("Informe o identificador da empresa ou o endereço completo do espaço.")
      return
    }
    setBusy(true)
    try {
      const company = await api.entryCompanyBySlug(companySlug)
      if (company.slug !== companySlug || (company.base_path && company.base_path !== `/w/${companySlug}`)) {
        throw new Error("Não consegui confirmar o endereço deste espaço. Confira o identificador e tente novamente.")
      }
      const next = `${company.base_path || ""}/entrar`.replace(/\/+/g, "/")
      window.location.assign(next)
    } catch (reason) {
      setError(reason instanceof ApiError && reason.status === 404
        ? "Não encontrei uma empresa com esse endereço. Confira o identificador com quem administra o espaço."
        : reason instanceof Error ? reason.message : "Não foi possível confirmar essa empresa.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="auth-shell">
      <header className="auth-header"><Link to="/boas-vindas" aria-label="Ziv, início"><Wordmark className="h-9 w-auto" /></Link><Link className="auth-back" to="/boas-vindas"><ArrowLeft size={15} /> Início</Link></header>
      <div className="auth-content">
        <section className="auth-intro">
          <div className="auth-kicker"><span /> ACESSO DA EMPRESA</div>
          <h1>Entre no espaço<br />da sua equipe.</h1>
          <p>Use o identificador ou o endereço compartilhado pela sua empresa. Cada equipe tem um espaço próprio.</p>
          <div className="auth-assurance"><ShieldCheck size={17} /><span>Confirmamos o espaço antes de continuar.</span></div>
        </section>
        <section className="auth-card">
          <div className="auth-card-icon"><Building2 size={19} /></div>
          <h2>Onde sua equipe trabalha?</h2>
          <p className="auth-card-description">Informe o identificador que recebeu da administração da empresa.</p>
          <form onSubmit={enterCompany} className="auth-form">
            <Field label="Identificador ou endereço" htmlFor="company-address" hint="Ex.: horizonte ou ziv.exemplo.com/w/horizonte">
              <Input id="company-address" value={companyInput} onChange={(event) => setCompanyInput(event.target.value)} placeholder="nome-da-empresa" autoComplete="url" required />
            </Field>
            {error && <p role="alert" className="auth-error">{error}</p>}
            <Button type="submit" size="lg" className="auth-submit" disabled={busy}>{busy ? "Conferindo espaço…" : "Continuar"} {!busy && <ArrowRight size={17} />}</Button>
          </form>
          <div className="auth-card-footer"><span>Ainda não tem um espaço?</span><Link to="/cadastre-se">Cadastre-se <ArrowRight size={14} /></Link></div>
          <p className="mt-4 text-center text-xs text-muted-foreground">Usa o espaço principal deste portal? <a href="/entrar" className="font-medium underline underline-offset-4">Entrar no espaço principal</a></p>
        </section>
      </div>
      <footer className="auth-footer"><span>Ziv · ferramentas compartilhadas, acesso separado</span><span>Precisa de acesso? Peça um convite à sua empresa.</span></footer>
    </main>
  )
}
