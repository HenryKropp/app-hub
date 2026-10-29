import { useEffect, useMemo, useState, type FormEvent } from "react"
import { Link } from "react-router-dom"
import { Check, CircleAlert, CircleCheck, Download, Laptop, RefreshCw, Users, WandSparkles } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, PageHeader } from "@/components/bits"
import { AGENT_DOWNLOAD_URL, api, type Onboarding } from "@/lib/api"
import { cn } from "@/lib/utils"

const steps = [
  { id: "company", title: "Dê um nome à empresa", description: "O nome aparece no espaço da sua equipe." },
  { id: "computer", title: "Conecte seu computador", description: "O Agente permite abrir os apps no computador de cada pessoa." },
  { id: "apps", title: "Registre o primeiro app", description: "Traga um script ou ferramenta para o catálogo." },
  { id: "run", title: "Faça uma execução de teste", description: "Confirme que o app abre e entrega os arquivos esperados." },
  { id: "team", title: "Convide a equipe", description: "Cadastre as pessoas que vão usar os apps." },
] as const

function completed(onboarding: Onboarding) {
  return [
    Boolean(onboarding.company_name.trim()),
    onboarding.computer_connected,
    onboarding.has_apps,
    onboarding.has_successful_run,
    onboarding.has_team,
  ]
}

