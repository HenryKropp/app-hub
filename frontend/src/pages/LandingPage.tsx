import { useEffect, useState, type CSSProperties, type ReactNode } from "react"
import { Link } from "react-router-dom"
import "@fontsource/poppins/latin-400.css"
import "@fontsource/poppins/latin-500.css"
import "@fontsource/poppins/latin-600.css"
import "@fontsource/instrument-serif/latin-400-italic.css"
import "./landing.css"
import { Logo, Wordmark } from "@/components/bits"
import { cn } from "@/lib/utils"
import claude from "@/assets/logos/claude.svg"
import openai from "@/assets/logos/openai.svg"
import gemini from "@/assets/logos/googlegemini.svg"
import python from "@/assets/logos/python.svg"
import javascript from "@/assets/logos/javascript.svg"
import react from "@/assets/logos/react.svg"
import sheets from "@/assets/logos/googlesheets.svg"
import html from "@/assets/logos/html5.svg"
import postgres from "@/assets/logos/postgresql.svg"

// Página pública. Desenho aprovado pelo Henry em 2026-11-09 (prévia B5, ADR 030).
// Os apps e as pessoas dos desenhos são de exemplo: nenhuma empresa ou pessoa real.

const wrap = "mx-auto w-full max-w-[1200px] px-4 sm:px-8"
const quiet = "text-[var(--lp-quiet)]"
const pill = "inline-flex items-center justify-center rounded-full bg-brand text-brand-foreground transition-colors hover:bg-brand-hover"
const caption = "mt-3 text-[13px] leading-normal text-[var(--lp-quiet)]"

function Ctas({ className }: { className?: string }) {
  return (
    <div className={cn("flex flex-wrap items-center justify-center gap-x-6 gap-y-4", className)}>
      <Link to="/cadastre-se" className={cn(pill, "h-12 px-6 text-[15px]")}>Criar o espaço da empresa</Link>
      <Link to="/acesso" className="border-b border-[var(--lp-coral)] pb-0.5 text-[15px] transition-colors hover:text-[var(--lp-coral-text)]">Já tenho convite · Entrar</Link>
    </div>
  )
}

// ---------- Desenho do topo: apps espalhados entram no Ziv e saem organizados por área ----------

const sources = ["fechamento.py", "vendas.xlsx", "Artifact do Claude", "PC da Marina", "painel-web"]
const areas = [
  { name: "Finanças", count: "4 apps" },
  { name: "Comercial", count: "3 apps" },
  { name: "Operações", count: "5 apps" },
]
const chip = { fill: "var(--card)", stroke: "var(--lp-line)" }
const label = { fill: "var(--lp-ink)", fontSize: 14 }
const sublabel = { fill: "var(--lp-quiet)", fontSize: 12 }

function DiagramWide() {
  const ly = [34, 102, 170, 238, 306]
  const ry = [80, 170, 260]
  return (
    <svg viewBox="0 0 1000 340" className="block h-auto w-full" aria-hidden="true">
      {ly.map((y) => <path key={`in${y}`} className="lp-flow" d={`M210 ${y} C340 ${y} 360 170 448 170`} fill="none" stroke="var(--lp-coral)" strokeWidth="1.6" />)}
      {ry.map((y) => <path key={`out${y}`} className="lp-flow" d={`M552 170 C650 170 670 ${y} 790 ${y}`} fill="none" stroke="var(--lp-petrol)" strokeOpacity=".55" strokeWidth="1.6" />)}
      {sources.map((name, i) => (
        <g key={name}>
          <rect x="20" y={ly[i] - 18} width="190" height="36" rx="8" {...chip} />
          <text x="36" y={ly[i] + 5} style={label}>{name}</text>
        </g>
      ))}
      {areas.map((area, i) => (
        <g key={area.name}>
          <rect x="790" y={ry[i] - 22} width="190" height="44" rx="8" {...chip} />
          <text x="806" y={ry[i] + 5} style={label}>{area.name}</text>
          <text x="964" y={ry[i] + 5} textAnchor="end" style={sublabel}>{area.count}</text>
        </g>
      ))}
      <Logo x="452" y="122" width="96" height="96" aria-hidden="true" />
    </svg>
  )
}

