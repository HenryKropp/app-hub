import { useEffect, useMemo, useState, type ReactNode } from "react"
import { Link, useLocation } from "react-router-dom"
import { Check, ChevronDown, CircleAlert } from "lucide-react"
import { Button, buttonVariants } from "@/components/ui/button"
import { PageHeader } from "@/components/bits"
import { AgentDownloadButton } from "@/components/AgentDownload"
import { ClaudeConnect } from "@/components/ClaudeConnect"
import { api, type Onboarding } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { visibleSteps, type StepId } from "@/lib/steps"
import { cn } from "@/lib/utils"

const POLL_MS = 5000

const COPY: Record<StepId, { title: string; summary: string }> = {
  computador: { title: "Conectar seu computador", summary: "Agente ligado" },
  claude: { title: "Conectar o Claude", summary: "Claude conectado" },
  app: { title: "Subir seu primeiro app", summary: "App registrado" },
  rodar: { title: "Rodar o app", summary: "App rodou" },
  artifact: { title: "Publicar um artifact", summary: "Artifact publicado" },
  equipe: { title: "Convidar a equipe", summary: "Equipe convidada" },
}

const linkButton = (variant: "default" | "outline" = "outline") => buttonVariants({ variant })

function Body({ id, onboarding }: { id: StepId; onboarding: Onboarding }): ReactNode {
  switch (id) {
    case "computador":
      return <>
        <p className="text-sm text-muted-foreground">O Agente é um programa pequeno que fica no seu Windows, sem ícone. É ele que abre os apps do catálogo no seu computador quando você clica em Rodar.</p>
        <ol className="mt-3 flex list-decimal flex-col gap-1.5 pl-5 text-sm">
          <li>Baixe o Agente pelo botão abaixo e abra o arquivo.</li>
          <li>Se o Windows avisar, clique em <strong>Mais informações</strong> e depois em <strong>Executar assim mesmo</strong>.</li>
          <li>O navegador vai abrir uma página do Ziv. Aprove a conexão em <strong>Conectar</strong>.</li>
        </ol>
        <div className="mt-4">
          {onboarding.agent_available
            ? <AgentDownloadButton variant="outline" available />
            : <p className="text-sm text-muted-foreground">O Agente ainda não foi preparado neste Ziv. Peça a quem administra o Ziv.</p>}
        </div>
      </>
    case "claude":
      return <>
        <p className="mb-4 text-sm text-muted-foreground">Com o Claude conectado, você pede “roda o app X” ou “publica esse relatório” direto na conversa. Fica ✓ quando o Claude falar com o Ziv pela primeira vez.</p>
        <ClaudeConnect />
      </>
    case "app":
      return <>
        <p className="text-sm text-muted-foreground">Um app pode ser um script, um app web ou um programa. Escolha a pasta e o Ziv prepara o resto.</p>
        <div className="mt-4"><Link to="/registrar" className={linkButton()}>Registrar app</Link></div>
      </>
    case "rodar":
      return <>
        <p className="text-sm text-muted-foreground">Abra o catálogo, escolha um app e clique em Rodar.</p>
        <div className="mt-4"><Link to="/apps" className={linkButton("default")}>Abrir o catálogo</Link></div>
      </>
    case "artifact":
      return <>
        <p className="text-sm text-muted-foreground">O caminho principal é pedir ao Claude: “publica isso no Ziv”. Também dá para publicar pelo site.</p>
        <div className="mt-4"><Link to="/artifacts/novo" className={linkButton()}>Publicar pelo site</Link></div>
      </>
    case "equipe":
      return <>
        <p className="text-sm text-muted-foreground">Cadastre as pessoas que vão usar os apps.</p>
        <div className="mt-4"><Link to="/equipe?aba=pessoas" className={linkButton()}>Abrir equipe</Link></div>
      </>
  }
}

export function SetupPage() {
  const { person } = useAuth()
  const { hash } = useLocation()
  const [onboarding, setOnboarding] = useState<Onboarding | null>(null)
  const [error, setError] = useState("")
  const [reload, setReload] = useState(0)
  const [chosen, setChosen] = useState<StepId | null>(null)

  useEffect(() => {
    let active = true
    const load = () => api.onboarding().then((data) => {
      if (!active) return
      setOnboarding(data)
      setError("")
    }, (e: Error) => { if (active) setError(e.message) })
    void load()
    const t = setInterval(() => void load(), POLL_MS)
    return () => { active = false; clearInterval(t) }
  }, [reload])

  const steps = useMemo(() => onboarding && person ? visibleSteps(onboarding, person) : [], [onboarding, person])
  const fromHash = steps.find((s) => `#${s.id}` === hash)?.id ?? null
  const open = chosen ?? fromHash ?? steps.find((s) => !s.done)?.id ?? null

  useEffect(() => {
    if (fromHash && onboarding) document.getElementById(fromHash)?.scrollIntoView?.({ block: "start" })
  }, [fromHash, onboarding])

  if (!onboarding) {
    if (error) return <section className="rounded-xl border border-warn/30 bg-warn-soft p-6">
      <CircleAlert className="size-5 text-warn" /><h1 className="mt-3 font-heading text-xl">Não consegui carregar o Começar</h1>
      <p role="alert" className="mt-2 text-sm">{error}</p><Button className="mt-4" onClick={() => setReload((n) => n + 1)}>Tentar novamente</Button>
    </section>
    return <div className="space-y-4" aria-label="Carregando"><div className="h-8 w-64 animate-pulse rounded bg-muted" /><div className="h-48 animate-pulse rounded-xl bg-muted" /></div>
  }

  const totalDone = steps.filter((s) => s.done).length

  return <>
    <PageHeader title="Começar"><p className="text-sm text-muted-foreground">{totalDone} de {steps.length} feitos</p></PageHeader>
    <p className="mb-5 max-w-2xl text-sm text-muted-foreground">Cada passo fica ✓ sozinho quando o Ziv confirma que ele foi concluído.</p>
    {error && <p role="alert" className="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}
    <ol className="flex flex-col gap-3">
      {steps.map((step, index) => {
        const isOpen = open === step.id
        const copy = COPY[step.id]
        return <li key={step.id} id={step.id} className="scroll-mt-6 rounded-xl border border-border bg-card">
          <button type="button" aria-expanded={isOpen} aria-controls={`passo-${step.id}`} onClick={() => setChosen(isOpen ? "" as StepId : step.id)}
            className="flex w-full items-center gap-3 p-4 text-left sm:p-5">
            <span className={cn("flex size-8 shrink-0 items-center justify-center rounded-full text-sm", step.done ? "bg-foreground text-background" : "bg-secondary font-mono text-muted-foreground")}>
              {step.done ? <><Check className="size-4" aria-hidden /><span className="sr-only">✓</span></> : index + 1}
            </span>
            <span className="min-w-0 flex-1"><span className="font-heading text-base">{copy.title}</span>{step.done && <span className="ml-2 text-sm text-muted-foreground">{copy.summary}</span>}</span>
            <ChevronDown className={cn("size-4 shrink-0 text-muted-foreground transition-transform", isOpen && "rotate-180")} aria-hidden />
          </button>
          {isOpen && <div id={`passo-${step.id}`} className="border-t border-border p-4 sm:p-5"><Body id={step.id} onboarding={onboarding} /></div>}
        </li>
      })}
    </ol>
    {steps.length > 0 && totalDone === steps.length && <p className="mt-5 rounded-xl border border-ok/25 bg-ok-soft p-4 text-sm font-medium text-ok">Tudo pronto. Sua equipe já pode encontrar e usar os apps no catálogo.</p>}
  </>
}
