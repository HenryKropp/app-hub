import { useState, type ReactNode } from "react"
import { Button } from "@/components/ui/button"
import { ApiError } from "@/lib/api"

/** One titled block of the ficha: the title on the left, the content on the right. */
export function Section({ title, aside, children, id }: { title: string; aside?: ReactNode; children: ReactNode; id?: string }) {
  return (
    <section className="grid gap-x-10 gap-y-3 border-t border-border py-6 md:grid-cols-[12rem_1fr]" aria-labelledby={id}>
      <div className="flex flex-col items-start gap-1.5">
        <h2 id={id} className="text-sm font-semibold">{title}</h2>
        {aside}
      </div>
      <div className="min-w-0">{children}</div>
    </section>
  )
}

/** The quiet grey "Editar" every part uses. */
export function EditLink({ part, onClick }: { part: string; onClick: () => void }) {
  return (
    <button type="button" className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline" onClick={onClick} aria-label={`Editar ${part}`}>
      Editar
    </button>
  )
}

/** Salvar/Cancelar for an open part, with the save error shown inside it. */
export function SaveBar({ dirty, onSave, onCancel, extra }: { dirty: boolean; onSave: () => Promise<void>; onCancel: () => void; extra?: ReactNode }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  // extra (e.g. "Recarregar") only helps when the ficha changed or is gone, not
  // when something is missing here: reloading would throw the edits away.
  const [stale, setStale] = useState(false)
  const save = async () => {
    setBusy(true)
    setError("")
    setStale(false)
    try {
      await onSave()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não consegui salvar.")
      setStale(e instanceof ApiError && (e.status === 409 || e.status === 404))
    } finally {
      setBusy(false)
    }
  }
  const cancel = () => {
    if (dirty && !window.confirm("Descartar o que você mudou?")) return
    setError("")
    onCancel()
  }
  return (
    <>
      {error && (
        <div role="alert" className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <span>{error}</span>
          {stale && extra}
        </div>
      )}
      <div className="flex gap-2">
        <Button size="sm" onClick={save} disabled={busy}>{busy ? "Salvando…" : "Salvar"}</Button>
        <Button size="sm" variant="ghost" onClick={cancel} disabled={busy}>Cancelar</Button>
      </div>
    </>
  )
}

// Part shows one piece of the ficha. With canEdit, a quiet "Editar" opens edit,
// and Salvar/Cancelar close it. Cancel with changes asks first.
export function Part({ title, canEdit, dirty, onSave, onCancel, view, edit, editing, onEdit, extra }: {
  title: string
  canEdit: boolean
  dirty: boolean
  editing: boolean
  onEdit: () => void
  onSave: () => Promise<void>
  onCancel: () => void
  view: ReactNode
  edit: ReactNode
  extra?: ReactNode
}) {
  const id = `part-${title.toLowerCase().replace(/\W+/g, "-")}`
  return (
    <Section title={title} id={id} aside={canEdit && !editing && <EditLink part={title} onClick={onEdit} />}>
      {editing ? (
        <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-xs">
          {edit}
          <SaveBar dirty={dirty} onSave={onSave} onCancel={onCancel} extra={extra} />
        </div>
      ) : (
        view
      )}
    </Section>
  )
}
