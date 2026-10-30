import { useState, type FormEvent } from "react"
import { ArrowRight, Building2, KeyRound, Link2, ShieldCheck } from "lucide-react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, Wordmark } from "@/components/bits"
import { api, ApiError } from "@/lib/api"
import { slugFromEntry, validCompanySlug } from "@/lib/company-entry"

function localPath(url: string) {
  try {
    const parsed = new URL(url, window.location.origin)
    if (parsed.origin !== window.location.origin) return null
    return parsed.pathname
  } catch {
    return null
  }
}

export function CompanyEntryPage() {
  const [companyInput, setCompanyInput] = useState("")
  const [companyBusy, setCompanyBusy] = useState(false)
  const [companyError, setCompanyError] = useState("")
  const [companyName, setCompanyName] = useState("")
  const [slug, setSlug] = useState("")
  const [ownerName, setOwnerName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [createBusy, setCreateBusy] = useState(false)
  const [createError, setCreateError] = useState("")

  const enterCompany = async (event: FormEvent) => {
    event.preventDefault()
    setCompanyError("")
    const companySlug = slugFromEntry(companyInput, window.location.origin)
    if (!companySlug) {
      setCompanyError("Informe um endereço de empresa válido ou um identificador em letras minúsculas, números e hífens.")
      return
    }
    setCompanyBusy(true)
    try {
      const company = await api.entryCompanyBySlug(companySlug)
      if (company.slug !== companySlug || (company.base_path && company.base_path !== `/w/${companySlug}`)) {
        throw new Error("Não consegui confirmar o endereço deste espaço. Confira o identificador e tente novamente.")
      }
      const basePath = company.base_path || ""
      const next = `${basePath}/entrar`.replace(/\/+/g, "/")
      window.location.assign(next)
    } catch (error) {
      setCompanyError(error instanceof ApiError && error.status === 404
        ? "Não encontrei uma empresa com esse endereço. Confira o identificador com quem administra o espaço."
        : error instanceof Error ? error.message : "Não foi possível confirmar essa empresa.")
    } finally {
      setCompanyBusy(false)
    }
  }

  const createCompany = async (event: FormEvent) => {
    event.preventDefault()
    setCreateError("")
    if (!validCompanySlug(slug)) {
      setCreateError("O identificador deve ter de 3 a 40 caracteres, começar com uma letra minúscula e conter apenas letras, números ou hífens.")
      return
    }
    if (new TextEncoder().encode(password).length < 8 || new TextEncoder().encode(password).length > 72) {
      setCreateError("A senha deve ter entre 8 e 72 bytes.")
      return
    }
    setCreateBusy(true)
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
      const destination = basePath ? `${basePath}/entrar` : "/entrar"
      window.location.assign(destination)
    } catch (error) {
      const status = error instanceof ApiError ? error.status : 0
      setCreateError(status === 409
        ? "Esse identificador já está em uso. Escolha outro."
        : status === 400
          ? (error instanceof Error ? error.message : "Confira os dados informados.")
          : status === 503
            ? "Novas empresas estão temporariamente indisponíveis. Tente novamente mais tarde."
            : error instanceof Error ? error.message : "Não foi possível criar a empresa.")
    } finally {
      setCreateBusy(false)
    }
  }

  return (
    <main className="entry-shell relative min-h-svh overflow-hidden">
      <div className="entry-grid pointer-events-none absolute inset-0 opacity-70" aria-hidden="true" />
      <div className="relative mx-auto flex min-h-svh max-w-7xl flex-col px-5 py-6 sm:px-8 lg:px-12">
        <header className="flex items-center justify-between">
          <Link to="/boas-vindas" aria-label="Ziv, início" className="inline-flex items-center rounded-sm text-foreground">
            <Wordmark className="h-10 w-auto" />
          </Link>
          <Link to="/entrar" className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground">Já tenho acesso <ArrowRight className="ml-1 inline size-4" /></Link>
        </header>
        <div className="grid flex-1 items-center gap-10 py-12 lg:grid-cols-[0.86fr_1.14fr] lg:gap-20 lg:py-16">
          <section className="entry-rise max-w-xl">
            <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-logo-square/15 bg-card/80 px-3 py-1.5 text-xs font-medium text-logo-square shadow-sm">
              <ShieldCheck className="size-3.5" /> Cada empresa no seu próprio espaço
            </div>
            <h1 className="font-heading max-w-lg text-4xl leading-[1.04] tracking-[-0.045em] text-balance sm:text-5xl lg:text-[3.75rem]">
              As ferramentas da sua equipe, <span className="text-logo-square">no lugar certo.</span>
            </h1>
            <p className="mt-5 max-w-md text-base leading-7 text-muted-foreground">
              Entre no espaço da sua empresa para encontrar e usar os microapps que ela mantém. O acesso nunca mistura equipes.
            </p>
            <div className="mt-8 flex items-center gap-3 text-sm text-muted-foreground">
              <span className="flex size-9 items-center justify-center rounded-full bg-brand/15 text-brand"><KeyRound className="size-4" /></span>
              <span>Seu endereço identifica o espaço privado da equipe.</span>
            </div>
          </section>

          <section className="entry-rise-delay grid gap-4 sm:grid-cols-2">
            <form onSubmit={enterCompany} className="flex flex-col rounded-2xl border border-border bg-card/95 p-5 shadow-pop backdrop-blur-sm sm:p-6">
              <div className="flex size-10 items-center justify-center rounded-xl bg-logo-square text-white"><Building2 className="size-5" /></div>
              <h2 className="mt-5 font-heading text-xl">Acessar uma empresa</h2>
              <p className="mt-1 min-h-10 text-sm leading-5 text-muted-foreground">Use o identificador ou o link que sua equipe compartilhou.</p>
              <Field className="mt-5" label="Identificador ou endereço" htmlFor="company-address" hint="Ex.: oficina ou ziv.exemplo.com/w/oficina">
                <Input id="company-address" value={companyInput} onChange={(event) => setCompanyInput(event.target.value)} placeholder="nome-da-empresa" autoComplete="url" required />
              </Field>
              {companyError && <p role="alert" className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{companyError}</p>}
              <Button type="submit" className="mt-5 w-full gap-2" disabled={companyBusy}>
                {companyBusy ? "Conferindo espaço…" : "Continuar"} {!companyBusy && <ArrowRight className="size-4" />}
              </Button>
            </form>

            <form onSubmit={createCompany} className="flex flex-col rounded-2xl border border-border bg-card/95 p-5 shadow-pop backdrop-blur-sm sm:p-6">
              <div className="flex size-10 items-center justify-center rounded-xl bg-brand/15 text-logo-square"><Link2 className="size-5" /></div>
              <h2 className="mt-5 font-heading text-xl">Criar um espaço</h2>
              <p className="mt-1 min-h-10 text-sm leading-5 text-muted-foreground">Comece a organizar os apps da sua empresa em um ambiente separado.</p>
              <div className="mt-5 flex flex-col gap-3">
                <Field label="Nome da empresa" htmlFor="company-name">
                  <Input id="company-name" value={companyName} onChange={(event) => setCompanyName(event.target.value)} placeholder="Ex.: Horizonte Logística" required />
                </Field>
                <Field label="Identificador" htmlFor="company-slug" hint="3–40 caracteres; letras minúsculas, números e hífens.">
                  <div className="flex items-center rounded-md border border-input bg-background focus-within:ring-3 focus-within:ring-ring/30">
                    <span className="pl-3 text-sm text-muted-foreground">/w/</span>
                    <Input id="company-slug" className="border-0 shadow-none focus-visible:ring-0" value={slug} onChange={(event) => setSlug(event.target.value)} placeholder="horizonte" minLength={3} maxLength={40} pattern="[a-z][a-z0-9-]{2,39}" required />
                  </div>
                </Field>
                <Field label="Seu nome" htmlFor="owner-name">
                  <Input id="owner-name" value={ownerName} onChange={(event) => setOwnerName(event.target.value)} autoComplete="name" required />
                </Field>
                <Field label="E-mail de trabalho" htmlFor="owner-email">
                  <Input id="owner-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" required />
                </Field>
                <Field label="Senha" htmlFor="owner-password" hint="Entre 8 e 72 bytes.">
                  <Input id="owner-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" minLength={8} required />
                </Field>
              </div>
              {createError && <p role="alert" className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{createError}</p>}
              <Button type="submit" variant="outline" className="mt-5 w-full gap-2" disabled={createBusy}>
                {createBusy ? "Criando espaço…" : "Criar empresa"} {!createBusy && <ArrowRight className="size-4" />}
              </Button>
              <p className="mt-3 text-center text-xs text-muted-foreground">A criação não entra automaticamente na conta.</p>
            </form>
          </section>
        </div>
        <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-border/70 py-4 text-xs text-muted-foreground">
          <span>Ziv · espaços privados para equipes</span>
          <span>Precisa de acesso? Peça um convite à administração da sua empresa.</span>
        </footer>
      </div>
    </main>
  )
}
