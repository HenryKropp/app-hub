import { ArrowRight, ArrowUp, Monitor, MousePointer2, Play, Search, Shield } from "lucide-react"
import { Link } from "react-router-dom"
import type { Kind } from "@/lib/api"
import { KindTab, Wordmark } from "@/components/bits"
import { buttonVariants } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// An illustration of the real catalog, labeled as an example: no real company, app or person.
const sampleApps: { name: string; summary: string; kind: Kind; area: string; owner: string }[] = [
  { name: "Fechamento mensal", summary: "Junta os extratos do mês numa planilha pronta para conferir.", kind: "script", area: "Finanças", owner: "Marina" },
  { name: "Relatório de vendas", summary: "Monta o relatório da semana com os pedidos do sistema.", kind: "web", area: "Comercial", owner: "Lucas" },
  { name: "Conferir notas fiscais", summary: "Abre as notas da pasta e aponta as que têm diferença.", kind: "desktop", area: "Operações", owner: "Ana" },
]

const benefits = [
  { icon: Search, title: "Encontre sem perguntar no chat.", text: "Um catálogo compartilhado mostra o que existe, para que serve e quem pode ajudar." },
  { icon: MousePointer2, title: "Use sem precisar abrir o código.", text: "Cada ferramenta tem contexto e instruções claras, para que colegas possam usá-la com confiança." },
  { icon: Monitor, title: "Scripts no computador da equipe.", text: "Scripts do Windows rodam no computador da equipe por meio do Agente do Ziv, não em um servidor remoto." },
]

const wrap = "mx-auto w-full max-w-6xl px-4 sm:px-8 lg:px-12"
const sectionTitle = "font-heading text-[1.875rem] leading-[1.12] tracking-[-0.03em] text-balance sm:text-[2.5rem]"
const inkLink = "inline-flex items-center gap-1.5 text-sm font-medium text-foreground underline decoration-foreground/30 underline-offset-4 transition-colors hover:decoration-foreground"

function CatalogSketch() {
  return (
    <figure className="entry-rise-delay min-w-0">
      <div aria-hidden="true" className="overflow-hidden rounded-xl border border-border bg-sidebar">
        <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3 sm:px-5">
          <span className="font-heading text-[1.05rem]">Apps da empresa</span>
          <span className="flex h-8 w-32 items-center gap-2 rounded-lg border border-input bg-background px-2.5 text-xs text-muted-foreground sm:w-44">
            <Search className="size-3.5 shrink-0" /> Buscar app
          </span>
        </div>
        <ul className="grid gap-3 p-3 sm:p-5">
          {sampleApps.map((app, index) => (
            <li key={app.name} className={cn("rounded-xl border border-border bg-card p-4 sm:p-5", index === 2 && "hidden sm:block")}>
              <p className="font-heading text-[1.05rem] leading-snug">{app.name}</p>
              <p className="mt-1 line-clamp-2 text-sm leading-relaxed text-muted-foreground">{app.summary}</p>
              <div className="mt-4 flex items-end justify-between gap-3 text-xs text-muted-foreground">
                <span className="flex min-w-0 flex-col items-start gap-2">
                  <KindTab kind={app.kind} />
                  <span className="max-w-full truncate">{app.area} · <span className="text-foreground/80">{app.owner}</span></span>
                </span>
                <span className={cn(buttonVariants({ variant: "run" }), "pointer-events-none h-8 gap-1.5 pr-3.5 pl-3")}>
                  <Play className="size-3.5" /> Rodar
                </span>
              </div>
            </li>
          ))}
        </ul>
      </div>
      <figcaption className="mt-3 text-xs text-muted-foreground">Exemplo ilustrativo do catálogo do Ziv. Os apps e as pessoas não são reais.</figcaption>
    </figure>
  )
}

