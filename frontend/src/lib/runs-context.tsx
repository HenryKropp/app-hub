import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { RunDialog } from "@/components/RunDialog"
import { api, ApiError, type ActiveRun } from "@/lib/api"
import { isLive } from "@/lib/runs"

export type RunTarget = { id: number; name: string; folder_display: string; remembered_folder: string }
// folder: run right away in this folder (Rodar de novo); ask: always open the dialog ("trocar").
export type RunOptions = { folder?: string; ask?: boolean }
// runId: the request succeeded; the notice stays until the server lists that run.
export type LocalNotice = { key: string; app_name: string; kind: "starting" | "error"; message?: string; offline?: boolean; target?: RunTarget; runId?: number }

type Ctx = {
  active: ActiveRun[]
  notices: LocalNotice[]
  busy: Set<number>
  run: (t: RunTarget, o?: RunOptions) => void
  dismiss: (id: number) => void
  dropNotice: (key: string) => void
  refresh: () => void
  rememberedFor: (t: RunTarget) => string
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

  // Hand over from "Abrindo…" to the server's card without a gap.
  useEffect(() => {
    const ids = new Set(active.map((r) => r.id))
    setNotices((ns) => (ns.some((n) => n.runId !== undefined && ids.has(n.runId)) ? ns.filter((n) => n.runId === undefined || !ids.has(n.runId)) : ns))
  }, [active])

  const live = active.some((r) => isLive(r.status)) || notices.some((n) => n.kind === "starting")
  useEffect(() => {
    refresh()
    const t = setInterval(refresh, live ? 3000 : 30000)
    return () => clearInterval(t)
  }, [refresh, live])

  const rememberedFor = useCallback((t: RunTarget) => (t.id in remembered ? remembered[t.id] : t.remembered_folder), [remembered])

  const send = useCallback(
    (t: RunTarget, folder: string, remember?: boolean) => {
      const key = `n${++seq}`
      setNotices((ns) => [...ns, { key, app_name: t.name, kind: "starting" }])
      setBusy((b) => new Set(b).add(t.id))
      api
        .startRun(t.id, folder, remember)
        .then(
          (r) => {
            if (remember !== undefined) setRemembered((m) => ({ ...m, [t.id]: remember ? folder : "" }))
            setNotices((ns) => ns.map((n) => (n.key === key ? { ...n, runId: r.id } : n)))
            refresh()
          },
          (e: unknown) => {
            const message = e instanceof Error ? e.message : "Não foi possível abrir o app."
            const offline = e instanceof ApiError && e.status === 409
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
      if (o.folder) return send(t, o.folder)
      const saved = rememberedFor(t)
      if (o.ask || saved === "") return setAsking({ ...t, remembered_folder: saved })
      send(t, saved)
    },
    [busy, send, rememberedFor],
  )

  const dismiss = useCallback((id: number) => {
    dismissed.current.add(id)
    setActive((a) => a.filter((r) => r.id !== id))
    api.dismissRun(id).catch(() => {})
  }, [])

  const dropNotice = useCallback((key: string) => setNotices((ns) => ns.filter((n) => n.key !== key)), [])

  const value = useMemo(
    () => ({ active, notices, busy, run, dismiss, dropNotice, refresh, rememberedFor }),
    [active, notices, busy, run, dismiss, dropNotice, refresh, rememberedFor],
  )

  return (
    <RunsCtx.Provider value={value}>
      {children}
      <RunDialog
        target={asking}
        onCancel={() => setAsking(null)}
        onConfirm={(folder, remember) => {
          const t = asking
          setAsking(null)
          if (t) send(t, folder, remember)
        }}
      />
    </RunsCtx.Provider>
  )
}