// No celular o mesmo desenho fica de pé: os apps em cima, o Ziv no meio, as áreas embaixo.
function DiagramTall() {
  const top = [
    { x: 8, y: 8 }, { x: 182, y: 8 },
    { x: 8, y: 56 }, { x: 182, y: 56 },
    { x: 95, y: 104 },
  ]
  const bottom = [8, 125, 242]
  return (
    <svg viewBox="0 0 360 470" className="mx-auto block h-auto w-full max-w-[420px]" aria-hidden="true">
      {top.map(({ x, y }, i) => <path key={`in${i}`} className="lp-flow" d={`M${x + 85} ${y + 36} C${x + 85} ${y + 110} 180 150 180 196`} fill="none" stroke="var(--lp-coral)" strokeWidth="1.6" />)}
      {bottom.map((x) => <path key={`out${x}`} className="lp-flow" d={`M180 276 C180 320 ${x + 55} 330 ${x + 55} 386`} fill="none" stroke="var(--lp-petrol)" strokeOpacity=".55" strokeWidth="1.6" />)}
      {sources.map((name, i) => (
        <g key={name}>
          <rect x={top[i].x} y={top[i].y} width="170" height="36" rx="8" {...chip} />
          <text x={top[i].x + 85} y={top[i].y + 23} textAnchor="middle" style={{ ...label, fontSize: 13 }}>{name}</text>
        </g>
      ))}
      <Logo x="140" y="196" width="80" height="80" aria-hidden="true" />
      {areas.map((area, i) => (
        <g key={area.name}>
          <rect x={bottom[i]} y="386" width="110" height="56" rx="8" {...chip} />
          <text x={bottom[i] + 55} y="410" textAnchor="middle" style={{ ...label, fontSize: 13 }}>{area.name}</text>
          <text x={bottom[i] + 55} y="428" textAnchor="middle" style={sublabel}>{area.count}</text>
        </g>
      ))}
    </svg>
  )
}

function Diagram() {
  return (
    <figure className="mx-auto mt-12 max-w-[1000px]">
      <div className="hidden sm:block"><DiagramWide /></div>
      <div className="sm:hidden"><DiagramTall /></div>
      <figcaption className="sr-only">Apps espalhados pela empresa entram no Ziv e ficam organizados por área.</figcaption>
    </figure>
  )
}

// ---------- Faixa de ferramentas ----------

const tools = [
  { src: claude, name: "Claude" },
  { src: openai, name: "ChatGPT" },
  { src: gemini, name: "Gemini" },
  { src: python, name: "Python" },
  { src: javascript, name: "JavaScript" },
  { src: react, name: "React" },
  { src: sheets, name: "Planilhas" },
  { src: html, name: "HTML" },
  { src: postgres, name: "Postgres" },
]

function ToolStrip() {
  const row = (copy: number) => tools.map((tool) => (
    <span key={`${copy}-${tool.name}`} aria-hidden={copy === 1 || undefined} className="flex items-center gap-2 whitespace-nowrap text-[16px] text-[var(--lp-logo-gray)]">
      <i className="lp-logo" style={{ "--lp-logo": `url("${tool.src}")` } as CSSProperties} />
      {tool.name}
    </span>
  ))
  return (
    <div id="integracoes" className="scroll-mt-4 border-y border-[var(--lp-line)] bg-[var(--lp-mist)] py-7">
      <div className={wrap}>
        <p className={cn("mb-4 text-center text-[16px]", quiet)}>Para apps feitos com as ferramentas que o seu time já usa</p>
        <div className="lp-marquee">
          <div className="lp-track">{row(0)}{row(1)}</div>
        </div>
      </div>
    </div>
  )
}

// ---------- Desenhos do produto ----------

