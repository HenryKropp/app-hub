import { useState } from "react"
import { ArrowDown, ArrowRight, Check, ChevronDown, CircleHelp, Command, Menu, Monitor, MousePointer2, Shield, Sparkles, X } from "lucide-react"
import { Link } from "react-router-dom"
import { Wordmark } from "@/components/bits"

const tools = [
  { mark: "F", name: "Fechamento mensal", detail: "Finanças · Planilha e arquivos", tone: "peach", type: "Aplicativo" },
  { mark: "↗", name: "Atualizar catálogo", detail: "Operações · Script local", tone: "mint", type: "Script Windows" },
  { mark: "R", name: "Relatório de vendas", detail: "Comercial · Aplicativo interno", tone: "lavender", type: "Aplicativo" },
]

export function LandingPage() {
  const [menuOpen, setMenuOpen] = useState(false)
  return (
    <main className="landing">
      <header className="landing-nav">
        <div className="landing-nav-inner">
          <Link to="/boas-vindas" aria-label="Ziv, início" className="landing-logo"><Wordmark className="h-9 w-auto" /></Link>
          <nav aria-label="Navegação principal" className={menuOpen ? "landing-links is-open" : "landing-links"}>
            <a href="#como-funciona" onClick={() => setMenuOpen(false)}>Como funciona</a>
            <a href="#para-equipes" onClick={() => setMenuOpen(false)}>Para equipes</a>
            <a href="#privacidade" onClick={() => setMenuOpen(false)}>Acesso por empresa</a>
            <Link to="/acesso" className="mobile-signup" onClick={() => setMenuOpen(false)}>Login / Cadastre-se <ArrowRight size={16} /></Link>
          </nav>
          <div className="landing-nav-actions">
            <Link to="/acesso" className="nav-signup">Login / Cadastre-se <ArrowRight size={15} /></Link>
          </div>
          <button className="landing-menu-toggle" type="button" aria-label={menuOpen ? "Fechar menu" : "Abrir menu"} aria-expanded={menuOpen} onClick={() => setMenuOpen(!menuOpen)}>
            {menuOpen ? <X size={21} /> : <Menu size={21} />}
          </button>
        </div>
      </header>

      <section className="landing-hero">
        <div className="hero-copy">
          <div className="eyebrow"><span className="eyebrow-dot" /> UM LUGAR COMPARTILHADO PARA O TRABALHO DE VERDADE</div>
          <h1>As ferramentas da sua equipe. <em>Finalmente juntas.</em></h1>
          <p className="hero-description">Microapps da empresa e scripts do Windows, organizados em uma casa compartilhada. Seu time encontra a ferramenta e segue com o trabalho — sem precisar lidar com código.</p>
          <div className="hero-actions">
            <Link to="/acesso" className="button-coral">Login <ArrowRight size={17} /></Link>
            <Link to="/cadastre-se" className="button-text">Cadastre-se <ArrowRight size={16} /></Link>
          </div>
          <div className="hero-note"><Shield size={15} /> Cada empresa tem seu próprio espaço de acesso.</div>
        </div>
        <div className="hero-art" aria-label="Ilustração de uma biblioteca de ferramentas do Ziv">
          <div className="art-orbit orbit-one" />
          <div className="art-orbit orbit-two" />
          <div className="art-sun" />
          <div className="product-window">
            <div className="window-top">
              <div className="window-brand"><span className="window-mark">z</span><span>Ziv <small>·</small> Aurora Studio</span></div>
              <span className="window-team"><span /> Espaço da equipe</span>
            </div>
            <div className="window-body">
              <aside className="window-rail" aria-hidden="true"><span className="rail-active"><Command size={15} /></span><span><Monitor size={15} /></span><span><CircleHelp size={15} /></span></aside>
              <div className="window-content">
                <div className="window-heading"><div><small>SEU ESPAÇO DE TRABALHO</small><h2>Bom dia, equipe</h2></div><span className="avatar-stack"><i>MA</i><i>LC</i><i>+4</i></span></div>
                <div className="window-search"><span>⌕</span> Encontre uma ferramenta <kbd>⌘ K</kbd></div>
                <div className="window-label">USADOS PELA EQUIPE <span>Ver todos <ArrowRight size={12} /></span></div>
                <div className="tool-list">
                  {tools.map((tool) => <div className="tool-row" key={tool.name}>
                    <div className={`tool-icon ${tool.tone}`}>{tool.mark}</div>
                    <div className="tool-info"><b>{tool.name}</b><small>{tool.detail}</small></div>
                    <span className="tool-kind">{tool.type}</span>
                    <span className="tool-row-arrow" aria-hidden="true"><ArrowRight size={14} /></span>
                  </div>)}
                </div>
                <div className="local-device"><span className="device-symbol"><Monitor size={14} /></span><span><b>Seu computador</b><small>Scripts executados localmente</small></span><span className="device-status"><i /> Conectado</span></div>
              </div>
            </div>
          </div>
          <div className="floating-stamp stamp-one"><span className="stamp-icon"><MousePointer2 size={15} /></span><span><b>Feito para usar</b><small>Não para decifrar</small></span></div>
          <div className="floating-stamp stamp-two"><span className="stamp-check"><Check size={14} /></span><span><b>Espaço da empresa</b><small>Separado por equipe</small></span></div>
          <p className="art-caption">Exemplo ilustrativo da biblioteca de ferramentas — os itens e nomes não representam dados reais.</p>
        </div>
        <a href="#como-funciona" className="scroll-cue"><ArrowDown size={15} /> CONHEÇA O ZIV</a>
      </section>

      <section id="como-funciona" className="story-section">
        <div className="story-index">01 <span>—</span> MENOS PROCURA, MAIS TRABALHO</div>
        <div className="story-grid">
          <h2>Uma casa para as pequenas ferramentas que fazem <em>grande diferença.</em></h2>
          <div className="story-aside"><p>Planilhas com macros, scripts que alguém da equipe escreveu, pequenos apps que resolvem uma tarefa específica. São úteis — até ninguém lembrar onde estão ou como começar.</p><a href="#para-equipes">Veja o que muda <ArrowRight size={15} /></a></div>
        </div>
        <div className="story-ribbon" aria-hidden="true"><span>MICROAPPS</span><b>+</b><span>SCRIPTS LOCAIS</span><b>+</b><span>UMA EQUIPE</span><b>+</b><span>MENOS ATRITO</span></div>
      </section>

      <section id="para-equipes" className="benefits-section">
        <div className="benefits-intro"><div className="story-index">02 <span>—</span> FEITO PARA O JEITO QUE EMPRESAS TRABALHAM</div><h2>Ferramentas de equipe, sem manual de instruções.</h2></div>
        <div className="benefit-list">
          <article className="benefit"><span className="benefit-number">01</span><div><h3>Encontre sem perguntar no chat.</h3><p>Um catálogo compartilhado mostra o que existe, para que serve e quem pode ajudar.</p></div><span className="benefit-mark">↗</span></article>
          <article className="benefit"><span className="benefit-number">02</span><div><h3>Use sem precisar abrir o código.</h3><p>As ferramentas têm contexto e instruções claras para que colegas possam usá-las com confiança.</p></div><span className="benefit-mark"><MousePointer2 size={19} /></span></article>
          <article className="benefit"><span className="benefit-number">03</span><div><h3>Scripts no computador da equipe.</h3><p>Scripts do Windows rodam no computador da equipe por meio do Agente Ziv — não em um servidor remoto.</p></div><span className="benefit-mark"><Monitor size={19} /></span></article>
        </div>
      </section>

      <section id="privacidade" className="privacy-section">
        <div className="privacy-copy"><div className="story-index">03 <span>—</span> UM ESPAÇO POR EMPRESA</div><h2>O trabalho é compartilhado. <em>O acesso, não.</em></h2><p>Você entra no espaço da sua empresa. As ferramentas e as pessoas de outras empresas ficam em espaços separados — sem diretório público de organizações.</p><Link to="/acesso" className="privacy-link">Acessar meu espaço <ArrowRight size={16} /></Link></div>
        <div className="privacy-visual" aria-hidden="true"><div className="privacy-ring"><div className="privacy-center"><Shield size={26} /><span>ESPAÇO<br />DA EMPRESA</span></div><span className="privacy-node node-a">FINANÇAS</span><span className="privacy-node node-b">OPERAÇÕES</span><span className="privacy-node node-c">COMERCIAL</span></div></div>
      </section>

      <section className="closing-section"><Sparkles size={18} /><h2>O que sua equipe já construiu merece ser encontrado.</h2><p>Comece pelo espaço da sua empresa.</p><Link to="/cadastre-se" className="button-coral">Cadastre-se <ArrowRight size={17} /></Link><span className="closing-login">Já tem um espaço? <Link to="/acesso">Login</Link></span></section>
      <footer className="landing-footer"><Link to="/boas-vindas" aria-label="Ziv, início"><Wordmark className="h-8 w-auto" /></Link><span>Um lugar compartilhado para as ferramentas da equipe.</span><a href="#top" onClick={(event) => { event.preventDefault(); window.scrollTo({ top: 0, behavior: "smooth" }) }}>Voltar ao topo <ChevronDown size={14} className="rotate-180" /></a></footer>
    </main>
  )
}
