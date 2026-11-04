import { useState, type FormEvent } from "react"
import { ArrowLeft, ArrowRight } from "lucide-react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { AuthLayout, authBack, authError, authLink } from "@/components/AuthLayout"
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
      setError("Digite o endereço da empresa no Ziv. Se não souber, pergunte a quem administra o Ziv na sua empresa.")
      return
    }
    setBusy(true)
    try {
      const company = await api.entryCompanyBySlug(companySlug)
      if (company.slug !== companySlug || (company.base_path && company.base_path !== `/w/${companySlug}`)) {
        throw new Error("Não consegui confirmar o endereço deste espaço. Confira o endereço e tente de novo. Se não souber, pergunte a quem administra o Ziv na sua empresa.")
      }
      const next = `${company.base_path || ""}/entrar`.replace(/\/+/g, "/")
      window.location.assign(next)
    } catch (reason) {
      setError(reason instanceof ApiError && reason.status === 404
        ? "Não encontrei uma empresa com esse endereço. Confira se digitou certo; se não souber, pergunte a quem administra o Ziv na sua empresa."
        : reason instanceof Error ? reason.message : "Não foi possível confirmar essa empresa. Tente de novo; se continuar, pergunte a quem administra o Ziv na sua empresa.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthLayout
      back={<Link className={authBack} to="/boas-vindas"><ArrowLeft size={15} /> Início</Link>}
      logo={<Link to="/boas-vindas" aria-label="Ziv, início" className="rounded-md"><Wordmark className="h-10 w-auto" /></Link>}
      below={<p>Precisa de acesso? Peça um convite à sua empresa.</p>}
    >
      <h1 className="font-heading text-2xl leading-tight text-balance">Entre no espaço da sua equipe</h1>
      <p className="mt-2 text-sm leading-relaxed text-muted-foreground">Use o endereço que a sua empresa compartilhou. Cada equipe tem um espaço próprio.</p>
      <form onSubmit={enterCompany} className="mt-6 flex flex-col gap-4">
        <Field label="Endereço da empresa no Ziv" htmlFor="company-address" hint="É a última parte do link da sua empresa. Ex.: em ziv.exemplo.com/w/horizonte, digite horizonte.">
          <Input id="company-address" value={companyInput} onChange={(event) => setCompanyInput(event.target.value)} placeholder="nome-da-empresa" autoComplete="url" required className="h-9" />
        </Field>
        {error && <p role="alert" className={authError}>{error}</p>}
        <Button type="submit" size="lg" className="w-full" disabled={busy}>{busy ? "Conferindo espaço…" : "Continuar"} {!busy && <ArrowRight data-icon="inline-end" />}</Button>
      </form>
      <div className="mt-6 space-y-2 border-t border-border pt-5 text-sm text-muted-foreground">
        <p>Ainda não tem um espaço? <Link to="/cadastre-se" className={authLink}>Cadastre-se</Link></p>
        <p>Usa o espaço principal deste portal? <a href="/entrar" className={authLink}>Entrar no espaço principal</a></p>
      </div>
    </AuthLayout>
  )
}
