import { useCallback, useEffect, useState } from "react"
import { Share2 } from "lucide-react"
import { toast } from "sonner"
import { Section } from "@/components/ficha/Part"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { api, type AppDetail, type Can, type EnvVar, type Readiness, type SharedInfo } from "@/lib/api"
import { Backups } from "./Backups"
import { PublishBar } from "./Publish"
import { ReadinessList } from "./Readiness"
import { SharedSettingsForm } from "./Settings"
import { SharedLine } from "./SharedLine"
import { errText, settingsOf } from "./settings"

type Confirm = "stop" | "unshare" | null

const LOAD_ERR = "O Kova não respondeu sobre o app compartilhado. Tente de novo em alguns segundos."

/** A load that failed: what happened and Tentar de novo. */
export function LoadFailed({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-start gap-2">
      <p className="text-sm text-destructive" role="alert">{message}</p>
      <Button size="sm" variant="outline" onClick={onRetry}>Tentar de novo</Button>
    </div>
  )
}

/**
 * The "Compartilhado" part of the ficha. Whoever takes care of the app
 * (can.restore) gets all of it; everyone else only the short line with Abrir.
 * onChanged gets the new ficha when a save returned one, or nothing when the
 * ficha should be loaded again.
 */
export function SharedSection({ app, can, onChanged }: { app: AppDetail; can: Can; onChanged: (next?: AppDetail) => void }) {
  const [info, setInfo] = useState<SharedInfo | null>(null)
  const [check, setCheck] = useState<Readiness | null>(null)
  const [checkError, setCheckError] = useState("")
  const [checking, setChecking] = useState(false)
  const [acks, setAcks] = useState<string[]>([])
  const [loadError, setLoadError] = useState("")
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState<Confirm>(null)
  const id = app.id
  const cares = can.restore

  const loadInfo = useCallback(() => {
    api.shared.get(id).then(
      (i) => {
        setInfo(i)
        setAcks(i.shared?.acks ?? [])
        setLoadError("")
      },
      // A failed look while the part is on screen keeps what it shows; a
      // failed first load says so instead of hiding the part.
      (e) => setLoadError(errText(e, LOAD_ERR)),
    )
  }, [id])
  const loadCheck = useCallback(() => {
    setChecking(true)
    api.shared.check(id).then(
      (c) => {
        setCheck(c)
        setCheckError("")
      },
      (e) => setCheckError(errText(e, "Não consegui conferir o app agora.")),
    ).finally(() => setChecking(false))
  }, [id])

  useEffect(loadInfo, [loadInfo])
  useEffect(() => {
    if (cares) loadCheck()
  }, [cares, loadCheck])

  // While it is turning on, look again every few seconds.
  const status = info?.shared?.status
  useEffect(() => {
    if (status !== "starting") return
    const t = setInterval(loadInfo, 3000)
    return () => clearInterval(t)
  }, [status, loadInfo])

  const state = info?.shared ?? null
  if (!info && loadError) {
    return (
      <Section title="Compartilhado">
        <LoadFailed message={loadError} onRetry={loadInfo} />
      </Section>
    )
  }
  if (!cares) {
    return (
      <Section title="Compartilhado">
        <SharedLine app={app} info={info} />
      </Section>
    )
  }
  if (!info) return <Section title="Compartilhado"><p className="text-sm text-muted-foreground">Carregando…</p></Section>
  if (!state) return null

  const refresh = () => {
    loadInfo()
    onChanged()
  }
  const act = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true)
    try {
      await fn()
      toast.success(done)
      refresh()
      return true
    } catch (e) {
      // A busy app answers 409 with a sentence; show it, do not try again by itself.
      toast.error(errText(e))
      return false
    } finally {
      setBusy(false)
    }
  }
  const start = () => act(() => api.shared.start(id), "Ligando o app.")
  const stop = async () => {
    if (await act(() => api.shared.stop(id), "App parado.")) setConfirm(null)
  }
  const unshare = async () => {
    if (await act(() => api.shared.unshare(id), "O app deixou de ser compartilhado.")) setConfirm(null)
  }

  const saveAcks = async (next: string[]) => {
    const before = acks
    setAcks(next)
    try {
      const saved = await api.shared.save(id, settingsOf(app, state, { acks: next }))
      // The marks now saved are the ones every later save must carry.
      setInfo((i) => (i?.shared ? { ...i, shared: { ...i.shared, acks: next } } : i))
      onChanged(saved)
      loadCheck()
    } catch (e) {
      setAcks(before)
      toast.error(errText(e, "Não consegui salvar."))
    }
  }
  const addEnv = async (e: EnvVar) => {
    // acks: the marks on screen (the saved ones), never an older copy.
    const s = settingsOf(app, state, { acks })
    if (s.env.some((x) => x.name === e.name)) throw new Error(`${e.name} já está na lista.`)
    const saved = await api.shared.save(id, { ...s, env: [...s.env, e] })
    onChanged(saved)
  }

  const live = info.live_version != null
  const ready = !!check?.ready
  const on = status === "running" || status === "sleeping" || status === "starting"

  return (
    <>
      <Section title="Compartilhado" id="part-compartilhado">
        <div className="flex flex-col gap-5">
          <div className="flex flex-col gap-1">
            <SharedLine app={app} info={info} onRestart={start} busy={busy} />
            {state.status_message && <p className="text-sm text-muted-foreground">{state.status_message}</p>}
          </div>

          {(!ready || !live) && (
            <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
              <h3 className="text-sm font-semibold">Checagem antes de publicar</h3>
              {checkError ? (
                <div className="flex flex-col items-start gap-2">
                  <p className="text-sm text-destructive" role="alert">{checkError}</p>
                  <Button size="sm" variant="outline" onClick={loadCheck} disabled={checking}>Conferir de novo</Button>
                </div>
              ) : check ? (
                <ReadinessList report={check} acks={acks} onAck={saveAcks} onRecheck={loadCheck} busy={checking} />
              ) : (
                <p className="text-sm text-muted-foreground">Conferindo o app…</p>
              )}
            </div>
          )}

          <PublishBar
            appId={id}
            ready={ready}
            live={live}
            latest={info.latest_version}
            liveVersion={info.live_version}
            hasPrevious={state.has_previous}
            testUrl={info.test_url}
            testOpen={info.test_open}
            slug={app.shared?.slug}
            onDone={refresh}
          />

          <div className="flex flex-wrap gap-2 border-t border-border pt-4">
            {live && on && <Button size="sm" variant="outline" onClick={() => setConfirm("stop")} disabled={busy}>Parar</Button>}
            {live && status === "stopped" && <Button size="sm" variant="outline" onClick={start} disabled={busy}>Ligar</Button>}
            <Button size="sm" variant="ghost" className="text-muted-foreground" onClick={() => setConfirm("unshare")} disabled={busy}>Deixar de compartilhar</Button>
          </div>
        </div>
      </Section>

      <Section title="Configurações">
        <SharedSettingsForm appId={id} onAdd={addEnv} />
      </Section>

      {state.wants_db && (
        <Section title="Cópias dos dados">
          <Backups appId={id} onRestored={refresh} />
        </Section>
      )}

      <Dialog open={confirm !== null} onOpenChange={(o) => !o && setConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{confirm === "stop" ? "Parar o app?" : "Deixar de compartilhar?"}</DialogTitle>
            <DialogDescription>
              {confirm === "stop"
                ? "Ninguém consegue abrir até alguém ligar de novo."
                : "O endereço para de funcionar. Os dados ficam guardados por 30 dias; depois são apagados."}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirm(null)}>Cancelar</Button>
            {confirm === "stop" ? (
              <Button variant="destructive" onClick={stop} disabled={busy}>Sim, parar</Button>
            ) : (
              <Button variant="destructive" onClick={unshare} disabled={busy}>Sim, deixar de compartilhar</Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

/** For a web app that is not shared yet: one sentence and the button. */
export function MakeShared({ app, onChanged }: { app: AppDetail; onChanged: (next: AppDetail) => void }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const make = async () => {
    setBusy(true)
    setError("")
    try {
      onChanged(await api.shared.save(app.id, settingsOf(app, null, { acks: [] })))
    } catch (e) {
      setError(errText(e, "Não consegui tornar o app compartilhado."))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Section title="Compartilhado">
      <div className="flex flex-col items-start gap-3 rounded-xl border border-dashed border-border p-4">
        <p className="flex gap-2 text-sm">
          <Share2 className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          Fica num endereço só, para várias pessoas usarem ao mesmo tempo, com os dados guardados no Kova.
        </p>
        {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
        <Button size="sm" variant="outline" onClick={make} disabled={busy}>Tornar compartilhado</Button>
      </div>
    </Section>
  )
}