function Window({ url, children }: { url: string; children: ReactNode }) {
  return (
    <div aria-hidden="true" className="overflow-hidden rounded-xl border border-[var(--lp-line)] bg-card text-[13px] leading-[1.45]">
      <div className="flex h-[34px] items-center gap-1.5 border-b border-[var(--lp-line)] bg-[var(--lp-mist)] px-3">
        <i className="size-[9px] rounded-full bg-[var(--lp-line)]" /><i className="size-[9px] rounded-full bg-[var(--lp-line)]" /><i className="size-[9px] rounded-full bg-[var(--lp-line)]" />
        <span className={cn("ml-3.5 truncate rounded-md border border-[var(--lp-line)] bg-card px-2.5 py-0.5 text-[12px]", quiet)}>{url}</span>
      </div>
      {children}
    </div>
  )
}

const catalogApps = [
  { name: "Fechamento mensal", summary: "Junta os extratos do mês numa planilha pronta para conferir.", kind: "Script", owner: "Marina" },
  { name: "Painel de vendas", summary: "Artifact com as vendas da semana por região.", kind: "Navegador", owner: "Lucas" },
  { name: "Conferir notas fiscais", summary: "Aponta as notas da pasta que têm diferença de valor.", kind: "Janela", owner: "Ana" },
  { name: "Cobrança de atrasados", summary: "Manda o lembrete por e-mail toda segunda, 8h.", kind: "Recorrente", owner: "Rafael" },
  { name: "Política de reembolso", summary: "Regras e o formulário que o financeiro usa.", kind: "Documento", owner: "Marina" },
]
const row = "grid grid-cols-[minmax(0,1fr)_56px] items-center gap-3 px-1 sm:grid-cols-[minmax(0,1fr)_84px_60px_56px]"
const runPill = "justify-self-end rounded-full bg-brand px-3 py-[3px] text-[12px] text-brand-foreground"

function CatalogSketch() {
  return (
    <Window url="ziv.app/w/suaempresa">
      <div className="grid min-[1100px]:grid-cols-[164px_minmax(0,1fr)]">
        <div className={cn("hidden border-r border-[var(--lp-line)] bg-[var(--lp-mist)] px-2.5 py-3.5 min-[1100px]:block", quiet)}>
          <div className="whitespace-nowrap rounded-md bg-card px-2.5 py-1.5 text-[var(--lp-ink)] shadow-[0_0_0_1px_var(--lp-line)]">Apps da empresa</div>
          <div className="px-2.5 py-1.5">Rodando agora</div>
          <div className="px-2.5 py-1.5">Pendências</div>
          <div className="mt-3.5 px-2.5 py-1.5 text-[11px] tracking-[0.06em] uppercase">Áreas</div>
          <div className="px-2.5 py-1.5">Finanças</div>
          <div className="px-2.5 py-1.5">Comercial</div>
          <div className="px-2.5 py-1.5">Operações</div>
        </div>
        <div className="min-w-0 px-5 py-[18px]">
          <div className="mb-3.5 flex items-center justify-between gap-3">
            <span className="text-[15px]">Apps da empresa</span>
            <span className={cn("w-[110px] rounded-[7px] border border-[var(--lp-line)] px-2.5 py-[5px] sm:w-[180px]", quiet)}>Buscar app</span>
          </div>
          <div className={cn(row, "pb-2.5 text-[12px]", quiet)}><span>App</span><span className="hidden sm:block">Tipo</span><span className="hidden sm:block">Dono</span><span /></div>
          {catalogApps.map((app) => (
            <div key={app.name} className={cn(row, "border-t border-[var(--lp-line)] py-[11px]")}>
              <div className="min-w-0">
                <div className="truncate">{app.name}</div>
                <div className={cn("truncate", quiet)}>{app.summary}</div>
              </div>
              <span className="hidden sm:block"><span className={cn("inline-block rounded-full bg-secondary px-2 py-px text-[12px]", quiet)}>{app.kind}</span></span>
              <span className={cn("hidden sm:block", quiet)}>{app.owner}</span>
              {app.kind === "Documento" ? <span className={cn("justify-self-end", quiet)}>Abrir</span> : <span className={runPill}>Rodar</span>}
            </div>
          ))}
        </div>
      </div>
    </Window>
  )
}