export function LandingPage() {
  return (
    <main id="top" className="min-h-svh bg-background text-foreground">
      <header className="border-b border-border">
        <div className={cn(wrap, "flex h-[84px] items-center gap-6 md:gap-10")}>
          <Link to="/boas-vindas" aria-label="Ziv, início" className="shrink-0 rounded-md"><Wordmark className="h-12 w-auto" /></Link>
          <nav aria-label="Navegação principal" className="hidden items-center gap-8 text-base text-muted-foreground md:flex">
            <a href="#como-funciona" className="transition-colors hover:text-foreground">Como funciona</a>
            <a href="#para-equipes" className="transition-colors hover:text-foreground">Para equipes</a>
            <a href="#privacidade" className="transition-colors hover:text-foreground">Acesso por empresa</a>
          </nav>
          <div className="ml-auto flex items-center gap-2">
            <Link to="/acesso" className={cn(buttonVariants({ variant: "ghost", size: "lg" }), "h-10 px-4 text-base")}>Entrar</Link>
            <Link to="/cadastre-se" className={cn(buttonVariants({ size: "lg" }), "h-10 px-4 text-base")}>Cadastre-se</Link>
          </div>
        </div>
      </header>

      <section className={cn(wrap, "grid items-center gap-12 pt-14 pb-20 sm:pt-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,29rem)] lg:gap-16 lg:pt-24 lg:pb-28")}>
        <div className="entry-rise">
          <h1 className="max-w-xl font-heading text-[2.5rem] leading-[1.05] tracking-[-0.035em] text-balance sm:text-5xl lg:text-[3.5rem]">
            As ferramentas da sua equipe. Finalmente juntas.
          </h1>
          <p className="mt-5 max-w-[46ch] text-base leading-relaxed text-muted-foreground sm:text-lg">
            Microapps da empresa e scripts do Windows, organizados em um lugar compartilhado. Seu time encontra a ferramenta e segue com o trabalho, sem precisar lidar com código.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link to="/cadastre-se" className={cn(buttonVariants({ size: "lg" }), "h-10 px-4")}>Cadastre-se <ArrowRight data-icon="inline-end" /></Link>
            <Link to="/acesso" className={cn(buttonVariants({ variant: "outline", size: "lg" }), "h-10 px-4")}>Entrar</Link>
          </div>
          <p className="mt-6 inline-flex items-center gap-2 text-sm text-muted-foreground"><Shield className="size-4 shrink-0" /> Cada empresa tem seu próprio espaço de acesso.</p>
        </div>
        <CatalogSketch />
      </section>

      <section id="como-funciona" className="scroll-mt-4 border-t border-border">
        <div className={cn(wrap, "grid gap-6 py-20 sm:py-24 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)] lg:items-end lg:gap-16")}>
          <h2 className={sectionTitle}>Uma casa para as pequenas ferramentas que fazem grande diferença.</h2>
          <div>
            <p className="max-w-[58ch] text-base leading-relaxed text-muted-foreground">
              Planilhas com macros, scripts que alguém da equipe escreveu, pequenos apps que resolvem uma tarefa específica. São úteis, até ninguém lembrar onde estão ou como começar.
            </p>
            <a href="#para-equipes" className={cn(inkLink, "mt-5")}>Veja o que muda <ArrowRight className="size-4" /></a>
          </div>
        </div>
      </section>

      <section id="para-equipes" className="scroll-mt-4 border-t border-border">
        <div className={cn(wrap, "grid gap-10 py-20 sm:py-24 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)] lg:gap-16")}>
          <h2 className={cn(sectionTitle, "max-w-md")}>Ferramentas de equipe, sem manual de instruções.</h2>
          <ul className="divide-y divide-border border-y border-border">
            {benefits.map(({ icon: Icon, title, text }) => (
              <li key={title} className="flex gap-4 py-6">
                <Icon className="mt-0.5 size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
                <div>
                  <h3 className="font-heading text-[1.05rem] leading-snug">{title}</h3>
                  <p className="mt-1.5 max-w-[58ch] text-sm leading-relaxed text-muted-foreground">{text}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section id="privacidade" className="scroll-mt-4 border-t border-border bg-sidebar">
        <div className={cn(wrap, "py-20 sm:py-24")}>
          <h2 className={cn(sectionTitle, "max-w-2xl")}>O trabalho é compartilhado. O acesso, não.</h2>
          <p className="mt-5 max-w-[58ch] text-base leading-relaxed text-muted-foreground">
            Você entra no espaço da sua empresa. As ferramentas e as pessoas de outras empresas ficam em espaços separados, sem diretório público de organizações.
          </p>
          <Link to="/acesso" className={cn(inkLink, "mt-5")}>Acessar meu espaço <ArrowRight className="size-4" /></Link>
        </div>
      </section>

      <section className="border-t border-border">
        <div className={cn(wrap, "flex flex-col items-center py-24 text-center sm:py-28")}>
          <h2 className={cn(sectionTitle, "max-w-2xl")}>O que sua equipe já construiu merece ser encontrado.</h2>
          <p className="mt-4 text-base text-muted-foreground">Comece pelo espaço da sua empresa.</p>
          <Link to="/cadastre-se" className={cn(buttonVariants({ size: "lg" }), "mt-8 h-10 px-4")}>Cadastre-se <ArrowRight data-icon="inline-end" /></Link>
          <p className="mt-5 text-sm text-muted-foreground">Já tem um espaço? <Link to="/acesso" className="font-medium text-foreground underline decoration-foreground/30 underline-offset-4 hover:decoration-foreground">Entrar</Link></p>
        </div>
      </section>

      <footer className="border-t border-border">
        <div className={cn(wrap, "flex flex-wrap items-center justify-between gap-x-6 gap-y-3 py-6 text-sm text-muted-foreground")}>
          <div className="flex items-center gap-4">
            <Link to="/boas-vindas" aria-label="Ziv, início" className="rounded-md text-foreground"><Wordmark className="h-7 w-auto" /></Link>
            <span>Um lugar compartilhado para as ferramentas da equipe.</span>
          </div>
          <a href="#top" className="inline-flex items-center gap-1.5 transition-colors hover:text-foreground" onClick={(event) => { event.preventDefault(); window.scrollTo({ top: 0, behavior: "smooth" }) }}>
            Voltar ao topo <ArrowUp className="size-3.5" />
          </a>
        </div>
      </footer>
    </main>
  )
}
