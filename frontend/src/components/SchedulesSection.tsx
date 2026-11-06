import { useCallback, useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { CalendarClock, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { RunState } from "@/components/bits"
import { Section } from "@/components/ficha/Part"
import { ScheduleDialog } from "@/components/ScheduleDialog"
import { api, type Manifest, type Schedule } from "@/lib/api"
import { formatRunTime } from "@/lib/schedules"
import { toast } from "sonner"

/**
 * SchedulesSection lists an app's schedules on its page (ADR 024): each person
 * sees theirs; whoever cares for the app sees everyone's, without their values,
 * and can pause or delete them. Hidden while there are none.
 */
export function SchedulesSection({ appId, appName, manifest, reloadKey }: { appId: number; appName: string; manifest: Manifest; reloadKey: number }) {
  const [list, setList] = useState<Schedule[] | null>(null)
  const [editing, setEditing] = useState<Schedule | null>(null)
  const [deleting, setDeleting] = useState<Schedule | null>(null)
  const [busy, setBusy] = useState(0)

  const load = useCallback(() => api.appSchedules(appId).then(setList, () => setList([])), [appId])
  useEffect(() => { void load() }, [load, reloadKey])

  const act = async (s: Schedule, run: () => Promise<unknown>, done: string) => {
    setBusy(s.id)
    try {
      await run()
      toast.success(done)
      await load()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não deu certo. Tente de novo.")
    } finally {
      setBusy(0)
    }
  }

  if (!list || list.length === 0) return null
  return (
    <Section title="Agendamentos">
      <ul className="divide-y divide-border">
        {list.map((s) => (
          <li key={s.id} className="flex flex-col gap-2 py-3 first:pt-0 sm:flex-row sm:items-start sm:justify-between sm:gap-6">
            <div className="min-w-0 space-y-1">
              <p className="flex flex-wrap items-center gap-x-2 text-sm font-medium">
                <CalendarClock className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                {s.name && <><span>{s.name}</span><span aria-hidden className="text-muted-foreground">·</span></>}
                <span>{s.describe}</span>
                {!s.active && <span className="rounded-full bg-secondary px-2 text-xs font-medium text-muted-foreground">Pausado</span>}
              </p>
              <p className="text-xs text-muted-foreground">
                {s.mine ? "Seu" : `De ${s.person_name}`} · no computador {s.computer_name}
                {s.active && s.upcoming[0] && <> · Próxima: {formatRunTime(s.upcoming[0])}</>}
              </p>
              {!s.active && s.paused_reason && (
                <p className="flex items-start gap-1.5 text-sm text-destructive"><TriangleAlert className="mt-0.5 size-4 shrink-0" />{s.paused_reason}</p>
              )}
              {s.active && s.situation && <p className="text-xs text-muted-foreground">{s.situation}</p>}
              {s.last_run_id && s.last_run_status && (
                <p className="flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
                  Última:
                  {s.mine
                    ? <Link to={`/execucoes/${s.last_run_id}`} className="underline-offset-2 hover:underline"><RunState status={s.last_run_status} /></Link>
                    : <RunState status={s.last_run_status} />}
                  {s.last_run_at && <span>· {formatRunTime(s.last_run_at)}</span>}
                </p>
              )}
            </div>
            <div className="flex shrink-0 flex-wrap gap-2">
              {s.active
                ? <Button size="sm" variant="outline" disabled={busy === s.id} onClick={() => void act(s, () => api.pauseSchedule(s.id), "Agendamento pausado.")}>Pausar</Button>
                : s.mine && <Button size="sm" variant="outline" disabled={busy === s.id} onClick={() => void act(s, () => api.resumeSchedule(s.id), "Agendamento reativado.")}>Reativar</Button>}
              {s.mine && <Button size="sm" variant="outline" disabled={busy === s.id} onClick={() => setEditing(s)}>Editar</Button>}
              <Button size="sm" variant="ghost" disabled={busy === s.id} onClick={() => setDeleting(s)}>Apagar</Button>
            </div>
          </li>
        ))}
      </ul>
      {editing && (
        <ScheduleDialog appId={appId} appName={appName} manifest={manifest} schedule={editing} onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); toast.success("Agendamento salvo."); void load() }} />
      )}
      <Dialog open={!!deleting} onOpenChange={(v) => !v && setDeleting(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Apagar este agendamento?</DialogTitle>
            <DialogDescription>
              {deleting?.describe}{deleting && !deleting.mine ? `, de ${deleting.person_name}` : ""}. O app para de rodar sozinho. As execuções que já aconteceram continuam no histórico.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>Manter</Button>
            <Button variant="destructive" onClick={() => {
              const s = deleting!
              setDeleting(null)
              void act(s, () => api.deleteSchedule(s.id), "Agendamento apagado.")
            }}>Apagar agendamento</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Section>
  )
}