function Field({ label: name, wide, children }: { label: string; wide?: boolean; children: ReactNode }) {
  return (
    <div className={cn(wide && "col-span-2")}>
      <dt className={cn("text-[12px]", quiet)}>{name}</dt>
      <dd className="mt-0.5">{children}</dd>
    </div>
  )
}

function SheetSketch() {
  return (
    <Window url="ziv.app/w/suaempresa/apps/fechamento-mensal">
      <div className="px-6 py-[22px]">
        <div className={quiet}>Finanças</div>
        <div className="text-[17px] tracking-[-0.01em]">Fechamento mensal</div>
        <div className="mt-3 mb-[18px] flex w-max gap-1 rounded-lg bg-secondary p-[3px]">
          <span className="rounded-md bg-card px-3.5 py-[3px] shadow-[inset_0_-2px_0_var(--lp-coral),0_1px_2px_rgb(0_0_0/0.06)]">Usar</span>
          <span className={cn("px-3.5 py-[3px]", quiet)}>Cuidar</span>
        </div>
        <dl className="grid grid-cols-2 gap-x-7 gap-y-4 border-t border-[var(--lp-line)] pt-4">
          <Field label="Para que serve" wide>Junta os extratos do mês numa planilha pronta para conferir.</Field>
          <Field label="Como usar" wide>Escolha a pasta com os extratos do mês e clique em Rodar. A planilha aparece em Downloads.</Field>
          <Field label="Pede">Pasta dos extratos</Field>
          <Field label="Entrega">fechamento-outubro.xlsx</Field>
          <Field label="Dono">Marina</Field>
          <Field label="Reserva">Lucas</Field>
          <Field label="Última vez" wide>Hoje, 9h12, por Ana · versão 3</Field>
        </dl>
      </div>
    </Window>
  )
}

// ---------- Como funciona: passos à esquerda, uma imagem que acompanha o passo ----------

const pic = "lp-pic flex min-h-[220px] flex-col justify-center rounded-xl border border-[var(--lp-line)] bg-[var(--lp-mist)] p-5 text-[12.5px] leading-[1.55] [grid-area:1/1]"
const term = "font-mono text-[12px]"
const inner = "rounded-[10px] border border-[var(--lp-line)] bg-card px-4 py-3.5"
const done = "mt-2 text-[var(--lp-coral-text)]"

function Terminal({ prompt, lines, result }: { prompt: string; lines: string[]; result: string }) {
  return (
    <>
      <div>&gt; {prompt}</div>
      <div className={cn("mt-2 whitespace-pre-wrap", quiet)}>{lines.map((line) => <div key={line}>{"  "}{line}</div>)}</div>
      <div className={done}>{"  "}✓ {result}</div>
    </>
  )
}

