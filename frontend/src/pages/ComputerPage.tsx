import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Download, Laptop, TriangleAlert } from "lucide-react"
import { PageHeader } from "@/components/bits"
import { Button, buttonVariants } from "@/components/ui/button"
import { AGENT_DOWNLOAD_URL, api, type ComputersInfo } from "@/lib/api"
import { formatWhen } from "@/lib/runs"
import { cn } from "@/lib/utils"

export function ComputerPage() {
  const [info, setInfo] = useState<ComputersInfo | null>(null)
  const [error, setError] = useState("")

  const load = () => api.computers().then(setInfo, (e: Error) => setError(e.message))
  useEffect(() => {
    load()
    const t = setInterval(load, 5000)
    return () => clearInterval(t)
  }, [])

  const disconnect = async (id: number, name: string) => {
    if (!window.confirm(`Desconectar ${name}? O Hub não vai mais abrir apps nele até você instalar o Agente de novo.`)) return
    await api.disconnect(id).then(() => toast.success(`${name} foi desconectado.`), (e: Error) => toast.error(e.message))
    load()
  }

  if (error) return <p role="alert" className="text-destructive">{error}</p>
  if (!info) return <p className="text-muted-foreground">Carregando…</p>

  const install = info.agent_available ? (
    <a href={AGENT_DOWNLOAD_URL} className={cn(buttonVariants(), "gap-2")}>
      <Download className="size-4" /> Instalar o Agente
    </a>
  ) : (
    <p className="text-sm text-warn">O Agente ainda não foi preparado neste Hub. Peça a quem administra o Hub.</p>
  )

  return (
    <>
      <PageHeader eyebrow="Agente do Hub" title="Meu computador">
        O Agente é um programa pequeno que fica no seu Windows, sem ícone. É ele que abre os apps quando você clica em Rodar.
      </PageHeader>

      {info.computers.length === 0 ? (
        <section className="paper rounded-lg border border-border bg-card p-8 shadow-card">
          <h2 className="font-heading text-2xl">Nenhum computador conectado</h2>
          <ol className="mt-4 flex list-decimal flex-col gap-2 pl-5 text-sm">
            <li>Baixe o Agente pelo botão abaixo.</li>
            <li>Abra o arquivo baixado (AppHubAgente.exe). Se o Windows perguntar, confirme que quer abrir.</li>
            <li>O navegador vai abrir uma página do Hub. Clique em <strong>Conectar</strong>.</li>
          </ol>
          <div className="mt-6">{install}</div>
        </section>
      ) : (
        <ul className="flex flex-col gap-4">
          {info.computers.map((c) => (
            <li key={c.id} className="paper flex flex-wrap items-center gap-4 rounded-lg border border-border bg-card p-6 shadow-card">
              <Laptop className="size-8 text-stamp" />
              <div className="min-w-0 flex-1">
                <p className="font-heading text-xl">{c.name}</p>
                <p className="text-sm text-muted-foreground">
                  <span className={c.connected ? "font-medium text-ok" : "font-medium text-warn"}>{c.connected ? "Conectado" : "Desligado ou sem internet"}</span>
                  {c.last_seen_at && ` · visto pela última vez em ${formatWhen(c.last_seen_at)}`}
                  {c.agent_version && ` · Agente ${c.agent_version}`}
                </p>
                {c.outdated && (
                  <p className="mt-2 flex items-center gap-2 text-sm text-warn">
                    <TriangleAlert className="size-4" /> Atualize o Agente: baixe e abra a versão {info.expected_version}.
                  </p>
                )}
              </div>
              <Button variant="outline" onClick={() => disconnect(c.id, c.name)}>Desconectar</Button>
            </li>
          ))}
          <li className="pt-2">{install}</li>
        </ul>
      )}
    </>
  )
}
