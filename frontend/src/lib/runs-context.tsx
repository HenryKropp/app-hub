import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { RunDialog } from "@/components/RunDialog"
import { StopDialog, type StopTarget } from "@/components/StopDialog"
import type { Submission } from "@/components/RunDialog"
import { api, ApiError, type ActiveRun, type Kind } from "@/lib/api"
import { isLive } from "@/lib/runs"

export type RunTarget = { id: number; name: string; folder_display: string; remembered_folder: string; kind?: Kind }
// folder: reuse this folder (Rodar de novo); scripts still ask for inputs.
// ask: always open the dialog ("trocar").
export type RunOptions = { folder?: string; ask?: boolean }
// runId: the request succeeded; the notice stays until the server lists that run.
export type LocalNotice = { key: string; app_name: string; kind: "starting" | "error"; folder?: string; message?: string; offline?: boolean; target?: RunTarget; runId?: number }

type Ctx = {
  active: ActiveRun[]
  notices: LocalNotice[]
  busy: Set<number>
  run: (t: RunTarget, o?: RunOptions) => void
  dismiss: (id: number) => void
  dropNotice: (key: string) => void
  refresh: () => void
  rememberedFor: (t: RunTarget) => string
  // Parar: asks first, then keeps the run as "Parando…" until it closes.
  askStop: (t: StopTarget) => void
  stopping: Set<number>
}

const RunsCtx = createContext<Ctx | null>(null)

export function useRuns(): Ctx {
  const c = useContext(RunsCtx)
  if (!c) throw new Error("useRuns fora do RunsProvider")
  return c
}

let seq = 0

