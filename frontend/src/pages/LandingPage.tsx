import { ArrowRight, ArrowUp, Download, FileSpreadsheet, LayoutDashboard, Link2, Play, RotateCcw, Search, Shield } from "lucide-react"
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

const wrap = "mx-auto w-full max-w-6xl px-[16px] sm:px-8 lg:px-12"
const sectionTitle = "font-heading text-[1.625rem] leading-[1.15] tracking-[-0.03em] text-balance sm:text-[2.125rem]"

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
      <figcaption className="mt-3 text-[12px] text-muted-foreground">Exemplo ilustrativo do catálogo do Ziv. Os apps e as pessoas não são reais.</figcaption>
    </figure>
  )
}

const frame = "overflow-hidden rounded-xl border border-border bg-sidebar p-3 sm:p-4"
const card = "rounded-xl border border-border bg-card p-4 sm:p-5"
const fieldLabel = "text-xs text-muted-foreground"

function SheetSketch() {
  return (
    <div aria-hidden="true" className={frame}>
      <div className={card}>
        <div className="flex items-start justify-between gap-3">
          <p className="font-heading text-[1.05rem] leading-snug">Fechamento mensal</p>
          <KindTab kind="script" />
        </div>
        <p className="mt-1 text-sm leading-relaxed text-muted-foreground">Junta os extratos do mês numa planilha pronta para conferir.</p>
        <dl className="mt-4 grid grid-cols-2 gap-3 border-t border-border pt-4 text-sm">
          <div><dt className={fieldLabel}>Dono</dt><dd className="mt-0.5">Marina</dd></div>
          <div><dt className={fieldLabel}>Reserva</dt><dd className="mt-0.5">Lucas</dd></div>
          <div className="col-span-2"><dt className={fieldLabel}>Como usar</dt><dd className="mt-0.5 text-muted-foreground">Escolha a pasta com os extratos do mês e clique em Rodar.</dd></div>
        </dl>
      </div>
    </div>
  )
}

function RunSketch() {
  return (
    <div aria-hidden="true" className={frame}>
      <div className={card}>
        <p className="font-heading text-[1.05rem] leading-snug">Fechamento mensal</p>
        <div className="mt-4">
          <p className={fieldLabel}>Pasta dos extratos</p>
          <div className="mt-1.5 flex h-9 items-center rounded-lg border border-input bg-background px-3 text-sm">Extratos / Outubro</div>
        </div>
        <span className={cn(buttonVariants({ variant: "run" }), "pointer-events-none mt-4 h-9 gap-1.5 pr-4 pl-3.5")}><Play className="size-3.5" /> Rodar</span>
        <div className="mt-5 border-t border-border pt-4">
          <p className={fieldLabel}>Pronto. Arquivos gerados:</p>
          <div className="mt-2 flex items-center justify-between gap-3 rounded-lg border border-border px-3 py-2.5 text-sm">
            <span className="flex min-w-0 items-center gap-2"><FileSpreadsheet className="size-4 shrink-0 text-muted-foreground" /><span className="truncate">fechamento-outubro.xlsx</span></span>
            <Download className="size-4 shrink-0 text-muted-foreground" />
          </div>
        </div>
      </div>
    </div>
  )
}

const versions = [
  { label: "Versão 3", note: "Inclui o banco novo", when: "hoje", current: true },
  { label: "Versão 2", note: "Corrige a soma das tarifas", when: "12 out", current: false },
  { label: "Versão 1", note: "Original", when: "2 set", current: false },
]

function HistorySketch() {
  return (
    <div aria-hidden="true" className={frame}>
      <div className={card}>
        <p className="font-heading text-[1.05rem] leading-snug">Versões</p>
        <ul className="mt-3 divide-y divide-border">
          {versions.map((v) => (
            <li key={v.label} className="flex items-center justify-between gap-3 py-2.5 text-sm">
              <span className="min-w-0">
                <span className="font-medium">{v.label}</span>
                <span className="text-muted-foreground"> · {v.note}</span>
              </span>
              {v.current
                ? <span className="shrink-0 rounded-full bg-secondary px-2 py-0.5 text-xs font-medium">Atual</span>
                : <span className="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground"><RotateCcw className="size-3" /> Voltar</span>}
            </li>
          ))}
        </ul>
        <p className="mt-4 border-t border-border pt-4 text-xs text-muted-foreground">Última execução: hoje, 9h12, por Ana · versão 3 · 1 arquivo</p>
      </div>
    </div>
  )
}

