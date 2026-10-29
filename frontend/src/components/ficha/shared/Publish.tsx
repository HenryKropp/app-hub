import { useEffect, useRef, useState } from "react"
import { ExternalLink, TriangleAlert } from "lucide-react"
import { toast } from "sonner"
import { Playing } from "@/components/bits"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, openSharedUrl, type Deployment } from "@/lib/api"
import { errText } from "./settings"

/** Testar, Publicar (or Publicar versão nova), the progress while it builds, and Voltar para a anterior. */
export function PublishBar({ appId, ready, live, latest, liveVersion, hasPrevious, testOpen, slug, interval = 2000, onDone }: {
  appId: number
  ready: boolean
  live: boolean
  latest?: number
  liveVersion?: number
  hasPrevious: boolean
  testUrl?: string
  testOpen?: boolean
  slug?: string
  interval?: number
  onDone: () => void
}) {
  const [dep, setDep] = useState<Deployment | null>(null)
  const [starting, setStarting] = useState(false)
  // null: nothing tested here yet, so the server's testOpen says.
  const [testReady, setTestReady] = useState<boolean | null>(null)
  const [logOpen, setLogOpen] = useState(false)
  const [backOpen, setBackOpen] = useState(false)
  const [withData, setWithData] = useState(false)
  const [goingBack, setGoingBack] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const gone = useRef(false)

  useEffect(() => {
    gone.current = false
    return () => {
      gone.current = true
      clearTimeout(timer.current)
    }
  }, [])

  const building = starting || dep?.status === "building"
  const newer = live && latest != null && liveVersion != null && latest > liveVersion
  const showPublish = !live || newer
  const blocked = !ready || building

  // follow asks for the deployment until it leaves "building".
  const follow = (id: number) => {
    timer.current = setTimeout(async () => {
      let d: Deployment
      try {
        d = await api.shared.deployment(appId, id)
      } catch (e) {
        if (gone.current) return
        toast.error(errText(e))
        setDep(null)
        return
      }
      if (gone.current) return
      setDep(d)
      if (d.status === "building") return follow(id)
      if (d.status === "live") {
        toast.success("No ar.")
        onDone()
      } else if (d.status === "ready") {
        setTestReady(true)
      } else if (d.status === "retired" && d.kind === "test") {
        setTestReady(false)
      }
    }, interval)
  }

  const start = async (test: boolean) => {
    setStarting(true)
    setDep(null)
    try {
      const d = await (test ? api.shared.test(appId) : api.shared.publish(appId))
      if (gone.current) return
      setDep(d)
      if (test) setTestReady(false)
      follow(d.id)
    } catch (e) {
      // 409 (another change in progress) and 503 (Podman off) come with a sentence that says what to do.
      toast.error(errText(e))
    } finally {
      if (!gone.current) setStarting(false)
    }
  }

  const goBack = async () => {
    setGoingBack(true)
    try {
      await api.shared.rollback(appId, withData)
      setBackOpen(false)
      toast.success("Voltou para a versão anterior.")
      onDone()
    } catch (e) {
      toast.error(errText(e))
    } finally {
      if (!gone.current) setGoingBack(false)
    }
  }

  const testing = dep?.kind === "test"
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="outline" className="h-8 px-4" disabled={blocked} onClick={() => start(true)}>Testar</Button>
        {showPublish && (
          <Button variant="run" className="h-8 px-4" disabled={blocked} onClick={() => start(false)}>
            {newer ? "Publicar versão nova" : "Publicar"}
          </Button>
        )}
        {!ready && !building && <span className="text-sm text-muted-foreground">Resolva a checagem primeiro.</span>}
        {live && !newer && !building && <span className="text-sm text-muted-foreground">A versão no ar é a mais nova.</span>}
      </div>

      {building && (
        <p className="flex items-center gap-2 text-sm font-medium text-foreground" role="status">
          <Playing /> {testing ? "Montando a caixa de teste…" : "Publicando…"}
        </p>
      )}

      {dep?.status === "failed" && (
        <div role="alert" className="flex flex-wrap items-start gap-x-3 gap-y-1 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" />
          <span className="min-w-0 flex-1">{dep.error_message || "Não deu certo. Tente de novo."}</span>
          {dep.build_log && (
            <button type="button" className="underline underline-offset-4" onClick={() => setLogOpen(true)}>Ver detalhes</button>
          )}
        </div>
      )}

      {(testReady ?? !!testOpen) && slug && (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
          <a href={openSharedUrl(slug, true)} target="_blank" rel="noopener" className="inline-flex items-center gap-1.5 font-medium text-foreground underline-offset-4 hover:underline">
            <ExternalLink className="size-3.5" /> Abrir o teste
          </a>
          <span className="text-muted-foreground">O teste fica aberto por 30 minutos, com dados separados.</span>
        </div>
      )}

      {hasPrevious && (
        <div>
          <Button size="sm" variant="ghost" className="-ml-2.5 text-muted-foreground" disabled={building} onClick={() => { setWithData(false); setBackOpen(true) }}>
            Voltar para a anterior
          </Button>
        </div>
      )}

      <Dialog open={logOpen} onOpenChange={setLogOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Registro da montagem</DialogTitle>
            <DialogDescription>O que o servidor escreveu enquanto montava o app.</DialogDescription>
          </DialogHeader>
          <pre className="max-h-[60vh] overflow-auto rounded-md bg-secondary p-3 font-mono text-xs whitespace-pre-wrap [overflow-wrap:anywhere]">{dep?.build_log}</pre>
        </DialogContent>
      </Dialog>

      <Dialog open={backOpen} onOpenChange={setBackOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Voltar para a versão anterior</DialogTitle>
            <DialogDescription>O app volta a rodar a versão que estava no ar antes desta.</DialogDescription>
          </DialogHeader>
          <fieldset className="flex flex-col gap-2 text-sm">
            <legend className="sr-only">E os dados?</legend>
            <label className="flex items-start gap-2">
              <input type="radio" name={`rollback-${appId}`} className="mt-0.5 accent-foreground" checked={!withData} onChange={() => setWithData(false)} />
              Só a versão (os dados de agora ficam)
            </label>
            <label className="flex items-start gap-2">
              <input type="radio" name={`rollback-${appId}`} className="mt-0.5 accent-foreground" checked={withData} onChange={() => setWithData(true)} />
              Voltar os dados também (para a cópia feita antes da versão nova)
            </label>
          </fieldset>
          <DialogFooter>
            <Button variant="outline" onClick={() => setBackOpen(false)}>Cancelar</Button>
            <Button onClick={goBack} disabled={goingBack}>Voltar</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