const steps: { title: string; text: string; mono?: boolean; picture: ReactNode }[] = [
  {
    title: "O time faz o app",
    text: "No Claude, como já faz hoje.",
    mono: true,
    picture: <Terminal prompt="faz um script que junta os extratos do mês numa planilha" lines={["Lendo a pasta Extratos/Outubro… 14 arquivos.", "Criando fechamento.py", "Testando com outubro… ok, 312 linhas."]} result="fechamento.py pronto" />,
  },
  {
    title: "Pede para subir",
    text: "O Claude envia, escreve a ficha e guarda as chaves.",
    mono: true,
    picture: <Terminal prompt="sobe no Ziv, área Finanças" lines={["Escrevendo a ficha: para que serve, como usar,", "o que pede e o que entrega.", "Guardando 1 chave no cofre do app."]} result="Fechamento mensal publicado · 2 min" />,
  },
  {
    title: "Você define quem cuida",
    text: "Dono, reserva e quem pode usar.",
    picture: (
      <div className={inner}>
        <div className="mb-2.5">Fechamento mensal · Cuidar · Acesso</div>
        <dl className="grid grid-cols-[110px_minmax(0,1fr)] items-center gap-x-3 gap-y-2">
          <dt className={quiet}>Dono</dt><dd>Marina</dd>
          <dt className={quiet}>Reserva</dt><dd>Lucas</dd>
          <dt className={quiet}>Área</dt><dd>Finanças</dd>
          <dt className={quiet}>Quem pode usar</dt>
          <dd>{["Finanças", "Ana", "+ pessoa"].map((who) => <span key={who} className="mr-1 mb-1 inline-block rounded-full border border-[var(--lp-line)] bg-card px-2 py-px">{who}</span>)}</dd>
          <dt className={quiet}>Aparece no catálogo</dt>
          <dd><span className="relative inline-block h-4 w-7 rounded-full bg-brand align-middle after:absolute after:top-0.5 after:right-0.5 after:size-3 after:rounded-full after:bg-white" /></dd>
        </dl>
      </div>
    ),
  },
  {
    title: "A empresa usa",
    text: "Clica em Rodar, na hora ou agendado.",
    picture: (
      <div className={inner}>
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0"><div>Fechamento mensal</div><div className={quiet}>Pasta dos extratos: Extratos / Outubro</div></div>
          <span className={runPill}>Rodar</span>
        </div>
        <div className="mt-2.5 border-t border-[var(--lp-line)] pt-2.5"><div className={quiet}>Pronto. Arquivo gerado:</div><div>fechamento-outubro.xlsx</div></div>
        <div className={cn("mt-2.5 border-t border-[var(--lp-line)] pt-2.5", quiet)}>Próxima execução agendada: 1º de dezembro, 8h</div>
      </div>
    ),
  },
]

function Stepper() {
  const [active, setActive] = useState(0)
  const [cycle, setCycle] = useState(0)
  useEffect(() => {
    if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return
    const timer = window.setInterval(() => setActive((current) => (current + 1) % steps.length), 3500)
    return () => window.clearInterval(timer)
  }, [cycle])
  return (
    <div className="mt-7 grid items-center gap-6 min-[900px]:grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)] min-[900px]:gap-12">
      <ol className="border-l border-[var(--lp-line)]">
        {steps.map((step, i) => {
          const on = i === active
          return (
            <li key={step.title} className={cn("-ml-px border-l-2 transition-colors", on ? "border-[var(--lp-coral)]" : "border-transparent")}>
              <button
                type="button"
                aria-current={on ? "step" : undefined}
                onClick={() => { setActive(i); setCycle((n) => n + 1) }}
                className={cn("block w-full py-3.5 pl-5 text-left text-[clamp(16px,1.45vw,20px)] transition-colors", on ? "text-[var(--lp-ink)]" : quiet)}
              >
                <span className={cn("mr-2.5 font-mono text-[12px]", on && "text-[var(--lp-coral-text)]")}>0{i + 1}</span>
                {step.title}
                <span className={cn("mt-0.5 text-[clamp(14px,1.1vw,16px)]", quiet, on ? "block" : "hidden")}>{step.text}</span>
              </button>
            </li>
          )
        })}
      </ol>
      <div className="grid" aria-hidden="true">
        {steps.map((step, i) => (
          <div key={step.title} data-on={i === active} className={cn(pic, step.mono && term)}>{step.picture}</div>
        ))}
      </div>
    </div>
  )
}

// ---------- Página ----------

