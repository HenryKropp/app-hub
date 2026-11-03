import { Download, TriangleAlert } from "lucide-react"
import { Link } from "react-router-dom"
import { Button, buttonVariants } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { AGENT_DOWNLOAD_URL } from "@/lib/api"
import { useComputerStatus } from "@/lib/computer-status"
import { cn } from "@/lib/utils"

export const NO_AGENT_TEXT = "Sem o Agente, não será possível rodar os apps do catálogo."

// AgentDownloadButton downloads this company's Agente: one EXE that already
// knows the company, nothing to extract.
export function AgentDownloadButton({ variant = "default", size = "default", available }: { variant?: "default" | "outline"; size?: "default" | "sm"; available?: boolean }) {
  const { info } = useComputerStatus()
  if (available === false || (available === undefined && info && !info.agent_available)) return <p className="text-sm text-muted-foreground">O Agente ainda não foi preparado neste Ziv. Peça a quem administra.</p>
  return <a href={AGENT_DOWNLOAD_URL} className={cn(buttonVariants({ variant, size }), "gap-2")}><Download className="size-4" /> Baixar Agente</a>
}

// AgentBanner is the strip at the top of every page while this person's
// computer is not connected.
export function AgentBanner() {
  const { connected } = useComputerStatus()
  if (connected !== false) return null
  return (
    <div role="status" className="mb-6 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-warn/30 bg-warn-soft px-4 py-3 text-sm">
      <TriangleAlert className="size-4 shrink-0 text-warn" />
      <p className="min-w-0 flex-1"><span className="font-medium">Seu computador não está conectado ao Ziv.</span> {NO_AGENT_TEXT}</p>
      <div className="flex items-center gap-3">
        <AgentDownloadButton size="sm" />
        <Link to="/instalar-agente" className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">Como instalar</Link>
      </div>
    </div>
  )
}

// AgentNeededDialog opens when someone clicks Rodar without a connected computer.
export function AgentNeededDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Conecte seu computador para rodar</DialogTitle>
          <DialogDescription>O Agente é um programa pequeno que abre os apps do catálogo no seu Windows. {NO_AGENT_TEXT}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Link to="/instalar-agente" onClick={onClose} className={buttonVariants({ variant: "outline" })}>Como instalar</Link>
          <Button variant="ghost" onClick={onClose}>Fechar</Button>
          <AgentDownloadButton />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