const docs = [
  { icon: LayoutDashboard, name: "Painel de inadimplência", meta: "Página · Finanças · versão 4" },
  { icon: FileSpreadsheet, name: "Relatório do trimestre", meta: "Página · Diretoria · versão 2" },
  { icon: Link2, name: "Indicadores de vendas", meta: "Link · Comercial" },
]

function DocsSketch() {
  return (
    <div aria-hidden="true" className={frame}>
      <div className={card}>
        <p className="font-heading text-[1.05rem] leading-snug">Documentos</p>
        <ul className="mt-3 divide-y divide-border">
          {docs.map(({ icon: Icon, name, meta }) => (
            <li key={name} className="flex items-center gap-3 py-2.5">
              <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-secondary"><Icon className="size-4 text-muted-foreground" /></span>
              <span className="min-w-0">
                <span className="block truncate text-sm font-medium">{name}</span>
                <span className="block truncate text-xs text-muted-foreground">{meta}</span>
              </span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}

const steps = [
  { title: "Sobe em minutos, já documentado.", text: "Quem criou envia o app pelo navegador. O Ziv lê o app e escreve a ficha sozinho: para que serve, como usar, quem é o dono e quem é a reserva. Ninguém precisa escrever manual.", sketch: SheetSketch },
  { title: "Qualquer pessoa roda, sem chamar quem fez.", text: "O analista acha o app no catálogo, clica em Rodar e baixa os arquivos. Não precisa abrir código.", sketch: RunSketch },
  { title: "Quebrou? Volte para a versão que funcionava.", text: "Toda mudança vira uma versão nova. Cada execução fica registrada: quando rodou, qual versão e quais arquivos saíram.", sketch: HistorySketch },
  { title: "Os painéis que a IA gera também ficam guardados.", text: "Relatórios, painéis e páginas ficam no Ziv, com versões e acesso por área. Nada de link perdido no chat.", sketch: DocsSketch },
]

export function LandingPage() {
  return (
    <main id="top" className="min-h-svh bg-background text-foreground">
      <header className="border-b border-border">
        <div className={cn(wrap, "flex h-[84px] items-center gap-6 md:gap-10")}>
          <Link to="/boas-vindas" aria-label="Ziv, início" className="shrink-0 rounded-md"><Wordmark className="h-12 w-auto" /></Link>
          <nav aria-label="Navegação principal" className="hidden items-center gap-8 text-base text-muted-foreground md:flex">
            <a href="#como-funciona" className="transition-colors hover:text-foreground">Como funciona</a>
            <a href="#comecar" className="transition-colors hover:text-foreground">Acesso</a>
          </nav>
          <div className="ml-auto flex items-center gap-2">
            <Link to="/acesso" className={cn(buttonVariants({ variant: "ghost", size: "lg" }), "h-10 px-4 text-base")}>Entrar</Link>
            <Link to="/cadastre-se" className={cn(buttonVariants({ size: "lg" }), "h-10 px-4 text-base")}>Criar espaço</Link>
          </div>
        </div>
      </header>

      <section className={cn(wrap, "grid items-start gap-12 pt-14 pb-20 sm:pt-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,29rem)] lg:gap-16 lg:pt-24 lg:pb-28")}>
        <div className="entry-rise lg:pt-10">
          <h1 className="max-w-xl font-heading text-[2.125rem] leading-[1.08] tracking-[-0.035em] text-balance sm:text-[2.5rem] lg:text-[2.875rem]">
            As ferramentas que sua equipe cria com IA viram ferramentas da empresa.
          </h1>
          <p className="mt-5 max-w-[46ch] text-base leading-relaxed text-muted-foreground">
            Hoje elas vivem na pasta de quem fez. No Ziv, cada uma ganha dono, instruções e histórico, e qualquer pessoa do time roda com um clique, sem mexer em código.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link to="/cadastre-se" className={cn(buttonVariants({ size: "lg" }), "h-10 px-4")}>Criar o espaço da empresa <ArrowRight data-icon="inline-end" /></Link>
            <Link to="/acesso" className={cn(buttonVariants({ variant: "outline", size: "lg" }), "h-10 px-4")}>Entrar</Link>
          </div>
          <p className="mt-6 inline-flex items-center gap-2 text-sm text-muted-foreground"><Shield className="size-4 shrink-0" /> <span>Recebeu um convite? Use <Link to="/acesso" className="font-medium text-foreground underline decoration-foreground/30 underline-offset-4 hover:decoration-foreground">Entrar</Link>, não crie outro espaço.</span></p>
        </div>
        <CatalogSketch />
      </section>

      <section id="como-funciona" className="scroll-mt-4 border-t border-border">
        <div className={cn(wrap, "py-20 sm:py-24")}>
          <h2 className={cn(sectionTitle, "max-w-2xl")}>Do computador de uma pessoa para a empresa inteira.</h2>
          <ol className="mt-14 grid gap-16 sm:mt-16 lg:gap-20">
            {steps.map(({ title, text, sketch: Sketch }, index) => (
              <li key={title} className="grid items-center gap-8 lg:grid-cols-2 lg:gap-16">
                <div className={cn(index % 2 === 1 && "lg:order-2")}>
                  <span className="font-heading text-sm text-muted-foreground">{index + 1}</span>
                  <h3 className="mt-2 font-heading text-[1.25rem] leading-tight tracking-[-0.02em] sm:text-[1.375rem]">{title}</h3>
                  <p className="mt-3 max-w-[46ch] text-[0.9375rem] leading-relaxed text-muted-foreground">{text}</p>
                </div>
                <div className="min-w-0"><Sketch /></div>
              </li>
            ))}
          </ol>
          <p className="mt-12 text-[12px] text-muted-foreground">Exemplos ilustrativos. Os apps e as pessoas não são reais.</p>
        </div>
      </section>

      <section id="comecar" className="scroll-mt-4 border-t border-border bg-sidebar">
        <div className={cn(wrap, "flex flex-col items-center py-24 text-center sm:py-28")}>
          <h2 className={cn(sectionTitle, "max-w-2xl")}>Comece pelas ferramentas que já existem.</h2>
          <p className="mt-4 max-w-[46ch] text-base leading-relaxed text-muted-foreground">Crie o espaço da empresa, convide quem faz as ferramentas e suba a primeira. Só entra quem você convida.</p>
          <Link to="/cadastre-se" className={cn(buttonVariants({ size: "lg" }), "mt-8 h-10 px-4")}>Criar o espaço da empresa <ArrowRight data-icon="inline-end" /></Link>
          <p className="mt-5 text-sm text-muted-foreground">Recebeu um convite ou já tem um espaço? <Link to="/acesso" className="font-medium text-foreground underline decoration-foreground/30 underline-offset-4 hover:decoration-foreground">Entrar</Link></p>
        </div>
      </section>

      <footer className="border-t border-border">
        <div className={cn(wrap, "flex flex-wrap items-center justify-between gap-x-6 gap-y-3 py-6 text-sm text-muted-foreground")}>
          <div className="flex items-center gap-4">
            <Link to="/boas-vindas" aria-label="Ziv, início" className="rounded-md text-foreground"><Wordmark className="h-7 w-auto" /></Link>
          </div>
          <a href="#top" className="inline-flex items-center gap-1.5 transition-colors hover:text-foreground" onClick={(event) => { event.preventDefault(); window.scrollTo({ top: 0, behavior: "smooth" }) }}>
            Voltar ao topo <ArrowUp className="size-3.5" />
          </a>
        </div>
      </footer>
    </main>
  )
}