export function LandingPage() {
  return (
    <main id="top" className="lp min-h-svh bg-background text-[var(--lp-ink)]">
      <header className={wrap}>
        <nav aria-label="Navegação principal" className="grid h-20 grid-cols-[1fr_auto] items-center min-[900px]:grid-cols-[1fr_auto_1fr]">
          <Link to="/boas-vindas" aria-label="Ziv, início" className="w-max rounded-md"><Wordmark className="h-10 w-auto" /></Link>
          <div className={cn("hidden justify-center gap-8 text-[14px] min-[900px]:flex", quiet)}>
            <a href="#catalogo" className="transition-colors hover:text-[var(--lp-ink)]">Catálogo</a>
            <a href="#ficha" className="transition-colors hover:text-[var(--lp-ink)]">Documentação</a>
            <a href="#como" className="transition-colors hover:text-[var(--lp-ink)]">Como funciona</a>
            <a href="#integracoes" className="transition-colors hover:text-[var(--lp-ink)]">Integrações</a>
          </div>
          <div className="flex items-center justify-end gap-5 text-[14px]">
            <Link to="/acesso" className="transition-colors hover:text-[var(--lp-coral-text)]">Entrar</Link>
            <Link to="/cadastre-se" className={cn(pill, "h-10 px-5 text-[15px]")}>Criar espaço</Link>
          </div>
        </nav>
      </header>

      <section className={cn(wrap, "pt-14 pb-[72px] text-center")}>
        <h1 className="lp-display lp-h1 mx-auto max-w-[15em]">Todos os apps da sua empresa, <span className="lp-accent">num lugar só.</span></h1>
        <Diagram />
        <Ctas className="mt-8" />
      </section>

      <ToolStrip />

      <section id="catalogo" className="scroll-mt-4 py-[72px]">
        <div className={cn(wrap, "grid items-center gap-12 min-[900px]:grid-cols-[minmax(0,1fr)_minmax(0,1.35fr)]")}>
          <div>
            <p className="lp-eyebrow">Catalogado</p>
            <h2 className="lp-display lp-h2">Uma lista só, com busca, separada por área.</h2>
            <p className={cn("mt-4", quiet)}>Scripts, planilhas, artifacts e apps com tela, cada um com dono e um botão Rodar.</p>
          </div>
          <figure className="min-w-0">
            <CatalogSketch />
            <figcaption className={caption}>Exemplo ilustrativo. Os apps e as pessoas não são reais.</figcaption>
          </figure>
        </div>
      </section>

      <section id="ficha" className="scroll-mt-4 border-t border-[var(--lp-line)] py-[72px]">
        <div className={cn(wrap, "grid items-center gap-12 min-[900px]:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]")}>
          <figure className="order-2 min-w-0 min-[900px]:order-none">
            <SheetSketch />
            <figcaption className={caption}>Exemplo ilustrativo. Os apps e as pessoas não são reais.</figcaption>
          </figure>
          <div>
            <p className="lp-eyebrow">Documentado</p>
            <h2 className="lp-display lp-h2">Documentado em minutos, sem ninguém escrever manual.</h2>
            <p className={cn("mt-4", quiet)}>O Claude escreve a ficha junto com o upload.</p>
          </div>
        </div>
      </section>

      <section id="como" className="scroll-mt-4 border-t border-[var(--lp-line)] py-[72px]">
        <div className={wrap}>
          <p className="lp-eyebrow">Como funciona</p>
          <h2 className="lp-display lp-h2">Do Claude para a empresa inteira.</h2>
          <Stepper />
        </div>
      </section>

      <section className="border-t border-[var(--lp-line)] py-[72px] text-center">
        <div className={wrap}>
          <h2 className="lp-display lp-h2">Comece pelos apps que já existem.</h2>
          <p className={cn("mx-auto mt-4 max-w-[34em]", quiet)}>Crie o espaço da empresa, convide quem faz os apps e suba o primeiro. Só entra quem você convida.</p>
          <Ctas className="mt-8" />
        </div>
      </section>

      <footer className="border-t border-[var(--lp-line)]">
        <div className={cn(wrap, "flex flex-wrap items-center justify-between gap-4 pt-10 pb-14 text-[13px]", quiet)}>
          <a href="#top" aria-label="Ziv, voltar ao topo" className="rounded-md text-[var(--lp-ink)]" onClick={(event) => { event.preventDefault(); window.scrollTo({ top: 0, behavior: "smooth" }) }}>
            <Wordmark className="h-8 w-auto" />
          </a>
          <span>Feito no Brasil</span>
        </div>
      </footer>
    </main>
  )
}