// RunsProvider is the one place that starts runs: it decides between asking
// for the folder and running right away, and keeps the strip's runs fresh.
export function RunsProvider({ children }: { children: ReactNode }) {
  const [active, setActive] = useState<ActiveRun[]>([])
  const [notices, setNotices] = useState<LocalNotice[]>([])
  const [busy, setBusy] = useState<Set<number>>(new Set())
  const [asking, setAsking] = useState<RunTarget | null>(null)
  // Folders remembered or forgotten in this tab, ahead of the page data.
  const [remembered, setRemembered] = useState<Record<number, string>>({})

  // Runs dismissed in this tab, so a poll already in flight can't bring them back.
  const dismissed = useRef(new Set<number>())

  const refresh = useCallback(() => {
    api.activeRuns().then((rs) => setActive(rs.filter((r) => !dismissed.current.has(r.id))), () => {})
  }, [])

  // Hand over from "Abrindo…" to the server's card without a gap, whichever
  // arrives first: the server listing the run or the notice learning its id.
  useEffect(() => {
    const ids = new Set(active.map((r) => r.id))
    if (notices.some((n) => n.runId !== undefined && ids.has(n.runId))) setNotices((ns) => ns.filter((n) => n.runId === undefined || !ids.has(n.runId)))
  }, [active, notices])

  const live = active.some((r) => isLive(r.status)) || notices.some((n) => n.kind === "starting")
  useEffect(() => {
    refresh()
    const t = setInterval(refresh, live ? 3000 : 30000)
    return () => clearInterval(t)
  }, [refresh, live])

  const rememberedFor = useCallback((t: RunTarget) => (t.id in remembered ? remembered[t.id] : t.remembered_folder), [remembered])

  const send = useCallback(
    (t: RunTarget, folder: string, remember?: boolean, inputs?: Record<string, string | string[]>, files?: Record<string, { name: string; content_base64: string }>) => {
      const key = `n${++seq}`
      setNotices((ns) => [...ns, { key, app_name: t.name, kind: "starting", folder }])
      setBusy((b) => new Set(b).add(t.id))
      const request = Object.keys(inputs ?? {}).length || Object.keys(files ?? {}).length
        ? api.startRun(t.id, folder, remember, inputs, files)
        : api.startRun(t.id, folder, remember)
      request
        .then(
          (r) => {
            if (remember !== undefined) setRemembered((m) => ({ ...m, [t.id]: remember ? folder : "" }))
            setNotices((ns) => ns.map((n) => (n.key === key ? { ...n, runId: r.id } : n)))
            refresh()
          },
          (e: unknown) => {
            const message = e instanceof Error ? e.message : "Não foi possível abrir o app."
            // A folder used by another open app is a 409 too, but the fix is another folder.
            const offline = e instanceof ApiError && e.status === 409 && !/pasta/i.test(message)
            setNotices((ns) => ns.map((n) => (n.key === key ? { ...n, kind: "error", message, offline, target: t } : n)))
          },
        )
        .finally(() =>
          setBusy((b) => {
            const nb = new Set(b)
            nb.delete(t.id)
            return nb
          }),
        )
    },
    [refresh],
  )

  const run = useCallback(
    (t: RunTarget, o: RunOptions = {}) => {
      if (busy.has(t.id)) return
      const decide = (target: RunTarget) => {
        if (o.folder) {
          if (target.kind === "script") setAsking({ ...target, remembered_folder: o.folder! })
          else send(target, o.folder)
          return
        }
        if (target.kind === "script") return setAsking({ ...target, remembered_folder: rememberedFor(target) })
        const saved = rememberedFor(target)
        if (o.ask || saved === "") return setAsking({ ...target, remembered_folder: saved })
        send(target, saved)
      }
      if (t.kind) return decide(t)
      // Older run-history links do not include the app kind. Resolve it before
      // deciding whether this is the desktop folder dialog or the script form.
      setBusy((current) => new Set(current).add(t.id))
      api.app(t.id).then((detail) => {
        setBusy((current) => {
          const next = new Set(current)
          next.delete(t.id)
          return next
        })
        decide({ ...t, kind: detail.manifest.kind })
      }, (e: unknown) => {
        setBusy((current) => {
          const next = new Set(current)
          next.delete(t.id)
          return next
        })
        const key = `n${++seq}`
        const message = e instanceof Error ? e.message : "Não consegui carregar os dados do app."
        setNotices((current) => [...current, { key, app_name: t.name, kind: "error", message, target: t }])
      })
    },
    [busy, send, rememberedFor],
  )

  const dismiss = useCallback((id: number) => {
    dismissed.current.add(id)
    setActive((a) => a.filter((r) => r.id !== id))
    api.dismissRun(id).catch(() => {})
  }, [])

  const dropNotice = useCallback((key: string) => setNotices((ns) => ns.filter((n) => n.key !== key)), [])

  const [stopAsk, setStopAsk] = useState<StopTarget | null>(null)
  const [stopping, setStopping] = useState<Set<number>>(new Set())
  const askStop = useCallback((t: StopTarget) => setStopAsk(t), [])
  // A run leaves "Parando…" once the server shows it closed (or gone from the strip).
  useEffect(() => {
    if (stopping.size === 0) return
    const open = new Set(active.filter((r) => isLive(r.status)).map((r) => r.id))
    if ([...stopping].some((id) => !open.has(id))) setStopping((s) => new Set([...s].filter((id) => open.has(id))))
  }, [active, stopping])

  const value = useMemo(
    () => ({ active, notices, busy, run, dismiss, dropNotice, refresh, rememberedFor, askStop, stopping }),
    [active, notices, busy, run, dismiss, dropNotice, refresh, rememberedFor, askStop, stopping],
  )

  return (
    <RunsCtx.Provider value={value}>
      {children}
      <RunDialog
        target={asking}
        onCancel={() => setAsking(null)}
        onConfirm={(submission: Submission) => {
          const t = asking
          setAsking(null)
          if (t) send(t, submission.folder, submission.remember, submission.inputs, submission.files)
        }}
      />
      <StopDialog
        target={stopAsk}
        onCancel={() => setStopAsk(null)}
        onStopped={(id) => {
          setStopAsk(null)
          setStopping((s) => new Set(s).add(id))
          refresh()
        }}
      />
    </RunsCtx.Provider>
  )
}