export function SetupPage() {
  const [onboarding, setOnboarding] = useState<Onboarding | null>(null)
  const [name, setName] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [reload, setReload] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError("")
    api.onboarding().then((data) => {
      if (!active) return
      setOnboarding(data)
      setName(data.company_name)
    }, (e: Error) => {
      if (active) setError(e.message)
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [reload])

  const done = useMemo(() => onboarding ? completed(onboarding) : [], [onboarding])
  const totalDone = done.filter(Boolean).length
  const saveName = async (event: FormEvent) => {
    event.preventDefault()
    if (!name.trim() || !onboarding?.can_manage) return
    setSaving(true)
    setError("")
    try {
      const updated = await api.saveCompanyName(name.trim())
      setOnboarding(updated)
      setName(updated.company_name)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não consegui salvar o nome da empresa.")
    } finally {
      setSaving(false)
    }
  }

  const refresh = () => setReload((n) => n + 1)
  if (loading && !onboarding) return <div className="space-y-4" aria-label="Carregando configuração"><div className="h-8 w-64 animate-pulse rounded bg-muted" /><div className="h-28 animate-pulse rounded-xl bg-muted" /><div className="h-48 animate-pulse rounded-xl bg-muted" /></div>
  if (error && !onboarding) return <section className="rounded-xl border border-warn/30 bg-warn-soft p-6">
    <CircleAlert className="size-5 text-warn" /><h1 className="mt-3 font-heading text-xl">Não consegui carregar o início</h1>
    <p role="alert" className="mt-2 text-sm">{error}</p><Button className="mt-4" onClick={refresh}>Tentar novamente</Button>
  </section>
  if (!onboarding) return null

  const rows = [
    <form key="company-form" onSubmit={saveName} className="mt-4 max-w-xl">
      <Field label="Nome da empresa" htmlFor="company-name" hint="Você pode mudar esse nome depois.">
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input id="company-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Ex.: Alis" disabled={!onboarding.can_manage || saving} required />
          <Button type="submit" disabled={!onboarding.can_manage || saving || !name.trim() || name.trim() === onboarding.company_name}>{saving ? "Salvando…" : "Salvar nome"}</Button>
        </div>
      </Field>
      {!onboarding.can_manage && <p className="mt-2 text-xs text-muted-foreground">Só uma pessoa com permissão de gestão pode alterar o nome.</p>}
    </form>,
    <div key="computer-actions" className="mt-4 flex flex-wrap items-center gap-2">
      <Link to="/configuracoes#computador" className={cn(buttonVariants({ variant: "outline" }), "gap-2")}><Laptop className="size-4" /> Configurar computador</Link>
      {onboarding.agent_available && <a href={AGENT_DOWNLOAD_URL} className={cn(buttonVariants({ variant: "outline" }), "gap-2")}><Download className="size-4" /> Baixar Agente</a>}
      {!onboarding.agent_available && <p className="text-sm text-muted-foreground">O Agente ainda não está disponível neste Kova. Peça ajuda a quem administra.</p>}
    </div>,
    <div key="apps-actions" className="mt-4 flex gap-2"><Link to="/registrar" className={cn(buttonVariants({ variant: "outline" }), "gap-2")}><WandSparkles className="size-4" /> Registrar app</Link><Link to="/" className="inline-flex h-9 items-center px-2 text-sm text-muted-foreground hover:text-foreground">Ver catálogo</Link></div>,
    <div key="run-actions" className="mt-4 flex gap-2"><Link to="/" className={cn(buttonVariants({ variant: "default" }))}>Abrir catálogo</Link><Link to="/execucoes" className="inline-flex h-9 items-center px-2 text-sm text-muted-foreground hover:text-foreground">Ver execuções</Link></div>,
    <div key="team-actions" className="mt-4 flex gap-2"><Link to="/equipe?aba=pessoas" className={cn(buttonVariants({ variant: "outline" }), "gap-2")}><Users className="size-4" /> Abrir equipe</Link></div>,
  ]

  return <>
    <PageHeader title="Comece por aqui"><Button variant="outline" size="sm" onClick={refresh} disabled={loading} className="gap-2"><RefreshCw className={cn("size-3.5", loading && "animate-spin")} /> Atualizar</Button></PageHeader>
    <div className="mb-7 rounded-xl border border-border bg-muted/40 p-5 md:flex md:items-center md:justify-between md:p-6">
      <div><h2 className="font-heading text-2xl">Prepare o Kova para sua equipe</h2><p className="mt-2 max-w-2xl text-sm text-muted-foreground">Faça cada etapa no seu ritmo. O progresso só muda quando o Kova confirma que algo foi concluído.</p></div>
      <div className="mt-4 shrink-0 md:mt-0 md:text-right"><p className="font-mono text-2xl font-semibold">{totalDone}<span className="text-muted-foreground">/5</span></p><p className="text-xs text-muted-foreground">etapas concluídas</p></div>
    </div>
    {error && onboarding && <p role="alert" className="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}
    {!onboarding.can_manage && <p className="mb-4 rounded-lg border border-border bg-card p-3 text-sm text-muted-foreground">Você pode acompanhar a configuração, mas apenas uma pessoa com permissão de gestão pode alterar os dados da empresa.</p>}
    <ol className="relative flex flex-col gap-3">
      {steps.map((step, index) => {
        const complete = done[index]
        const StepIcon = complete ? CircleCheck : index === 1 ? Laptop : index === 4 ? Users : WandSparkles
        return <li key={step.id} className="grid grid-cols-[2.5rem_1fr] gap-3 rounded-xl border border-border bg-card p-4 sm:grid-cols-[3rem_1fr] sm:gap-4 sm:p-5">
          <div className={cn("flex size-10 items-center justify-center rounded-full", complete ? "bg-ok-soft text-ok" : "bg-secondary text-muted-foreground")}><StepIcon className="size-5" /></div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1"><span className="font-mono text-xs text-muted-foreground">0{index + 1}</span><h2 className="font-heading text-base">{step.title}</h2>{complete && <span className="inline-flex items-center gap-1 text-xs font-medium text-ok"><Check className="size-3.5" /> Concluída</span>}</div>
            <p className="mt-1 text-sm text-muted-foreground">{step.description}</p>
            {rows[index]}
          </div>
        </li>
      })}
    </ol>
    {totalDone === 5 && <p className="mt-5 rounded-xl border border-ok/25 bg-ok-soft p-4 text-sm font-medium text-ok">Tudo pronto. Sua equipe já pode encontrar e usar os apps no catálogo.</p>}
  </>
}
