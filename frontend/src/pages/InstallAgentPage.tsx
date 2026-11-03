import { useEffect } from "react"
import { Link, useNavigate } from "react-router-dom"
import { CircleCheck, LoaderCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { AgentDownloadButton } from "@/components/AgentDownload"
import { useComputerStatus } from "@/lib/computer-status"

// InstallAgentPage is the step right after signup (and the "Como instalar"
// link): download, open, and watch the status turn to connected by itself.
export function InstallAgentPage() {
  const { connected, refresh } = useComputerStatus()
  const navigate = useNavigate()
  useEffect(() => {
    if (connected) return
    const t = setInterval(refresh, 3000)
    return () => clearInterval(t)
  }, [connected, refresh])

  return (
    <section className="mx-auto max-w-xl pt-4">
      <h1 className="font-heading text-2xl">Conecte seu computador</h1>
      <p className="mt-2 text-muted-foreground">O Agente é um programa pequeno que fica no seu Windows, sem ícone. É ele que abre os apps do catálogo no seu computador quando você clica em Rodar.</p>
      <ol className="mt-6 flex list-decimal flex-col gap-2 pl-5 text-sm">
        <li>Baixe o Agente pelo botão abaixo.</li>
        <li>Abra o arquivo baixado. Se o Windows perguntar, confirme que quer abrir.</li>
        <li>O navegador vai abrir uma página do Ziv. Clique em <strong>Conectar</strong>.</li>
      </ol>
      <div className="mt-6"><AgentDownloadButton /></div>
      <div className="mt-8 rounded-xl border border-border bg-card p-5" aria-live="polite">
        {connected ? (
          <div className="flex flex-wrap items-center gap-3">
            <CircleCheck className="size-5 text-ok" />
            <p className="flex-1 font-medium text-ok">Computador conectado. Você já pode rodar os apps do catálogo.</p>
            <Button onClick={() => navigate("/comecar")}>Continuar</Button>
          </div>
        ) : (
          <div className="flex items-center gap-3 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            <p>Esperando o seu computador conectar…</p>
          </div>
        )}
      </div>
      {!connected && <p className="mt-4 text-sm"><Link to="/comecar" className="text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">Pular por agora</Link></p>}
    </section>
  )
}
